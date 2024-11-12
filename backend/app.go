package main

import (
	"context"
	"encoding/json"
	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgxpool"
	"math/rand/v2"
	"net/http"
)

var Cache *memcache.Client
var Database *pgxpool.Pool

func main() {
	Cache = memcache.New("localhost:11211")
	var err error
	Database, err = pgxpool.New(context.Background(), "postgresql://shortenurl:shortenurl@localhost:5432/shortenurl?pool_max_conns=95")
	if err != nil {
		panic(err)
	}

	router := mux.NewRouter()
	router.HandleFunc("/short/{id}", GetLink)
	router.HandleFunc("/create", ShortenURL)
	if http.ListenAndServe(":8080", router) != nil {
		return
	}
}
func GetLink(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	id := mux.Vars(r)["id"]
	var url []byte
	data, err := Cache.Get(id)
	if err != nil {
		err = Database.QueryRow(context.Background(), "select url from urls where id = $1", id).Scan(&url)
		if err != nil {
			w.WriteHeader(404)
			return
		} else {
			_ = Cache.Set(&memcache.Item{Key: id, Value: url})
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": string(url)})
		}
	} else {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"originalUrl": string(data.Value)})
	}
}

func ShortenURL(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	newID := makeID(5)
	tag, err := Database.Exec(context.Background(), "insert into urls(id, url) values($1, $2) on conflict do nothing", newID, url)
	if err != nil || tag.RowsAffected() == 0 {
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(newID))
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
