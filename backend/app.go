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

var RedisClient *redis.Client  // Global Redis client instance
var DB *gorm.DB                // Global GORM database instance
var ctx = context.Background() // Context for Redis operations

// URL model for GORM, representing a shortened URL entry
type URL struct {
	ID         string      `gorm:"primary_key"` // Primary key for the URL
	URL        string      `gorm:"not null"`    // The original URL
	ResultChan chan string `gorm:"-"`           // A channel to send results, not persisted in the DB
}

var urlQueue = make(chan URL, 5000) // Buffered channel to hold URL entries for batch processing

func main() {
	port := flag.String("port", "8080", "Port for the server to listen on")
	flag.Parse()

	// Initialize Redis client
	RedisClient = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
		DB:   0,
	})

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		panic(err) // Exit if Redis is not available
	}

	// Initialize PostgreSQL connection using GORM
	var err error
	dsn := "host=localhost user=shortenurl password=shortenurl dbname=shortenurl port=5432"
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		PrepareStmt:            true, // Use prepared statements for performance
		SkipDefaultTransaction: true, // Disable default transactions for performance
	})
	if err != nil {
		panic(err)
	}

	// Start workers to process the URL queue
	startWorkers(3)

	// Set up HTTP router and define routes
	router := mux.NewRouter()
	router.HandleFunc("/short/{id}", GetLink).Methods("GET")                 // Retrieve original URL
	router.HandleFunc("/create", ShortenURL).Methods("POST")                 // Create shortened URL
	router.HandleFunc("/delete-all", deleteAllURLsHandler).Methods("DELETE") // Delete all URLs

	// Configure CORS for the server
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"}, // Allow specific origin
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowCredentials: true,
	})

	// Wrap router with CORS middleware
	handler := corsHandler.Handler(router)

	// Start the server
	fmt.Printf("Starting server on port %s...\n", *port)
	if http.ListenAndServe(":"+*port, handler) != nil {
		return
	}
}

// startWorkers initializes worker goroutines to handle batch database inserts
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

// ShortenURL handles requests to create a shortened URL
func ShortenURL(w http.ResponseWriter, r *http.Request) {
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

	// Send the response with the generated ID
	response := map[string]string{"id": newID}
	_ = json.NewEncoder(w).Encode(response)
}

// batchInsert performs batch inserts into the database
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

// GetLink handles requests to fetch the original URL using the shortened ID
func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	// Lua script to atomically retrieve the URL from Redis
	script := redis.NewScript(`
		local url = redis.call("GET", KEYS[1])
		if url then
			return url
		end
		return nil
	`)

	data, err := script.Run(ctx, RedisClient, []string{id}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"error": "Redis script execution failed: %v"}`, err)
		return
	}

	if data != nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": data.(string)})
		return
	}

	// Fallback to database lookup if URL is not in Redis
	var url URL
	if err := DB.Where("id = ?", id).First(&url).Error; err != nil {
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"error": "URL not found for ID: %s"}`, id)
		return
	}

	// Cache the URL in Redis
	if err := RedisClient.Set(ctx, id, url.URL, 5*time.Minute).Err(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = fmt.Fprintf(w, `{"error": "Failed to cache URL in Redis: %v"}`, err)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": url.URL})
}

// deleteAllURLsHandler handles requests to delete all URL records
func deleteAllURLsHandler(w http.ResponseWriter, _ *http.Request) {
	if err := deleteAllRecords(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("Failed to delete records"))
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("All records deleted successfully"))
}

// deleteAllRecords deletes all URL records from the database
func deleteAllRecords() error {
	if err := DB.Unscoped().Where("1 = 1").Delete(&URL{}).Error; err != nil {
		return err
	}
	fmt.Println("All records deleted from URL table")
	return nil
}
