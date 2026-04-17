package models

import "time"

// Role constants for User
const (
	RoleAdmin    = "admin"
	RoleOperator = "operator"
	RoleViewer   = "viewer"
)

// ValidRoles lists all allowed role values
var ValidRoles = []string{RoleAdmin, RoleOperator, RoleViewer}

// User represents a system user within a tenant
type User struct {
	BaseModel
	TenantID  string     `gorm:"type:uuid;not null;index:idx_users_tenant" json:"tenant_id"`
	Email     string     `gorm:"size:255;uniqueIndex;not null" json:"email"`
	Password  string     `gorm:"size:255;not null" json:"-"` // bcrypt hash, excluded from JSON
	Role      string     `gorm:"size:32;not null;default:viewer" json:"role"`
	LastLogin *time.Time `gorm:"column:last_login" json:"last_login,omitempty"`
	Tenant    Tenant     `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
}

// TableName overrides the default table name
func (User) TableName() string {
	return "users"
}
