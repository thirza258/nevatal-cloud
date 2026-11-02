package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// UserRepository persists platform accounts (users).
type UserRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.User, error)
	// FindByIDWithQuota returns a user with Quota preloaded.
	FindByIDWithQuota(ctx context.Context, id uuid.UUID) (*models.User, error)
	FindByEmail(ctx context.Context, email string) (*models.User, error)
	CountByEmail(ctx context.Context, email string) (int64, error)
	Count(ctx context.Context) (int64, error)
	// ListWithQuota returns every user (newest first) with Quota preloaded.
	ListWithQuota(ctx context.Context) ([]models.User, error)

	Create(ctx context.Context, user *models.User) error
	// Save writes every column of user.
	Save(ctx context.Context, user *models.User) error
	// UpdateLastLogin sets last_login_at on user.
	UpdateLastLogin(ctx context.Context, user *models.User, at *time.Time) error
	// UpdateStatus sets the status of the user with the given id and reports
	// how many rows changed.
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) (int64, error)
}

type userRepository struct{ base }

var _ UserRepository = (*userRepository)(nil)

func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{base{db}}
}

func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.conn(ctx).First(&user, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &user, nil
}

func (r *userRepository) FindByIDWithQuota(ctx context.Context, id uuid.UUID) (*models.User, error) {
	var user models.User
	if err := r.conn(ctx).Preload("Quota").First(&user, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &user, nil
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	if err := r.conn(ctx).Where("email = ?", email).First(&user).Error; err != nil {
		return nil, mapErr(err)
	}
	return &user, nil
}

func (r *userRepository) CountByEmail(ctx context.Context, email string) (int64, error) {
	var count int64
	err := r.conn(ctx).Model(&models.User{}).Where("email = ?", email).Count(&count).Error
	return count, err
}

func (r *userRepository) Count(ctx context.Context) (int64, error) {
	var count int64
	err := r.conn(ctx).Model(&models.User{}).Count(&count).Error
	return count, err
}

func (r *userRepository) ListWithQuota(ctx context.Context) ([]models.User, error) {
	var users []models.User
	err := r.conn(ctx).Preload("Quota").Order("created_at desc").Find(&users).Error
	return users, err
}

func (r *userRepository) Create(ctx context.Context, user *models.User) error {
	return r.conn(ctx).Create(user).Error
}

func (r *userRepository) Save(ctx context.Context, user *models.User) error {
	return r.conn(ctx).Save(user).Error
}

func (r *userRepository) UpdateLastLogin(ctx context.Context, user *models.User, at *time.Time) error {
	return r.conn(ctx).Model(user).Update("last_login_at", at).Error
}

func (r *userRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) (int64, error) {
	res := r.conn(ctx).Model(&models.User{}).Where("id = ?", id).Update("status", status)
	return res.RowsAffected, res.Error
}
