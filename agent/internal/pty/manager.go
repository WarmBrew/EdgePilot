package pty

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/creack/pty"
	"github.com/gorilla/websocket"
	"github.com/robot-remote-maint/agent/pkg/logger"
)

const maxSessions = 3

var allowedShells = map[string]bool{
	"/bin/bash":     true,
	"/bin/sh":       true,
	"/bin/zsh":      true,
	"/usr/bin/bash": true,
	"/usr/bin/sh":   true,
	"/usr/bin/zsh":  true,
}

func validateShell(shell string) (string, error) {
	if shell == "" {
		shell = "/bin/bash"
	}
	resolved, err := filepath.EvalSymlinks(shell)
	if err != nil {
		return "", fmt.Errorf("shell '%s' not found: %w", shell, err)
	}
	if !allowedShells[resolved] {
		return "", fmt.Errorf("shell '%s' (resolved to '%s') is not allowed", shell, resolved)
	}
	if _, err := exec.LookPath(resolved); err != nil {
		return "", fmt.Errorf("shell '%s' not executable: %w", resolved, err)
	}
	return resolved, nil
}

type PtyMessageWriter interface {
	WriteJSON(v interface{}) error
	WriteMessage(messageType int, data []byte) error
}

type PTYSession struct {
	ID     string
	Pty    *os.File
	Cmd    *exec.Cmd
	Cancel context.CancelFunc
	mu     sync.Mutex
	done   chan struct{}
}

type PTYManager struct {
	sessions  map[string]*PTYSession
	mu        sync.RWMutex
	log       *logger.Logger
	writeMu   sync.Mutex
	ws        PtyMessageWriter
	semaphore chan struct{}
}

func NewManager(log *logger.Logger) *PTYManager {
	return &PTYManager{
		sessions:  make(map[string]*PTYSession),
		log:       log,
		semaphore: make(chan struct{}, maxSessions),
	}
}

func (m *PTYManager) tryAcquire() bool {
	select {
	case m.semaphore <- struct{}{}:
		return true
	default:
		return false
	}
}

func (m *PTYManager) release() {
	<-m.semaphore
}

func (m *PTYManager) SetConnection(ws PtyMessageWriter) {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()
	m.ws = ws
}

func (m *PTYManager) CreateSession(payload json.RawMessage, sessionID string) error {
	var req CreateSessionPayload
	if err := json.Unmarshal(payload, &req); err != nil {
		return fmt.Errorf("failed to parse payload: %w", err)
	}

	if req.SessionID == "" {
		return fmt.Errorf("session_id is required")
	}

	if !m.tryAcquire() {
		return fmt.Errorf("max PTY sessions (%d) reached", maxSessions)
	}

	shell := req.Shell
	if shell == "" {
		shell = "/bin/bash"
	}
	resolvedShell, err := validateShell(shell)
	if err != nil {
		return fmt.Errorf("invalid shell: %w", err)
	}

	cols := req.Cols
	if cols == 0 {
		cols = 80
	}
	rows := req.Rows
	if rows == 0 {
		rows = 24
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, resolvedShell)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
		Setpgid:   true,
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		m.release()
		cancel()
		return fmt.Errorf("failed to start PTY: %w", err)
	}

	if err := pty.Setsize(ptmx, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	}); err != nil {
		ptmx.Close()
		cancel()
		cmd.Process.Kill()
		m.release()
		return fmt.Errorf("failed to set window size: %w", err)
	}

	session := &PTYSession{
		ID:     req.SessionID,
		Pty:    ptmx,
		Cmd:    cmd,
		Cancel: cancel,
		done:   make(chan struct{}),
	}

	m.mu.Lock()
	if _, exists := m.sessions[req.SessionID]; exists {
		m.mu.Unlock()
		ptmx.Close()
		cancel()
		cmd.Process.Kill()
		m.release()
		return fmt.Errorf("session %s already exists", req.SessionID)
	}
	m.sessions[req.SessionID] = session
	m.mu.Unlock()

	m.log.Info("PTY session created",
		"session_id", req.SessionID,
		"shell", shell,
		"cols", cols,
		"rows", rows,
		"pty_path", ptmx.Name())

	if err := m.sendJSON(map[string]interface{}{
		"type":    "create_pty_resp",
		"session": sessionID,
		"payload": PTYReady{
			SessionID: req.SessionID,
			PtyPath:   ptmx.Name(),
		},
	}); err != nil {
		m.log.Error("Failed to send PTY ready message", "error", err)
	}

	go m.readLoop(session)
	go m.waitProcess(session)

	return nil
}

func (m *PTYManager) WriteToPTY(sessionID string, data []byte) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	select {
	case <-session.done:
		return fmt.Errorf("session %s is closed", sessionID)
	default:
	}

	n, err := session.Pty.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to PTY: %w", err)
	}
	if n < len(data) {
		m.log.Warn("Partial write to PTY",
			"session_id", sessionID,
			"requested", len(data),
			"written", n)
	}

	return nil
}

func (m *PTYManager) ResizePTY(sessionID string, cols, rows uint16) error {
	m.mu.RLock()
	session, exists := m.sessions[sessionID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	select {
	case <-session.done:
		return fmt.Errorf("session %s is closed", sessionID)
	default:
	}

	if err := pty.Setsize(session.Pty, &pty.Winsize{
		Cols: cols,
		Rows: rows,
	}); err != nil {
		return fmt.Errorf("failed to resize PTY: %w", err)
	}

	m.log.Debug("PTY resized",
		"session_id", sessionID,
		"cols", cols,
		"rows", rows)

	return nil
}

func (m *PTYManager) CloseSession(sessionID string) error {
	m.mu.Lock()
	session, exists := m.sessions[sessionID]
	m.mu.Unlock()

	if !exists {
		return fmt.Errorf("session %s not found", sessionID)
	}

	return m.removeSession(session, "closed by client")
}

func (m *PTYManager) CloseAll() {
	m.mu.Lock()
	sessions := make([]*PTYSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()

	for _, s := range sessions {
		m.removeSession(s, "manager closing")
	}
}

func (m *PTYManager) readLoop(session *PTYSession) {
	buf := make([]byte, 4096)

	for {
		select {
		case <-session.done:
			return
		default:
		}

		n, err := session.Pty.Read(buf)
		if err != nil {
			if err != io.EOF {
				m.log.Error("PTY read error",
					"session_id", session.ID,
					"error", err)
			}
			return
		}

		if n == 0 {
			continue
		}

		encoded := base64.StdEncoding.EncodeToString(buf[:n])

		if err := m.sendJSON(PTYOutput{
			SessionID: session.ID,
			Data:      encoded,
		}); err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway) {
				m.log.Error("Failed to forward PTY output",
					"session_id", session.ID,
					"error", err)
			} else if err == websocket.ErrCloseSent {
				select {
				case <-session.done:
				default:
					m.removeSession(session, "websocket closed")
				}
			}
			return
		}
	}
}

func (m *PTYManager) waitProcess(session *PTYSession) {
	exitCode := 0

	if err := session.Cmd.Wait(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			m.log.Error("Process wait error",
				"session_id", session.ID,
				"error", err)
		}
	}

	m.log.Info("PTY process exited",
		"session_id", session.ID,
		"exit_code", exitCode)

	m.removeSession(session, fmt.Sprintf("process exited with code %d", exitCode))
}

func (m *PTYManager) removeSession(session *PTYSession, reason string) error {
	session.mu.Lock()
	defer session.mu.Unlock()

	select {
	case <-session.done:
		return nil
	default:
	}

	m.mu.Lock()
	delete(m.sessions, session.ID)
	m.mu.Unlock()

	session.Cancel()
	close(session.done)

	if session.Pty != nil {
		session.Pty.Close()
	}

	if session.Cmd.Process != nil {
		session.Cmd.Process.Kill()
	}

	m.release()

	m.log.Info("PTY session removed",
		"session_id", session.ID,
		"reason", reason)

	if err := m.sendJSON(PTYClosed{
		SessionID: session.ID,
		Reason:    reason,
	}); err != nil {
		m.log.Error("Failed to send PTY closed message",
			"session_id", session.ID,
			"error", err)
	}

	return nil
}

func (m *PTYManager) sendJSON(v interface{}) error {
	m.writeMu.Lock()
	defer m.writeMu.Unlock()

	if m.ws == nil {
		return fmt.Errorf("websocket connection not set")
	}

	return m.ws.WriteJSON(v)
}
