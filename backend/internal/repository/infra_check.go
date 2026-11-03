package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// InfraCheckRepository persists scheduled infrastructure checks and claims
// due checks for the monitoring loop.
type InfraCheckRepository interface {
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.InfraCheck, error)
	FindByID(ctx context.Context, id, ownerID uuid.UUID) (*models.InfraCheck, error)
	Create(ctx context.Context, c *models.InfraCheck) error
	Save(ctx context.Context, c *models.InfraCheck) error
	Delete(ctx context.Context, id, ownerID uuid.UUID) error
	// ClaimDue atomically schedules the next run of up to limit due checks
	// and returns them. Concurrent replicas never claim the same row.
	ClaimDue(ctx context.Context, limit int) ([]models.InfraCheck, error)
	MarkRun(ctx context.Context, id uuid.UUID, status, result string, failures int) error
}

type infraCheckRepository struct{ base }

var _ InfraCheckRepository = (*infraCheckRepository)(nil)

func NewInfraCheckRepository(db *gorm.DB) InfraCheckRepository {
	return &infraCheckRepository{base{db}}
}

func (r *infraCheckRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.InfraCheck, error) {
	var out []models.InfraCheck
	err := r.conn(ctx).Where("owner_id = ?", ownerID).Order("created_at desc").Find(&out).Error
	return out, err
}

func (r *infraCheckRepository) FindByID(ctx context.Context, id, ownerID uuid.UUID) (*models.InfraCheck, error) {
	var c models.InfraCheck
	err := r.conn(ctx).Where("id = ? AND owner_id = ?", id, ownerID).First(&c).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &c, nil
}

func (r *infraCheckRepository) Create(ctx context.Context, c *models.InfraCheck) error {
	err := r.conn(ctx).Create(c).Error
	if uniqueViolation(r.db, err) {
		return ErrDuplicate
	}
	return err
}

func (r *infraCheckRepository) Save(ctx context.Context, c *models.InfraCheck) error {
	return r.conn(ctx).Save(c).Error
}

func (r *infraCheckRepository) Delete(ctx context.Context, id, ownerID uuid.UUID) error {
	res := r.conn(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.InfraCheck{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *infraCheckRepository) ClaimDue(ctx context.Context, limit int) ([]models.InfraCheck, error) {
	if limit <= 0 {
		limit = 10
	}
	var out []models.InfraCheck
	err := r.conn(ctx).Raw(`
		WITH due AS (
			SELECT id FROM infra_checks
			WHERE enabled = true AND next_run_at <= now()
			ORDER BY next_run_at
			LIMIT ?
			FOR UPDATE SKIP LOCKED
		)
		UPDATE infra_checks c
		SET next_run_at = now() + make_interval(secs => c.interval_seconds), updated_at = now()
		FROM due WHERE c.id = due.id
		RETURNING c.*`, limit).Scan(&out).Error
	return out, err
}

func (r *infraCheckRepository) MarkRun(ctx context.Context, id uuid.UUID, status, result string, failures int) error {
	now := time.Now()
	return r.conn(ctx).Model(&models.InfraCheck{}).Where("id = ?", id).Updates(map[string]any{
		"status": status, "last_result": result, "consecutive_failures": failures,
		"last_run_at": now, "updated_at": now,
	}).Error
}
