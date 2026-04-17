package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/edge-platform/server/internal/config"

	goredis "github.com/redis/go-redis/v9"
)

// RedisClient wraps the go-redis client.
type RedisClient struct {
	client *goredis.Client
}

var defaultClient *RedisClient

// InitializeRedis creates and sets the global Redis client.
func InitializeRedis(cfg *config.Config) (*RedisClient, error) {
	redisCfg := cfg.Redis

	client := goredis.NewClient(&goredis.Options{
		Addr:         redisCfg.Addr(),
		Password:     redisCfg.Password,
		DB:           redisCfg.DB,
		DialTimeout:  5 * time.Second,
		ReadTimeout:  3 * time.Second,
		WriteTimeout: 3 * time.Second,
		PoolSize:     10,
		MinIdleConns: 5,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("redis ping failed: %w", err)
	}

	defaultClient = &RedisClient{client: client}
	return defaultClient, nil
}

// GetClient returns the global Redis client instance.
func GetClient() *RedisClient {
	if defaultClient == nil {
		panic("redis client not initialized, call InitializeRedis() first")
	}
	return defaultClient
}

// Close closes the Redis connection.
func (r *RedisClient) Close() error {
	if r.client != nil {
		return r.client.Close()
	}
	return nil
}

// Raw returns the underlying go-redis client for advanced usage.
func (r *RedisClient) Raw() *goredis.Client {
	return r.client
}
