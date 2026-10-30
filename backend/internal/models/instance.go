package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	StateQueued       = "queued"
	StateProvisioning = "provisioning"
	StateStarting     = "starting"
	StateRunning      = "running"
	StateStopping     = "stopping"
	StateStopped      = "stopped"
	StateRestarting   = "restarting"
	StateDeleting     = "deleting"
	StateFailed       = "failed"
	StateUnknown      = "unknown"
)

type Instance struct {
	ID           uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID      uuid.UUID      `gorm:"type:uuid;index;not null" json:"owner_id"`
	NodeID       uuid.UUID      `gorm:"type:uuid;index;not null" json:"node_id"`
	OSTemplateID uuid.UUID      `gorm:"type:uuid;not null" json:"os_template_id"`
	ProxmoxVMID  int            `gorm:"column:proxmox_vmid;index;not null" json:"proxmox_vmid"`
	Name         string         `gorm:"type:varchar(100);not null" json:"name"`
	Hostname     string         `gorm:"type:varchar(100);not null" json:"hostname"`
	Status       string         `gorm:"type:varchar(32);index;not null;default:'queued'" json:"status"`
	VCPUs        int            `gorm:"column:vcpus;not null" json:"vcpus"`
	MemoryMB     int            `gorm:"not null" json:"memory_mb"`
	DiskGB       int            `gorm:"not null" json:"disk_gb"`
	PrivateIP    *string        `gorm:"type:varchar(45)" json:"private_ip"`
	PublicIP     *string        `gorm:"type:varchar(45)" json:"public_ip"`
	SSHUsername  string         `gorm:"type:varchar(50);not null;default:'ubuntu'" json:"ssh_username"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`

	// Associations
	Owner      *User       `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Node       *Node       `gorm:"foreignKey:NodeID" json:"node,omitempty"`
	OSTemplate *OSTemplate `gorm:"foreignKey:OSTemplateID" json:"os_template,omitempty"`
}

func (i *Instance) BeforeCreate(tx *gorm.DB) error {
	if i.ID == uuid.Nil {
		i.ID = uuid.New()
	}
	return nil
}
