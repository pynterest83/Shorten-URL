package main

import (
	"fmt"
	"net/http"

	"github.com/rs/cors"
)

func main() {
	// Initialize Cassandra session
	session, err := PrepareSession()
	if err != nil {
		panic(err)
	}
	defer session.Close()

	// Initialize Redis cache
	cache := PrepareCache()
	defer cache.Close()

	// Set up URL shortener and application
	urlShortener := NewURLShortener(session, cache)
	app := NewApplication(urlShortener)

	// Create a new CORS handler
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},  // Allow your frontend origin
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"}, // Allow specific methods
		AllowCredentials: true,
	})

	// Start the server with CORS
	fmt.Println("Server started at :8080")
	if err := http.ListenAndServe(":8080", corsHandler.Handler(app.Router)); err != nil {
		fmt.Println("Failed to start server:", err)
	}
}
