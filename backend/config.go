package main

import (
	"github.com/gocql/gocql"
	"github.com/redis/go-redis/v9"
)

// PrepareSession initializes the Cassandra session
func PrepareSession() (*gocql.Session, error) {
	cluster := gocql.NewCluster("localhost:9042")
	cluster.Keyspace = "shortenurl"
	cluster.Consistency = gocql.One
	cluster.PoolConfig.HostSelectionPolicy = gocql.TokenAwareHostPolicy(gocql.RoundRobinHostPolicy())
	return cluster.CreateSession()
}

// PrepareCache initializes the Redis client
func PrepareCache() *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
}
