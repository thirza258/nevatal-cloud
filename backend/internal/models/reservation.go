package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ReservationActive    = "active"
	ReservationCommitted = "committed"
	ReservationReleased  = "released"
)

type ResourceReservation struct {
	ID                uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID           uuid.UUID  `gorm:"type:uuid;index;not null" json:"owner_id"`
	JobID             uuid.UUID  `gorm:"type:uuid;not null" json:"job_id"`
	NodeID            *uuid.UUID `gorm:"type:uuid" json:"node_id,omitempty"`
	ReservedVCPUs     int        `gorm:"column:reserved_vcpus;not null" json:"reserved_vcpus"`
	ReservedMemoryMB  int        `gorm:"not null" json:"reserved_memory_mb"`
	ReservedStorageGB int        `gorm:"not null" json:"reserved_storage_gb"`
	Status            string     `gorm:"type:varchar(32);index;not null;default:'active'" json:"status"`
	CreatedAt         time.Time  `json:"created_at"`
	ExpiresAt         *time.Time `json:"expires_at,omitempty"`
	ReleasedAt        *time.Time `json:"released_at,omitempty"`
}

func (r *ResourceReservation) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (ResourceReservation) TableName() string {
	return "resource_reservations"
}
