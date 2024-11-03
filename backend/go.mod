module ShortenURL

go 1.23.2

replace github.com/gocql/gocql => github.com/scylladb/gocql v1.14.4

require (
	github.com/gocql/gocql v1.7.0
	github.com/gorilla/mux v1.8.1
	github.com/redis/go-redis/v9 v9.7.0
)

require (
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/hailocab/go-hostpool v0.0.0-20160125115350-e80d13ce29ed // indirect
	github.com/scylladb/go-reflectx v1.0.1 // indirect
	github.com/scylladb/gocqlx v1.5.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
)
