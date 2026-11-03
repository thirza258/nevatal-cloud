package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Managed database statuses.
const (
	DatabaseStatusProvisioning = "provisioning"
	DatabaseStatusRunning      = "running"
	DatabaseStatusStopped      = "stopped"
	DatabaseStatusFailed       = "failed"
	DatabaseStatusDeleting     = "deleting"
)

// Supported managed database engines.
const (
	DatabaseEnginePostgres = "postgresql"
	DatabaseEngineMySQL    = "mysql"
	DatabaseEngineRedis    = "redis"
	DatabaseEngineMongo    = "mongodb"
)

// ManagedDatabase is a database provisioned for one tenant. Credentials are
// never stored in plaintext: PasswordCiphertext is AES-GCM encrypted and only
// decrypted by the owner-facing credentials endpoint.
type ManagedDatabase struct {
	ID                 uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID            uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name               string     `gorm:"type:varchar(100);not null" json:"name"`
	Engine             string     `gorm:"type:varchar(32);not null" json:"engine"`
	EngineVersion      string     `gorm:"type:varchar(32);not null;default:''" json:"engine_version"`
	Status             string     `gorm:"type:varchar(32);not null;default:'provisioning'" json:"status"`
	PlanName           string     `gorm:"type:varchar(32);not null;default:'starter'" json:"plan_name"`
	VCPUs              int        `gorm:"column:vcpus;not null;default:1" json:"vcpus"`
	MemoryMB           int        `gorm:"not null;default:1024" json:"memory_mb"`
	StorageGB          int        `gorm:"not null;default:10" json:"storage_gb"`
	Host               string     `gorm:"type:varchar(255);not null;default:''" json:"host"`
	Port               int        `gorm:"not null;default:5432" json:"port"`
	DatabaseName       string     `gorm:"type:varchar(100);not null;default:''" json:"database_name"`
	Username           string     `gorm:"type:varchar(100);not null;default:''" json:"username"`
	PasswordCiphertext string     `gorm:"type:text;not null;default:''" json:"-"`
	AutoBackup         bool       `gorm:"not null" json:"auto_backup"`
	LastBackupAt       *time.Time `json:"last_backup_at"`
	// NextBackupAt is when the backup scheduler may take the next automatic
	// backup. Claiming a database advances it, so replicas never duplicate a
	// backup; nil means the next scan claims it.
	NextBackupAt        *time.Time `json:"next_backup_at"`
	LastHealthCheckAt   *time.Time `json:"last_health_check_at"`
	CPUPercent          float64    `gorm:"not null;default:0" json:"cpu_percent"`
	MemoryUsagePercent  float64    `gorm:"not null;default:0" json:"memory_usage_percent"`
	StorageUsagePercent float64    `gorm:"not null;default:0" json:"storage_usage_percent"`
	ConnectionCount     int        `gorm:"not null;default:0" json:"connection_count"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	DeletedAt           *time.Time `json:"deleted_at,omitempty"`
}

func (d *ManagedDatabase) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}

func (ManagedDatabase) TableName() string { return "managed_databases" }

// ManagedDatabaseBackup is a logical backup of a managed database.
type ManagedDatabaseBackup struct {
	ID              uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	DatabaseID      uuid.UUID `gorm:"type:uuid;not null;index" json:"database_id"`
	Name            string    `gorm:"type:varchar(150);not null" json:"name"`
	BackupReference string    `gorm:"type:varchar(255);not null" json:"backup_reference"`
	SizeBytes       int64     `gorm:"not null;default:0" json:"size_bytes"`
	Status          string    `gorm:"type:varchar(32);not null;default:'completed'" json:"status"`
	CreatedAt       time.Time `json:"created_at"`
}

func (b *ManagedDatabaseBackup) BeforeCreate(tx *gorm.DB) error {
	if b.ID == uuid.Nil {
		b.ID = uuid.New()
	}
	return nil
}

func (ManagedDatabaseBackup) TableName() string { return "managed_database_backups" }
