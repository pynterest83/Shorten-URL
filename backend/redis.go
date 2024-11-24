package main

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
)

var (
	RedisClient *redis.ClusterClient
	ctx         = context.Background()
)

func initRedis() {
	RedisClient = redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{
			"127.0.0.1:7000",
			"127.0.0.1:7001",
			"127.0.0.1:7002",
		},
	})

	if _, err := RedisClient.Ping(ctx).Result(); err != nil {
		panic(fmt.Sprintf("Failed to connect to Redis Cluster: %v", err))
	}
}
