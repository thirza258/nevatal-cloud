package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type SSHKey struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID     uuid.UUID `gorm:"type:uuid;index;not null" json:"owner_id"`
	Name        string    `gorm:"type:varchar(100);not null" json:"name"`
	PublicKey   string    `gorm:"type:text;not null" json:"public_key"`
	Fingerprint string    `gorm:"type:varchar(100);not null" json:"fingerprint"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func (k *SSHKey) BeforeCreate(tx *gorm.DB) error {
	if k.ID == uuid.Nil {
		k.ID = uuid.New()
	}
	return nil
}

func (SSHKey) TableName() string {
	return "ssh_keys"
}
