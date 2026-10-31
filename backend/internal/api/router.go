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
	Cfg                *config.Config
	DB                 *gorm.DB
	PVE                proxmox.ProxmoxClient
	AuthService        *services.AuthService
	QuotaService       *services.QuotaService
	SSHKeyService      *services.SSHKeyService
	AuditService       *services.AuditService
	InstanceService    *services.InstanceService
	WorkerService      *services.WorkerService
	AppService         *services.AppService
	AutomationService  *services.AutomationService
	OpenClawService    *services.OpenClawService
	ISOService         *services.ISOService
	StorageService     *services.StorageService
	GameServerService  *services.GameServerService
	AndroidService     *services.AndroidService
	AIAssistantService *services.AIAssistantService
	MCPService         *services.MCPService
	AIAgentService     *services.AIAgentService
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
				"game_server_hosting",
				"android_cloud",
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
	gameServerHandler := handlers.NewGameServerHandler(p.GameServerService)
	androidHandler := handlers.NewAndroidHandler(p.AndroidService)
	aiAssistantHandler := handlers.NewAIAssistantHandler(p.AIAssistantService)
	mcpHandler := handlers.NewMCPHandler(p.MCPService)
	aiAgentHandler := handlers.NewAIAgentHandler(p.AIAgentService)
	resourceHandler := handlers.NewResourceHandler(p.InstanceService, p.AppService, p.AutomationService, p.OpenClawService, p.StorageService, p.GameServerService, p.AndroidService, p.QuotaService)

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

			// 43.1 Nevatal AI Cloud Agent (PRD Section 43.1, 43.2, 43.3)
			ai := protected.Group("/ai")
			{
				ai.GET("/conversations", aiAgentHandler.ListConversations)
				ai.POST("/conversations", aiAgentHandler.CreateConversation)
				ai.GET("/conversations/:id", aiAgentHandler.GetConversation)
				ai.DELETE("/conversations/:id", aiAgentHandler.DeleteConversation)

				ai.POST("/conversations/:id/messages", aiAgentHandler.SendMessage)
				ai.GET("/conversations/:id/messages", aiAgentHandler.ListMessages)
				ai.GET("/conversations/:id/state", aiAgentHandler.GetState)

				// Plans & Approvals
				ai.GET("/conversations/:id/plans", aiAgentHandler.ListPlans)
				ai.GET("/plans/:id", aiAgentHandler.GetPlan)
				ai.POST("/plans/:id/validate", aiAgentHandler.ValidatePlan)
				ai.POST("/plans/:id/approve", aiAgentHandler.ApprovePlan)
				ai.POST("/plans/:id/cancel", aiAgentHandler.CancelPlan)

				// Agent Runs
				ai.GET("/runs/:id", aiAgentHandler.GetRun)
				ai.GET("/runs/:id/events", aiAgentHandler.GetRunEvents)
				ai.POST("/runs/:id/cancel", aiAgentHandler.CancelRun)
			}

			// 43.4 MCP Integrations (PRD Section 37 & 43.4)
			mcp := protected.Group("/integrations/mcp")
			{
				mcp.GET("/servers", mcpHandler.ListServers)
				mcp.GET("/connections", mcpHandler.ListConnections)
				mcp.POST("/connections", mcpHandler.Connect)
				mcp.DELETE("/connections/:id", mcpHandler.Disconnect)
				mcp.GET("/connections/:id/tools", mcpHandler.GetConnectionTools)
				mcp.PATCH("/connections/:id/permissions", mcpHandler.UpdatePermission)
				mcp.GET("/audit-logs", mcpHandler.ListAuditLogs)
			}

			// Game Servers routes per PRD Section 59
			gs := protected.Group("/game-servers")
			{
				gs.GET("", gameServerHandler.List)
				gs.POST("", gameServerHandler.Create)
				gs.GET("/templates", gameServerHandler.ListTemplates)
				gs.GET("/versions", gameServerHandler.ListVersions)
				gs.GET("/plans", gameServerHandler.GetPlans)

				gs.GET("/:id", gameServerHandler.Get)
				gs.PATCH("/:id", gameServerHandler.Update)
				gs.DELETE("/:id", gameServerHandler.Delete)

				gs.POST("/:id/start", gameServerHandler.Start)
				gs.POST("/:id/stop", gameServerHandler.Stop)
				gs.POST("/:id/restart", gameServerHandler.Restart)
				gs.POST("/:id/reinstall", gameServerHandler.Reinstall)

				gs.GET("/:id/status", gameServerHandler.Get)
				gs.GET("/:id/metrics", gameServerHandler.GetMetrics)
				gs.GET("/:id/logs", gameServerHandler.GetLogs)
				gs.POST("/:id/command", gameServerHandler.ExecuteCommand)

				gs.GET("/:id/configuration", gameServerHandler.GetConfiguration)
				gs.PATCH("/:id/configuration", gameServerHandler.UpdateConfiguration)

				gs.GET("/:id/players", gameServerHandler.ListPlayers)
				gs.POST("/:id/players/kick", gameServerHandler.KickPlayer)
				gs.POST("/:id/players/ban", gameServerHandler.BanPlayer)
				gs.POST("/:id/players/unban", gameServerHandler.UnbanPlayer)
				gs.POST("/:id/players/op", gameServerHandler.SetOperator)
				gs.POST("/:id/players/whitelist", gameServerHandler.SetWhitelist)

				gs.GET("/:id/plugins", gameServerHandler.ListPlugins)
				gs.POST("/:id/plugins", gameServerHandler.InstallPlugin)
				gs.PATCH("/:id/plugins/:pluginId", gameServerHandler.TogglePlugin)
				gs.DELETE("/:id/plugins/:pluginId", gameServerHandler.RemovePlugin)

				gs.GET("/:id/mods", gameServerHandler.ListMods)
				gs.POST("/:id/mods", gameServerHandler.InstallMod)
				gs.PATCH("/:id/mods/:modId", gameServerHandler.ToggleMod)
				gs.DELETE("/:id/mods/:modId", gameServerHandler.RemoveMod)

				gs.GET("/:id/worlds", gameServerHandler.ListWorlds)
				gs.POST("/:id/worlds", gameServerHandler.CreateWorld)
				gs.POST("/:id/worlds/reset", gameServerHandler.ResetWorld)
				gs.DELETE("/:id/worlds/:worldId", gameServerHandler.DeleteWorld)

				gs.GET("/:id/backups", gameServerHandler.ListBackups)
				gs.POST("/:id/backups", gameServerHandler.CreateBackup)
				gs.POST("/:id/backups/:backupId/restore", gameServerHandler.RestoreBackup)
				gs.DELETE("/:id/backups/:backupId", gameServerHandler.DeleteBackup)
			}

			// Waydroid Android Cloud routes per PRD Section 67 & 76
			android := protected.Group("/android")
			{
				android.GET("/instances", androidHandler.List)
				android.POST("/instances", androidHandler.Create)
				android.GET("/plans", androidHandler.ListPlans)
				android.GET("/images", androidHandler.ListImages)
				android.GET("/images/:id", androidHandler.GetImage)

				android.GET("/instances/:id", androidHandler.Get)
				android.PATCH("/instances/:id", androidHandler.Update)
				android.DELETE("/instances/:id", androidHandler.Delete)

				android.POST("/instances/:id/start", androidHandler.Start)
				android.POST("/instances/:id/stop", androidHandler.Stop)
				android.POST("/instances/:id/restart", androidHandler.Restart)
				android.POST("/instances/:id/suspend", androidHandler.Suspend)
				android.POST("/instances/:id/resume", androidHandler.Resume)
				android.POST("/instances/:id/reinstall", androidHandler.Reinstall)

				android.GET("/instances/:id/session", androidHandler.CreateSession)
				android.GET("/instances/:id/sessions", androidHandler.ListSessions)
				android.DELETE("/instances/:id/sessions/:sessionId", androidHandler.TerminateSession)

				android.GET("/instances/:id/metrics", androidHandler.Metrics)
				android.GET("/instances/:id/logs", androidHandler.Logs)

				android.GET("/instances/:id/applications", androidHandler.ListApplications)
				android.POST("/instances/:id/applications", androidHandler.InstallAPK)
				android.DELETE("/instances/:id/applications/:packageName", androidHandler.UninstallApplication)
				android.GET("/instances/:id/applications/:packageName", androidHandler.GetApplication)

				android.GET("/instances/:id/backups", androidHandler.ListBackups)
				android.POST("/instances/:id/backups", androidHandler.CreateBackup)
				android.POST("/instances/:id/backups/:backupId/restore", androidHandler.RestoreBackup)
				android.DELETE("/instances/:id/backups/:backupId", androidHandler.DeleteBackup)
			}

			// First-party MCP JSON-RPC 2.0 Endpoint
			protected.POST("/mcp/rpc", mcpHandler.HandleRPC)

			// Unified Resources Endpoint (PRD Section 34.1: /dashboard/resources)
			protected.GET("/resources", resourceHandler.ListAll)
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

			// Admin Waydroid Image Management per PRD Section 69
			adminAndroid := admin.Group("/android")
			{
				adminAndroid.GET("/images", androidHandler.AdminListImages)
				adminAndroid.POST("/images", androidHandler.AdminCreateImage)
				adminAndroid.PUT("/images/:id", androidHandler.AdminUpdateImage)
				adminAndroid.DELETE("/images/:id", androidHandler.AdminDeleteImage)
				adminAndroid.PATCH("/images/:id/publish", androidHandler.AdminPublishImage)
				adminAndroid.PATCH("/images/:id/unpublish", androidHandler.AdminUnpublishImage)
				adminAndroid.POST("/images/:id/verify", androidHandler.AdminVerifyImage)
			}
		}
	}

	return router
}
