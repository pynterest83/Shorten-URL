package main

import (
	"context"
	"github.com/alphadose/haxmap"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/rs/cors"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

var RedisClient *redis.Client
var DB *gorm.DB
var ctx = context.Background()
var WorkerTasks = haxmap.New[string, *WorkerTask]()

// URL model for GORM
type URL struct {
	ID  string `gorm:"primary_key"`
	URL string `gorm:"not null"`
}

var urlQueue = make(chan URL, 1000) // Channel hàng đợi với buffer 1000

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

	go processQueue()

	router := mux.NewRouter()
	router.HandleFunc("/short/{id}", GetLink)
	router.HandleFunc("/create", ShortenURL)

	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowCredentials: true,
	})

	handler := corsHandler.Handler(router)
	if http.ListenAndServe(":8080", handler) != nil {
		return
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
	if !exist {
		go TaskStart(id, task)
	}
	task.mutex.Lock()
	task.waiters = append(task.waiters, channel)
	task.mutex.Unlock()
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
			_ = RedisClient.Set(ctx, id, url.URL, 0).Err()
		}
	} else {
		result = []byte(data)
	}
	time.Sleep(45 * time.Millisecond)
	WorkerTasks.Del(id)
	for _, waiter := range task.waiters {
		waiter <- result
	}
}

// GetLink handles the request to fetch the original URL based on the shortened ID
func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := mux.Vars(r)["id"]
	var waiter sync.WaitGroup
	waiter.Add(1)
	go func() {
		defer waiter.Done()
		returnValue := make(chan []byte)
		MergeRequest(id, returnValue)
		result := <-returnValue
		_, _ = w.Write(result)
	}()
	waiter.Wait()
}

// ShortenURL handles the request to shorten a given URL
func ShortenURL(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")

	url := r.FormValue("url")
	if url == "" {
		w.WriteHeader(400) // Bad Request if no URL is provided
		return
	}

	// Generate a new unique ID for the shortened URL
	newID := makeID(10)

	// Đưa bản ghi vào hàng đợi thay vì ghi trực tiếp vào cơ sở dữ liệu
	urlRecord := URL{ID: newID, URL: url}
	urlQueue <- urlRecord // Đưa vào hàng đợi

	// Trả về ID của URL rút gọn ngay lập tức
	_, _ = w.Write([]byte(newID))
}

// processQueue xử lý các yêu cầu từ hàng đợi và ghi vào cơ sở dữ liệu
func processQueue() {
	batchSize := 500
	ticker := time.NewTicker(1500 * time.Millisecond)
	defer ticker.Stop()

	var urls []URL

	for {
		select {
		case url := <-urlQueue:
			urls = append(urls, url)

			// Nếu đạt đến kích thước batch, ghi tất cả vào DB
			if len(urls) >= batchSize {
				batchInsert(urls)
				urls = urls[:0] // Reset lại slice sau khi ghi
			}
		case <-ticker.C:
			// Ghi bất cứ bản ghi nào còn lại vào cuối mỗi giây
			if len(urls) > 0 {
				batchInsert(urls)
				urls = urls[:0]
			}
		}
	}
}

var totalInserts int

// batchInsert thực hiện batch insert vào cơ sở dữ liệu
func batchInsert(urls []URL) {
	if err := DB.Create(&urls).Error; err != nil {
		// Log lỗi nếu batch insert thất bại
		println("Batch insert failed:", err.Error())
	} else {
		totalInserts += len(urls)
		println("Total URLs inserted:", totalInserts)
	}
}

// makeID generates a random alphanumeric string of the specified length
func makeID(length int) string {
	var result string
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	for i := 0; i < length; i++ {
		result += string(characters[rand.Intn(len(characters))])
	}
	return result
}
