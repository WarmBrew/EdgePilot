package routes

import (
	"github.com/edge-platform/server/internal/api/handlers"
	"github.com/edge-platform/server/internal/api/middleware"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/service"
	"github.com/edge-platform/server/internal/websocket"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupTerminalRoutes(r *gin.Engine, db *gorm.DB, redis *pkgRedis.RedisClient, gw *websocket.Gateway) {
	termSessionSvc := service.NewTerminalSessionService(db, redis, gw)
	termHandler := handlers.NewTerminalHandler(db, redis, gw, termSessionSvc)

	// HTTP API endpoints
	api := r.Group("/api/v1")
	{
		authGroup := api.Group("")
		authGroup.Use(middleware.JWTAuth(), middleware.RequireTenantIsolation())
		{
			authGroup.POST("/devices/:id/terminal", termHandler.CreateTerminalSession)
			authGroup.GET("/terminal/sessions", termHandler.ListTerminalSessions)
			authGroup.POST("/terminal/sessions/:id/close", termHandler.CloseTerminalSession)
		}
	}

	// WebSocket endpoint
	wsGroup := r.Group("/ws/terminal")
	{
		wsGroup.GET("/:session_id", termHandler.HandleTerminalWebSocket)
	}
}
