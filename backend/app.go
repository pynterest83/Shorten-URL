package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
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

var (
	RedisClient *redis.Client
	DB          *gorm.DB
	ctx         = context.Background()
	writeQueue  = make(chan URL, 5000)         // Queue for batch processing
	readQueue   = make(chan ReadRequest, 5000) // Queue for read requests
)

type URL struct {
	ID         string      `gorm:"primary_key"`
	URL        string      `gorm:"not null"`
	ResultChan chan string `gorm:"-"` // Used for communicating results
}

type ReadRequest struct {
	ID         string
	ResultChan chan map[string]string
}

func main() {
	// Parse command-line flags
	port := flag.String("port", "8080", "Port for the server to listen on")
	flag.Parse()

	// Initialize Redis client
	initRedis()

	// Initialize PostgreSQL connection
	initDB()

	// Start workers
	startWriteWorkers(3, 5)
	startReadWorkers(5, 10)

	// Set up HTTP router
	router := mux.NewRouter()
	router.HandleFunc("/short/{id}", GetLink).Methods("GET")
	router.HandleFunc("/create", ShortenURL).Methods("POST")
	router.HandleFunc("/delete-all", deleteAllURLsHandler).Methods("DELETE")
	router.HandleFunc("/delete-urls", DeleteURLs).Methods("DELETE")

	// Set up CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowCredentials: true,
	})
	handler := corsHandler.Handler(router)

	// Start the HTTP server
	fmt.Printf("Starting server on port %s...\n", *port)
	err := http.ListenAndServe(":"+*port, handler)
	if err != nil {
		return
	}
}

func initRedis() {
	RedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis: %v", err))
	}
}

func initDB() {
	var err error
	dsn := "host=localhost user=shortenurl password=shortenurl dbname=shortenurl port=5432"
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to PostgreSQL: %v", err))
	}

	// Configure connection pool
	sqlDB, _ := DB.DB()
	sqlDB.SetMaxOpenConns(50)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)
}

func startWriteWorkers(baseWorkers int, maxWorkers int) {
	workerCount := baseWorkers

	// Start initial workers
	for i := 0; i < workerCount; i++ {
		go writeWorker(i)
	}

	// Automatically scale workers based on queue length
	go func() {
		for {
			time.Sleep(1 * time.Second) // Check queue every second
			queueLen := len(writeQueue)
			if queueLen > len(writeQueue)/2 && workerCount < maxWorkers {
				//fmt.Printf("Scaling up write workers: %d -> %d\n", workerCount, workerCount+1)
				go writeWorker(workerCount)
				workerCount++
			}
		}
	}()
}

func writeWorker(workerID int) {
	batchSize := 200
	ticker := time.NewTicker(5 * time.Millisecond)
	defer ticker.Stop()

	var urls []URL
	var mutex sync.Mutex

	fmt.Printf("Write worker %d started\n", workerID)

	for {
		select {
		case url := <-writeQueue:
			mutex.Lock()
			urls = append(urls, url)
			if len(urls) >= batchSize {
				batchInsert(urls)
				urls = nil
			}
			mutex.Unlock()

		case <-ticker.C:
			mutex.Lock()
			if len(urls) > 0 {
				batchInsert(urls)
				urls = nil
			}
			mutex.Unlock()
		}
	}
}

func ShortenURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	url := r.FormValue("url")
	if url == "" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	resultChan := make(chan string)
	writeQueue <- URL{URL: url, ResultChan: resultChan}
	newID := <-resultChan

	response := map[string]string{"id": newID}
	_ = json.NewEncoder(w).Encode(response)
}

func batchInsert(urls []URL) {
	const maxRetries = 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		for i := range urls {
			urls[i].ID = makeID()
		}

		tx := DB.Begin()
		if err := tx.Create(&urls).Error; err != nil {
			tx.Rollback()
			fmt.Printf("Batch insert failed on attempt %d: %v\n", attempt, err)
			continue
		}

		if err := tx.Commit().Error; err != nil {
			fmt.Printf("Failed to commit transaction: %v\n", err)
		} else {
			for _, url := range urls {
				if url.ResultChan != nil {
					url.ResultChan <- url.ID
				}
			}
			return
		}
	}

	for _, url := range urls {
		if url.ResultChan != nil {
			url.ResultChan <- "error"
		}
	}
}

func startReadWorkers(baseWorkers int, maxWorkers int) {
	workerCount := baseWorkers

	// Tạo readWorker ban đầu
	for i := 0; i < workerCount; i++ {
		go readWorker()
	}

	// Tự động mở rộng readWorker khi tải tăng
	go func() {
		for {
			time.Sleep(1 * time.Second) // Kiểm tra hàng đợi mỗi giây
			queueLen := len(readQueue)
			if queueLen > len(readQueue)/2 && workerCount < maxWorkers {
				//fmt.Printf("Scaling up workers: %d -> %d\n", workerCount, workerCount+1)
				go readWorker()
				workerCount++
			}
		}
	}()
}

func readWorker() {
	for req := range readQueue {
		id := req.ID
		result := make(map[string]string)

		// Kiểm tra trạng thái xử lý ID trong sync.Map
		ch, loaded := getOrCreateChannel(id)
		if loaded {
			// Một readWorker khác đang xử lý ID này -> chờ tín hiệu
			resultData := <-ch
			if resultData == "" {
				result["error"] = "URL not found"
			} else {
				result["originalUrl"] = resultData
			}
			req.ResultChan <- result
			continue
		}

		// Worker này sẽ xử lý ID
		// fmt.Printf("Worker %d processing ID: %s\n", workerID, id)

		// Truy vấn cơ sở dữ liệu
		var url URL
		if dbErr := DB.Where("id = ?", id).First(&url).Error; dbErr != nil {
			result["error"] = "URL not found"
			notifyChannel(id, "") // Gửi kết quả lỗi qua channel
		} else {
			result["originalUrl"] = url.URL
			_ = RedisClient.Set(ctx, id, url.URL, 5*time.Minute).Err() // Cập nhật cache
			notifyChannel(id, url.URL)                                 // Gửi kết quả thành công qua channel
		}

		req.ResultChan <- result
		closeChannel(id) // Đóng channel sau khi xử lý xong
	}
}

func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	// Check Redis cache first
	data, err := RedisClient.Get(ctx, id).Result()
	if err == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": data})
		return
	}

	// Cache miss: Create a ResultChan from the pool
	resultChan := channelPool.Get().(chan map[string]string)
	defer func() {
		// Return the channel to the pool after use
		channelPool.Put(resultChan)
	}()

	// Add the request to the read queue
	readQueue <- ReadRequest{
		ID:         id,
		ResultChan: resultChan,
	}

	// Wait for the result from the readWorker
	select {
	case result := <-resultChan:
		// Receive the result from the readWorker
		if errorMsg, exists := result["error"]; exists {
			// Friendly message if the URL is not found
			if errorMsg == "URL not found" {
				_ = json.NewEncoder(w).Encode(map[string]string{"message": errorMsg})
			} else {
				http.Error(w, errorMsg, http.StatusInternalServerError)
			}
		} else {
			_ = json.NewEncoder(w).Encode(result)
		}
	case <-time.After(3*time.Second + 500*time.Millisecond): // Timeout to protect the HTTP handler
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
}

var idProcessing sync.Map // Map quản lý trạng thái xử lý từng ID
var channelPool = sync.Pool{
	New: func() interface{} {
		return make(chan map[string]string, 1) // Buffer 1 để tránh chặn
	},
}

func getOrCreateChannel(id string) (chan string, bool) {
	ch, loaded := idProcessing.LoadOrStore(id, make(chan string, 1)) // Channel có buffer 1
	return ch.(chan string), loaded
}

func notifyChannel(id string, data string) {
	if ch, ok := idProcessing.Load(id); ok {
		ch.(chan string) <- data // Gửi tín hiệu qua channel
	}
}

func closeChannel(id string) {
	if ch, ok := idProcessing.LoadAndDelete(id); ok {
		close(ch.(chan string))
	}
}

func makeID() string {
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	id := make([]byte, 6)
	for i := range id {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		id[i] = charset[num.Int64()]
	}
	return string(id)
}

func deleteAllURLsHandler(w http.ResponseWriter, r *http.Request) {
	// Call deleteAllRecords to delete all records in both the database and cache
	if err := deleteAllRecords(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to delete records"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("All records deleted successfully"))
}

func deleteAllRecords() error {
	// Delete all records from PostgreSQL
	if err := DB.Unscoped().Where("1 = 1").Delete(&URL{}).Error; err != nil {
		log.Printf("Failed to delete records from database: %v", err)
		return err
	}
	log.Println("All records deleted from PostgreSQL")

	// Delete all keys from Redis
	if err := RedisClient.FlushDB(ctx).Err(); err != nil {
		log.Printf("Failed to delete all keys from Redis: %v", err)
		return err
	}
	log.Println("All keys deleted from Redis")

	return nil
}

// DeleteURLs handles the request to delete multiple URLs by their IDs
func DeleteURLs(w http.ResponseWriter, r *http.Request) {
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
