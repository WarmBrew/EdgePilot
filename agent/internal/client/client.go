package client

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/robot-remote-maint/agent/internal/config"
	"github.com/robot-remote-maint/agent/pkg/logger"
)

type ConnectionState int

const (
	StateDisconnected ConnectionState = iota
	StateConnecting
	StateConnected
	StateAuthenticating
	StateAuthenticated
)

type authMessage struct {
	Type     string `json:"type"`
	DeviceID string `json:"device_id"`
	Token    string `json:"agent_token"`
}

type heartbeatMessage struct {
	Type string `json:"type"`
}

type Client struct {
	cfg    *config.Config
	log    *logger.Logger
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	mu    sync.RWMutex
	state ConnectionState

	retries    int
	maxRetries int
}

func New(cfg *config.Config, log *logger.Logger) *Client {
	return &Client{
		cfg:        cfg,
		log:        log,
		state:      StateDisconnected,
		maxRetries: 5,
	}
}

func (c *Client) Connect(ctx context.Context) error {
	c.ctx, c.cancel = context.WithCancel(ctx)
	return c.connectWithRetry()
}

func (c *Client) connectWithRetry() error {
	for c.retries <= c.maxRetries {
		c.setState(StateConnecting)
		c.log.Info("Connecting to server",
			"url", c.cfg.ServerURL,
			"attempt", c.retries+1,
			"max_retries", c.maxRetries)

		if err := c.dialAndAuth(); err != nil {
			c.log.Error("Connection failed", "error", err, "retries", c.retries)
			c.setState(StateDisconnected)

			if c.retries >= c.maxRetries {
				return fmt.Errorf("max retries reached (%d): %w", c.maxRetries, err)
			}

			backoff := c.calculateBackoff()
			c.log.Info("Retrying after backoff", "backoff_seconds", backoff.Seconds())

			select {
			case <-time.After(backoff):
				c.retries++
				continue
			case <-c.ctx.Done():
				return c.ctx.Err()
			}
		}

		c.retries = 0
		c.setState(StateAuthenticated)
		c.log.Info("Successfully connected and authenticated")

		go c.sendHeartbeat()
		go c.messageLoop()

		return nil
	}

	return fmt.Errorf("max connection retries exceeded")
}

func (c *Client) dialAndAuth() error {
	dialer := websocket.Dialer{
		HandshakeTimeout: 10 * time.Second,
	}

	conn, _, err := dialer.Dial(c.cfg.ServerURL, nil)
	if err != nil {
		return fmt.Errorf("dial failed: %w", err)
	}
	c.conn = conn

	c.setState(StateAuthenticating)

	authMsg := authMessage{
		Type:     "auth",
		DeviceID: c.cfg.DeviceID,
		Token:    c.cfg.AgentToken,
	}

	if err := c.conn.WriteJSON(authMsg); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send auth: %w", err)
	}

	var resp map[string]interface{}
	if err := c.conn.ReadJSON(&resp); err != nil {
		conn.Close()
		return fmt.Errorf("failed to read auth response: %w", err)
	}

	if status, ok := resp["status"]; !ok || status != "ok" {
		conn.Close()
		return fmt.Errorf("auth failed: %v", resp)
	}

	return nil
}

func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		c.setState(StateDisconnected)
		return err
	}
	return nil
}

func (c *Client) messageLoop() {
	defer func() {
		c.log.Info("Message loop exited")
		c.Close()
	}()

	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}

		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway) {
				c.log.Error("Unexpected connection close", "error", err)
			}
			return
		}

		c.handleMessage(msg)
	}
}

func (c *Client) handleMessage(msg []byte) {
	var base struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(msg, &base); err != nil {
		c.log.Error("Failed to parse message", "error", err, "raw", string(msg))
		return
	}

	switch base.Type {
	case "ping":
		c.log.Debug("Received ping")
	case "heartbeat_ack":
		c.log.Debug("Received heartbeat ack")
	case "pty_create":
		c.log.Info("Received pty_create command")
	case "pty_input":
		c.log.Info("Received pty_input command")
	case "pty_resize":
		c.log.Info("Received pty_resize command")
	case "pty_close":
		c.log.Info("Received pty_close command")
	case "file_read":
		c.log.Info("Received file_read command")
	case "file_write":
		c.log.Info("Received file_write command")
	default:
		c.log.Warn("Unknown message type", "type", base.Type)
	}
}

func (c *Client) sendHeartbeat() {
	ticker := time.NewTicker(time.Duration(c.cfg.HeartbeatInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			msg := heartbeatMessage{Type: "heartbeat"}
			c.mu.RLock()
			conn := c.conn
			c.mu.RUnlock()

			if conn == nil {
				return
			}

			if err := conn.WriteJSON(msg); err != nil {
				c.log.Error("Failed to send heartbeat", "error", err)
				return
			}
			c.log.Debug("Heartbeat sent")
		}
	}
}

func (c *Client) setState(state ConnectionState) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = state
}

func (c *Client) GetState() ConnectionState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

func (c *Client) calculateBackoff() time.Duration {
	base := 1 * time.Second
	maxBackoff := 30 * time.Second
	backoff := time.Duration(math.Pow(2, float64(c.retries))) * base
	if backoff > maxBackoff {
		return maxBackoff
	}
	return backoff
}
