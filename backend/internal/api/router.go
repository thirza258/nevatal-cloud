package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/handlers"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/services"
	"gorm.io/gorm"
)

type RouterParams struct {
	Cfg               *config.Config
	DB                *gorm.DB
	PVE               proxmox.ProxmoxClient
	AuthService       *services.AuthService
	QuotaService      *services.QuotaService
	SSHKeyService     *services.SSHKeyService
	AuditService      *services.AuditService
	InstanceService   *services.InstanceService
	WorkerService     *services.WorkerService
	AppService        *services.AppService
	AutomationService *services.AutomationService
	OpenClawService   *services.OpenClawService
	ISOService        *services.ISOService
	StorageService    *services.StorageService
	AIAssistantService *services.AIAssistantService
}

func SetupRouter(p RouterParams) *gin.Engine {
	if p.Cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	router.Use(middleware.SetupCORS())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"product": "Nevatal Cloud",
			"version": "1.1.0",
			"features": []string{
				"compute_vps",
				"wordpress_lemp_bridge",
				"app_automated_deployment",
				"n8n_workflow_automation",
				"openclaw_ai_agents",
				"s3_object_storage",
				"admin_iso_management",
				"openrouter_ai_assistant",
			},
		})
	})

	// Handlers initialization
	authHandler := handlers.NewAuthHandler(p.AuthService, p.AuditService, p.Cfg)
	dashboardHandler := handlers.NewDashboardHandler(p.DB, p.QuotaService, p.AuditService)
	instanceHandler := handlers.NewInstanceHandler(p.DB, p.InstanceService, p.AuditService, p.PVE)
	sshKeyHandler := handlers.NewSSHKeyHandler(p.SSHKeyService, p.AuditService)
	jobHandler := handlers.NewJobHandler(p.DB)
	templatePlanHandler := handlers.NewTemplatePlanHandler(p.DB, p.QuotaService, p.AuditService)
	adminHandler := handlers.NewAdminHandler(p.DB, p.QuotaService, p.AuditService, p.WorkerService, p.PVE)
	wpHandler := handlers.NewWordPressHandler(p.DB, p.Cfg, p.AuthService, p.InstanceService, p.AuditService, p.PVE)

	// New module handlers
	appHandler := handlers.NewAppHandler(p.AppService)
	automationHandler := handlers.NewAutomationHandler(p.AutomationService)
	openClawHandler := handlers.NewOpenClawHandler(p.OpenClawService)
	isoHandler := handlers.NewISOHandler(p.ISOService)
	storageHandler := handlers.NewStorageHandler(p.StorageService)
	aiAssistantHandler := handlers.NewAIAssistantHandler(p.AIAssistantService)

	// API v1 group
	v1 := router.Group("/api/v1")
	{
		// Authentication endpoints
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.AuthRequired(p.AuthService), authHandler.Me)

			// Social & Single Sign-On (OAuth)
			auth.GET("/oauth/urls", authHandler.GetOAuthURLs)
			auth.POST("/oauth/google", authHandler.OAuthGoogle)
			auth.POST("/oauth/github", authHandler.OAuthGitHub)
			auth.POST("/oauth/wordpress", authHandler.OAuthWordPress)
		}

		// Public/Semi-public catalogs
		v1.GET("/templates", templatePlanHandler.ListTemplates)
		v1.GET("/plans", templatePlanHandler.ListPlans)
		v1.GET("/operating-systems", isoHandler.List)

		// Public Storage Shared Links
		v1.GET("/storage/shared/:token", storageHandler.GetSharedLink)
		v1.GET("/storage/shared/:token/download", storageHandler.DownloadSharedLink)

		// WordPress Webhooks & External Integration
		v1.GET("/wordpress/status", wpHandler.GetStatus)
		v1.POST("/webhooks/wordpress", wpHandler.HandleWebhook)

		// Authenticated customer endpoints
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired(p.AuthService))
		{
			// Dashboard & Quotas
			protected.GET("/dashboard", dashboardHandler.GetSummary)
			protected.GET("/quotas", templatePlanHandler.GetQuotas)
			protected.GET("/usage", templatePlanHandler.GetUsage)
			protected.GET("/activity", templatePlanHandler.GetActivity)

			// Instances (Compute VMs)
			protected.GET("/instances", instanceHandler.List)
			protected.POST("/instances", instanceHandler.Create)
			protected.GET("/instances/:id", instanceHandler.Get)
			protected.POST("/instances/:id/start", instanceHandler.Start)
			protected.POST("/instances/:id/shutdown", instanceHandler.Shutdown)
			protected.POST("/instances/:id/restart", instanceHandler.Restart)
			protected.POST("/instances/:id/resize", instanceHandler.Resize)
			protected.DELETE("/instances/:id", instanceHandler.Delete)
			protected.GET("/instances/:id/activity", instanceHandler.GetActivity)
			protected.GET("/instances/:id/metrics", instanceHandler.GetMetrics)
			protected.GET("/instances/:id/console", instanceHandler.GetConsoleTicket)
			protected.GET("/instances/:id/snapshots", instanceHandler.ListSnapshots)
			protected.POST("/instances/:id/snapshots", instanceHandler.CreateSnapshot)
			protected.POST("/instances/:id/snapshots/:snapname/restore", instanceHandler.RestoreSnapshot)
			protected.DELETE("/instances/:id/snapshots/:snapname", instanceHandler.DeleteSnapshot)

			// Applications (Automated Git / Web Deployments)
			protected.GET("/apps", appHandler.List)
			protected.POST("/apps", appHandler.Create)
			protected.GET("/apps/:id", appHandler.Get)
			protected.DELETE("/apps/:id", appHandler.Delete)
			protected.POST("/apps/:id/deploy", appHandler.Deploy)
			protected.POST("/apps/:id/power/:action", appHandler.Power)
			protected.POST("/apps/:id/rollback/:deployment_id", appHandler.Rollback)
			protected.GET("/apps/:id/env", appHandler.GetEnvVars)
			protected.POST("/apps/:id/env", appHandler.SaveEnvVar)
			protected.DELETE("/apps/:id/env/:key", appHandler.DeleteEnvVar)

			// Workflow Automation (n8n Instances)
			protected.GET("/automation/instances", automationHandler.List)
			protected.POST("/automation/instances", automationHandler.Create)
			protected.GET("/automation/instances/:id", automationHandler.Get)
			protected.POST("/automation/instances/:id/start", automationHandler.Start)
			protected.POST("/automation/instances/:id/stop", automationHandler.Stop)
			protected.POST("/automation/instances/:id/restart", automationHandler.Restart)
			protected.POST("/automation/instances/:id/backups", automationHandler.CreateBackup)
			protected.GET("/automation/instances/:id/logs", automationHandler.GetLogs)
			protected.DELETE("/automation/instances/:id", automationHandler.Delete)

			// AI Agents (OpenClaw Personal AI Assistants)
			protected.GET("/ai-agents/openclaw/instances", openClawHandler.List)
			protected.POST("/ai-agents/openclaw/instances", openClawHandler.Create)
			protected.GET("/ai-agents/openclaw/instances/:id", openClawHandler.Get)
			protected.PUT("/ai-agents/openclaw/instances/:id/config", openClawHandler.UpdateConfig)
			protected.POST("/ai-agents/openclaw/instances/:id/integrations", openClawHandler.ConfigureIntegration)
			protected.DELETE("/ai-agents/openclaw/instances/:id/integrations/:channel", openClawHandler.DeleteIntegration)
			protected.POST("/ai-agents/openclaw/instances/:id/start", openClawHandler.Start)
			protected.POST("/ai-agents/openclaw/instances/:id/stop", openClawHandler.Stop)
			protected.POST("/ai-agents/openclaw/instances/:id/restart", openClawHandler.Restart)
			protected.POST("/ai-agents/openclaw/instances/:id/backups", openClawHandler.CreateBackup)
			protected.GET("/ai-agents/openclaw/instances/:id/logs", openClawHandler.GetLogs)
			protected.DELETE("/ai-agents/openclaw/instances/:id", openClawHandler.Delete)

			// AI DevOps & Cloud Support Assistant (OpenRouter)
			protected.POST("/ai-assistant/chat", aiAssistantHandler.Chat)
			protected.POST("/ai-assistant/execute", aiAssistantHandler.Execute)
			protected.GET("/ai-assistant/context", aiAssistantHandler.GetContext)
			protected.GET("/ai-assistant/history", aiAssistantHandler.GetHistory)
			protected.DELETE("/ai-assistant/history", aiAssistantHandler.ClearHistory)
			protected.GET("/ai-assistant/models", aiAssistantHandler.ListModels)
			protected.POST("/ai-assistant/verify-key", aiAssistantHandler.VerifyKey)

			// Object Storage (MinIO S3 Buckets & File Manager)
			protected.GET("/storage/overview", storageHandler.GetOverview)
			protected.GET("/storage/buckets", storageHandler.ListBuckets)
			protected.POST("/storage/buckets", storageHandler.CreateBucket)
			protected.GET("/storage/buckets/:id", storageHandler.GetBucket)
			protected.PUT("/storage/buckets/:id", storageHandler.UpdateBucket)
			protected.DELETE("/storage/buckets/:id", storageHandler.DeleteBucket)
			protected.GET("/storage/buckets/:id/objects", storageHandler.ListObjects)
			protected.POST("/storage/buckets/:id/objects", storageHandler.UploadObject)
			protected.POST("/storage/buckets/:id/folders", storageHandler.CreateFolder)
			protected.GET("/storage/objects/:object_id/download", storageHandler.DownloadObject)
			protected.DELETE("/storage/objects/:object_id", storageHandler.DeleteObject)
			protected.GET("/storage/access-keys", storageHandler.ListAccessKeys)
			protected.POST("/storage/access-keys", storageHandler.CreateAccessKey)
			protected.DELETE("/storage/access-keys/:id", storageHandler.RevokeAccessKey)
			protected.POST("/storage/shared-links", storageHandler.CreateSharedLink)
			protected.GET("/storage/shared-links", storageHandler.ListSharedLinks)
			protected.DELETE("/storage/shared-links/:id", storageHandler.DeleteSharedLink)

			// SSH Keys
			protected.GET("/ssh-keys", sshKeyHandler.List)
			protected.POST("/ssh-keys", sshKeyHandler.Create)
			protected.PATCH("/ssh-keys/:id", sshKeyHandler.Rename)
			protected.DELETE("/ssh-keys/:id", sshKeyHandler.Delete)

			// Jobs (Provisioning status polling)
			protected.GET("/jobs/:id", jobHandler.Get)
		}

		// Authenticated administrator endpoints
		admin := v1.Group("/admin")
		admin.Use(middleware.AuthRequired(p.AuthService), middleware.AdminOnly())
		{
			admin.GET("/overview", adminHandler.Overview)
			admin.GET("/users", adminHandler.ListUsers)
			admin.PATCH("/users/:id", adminHandler.UpdateUserStatus)
			admin.PUT("/users/:id/quotas", adminHandler.UpdateUserQuotas)
			admin.GET("/instances", adminHandler.ListAllInstances)
			admin.GET("/jobs", adminHandler.ListJobs)
			admin.POST("/jobs/:id/retry", adminHandler.RetryJob)
			admin.GET("/nodes", adminHandler.ListNodes)
			admin.GET("/audit-logs", adminHandler.ListAuditLogs)

			// Templates governance
			admin.GET("/templates", adminHandler.ListTemplates)
			admin.POST("/templates", adminHandler.CreateTemplate)
			admin.PUT("/templates/:id", adminHandler.UpdateTemplate)
			admin.DELETE("/templates/:id", adminHandler.DeleteTemplate)

			// Resource plans governance
			admin.GET("/plans", adminHandler.ListPlans)
			admin.POST("/plans", adminHandler.CreatePlan)
			admin.PUT("/plans/:id", adminHandler.UpdatePlan)
			admin.DELETE("/plans/:id", adminHandler.DeletePlan)

			// Admin ISO Management
			admin.GET("/isos", isoHandler.List)
			admin.POST("/isos/upload", isoHandler.Upload)
			admin.PATCH("/isos/:id/publish", isoHandler.SetPublished)
			admin.DELETE("/isos/:id", isoHandler.Delete)
		}
	}

	return router
}
