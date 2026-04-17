package websocket

import "encoding/json"

// WebSocket message type constants.
const (
	MessageTypeAuth         = "auth"
	MessageTypeCreatePTY    = "create_pty"
	MessageTypePTYOutput    = "pty_output"
	MessageTypePTYInput     = "pty_input"
	MessageTypePTYResize    = "pty_resize"
	MessageTypePTYClose     = "pty_close"
	MessageTypeHeartbeat    = "heartbeat"
	MessageTypePong         = "pong"
	MessageTypeConfirm      = "confirm"
	MessageTypeConfirmResp  = "confirm_response"
	MessageTypeListDir      = "list_dir"
	MessageTypeReadFile     = "read_file"
	MessageTypeWriteFile    = "write_file"
	MessageTypeDeleteFile   = "delete_file"
	MessageTypeUploadFile   = "upload_file"
	MessageTypeDownloadFile = "download_file"
)

// WSMessage defines the unified WebSocket message format.
type WSMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	Session string          `json:"session,omitempty"`
}
