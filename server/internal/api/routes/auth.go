package routes

import (
	"time"

	"github.com/edge-platform/server/internal/api/handlers"
	"github.com/edge-platform/server/internal/api/middleware"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupAuthRoutes(r *gin.Engine, db *gorm.DB, redis *pkgRedis.RedisClient) {
	authHandler := handlers.NewAuthHandler(db, redis)
	authHandler.RegisterValidators()

	authGroup := r.Group("/api/v1/auth")
	{
		authGroup.POST("/register", middleware.RateLimiter(10, time.Hour), authHandler.Register)
		authGroup.POST("/login", middleware.RateLimiter(20, time.Hour), authHandler.Login)
		authGroup.POST("/refresh", authHandler.Refresh)
		authGroup.POST("/logout", authHandler.Logout)
	}
}
