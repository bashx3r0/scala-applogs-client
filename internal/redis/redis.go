package redis

import (
	"context"

	"github.com/go-redis/redis/v8"
)

var ctx = context.Background()

// NewRedisClient initializes and returns a Redis client
func NewRedisClient(redisAddr string) *redis.Client {
	return redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
}

// PushLog pushes log data to Redis
func PushLog(rdb *redis.Client, key string, logData []byte) error {
	_, err := rdb.LPush(ctx, key, logData).Result()
	return err
}
