package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OSImage struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name           string    `gorm:"type:varchar(100);not null" json:"name"`
	Version        string    `gorm:"type:varchar(50);not null" json:"version"`
	Architecture   string    `gorm:"type:varchar(32);not null;default:'x86_64'" json:"architecture"`
	Category       string    `gorm:"type:varchar(50);not null;default:'Linux'" json:"category"` // Linux, Windows, BSD, Other
	Description    string    `gorm:"type:text" json:"description"`
	Filename       string    `gorm:"type:varchar(255);not null" json:"filename"`
	FileSizeBytes  int64     `gorm:"not null" json:"file_size_bytes"`
	SHA256         string    `gorm:"type:varchar(64);not null" json:"sha256"`
	ProxmoxStorage string    `gorm:"type:varchar(100);not null;default:'local'" json:"proxmox_storage"`
	ProxmoxVolume  string    `gorm:"type:varchar(255);not null" json:"proxmox_volume"`
	Visibility     string    `gorm:"type:varchar(32);not null;default:'published'" json:"visibility"` // published, unpublished, draft
	Status         string    `gorm:"type:varchar(32);not null;default:'available'" json:"status"` // available, uploading, failed
	CreatedByID    *uuid.UUID `gorm:"type:uuid" json:"created_by_id,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (o *OSImage) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (OSImage) TableName() string {
	return "os_images"
}
