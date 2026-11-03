package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// InfraTemplate is a reusable, parameterized infrastructure specification.
// Specifications are stored as canonical JSON; parameters describe the values
// a user supplies when deploying the template. Templates never store secrets.
type InfraTemplate struct {
	ID               uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID          uuid.UUID  `gorm:"type:uuid;not null;index" json:"owner_id"`
	Name             string     `gorm:"type:varchar(120);not null" json:"name"`
	Description      string     `gorm:"type:text;not null;default:''" json:"description"`
	Intent           string     `gorm:"type:varchar(64);not null;default:''" json:"intent"`
	Specification    string     `gorm:"type:text;not null" json:"-"`
	Parameters       string     `gorm:"type:text;not null;default:'[]'" json:"-"`
	Version          int        `gorm:"not null;default:1" json:"version"`
	IsPublic         bool       `gorm:"not null;default:false" json:"is_public"`
	UsageCount       int64      `gorm:"not null;default:0" json:"usage_count"`
	ValidationStatus string     `gorm:"type:varchar(16);not null;default:'valid'" json:"validation_status"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

func (t *InfraTemplate) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (InfraTemplate) TableName() string { return "infra_templates" }

// Template parameter types.
const (
	TemplateParamString = "string"
	TemplateParamInt    = "integer"
	TemplateParamBool   = "boolean"
	TemplateParamEnum   = "enum"
)
