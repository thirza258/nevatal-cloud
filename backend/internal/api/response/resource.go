package response

import (
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

// ResourceList is the body of GET /resources.
type ResourceList struct {
	Counts    ResourceCounts                `json:"counts"`
	Resources []usecase.UnifiedResourceItem `json:"resources"`
	Usage     *models.QuotaUsage            `json:"usage"`
}

type ResourceCounts struct {
	AIAgents         int `json:"ai_agents"`
	AndroidInstances int `json:"android_instances"`
	Apps             int `json:"apps"`
	Automations      int `json:"automations"`
	Buckets          int `json:"buckets"`
	Databases        int `json:"databases"`
	GameServers      int `json:"game_servers"`
	Total            int `json:"total"`
	VPS              int `json:"vps"`
}

func NewResourceList(s *usecase.ResourceSummary) ResourceList {
	return ResourceList{
		Counts: ResourceCounts{
			AIAgents:         s.Counts.AIAgents,
			AndroidInstances: s.Counts.AndroidInstances,
			Apps:             s.Counts.Apps,
			Automations:      s.Counts.Automations,
			Buckets:          s.Counts.Buckets,
			Databases:        s.Counts.Databases,
			GameServers:      s.Counts.GameServers,
			Total:            len(s.Items),
			VPS:              s.Counts.VPS,
		},
		Resources: s.Items,
		Usage:     s.Usage,
	}
}
