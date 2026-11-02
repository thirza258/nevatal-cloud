package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type OpenClawInstance struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID           uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name              string     `gorm:"type:varchar(100);not null" json:"name"`
	Version           string     `gorm:"type:varchar(50);not null;default:'2.4.0'" json:"version"`
	Status            string     `gorm:"type:varchar(32);not null;default:'running'" json:"status"` // provisioning, running, stopped, upgrading, failed
	Hostname          string     `gorm:"type:varchar(150);unique;not null" json:"hostname"`
	GatewayEndpoint   string     `gorm:"type:varchar(255);not null" json:"gateway_endpoint"`
	GatewayAuthToken  string     `gorm:"type:varchar(255);not null" json:"-"` // secret token for gateway access
	WorkspacePath     string     `gorm:"type:varchar(255);not null" json:"workspace_path"`
	AIProvider        string     `gorm:"column:ai_provider;type:varchar(50);not null;default:'openai'" json:"ai_provider"` // openai, anthropic, gemini, ollama, deepseek
	DefaultModel      string     `gorm:"type:varchar(100);not null;default:'gpt-4o'" json:"default_model"`
	EncryptedAPIKey   string     `gorm:"type:varchar(512)" json:"-"`
	MaskedAPIKey      string     `gorm:"type:varchar(50)" json:"masked_api_key"`
	CPULimit          int        `gorm:"default:2" json:"cpu_limit"`
	MemoryLimitMB     int        `gorm:"default:4096" json:"memory_limit_mb"`
	StorageLimitGB    int        `gorm:"default:30" json:"storage_limit_gb"`
	StorageUsedGB     float64    `gorm:"default:2.4" json:"storage_used_gb"`
	ActiveSkillsCount int        `gorm:"default:5" json:"active_skills_count"`
	TotalInteractions int64      `gorm:"default:0" json:"total_interactions"`
	LastHealthCheckAt *time.Time `json:"last_health_check_at"`
	CreatedAt         time.Time  `json:"created_at"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`

	Owner        *User                 `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Integrations []OpenClawIntegration `gorm:"foreignKey:OpenClawInstanceID" json:"integrations,omitempty"`
	Backups      []OpenClawBackup      `gorm:"foreignKey:OpenClawInstanceID" json:"backups,omitempty"`
}

func (o *OpenClawInstance) BeforeCreate(tx *gorm.DB) error {
	if o.ID == uuid.Nil {
		o.ID = uuid.New()
	}
	return nil
}

func (OpenClawInstance) TableName() string {
	return "openclaw_instances"
}

type OpenClawIntegration struct {
	ID                 uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OpenClawInstanceID uuid.UUID `gorm:"type:uuid;not null;index" json:"openclaw_instance_id"`
	Channel            string    `gorm:"type:varchar(50);not null" json:"channel"`     // telegram, discord, whatsapp, webhook
	Identifier         string    `gorm:"type:varchar(100);not null" json:"identifier"` // bot username or phone
	Status             string    `gorm:"type:varchar(32);default:'connected'" json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func (OpenClawIntegration) TableName() string {
	return "openclaw_channel_integrations"
}

type OpenClawBackup struct {
	ID                 uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OpenClawInstanceID uuid.UUID `gorm:"type:uuid;not null;index" json:"openclaw_instance_id"`
	Name               string    `gorm:"type:varchar(150);not null" json:"name"`
	BackupReference    string    `gorm:"type:varchar(255);not null" json:"backup_reference"`
	SizeBytes          int64     `json:"size_bytes"`
	Status             string    `gorm:"type:varchar(32);default:'completed'" json:"status"`
	CreatedAt          time.Time `json:"created_at"`
}

func (OpenClawBackup) TableName() string {
	return "openclaw_backups"
}
