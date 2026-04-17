package models

import "time"

// TerminalSessionStatus constants
const (
	SessionActive = "active"
	SessionClosed = "closed"
)

// ValidSessionStatuses lists all allowed session status values
var ValidSessionStatuses = []string{SessionActive, SessionClosed}

// TerminalSession represents an active or closed terminal session
type TerminalSession struct {
	ID        string     `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	DeviceID  string     `gorm:"type:uuid;not null;index" json:"device_id"`
	UserID    string     `gorm:"type:uuid;not null;index" json:"user_id"`
	PtyPath   string     `gorm:"size:255" json:"pty_path,omitempty"`
	Status    string     `gorm:"size:16;not null;default:active" json:"status"`
	StartedAt time.Time  `gorm:"not null" json:"started_at"`
	ClosedAt  *time.Time `json:"closed_at,omitempty"`
	CreatedAt time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	Device    Device     `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	User      User       `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// TableName overrides the default table name
func (TerminalSession) TableName() string {
	return "terminal_sessions"
}
