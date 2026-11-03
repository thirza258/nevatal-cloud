package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Notification types and severities.
const (
	NotificationTypeExecution = "execution"
	NotificationTypeCheck     = "check"
	NotificationTypeAdvisor   = "advisor"
	NotificationTypeSystem    = "system"
)

const (
	NotificationSeverityInfo     = "info"
	NotificationSeverityWarning  = "warning"
	NotificationSeverityCritical = "critical"
)

// Notification is an in-app notification. DedupeKey (when set) makes
// creation idempotent: the same key is delivered at most once per user.
type Notification struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Type         string     `gorm:"type:varchar(32);not null" json:"type"`
	Severity     string     `gorm:"type:varchar(16);not null;default:'info'" json:"severity"`
	Title        string     `gorm:"type:varchar(255);not null" json:"title"`
	Message      string     `gorm:"type:text;not null;default:''" json:"message"`
	ResourceType string     `gorm:"type:varchar(32);not null;default:''" json:"resource_type"`
	ResourceID   *uuid.UUID `gorm:"type:uuid" json:"resource_id"`
	ActionURL    string     `gorm:"type:varchar(255);not null;default:''" json:"action_url"`
	DedupeKey    string     `gorm:"type:varchar(255);not null;default:''" json:"-"`
	ReadAt       *time.Time `json:"read_at"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == uuid.Nil {
		n.ID = uuid.New()
	}
	return nil
}

func (Notification) TableName() string { return "notifications" }
