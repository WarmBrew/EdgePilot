package routes

import (
	"github.com/edge-platform/server/internal/api/handlers"
	"github.com/edge-platform/server/internal/api/middleware"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func SetupGroupRoutes(r *gin.Engine, db *gorm.DB) {
	groupHandler := handlers.NewDeviceGroupHandler(db)

	api := r.Group("/api/v1")
	{
		authGroup := api.Group("")
		authGroup.Use(middleware.JWTAuth(), middleware.RequireTenantIsolation())
		{
			authGroup.GET("/groups", groupHandler.ListGroups)
			authGroup.POST("/groups", groupHandler.CreateGroup)
			authGroup.GET("/groups/:id", groupHandler.GetGroup)
			authGroup.PUT("/groups/:id", groupHandler.UpdateGroup)
			authGroup.DELETE("/groups/:id", groupHandler.DeleteGroup)
			authGroup.POST("/groups/:id/devices", groupHandler.AssignDevices)
		}
	}
}
