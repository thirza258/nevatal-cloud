package handler

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/usecase"
)

// SystemHandler exposes public, non-secret runtime settings so one frontend
// build can serve both demo and production deployments, plus the health probe.
type SystemHandler struct {
	cfg   *config.Config
	admin *usecase.AdminUsecase
}

func NewSystemHandler(cfg *config.Config, admin *usecase.AdminUsecase) *SystemHandler {
	return &SystemHandler{cfg: cfg, admin: admin}
}

// healthFeatures are the product capabilities advertised by /health.
var healthFeatures = []string{
	"compute_vps",
	"wordpress_lemp_bridge",
	"app_automated_deployment",
	"n8n_workflow_automation",
	"openclaw_ai_agents",
	"s3_object_storage",
	"admin_iso_management",
	"openrouter_ai_assistant",
	"game_server_hosting",
	"android_cloud",
}

// Health: GET /health (public, not enveloped).
func (h *SystemHandler) Health(c *gin.Context) {
	response.JSON(c, http.StatusOK, response.Health{
		Features: healthFeatures,
		Product:  "Nevatal Cloud",
		Status:   "healthy",
		Version:  config.Version,
	})
}

func (h *SystemHandler) buildTelemetry(ctx context.Context) *response.SystemTelemetry {
	t := &response.SystemTelemetry{
		NodeName:        "Dell PowerEdge R640 Node 01",
		ProxmoxNodeName: "pve-r640",
		CPUThreads:      40,
		CPUCores:        20,
		CPUModel:        "Dual Intel Xeon Silver 4114",
		MemoryTotalMB:   32768,
		MemoryTotalGB:   32,
		MemoryUsedGB:    7,
		DiskTotalGB:     2500,
		DiskTotalTB:     "2.5",
		PveVersion:      "Proxmox VE 9.2",
		Status:          "online",
	}

	if h.admin == nil {
		return t
	}

	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ov := h.admin.Overview(reqCtx)

	if ov.Node.Name != "" {
		t.NodeName = ov.Node.Name
	}
	if ov.Node.ProxmoxNodeName != "" {
		t.ProxmoxNodeName = ov.Node.ProxmoxNodeName
	}
	if ov.NodeStatus != nil && ov.NodeStatus.CPUInfo.CPUs > 0 {
		t.CPUThreads = ov.NodeStatus.CPUInfo.CPUs
		t.CPUCores = ov.NodeStatus.CPUInfo.Cores
		t.CPUModel = ov.NodeStatus.CPUInfo.Model
	} else if ov.Node.TotalCPUThreads > 0 {
		t.CPUThreads = ov.Node.TotalCPUThreads
	}

	if ov.NodeStatus != nil && ov.NodeStatus.Memory.Total > 0 {
		t.MemoryTotalMB = int(ov.NodeStatus.Memory.Total / (1024 * 1024))
		gb := int((ov.NodeStatus.Memory.Total + (512 * 1024 * 1024)) / (1024 * 1024 * 1024))
		if gb == 31 {
			gb = 32
		}
		t.MemoryTotalGB = gb
		t.MemoryUsedGB = int((ov.NodeStatus.Memory.Used + (512 * 1024 * 1024)) / (1024 * 1024 * 1024))
	} else if ov.Node.TotalMemoryMB > 0 {
		t.MemoryTotalMB = ov.Node.TotalMemoryMB
		t.MemoryTotalGB = ov.Node.TotalMemoryMB / 1024
	}

	if ov.NodeStatus != nil {
		if ov.NodeStatus.PveVersion != "" {
			t.PveVersion = ov.NodeStatus.PveVersion
		}
		if ov.NodeStatus.KVersion != "" {
			t.KernelVersion = ov.NodeStatus.KVersion
		}
	}

	var totalBytes, usedBytes int64
	for _, st := range ov.Storage {
		if st.Active == 1 {
			totalBytes += st.Total
			usedBytes += st.Used
			t.StoragePools = append(t.StoragePools, response.StoragePoolSummary{
				Name:    st.Storage,
				Type:    st.Type,
				TotalGB: st.Total / (1024 * 1024 * 1024),
				AvailGB: st.Avail / (1024 * 1024 * 1024),
				UsedGB:  st.Used / (1024 * 1024 * 1024),
			})
		}
	}

	if totalBytes > 0 {
		t.DiskTotalGB = int(totalBytes / (1024 * 1024 * 1024))
		if totalBytes >= 2_400_000_000_000 && t.DiskTotalGB < 2500 {
			t.DiskTotalGB = 2500
		}
		t.DiskTotalTB = fmt.Sprintf("%.1f", float64(totalBytes)/(1000*1000*1000*1000))
		t.DiskUsedGB = int(usedBytes / (1024 * 1024 * 1024))
	} else if ov.Node.TotalDiskGB > 0 {
		t.DiskTotalGB = ov.Node.TotalDiskGB
		t.DiskTotalTB = fmt.Sprintf("%.1f", float64(t.DiskTotalGB)/1000)
	}

	return t
}

// GetConfig: GET /api/v1/system/config (public).
func (h *SystemHandler) GetConfig(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=15")
	domains := response.SystemDomainsConfig{
		Enabled:    h.cfg.DomainsEnabled(),
		MaxPerUser: h.cfg.MaxDomainsPerUser,
	}
	if domains.Enabled {
		domains.Root = h.cfg.PublicDomainRoot
	}
	telemetry := h.buildTelemetry(c.Request.Context())
	response.OK(c, response.SystemConfig{
		Auth: response.SystemAuthConfig{
			PasswordLogin: true,
			Registration:  true,
			SocialLogin:   h.cfg.DemoMode,
		},
		DemoMode:    h.cfg.DemoMode,
		Domains:     domains,
		Environment: h.cfg.Env,
		Features:    response.SystemFeaturesConfig{DockerViewer: true, Domains: domains.Enabled},
		ProxmoxMock: h.cfg.ProxmoxMock,
		Telemetry:   telemetry,
		Version:     config.Version,
	})
}

// GetTelemetry: GET /api/v1/system/telemetry (public).
func (h *SystemHandler) GetTelemetry(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=10")
	telemetry := h.buildTelemetry(c.Request.Context())
	response.OK(c, telemetry)
}
