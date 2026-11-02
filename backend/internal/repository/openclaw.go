package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// OpenClawRepository persists OpenClaw AI agent instances with their channel
// integrations and backups. Instances are soft-deleted through their own
// deleted_at column.
type OpenClawRepository interface {
	// List returns live instances (newest first) with integrations; at most
	// three backups, the newest across the result, are preloaded.
	List(ctx context.Context, ownerID *uuid.UUID) ([]models.OpenClawInstance, error)
	// FindByID returns a live instance with integrations and backups (newest
	// first).
	FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.OpenClawInstance, error)
	CountByHostname(ctx context.Context, hostname string) (int64, error)
	Create(ctx context.Context, inst *models.OpenClawInstance) error
	Save(ctx context.Context, inst *models.OpenClawInstance) error
	// SoftDelete sets deleted_at on inst.
	SoftDelete(ctx context.Context, inst *models.OpenClawInstance, at *time.Time) error
	CreateIntegration(ctx context.Context, integration *models.OpenClawIntegration) error
	DeleteIntegration(ctx context.Context, instanceID, integrationID uuid.UUID) error
	CreateBackup(ctx context.Context, backup *models.OpenClawBackup) error
}

type openClawRepository struct{ base }

var _ OpenClawRepository = (*openClawRepository)(nil)

func NewOpenClawRepository(db *gorm.DB) OpenClawRepository {
	return &openClawRepository{base{db}}
}

func (r *openClawRepository) List(ctx context.Context, ownerID *uuid.UUID) ([]models.OpenClawInstance, error) {
	var list []models.OpenClawInstance
	// Keep the 3 newest backups per instance (a Limit inside Preload would
	// cap the whole IN (...) query instead).
	query := r.conn(ctx).Preload("Integrations").Preload("Backups", func(db *gorm.DB) *gorm.DB {
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

func (r *openClawRepository) FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.OpenClawInstance, error) {
	var item models.OpenClawInstance
	query := r.conn(ctx).Preload("Integrations").Preload("Backups", func(db *gorm.DB) *gorm.DB {
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

func (r *openClawRepository) CountByHostname(ctx context.Context, hostname string) (int64, error) {
	var count int64
	err := r.conn(ctx).Model(&models.OpenClawInstance{}).Where("hostname = ? AND deleted_at IS NULL", hostname).Count(&count).Error
	return count, err
}

func (r *openClawRepository) Create(ctx context.Context, inst *models.OpenClawInstance) error {
	return r.conn(ctx).Create(inst).Error
}

func (r *openClawRepository) Save(ctx context.Context, inst *models.OpenClawInstance) error {
	return r.conn(ctx).Save(inst).Error
}

func (r *openClawRepository) SoftDelete(ctx context.Context, inst *models.OpenClawInstance, at *time.Time) error {
	return r.conn(ctx).Model(inst).Update("deleted_at", at).Error
}

func (r *openClawRepository) CreateIntegration(ctx context.Context, integration *models.OpenClawIntegration) error {
	return r.conn(ctx).Create(integration).Error
}

func (r *openClawRepository) DeleteIntegration(ctx context.Context, instanceID, integrationID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND openclaw_instance_id = ?", integrationID, instanceID).Delete(&models.OpenClawIntegration{}).Error
}

func (r *openClawRepository) CreateBackup(ctx context.Context, backup *models.OpenClawBackup) error {
	return r.conn(ctx).Create(backup).Error
}
