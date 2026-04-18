package middleware

import (
	"fmt"
	"net/http"
	"time"

	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
)

// Config holds rate limiting configuration per endpoint type.
type Config struct {
	DefaultMaxRequests int64
	DefaultWindow      time.Duration
	EndpointLimits     map[string]EndpointLimit
}

// EndpointLimit defines rate limit for a specific endpoint pattern.
type EndpointLimit struct {
	MaxRequests int64
	Window      time.Duration
}

// DefaultConfig returns a sensible default rate limit configuration.
func DefaultConfig() *Config {
	return &Config{
		DefaultMaxRequests: 200,
		DefaultWindow:      time.Minute,
		EndpointLimits: map[string]EndpointLimit{
			"/login":    {MaxRequests: 10, Window: time.Minute},
			"/register": {MaxRequests: 5, Window: time.Minute},
			"/refresh":  {MaxRequests: 20, Window: time.Minute},
			"/api/v1/":  {MaxRequests: 500, Window: time.Minute},
		},
	}
}

// RateLimiter creates a generic rate limiting middleware using Redis.
func RateLimiter(maxRequests int64, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		client := pkgRedis.GetClient()

		key := fmt.Sprintf("ratelimit:%s:%s", c.Request.URL.Path, c.ClientIP())

		count, err := client.Increment(c, key, window)
		if err != nil {
			c.Next()
			return
		}

		if count > maxRequests {
			c.Header("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "TOO_MANY_REQUESTS",
				"message":     "请求频率过高，请稍后再试",
				"retry_after": window.Seconds(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RateLimiterWithConfig creates a rate limiting middleware with per-endpoint configuration.
func RateLimiterWithConfig(cfg *Config) gin.HandlerFunc {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	return func(c *gin.Context) {
		client := pkgRedis.GetClient()
		path := c.Request.URL.Path

		maxRequests := cfg.DefaultMaxRequests
		window := cfg.DefaultWindow

		for pattern, limit := range cfg.EndpointLimits {
			if len(path) >= len(pattern) && path[:len(pattern)] == pattern {
				maxRequests = limit.MaxRequests
				window = limit.Window
				break
			}
		}

		key := fmt.Sprintf("ratelimit:%s:%s", path, c.ClientIP())

		count, err := client.Increment(c, key, window)
		if err != nil {
			c.Next()
			return
		}

		if count > int64(maxRequests) {
			c.Header("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "TOO_MANY_REQUESTS",
				"message":     "请求频率过高，请稍后再试",
				"retry_after": window.Seconds(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// LoginRateLimiter creates a stricter rate limiter for login endpoints.
func LoginRateLimiter(maxAttempts int, window time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		client := pkgRedis.GetClient()

		key := fmt.Sprintf("ratelimit:login:%s", c.ClientIP())

		count, err := client.Increment(c, key, window)
		if err != nil {
			c.Next()
			return
		}

		if count > int64(maxAttempts) {
			c.Header("Retry-After", fmt.Sprintf("%.0f", window.Seconds()))
			c.JSON(http.StatusTooManyRequests, gin.H{
				"error":       "LOGIN_RATE_LIMITED",
				"message":     "登录尝试次数过多，请稍后再试",
				"retry_after": window.Seconds(),
			})
			c.Abort()
			return
		}

		c.Next()
	}
}
