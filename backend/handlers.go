package main

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

func ShortenURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	url := strings.TrimSpace(r.URL.Query().Get("url"))
	if url == "" {
		http.Error(w, "URL parameter is required", http.StatusBadRequest)
		return
	}
	resultChan := make(chan string)
	writeQueue <- URL{URL: url, ResultChan: resultChan}
	newID := <-resultChan
	if newID == "error" {
		http.Error(w, "Failed to process URL", http.StatusInternalServerError)
		return
	}
	response := map[string]string{"id": newID}
	_ = json.NewEncoder(w).Encode(response)
}

func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	data, err := RedisClient.Get(ctx, id).Result()
	if err == nil {
		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": data})
		return
	}
	resultChan := channelPool.Get().(chan map[string]string)
	defer func() {
		channelPool.Put(resultChan)
	}()
	readQueue <- ReadRequest{
		ID:         id,
		ResultChan: resultChan,
	}
	// Wait for the result from the readWorker
	select {
	case result := <-resultChan:
		if errorMsg, exists := result["error"]; exists {
			if errorMsg == "URL not found" {
				_ = json.NewEncoder(w).Encode(map[string]string{"message": errorMsg})
			} else {
				http.Error(w, errorMsg, http.StatusInternalServerError)
			}
		} else {
			_ = json.NewEncoder(w).Encode(result)
		}
	case <-time.After(3*time.Second + 500*time.Millisecond):
		http.Error(w, "Request timed out", http.StatusGatewayTimeout)
	}
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
