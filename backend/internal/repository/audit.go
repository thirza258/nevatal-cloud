package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// AuditRepository persists the platform audit trail (audit_logs).
type AuditRepository interface {
	Create(ctx context.Context, entry *models.AuditLog) error
	// ListRecent returns the newest entries with their actor preloaded,
	// optionally restricted to one actor.
	ListRecent(ctx context.Context, actorID *uuid.UUID, limit int) ([]models.AuditLog, error)
	// ListByActor returns the actor's newest entries without preloads.
	ListByActor(ctx context.Context, actorID uuid.UUID, limit int) ([]models.AuditLog, error)
	// ListForInstance returns the newest entries about one instance, without
	// preloads.
	ListForInstance(ctx context.Context, instanceID uuid.UUID, limit int) ([]models.AuditLog, error)
}

type auditRepository struct{ base }

var _ AuditRepository = (*auditRepository)(nil)

func NewAuditRepository(db *gorm.DB) AuditRepository {
	return &auditRepository{base{db}}
}

func (r *auditRepository) Create(ctx context.Context, entry *models.AuditLog) error {
	return r.conn(ctx).Create(entry).Error
}

func (r *auditRepository) ListRecent(ctx context.Context, actorID *uuid.UUID, limit int) ([]models.AuditLog, error) {
	query := r.conn(ctx).Model(&models.AuditLog{}).Preload("Actor").Order("created_at desc").Limit(limit)
	if actorID != nil {
		query = query.Where("actor_user_id = ?", *actorID)
	}
	var logs []models.AuditLog
	err := query.Find(&logs).Error
	return logs, err
}

func (r *auditRepository) ListForInstance(ctx context.Context, instanceID uuid.UUID, limit int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	err := r.conn(ctx).Where("resource_type = 'instance' AND resource_id = ?", instanceID).
		Order("created_at desc").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *auditRepository) ListByActor(ctx context.Context, actorID uuid.UUID, limit int) ([]models.AuditLog, error) {
	var logs []models.AuditLog
	err := r.conn(ctx).Where("actor_user_id = ?", actorID).Order("created_at desc").Limit(limit).Find(&logs).Error
	return logs, err
}
