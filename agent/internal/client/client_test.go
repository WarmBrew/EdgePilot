package client

import (
	"testing"
	"time"

	"github.com/robot-remote-maint/agent/internal/config"
	"github.com/robot-remote-maint/agent/pkg/logger"
)

func TestNew_ClientInitialState(t *testing.T) {
	cfg := &config.Config{
		ServerURL:         "wss://localhost:8080/ws/agent",
		AgentToken:        "test-token",
		DeviceID:          "test-device",
		Platform:          "linux",
		Arch:              "amd64",
		LogLevel:          "info",
		HeartbeatInterval: 30,
	}
	lg := logger.New("info")
	c := New(cfg, lg)

	if c.state != StateDisconnected {
		t.Errorf("expected initial state Disconnected, got %v", c.state)
	}
	if c.maxRetries != 5 {
		t.Errorf("expected maxRetries=5, got %d", c.maxRetries)
	}
	if c.retries != 0 {
		t.Errorf("expected retries=0, got %d", c.retries)
	}
}

func TestClient_StateTransitions(t *testing.T) {
	c := &Client{state: StateDisconnected}

	c.setState(StateConnecting)
	if c.GetState() != StateConnecting {
		t.Errorf("expected Connecting, got %v", c.GetState())
	}

	c.setState(StateConnected)
	if c.GetState() != StateConnected {
		t.Errorf("expected Connected, got %v", c.GetState())
	}

	c.setState(StateAuthenticated)
	if c.GetState() != StateAuthenticated {
		t.Errorf("expected Authenticated, got %v", c.GetState())
	}

	c.setState(StateDisconnected)
	if c.GetState() != StateDisconnected {
		t.Errorf("expected Disconnected, got %v", c.GetState())
	}
}

func TestCalculateBackoff(t *testing.T) {
	c := &Client{retries: 0}

	tests := []struct {
		retries int
		want    time.Duration
	}{
		{0, 1 * time.Second},
		{1, 2 * time.Second},
		{2, 4 * time.Second},
		{3, 8 * time.Second},
		{4, 16 * time.Second},
	}

	for _, tt := range tests {
		c.retries = tt.retries
		backoff := c.calculateBackoff()
		if backoff != tt.want {
			t.Errorf("retries=%d: expected %v, got %v", tt.retries, tt.want, backoff)
		}
	}
}

func TestCalculateBackoff_CappedAtMax(t *testing.T) {
	c := &Client{retries: 20}
	backoff := c.calculateBackoff()
	if backoff > 30*time.Second {
		t.Errorf("backoff should be capped at 30s, got %v", backoff)
	}
}
