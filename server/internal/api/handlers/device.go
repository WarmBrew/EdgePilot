package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/edge-platform/server/internal/api/middleware"
	"github.com/edge-platform/server/internal/domain/models"
	"github.com/edge-platform/server/internal/domain/service"
	devpkg "github.com/edge-platform/server/internal/pkg/device"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/websocket"

	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
	"gorm.io/gorm"
)

var wsUpgrader = gorillaws.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		allowed := os.Getenv("CORS_ORIGINS")
		if allowed == "" {
			return false
		}
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		u, err := url.Parse(origin)
		if err != nil {
			return false
		}
		for _, a := range strings.Split(allowed, ",") {
			a = strings.TrimSpace(a)
			if a == "" {
				continue
			}
			au, err := url.Parse(a)
			if err != nil {
				continue
			}
			if au.Host == u.Host {
				return true
			}
		}
		return false
	},
}

// DeviceHandler handles device/agent registration and management.
type DeviceHandler struct {
	db          *gorm.DB
	redisClient *pkgRedis.RedisClient
	svc         *service.DeviceService
	hub         *websocket.Hub
}

// NewDeviceHandler creates a new DeviceHandler.
func NewDeviceHandler(db *gorm.DB, redis *pkgRedis.RedisClient, hub *websocket.Hub) *DeviceHandler {
	return &DeviceHandler{
		db:          db,
		redisClient: redis,
		svc:         service.NewDeviceService(db, redis),
		hub:         hub,
	}
}

type RegisterDeviceRequest struct {
	Name     string `json:"name" binding:"required,min=1,max=128"`
	Platform string `json:"platform" binding:"required,oneof=jetson rdx rpi"`
	Arch     string `json:"arch" binding:"required,oneof=arm64 amd64"`
}

type RegisterDeviceResponse struct {
	DeviceID   string `json:"device_id"`
	AgentToken string `json:"agent_token"`
}

type VerifyDeviceRequest struct {
	DeviceID   string `json:"device_id" binding:"required,uuid"`
	AgentToken string `json:"agent_token" binding:"required"`
}

type VerifyDeviceResponse struct {
	Verified   bool                `json:"verified"`
	DeviceInfo *DeviceInfoResponse `json:"device_info,omitempty"`
}

type DeviceInfoResponse struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Platform string `json:"platform"`
	Arch     string `json:"arch"`
	Status   string `json:"status"`
	TenantID string `json:"tenant_id"`
}

type AgentAuthMessage struct {
	Type       string `json:"type"`
	DeviceID   string `json:"device_id"`
	AgentToken string `json:"agent_token"`
}

type UpdateDeviceRequest struct {
	Name        string  `json:"name"`
	GroupID     *string `json:"group_id"`
	Description *string `json:"description"`
}

type BatchDeviceRequest struct {
	DeviceIDs []string       `json:"device_ids" binding:"required"`
	Action    string         `json:"action" binding:"required,oneof=delete move_to_group"`
	Params    map[string]any `json:"params"`
}

// RegisterDevice handles POST /api/v1/devices/register.
func (h *DeviceHandler) RegisterDevice(c *gin.Context) {
	var req RegisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	userRole, _ := middleware.GetRole(c)
	if userRole != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin role required to register devices"})
		return
	}

	req.Name = strings.TrimSpace(req.Name)

	plainToken := devpkg.GenerateAgentToken()
	hashedToken := devpkg.HashAgentToken(plainToken)

	device := models.Device{
		Name:       req.Name,
		Platform:   req.Platform,
		Arch:       req.Arch,
		Status:     models.StatusOffline,
		AgentToken: hashedToken,
		TenantID:   tenantID,
	}

	if err := h.db.Create(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register device"})
		return
	}

	c.JSON(http.StatusCreated, RegisterDeviceResponse{
		DeviceID:   device.ID,
		AgentToken: plainToken,
	})
}

// VerifyDevice handles POST /api/v1/devices/verify.
func (h *DeviceHandler) VerifyDevice(c *gin.Context) {
	var req VerifyDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	var device models.Device
	if err := h.db.Where("id = ?", req.DeviceID).First(&device).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "device not found"})
		return
	}

	if !devpkg.VerifyAgentToken(device.AgentToken, req.AgentToken) {
		c.JSON(http.StatusUnauthorized, gin.H{"verified": false})
		return
	}

	c.JSON(http.StatusOK, VerifyDeviceResponse{
		Verified: true,
		DeviceInfo: &DeviceInfoResponse{
			ID:       device.ID,
			Name:     device.Name,
			Platform: device.Platform,
			Arch:     device.Arch,
			Status:   device.Status,
			TenantID: device.TenantID,
		},
	})
}

// AgentWebSocketAuth handles WebSocket authentication for agents.
// After successful auth, the Agent's WebSocket connection is registered to the Hub
// so that ReadPump/WritePump handle all subsequent messages (heartbeats, PTY, file ops).
func (h *DeviceHandler) AgentWebSocketAuth(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("websocket upgrade failed during agent auth", "error", err)
		return
	}

	// Set read deadline for auth message
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))

	_, msg, err := conn.ReadMessage()
	if err != nil {
		slog.Warn("failed to read auth message", "error", err)
		conn.Close()
		return
	}

	// Clear read deadline
	conn.SetReadDeadline(time.Time{})

	var authMsg AgentAuthMessage
	if err := json.Unmarshal(msg, &authMsg); err != nil {
		slog.Warn("invalid auth message format", "error", err)
		conn.WriteJSON(gin.H{"error": "invalid auth message"})
		conn.Close()
		return
	}

	if authMsg.Type != "auth" {
		conn.WriteJSON(gin.H{"error": "first message must be auth"})
		conn.Close()
		return
	}

	if authMsg.DeviceID == "" || authMsg.AgentToken == "" {
		conn.WriteJSON(gin.H{"error": "device_id and agent_token are required"})
		conn.Close()
		return
	}

	var device models.Device
	if err := h.db.Where("id = ?", authMsg.DeviceID).First(&device).Error; err != nil {
		conn.WriteJSON(gin.H{"error": "device not found"})
		conn.Close()
		return
	}

	if !devpkg.VerifyAgentToken(device.AgentToken, authMsg.AgentToken) {
		conn.WriteJSON(gin.H{"error": "authentication failed"})
		conn.Close()
		return
	}

	// Update device status to online
	now := time.Now()
	h.db.Model(&device).Updates(map[string]interface{}{
		"status":         models.StatusOnline,
		"last_heartbeat": now,
	})

	ctx := c.Request.Context()
	h.redisClient.Raw().Set(ctx, "device:online:"+device.ID, "1", 0)

	slog.Info("agent websocket authenticated", "device_id", authMsg.DeviceID)
	conn.WriteJSON(gin.H{"status": "ok"})

	// Create a Client and register to Hub - the Hub's ReadPump will handle all subsequent messages
	client := websocket.NewClient(h.hub, conn, authMsg.DeviceID)
	h.hub.RegisterClient(client)

	// Start ReadPump and WritePump goroutines
	go client.ReadPump()
	go client.WritePump()
}

// ListDevices handles GET /api/v1/devices
func (h *DeviceHandler) ListDevices(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	filters := service.DeviceListFilters{
		Page:    1,
		Size:    20,
		SortBy:  "created_at",
		SortDir: "desc",
	}

	if p := c.Query("page"); p != "" {
		if v, err := strconv.Atoi(p); err == nil && v > 0 {
			filters.Page = v
		}
	}
	if s := c.Query("size"); s != "" {
		if v, err := strconv.Atoi(s); err == nil && v > 0 {
			filters.Size = v
		}
	}
	if status := c.Query("status"); status != "" {
		filters.Status = status
	}
	if groupID := c.Query("group_id"); groupID != "" {
		filters.GroupID = groupID
	}
	if search := c.Query("search"); search != "" {
		filters.Search = search
	}
	if sortBy := c.Query("sort_by"); sortBy != "" {
		filters.SortBy = sortBy
	}
	if sortDir := c.Query("sort_dir"); sortDir != "" {
		filters.SortDir = sortDir
	}

	resp, err := h.svc.ListDevices(c.Request.Context(), tenantID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list devices"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// GetDevice handles GET /api/v1/devices/:id
func (h *DeviceHandler) GetDevice(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	deviceID := c.Param("id")

	device, err := h.svc.GetDevice(c.Request.Context(), tenantID, deviceID)
	if err != nil {
		if err.Error() == "device not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to get device"})
		return
	}

	c.JSON(http.StatusOK, device)
}

// UpdateDevice handles PUT /api/v1/devices/:id
func (h *DeviceHandler) UpdateDevice(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	deviceID := c.Param("id")

	var req UpdateDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if req.Name == "" && req.GroupID == nil && req.Description == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least one field must be provided for update"})
		return
	}

	input := service.UpdateDeviceInput{
		Name:        req.Name,
		GroupID:     req.GroupID,
		Description: req.Description,
	}

	device, err := h.svc.UpdateDevice(c.Request.Context(), tenantID, deviceID, input)
	if err != nil {
		if err.Error() == "device not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		if err.Error() == "device group not found or does not belong to tenant" {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to update device"})
		return
	}

	c.JSON(http.StatusOK, device)
}

// DeleteDevice handles DELETE /api/v1/devices/:id
func (h *DeviceHandler) DeleteDevice(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	userID, _ := middleware.GetUserID(c)
	deviceID := c.Param("id")

	if err := h.svc.DeleteDevice(c.Request.Context(), tenantID, deviceID, userID); err != nil {
		if err.Error() == "device not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "device not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete device"})
		return
	}

	c.Status(http.StatusNoContent)
}

// BatchDevices handles POST /api/v1/devices/batch
func (h *DeviceHandler) BatchDevices(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	userID, _ := middleware.GetUserID(c)

	var req BatchDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	if len(req.DeviceIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_ids cannot be empty"})
		return
	}

	result, err := h.svc.BatchOperation(c.Request.Context(), tenantID, req.DeviceIDs, req.Action, req.Params, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}
