package websocket

import (
	"errors"
	"log/slog"
	"sync"

	"github.com/edge-platform/server/internal/domain/models"

	"gorm.io/gorm"
)

var ErrClientChannelFull = errors.New("client send channel is full")

var ErrDeviceOnlineLimit = errors.New("device has reached maximum number of connections")

// MessageHandler is called when a message is received from a device.
type MessageHandler func(deviceID string, msg *WSMessage)

// HeartbeatCallback is called when a heartbeat message is received from a device.
type HeartbeatCallback func(deviceID string)

// DisconnectCallback is called when a device disconnects from the hub.
type DisconnectCallback func(deviceID string)

// Hub maintains the set of active clients and broadcasts messages to them.
type Hub struct {
	clients    map[string]*Client // deviceID -> Client
	mu         sync.RWMutex
	register   chan *Client
	unregister chan *Client

	messageHandler     MessageHandler
	heartbeatCallback  HeartbeatCallback
	disconnectCallback DisconnectCallback
	db                 *gorm.DB
}

// NewHub creates a new Hub instance.
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[string]*Client),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// SetDB sets the database connection for heartbeat tracking.
func (h *Hub) SetDB(db *gorm.DB) {
	h.db = db
}

// SetDisconnectCallback sets the callback for device disconnect events.
func (h *Hub) SetDisconnectCallback(cb DisconnectCallback) {
	h.disconnectCallback = cb
}

// SetMessageHandler sets the handler for incoming messages from devices.
func (h *Hub) SetMessageHandler(handler MessageHandler) {
	h.messageHandler = handler
}

// Run starts the hub's event loop. It should be called as a goroutine.
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client.deviceID] = client
			count := len(h.clients)
			h.mu.Unlock()
			slog.Info("device connected", "device_id", client.deviceID, "online_devices", count)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client.deviceID]; ok {
				delete(h.clients, client.deviceID)
				close(client.send)
			}
			count := len(h.clients)
			h.mu.Unlock()
			slog.Info("device disconnected", "device_id", client.deviceID, "online_devices", count)

			// Notify disconnect callback (e.g., Gateway to clean up Redis)
			if h.disconnectCallback != nil {
				go h.disconnectCallback(client.deviceID)
			}

			// Update DB status to offline when last connection for this device closes
			if h.db != nil {
				go func(dID string) {
					h.db.Model(&models.Device{}).
						Where("id = ?", dID).
						Updates(map[string]interface{}{
							"status": models.StatusOffline,
						})
					slog.Info("device marked offline in DB", "device_id", dID)
				}(client.deviceID)
			}
		}
	}
}

// RegisterClient sends a client to be registered with the hub.
func (h *Hub) RegisterClient(client *Client) {
	h.register <- client
}

// UnregisterClient sends a client to be unregistered from the hub.
func (h *Hub) UnregisterClient(client *Client) {
	h.unregister <- client
}

// GetClient returns the client for the given deviceID, or nil if not found.
func (h *Hub) GetClient(deviceID string) *Client {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.clients[deviceID]
}

// Broadcast sends a message to all connected clients.
func (h *Hub) Broadcast(data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for _, client := range h.clients {
		select {
		case client.send <- data:
		default:
			close(client.send)
			delete(h.clients, client.deviceID)
		}
	}
}

// GetOnlineDevices returns a slice of all connected device IDs.
func (h *Hub) GetOnlineDevices() []string {
	h.mu.RLock()
	defer h.mu.RUnlock()
	deviceIDs := make([]string, 0, len(h.clients))
	for id := range h.clients {
		deviceIDs = append(deviceIDs, id)
	}
	return deviceIDs
}
