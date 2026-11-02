package usecase

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// JobUsecase exposes provisioning job status for polling clients.
type JobUsecase struct {
	jobs repository.JobRepository
}

func NewJobUsecase(jobs repository.JobRepository) *JobUsecase {
	return &JobUsecase{jobs: jobs}
}

// Get returns a job; non-admins only see their own jobs.
func (s *JobUsecase) Get(ctx context.Context, id, userID uuid.UUID, isAdmin bool) (*models.ProvisioningJob, error) {
	return s.jobs.FindByID(ctx, id, ownerScope(userID, isAdmin))
}
