package handlers

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/edge-platform/server/internal/api/middleware"
	"github.com/edge-platform/server/internal/domain/models"
	"github.com/edge-platform/server/internal/pkg/auth"
	pkgRedis "github.com/edge-platform/server/internal/pkg/redis"
	"github.com/edge-platform/server/internal/service"
	"github.com/edge-platform/server/internal/websocket"

	"github.com/gin-gonic/gin"
	gorillaws "github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	termIdleTimeout          = 30 * time.Minute
	maxTerminalPerDevice     = 3
	writeWait                = 10 * time.Second
	terminalReadLimit        = 64 * 1024
	redisTermActivePrefix    = "terminal:active:"
	redisConfirmPrefix       = "terminal:confirm:"
	confirmTimeout           = 30 * time.Second
	maxConfirmAttemptsPerMin = 5
	redisRateLimitPrefix     = "terminal:confirm_rate:"
)

// upgrader upgrades HTTP connections to WebSocket for browser terminal clients.
var termUpgrader = gorillaws.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     checkTerminalAllowedOrigin,
}

func checkTerminalAllowedOrigin(r *http.Request) bool {
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
	for _, a := range stringsSplit(allowed, ",") {
		a = stringsTrimWs(a)
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
}

func stringsSplit(s, sep string) []string {
	result := []string{}
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i:i+len(sep)] == sep {
			result = append(result, s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	result = append(result, s[start:])
	return result
}

func stringsTrimWs(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t' || s[i] == '\n' || s[i] == '\r') {
		i++
	}
	j := len(s)
	for j > i && (s[j-1] == ' ' || s[j-1] == '\t' || s[j-1] == '\n' || s[j-1] == '\r') {
		j--
	}
	return s[i:j]
}

// TerminalHandler manages WebSocket terminal sessions for browsers.
type TerminalHandler struct {
	db             *gorm.DB
	redis          *pkgRedis.RedisClient
	gw             *websocket.Gateway
	termSessionSvc *service.TerminalSessionService

	activeSessions sync.Map // sessionID -> *browserConn
}

// browserConn holds the state for a single browser WebSocket terminal connection.
type browserConn struct {
	sessionID    string
	userID       string
	deviceID     string
	role         string
	clientIP     string
	conn         *gorillaws.Conn
	ctx          context.Context
	cancel       context.CancelFunc
	mu           sync.Mutex
	lastActivity time.Time
	closed       bool
}

// NewTerminalHandler creates a new TerminalHandler.
func NewTerminalHandler(db *gorm.DB, redis *pkgRedis.RedisClient, gw *websocket.Gateway, termSvc *service.TerminalSessionService) *TerminalHandler {
	h := &TerminalHandler{
		db:             db,
		redis:          redis,
		gw:             gw,
		termSessionSvc: termSvc,
	}
	return h
}

// HandleTerminalWebSocket handles GET /ws/terminal/:session_id.
// Upgrades the HTTP connection to WebSocket and starts bidirectional forwarding.
func (h *TerminalHandler) HandleTerminalWebSocket(c *gin.Context) {
	sessionID := c.Param("session_id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	token := c.Query("token")
	if token == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token is required"})
		return
	}

	claims, err := auth.ValidateToken(token)
	if err != nil {
		slog.Warn("terminal ws: invalid token",
			"session_id", sessionID, "ip", c.ClientIP(), "error", err)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	var session models.TerminalSession
	if err := h.db.Preload("Device").Where("id = ? AND user_id = ?", sessionID, claims.UserID).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		} else {
			slog.Error("terminal ws: database query failed", "session_id", sessionID, "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	if session.Status != models.SessionActive {
		c.JSON(http.StatusConflict, gin.H{"error": "session is not active (status: " + session.Status + ")"})
		return
	}

	if err := h.checkSessionLimit(session.DeviceID); err != nil {
		slog.Warn("terminal ws: session limit exceeded",
			"session_id", sessionID, "device_id", session.DeviceID, "error", err)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}

	clientIP := c.ClientIP()

	conn, err := termUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("terminal ws: upgrade failed", "session_id", sessionID, "error", err)
		return
	}

	conn.SetReadLimit(terminalReadLimit)

	ctx, cancel := context.WithCancel(context.Background())
	bc := &browserConn{
		sessionID:    sessionID,
		userID:       claims.UserID,
		role:         claims.Role,
		deviceID:     session.DeviceID,
		clientIP:     clientIP,
		conn:         conn,
		ctx:          ctx,
		cancel:       cancel,
		lastActivity: time.Now(),
		closed:       false,
	}

	h.activeSessions.Store(sessionID, bc)

	if err := h.redis.Raw().Set(ctx, redisTermActivePrefix+sessionID, session.DeviceID, termIdleTimeout*2).Err(); err != nil {
		slog.Warn("terminal ws: failed to cache session in redis", "session_id", sessionID, "error", err)
	}

	h.writeAuditLog(ctx, claims.UserID, session.DeviceID, sessionID, models.ActionTerminalOpen, clientIP, map[string]interface{}{
		"device_id": session.DeviceID,
		"role":      claims.Role,
		"client_ip": clientIP,
		"pty_path":  session.PtyPath,
	})

	unsubscribe := h.gw.SubscribeToDevice(session.DeviceID, func(msg *websocket.WSMessage) {
		h.routeDeviceMessage(sessionID, msg)
	})
	defer unsubscribe()

	slog.Info("terminal session started",
		"session_id", sessionID, "user_id", claims.UserID,
		"device_id", session.DeviceID, "role", claims.Role)

	h.forwardLoop(bc)
}

// CreateTerminalSession handles POST /api/v1/devices/:id/terminal
func (h *TerminalHandler) CreateTerminalSession(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	deviceID := c.Param("id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}

	session, err := h.termSessionSvc.CreateSession(c.Request.Context(), userID, deviceID)
	if err != nil {
		slog.Warn("failed to create terminal session",
			"device_id", deviceID, "user_id", userID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"session_id": session.SessionID,
		"device_id":  session.DeviceID,
		"pty_path":   session.PtyPath,
	})
}

// ListTerminalSessions handles GET /api/v1/terminal/sessions
func (h *TerminalHandler) ListTerminalSessions(c *gin.Context) {
	tenantID, ok := middleware.GetTenantID(c)
	if !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "tenant context missing"})
		return
	}

	filter := service.ListSessionsFilter{
		TenantID: tenantID,
		Page:     1,
		PageSize: 20,
	}

	if page := c.Query("page"); page != "" {
		if v, err := strconv.Atoi(page); err == nil && v > 0 {
			filter.Page = v
		}
	}
	if size := c.Query("page_size"); size != "" {
		if v, err := strconv.Atoi(size); err == nil && v > 0 {
			filter.PageSize = v
		}
	}
	if deviceID := c.Query("device_id"); deviceID != "" {
		filter.DeviceID = deviceID
	}
	if status := c.Query("status"); status != "" {
		filter.Status = status
	}
	if userID := c.Query("user_id"); userID != "" {
		filter.UserID = userID
	}

	result, err := h.termSessionSvc.ListSessions(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list sessions"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// CloseTerminalSession handles POST /api/v1/terminal/sessions/:id/close
func (h *TerminalHandler) CloseTerminalSession(c *gin.Context) {
	userID, ok := middleware.GetUserID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
		return
	}

	sessionID := c.Param("id")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "session_id is required"})
		return
	}

	if err := h.termSessionSvc.CloseSession(c.Request.Context(), userID, sessionID); err != nil {
		slog.Warn("failed to close terminal session",
			"session_id", sessionID, "user_id", userID, "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "session closed"})
}

// forwardLoop starts the bidirectional forwarding between browser and device.
func (h *TerminalHandler) forwardLoop(bc *browserConn) {
	defer func() {
		h.closeSession(bc)
		slog.Info("terminal session ended", "session_id", bc.sessionID)
	}()

	go h.browserReadLoop(bc)
	go h.monitorIdle(bc)

	select {
	case <-bc.ctx.Done():
		return
	}
}

// browserReadLoop reads messages from the browser WebSocket and forwards them to the device.
func (h *TerminalHandler) browserReadLoop(bc *browserConn) {
	defer func() {
		bc.cancel()
	}()

	for {
		_, raw, err := bc.conn.ReadMessage()
		if err != nil {
			if !gorillaws.IsCloseError(err, gorillaws.CloseNormalClosure, gorillaws.CloseGoingAway) {
				slog.Info("browser read error",
					"session_id", bc.sessionID, "error", err)
			}
			return
		}

		h.updateActivity(bc)

		var msg websocket.WSMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			h.sendToBrowser(bc, "error", map[string]string{"message": "invalid message format"})
			continue
		}

		switch msg.Type {
		case websocket.MessageTypePTYInput:
			h.handleBrowserInput(bc, msg)
		case websocket.MessageTypePTYResize:
			h.handleBrowserResize(bc, msg)
		case websocket.MessageTypePTYClose:
			return
		case websocket.MessageTypeConfirmResp:
			h.handleConfirmResponse(bc, msg)
		default:
			slog.Debug("terminal ws: unknown message type",
				"session_id", bc.sessionID, "type", msg.Type)
		}
	}
}

// handleBrowserInput validates and forwards user keystrokes to the device.
func (h *TerminalHandler) handleBrowserInput(bc *browserConn, msg websocket.WSMessage) {
	var payload struct {
		Data string `json:"data"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		slog.Warn("terminal ws: invalid input payload",
			"session_id", bc.sessionID, "error", err)
		return
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(payload.Data)
	if err != nil {
		slog.Warn("terminal ws: base64 decode failed",
			"session_id", bc.sessionID, "error", err)
		h.sendToBrowser(bc, "error", map[string]string{"message": "invalid input encoding"})
		return
	}

	input := string(decodedBytes)

	if !h.checkConfirmRate(bc) {
		slog.Info("terminal ws: confirm rate limit exceeded",
			"session_id", bc.sessionID, "user_id", bc.userID)
		h.sendToBrowser(bc, "error", map[string]string{"message": "too many confirmation attempts, please wait"})
		return
	}

	checkResult := service.CheckCommand(input, bc.role)
	if checkResult.Blocked {
		slog.Info("terminal ws: command blocked by filter",
			"session_id", bc.sessionID, "reason", checkResult.Reason,
			"role", bc.role, "input", input)
		h.sendToBrowser(bc, "blocked", map[string]string{"reason": checkResult.Reason})

		h.writeAuditLog(bc.ctx, bc.userID, bc.deviceID, bc.sessionID, "command_blocked", bc.clientIP, map[string]interface{}{
			"user_id":   bc.userID,
			"device_id": bc.deviceID,
			"command":   input,
			"reason":    checkResult.Reason,
			"role":      bc.role,
		})
		return
	}

	if checkResult.NeedsConfirm {
		h.sendConfirmRequest(bc, checkResult.Command, input)
		return
	}

	h.forwardInput(bc, payload.Data, input)
}

// handleConfirmResponse processes the user's confirmation response.
func (h *TerminalHandler) handleConfirmResponse(bc *browserConn, msg websocket.WSMessage) {
	var payload struct {
		Confirmed bool   `json:"confirmed"`
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		slog.Warn("terminal ws: invalid confirm response payload",
			"session_id", bc.sessionID, "error", err)
		return
	}

	if payload.SessionID != bc.sessionID {
		slog.Warn("terminal ws: confirm response session mismatch",
			"session_id", bc.sessionID, "payload_session", payload.SessionID)
		return
	}

	confirmKey := redisConfirmPrefix + bc.sessionID
	storedData, err := h.redis.Raw().Get(bc.ctx, confirmKey).Result()
	if err != nil {
		slog.Warn("terminal ws: no pending confirmation found",
			"session_id", bc.sessionID, "error", err)
		return
	}

	var confirmData struct {
		Command  string `json:"command"`
		RawInput string `json:"raw_input"`
		UserID   string `json:"user_id"`
		DeviceID string `json:"device_id"`
	}
	if err := json.Unmarshal([]byte(storedData), &confirmData); err != nil {
		slog.Warn("terminal ws: failed to parse confirm data",
			"session_id", bc.sessionID, "error", err)
		return
	}

	if payload.Confirmed {
		ptPayload := map[string]interface{}{
			"session_id": bc.sessionID,
			"user_id":    bc.userID,
			"data":       base64.StdEncoding.EncodeToString([]byte(confirmData.RawInput)),
		}

		if err := h.gw.SendMessageToDevice(bc.deviceID, websocket.MessageTypePTYInput, ptPayload); err != nil {
			slog.Warn("terminal ws: failed to forward confirmed input to device",
				"session_id", bc.sessionID, "device_id", bc.deviceID, "error", err)
			h.sendToBrowser(bc, "error", map[string]string{"message": "device unreachable"})
			return
		}

		h.writeAuditLog(bc.ctx, bc.userID, bc.deviceID, bc.sessionID, "command_confirmed", bc.clientIP, map[string]interface{}{
			"user_id":      bc.userID,
			"device_id":    bc.deviceID,
			"command":      confirmData.Command,
			"confirmed_by": bc.userID,
		})

		slog.Info("terminal ws: sensitive command confirmed and forwarded",
			"session_id", bc.sessionID, "command", confirmData.Command)
	} else {
		h.writeAuditLog(bc.ctx, bc.userID, bc.deviceID, bc.sessionID, "command_rejected", bc.clientIP, map[string]interface{}{
			"user_id":   bc.userID,
			"device_id": bc.deviceID,
			"command":   confirmData.Command,
			"reason":    "user_rejected",
		})

		slog.Info("terminal ws: sensitive command rejected by user",
			"session_id", bc.sessionID, "command", confirmData.Command)
	}

	_ = h.redis.Raw().Del(bc.ctx, confirmKey).Err()
}

// checkConfirmRate checks if the user has exceeded the confirmation rate limit.
func (h *TerminalHandler) checkConfirmRate(bc *browserConn) bool {
	rateKey := redisRateLimitPrefix + bc.sessionID
	pipe := h.redis.Raw().Pipeline()

	pipe.Incr(bc.ctx, rateKey)
	pipe.Expire(bc.ctx, rateKey, time.Minute)

	results, err := pipe.Exec(bc.ctx)
	if err != nil {
		slog.Warn("terminal ws: failed to check confirm rate",
			"session_id", bc.sessionID, "error", err)
		return true
	}

	count, err := results[0].(*redis.IntCmd).Result()
	if err != nil {
		return true
	}

	return count <= maxConfirmAttemptsPerMin
}

// sendConfirmRequest sends a confirmation request to the browser and stores it in Redis.
func (h *TerminalHandler) sendConfirmRequest(bc *browserConn, displayCommand, rawInput string) {
	confirmData := map[string]interface{}{
		"command":   displayCommand,
		"raw_input": rawInput,
		"user_id":   bc.userID,
		"device_id": bc.deviceID,
	}

	dataBytes, err := json.Marshal(confirmData)
	if err != nil {
		slog.Error("terminal ws: failed to marshal confirm data",
			"session_id", bc.sessionID, "error", err)
		return
	}

	confirmKey := redisConfirmPrefix + bc.sessionID
	if err := h.redis.Raw().Set(bc.ctx, confirmKey, string(dataBytes), confirmTimeout).Err(); err != nil {
		slog.Warn("terminal ws: failed to store confirm data in redis",
			"session_id", bc.sessionID, "error", err)
	}

	h.sendToBrowser(bc, websocket.MessageTypeConfirm, map[string]interface{}{
		"command":    displayCommand,
		"session_id": bc.sessionID,
		"timeout":    confirmTimeout.Seconds(),
	})

	slog.Info("terminal ws: confirmation request sent",
		"session_id", bc.sessionID, "command", displayCommand)

	go h.monitorConfirmTimeout(bc, confirmKey, displayCommand, rawInput)
}

// monitorConfirmTimeout waits for the confirmation timeout and cleans up if not confirmed.
func (h *TerminalHandler) monitorConfirmTimeout(bc *browserConn, confirmKey, displayCommand, rawInput string) {
	timer := time.NewTimer(confirmTimeout)
	defer timer.Stop()

	select {
	case <-bc.ctx.Done():
		return
	case <-timer.C:
		_, err := h.redis.Raw().Get(bc.ctx, confirmKey).Result()
		if err == nil {
			h.writeAuditLog(bc.ctx, bc.userID, bc.deviceID, bc.sessionID, "command_timeout", bc.clientIP, map[string]interface{}{
				"user_id":   bc.userID,
				"device_id": bc.deviceID,
				"command":   displayCommand,
				"reason":    "confirm_timeout",
			})

			slog.Info("terminal ws: sensitive command confirmation timed out",
				"session_id", bc.sessionID, "command", displayCommand)

			_ = h.redis.Raw().Del(bc.ctx, confirmKey).Err()
		}
	}
}

// forwardInput forwards the decoded input to the device.
func (h *TerminalHandler) forwardInput(bc *browserConn, data, input string) {
	ptPayload := map[string]interface{}{
		"session_id": bc.sessionID,
		"user_id":    bc.userID,
		"data":       data,
	}

	if err := h.gw.SendMessageToDevice(bc.deviceID, websocket.MessageTypePTYInput, ptPayload); err != nil {
		slog.Warn("terminal ws: failed to forward input to device",
			"session_id", bc.sessionID, "device_id", bc.deviceID, "error", err)
		h.sendToBrowser(bc, "error", map[string]string{"message": "device unreachable"})
	}
}

// handleBrowserResize forwards terminal resize requests to the device.
func (h *TerminalHandler) handleBrowserResize(bc *browserConn, msg websocket.WSMessage) {
	var payload struct {
		Cols int `json:"cols"`
		Rows int `json:"rows"`
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		slog.Warn("terminal ws: invalid resize payload",
			"session_id", bc.sessionID, "error", err)
		return
	}

	if payload.Cols <= 0 || payload.Rows <= 0 {
		return
	}

	resizePayload := map[string]interface{}{
		"session_id": bc.sessionID,
		"user_id":    bc.userID,
		"cols":       payload.Cols,
		"rows":       payload.Rows,
	}

	if err := h.gw.SendMessageToDevice(bc.deviceID, websocket.MessageTypePTYResize, resizePayload); err != nil {
		slog.Warn("terminal ws: failed to forward resize to device",
			"session_id", bc.sessionID, "device_id", bc.deviceID, "error", err)
	}

	slog.Info("terminal ws: resize forwarded to device",
		"session_id", bc.sessionID, "cols", payload.Cols, "rows", payload.Rows)
}

// routeDeviceMessage routes messages received from the device to the corresponding browser session.
func (h *TerminalHandler) routeDeviceMessage(sessionID string, msg *websocket.WSMessage) {
	raw, ok := h.activeSessions.Load(sessionID)
	if !ok {
		return
	}

	bc, ok := raw.(*browserConn)
	if !ok {
		return
	}

	switch msg.Type {
	case websocket.MessageTypePTYOutput:
		var ptyPayload struct {
			SessionID string `json:"session_id"`
			Data      string `json:"data"`
		}
		if err := json.Unmarshal(msg.Payload, &ptyPayload); err != nil {
			slog.Warn("terminal ws: invalid pty_output from device",
				"session_id", sessionID, "error", err)
			h.sendToBrowser(bc, "error", map[string]string{"message": "data from device is corrupted"})
			return
		}

		outputMsg := map[string]interface{}{
			"data": ptyPayload.Data,
		}
		h.sendToBrowser(bc, "output", outputMsg)

	case websocket.MessageTypePTYClose:
		var closePayload struct {
			SessionID string `json:"session_id"`
			Reason    string `json:"reason,omitempty"`
		}
		json.Unmarshal(msg.Payload, &closePayload)

		slog.Info("terminal ws: device closed session",
			"session_id", sessionID, "reason", closePayload.Reason)

		closePayloadOut := map[string]interface{}{
			"reason": closePayload.Reason,
		}
		h.sendToBrowser(bc, "close", closePayloadOut)
		bc.cancel()
	}
}

// sendToBrowser sends a JSON message to the browser WebSocket connection.
func (h *TerminalHandler) sendToBrowser(bc *browserConn, msgType string, payload interface{}) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	if bc.closed {
		return
	}

	wsMsg := websocket.WSMessage{
		Type: msgType,
	}

	if payload != nil {
		raw, err := json.Marshal(payload)
		if err != nil {
			slog.Error("terminal ws: failed to marshal response payload",
				"session_id", bc.sessionID, "type", msgType, "error", err)
			return
		}
		wsMsg.Payload = raw
	}

	data, err := json.Marshal(wsMsg)
	if err != nil {
		return
	}

	bc.conn.SetWriteDeadline(time.Now().Add(writeWait))
	if err := bc.conn.WriteMessage(gorillaws.TextMessage, data); err != nil {
		slog.Info("terminal ws: browser write error",
			"session_id", bc.sessionID, "error", err)
		bc.cancel()
	}
}

// updateActivity records the last activity timestamp for idle detection.
func (h *TerminalHandler) updateActivity(bc *browserConn) {
	bc.mu.Lock()
	bc.lastActivity = time.Now()
	bc.mu.Unlock()
}

// monitorIdle closes the session after idleTimeout of inactivity.
func (h *TerminalHandler) monitorIdle(bc *browserConn) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-bc.ctx.Done():
			return
		case <-ticker.C:
			bc.mu.Lock()
			lastActivity := bc.lastActivity
			closed := bc.closed
			bc.mu.Unlock()

			if closed {
				return
			}

			if time.Since(lastActivity) > termIdleTimeout {
				slog.Info("terminal ws: session idle timeout reached",
					"session_id", bc.sessionID, "duration", time.Since(lastActivity))

				h.sendToBrowser(bc, "close", map[string]string{"reason": "session_timeout"})
				h.writeAuditLog(bc.ctx, bc.userID, bc.deviceID, bc.sessionID, models.ActionTerminalExpire, bc.clientIP, map[string]interface{}{
					"reason":   "idle_timeout",
					"duration": time.Since(lastActivity).String(),
				})

				_ = h.redis.Raw().Del(bc.ctx, redisTermActivePrefix+bc.sessionID).Err()
				bc.cancel()
				return
			}

			_ = h.redis.Raw().Set(bc.ctx, redisTermActivePrefix+bc.sessionID, bc.deviceID, termIdleTimeout*2).Err()
		}
	}
}

// closeSession gracefully closes the browser connection and cleans up resources.
func (h *TerminalHandler) closeSession(bc *browserConn) {
	bc.mu.Lock()
	if bc.closed {
		bc.mu.Unlock()
		return
	}
	bc.closed = true
	bc.conn.Close()
	bc.mu.Unlock()

	h.activeSessions.Delete(bc.sessionID)

	_ = h.redis.Raw().Del(bc.ctx, redisTermActivePrefix+bc.sessionID).Err()
	_ = h.redis.DeleteTerminalSessionCache(bc.ctx, bc.sessionID)

	if bc.deviceID != "" {
		if err := h.termSessionSvc.CloseSession(bc.ctx, bc.userID, bc.sessionID); err != nil {
			slog.Warn("terminal ws: failed to close session in service",
				"session_id", bc.sessionID, "error", err)
		}
	}

	h.writeAuditLog(bc.ctx, bc.userID, bc.deviceID, bc.sessionID, models.ActionTerminalClose, bc.clientIP, map[string]interface{}{
		"reason": "browser_disconnect",
	})
}

// checkSessionLimit ensures a device does not exceed the maximum concurrent terminal sessions.
func (h *TerminalHandler) checkSessionLimit(deviceID string) error {
	ctx := context.Background()
	pattern := redisTermActivePrefix + "*"
	var activeCount int
	var cursor uint64

	for {
		keys, nextCursor, err := h.redis.Raw().Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			slog.Warn("terminal ws: failed to check session limit",
				"device_id", deviceID, "error", err)
			return nil
		}

		for _, key := range keys {
			val, err := h.redis.Raw().Get(ctx, key).Result()
			if err == nil && val == deviceID {
				activeCount++
				if activeCount >= maxTerminalPerDevice {
					return fmt.Errorf("too many active terminal sessions on this device (max %d)", maxTerminalPerDevice)
				}
			}
		}

		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return nil
}

// GetActiveSessions returns the count of currently active terminal sessions.
func (h *TerminalHandler) GetActiveSessions() int {
	count := 0
	h.activeSessions.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}

// writeAuditLog writes a terminal audit log entry to the database.
func (h *TerminalHandler) writeAuditLog(ctx context.Context, userID, deviceID, sessionID, action string, ipAddr string, detail map[string]interface{}) {
	if h.db == nil {
		return
	}

	detailBytes, err := json.Marshal(detail)
	if err != nil {
		slog.Warn("terminal ws: failed to marshal audit detail",
			"error", err, "session_id", sessionID)
		return
	}

	var tenantID string
	var device models.Device
	if err := h.db.WithContext(ctx).Where("id = ?", deviceID).First(&device).Error; err == nil {
		tenantID = device.TenantID
	}

	auditLog := models.AuditLog{
		TenantID:  tenantID,
		UserID:    userID,
		DeviceID:  deviceID,
		Action:    action,
		Detail:    datatypes.JSON(detailBytes),
		IPAddress: ipAddr,
	}

	if err := h.db.WithContext(ctx).Create(&auditLog).Error; err != nil {
		slog.Error("terminal ws: failed to write audit log",
			"action", action, "session_id", sessionID, "error", err)
	} else {
		slog.Info("terminal ws: audit log written",
			"action", action, "session_id", sessionID, "device_id", deviceID)
	}
}
