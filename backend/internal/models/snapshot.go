package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type InstanceSnapshot struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	InstanceID  uuid.UUID `gorm:"type:uuid;index;not null" json:"instance_id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IncludesRAM bool      `gorm:"not null;default:false" json:"includes_ram"`
	CreatedAt   time.Time `json:"created_at"`

	Instance *Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (s *InstanceSnapshot) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = time.Now()
	}
	return nil
}

func (InstanceSnapshot) TableName() string {
	return "instance_snapshots"
}
