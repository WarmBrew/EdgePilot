package redis

import (
	"context"
	"fmt"
	"time"
)

// Increment performs an atomic increment on a key and sets expiration
// if the key is newly created. This implements a sliding window rate limiter.
// Returns the current count after increment.
func (r *RedisClient) Increment(ctx context.Context, key string, expiration time.Duration) (int64, error) {
	c, cancel := ctxWithTimeout(ctx)
	defer cancel()

	// Use a Lua script for atomic INCR + conditional EXPIRE
	script := `
		local count = redis.call("INCR", KEYS[1])
		if count == 1 then
			redis.call("EXPIRE", KEYS[1], ARGV[1])
		end
		return count
	`

	expireSeconds := int64(expiration.Seconds())
	result, err := r.client.Eval(c, script, []string{key}, expireSeconds).Int64()
	if err != nil {
		return 0, fmt.Errorf("redis increment key %q failed: %w", key, err)
	}
	return result, nil
}
