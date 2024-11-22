package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"math/big"
	"net/http"
	"sync"
	"time"

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
	ID         string      `gorm:"primary_key"`
	URL        string      `gorm:"not null"`
	ResultChan chan string `gorm:"-"`
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
			ticker := time.NewTicker(5 * time.Millisecond)
			defer ticker.Stop()

			var urls []URL
			var mutex sync.Mutex

			for {
				select {
				case url := <-urlQueue:
					mutex.Lock()
					urls = append(urls, url)
					mutex.Unlock()

					// Nếu đạt đến kích thước batch, ghi tất cả vào DB
					if len(urls) >= batchSize {
						mutex.Lock()
						batchInsert(urls)
						urls = nil // Reset slice sau khi ghi
						mutex.Unlock()
					}
				case <-ticker.C:
					// Ghi bất cứ bản ghi nào còn lại vào cuối mỗi giây
					mutex.Lock()
					if len(urls) > 0 {
						batchInsert(urls)
						urls = nil
					}
					mutex.Unlock()
				}
			}
		}(i)
	}
}

// ShortenURL handles the request to shorten a given URL
func ShortenURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Tạo channel để chờ kết quả
	resultChan := make(chan string)

	// Đưa URL vào hàng đợi
	urlQueue <- URL{URL: url, ResultChan: resultChan}

	// Chờ kết quả từ worker
	newID := <-resultChan

	// Trả về ID
	response := map[string]string{"id": newID}
	_ = json.NewEncoder(w).Encode(response)
}

// batchInsert thực hiện batch insert vào cơ sở dữ liệu
func batchInsert(urls []URL) {
	const maxRetries = 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Tạo ID mới cho các URL trong mỗi lần retry
		for i := range urls {
			urls[i].ID = makeID() // Tạo lại ID mỗi lần thử lại
		}

		// Bắt đầu transaction
		tx := DB.Begin()

		// Thực hiện batch insert
		if err := tx.Create(&urls).Error; err != nil {
			tx.Rollback() // Rollback nếu xảy ra lỗi

			// Kiểm tra lỗi duplicate key
			fmt.Printf("Batch insert failed on attempt %d: %v\n", attempt, err)

			// Nếu là lỗi duplicate key, thử lại với ID mới
			continue
		} else {
			// Commit transaction nếu thành công
			if err := tx.Commit().Error; err != nil {
				fmt.Printf("Transaction commit failed on attempt %d: %v\n", attempt, err)
			} else {
				// Gửi kết quả cho từng URL
				for _, url := range urls {
					if url.ResultChan != nil {
						url.ResultChan <- url.ID
					}
				}
				return
			}
		}
	}

	// Nếu sau maxRetries vẫn thất bại, trả về lỗi
	for _, url := range urls {
		if url.ResultChan != nil {
			url.ResultChan <- "error"
		}
	}
}

var charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// makeID generates a random alphanumeric string of the specified length
func makeID() string {
	id := make([]byte, 6)
	for i := range id {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		id[i] = charset[num.Int64()]
	}
	return string(id)
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
