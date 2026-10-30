package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AutomationInstance struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID             uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name                string     `gorm:"type:varchar(100);not null" json:"name"`
	N8nVersion          string     `gorm:"type:varchar(50);not null;default:'1.71.0'" json:"n8n_version"`
	Status              string     `gorm:"type:varchar(32);not null;default:'running'" json:"status"` // provisioning, running, stopped, upgrading, failed
	Hostname            string     `gorm:"type:varchar(150);unique;not null" json:"hostname"`
	DashboardURL        string     `gorm:"type:varchar(255);not null" json:"dashboard_url"`
	EncryptionKey       string     `gorm:"type:varchar(255);not null" json:"-"` // stable n8n encryption key
	DatabaseName        string     `gorm:"type:varchar(100);not null" json:"database_name"`
	CPULimit            int        `gorm:"default:2" json:"cpu_limit"`
	MemoryLimitMB       int        `gorm:"default:2048" json:"memory_limit_mb"`
	StorageLimitGB      int        `gorm:"default:20" json:"storage_limit_gb"`
	StorageUsedGB       float64    `gorm:"default:1.2" json:"storage_used_gb"`
	ActiveWorkflowsCount int       `gorm:"default:0" json:"active_workflows_count"`
	TotalExecutions     int64      `gorm:"default:0" json:"total_executions"`
	LastHealthCheckAt   *time.Time `json:"last_health_check_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`

	Owner   *User              `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Backups []AutomationBackup `gorm:"foreignKey:AutomationInstanceID" json:"backups,omitempty"`
}

func (a *AutomationInstance) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	return nil
}

func (AutomationInstance) TableName() string {
	return "automation_instances"
}

type AutomationBackup struct {
	ID                   uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	AutomationInstanceID uuid.UUID  `gorm:"type:uuid;not null;index" json:"automation_instance_id"`
	Name                 string     `gorm:"type:varchar(150);not null" json:"name"`
	BackupReference      string     `gorm:"type:varchar(255);not null" json:"backup_reference"`
	SizeBytes            int64      `json:"size_bytes"`
	Status               string     `gorm:"type:varchar(32);default:'completed'" json:"status"` // in_progress, completed, failed
	CreatedAt            time.Time  `json:"created_at"`
}

func (b *AutomationBackup) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

func (AutomationBackup) TableName() string {
	return "automation_backups"
}
