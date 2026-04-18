package handlers

import (
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/edge-platform/server/internal/domain/models"
	"github.com/edge-platform/server/internal/pkg/device"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/service"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type HeartbeatHandler struct {
	db          *gorm.DB
	redisClient *pkgRedis.RedisClient
	metricsSvc  *service.MetricsService
}

func NewHeartbeatHandler(db *gorm.DB, redis *pkgRedis.RedisClient) *HeartbeatHandler {
	return &HeartbeatHandler{
		db:          db,
		redisClient: redis,
		metricsSvc:  service.NewMetricsService(redis),
	}
}

type HeartbeatRequest struct {
	Status      string  `json:"status" binding:"omitempty,oneof=healthy warning error"`
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
}

type HeartbeatResponse struct {
	NextHeartbeat int `json:"next_heartbeat"`
}

type MetricsResponse struct {
	CPUUsage    float64 `json:"cpu_usage"`
	MemoryUsage float64 `json:"memory_usage"`
	DiskUsage   float64 `json:"disk_usage"`
	Uptime      int64   `json:"uptime"`
	CollectedAt string  `json:"collected_at"`
}

func (h *HeartbeatHandler) HandleHeartbeat(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device id is required"})
		return
	}

	agentToken := extractAgentToken(c)
	if agentToken == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "agent token is required"})
		return
	}

	var dbDevice models.Device
	if err := h.db.Where("id = ?", deviceID).First(&dbDevice).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	if !device.VerifyAgentToken(dbDevice.AgentToken, agentToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
		return
	}

	var req HeartbeatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	now := time.Now()
	updates := map[string]any{
		"last_heartbeat": now,
		"last_seen":      now,
	}

	if dbDevice.Status == models.StatusOffline || dbDevice.Status == models.StatusHeartbeatMiss {
		updates["status"] = models.StatusOnline
	}

	if err := h.db.Model(&dbDevice).Updates(updates).Error; err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to update device heartbeat",
			"device_id", deviceID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to process heartbeat"})
		return
	}

	if req.CPUUsage > 0 || req.MemoryUsage > 0 || req.DiskUsage > 0 {
		m := service.SystemMetrics{
			CPUUsage:    req.CPUUsage,
			MemoryUsage: req.MemoryUsage,
			DiskUsage:   req.DiskUsage,
			Uptime:      0,
		}
		if err := h.metricsSvc.StoreMetrics(c.Request.Context(), deviceID, m); err != nil {
			slog.ErrorContext(c.Request.Context(), "failed to store device metrics",
				"device_id", deviceID, "error", err)
		}
	}

	slog.InfoContext(c.Request.Context(), "heartbeat received",
		"device_id", deviceID,
		"device_name", dbDevice.Name,
		"status", req.Status,
	)

	c.JSON(http.StatusOK, HeartbeatResponse{NextHeartbeat: 30})
}

func (h *HeartbeatHandler) GetDeviceMetrics(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device id is required"})
		return
	}

	var dbDevice models.Device
	if err := h.db.Where("id = ?", deviceID).First(&dbDevice).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
		return
	}

	agentToken := extractAgentToken(c)
	if agentToken != "" {
		if !device.VerifyAgentToken(dbDevice.AgentToken, agentToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid agent token"})
			return
		}
	} else {
		if _, ok := c.Get("user_id"); !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}
	}

	metrics, err := h.metricsSvc.GetDeviceMetrics(c.Request.Context(), deviceID)
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "failed to get device metrics",
			"device_id", deviceID, "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to retrieve metrics"})
		return
	}

	if metrics == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "no metrics available"})
		return
	}

	c.JSON(http.StatusOK, MetricsResponse{
		CPUUsage:    metrics.CPUUsage,
		MemoryUsage: metrics.MemoryUsage,
		DiskUsage:   metrics.DiskUsage,
		Uptime:      metrics.Uptime,
		CollectedAt: metrics.CollectedAt,
	})
}

func extractAgentToken(c *gin.Context) string {
	if token := c.GetHeader("Authorization"); token != "" {
		if strings.HasPrefix(token, "Bearer ") {
			return strings.TrimPrefix(token, "Bearer ")
		}
		return token
	}

	if token := c.Query("token"); token != "" {
		return token
	}

	return ""
}
