package models

import (
	"time"

	"gorm.io/gorm"
)

// Tenant represents a multi-tenant organization
type Tenant struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Name      string    `gorm:"size:128;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// User represents a system user within a tenant
type User struct {
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID  string    `gorm:"type:uuid;index"`
	Email     string    `gorm:"size:255;uniqueIndex"`
	Role      string    `gorm:"size:32;not null"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// Device represents an edge device managed by the system
type Device struct {
	ID            string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID      string    `gorm:"type:uuid;index:idx_devices_tenant_status"`
	GroupID       string    `gorm:"type:uuid;index:idx_devices_group"`
	Name          string    `gorm:"size:128"`
	Status        string    `gorm:"size:16;index:idx_devices_tenant_status"`
	LastHeartbeat time.Time `gorm:"index:idx_devices_heartbeat"`
	CreatedAt     time.Time `gorm:"autoCreateTime"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime"`
}

// DeviceGroup represents a logical grouping of devices
type DeviceGroup struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	TenantID    string    `gorm:"type:uuid;index"`
	Name        string    `gorm:"size:128;not null"`
	Description string    `gorm:"size:512"`
	CreatedAt   time.Time `gorm:"autoCreateTime"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime"`
}

// TerminalSession represents an active or closed terminal session
type TerminalSession struct {
	ID        string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	DeviceID  string `gorm:"type:uuid;index"`
	UserID    string `gorm:"type:uuid;index"`
	Status    string `gorm:"size:16"`
	StartedAt time.Time
	ClosedAt  time.Time
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

// AuditLog records user actions for compliance and debugging
type AuditLog struct {
	ID        uint      `gorm:"primaryKey;autoIncrement"`
	TenantID  string    `gorm:"type:uuid;index:idx_audit_logs_tenant_time"`
	UserID    string    `gorm:"type:uuid;index"`
	DeviceID  string    `gorm:"type:uuid;index"`
	Action    string    `gorm:"size:64"`
	Detail    string    `gorm:"type:jsonb"`
	IPAddress string    `gorm:"type:inet"`
	CreatedAt time.Time `gorm:"index:idx_audit_logs_tenant_time"`
}

// AutoMigrate runs schema migration for all models
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&Tenant{},
		&User{},
		&Device{},
		&DeviceGroup{},
		&TerminalSession{},
		&AuditLog{},
	)
}

// CreateIndexes creates composite and partial indexes after migration
func CreateIndexes(db *gorm.DB) error {
	indexes := []struct {
		name  string
		table string
		expr  string
	}{
		{
			name:  "idx_devices_tenant_status",
			table: "devices",
			expr:  "tenant_id, status",
		},
		{
			name:  "idx_devices_group",
			table: "devices",
			expr:  "group_id",
		},
		{
			name:  "idx_devices_heartbeat",
			table: "devices",
			expr:  "last_heartbeat DESC",
		},
		{
			name:  "idx_audit_logs_tenant_time",
			table: "audit_logs",
			expr:  "tenant_id, created_at DESC",
		},
	}

	for _, idx := range indexes {
		stmt := "CREATE INDEX IF NOT EXISTS " + idx.name + " ON " + idx.table + " (" + idx.expr + ")"
		if err := db.Exec(stmt).Error; err != nil {
			return err
		}
	}

	return nil
}

// InitializeDatabase enables PostgreSQL extensions, runs AutoMigrate, and creates indexes
func InitializeDatabase(db *gorm.DB) error {
	// Enable uuid-ossp extension for gen_random_uuid()
	if err := db.Exec("CREATE EXTENSION IF NOT EXISTS \"uuid-ossp\"").Error; err != nil {
		return err
	}

	if err := AutoMigrate(db); err != nil {
		return err
	}

	return CreateIndexes(db)
}
