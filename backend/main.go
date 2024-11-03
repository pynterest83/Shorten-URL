package main

import (
	"fmt"
	"net/http"
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

	// Start the server
	fmt.Println("Server started at :3000")
	if err := http.ListenAndServe(":3000", app.Router); err != nil {
		fmt.Println("Failed to start server:", err)
	}
}
