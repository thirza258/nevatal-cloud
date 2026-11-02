package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/config"
)

// SystemHandler exposes public, non-secret runtime settings so one frontend
// build can serve both demo and production deployments, plus the health probe.
type SystemHandler struct {
	cfg *config.Config
}

func NewSystemHandler(cfg *config.Config) *SystemHandler {
	return &SystemHandler{cfg: cfg}
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

// GetConfig: GET /api/v1/system/config (public).
func (h *SystemHandler) GetConfig(c *gin.Context) {
	c.Header("Cache-Control", "public, max-age=60")
	domains := response.SystemDomainsConfig{
		Enabled:    h.cfg.DomainsEnabled(),
		MaxPerUser: h.cfg.MaxDomainsPerUser,
	}
	if domains.Enabled {
		domains.Root = h.cfg.PublicDomainRoot
	}
	response.OK(c, response.SystemConfig{
		Auth: response.SystemAuthConfig{
			PasswordLogin: true,
			Registration:  true,
			// Social buttons only run the mock flow; real OAuth redirects
			// are not wired in the UI yet, so they are demo-only.
			SocialLogin: h.cfg.DemoMode,
		},
		DemoMode:    h.cfg.DemoMode,
		Domains:     domains,
		Environment: h.cfg.Env,
		Features:    response.SystemFeaturesConfig{DockerViewer: true, Domains: domains.Enabled},
		ProxmoxMock: h.cfg.ProxmoxMock,
		Version:     config.Version,
	})
}
