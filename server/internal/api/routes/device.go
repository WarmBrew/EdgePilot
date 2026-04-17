package routes

import (
	"github.com/edge-platform/server/internal/api/handlers"
	"github.com/edge-platform/server/internal/api/middleware"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupDeviceRoutes(r *gin.Engine, db *gorm.DB, redis *pkgRedis.RedisClient) {
	deviceHandler := handlers.NewDeviceHandler(db, redis)
	heartbeatHandler := handlers.NewHeartbeatHandler(db, redis)

	api := r.Group("/api/v1")
	{
		// Public endpoints (agent registration/verification)
		api.POST("/devices/register", deviceHandler.RegisterDevice)
		api.POST("/devices/verify", deviceHandler.VerifyDevice)
		api.GET("/devices/ws", deviceHandler.AgentWebSocketAuth)

		// Agent heartbeat endpoint (agent token auth)
		api.POST("/devices/:id/heartbeat", heartbeatHandler.HandleHeartbeat)
		api.GET("/devices/:id/metrics", heartbeatHandler.GetDeviceMetrics)

		// Authenticated endpoints (tenant-scoped)
		authGroup := api.Group("")
		authGroup.Use(middleware.JWTAuth(), middleware.RequireTenantIsolation())
		{
			authGroup.GET("/devices", deviceHandler.ListDevices)
			authGroup.GET("/devices/:id", deviceHandler.GetDevice)
			authGroup.PUT("/devices/:id", deviceHandler.UpdateDevice)
			authGroup.DELETE("/devices/:id", deviceHandler.DeleteDevice)
			authGroup.POST("/devices/batch", deviceHandler.BatchDevices)
		}
	}
}
