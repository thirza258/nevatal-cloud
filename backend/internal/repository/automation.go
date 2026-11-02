package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// AutomationRepository persists managed n8n instances and their backups.
// Instances are soft-deleted through their own deleted_at column.
type AutomationRepository interface {
	// List returns live instances (newest first); at most three backups,
	// the newest across the result, are preloaded (preload LIMIT semantics).
	List(ctx context.Context, ownerID *uuid.UUID) ([]models.AutomationInstance, error)
	// FindByID returns a live instance with its backups (newest first).
	FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.AutomationInstance, error)
	CountByHostname(ctx context.Context, hostname string) (int64, error)
	Create(ctx context.Context, inst *models.AutomationInstance) error
	Save(ctx context.Context, inst *models.AutomationInstance) error
	// SoftDelete sets deleted_at on inst.
	SoftDelete(ctx context.Context, inst *models.AutomationInstance, at *time.Time) error
	CreateBackup(ctx context.Context, backup *models.AutomationBackup) error
}

type automationRepository struct{ base }

var _ AutomationRepository = (*automationRepository)(nil)

func NewAutomationRepository(db *gorm.DB) AutomationRepository {
	return &automationRepository{base{db}}
}

func (r *automationRepository) List(ctx context.Context, ownerID *uuid.UUID) ([]models.AutomationInstance, error) {
	var list []models.AutomationInstance
	// Keep the 3 newest backups per instance (a Limit inside Preload would
	// cap the whole IN (...) query instead).
	query := r.conn(ctx).Preload("Backups", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at desc")
	}).Where("deleted_at IS NULL")

	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}

	err := query.Order("created_at desc").Find(&list).Error
	for i := range list {
		if len(list[i].Backups) > 3 {
			list[i].Backups = list[i].Backups[:3]
		}
	}
	return list, err
}

func (r *automationRepository) FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.AutomationInstance, error) {
	var item models.AutomationInstance
	query := r.conn(ctx).Preload("Backups", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at desc")
	}).Where("id = ? AND deleted_at IS NULL", id)

	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}

	if err := query.First(&item).Error; err != nil {
		return nil, mapErr(err)
	}
	return &item, nil
}

func (r *automationRepository) CountByHostname(ctx context.Context, hostname string) (int64, error) {
	var count int64
	err := r.conn(ctx).Model(&models.AutomationInstance{}).Where("hostname = ? AND deleted_at IS NULL", hostname).Count(&count).Error
	return count, err
}

func (r *automationRepository) Create(ctx context.Context, inst *models.AutomationInstance) error {
	return r.conn(ctx).Create(inst).Error
}

func (r *automationRepository) Save(ctx context.Context, inst *models.AutomationInstance) error {
	return r.conn(ctx).Save(inst).Error
}

func (r *automationRepository) SoftDelete(ctx context.Context, inst *models.AutomationInstance, at *time.Time) error {
	return r.conn(ctx).Model(inst).Update("deleted_at", at).Error
}

func (r *automationRepository) CreateBackup(ctx context.Context, backup *models.AutomationBackup) error {
	return r.conn(ctx).Create(backup).Error
}
