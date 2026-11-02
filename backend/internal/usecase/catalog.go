package usecase

import (
	"context"

	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// CatalogUsecase serves the public VM catalog: enabled OS templates and
// resource plans.
type CatalogUsecase struct {
	templates repository.TemplateRepository
	plans     repository.PlanRepository
}

func NewCatalogUsecase(templates repository.TemplateRepository, plans repository.PlanRepository) *CatalogUsecase {
	return &CatalogUsecase{templates: templates, plans: plans}
}

// ListTemplates returns enabled templates, oldest first.
func (s *CatalogUsecase) ListTemplates(ctx context.Context) ([]models.OSTemplate, error) {
	return s.templates.ListEnabled(ctx)
}

// ListPlans returns enabled plans, smallest first.
func (s *CatalogUsecase) ListPlans(ctx context.Context) ([]models.ResourcePlan, error) {
	return s.plans.ListEnabled(ctx)
}
