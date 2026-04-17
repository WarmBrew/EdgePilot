package middleware

import (
	"fmt"
	"net/http"
	"time"

	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
)

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
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests"})
			c.Abort()
			return
		}

		c.Next()
	}
}
