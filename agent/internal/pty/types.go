package pty

type CreateSessionPayload struct {
	SessionID string `json:"session_id"`
	Shell     string `json:"shell"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

type ResizePayload struct {
	SessionID string `json:"session_id"`
	Cols      uint16 `json:"cols"`
	Rows      uint16 `json:"rows"`
}

type WritePayload struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

type ClosePayload struct {
	SessionID string `json:"session_id"`
}

type PTYOutput struct {
	SessionID string `json:"session_id"`
	Data      string `json:"data"`
}

type PTYReady struct {
	SessionID string `json:"session_id"`
	PtyPath   string `json:"pty_path"`
}

type PTYClosed struct {
	SessionID string `json:"session_id"`
	Reason    string `json:"reason"`
}
