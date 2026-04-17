package models

import (
	"time"

	"gorm.io/gorm"
)

// BaseModel provides common fields for most models
type BaseModel struct {
	ID        string         `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CreatedAt time.Time      `gorm:"autoCreateTime"`
	UpdatedAt time.Time      `gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `gorm:"index"`
}
