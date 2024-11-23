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

var (
	RedisClient *redis.Client
	DB          *gorm.DB
	ctx         = context.Background()
	urlQueue    = make(chan URL, 5000) // Queue for batch processing
)

type URL struct {
	ID         string      `gorm:"primary_key"`
	URL        string      `gorm:"not null"`
	ResultChan chan string `gorm:"-"` // Used for communicating results
}

func main() {
	// Parse command-line flags
	port := flag.String("port", "8080", "Port for the server to listen on")
	flag.Parse()

	// Initialize Redis client
	initRedis()

	// Initialize PostgreSQL connection
	initDB()

	// Start workers for batch processing
	startWorkers(3)

	// Set up HTTP router
	router := mux.NewRouter()
	router.HandleFunc("/short/{id}", GetLink).Methods("GET")
	router.HandleFunc("/create", ShortenURL).Methods("POST")
	router.HandleFunc("/delete-all", deleteAllURLsHandler).Methods("DELETE")

	// Set up CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowCredentials: true,
	})
	handler := corsHandler.Handler(router)

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
		}(i)
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
	urlQueue <- URL{URL: url, ResultChan: resultChan}
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

func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	data, err := RedisClient.Get(ctx, id).Result()
	if err == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": data})
		return
	}

	if !errors.Is(err, redis.Nil) {
		http.Error(w, "Redis error", http.StatusInternalServerError)
		return
	}

	var url URL
	if err := DB.Where("id = ?", id).First(&url).Error; err != nil {
		http.Error(w, "URL not found", http.StatusNotFound)
		return
	}

	if err := RedisClient.Set(ctx, id, url.URL, 5*time.Minute).Err(); err != nil {
		http.Error(w, "Failed to cache URL", http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": url.URL})
}

func deleteAllURLsHandler(w http.ResponseWriter, _ *http.Request) {
	if err := DB.Unscoped().Where("1 = 1").Delete(&URL{}).Error; err != nil {
		http.Error(w, "Failed to delete records", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
	_, err := w.Write([]byte("All records deleted successfully"))
	if err != nil {
		return
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
