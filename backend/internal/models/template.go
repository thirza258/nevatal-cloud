package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OSTemplate struct {
	ID                  uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name                string    `gorm:"type:varchar(100);not null" json:"name"`
	Distribution        string    `gorm:"type:varchar(50);not null" json:"distribution"` // ubuntu, debian, etc.
	Version             string    `gorm:"type:varchar(50);not null" json:"version"`
	Description         string    `gorm:"type:text" json:"description"`
	ProxmoxTemplateVMID int       `gorm:"column:proxmox_template_vmid;not null" json:"proxmox_template_vmid"`
	Architecture        string    `gorm:"type:varchar(32);not null;default:'x86_64'" json:"architecture"`
	CloudInitSupported  bool      `gorm:"not null;default:true" json:"cloud_init_supported"`
	DefaultUsername     string    `gorm:"type:varchar(50);not null;default:'ubuntu'" json:"default_username"`
	Enabled             bool      `gorm:"not null;default:true" json:"enabled"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (t *OSTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (OSTemplate) TableName() string {
	return "os_templates"
}
