package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ResourcePlan struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	VCPUs       int       `gorm:"column:vcpus;not null" json:"vcpus"`
	MemoryMB    int       `gorm:"not null" json:"memory_mb"`
	DiskGB      int       `gorm:"not null" json:"disk_gb"`
	Enabled     bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (p *ResourcePlan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (ResourcePlan) TableName() string {
	return "resource_plans"
}
