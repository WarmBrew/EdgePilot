package models

// Plan Type constants
const (
	PlanFree       = "free"
	PlanPro        = "pro"
	PlanEnterprise = "enterprise"
)

// ValidPlans lists all allowed plan values
var ValidPlans = []string{PlanFree, PlanPro, PlanEnterprise}

// Tenant represents a multi-tenant organization
type Tenant struct {
	BaseModel
	Name string `gorm:"size:128;not null" json:"name"`
	Plan string `gorm:"size:32;not null;default:free" json:"plan"`
}

// TableName overrides the default table name
func (Tenant) TableName() string {
	return "tenants"
}
