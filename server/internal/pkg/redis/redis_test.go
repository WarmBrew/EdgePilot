package redis

import (
	"context"
	"testing"
	"time"

	"github.com/edge-platform/server/internal/config"

	goredis "github.com/redis/go-redis/v9"
)

// testClient returns a RedisClient connected to localhost:6379 (DB 15 for test isolation).
// Tests are skipped if Redis is not available.
func testClient(t *testing.T) *RedisClient {
	t.Helper()

	// Try to connect to Redis
	client := goredis.NewClient(&goredis.Options{
		Addr: "localhost:6379",
		DB:   15, // Use DB 15 for test isolation
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available, skipping test")
	}

	return &RedisClient{client: client}
}

func TestInitializeRedis(t *testing.T) {
	// Build config manually
	cfg := &config.Config{
		Redis: config.RedisConfig{
			Host: "localhost",
			Port: 6379,
			DB:   15,
		},
	}

	r, err := InitializeRedis(cfg)
	if err != nil {
		// Redis may not be available; this is acceptable
		t.Logf("InitializeRedis returned error (Redis may not be running): %v", err)
		return
	}

	if r == nil {
		t.Fatal("expected non-nil RedisClient")
	}

	if r.client == nil {
		t.Fatal("expected non-nil underlying client")
	}

	t.Cleanup(func() {
		r.Close()
	})
}

func TestSetAndGet(t *testing.T) {
	rc := testClient(t)
	defer rc.Close()

	ctx := context.Background()

	// Clean up test key
	defer rc.Del(ctx, "test:key")

	// Set
	err := rc.Set(ctx, "test:key", "hello", 5*time.Minute)
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	// Get
	val, err := rc.Get(ctx, "test:key")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "hello" {
		t.Fatalf("expected 'hello', got %q", val)
	}
}

func TestGetNotFound(t *testing.T) {
	rc := testClient(t)
	defer rc.Close()

	ctx := context.Background()

	_, err := rc.Get(ctx, "nonexistent:key:12345")
	if err == nil {
		t.Fatal("expected error for nonexistent key")
	}
	if err.Error() == "" {
		t.Fatal("expected non-empty error message")
	}
}

func TestDel(t *testing.T) {
	rc := testClient(t)
	defer rc.Close()

	ctx := context.Background()

	// Set then delete
	rc.Set(ctx, "test:del:key", "value", 5*time.Minute)

	err := rc.Del(ctx, "test:del:key")
	if err != nil {
		t.Fatalf("Del failed: %v", err)
	}

	// Verify deletion
	exists, err := rc.Exists(ctx, "test:del:key")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected key to be deleted")
	}
}

func TestExists(t *testing.T) {
	rc := testClient(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test:exists:key"

	// Clean up
	defer rc.Del(ctx, key)

	// Should not exist
	exists, err := rc.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Fatal("expected key to not exist")
	}

	// Set and check again
	rc.Set(ctx, key, "value", 5*time.Minute)

	exists, err = rc.Exists(ctx, key)
	if err != nil {
		t.Fatalf("Exists after Set failed: %v", err)
	}
	if !exists {
		t.Fatal("expected key to exist after Set")
	}
}

func TestTTL(t *testing.T) {
	rc := testClient(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test:ttl:key"

	defer rc.Del(ctx, key)

	// Set with 10-second TTL
	rc.Set(ctx, key, "value", 10*time.Second)

	ttl, err := rc.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL failed: %v", err)
	}
	if ttl <= 0 || ttl > 10*time.Second {
		t.Fatalf("expected TTL between 0 and 10s, got %v", ttl)
	}
}

func TestAcquireAndReleaseLock(t *testing.T) {
	rc := testClient(t)
	defer rc.Close()

	ctx := context.Background()
	lockKey := "test:lock:key"

	defer rc.Del(ctx, lockKey)

	// Acquire lock
	token, acquired, err := rc.AcquireLock(ctx, lockKey, 5*time.Second)
	if err != nil {
		t.Fatalf("AcquireLock failed: %v", err)
	}
	if !acquired {
		t.Fatal("expected to acquire lock")
	}
	if token == "" {
		t.Fatal("expected non-empty lock token")
	}

	// Try to acquire again (should fail)
	_, acquired2, err := rc.AcquireLock(ctx, lockKey, 5*time.Second)
	if err != nil {
		t.Fatalf("second AcquireLock failed: %v", err)
	}
	if acquired2 {
		t.Fatal("expected second acquire to fail")
	}

	// Release lock with correct token
	err = rc.ReleaseLock(ctx, lockKey, token)
	if err != nil {
		t.Fatalf("ReleaseLock failed: %v", err)
	}

	// Try to release again (should fail - token mismatch)
	err = rc.ReleaseLock(ctx, lockKey, token)
	if err == nil {
		t.Fatal("expected error releasing already-released lock")
	}
}

func TestIncrement(t *testing.T) {
	rc := testClient(t)
	defer rc.Close()

	ctx := context.Background()
	key := "test:ratelimit:key"

	defer rc.Del(ctx, key)

	// First increment
	count, err := rc.Increment(ctx, key, 10*time.Second)
	if err != nil {
		t.Fatalf("Increment failed: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected count 1, got %d", count)
	}

	// Second increment
	count, err = rc.Increment(ctx, key, 10*time.Second)
	if err != nil {
		t.Fatalf("second Increment failed: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected count 2, got %d", count)
	}

	// Verify TTL was set
	ttl, err := rc.TTL(ctx, key)
	if err != nil {
		t.Fatalf("TTL check failed: %v", err)
	}
	if ttl <= 0 {
		t.Fatal("expected TTL to be set on rate limit key")
	}
}

func TestContextTimeout(t *testing.T) {
	rc := testClient(t)
	defer rc.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	// Wait for context to expire
	time.Sleep(10 * time.Millisecond)

	_, err := rc.Get(ctx, "any:key")
	if err == nil {
		t.Fatal("expected error with expired context")
	}
}
