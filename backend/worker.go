package main

import (
	"fmt"
	"sync"
	"time"
)

var (
	writeQueue = make(chan URL, 5000)
	readQueue  = make(chan ReadRequest, 5000)
)

func startWriteWorkers(baseWorkers int, maxWorkers int) {
	workerCount := baseWorkers
	for i := 0; i < workerCount; i++ {
		go writeWorker(i)
	}
	go func() {
		for {
			time.Sleep(1 * time.Second)
			queueLen := len(writeQueue)
			if queueLen > len(writeQueue)/2 && workerCount < maxWorkers {
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

func startReadWorkers(baseWorkers int, maxWorkers int) {
	workerCount := baseWorkers
	for i := 0; i < workerCount; i++ {
		go readWorker()
	}
	go func() {
		for {
			time.Sleep(1 * time.Second)
			queueLen := len(readQueue)
			if queueLen > len(readQueue)/2 && workerCount < maxWorkers {
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
		ch, loaded := getOrCreateChannel(id)
		if loaded {
			resultData := <-ch
			if resultData == "" {
				result["error"] = "URL not found"
			} else {
				result["originalUrl"] = resultData
			}
			req.ResultChan <- result
			continue
		}
		// Handle the request
		var url URL
		if dbErr := DB.Where("id = ?", id).First(&url).Error; dbErr != nil {
			result["error"] = "URL not found"
			notifyChannel(id, "")
		} else {
			result["originalUrl"] = url.URL
			_ = RedisClient.Set(ctx, id, url.URL, 5*time.Minute).Err()
			notifyChannel(id, url.URL)
		}

		req.ResultChan <- result
		closeChannel(id)
	}
}
