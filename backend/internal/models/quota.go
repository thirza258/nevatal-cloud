package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type ResourceQuota struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;unique;not null" json:"user_id"`
	MaxVCPUs     int       `gorm:"column:max_vcpus;not null;default:4" json:"max_vcpus"`
	MaxMemoryMB  int       `gorm:"not null;default:8192" json:"max_memory_mb"`
	MaxStorageGB int       `gorm:"not null;default:100" json:"max_storage_gb"`
	MaxInstances int       `gorm:"not null;default:3" json:"max_instances"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (q *ResourceQuota) BeforeCreate(tx *gorm.DB) error {
	if q.ID == uuid.Nil {
		q.ID = uuid.New()
	}
	return nil
}

func (ResourceQuota) TableName() string {
	return "resource_quotas"
}

type QuotaUsage struct {
	MaxVCPUs     int `json:"max_vcpus"`
	UsedVCPUs    int `json:"used_vcpus"`
	MaxMemoryMB  int `json:"max_memory_mb"`
	UsedMemoryMB int `json:"used_memory_mb"`
	MaxStorageGB int `json:"max_storage_gb"`
	UsedStorageGB int `json:"used_storage_gb"`
	MaxInstances int `json:"max_instances"`
	UsedInstances int `json:"used_instances"`
}
