package redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

var (
	ErrKeyNotFound = errors.New("redis: key not found")
	ErrLockBusy    = errors.New("redis: lock is busy")
)

const defaultTimeout = 3 * time.Second

func ctxWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); !ok {
		return context.WithTimeout(ctx, defaultTimeout)
	}
	return ctx, func() {}
}

// Set stores a key-value pair with an expiration time.
func (r *RedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	c, cancel := ctxWithTimeout(ctx)
	defer cancel()

	err := r.client.Set(c, key, value, expiration).Err()
	if err != nil {
		return fmt.Errorf("redis set key %q failed: %w", key, err)
	}
	return nil
}

// Get retrieves the value of a key as a string.
func (r *RedisClient) Get(ctx context.Context, key string) (string, error) {
	c, cancel := ctxWithTimeout(ctx)
	defer cancel()

	val, err := r.client.Get(c, key).Result()
	if err != nil {
		if errors.Is(err, goredis.Nil) {
			return "", fmt.Errorf("key %q: %w", key, ErrKeyNotFound)
		}
		return "", fmt.Errorf("redis get key %q failed: %w", key, err)
	}
	return val, nil
}

// Del removes a key from Redis.
func (r *RedisClient) Del(ctx context.Context, key string) error {
	c, cancel := ctxWithTimeout(ctx)
	defer cancel()

	err := r.client.Del(c, key).Err()
	if err != nil {
		return fmt.Errorf("redis del key %q failed: %w", key, err)
	}
	return nil
}

// Exists checks whether a key exists in Redis.
func (r *RedisClient) Exists(ctx context.Context, key string) (bool, error) {
	c, cancel := ctxWithTimeout(ctx)
	defer cancel()

	count, err := r.client.Exists(c, key).Result()
	if err != nil {
		return false, fmt.Errorf("redis exists key %q failed: %w", key, err)
	}
	return count > 0, nil
}

// TTL returns the remaining time-to-live of a key.
func (r *RedisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	c, cancel := ctxWithTimeout(ctx)
	defer cancel()

	ttl, err := r.client.TTL(c, key).Result()
	if err != nil {
		return 0, fmt.Errorf("redis ttl key %q failed: %w", key, err)
	}
	return ttl, nil
}

const (
	terminalSessionPrefix = "terminal:session:"
	terminalSessionTTL    = 7 * 24 * time.Hour
)

type TerminalSessionCacheEntry struct {
	UserID    string    `json:"user_id"`
	DeviceID  string    `json:"device_id"`
	PtyPath   string    `json:"pty_path"`
	CreatedAt time.Time `json:"created_at"`
}

func (r *RedisClient) CacheTerminalSession(ctx context.Context, sessionID string, entry TerminalSessionCacheEntry) error {
	data, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("marshal terminal session cache: %w", err)
	}

	key := terminalSessionPrefix + sessionID
	return r.Set(ctx, key, data, terminalSessionTTL)
}

func (r *RedisClient) GetTerminalSessionCache(ctx context.Context, sessionID string) (*TerminalSessionCacheEntry, error) {
	key := terminalSessionPrefix + sessionID
	val, err := r.Get(ctx, key)
	if err != nil {
		if errors.Is(err, ErrKeyNotFound) {
			return nil, nil
		}
		return nil, err
	}

	var entry TerminalSessionCacheEntry
	if err := json.Unmarshal([]byte(val), &entry); err != nil {
		return nil, fmt.Errorf("unmarshal terminal session cache: %w", err)
	}
	return &entry, nil
}

func (r *RedisClient) DeleteTerminalSessionCache(ctx context.Context, sessionID string) error {
	key := terminalSessionPrefix + sessionID
	return r.Del(ctx, key)
}
