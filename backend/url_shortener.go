package main

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gocql/gocql"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"
	"github.com/scylladb/gocqlx"
	"github.com/scylladb/gocqlx/qb"
)

type URLShortener struct {
	db    *gocql.Session
	cache *redis.Client
}

// NewURLShortener initializes a new URLShortener
func NewURLShortener(db *gocql.Session, cache *redis.Client) *URLShortener {
	return &URLShortener{
		db:    db,
		cache: cache,
	}
}

// GetLink retrieves the original URL from a shortened ID and returns it as JSON
func (s *URLShortener) GetLink(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var url string
	url, err := s.cache.Get(context.Background(), id).Result()
	if err == redis.Nil {
		stmt, names := qb.Select("urls").Columns("url").Where(qb.Eq("id")).Limit(1).ToCql()
		q := gocqlx.Query(s.db.Query(stmt), names).BindMap(qb.M{"id": id})
		err = q.GetRelease(&url)

		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		s.cache.Set(context.Background(), id, url, 0)
	}

	// Return the URL as a JSON response
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"originalUrl": url})
}

// ShortenURL creates a new shortened URL ID
func (s *URLShortener) ShortenURL(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	var newID string
	for {
		newID = makeID(5)
		stmt, names := qb.Select("urls").Columns("id").Where(qb.Eq("id")).Limit(1).ToCql()
		q := gocqlx.Query(s.db.Query(stmt), names).BindMap(qb.M{"id": newID})

		var existingID string
		err := q.GetRelease(&existingID)

		if err == gocql.ErrNotFound {
			break
		} else if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	stmt, names := qb.Insert("urls").Columns("id", "url").ToCql()
	q := gocqlx.Query(s.db.Query(stmt), names).BindMap(qb.M{"id": newID, "url": url})

	if err := q.ExecRelease(); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if _, err := w.Write([]byte(newID)); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	}
}
