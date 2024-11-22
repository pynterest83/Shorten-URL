package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var RedisClient *redis.Client
var DB *gorm.DB
var ctx = context.Background()

// URL model for GORM
type URL struct {
	ID  string `gorm:"primary_key"`
	URL string `gorm:"not null"`
}

var urlQueue = make(chan URL, 5000) // Channel hàng đợi với buffer 5000

func main() {
	port := flag.String("port", "8080", "Port for the server to listen on")
	flag.Parse()

	// Khởi tạo Redis
	RedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		panic(err)
	}

	// Khởi tạo kết nối với cơ sở dữ liệu và thiết lập pool
	var err error
	dsn := "host=localhost user=shortenurl password=shortenurl dbname=shortenurl port=5432"
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic(err)
	}

	// Khởi tạo các worker để xử lý hàng đợi
	startWorkers(3)

	// Set up the router with CORS
	router := mux.NewRouter()
	router.HandleFunc("/short/{id}", GetLink).Methods("GET")
	router.HandleFunc("/create", ShortenURL).Methods("POST")
	router.HandleFunc("/delete-all", deleteAllURLsHandler).Methods("DELETE")

	// Configure CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowCredentials: true,
	})

	// Wrap router with CORS middleware
	handler := corsHandler.Handler(router)

	// Start server with CORS-enabled handler and specified port
	fmt.Printf("Starting server on port %s...\n", *port)
	if http.ListenAndServe(":"+*port, handler) != nil {
		return
	}
}

// startWorkers khởi tạo các worker goroutine để xử lý batch ghi
func startWorkers(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			batchSize := 200
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()

			var urls []URL

			for {
				select {
				case url := <-urlQueue:
					urls = append(urls, url)

					// Nếu đạt đến kích thước batch, ghi tất cả vào DB
					if len(urls) >= batchSize {
						batchInsert(urls)
						urls = urls[:0] // Reset lại slice sau khi ghi
					}
				case <-ticker.C:
					// Ghi bất cứ bản ghi nào còn lại vào cuối mỗi giây
					if len(urls) > 0 {
						batchInsert(urls)
						urls = urls[:0]
					}
				}
			}
		}(i)
	}
}

// GetLink handles the request to fetch the original URL based on the shortened ID
func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	// Kiểm tra trong Redis cache
	data, err := RedisClient.Get(ctx, id).Result()
	if errors.Is(err, redis.Nil) {
		// Nếu không tìm thấy trong cache, tìm trong cơ sở dữ liệu
		var url URL
		if err := DB.Where("id = ?", id).First(&url).Error; err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		// Lưu vào cache Redis với TTL (thời gian sống) là 5 phút
		_ = RedisClient.Set(ctx, id, url.URL, 5*time.Minute).Err()

		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": url.URL})
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else {
		// Nếu tìm thấy trong cache, trả về dữ liệu từ Redis
		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": data})
	}
}

// ShortenURL handles the request to shorten a given URL
func ShortenURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	url := r.FormValue("url")
	if url == "" {
		w.WriteHeader(400) // Bad Request if no URL is provided
		return
	}

	// Generate a new unique ID for the shortened URL
	newID := makeID()

	// Đưa bản ghi vào hàng đợi thay vì ghi trực tiếp vào cơ sở dữ liệu
	urlRecord := URL{ID: newID, URL: url}
	urlQueue <- urlRecord // Đưa vào hàng đợi

	// Trả về ID của URL rút gọn ngay lập tức
	_, _ = w.Write([]byte(newID))
}

// batchInsert thực hiện batch insert vào cơ sở dữ liệu
func batchInsert(urls []URL) {
	if err := DB.Create(&urls).Error; err != nil {
		// Log lỗi nếu batch insert thất bại
		println("Batch insert failed:", err.Error())
	}
}

// makeID generates a random alphanumeric string of the specified length
func makeID() string {
	return uuid.New().String()
}

func deleteAllURLsHandler(w http.ResponseWriter, _ *http.Request) {
	if err := deleteAllRecords(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to delete records"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("All records deleted successfully"))
}

func deleteAllRecords() error {
	if err := DB.Unscoped().Where("1 = 1").Delete(&URL{}).Error; err != nil {
		return err
	}
	println("All records deleted from URL table")
	return nil
}
