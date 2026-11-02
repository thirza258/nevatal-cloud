package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// TemplateRepository persists VM OS templates (os_templates).
type TemplateRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.OSTemplate, error)
	FindEnabledByID(ctx context.Context, id uuid.UUID) (*models.OSTemplate, error)
	// FindByRawID looks a template up by an unvalidated id string (an invalid
	// UUID makes the query fail).
	FindByRawID(ctx context.Context, id string) (*models.OSTemplate, error)
	// FindByNameLike returns the first template whose name matches the
	// case-insensitive LIKE pattern.
	FindByNameLike(ctx context.Context, pattern string) (*models.OSTemplate, error)
	FindByDistribution(ctx context.Context, distribution string) (*models.OSTemplate, error)
	// ListEnabled returns enabled templates, oldest first.
	ListEnabled(ctx context.Context) ([]models.OSTemplate, error)
	// ListEnabledUnordered returns enabled templates in database order.
	ListEnabledUnordered(ctx context.Context) ([]models.OSTemplate, error)
	// ListEnabledByName returns enabled templates sorted by name.
	ListEnabledByName(ctx context.Context) ([]models.OSTemplate, error)
	// ListAll returns every template, newest first.
	ListAll(ctx context.Context) ([]models.OSTemplate, error)

	Create(ctx context.Context, tmpl *models.OSTemplate) error
	Save(ctx context.Context, tmpl *models.OSTemplate) error
	DeleteByID(ctx context.Context, id uuid.UUID) error
}

type templateRepository struct{ base }

var _ TemplateRepository = (*templateRepository)(nil)

func NewTemplateRepository(db *gorm.DB) TemplateRepository {
	return &templateRepository{base{db}}
}

func (r *templateRepository) first(ctx context.Context, conds ...interface{}) (*models.OSTemplate, error) {
	var tmpl models.OSTemplate
	if err := r.conn(ctx).First(&tmpl, conds...).Error; err != nil {
		return nil, mapErr(err)
	}
	return &tmpl, nil
}

func (r *templateRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.OSTemplate, error) {
	return r.first(ctx, "id = ?", id)
}

func (r *templateRepository) FindEnabledByID(ctx context.Context, id uuid.UUID) (*models.OSTemplate, error) {
	return r.first(ctx, "id = ? AND enabled = true", id)
}

func (r *templateRepository) FindByRawID(ctx context.Context, id string) (*models.OSTemplate, error) {
	return r.first(ctx, "id = ?", id)
}

func (r *templateRepository) FindByNameLike(ctx context.Context, pattern string) (*models.OSTemplate, error) {
	return r.first(ctx, "name ILIKE ?", pattern)
}

func (r *templateRepository) FindByDistribution(ctx context.Context, distribution string) (*models.OSTemplate, error) {
	return r.first(ctx, "distribution = ?", distribution)
}

func (r *templateRepository) ListEnabled(ctx context.Context) ([]models.OSTemplate, error) {
	var templates []models.OSTemplate
	err := r.conn(ctx).Where("enabled = true").Order("created_at asc").Find(&templates).Error
	return templates, err
}

func (r *templateRepository) ListEnabledUnordered(ctx context.Context) ([]models.OSTemplate, error) {
	var templates []models.OSTemplate
	err := r.conn(ctx).Where("enabled = true").Find(&templates).Error
	return templates, err
}

func (r *templateRepository) ListAll(ctx context.Context) ([]models.OSTemplate, error) {
	var templates []models.OSTemplate
	err := r.conn(ctx).Order("created_at desc").Find(&templates).Error
	return templates, err
}

func (r *templateRepository) Create(ctx context.Context, tmpl *models.OSTemplate) error {
	return r.conn(ctx).Create(tmpl).Error
}

func (r *templateRepository) Save(ctx context.Context, tmpl *models.OSTemplate) error {
	return r.conn(ctx).Save(tmpl).Error
}

func (r *templateRepository) DeleteByID(ctx context.Context, id uuid.UUID) error {
	return r.conn(ctx).Delete(&models.OSTemplate{}, "id = ?", id).Error
}

func (r *templateRepository) ListEnabledByName(ctx context.Context) ([]models.OSTemplate, error) {
	var templates []models.OSTemplate
	err := r.conn(ctx).Where("enabled = ?", true).Order("name asc").Find(&templates).Error
	return templates, err
}
