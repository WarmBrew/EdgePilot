package models

import "gorm.io/gorm"

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
		{
			name:  "idx_device_groups_tenant_name_unique",
			table: "device_groups",
			expr:  "tenant_id, name",
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
