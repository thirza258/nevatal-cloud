package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type ResourceHandler struct {
	instanceService   *services.InstanceService
	appService        *services.AppService
	automationService *services.AutomationService
	openClawService   *services.OpenClawService
	storageService    *services.StorageService
	quotaService      *services.QuotaService
}

func NewResourceHandler(
	instanceService *services.InstanceService,
	appService *services.AppService,
	automationService *services.AutomationService,
	openClawService *services.OpenClawService,
	storageService *services.StorageService,
	quotaService *services.QuotaService,
) *ResourceHandler {
	return &ResourceHandler{
		instanceService:   instanceService,
		appService:        appService,
		automationService: automationService,
		openClawService:   openClawService,
		storageService:    storageService,
		quotaService:      quotaService,
	}
}

type UnifiedResourceItem struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // "vps", "app", "automation", "ai_agent", "bucket"
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Endpoint  string                 `json:"endpoint,omitempty"`
	Specs     string                 `json:"specs,omitempty"`
	CreatedAt string                 `json:"created_at"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

func (h *ResourceHandler) ListAll(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var items []UnifiedResourceItem

	// 1. Instances
	instances, err := h.instanceService.List(userID, false, "")
	if err == nil {
		for _, inst := range instances {
			ip := "allocating"
			if inst.PublicIP != nil && *inst.PublicIP != "" {
				ip = *inst.PublicIP
			} else if inst.PrivateIP != nil && *inst.PrivateIP != "" {
				ip = *inst.PrivateIP
			}
			items = append(items, UnifiedResourceItem{
				ID:        inst.ID.String(),
				Type:      "vps",
				Name:      inst.Name,
				Status:    inst.Status,
				Endpoint:  ip,
				Specs:     inst.OSTemplate.Name,
				CreatedAt: inst.CreatedAt.Format("2006-01-02 15:04"),
				Details: map[string]interface{}{
					"vcpus":     inst.VCPUs,
					"memory_mb": inst.MemoryMB,
					"disk_gb":   inst.DiskGB,
					"vmid":      inst.ProxmoxVMID,
				},
			})
		}
	}

	// 2. Apps
	apps, err := h.appService.List(userID, false)
	if err == nil {
		for _, app := range apps {
			items = append(items, UnifiedResourceItem{
				ID:        app.ID.String(),
				Type:      "app",
				Name:      app.Name,
				Status:    app.Status,
				Endpoint:  "https://" + app.AssignedSubdomain,
				Specs:     app.ApplicationType,
				CreatedAt: app.CreatedAt.Format("2006-01-02 15:04"),
				Details: map[string]interface{}{
					"repository": app.RepositoryURL,
					"branch":     app.ProductionBranch,
				},
			})
		}
	}

	// 3. Automation (n8n)
	autos, err := h.automationService.List(userID, false)
	if err == nil {
		for _, auto := range autos {
			items = append(items, UnifiedResourceItem{
				ID:        auto.ID.String(),
				Type:      "automation",
				Name:      auto.Name,
				Status:    auto.Status,
				Endpoint:  auto.DashboardURL,
				Specs:     "n8n Workflow Automation",
				CreatedAt: auto.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
	}

	// 4. OpenClaw AI Agents
	agents, err := h.openClawService.List(userID, false)
	if err == nil {
		for _, agent := range agents {
			items = append(items, UnifiedResourceItem{
				ID:        agent.ID.String(),
				Type:      "ai_agent",
				Name:      agent.Name,
				Status:    agent.Status,
				Specs:     agent.AIProvider + " (" + agent.DefaultModel + ")",
				CreatedAt: agent.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
	}

	// 5. Storage Buckets
	buckets, err := h.storageService.ListBuckets(userID)
	if err == nil {
		for _, b := range buckets {
			items = append(items, UnifiedResourceItem{
				ID:        b.ID.String(),
				Type:      "bucket",
				Name:      b.Name,
				Status:    "active",
				Endpoint:  "s3://" + b.Name,
				Specs:     b.Visibility,
				CreatedAt: b.CreatedAt.Format("2006-01-02 15:04"),
				Details: map[string]interface{}{
					"objects": b.TotalObjects,
					"bytes":   b.TotalSizeBytes,
				},
			})
		}
	}

	usage, _ := h.quotaService.GetUsage(userID)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"resources": items,
			"usage":     usage,
			"counts": gin.H{
				"total":       len(items),
				"vps":         len(instances),
				"apps":        len(apps),
				"automations": len(autos),
				"ai_agents":   len(agents),
				"buckets":     len(buckets),
			},
		},
	})
}
