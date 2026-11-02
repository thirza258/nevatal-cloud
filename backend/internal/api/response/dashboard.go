package response

import (
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

// DashboardSummary is the body of GET /dashboard.
type DashboardSummary struct {
	Instances       DashboardInstanceCounts `json:"instances"`
	RecentActivity  []models.AuditLog       `json:"recent_activity"`
	RecentInstances []models.Instance       `json:"recent_instances"`
	Usage           *models.QuotaUsage      `json:"usage"`
}

type DashboardInstanceCounts struct {
	Provisioning int `json:"provisioning"`
	Running      int `json:"running"`
	Stopped      int `json:"stopped"`
	Total        int `json:"total"`
}

func NewDashboardSummary(s *usecase.DashboardSummary) DashboardSummary {
	return DashboardSummary{
		Instances: DashboardInstanceCounts{
			Provisioning: s.Provisioning,
			Running:      s.Running,
			Stopped:      s.Stopped,
			Total:        s.Total,
		},
		RecentActivity:  s.RecentActivity,
		RecentInstances: s.RecentInstances,
		Usage:           s.Usage,
	}
}
