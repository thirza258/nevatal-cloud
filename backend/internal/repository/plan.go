package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// PlanRepository persists VM resource plans (resource_plans).
type PlanRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.ResourcePlan, error)
	FindEnabledByID(ctx context.Context, id uuid.UUID) (*models.ResourcePlan, error)
	// FindByRawID looks a plan up by an unvalidated id string (an invalid
	// UUID makes the query fail).
	FindByRawID(ctx context.Context, id string) (*models.ResourcePlan, error)
	FindByName(ctx context.Context, name string) (*models.ResourcePlan, error)
	// ListEnabled returns enabled plans, smallest first.
	ListEnabled(ctx context.Context) ([]models.ResourcePlan, error)
	// ListEnabledUnordered returns enabled plans in database order.
	ListEnabledUnordered(ctx context.Context) ([]models.ResourcePlan, error)
	// ListEnabledBySortOrder returns enabled plans by sort_order, then memory.
	ListEnabledBySortOrder(ctx context.Context) ([]models.ResourcePlan, error)
	// ListAll returns every plan, smallest first.
	ListAll(ctx context.Context) ([]models.ResourcePlan, error)

	Create(ctx context.Context, plan *models.ResourcePlan) error
	Save(ctx context.Context, plan *models.ResourcePlan) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

type planRepository struct{ base }

var _ PlanRepository = (*planRepository)(nil)

func NewPlanRepository(db *gorm.DB) PlanRepository {
	return &planRepository{base{db}}
}

func (r *planRepository) first(ctx context.Context, conds ...interface{}) (*models.ResourcePlan, error) {
	var plan models.ResourcePlan
	if err := r.conn(ctx).First(&plan, conds...).Error; err != nil {
		return nil, mapErr(err)
	}
	return &plan, nil
}

func (r *planRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.ResourcePlan, error) {
	return r.first(ctx, "id = ?", id)
}

func (r *planRepository) FindEnabledByID(ctx context.Context, id uuid.UUID) (*models.ResourcePlan, error) {
	return r.first(ctx, "id = ? AND enabled = true", id)
}

func (r *planRepository) FindByRawID(ctx context.Context, id string) (*models.ResourcePlan, error) {
	return r.first(ctx, "id = ?", id)
}

func (r *planRepository) FindByName(ctx context.Context, name string) (*models.ResourcePlan, error) {
	return r.first(ctx, "name = ?", name)
}

func (r *planRepository) ListEnabled(ctx context.Context) ([]models.ResourcePlan, error) {
	var plans []models.ResourcePlan
	err := r.conn(ctx).Where("enabled = true").Order("vcpus asc, memory_mb asc").Find(&plans).Error
	return plans, err
}

func (r *planRepository) ListEnabledUnordered(ctx context.Context) ([]models.ResourcePlan, error) {
	var plans []models.ResourcePlan
	err := r.conn(ctx).Where("enabled = true").Find(&plans).Error
	return plans, err
}

func (r *planRepository) ListAll(ctx context.Context) ([]models.ResourcePlan, error) {
	var plans []models.ResourcePlan
	err := r.conn(ctx).Order("vcpus asc, memory_mb asc").Find(&plans).Error
	return plans, err
}

func (r *planRepository) Create(ctx context.Context, plan *models.ResourcePlan) error {
	return r.conn(ctx).Create(plan).Error
}

func (r *planRepository) Save(ctx context.Context, plan *models.ResourcePlan) error {
	return r.conn(ctx).Save(plan).Error
}

func (r *planRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return r.conn(ctx).Delete(&models.ResourcePlan{}, "id = ?", id).Error
}

func (r *planRepository) ListEnabledBySortOrder(ctx context.Context) ([]models.ResourcePlan, error) {
	var plans []models.ResourcePlan
	err := r.conn(ctx).Where("enabled = ?", true).Order("sort_order asc, memory_mb asc").Find(&plans).Error
	return plans, err
}
