package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Domain publishes one port of an instance at https://<Hostname>
// (<Subdomain>.<PUBLIC_DOMAIN_ROOT>) through the nevatal-edge proxy.
//
// A domain is active while ReleasedAt is nil. Releasing keeps the row so the
// hostname can be quarantined for other accounts. The bool columns carry no
// gorm default tag on purpose: GORM would skip a false value on insert and
// let the database default (true) win.
type Domain struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID        uuid.UUID  `gorm:"type:uuid;index;not null" json:"owner_id"`
	InstanceID     *uuid.UUID `gorm:"type:uuid;index" json:"instance_id"`
	Subdomain      string     `gorm:"type:varchar(63);not null" json:"subdomain"`
	Hostname       string     `gorm:"type:varchar(253);not null" json:"hostname"`
	TargetPort     int        `gorm:"not null" json:"target_port"`
	Enabled        bool       `gorm:"not null" json:"enabled"`
	AdminDisabled  bool       `gorm:"not null" json:"admin_disabled"`
	DisabledReason string     `gorm:"type:varchar(255);not null" json:"disabled_reason"`
	ReleasedAt     *time.Time `json:"released_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`

	// Associations
	Owner    *User     `gorm:"foreignKey:OwnerID" json:"owner,omitempty"`
	Instance *Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (d *Domain) BeforeCreate(tx *gorm.DB) error {
	if d.ID == uuid.Nil {
		d.ID = uuid.New()
	}
	return nil
}
