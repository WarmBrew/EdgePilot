package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/edge-platform/server/internal/domain/models"
	devpkg "github.com/edge-platform/server/internal/pkg/device"
	"github.com/edge-platform/server/internal/pkg/redis"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"gorm.io/gorm"
)

type DeviceHandler struct {
	db          *gorm.DB
	redisClient *redis.RedisClient
}

func NewDeviceHandler(db *gorm.DB, redis *redis.RedisClient) *DeviceHandler {
	return &DeviceHandler{db: db, redisClient: redis}
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

// RegisterDevice handles POST /api/v1/devices/register.
func (h *DeviceHandler) RegisterDevice(c *gin.Context) {
	var req RegisterDeviceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
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
		TenantID:   "00000000-0000-0000-0000-000000000000",
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
func (h *DeviceHandler) AgentWebSocketAuth(c *gin.Context) {
	conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}

	defer conn.Close()

	_, msg, err := conn.ReadMessage()
	if err != nil {
		return
	}

	var authMsg AgentAuthMessage
	if err := json.Unmarshal(msg, &authMsg); err != nil {
		conn.WriteJSON(gin.H{"error": "invalid auth message"})
		return
	}

	if authMsg.Type != "auth" {
		conn.WriteJSON(gin.H{"error": "first message must be auth"})
		return
	}

	if authMsg.DeviceID == "" || authMsg.AgentToken == "" {
		conn.WriteJSON(gin.H{"error": "device_id and agent_token are required"})
		return
	}

	var device models.Device
	if err := h.db.Where("id = ?", authMsg.DeviceID).First(&device).Error; err != nil {
		conn.WriteJSON(gin.H{"error": "device not found"})
		return
	}

	if !devpkg.VerifyAgentToken(device.AgentToken, authMsg.AgentToken) {
		conn.WriteJSON(gin.H{"error": "authentication failed"})
		return
	}

	ctx := c.Request.Context()

	if err := h.redisClient.Raw().Set(ctx, "device:online:"+device.ID, "1", 0).Err(); err != nil {
		conn.WriteJSON(gin.H{"error": "internal error"})
		return
	}

	if err := h.redisClient.Raw().Set(ctx, "device:conn:"+device.ID, conn.RemoteAddr().String(), 0).Err(); err != nil {
		conn.WriteJSON(gin.H{"error": "internal error"})
		return
	}

	conn.WriteJSON(gin.H{"status": "authenticated", "device_id": device.ID})

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}

	h.redisClient.Raw().Del(ctx, "device:online:"+device.ID)
	h.redisClient.Raw().Del(ctx, "device:conn:"+device.ID)
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}
