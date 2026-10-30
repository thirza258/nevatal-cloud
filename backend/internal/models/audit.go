package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AuditLog struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ActorUserID  *uuid.UUID `gorm:"type:uuid;index" json:"actor_user_id,omitempty"`
	Action       string     `gorm:"type:varchar(64);index;not null" json:"action"`
	ResourceType string     `gorm:"type:varchar(64);not null" json:"resource_type"`
	ResourceID   *uuid.UUID `gorm:"type:uuid" json:"resource_id,omitempty"`
	Result       string     `gorm:"type:varchar(32);not null" json:"result"` // success, failure
	IPAddress    *string    `gorm:"type:varchar(45)" json:"ip_address,omitempty"`
	Metadata     string     `gorm:"type:text;default:'{}'" json:"metadata"` // JSON string representation
	CreatedAt    time.Time  `gorm:"index" json:"created_at"`

	Actor *User `gorm:"foreignKey:ActorUserID" json:"actor,omitempty"`
}

func (a *AuditLog) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.CreatedAt.IsZero() {
		a.CreatedAt = time.Now()
	}
	return nil
}
