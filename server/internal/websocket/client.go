package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/edge-platform/server/internal/domain/models"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 16 * 1024
)

// Client represents a single WebSocket connection to an edge device.
type Client struct {
	hub      *Hub
	ws       *websocket.Conn
	deviceID string
	send     chan []byte
	mu       sync.Mutex
}

// NewClient creates a new Client instance.
func NewClient(hub *Hub, ws *websocket.Conn, deviceID string) *Client {
	return &Client{
		hub:      hub,
		ws:       ws,
		deviceID: deviceID,
		send:     make(chan []byte, 256),
	}
}

// ReadPump reads messages from the WebSocket connection and forwards them to the hub.
func (c *Client) ReadPump() {
	defer func() {
		c.hub.UnregisterClient(c)
		c.ws.Close()
		slog.Info("device read pump closed", "device_id", c.deviceID)
	}()

	c.ws.SetReadLimit(maxMessageSize)
	c.ws.SetReadDeadline(time.Now().Add(pongWait))
	c.ws.SetPongHandler(func(string) error {
		c.ws.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.ws.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				slog.Warn("device websocket read error", "device_id", c.deviceID, "error", err)
			} else {
				slog.Info("device disconnected", "device_id", c.deviceID)
			}
			break
		}

		msg := &WSMessage{}
		if err := json.Unmarshal(message, msg); err != nil {
			slog.Warn("invalid websocket message format", "device_id", c.deviceID, "error", err)
			continue
		}

		slog.Debug("received message from device", "device_id", c.deviceID, "type", msg.Type)

		switch msg.Type {
		case MessageTypeHeartbeat:
			c.sendPong()
			if c.hub.db != nil {
				c.hub.db.Model(&models.Device{}).
					Where("id = ?", c.deviceID).
					Updates(map[string]interface{}{
						"last_heartbeat": time.Now(),
						"status":         models.StatusOnline,
					})
			}
		default:
			if c.hub.messageHandler != nil {
				c.hub.messageHandler(c.deviceID, msg)
			}
		}
	}
}

// WritePump writes messages from the hub to the WebSocket connection.
func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.ws.Close()
		slog.Info("device write pump closed", "device_id", c.deviceID)
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.mu.Lock()
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.ws.WriteMessage(websocket.CloseMessage, []byte{})
				c.mu.Unlock()
				return
			}
			w, err := c.ws.NextWriter(websocket.TextMessage)
			if err != nil {
				c.mu.Unlock()
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()

		case <-ticker.C:
			c.mu.Lock()
			c.ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.ws.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		}
	}
}

// sendPong sends a pong message back to the client.
func (c *Client) sendPong() {
	msg, err := json.Marshal(&WSMessage{Type: MessageTypePong})
	if err != nil {
		return
	}
	select {
	case c.send <- msg:
	default:
	}
}

// Send sends a WebSocket message to the device.
func (c *Client) Send(msg *WSMessage) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case c.send <- data:
		return nil
	default:
		return ErrClientChannelFull
	}
}

// Close closes the client connection.
func (c *Client) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.ws.Close()
}
