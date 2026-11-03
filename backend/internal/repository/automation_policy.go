package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// AutomationPolicyRepository persists user automation policies.
type AutomationPolicyRepository interface {
	ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.AutomationPolicy, error)
	FindByID(ctx context.Context, id, ownerID uuid.UUID) (*models.AutomationPolicy, error)
	Create(ctx context.Context, p *models.AutomationPolicy) error
	Save(ctx context.Context, p *models.AutomationPolicy) error
	Delete(ctx context.Context, id, ownerID uuid.UUID) error
	ListEnabledForTrigger(ctx context.Context, ownerID uuid.UUID, trigger string) ([]models.AutomationPolicy, error)
	MarkTriggered(ctx context.Context, id uuid.UUID) error
}

type automationPolicyRepository struct{ base }

var _ AutomationPolicyRepository = (*automationPolicyRepository)(nil)

func NewAutomationPolicyRepository(db *gorm.DB) AutomationPolicyRepository {
	return &automationPolicyRepository{base{db}}
}

func (r *automationPolicyRepository) ListByOwner(ctx context.Context, ownerID uuid.UUID) ([]models.AutomationPolicy, error) {
	var out []models.AutomationPolicy
	err := r.conn(ctx).Where("owner_id = ?", ownerID).Order("created_at desc").Find(&out).Error
	return out, err
}

func (r *automationPolicyRepository) FindByID(ctx context.Context, id, ownerID uuid.UUID) (*models.AutomationPolicy, error) {
	var p models.AutomationPolicy
	err := r.conn(ctx).Where("id = ? AND owner_id = ?", id, ownerID).First(&p).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &p, nil
}

func (r *automationPolicyRepository) Create(ctx context.Context, p *models.AutomationPolicy) error {
	return r.conn(ctx).Create(p).Error
}

func (r *automationPolicyRepository) Save(ctx context.Context, p *models.AutomationPolicy) error {
	return r.conn(ctx).Save(p).Error
}

func (r *automationPolicyRepository) Delete(ctx context.Context, id, ownerID uuid.UUID) error {
	res := r.conn(ctx).Where("id = ? AND owner_id = ?", id, ownerID).Delete(&models.AutomationPolicy{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *automationPolicyRepository) ListEnabledForTrigger(ctx context.Context, ownerID uuid.UUID, trigger string) ([]models.AutomationPolicy, error) {
	var out []models.AutomationPolicy
	err := r.conn(ctx).
		Where("owner_id = ? AND enabled = true AND trigger = ?", ownerID, trigger).
		Order("created_at asc").Find(&out).Error
	return out, err
}

func (r *automationPolicyRepository) MarkTriggered(ctx context.Context, id uuid.UUID) error {
	return r.conn(ctx).Model(&models.AutomationPolicy{}).Where("id = ?", id).Updates(map[string]any{
		"last_triggered_at": gorm.Expr("CURRENT_TIMESTAMP"),
		"trigger_count":     gorm.Expr("trigger_count + 1"),
	}).Error
}
