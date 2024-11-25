package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
)

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
	router.HandleFunc("/delete-urls", DeleteURLs).Methods("DELETE")

	// Set up CORS
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "DELETE", "OPTIONS"},
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
