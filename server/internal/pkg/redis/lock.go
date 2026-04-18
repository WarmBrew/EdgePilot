package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// AcquireLock attempts to acquire a distributed lock using SET NX EX.
// Returns the lock token and true if successfully acquired.
// The token must be passed to ReleaseLock to safely release the lock.
func (r *RedisClient) AcquireLock(ctx context.Context, key string, expiration time.Duration) (token string, acquired bool, err error) {
	c, cancel := ctxWithTimeout(ctx)
	defer cancel()

	// Generate a unique lock token so only the holder can release it
	token = uuid.New().String()

	ok, err := r.client.SetNX(c, key, token, expiration).Result()
	if err != nil {
		return "", false, fmt.Errorf("redis acquire lock %q failed: %w", key, err)
	}
	return token, ok, nil
}

// ReleaseLock removes the lock key, but only if the token matches
// (prevents releasing a lock that has already expired and been re-acquired).
func (r *RedisClient) ReleaseLock(ctx context.Context, key string, token string) error {
	c, cancel := ctxWithTimeout(ctx)
	defer cancel()

	// Use a Lua script for atomic comparison and deletion
	script := `
		if redis.call("get", KEYS[1]) == ARGV[1] then
			return redis.call("del", KEYS[1])
		else
			return 0
		end
	`

	result, err := r.client.Eval(c, script, []string{key}, token).Int()
	if err != nil {
		return fmt.Errorf("redis release lock %q failed: %w", key, err)
	}
	if result == 0 {
		return fmt.Errorf("redis release lock %q: lock token mismatch or already expired", key)
	}
	return nil
}
