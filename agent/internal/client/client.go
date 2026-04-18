package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/robot-remote-maint/agent/internal/config"
	"github.com/robot-remote-maint/agent/internal/fileop"
	"github.com/robot-remote-maint/agent/internal/pty"
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

	ptyMgr      *pty.PTYManager
	fileHandler *fileop.FileHandler
}

func New(cfg *config.Config, log *logger.Logger) *Client {
	return &Client{
		cfg:         cfg,
		log:         log,
		state:       StateDisconnected,
		maxRetries:  5,
		ptyMgr:      pty.NewManager(log),
		fileHandler: fileop.NewFileHandler(log),
	}
}

func (c *Client) Connect(ctx context.Context) error {
	if !strings.HasPrefix(c.cfg.ServerURL, "wss://") {
		return fmt.Errorf("insecure connection: server URL must use wss:// protocol")
	}
	if c.cfg.AgentToken == "" {
		return fmt.Errorf("AGENT_TOKEN is required but not set")
	}
	if c.cfg.DeviceID == "" {
		return fmt.Errorf("DEVICE_ID is required but not set")
	}

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

		c.mu.Lock()
		c.ptyMgr.SetConnection(c.conn)
		c.mu.Unlock()

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

	c.ptyMgr.CloseAll()

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
		Type    string          `json:"type"`
		Payload json.RawMessage `json:"payload"`
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
		c.handlePTYCreate(base.Payload)
	case "pty_input":
		c.handlePTYInput(base.Payload)
	case "pty_resize":
		c.handlePTYResize(base.Payload)
	case "pty_close":
		c.handlePTYClose(base.Payload)
	case "list_dir":
		c.handleListDir(base.Payload)
	case "read_file":
		c.handleReadFile(base.Payload)
	case "write_file":
		c.handleWriteFile(base.Payload)
	case "delete_file":
		c.handleDeleteFile(base.Payload)
	case "stat_file":
		c.handleStatFile(base.Payload)
	case "chmod":
		c.handleChmod(base.Payload)
	case "chown":
		c.handleChown(base.Payload)
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

func (c *Client) handlePTYCreate(payload json.RawMessage) {
	if err := c.ptyMgr.CreateSession(payload); err != nil {
		c.log.Error("Failed to create PTY session", "error", err)
	}
}

func (c *Client) handlePTYInput(payload json.RawMessage) {
	var req pty.WritePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		c.log.Error("Failed to parse pty_input payload", "error", err)
		return
	}

	data, err := base64.StdEncoding.DecodeString(req.Data)
	if err != nil {
		c.log.Error("Failed to decode base64 input", "error", err)
		return
	}

	if err := c.ptyMgr.WriteToPTY(req.SessionID, data); err != nil {
		c.log.Error("Failed to write to PTY", "error", err, "session_id", req.SessionID)
	}
}

func (c *Client) handlePTYResize(payload json.RawMessage) {
	var req pty.ResizePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		c.log.Error("Failed to parse pty_resize payload", "error", err)
		return
	}

	if err := c.ptyMgr.ResizePTY(req.SessionID, req.Cols, req.Rows); err != nil {
		c.log.Error("Failed to resize PTY", "error", err, "session_id", req.SessionID)
	}
}

func (c *Client) handlePTYClose(payload json.RawMessage) {
	var req pty.ClosePayload
	if err := json.Unmarshal(payload, &req); err != nil {
		c.log.Error("Failed to parse pty_close payload", "error", err)
		return
	}

	if err := c.ptyMgr.CloseSession(req.SessionID); err != nil {
		c.log.Error("Failed to close PTY session", "error", err, "session_id", req.SessionID)
	}
}

func (c *Client) handleListDir(payload json.RawMessage) {
	var req fileop.ListDirRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorResponse("list_dir", "invalid payload: "+err.Error())
		return
	}

	files, err := c.fileHandler.ListDirectory(req.Path)
	if err != nil {
		c.sendErrorResponse("list_dir", err.Error())
		return
	}

	resp := fileop.ListDirResponse{
		Files: files,
		Path:  req.Path,
	}
	c.sendResponse("list_dir", resp)
}

func (c *Client) handleReadFile(payload json.RawMessage) {
	var req fileop.ReadFileRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorResponse("read_file", "invalid payload: "+err.Error())
		return
	}

	result, err := c.fileHandler.ReadFile(req.Path)
	if err != nil {
		c.sendErrorResponse("read_file", err.Error())
		return
	}

	c.sendResponse("read_file", result)
}

func (c *Client) handleWriteFile(payload json.RawMessage) {
	var req fileop.WriteFileRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorResponse("write_file", "invalid payload: "+err.Error())
		return
	}

	err := c.fileHandler.WriteFile(req.Path, req.Content, req.Mode)
	if err != nil {
		c.sendErrorResponse("write_file", err.Error())
		return
	}

	c.sendResponse("write_file", fileop.WriteFileResponse{Success: true})
}

func (c *Client) handleDeleteFile(payload json.RawMessage) {
	var req fileop.DeleteFileRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorResponse("delete_file", "invalid payload: "+err.Error())
		return
	}

	err := c.fileHandler.DeleteFile(req.Path)
	if err != nil {
		c.sendErrorResponse("delete_file", err.Error())
		return
	}

	c.sendResponse("delete_file", map[string]bool{"success": true})
}

func (c *Client) handleStatFile(payload json.RawMessage) {
	var req fileop.GetFileInfoRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorResponse("stat_file", "invalid payload: "+err.Error())
		return
	}

	info, err := c.fileHandler.GetFileInfo(req.Path)
	if err != nil {
		c.sendErrorResponse("stat_file", err.Error())
		return
	}

	c.sendResponse("stat_file", info)
}

func (c *Client) handleChmod(payload json.RawMessage) {
	var req fileop.ChmodRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorResponse("chmod", "invalid payload: "+err.Error())
		return
	}

	err := c.fileHandler.ChangePermission(req.Path, req.Mode)
	if err != nil {
		c.sendErrorResponse("chmod", err.Error())
		return
	}

	c.sendResponse("chmod", map[string]bool{"success": true})
}

func (c *Client) handleChown(payload json.RawMessage) {
	var req fileop.ChownRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		c.sendErrorResponse("chown", "invalid payload: "+err.Error())
		return
	}

	err := c.fileHandler.ChangeOwnership(req.Path, req.Owner, req.Group)
	if err != nil {
		c.sendErrorResponse("chown", err.Error())
		return
	}

	c.sendResponse("chown", map[string]bool{"success": true})
}

func (c *Client) sendResponse(msgType string, payload interface{}) {
	resp := map[string]interface{}{
		"type":    msgType + "_resp",
		"payload": payload,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		c.log.Error("Failed to marshal response", "error", err)
		return
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		c.log.Error("No connection to send response")
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.log.Error("Failed to send response", "error", err)
	}
}

func (c *Client) sendErrorResponse(msgType, errMsg string) {
	resp := map[string]interface{}{
		"type":  msgType + "_resp",
		"error": errMsg,
	}

	data, err := json.Marshal(resp)
	if err != nil {
		c.log.Error("Failed to marshal error response", "error", err)
		return
	}

	c.mu.RLock()
	conn := c.conn
	c.mu.RUnlock()

	if conn == nil {
		c.log.Error("No connection to send error response")
		return
	}

	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		c.log.Error("Failed to send error response", "error", err)
	}
}
