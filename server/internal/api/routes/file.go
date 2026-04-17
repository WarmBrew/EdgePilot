package routes

import (
	"github.com/edge-platform/server/internal/api/handlers"
	"github.com/edge-platform/server/internal/api/middleware"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/websocket"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupFileRoutes(r *gin.Engine, db *gorm.DB, redis *pkgRedis.RedisClient, gw *websocket.Gateway) {
	fileHandler := handlers.NewFileHandler(db, redis, gw)

	api := r.Group("/api/v1")
	authGroup := api.Group("")
	authGroup.Use(middleware.JWTAuth(), middleware.RequireTenantIsolation())
	{
		authGroup.GET("/devices/:id/files", fileHandler.ListFiles)
		authGroup.GET("/devices/:id/files/:filepath", fileHandler.GetFileContent)
		authGroup.PUT("/devices/:id/files/:filepath", fileHandler.UpdateFile)
		authGroup.DELETE("/devices/:id/files/:filepath", fileHandler.DeleteFile)
		authGroup.POST("/devices/:id/files/upload", fileHandler.UploadFile)
		authGroup.GET("/devices/:id/files/:filepath/download", fileHandler.DownloadFile)

		authGroup.GET("/download/:token", fileHandler.HandleDownloadToken)
	}
}
