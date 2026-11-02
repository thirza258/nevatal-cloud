package usecase

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// ErrUsageUnavailable is returned when quota usage cannot be computed.
var ErrUsageUnavailable = errors.New("failed to calculate quota usage")

// DashboardSummary is the customer dashboard: instance state counts, quota
// usage, the five newest instances and the ten newest audit entries.
type DashboardSummary struct {
	Total           int
	Running         int
	Stopped         int
	Provisioning    int
	Usage           *models.QuotaUsage
	RecentInstances []models.Instance
	RecentActivity  []models.AuditLog
}

type DashboardUsecase struct {
	instances    repository.InstanceRepository
	quotaService *QuotaUsecase
	auditService *AuditUsecase
}

func NewDashboardUsecase(instances repository.InstanceRepository, quotaService *QuotaUsecase, auditService *AuditUsecase) *DashboardUsecase {
	return &DashboardUsecase{instances: instances, quotaService: quotaService, auditService: auditService}
}

func (s *DashboardUsecase) GetSummary(ctx context.Context, userID uuid.UUID) (*DashboardSummary, error) {
	// 1. Quota & usage
	usage, err := s.quotaService.GetUsage(ctx, userID)
	if err != nil {
		return nil, ErrUsageUnavailable
	}

	// 2. Instance state breakdown
	instances, _ := s.instances.ListByOwnerWithTemplate(ctx, userID)

	summary := &DashboardSummary{Total: len(instances), Usage: usage}
	for _, inst := range instances {
		switch inst.Status {
		case models.StateRunning:
			summary.Running++
		case models.StateStopped:
			summary.Stopped++
		case models.StateProvisioning, models.StateQueued, models.StateStarting, models.StateRestarting:
			summary.Provisioning++
		}
	}

	// 3. Recent activity
	summary.RecentActivity, _ = s.auditService.ListRecentActivity(ctx, &userID, 10)

	// 4. Five newest instances
	summary.RecentInstances = instances
	if len(summary.RecentInstances) > 5 {
		summary.RecentInstances = summary.RecentInstances[:5]
	}
	return summary, nil
}
