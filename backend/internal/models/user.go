package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	RoleCustomer = "customer"
	RoleAdmin    = "admin"

	UserStatusActive    = "active"
	UserStatusSuspended = "suspended"
)

type User struct {
	ID              uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Email           string         `gorm:"type:varchar(255);unique;not null" json:"email"`
	PasswordHash    string         `gorm:"type:varchar(255);not null" json:"-"`
	DisplayName     string         `gorm:"type:varchar(100);not null" json:"display_name"`
	Role            string         `gorm:"type:varchar(32);not null;default:'customer'" json:"role"`
	Status          string         `gorm:"type:varchar(32);not null;default:'active'" json:"status"`
	EmailVerifiedAt *time.Time     `json:"email_verified_at"`
	LastLoginAt     *time.Time     `json:"last_login_at"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	Quota           *ResourceQuota `gorm:"foreignKey:UserID" json:"quota,omitempty"`
}

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
