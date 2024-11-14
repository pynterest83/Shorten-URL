package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"net/http"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
	"github.com/google/uuid"
	"github.com/julienschmidt/httprouter"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var RedisClient *redis.Client
var DB *gorm.DB
var ctx = context.Background()
var WorkerTasks = haxmap.New[string, *WorkerTask]()

// URL model for GORM
type URL struct {
	ID  string `gorm:"primary_key"`
	URL string `gorm:"not null"`
}

var urlQueue = make(chan URL, 1000) // Channel hàng đợi với buffer 1000

func main() {
	// Port flag
	port := flag.Int("port", 8080, "Port to run the server on")
	flag.Parse()

	RedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		panic(err)
	}

	var err error
	DB, err = gorm.Open(postgres.Open("host=localhost user=shortenurl password=shortenurl dbname=shortenurl port=5432"))
	if err != nil {
		panic(err)
	}

	for i := 0; i < 3; i++ {
		go processQueue()
	}

	router := httprouter.New()
	router.GET("/short/:id", GetLink)
	router.POST("/create", ShortenURL)
	router.DELETE("/delete-all", deleteAllURLsHandler)
	router.DELETE("/delete-urls", DeleteURLs)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "http://localhost:80"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowCredentials: true,
		AllowedHeaders:   []string{"Content-Type"},
	})
	handler := corsHandler.Handler(router)

	// Create server with configured port
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", *port),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting server on port %d", *port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
}

type WorkerTask struct {
	waiters []chan<- []byte
	mutex   sync.Mutex
}

func MergeRequest(id string, channel chan<- []byte) {
	task, exist := WorkerTasks.GetOrSet(id, &WorkerTask{
		waiters: make([]chan<- []byte, 0),
	})
	task.mutex.Lock()
	task.waiters = append(task.waiters, channel)
	task.mutex.Unlock()
	if !exist {
		go TaskStart(id, task)
	}

}

func TaskStart(id string, task *WorkerTask) {
	var result []byte
	data, err := RedisClient.Get(ctx, id).Result()
	if err != nil {
		var url URL
		if err := DB.Where("id = ?", id).First(&url).Error; err != nil {
			result = nil
		} else {
			result = []byte(url.URL)
			// Set cache with a 24-hour expiration time
			_ = RedisClient.Set(ctx, id, url.URL, 24*time.Hour).Err()
		}
	} else {
		result = []byte(data)
	}
	time.Sleep(10 * time.Millisecond)
	WorkerTasks.Del(id)
	for _, waiter := range task.waiters {
		waiter <- result
	}
}

// GetLink handles the request to fetch the original URL based on the shortened ID
func GetLink(w http.ResponseWriter, _ *http.Request, ps httprouter.Params) {
	w.Header().Set("Content-Type", "application/json")
	id := ps.ByName("id")
	var waiter sync.WaitGroup
	waiter.Add(1)
	go func() {
		defer waiter.Done()
		returnValue := make(chan []byte)
		MergeRequest(id, returnValue)
		result := <-returnValue

		// Return JSON response
		response := map[string]string{
			"originalUrl": string(result),
		}
		json.NewEncoder(w).Encode(response)
	}()
	waiter.Wait()
}

// ShortenURL handles the request to shorten a given URL
func ShortenURL(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

// processQueue xử lý các yêu cầu từ hàng đợi và ghi vào cơ sở dữ liệu
func processQueue() {
	batchSize := 500
	ticker := time.NewTicker(1500 * time.Millisecond)
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
}

var totalInserts int

// batchInsert thực hiện batch insert vào cơ sở dữ liệu
func batchInsert(urls []URL) {
	if err := DB.Create(&urls).Error; err != nil {
		// Log lỗi nếu batch insert thất bại
		println("Batch insert failed:", err.Error())
	} else {
		totalInserts += len(urls)
		println("Total URLs inserted:", totalInserts)
	}
}

// makeID generates a random alphanumeric string of the specified length
func makeID() string {
	return uuid.New().String()
}

func deleteAllURLsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
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

// DeleteURLs handles the request to delete multiple URLs by their IDs
func DeleteURLs(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	var ids []string

	// Decode JSON array of IDs from the request body
	if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("Invalid request format"))
		return
	}

	if len(ids) == 0 {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("No IDs provided"))
		return
	}

	// Delete from Redis in a batch
	if err := RedisClient.Del(ctx, ids...).Err(); err != nil {
		log.Printf("Failed to delete from Redis: %v", err)
	}

	// Delete from PostgreSQL in a batch
	if err := DB.Unscoped().Where("id IN ?", ids).Delete(&URL{}).Error; err != nil {
		log.Printf("Failed to delete from database: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to delete records"))
		return
	}

	log.Printf("Records with IDs %v deleted successfully", ids)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("Records deleted successfully"))
}
