package main

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"gorm.io/driver/postgres"
	_ "gorm.io/driver/postgres"
	"gorm.io/gorm"
	"math/rand/v2"
	"net/http"
)

var RedisClient *redis.Client
var DB *gorm.DB
var ctx = context.Background()

// URL model for GORM
type URL struct {
	ID  string `gorm:"primary_key"`
	URL string `gorm:"not null"`
}

func main() {
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

	// Set up the router
	router := mux.NewRouter()
	router.HandleFunc("/short/{id}", GetLink)
	router.HandleFunc("/create", ShortenURL)
	if http.ListenAndServe(":8080", router) != nil {
		return
	}
}

// GetLink handles the request to fetch the original URL based on the shortened ID
func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]

	data, err := RedisClient.Get(ctx, id).Result()
	if errors.Is(err, redis.Nil) {
		var url URL
		if err := DB.Where("id = ?", id).First(&url).Error; err != nil {
			w.WriteHeader(404)
			return
		}

		_ = RedisClient.Set(ctx, id, url.URL, 0).Err()

		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": url.URL})
	} else if err != nil {
		w.WriteHeader(500)
		return
	} else {
		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": data})
	}
}

// ShortenURL handles the request to shorten a given URL
func ShortenURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	url := r.URL.Query().Get("url")
	if url == "" {
		w.WriteHeader(400) // Bad Request if no URL is provided
		return
	}

	// Generate a new unique ID for the shortened URL
	newID := makeID(5)

	// Insert the new URL into the database using GORM
	urlRecord := URL{ID: newID, URL: url}
	if err := DB.Create(&urlRecord).Error; err != nil {
		w.WriteHeader(500) // Internal Server Error if something goes wrong
		return
	}

	// Return the shortened URL ID
	_, _ = w.Write([]byte(newID))
}

// makeID generates a random alphanumeric string of the specified length
func makeID(length int) string {
	var result string
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const charactersLength = len(characters)
	for i := 0; i < length; i++ {
		result += string(characters[rand.IntN(charactersLength)])
	}
	return result
}
