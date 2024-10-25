package main

import (
	"context"
	"github.com/gocql/gocql"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"math/rand/v2"
	"net/http"
)

func Prepare() (*gocql.Session, error) {
	Database.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	Database.Keyspace = "shortenurl"
	return Database.CreateSession()
}

var Cache *redis.Client
var Database = gocql.NewCluster("localhost:9042")
var Session, _ = Prepare()

func main() {
	Cache = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	defer Session.Close()

	router := mux.NewRouter()
	router.HandleFunc("/short/{id}", GetLink)
	router.HandleFunc("/create", ShortenURL)
	if http.ListenAndServe(":3000", router) != nil {
		return
	}
}
func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	id := mux.Vars(r)["id"]
	var url string
	err := Session.Query("select url from urls where id = ? limit 1", id).WithContext(context.Background()).Consistency(gocql.One).Scan(&url)
	if err != nil {
		url, err = Cache.Get(context.Background(), id).Result()
		if err != nil {
			w.WriteHeader(404)
			return
		} else {
			_, err = w.Write([]byte(url))
			return
		}
	} else {
		_ = Cache.Set(context.Background(), id, url, 0)
		_, err = w.Write([]byte(url))
		return
	}
}

func ShortenURL(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	newID := makeID(5)
	err := Session.Query("select url from urls where id = ? limit 1", newID).WithContext(context.Background()).Consistency(gocql.One).Scan()
	if err != nil {
		err = Session.Query("insert into urls(id, url) values(?, ?)", newID, url).WithContext(context.Background()).Consistency(gocql.One).Exec()
		if err != nil {
			w.WriteHeader(500)
			return
		} else {
			_, err := w.Write([]byte(newID))
			if err != nil {
				w.WriteHeader(500)
				return
			}
			return
		}
	} else {
		w.WriteHeader(409)
	}
}

func makeID(length int) string {
	var result string
	const characters = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	const charactersLength = len(characters)
	for i := 0; i < length; i++ {
		result += string(characters[rand.IntN(charactersLength)])
	}
	return result
}
