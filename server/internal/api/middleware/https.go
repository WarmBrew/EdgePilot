package middleware

import (
	"os"

	"github.com/gin-gonic/gin"
)

func HTTPSEnforcement() gin.HandlerFunc {
	return func(c *gin.Context) {
		if os.Getenv("APP_ENV") == "production" && c.Request.Header.Get("X-Forwarded-Proto") != "https" {
			c.Redirect(301, "https://"+c.Request.Host+c.Request.URL.String())
			return
		}
		c.Next()
	}
}
