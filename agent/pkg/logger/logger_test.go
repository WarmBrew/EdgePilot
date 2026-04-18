package logger

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
)

func TestNew_DefaultLevel(t *testing.T) {
	lg := New("info")
	if lg == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_DebugLevel(t *testing.T) {
	lg := New("debug")
	if lg == nil {
		t.Fatal("expected non-nil logger")
	}
	if !lg.Enabled(nil, slog.LevelDebug) {
		t.Error("debug level should be enabled")
	}
}

func TestNew_WarnLevel(t *testing.T) {
	lg := New("warn")
	if lg == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_ErrorLevel(t *testing.T) {
	lg := New("error")
	if lg == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestNew_UnknownLevelDefaultsToInfo(t *testing.T) {
	lg := New("unknown")
	if lg == nil {
		t.Fatal("expected non-nil logger")
	}
}

func TestLogger_OutputFormat(t *testing.T) {
	var buf bytes.Buffer
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})
	lg := &Logger{slog.New(handler)}

	lg.Info("test message", "key", "value")

	output := buf.String()
	if !strings.Contains(output, "level=INFO") {
		t.Errorf("expected level=INFO in output, got: %s", output)
	}
	if !strings.Contains(output, "msg=\"test message\"") {
		t.Errorf("expected msg in output, got: %s", output)
	}
	if !strings.Contains(output, "key=value") {
		t.Errorf("expected key=value in output, got: %s", output)
	}
}
