package pty

import (
	"encoding/json"
	"os"
	"sync"
	"testing"

	"github.com/robot-remote-maint/agent/pkg/logger"
)

func newTestLogger() *logger.Logger {
	return logger.New("info")
}

func newTestManager(t *testing.T) *PTYManager {
	t.Helper()
	mgr := NewManager(newTestLogger())
	return mgr
}

func TestNewManager(t *testing.T) {
	log := newTestLogger()
	mgr := NewManager(log)

	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}
	if mgr.sessions == nil {
		t.Fatal("expected non-nil sessions map")
	}
	if len(mgr.sessions) != 0 {
		t.Fatalf("expected empty sessions map, got %d entries", len(mgr.sessions))
	}
}

func TestCreateSession_EmptySessionID(t *testing.T) {
	mgr := newTestManager(t)

	payload, _ := json.Marshal(map[string]interface{}{
		"session_id": "",
	})

	err := mgr.CreateSession(payload)
	if err == nil {
		t.Fatal("expected error for empty session_id")
	}
}

func TestCreateSession_InvalidPayload(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.CreateSession(json.RawMessage(`{invalid json`))
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}

func TestCreateSession_SessionIDRequired(t *testing.T) {
	mgr := newTestManager(t)

	payload, _ := json.Marshal(map[string]interface{}{
		"cols": 80,
		"rows": 24,
	})

	err := mgr.CreateSession(payload)
	if err == nil {
		t.Fatal("expected error when session_id is missing")
	}
}

func TestCloseSession_NonExistent(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.CloseSession("non-existent")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestWriteToPTY_NonExistent(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.WriteToPTY("non-existent", []byte("test"))
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestResizePTY_NonExistent(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.ResizePTY("non-existent", 120, 40)
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}
}

func TestPTYManager_SetConnection(t *testing.T) {
	mgr := newTestManager(t)

	mockConn := &mockWebSocketConn{}
	mgr.SetConnection(mockConn)

	mgr.writeMu.Lock()
	if mgr.ws == nil {
		t.Fatal("expected ws to be set after SetConnection")
	}
	mgr.writeMu.Unlock()
}

func TestPTYManager_SendJSON_NoConnection(t *testing.T) {
	mgr := newTestManager(t)

	err := mgr.sendJSON(PTYReady{SessionID: "test", PtyPath: "/dev/pts/0"})
	if err == nil {
		t.Fatal("expected error when websocket not set")
	}
}

func TestPTYManager_CloseAll_Empty(t *testing.T) {
	mgr := newTestManager(t)

	mgr.CloseAll()

	mgr.mu.RLock()
	count := len(mgr.sessions)
	mgr.mu.RUnlock()

	if count != 0 {
		t.Fatalf("expected 0 sessions after CloseAll on empty manager, got %d", count)
	}
}

func TestPTYManager_RemoveSession_Idempotent(t *testing.T) {
	mgr := newTestManager(t)

	session := &PTYSession{
		ID:     "test-session",
		done:   make(chan struct{}),
		Cancel: func() {},
	}

	session.Cancel()
	close(session.done)

	if err := mgr.removeSession(session, "test"); err != nil {
		t.Fatalf("removeSession should be idempotent on already closed session: %v", err)
	}
}

func TestPTYManager_MaxSessionLimit_Concurrent(t *testing.T) {
	mgr := newTestManager(t)

	var wg sync.WaitGroup
	var successCount int
	var errCount int
	var mu sync.Mutex

	for i := 0; i < maxSessions*2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			sessionID := "concurrent-test-" + string(rune('0'+id%10)) + "-" + string(rune('0'+id/10))
			payload, _ := json.Marshal(CreateSessionPayload{
				SessionID: sessionID,
				Cols:      80,
				Rows:      24,
			})
			err := mgr.CreateSession(payload)
			mu.Lock()
			defer mu.Unlock()
			if err == nil {
				successCount++
			} else {
				errCount++
			}
		}(i)
	}

	wg.Wait()

	if successCount > maxSessions {
		t.Errorf("expected at most %d concurrent sessions, got %d", maxSessions, successCount)
	}
	if errCount == 0 {
		t.Errorf("expected some sessions to be rejected, but all %d succeeded", successCount)
	}

	mgr.CloseAll()
}

func TestMaxSessions_Constant(t *testing.T) {
	if maxSessions != 3 {
		t.Errorf("expected maxSessions to be 3, got %d", maxSessions)
	}
}

func TestTypes_MarshalUnmarshal(t *testing.T) {
	t.Run("CreateSessionPayload", func(t *testing.T) {
		orig := CreateSessionPayload{
			SessionID: "test-1",
			Shell:     "/bin/bash",
			Cols:      120,
			Rows:      40,
		}
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var parsed CreateSessionPayload
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if parsed != orig {
			t.Errorf("expected %+v, got %+v", orig, parsed)
		}
	})

	t.Run("ResizePayload", func(t *testing.T) {
		orig := ResizePayload{
			SessionID: "test-2",
			Cols:      150,
			Rows:      50,
		}
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var parsed ResizePayload
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if parsed != orig {
			t.Errorf("expected %+v, got %+v", orig, parsed)
		}
	})

	t.Run("WritePayload", func(t *testing.T) {
		orig := WritePayload{
			SessionID: "test-3",
			Data:      "aGVsbG8=",
		}
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var parsed WritePayload
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if parsed != orig {
			t.Errorf("expected %+v, got %+v", orig, parsed)
		}
	})

	t.Run("ClosePayload", func(t *testing.T) {
		orig := ClosePayload{
			SessionID: "test-4",
		}
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var parsed ClosePayload
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if parsed != orig {
			t.Errorf("expected %+v, got %+v", orig, parsed)
		}
	})

	t.Run("PTYOutput", func(t *testing.T) {
		orig := PTYOutput{
			SessionID: "test-5",
			Data:      "aGVsbG8=",
		}
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var parsed PTYOutput
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if parsed != orig {
			t.Errorf("expected %+v, got %+v", orig, parsed)
		}
	})

	t.Run("PTYReady", func(t *testing.T) {
		orig := PTYReady{
			SessionID: "test-6",
			PtyPath:   "/dev/pts/0",
		}
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var parsed PTYReady
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if parsed != orig {
			t.Errorf("expected %+v, got %+v", orig, parsed)
		}
	})

	t.Run("PTYClosed", func(t *testing.T) {
		orig := PTYClosed{
			SessionID: "test-7",
			Reason:    "client closed",
		}
		data, err := json.Marshal(orig)
		if err != nil {
			t.Fatalf("marshal failed: %v", err)
		}

		var parsed PTYClosed
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatalf("unmarshal failed: %v", err)
		}

		if parsed != orig {
			t.Errorf("expected %+v, got %+v", orig, parsed)
		}
	})
}

func TestPTYIntegration(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("PTY integration tests require non-root user with /dev/pts")
	}

	mgr := newTestManager(t)

	payload, _ := json.Marshal(CreateSessionPayload{
		SessionID: "integration-test",
		Cols:      80,
		Rows:      24,
	})

	err := mgr.CreateSession(payload)
	if err != nil {
		t.Fatalf("failed to create PTY session: %v", err)
	}

	mgr.mu.RLock()
	session, exists := mgr.sessions["integration-test"]
	mgr.mu.RUnlock()

	if !exists || session == nil {
		t.Fatal("expected session to exist after creation")
	}

	if session.Pty == nil {
		t.Fatal("expected PTY file descriptor")
	}
	if session.Cmd == nil {
		t.Fatal("expected command object")
	}
	if session.Cancel == nil {
		t.Fatal("expected context cancel function")
	}

	if err := mgr.WriteToPTY("integration-test", []byte("echo hello\n")); err != nil {
		t.Fatalf("failed to write to PTY: %v", err)
	}

	if err := mgr.ResizePTY("integration-test", 120, 40); err != nil {
		t.Fatalf("failed to resize PTY: %v", err)
	}

	if err := mgr.CloseSession("integration-test"); err != nil {
		t.Fatalf("failed to close session: %v", err)
	}

	mgr.mu.RLock()
	_, exists = mgr.sessions["integration-test"]
	mgr.mu.RUnlock()

	if exists {
		t.Fatal("expected session to be removed after close")
	}
}

var _ PtyMessageWriter = (*mockWebSocketConn)(nil)

type mockWebSocketConn struct {
	mu       sync.Mutex
	messages []interface{}
}

func (m *mockWebSocketConn) WriteJSON(v interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, v)
	return nil
}

func (m *mockWebSocketConn) WriteMessage(messageType int, data []byte) error {
	return nil
}

func TestPTYManager_SendJSON_WithMockConnection(t *testing.T) {
	mgr := newTestManager(t)

	mockConn := &mockWebSocketConn{}
	mgr.SetConnection(mockConn)

	err := mgr.sendJSON(PTYReady{SessionID: "test", PtyPath: "/dev/pts/42"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	mockConn.mu.Lock()
	if len(mockConn.messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(mockConn.messages))
	}
	msg, ok := mockConn.messages[0].(PTYReady)
	mockConn.mu.Unlock()

	if !ok {
		t.Fatal("expected PTYReady message")
	}
	if msg.SessionID != "test" {
		t.Errorf("expected session_id 'test', got %q", msg.SessionID)
	}
	if msg.PtyPath != "/dev/pts/42" {
		t.Errorf("expected pty_path '/dev/pts/42', got %q", msg.PtyPath)
	}
}
