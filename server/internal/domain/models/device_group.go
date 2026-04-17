package models

// DeviceGroup represents a logical grouping of devices
type DeviceGroup struct {
	BaseModel
	TenantID    string   `gorm:"type:uuid;not null;index:idx_device_groups_tenant" json:"tenant_id"`
	Name        string   `gorm:"size:128;not null" json:"name"`
	Description string   `gorm:"size:512" json:"description,omitempty"`
	Tenant      Tenant   `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	Devices     []Device `gorm:"foreignKey:GroupID" json:"devices,omitempty"`
}

// TableName overrides the default table name
func (DeviceGroup) TableName() string {
	return "device_groups"
}
