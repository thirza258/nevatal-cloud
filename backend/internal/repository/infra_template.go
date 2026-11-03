package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// InfraTemplateRepository persists reusable infrastructure templates. A
// template is visible to its owner and, when public, to every user.
type InfraTemplateRepository interface {
	List(ctx context.Context, ownerID uuid.UUID, includePublic bool) ([]models.InfraTemplate, error)
	FindVisible(ctx context.Context, id, userID uuid.UUID) (*models.InfraTemplate, error)
	Create(ctx context.Context, t *models.InfraTemplate) error
	Save(ctx context.Context, t *models.InfraTemplate) error
	SoftDelete(ctx context.Context, id, ownerID uuid.UUID) error
	IncrementUsage(ctx context.Context, id uuid.UUID) error
}

type infraTemplateRepository struct{ base }

var _ InfraTemplateRepository = (*infraTemplateRepository)(nil)

func NewInfraTemplateRepository(db *gorm.DB) InfraTemplateRepository {
	return &infraTemplateRepository{base{db}}
}

func (r *infraTemplateRepository) List(ctx context.Context, ownerID uuid.UUID, includePublic bool) ([]models.InfraTemplate, error) {
	q := r.conn(ctx).Where("deleted_at IS NULL")
	if includePublic {
		q = q.Where("owner_id = ? OR is_public = true", ownerID)
	} else {
		q = q.Where("owner_id = ?", ownerID)
	}
	var out []models.InfraTemplate
	err := q.Order("updated_at desc").Find(&out).Error
	return out, err
}

func (r *infraTemplateRepository) FindVisible(ctx context.Context, id, userID uuid.UUID) (*models.InfraTemplate, error) {
	var t models.InfraTemplate
	err := r.conn(ctx).
		Where("id = ? AND deleted_at IS NULL AND (owner_id = ? OR is_public = true)", id, userID).
		First(&t).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &t, nil
}

func (r *infraTemplateRepository) Create(ctx context.Context, t *models.InfraTemplate) error {
	err := r.conn(ctx).Create(t).Error
	if uniqueViolation(r.db, err) {
		return ErrDuplicate
	}
	return err
}

func (r *infraTemplateRepository) Save(ctx context.Context, t *models.InfraTemplate) error {
	return r.conn(ctx).Save(t).Error
}

func (r *infraTemplateRepository) SoftDelete(ctx context.Context, id, ownerID uuid.UUID) error {
	res := r.conn(ctx).Model(&models.InfraTemplate{}).
		Where("id = ? AND owner_id = ? AND deleted_at IS NULL", id, ownerID).
		Update("deleted_at", gorm.Expr("CURRENT_TIMESTAMP"))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *infraTemplateRepository) IncrementUsage(ctx context.Context, id uuid.UUID) error {
	return r.conn(ctx).Model(&models.InfraTemplate{}).Where("id = ?", id).
		UpdateColumn("usage_count", gorm.Expr("usage_count + 1")).Error
}
