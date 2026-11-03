package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Monitored check types.
const (
	CheckTypeInstanceHealth = "instance.health"
	CheckTypeAppHealth      = "app.health"
	CheckTypeGameHealth     = "game_server.health"
	CheckTypeAndroidHealth  = "android.health"
	CheckTypeDatabaseHealth = "database.health"
	CheckTypeBackupFresh    = "backup.freshness"
	CheckTypeQuotaUsage     = "quota.usage"
)

// Check result statuses.
const (
	CheckStatusOK       = "ok"
	CheckStatusWarning  = "warning"
	CheckStatusCritical = "critical"
	CheckStatusUnknown  = "unknown"
)

// InfraCheck is a user-configured recurring infrastructure check. Checks are
// claimed from Postgres (FOR UPDATE SKIP LOCKED), so several API replicas can
// run them without duplicates.
type InfraCheck struct {
	ID                  uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID             uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name                string     `gorm:"type:varchar(120);not null" json:"name"`
	ResourceType        string     `gorm:"type:varchar(32);not null;default:''" json:"resource_type"`
	ResourceID          uuid.UUID  `gorm:"type:uuid;not null" json:"resource_id"`
	CheckType           string     `gorm:"type:varchar(32);not null" json:"check_type"`
	IntervalSeconds     int        `gorm:"not null;default:300" json:"interval_seconds"`
	Enabled             bool       `gorm:"not null" json:"enabled"`
	Status              string     `gorm:"type:varchar(16);not null;default:'unknown'" json:"status"`
	LastResult          string     `gorm:"type:text;not null;default:'{}'" json:"-"`
	ConsecutiveFailures int        `gorm:"not null;default:0" json:"consecutive_failures"`
	LastRunAt           *time.Time `json:"last_run_at"`
	NextRunAt           time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP" json:"next_run_at"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

func (c *InfraCheck) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (InfraCheck) TableName() string { return "infra_checks" }
