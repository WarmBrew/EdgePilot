package routes

import (
	"github.com/edge-platform/server/internal/api/handlers"
	"github.com/edge-platform/server/internal/api/middleware"
	"github.com/edge-platform/server/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// SetupAuditRoutes registers audit log query and export endpoints.
func SetupAuditRoutes(r *gin.Engine, db *gorm.DB, auditSvc *service.AuditService) {
	handler := handlers.NewAuditHandler(db, auditSvc)

	api := r.Group("/api/v1")
	authGroup := api.Group("")
	authGroup.Use(middleware.JWTAuth(), middleware.RequireTenantIsolation())
	{
		authGroup.GET("/audit/logs", handler.ListAuditLogs)
		authGroup.GET("/audit/logs/:id", handler.GetAuditLogDetail)
		authGroup.POST("/audit/logs/export", handler.ExportAuditLogs)
		authGroup.GET("/audit/exports/:export_id", handler.DownloadExport)
	}
}
