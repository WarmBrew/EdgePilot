package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/edge-platform/server/internal/config"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	goredis "github.com/redis/go-redis/v9"
)

const (
	defaultMaxSessions = 3
	redisKeyOnline     = "ws:online"
	redisKeyPrefix     = "ws:device:"
	redisSessionSuffix = ":sessions"
	redisSessionTTL    = 5 * time.Minute
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Gateway manages WebSocket connections with Redis-backed distributed state.
type Gateway struct {
	redis       *goredis.Client
	hub         *Hub
	maxSessions int
	serverID    string
	mu          sync.Mutex

	respMu      sync.RWMutex
	respWaiters map[string]chan *WSMessage
}

// NewGateway creates a new Gateway instance.
func NewGateway(redisClient *goredis.Client, hub *Hub) *Gateway {
	cfg := config.Get()
	maxSessions := defaultMaxSessions
	if cfg.Agent.MaxBufferSize > 0 && cfg.Agent.MaxBufferSize < 100 {
		maxSessions = cfg.Agent.MaxBufferSize
	}

	return &Gateway{
		redis:       redisClient,
		hub:         hub,
		maxSessions: maxSessions,
		serverID:    fmt.Sprintf("server-%d", time.Now().UnixNano()),
		respWaiters: make(map[string]chan *WSMessage),
	}
}

// HandleWebSocket upgrades the HTTP connection to WebSocket and manages the device session.
func (g *Gateway) HandleWebSocket(c *gin.Context) {
	deviceID := c.Query("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "device_id is required"})
		return
	}

	ctx := context.Background()

	if err := g.checkConnectionLimit(ctx, deviceID); err != nil {
		slog.Warn("device connection limit exceeded", "device_id", deviceID, "error", err)
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many connections for this device"})
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		slog.Error("websocket upgrade failed", "device_id", deviceID, "error", err)
		return
	}

	client := NewClient(g.hub, conn, deviceID)

	if err := g.registerDevice(ctx, deviceID); err != nil {
		slog.Error("failed to register device in redis", "device_id", deviceID, "error", err)
		conn.Close()
		return
	}

	go g.hub.Run()
	go client.WritePump()
	go client.ReadPump()
}

// SendMessageToDevice sends a message to a specific device.
func (g *Gateway) SendMessageToDevice(deviceID, messageType string, payload interface{}) error {
	ctx := context.Background()

	online, err := g.isDeviceOnline(ctx, deviceID)
	if err != nil {
		slog.Warn("failed to check device online status", "device_id", deviceID, "error", err)
	}
	if !online {
		return fmt.Errorf("device %s is not online", deviceID)
	}

	client := g.hub.GetClient(deviceID)
	if client == nil {
		return fmt.Errorf("device %s is not connected to this server", deviceID)
	}

	var rawPayload json.RawMessage
	if payload != nil {
		rawPayload, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
	}

	msg := &WSMessage{
		Type:    messageType,
		Payload: rawPayload,
	}

	return client.Send(msg)
}

// GetOnlineDevices returns a list of currently online device IDs.
func (g *Gateway) GetOnlineDevices() []string {
	ctx := context.Background()
	members, err := g.redis.SMembers(ctx, redisKeyOnline).Result()
	if err != nil {
		slog.Error("failed to get online devices from redis", "error", err)
		return g.hub.GetOnlineDevices()
	}
	return members
}

// Shutdown gracefully closes all client connections.
func (g *Gateway) Shutdown() {
	slog.Info("shutting down websocket gateway...")

	ctx := context.Background()
	g.hub.mu.RLock()
	for _, client := range g.hub.clients {
		g.unregisterDevice(ctx, client.deviceID)
		client.Close()
	}
	g.hub.mu.RUnlock()
}

// checkConnectionLimit checks if the device has reached the maximum session limit.
func (g *Gateway) checkConnectionLimit(ctx context.Context, deviceID string) error {
	key := redisKeyPrefix + deviceID + redisSessionSuffix

	count, err := g.redis.Incr(ctx, key).Result()
	if err != nil {
		slog.Error("failed to increment session count in redis", "device_id", deviceID, "error", err)
		return err
	}

	if count > int64(g.maxSessions) {
		g.redis.Decr(ctx, key)
		return ErrDeviceOnlineLimit
	}

	g.redis.Expire(ctx, key, redisSessionTTL)
	return nil
}

// registerDevice marks a device as online in Redis.
func (g *Gateway) registerDevice(ctx context.Context, deviceID string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, err := g.redis.SAdd(ctx, redisKeyOnline, deviceID).Result(); err != nil {
		return fmt.Errorf("failed to add device to online set: %w", err)
	}

	slog.Info("device registered in redis", "device_id", deviceID)
	return nil
}

// unregisterDevice decrements the session count and removes from online set if zero.
func (g *Gateway) unregisterDevice(ctx context.Context, deviceID string) error {
	key := redisKeyPrefix + deviceID + redisSessionSuffix

	count, err := g.redis.Decr(ctx, key).Result()
	if err != nil {
		slog.Error("failed to decrement session count in redis", "device_id", deviceID, "error", err)
		return err
	}

	if count <= 0 {
		g.redis.Del(ctx, key)
		g.redis.SRem(ctx, redisKeyOnline, deviceID)
		slog.Info("device unregistered from redis", "device_id", deviceID)
	}

	return nil
}

// isDeviceOnline checks if a device is currently online.
func (g *Gateway) isDeviceOnline(ctx context.Context, deviceID string) (bool, error) {
	return g.redis.SIsMember(ctx, redisKeyOnline, deviceID).Result()
}

// IsDeviceOnline checks if a device is currently connected to this server instance.
func (g *Gateway) IsDeviceOnline(deviceID string) bool {
	return g.hub.GetClient(deviceID) != nil
}

// SetMessageHandler sets the message handler on the hub for incoming device messages.
func (g *Gateway) SetMessageHandler(handler MessageHandler) {
	g.hub.SetMessageHandler(g.handleDeviceMessage)

	if handler != nil {
		g.hub.SetMessageHandler(func(deviceID string, msg *WSMessage) {
			if g.tryResolveResponse(msg) {
				return
			}
			handler(deviceID, msg)
		})
	}
}

// SendAndWait sends a message to a device and waits for a response with the given session ID.
// Returns the response message or an error if the device is offline, timeout, or send fails.
func (g *Gateway) SendAndWait(ctx context.Context, deviceID, messageType string, payload interface{}, sessionID string, timeout time.Duration) (*WSMessage, error) {
	online, err := g.isDeviceOnline(ctx, deviceID)
	if err != nil {
		return nil, fmt.Errorf("failed to check device online status: %w", err)
	}
	if !online {
		return nil, fmt.Errorf("device %s is not online", deviceID)
	}

	waiter := make(chan *WSMessage, 1)

	g.respMu.Lock()
	g.respWaiters[sessionID] = waiter
	g.respMu.Unlock()

	defer func() {
		g.respMu.Lock()
		delete(g.respWaiters, sessionID)
		g.respMu.Unlock()
	}()

	if err := g.SendMessageToDevice(deviceID, messageType, payload); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}

	select {
	case resp := <-waiter:
		return resp, nil
	case <-time.After(timeout):
		return nil, fmt.Errorf("waiting for device response timed out after %v", timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *Gateway) handleDeviceMessage(deviceID string, msg *WSMessage) {
	if g.tryResolveResponse(msg) {
		return
	}
}

func (g *Gateway) tryResolveResponse(msg *WSMessage) bool {
	if msg.Session == "" {
		return false
	}

	g.respMu.RLock()
	waiter, exists := g.respWaiters[msg.Session]
	g.respMu.RUnlock()

	if !exists {
		return false
	}

	select {
	case waiter <- msg:
	default:
	}

	return true
}
