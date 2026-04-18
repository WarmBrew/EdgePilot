package routes

import (
	"github.com/edge-platform/server/internal/api/handlers"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/service"
	"github.com/edge-platform/server/internal/websocket"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupTerminalRoutes(r *gin.Engine, db *gorm.DB, redis *pkgRedis.RedisClient, gw *websocket.Gateway) {
	termSessionSvc := service.NewTerminalSessionService(db, redis, gw)
	termHandler := handlers.NewTerminalHandler(db, redis, gw, termSessionSvc)

	wsGroup := r.Group("/ws/terminal")
	{
		wsGroup.GET("/:session_id", termHandler.HandleTerminalWebSocket)
	}
}
