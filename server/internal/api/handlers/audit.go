package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/edge-platform/server/internal/api/middleware"
	"github.com/edge-platform/server/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// AuditHandler handles audit log query and export endpoints.
type AuditHandler struct {
	querySvc *service.AuditQueryService
}

// NewAuditHandler creates a new audit handler.
func NewAuditHandler(db *gorm.DB, auditSvc *service.AuditService) *AuditHandler {
	return &AuditHandler{
		querySvc: service.NewAuditQueryService(db, auditSvc),
	}
}

// ListAuditLogs handles GET /api/v1/audit/logs.
func (h *AuditHandler) ListAuditLogs(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	filter := h.parseFilter(c)

	resp, err := h.querySvc.ListLogs(c.Request.Context(), tenantID, filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list audit logs"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetAuditLogDetail handles GET /api/v1/audit/logs/:id.
func (h *AuditHandler) GetAuditLogDetail(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	logID := c.Param("id")
	if logID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "log ID is required"})
		return
	}

	log, err := h.querySvc.GetLogDetail(c.Request.Context(), tenantID, logID)
	if err != nil {
		if err.Error() == "audit log not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "audit log not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get audit log"})
		return
	}

	c.JSON(http.StatusOK, log)
}

// ExportAuditLogs handles POST /api/v1/audit/logs/export.
func (h *AuditHandler) ExportAuditLogs(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	userID, _ := middleware.GetUserID(c)

	var req service.ExportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Format != "csv" && req.Format != "json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "format must be csv or json"})
		return
	}

	result, err := h.querySvc.ExportLogs(c.Request.Context(), tenantID, userID, req.Filters, req.Format)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to export audit logs"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DownloadExport handles GET /api/v1/audit/exports/:export_id.
func (h *AuditHandler) DownloadExport(c *gin.Context) {
	exportID := c.Param("export_id")
	if exportID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "export ID is required"})
		return
	}

	filePath, contentType, err := h.querySvc.DownloadExport(exportID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "export not found or expired"})
		return
	}

	defer h.querySvc.DeleteExport(exportID)

	c.Header("Content-Disposition", "attachment; filename=\"audit-export-"+exportID+"."+getExtension(contentType)+"\"")
	c.Header("Content-Type", contentType)
	c.File(filePath)
}

// parseFilter extracts query parameters into an AuditLogFilter.
func (h *AuditHandler) parseFilter(c *gin.Context) service.AuditLogFilter {
	filter := service.AuditLogFilter{
		Page:   1,
		Size:   20,
		SortBy: "created_at",
	}

	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			filter.Page = v
		}
	}
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			filter.Size = v
		}
	}
	if userID := c.Query("user_id"); userID != "" {
		filter.UserID = &userID
	}
	if deviceID := c.Query("device_id"); deviceID != "" {
		filter.DeviceID = &deviceID
	}
	if action := c.Query("action"); action != "" {
		filter.Action = &action
	}
	if startTime := c.Query("start_time"); startTime != "" {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			filter.StartTime = &t
		}
	}
	if endTime := c.Query("end_time"); endTime != "" {
		if t, err := time.Parse(time.RFC3339, endTime); err == nil {
			filter.EndTime = &t
		}
	}
	if sortBy := c.Query("sort_by"); sortBy != "" {
		allowedFields := map[string]bool{
			"created_at": true,
			"action":     true,
			"user_id":    true,
			"device_id":  true,
		}
		if allowedFields[sortBy] {
			filter.SortBy = sortBy
		}
	}

	return filter
}

// getExtension extracts the file extension from a content type.
func getExtension(contentType string) string {
	switch contentType {
	case "text/csv":
		return "csv"
	case "application/json":
		return "json"
	default:
		return "json"
	}
}
