package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/signal"

	"net/http"
	"sync"
	"time"

	"github.com/alphadose/haxmap"
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
	ID         string      `gorm:"primary_key"`
	URL        string      `gorm:"not null"`
	ResultChan chan string `gorm:"-"`
}

var urlQueue = make(chan URL, 5000) // Channel hàng đợi với buffer 5000

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
	DB, err = gorm.Open(postgres.Open("host=localhost user=shortenurl password=shortenurl dbname=shortenurl port=5432"), &gorm.Config{
		PrepareStmt:            true,
		SkipDefaultTransaction: true,
	})
	if err != nil {
		panic(err)
	}

	startWorkers(3)

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

// startWorkers initializes worker goroutines to handle batch database inserts
func startWorkers(numWorkers int) {
	for i := 0; i < numWorkers; i++ {
		go func(workerID int) {
			batchSize := 200                          // Maximum number of URLs per batch
			ticker := time.NewTicker(1 * time.Second) // Time-based batch processing
			defer ticker.Stop()

			var urls []URL
			var mutex sync.Mutex

			for {
				select {
				case url := <-urlQueue:
					// Add URL to batch
					mutex.Lock()
					urls = append(urls, url)
					mutex.Unlock()

					// Insert batch into DB if it reaches the batch size
					if len(urls) >= batchSize {
						mutex.Lock()
						batchInsert(urls)
						urls = nil // Reset the slice after insertion
						mutex.Unlock()
					}
				case <-ticker.C:
					// Insert any remaining URLs at the end of each second
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
	w.Header().Set("Content-Type", "application/json")

	url := r.FormValue("url") // Extract the original URL from the form
	if url == "" {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// Create a channel to receive the result
	resultChan := make(chan string)

	// Add the URL to the processing queue
	urlQueue <- URL{URL: url, ResultChan: resultChan}

	// Wait for the worker to generate the shortened ID
	newID := <-resultChan

	_, _ = w.Write([]byte(newID))
}

func batchInsert(urls []URL) {
	const maxRetries = 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Generate new IDs for the URLs during each retry
		for i := range urls {
			urls[i].ID = makeID()
		}

		// Start a transaction
		tx := DB.Begin()

		// Perform batch insert
		if err := tx.Create(&urls).Error; err != nil {
			tx.Rollback() // Rollback transaction on error
			fmt.Printf("Batch insert failed on attempt %d: %v\n", attempt, err)
			continue // Retry on error
		} else {
			// Commit transaction if successful
			if err := tx.Commit().Error; err != nil {
				fmt.Printf("Transaction commit failed on attempt %d: %v\n", attempt, err)
			} else {
				// Notify each URL's result channel
				for _, url := range urls {
					if url.ResultChan != nil {
						url.ResultChan <- url.ID
					}
				}
				return
			}
		}
	}

	// Handle failed retries
	for _, url := range urls {
		if url.ResultChan != nil {
			url.ResultChan <- "error"
		}
	}
}

var charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// makeID generates a random alphanumeric string of length 6
func makeID() string {
	id := make([]byte, 6)
	for i := range id {
		num, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		id[i] = charset[num.Int64()]
	}
	return string(id)
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
