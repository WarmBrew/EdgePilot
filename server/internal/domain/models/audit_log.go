package models

import (
	"time"

	"gorm.io/datatypes"
)

// AuditAction constants
const (
	ActionExecCommand    = "exec_command"
	ActionUpload         = "upload"
	ActionDownload       = "download"
	ActionEdit           = "edit"
	ActionTerminalOpen   = "terminal_open"
	ActionTerminalClose  = "terminal_close"
	ActionTerminalExpire = "terminal_expire"
)

// ValidAuditActions lists all allowed audit action values
var ValidAuditActions = []string{ActionExecCommand, ActionUpload, ActionDownload, ActionEdit, ActionTerminalOpen, ActionTerminalClose, ActionTerminalExpire}

// AuditLog records user actions for compliance and debugging.
// NOTE: AuditLog does NOT use BaseModel and has NO soft delete -- audit logs are permanent.
type AuditLog struct {
	ID        uint           `gorm:"primaryKey;autoIncrement" json:"id"`
	TenantID  string         `gorm:"type:uuid;not null;index:idx_audit_logs_tenant_time" json:"tenant_id"`
	UserID    string         `gorm:"type:uuid;not null;index" json:"user_id"`
	DeviceID  string         `gorm:"type:uuid;not null;index" json:"device_id"`
	Action    string         `gorm:"size:64;not null" json:"action"`
	Detail    datatypes.JSON `gorm:"type:jsonb" json:"detail"`
	IPAddress string         `gorm:"type:inet" json:"ip_address,omitempty"`
	CreatedAt time.Time      `gorm:"not null;index:idx_audit_logs_tenant_time" json:"created_at"`
}

// TableName overrides the default table name
func (AuditLog) TableName() string {
	return "audit_logs"
}
