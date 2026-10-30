package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Node struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name              string     `gorm:"type:varchar(100);not null" json:"name"`
	ProxmoxNodeName   string     `gorm:"type:varchar(100);unique;not null" json:"proxmox_node_name"`
	Status            string     `gorm:"type:varchar(32);not null;default:'online'" json:"status"` // online, maintenance, offline
	TotalCPUThreads   int        `gorm:"not null;default:64" json:"total_cpu_threads"`
	TotalMemoryMB     int        `gorm:"not null;default:262144" json:"total_memory_mb"`
	TotalDiskGB       int        `gorm:"not null;default:3840" json:"total_disk_gb"`
	LastHealthCheckAt *time.Time `json:"last_health_check_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
}

func (n *Node) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}
