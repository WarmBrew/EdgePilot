package models

import "time"

// Platform constants for Device
const (
	PlatformJetson = "jetson"
	PlatformRDX    = "rdx"
	PlatformRPI    = "rpi"
)

// ValidPlatforms lists all allowed platform values
var ValidPlatforms = []string{PlatformJetson, PlatformRDX, PlatformRPI}

// Architecture constants for Device
const (
	ArchARM64 = "arm64"
	ArchAMD64 = "amd64"
)

// ValidArchitectures lists all allowed architecture values
var ValidArchitectures = []string{ArchARM64, ArchAMD64}

// DeviceStatus constants
const (
	StatusOnline        = "online"
	StatusOffline       = "offline"
	StatusHeartbeatMiss = "heartbeat_miss"
)

// ValidStatuses lists all allowed device status values
var ValidStatuses = []string{StatusOnline, StatusOffline, StatusHeartbeatMiss}

// Device represents an edge device managed by the system
type Device struct {
	BaseModel
	TenantID         string            `gorm:"type:uuid;not null;index:idx_devices_tenant_status" json:"tenant_id"`
	GroupID          *string           `gorm:"type:uuid;index:idx_devices_group" json:"group_id,omitempty"`
	Name             string            `gorm:"size:128;not null" json:"name"`
	AgentToken       string            `gorm:"size:255;uniqueIndex;not null" json:"-"`
	Platform         string            `gorm:"size:32;not null" json:"platform"`
	Arch             string            `gorm:"size:16;not null" json:"arch"`
	Status           string            `gorm:"size:16;not null;default:offline;index:idx_devices_tenant_status" json:"status"`
	IPAddress        string            `gorm:"type:inet" json:"ip_address,omitempty"`
	LastHeartbeat    *time.Time        `gorm:"index:idx_devices_heartbeat" json:"last_heartbeat,omitempty"`
	LastSeen         *time.Time        `json:"last_seen,omitempty"`
	Tenant           Tenant            `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	DeviceGroup      *DeviceGroup      `gorm:"foreignKey:GroupID" json:"device_group,omitempty"`
	TerminalSessions []TerminalSession `gorm:"foreignKey:DeviceID" json:"terminal_sessions,omitempty"`
}

// TableName overrides the default table name
func (Device) TableName() string {
	return "devices"
}
