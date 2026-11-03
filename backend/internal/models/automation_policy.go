package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Automation policy triggers and actions.
const (
	PolicyTriggerExecutionFailed    = "execution.failed"
	PolicyTriggerExecutionCompleted = "execution.completed"
	PolicyTriggerCheckFailed        = "check.failed"
	PolicyTriggerAdvisor            = "advisor.recommendation"
)

const (
	PolicyActionNotify     = "notify"
	PolicyActionCreatePlan = "create_plan"
)

// AutomationPolicy is a user-owned, deterministic rule evaluated on
// platform events. Policies can notify or propose a plan; they can never
// execute infrastructure changes directly.
type AutomationPolicy struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID         uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name            string     `gorm:"type:varchar(120);not null" json:"name"`
	Trigger         string     `gorm:"type:varchar(48);not null" json:"trigger"`
	Conditions      string     `gorm:"type:text;not null;default:'{}'" json:"-"`
	Action          string     `gorm:"type:varchar(32);not null" json:"action"`
	ActionConfig    string     `gorm:"type:text;not null;default:'{}'" json:"-"`
	Enabled         bool       `gorm:"not null" json:"enabled"`
	LastTriggeredAt *time.Time `json:"last_triggered_at"`
	TriggerCount    int64      `gorm:"not null;default:0" json:"trigger_count"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (p *AutomationPolicy) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (AutomationPolicy) TableName() string { return "automation_policies" }
