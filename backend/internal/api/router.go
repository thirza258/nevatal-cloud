package api

import (
	"context"
	"log"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/handler"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/usecase"
)

// RouterParams carries the use cases the HTTP handlers delegate to. Every
// field except Cfg may be nil when only the route table is needed (tests):
// handlers only dereference their use case while serving a request.
type RouterParams struct {
	Cfg *config.Config

	Auth          *usecase.AuthUsecase
	Quota         *usecase.QuotaUsecase
	SSHKey        *usecase.SSHKeyUsecase
	Audit         *usecase.AuditUsecase
	Catalog       *usecase.CatalogUsecase
	Dashboard     *usecase.DashboardUsecase
	Job           *usecase.JobUsecase
	Instance      *usecase.InstanceUsecase
	Docker        *usecase.DockerUsecase
	App           *usecase.AppUsecase
	Automation    *usecase.AutomationUsecase
	OpenClaw      *usecase.OpenClawUsecase
	ISO           *usecase.ISOUsecase
	Storage       *usecase.StorageUsecase
	GameServer    *usecase.GameServerUsecase
	Android       *usecase.AndroidUsecase
	AIAssistant   *usecase.AIAssistantUsecase
	MCP           *usecase.MCPUsecase
	AIAgent       *usecase.AIAgentUsecase
	Resource      *usecase.ResourceUsecase
	WordPress     *usecase.WordPressUsecase
	Admin         *usecase.AdminUsecase
	Domain        *usecase.DomainUsecase
	Database      *usecase.DatabaseUsecase
	Notification  *usecase.NotificationUsecase
	Monitoring    *usecase.MonitoringUsecase
	Policies      *usecase.AutomationPolicyUsecase
	Observability *usecase.ObservabilityUsecase
	Network       *usecase.NetworkUsecase
	LoadBalancer  *usecase.LoadBalancerUsecase
	Developer     *usecase.DeveloperUsecase

	// Orchestrator serves plans, approvals and executions (/ai/plans,
	// /ai/executions, /ai/tools).
	Orchestrator *usecase.OrchestratorUsecase
	// Streams is cancelled when the HTTP server shuts down so long-lived
	// responses (SSE) end promptly. Nil means never.
	Streams context.Context
}

func SetupRouter(p RouterParams) *gin.Engine {
	if p.Cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.New()
	// Probes hit /health every few seconds; keep them out of the access log.
	router.Use(gin.LoggerWithConfig(gin.LoggerConfig{SkipPaths: []string{"/health"}}), gin.Recovery())
	// Only trust X-Forwarded-For from known proxies (nginx, ingress, LB);
	// otherwise c.ClientIP() in audit logs is spoofable.
	if err := router.SetTrustedProxies(p.Cfg.TrustedProxies); err != nil {
		log.Printf("Warning: invalid TRUSTED_PROXIES (%v); trusting no proxies", err)
		_ = router.SetTrustedProxies(nil)
	}
	router.Use(middleware.SetupCORSWithConfig(p.Cfg))
	router.Use(middleware.CorrelationMiddleware())

	systemHandler := handler.NewSystemHandler(p.Cfg, p.Admin)

	// Health check endpoint
	router.GET("/health", systemHandler.Health)

	// Handlers initialization
	authHandler := handler.NewAuthHandler(p.Auth, p.Cfg)
	dashboardHandler := handler.NewDashboardHandler(p.Dashboard)
	instanceHandler := handler.NewInstanceHandler(p.Instance)
	sshKeyHandler := handler.NewSSHKeyHandler(p.SSHKey)
	jobHandler := handler.NewJobHandler(p.Job)
	templatePlanHandler := handler.NewTemplatePlanHandler(p.Catalog, p.Quota, p.Audit)
	adminHandler := handler.NewAdminHandler(p.Admin)
	wpHandler := handler.NewWordPressHandler(p.WordPress)

	// New module handlers
	appHandler := handler.NewAppHandler(p.App)
	automationHandler := handler.NewAutomationHandler(p.Automation)
	openClawHandler := handler.NewOpenClawHandler(p.OpenClaw)
	isoHandler := handler.NewISOHandler(p.ISO)
	storageHandler := handler.NewStorageHandler(p.Storage)
	gameServerHandler := handler.NewGameServerHandler(p.GameServer)
	androidHandler := handler.NewAndroidHandler(p.Android)
	aiAssistantHandler := handler.NewAIAssistantHandler(p.AIAssistant)
	mcpHandler := handler.NewMCPHandler(p.MCP)
	aiAgentHandler := handler.NewAIAgentHandler(p.AIAgent)
	dockerHandler := handler.NewDockerHandler(p.Docker)
	resourceHandler := handler.NewResourceHandler(p.Resource)
	domainHandler := handler.NewDomainHandler(p.Domain)
	orchHandler := handler.NewOrchestratorHandler(p.Orchestrator, p.Streams)
	opsHandler := handler.NewOperationsHandler(p.Database, p.Notification, p.Monitoring, p.Policies, p.Orchestrator)
	obsHandler := handler.NewObservabilityHandler(p.Observability, p.Admin, p.Streams)
	netHandler := handler.NewNetworkHandler(p.Network)
	lbHandler := handler.NewLoadBalancerHandler(p.LoadBalancer)
	devHandler := handler.NewDeveloperHandler(p.Developer)

	// API v1 group
	v1 := router.Group("/api/v1")
	{
		// OpenAPI Specification
		v1.GET("/openapi.json", devHandler.GetOpenAPISpec)

		// Authentication endpoints
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.AuthRequired(p.Auth, p.Developer, p.Cfg.CookieSecure), authHandler.Me)

			// Social & Single Sign-On (OAuth)
			auth.GET("/oauth/urls", authHandler.GetOAuthURLs)
			auth.POST("/oauth/google", authHandler.OAuthGoogle)
			auth.POST("/oauth/github", authHandler.OAuthGitHub)
			auth.POST("/oauth/wordpress", authHandler.OAuthWordPress)
		}

		// Public runtime settings & dynamic server telemetry
		v1.GET("/system/config", systemHandler.GetConfig)
		v1.GET("/system/telemetry", systemHandler.GetTelemetry)

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
		protected.Use(middleware.AuthRequired(p.Auth, p.Developer, p.Cfg.CookieSecure))
		{
			// Dashboard & Quotas
			protected.GET("/dashboard", dashboardHandler.GetSummary)
			protected.GET("/quotas", templatePlanHandler.GetQuotas)
			protected.GET("/usage", templatePlanHandler.GetUsage)
			protected.GET("/activity", templatePlanHandler.GetActivity)

			// Instances (Compute VMs)
			instances := protected.Group("/instances")
			instances.Use(middleware.RequireScope("instances"))
			{
				instances.GET("", instanceHandler.List)
				instances.POST("", instanceHandler.Create)
				instances.GET("/:id", instanceHandler.Get)
				instances.POST("/:id/start", instanceHandler.Start)
				instances.POST("/:id/shutdown", instanceHandler.Shutdown)
				instances.POST("/:id/restart", instanceHandler.Restart)
				instances.POST("/:id/resize", instanceHandler.Resize)
				instances.DELETE("/:id", instanceHandler.Delete)
				instances.GET("/:id/activity", instanceHandler.GetActivity)
				instances.GET("/:id/metrics", instanceHandler.GetMetrics)
				instances.GET("/:id/console", instanceHandler.GetConsoleTicket)
				instances.GET("/:id/snapshots", instanceHandler.ListSnapshots)
				instances.POST("/:id/snapshots", instanceHandler.CreateSnapshot)
				instances.POST("/:id/snapshots/:snapname/restore", instanceHandler.RestoreSnapshot)
				instances.DELETE("/:id/snapshots/:snapname", instanceHandler.DeleteSnapshot)

				// Docker viewer (via the QEMU guest agent)
				instances.GET("/:id/docker", dockerHandler.Overview)
				instances.GET("/:id/docker/containers/:container/logs", dockerHandler.Logs)
			}

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

				// Plans & Approvals (AI infrastructure orchestrator,
				// docs/ORCHESTRATOR_API.md)
				ai.GET("/conversations/:id/plans", aiAgentHandler.ListPlans)
				ai.GET("/plans/:id", orchHandler.GetPlan)
				ai.PATCH("/plans/:id", orchHandler.PatchPlan)
				ai.GET("/plans/:id/revisions", orchHandler.ListRevisions)
				ai.GET("/plans/:id/revisions/:version", orchHandler.GetRevision)
				ai.POST("/plans/:id/validate", orchHandler.ValidatePlan)
				ai.POST("/plans/:id/approve", orchHandler.ApprovePlan)
				ai.POST("/plans/:id/reject", orchHandler.RejectPlan)
				ai.POST("/plans/:id/cancel", orchHandler.CancelPlan)
				ai.POST("/plans/:id/execute", orchHandler.ExecutePlan)

				// Executions
				ai.GET("/executions", orchHandler.ListExecutions)
				ai.GET("/executions/:id", orchHandler.GetExecution)
				ai.GET("/executions/:id/events", orchHandler.Events)
				ai.POST("/executions/:id/cancel", orchHandler.CancelExecution)
				ai.POST("/executions/:id/tasks/:taskId/retry", orchHandler.RetryTask)
				ai.POST("/executions/:id/recovery", orchHandler.Recovery)

				// Tool registry (also MCP tools/list)
				ai.GET("/tools", orchHandler.ListTools)

				// Phase 2: reusable templates, topology and logical agent roles
				ai.GET("/templates", opsHandler.ListTemplates)
				ai.POST("/templates", opsHandler.CreateTemplate)
				ai.GET("/templates/:id", opsHandler.GetTemplate)
				ai.PATCH("/templates/:id", opsHandler.UpdateTemplate)
				ai.DELETE("/templates/:id", opsHandler.DeleteTemplate)
				ai.POST("/templates/:id/deploy", opsHandler.DeployTemplate)
				ai.GET("/topology", opsHandler.Topology)
				ai.GET("/agents", opsHandler.AgentRoles)

				// Phase 3: diagnostics, scheduled checks and automation policies
				ai.GET("/diagnostics/:resourceType/:id", opsHandler.Diagnose)
				ai.POST("/diagnostics/:resourceType/:id/remediate", opsHandler.Remediate)
				ai.GET("/checks", opsHandler.ListChecks)
				ai.POST("/checks", opsHandler.CreateCheck)
				ai.PATCH("/checks/:id", opsHandler.UpdateCheck)
				ai.DELETE("/checks/:id", opsHandler.DeleteCheck)
				ai.POST("/checks/:id/run", opsHandler.RunCheck)
				ai.GET("/policies", opsHandler.ListPolicies)
				ai.POST("/policies", opsHandler.CreatePolicy)
				ai.PATCH("/policies/:id", opsHandler.UpdatePolicy)
				ai.DELETE("/policies/:id", opsHandler.DeletePolicy)

				// Phase 4: optimization advisor and cost estimate
				ai.GET("/advisor", opsHandler.Optimize)
				ai.GET("/advisor/costs", opsHandler.Costs)

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

			// In-app notifications
			notifications := protected.Group("/notifications")
			{
				notifications.GET("", opsHandler.ListNotifications)
				notifications.GET("/unread-count", opsHandler.NotificationUnreadCount)
				notifications.POST("/read-all", opsHandler.MarkAllNotificationsRead)
				notifications.POST("/:id/read", opsHandler.MarkNotificationRead)
			}

			// Managed databases
			databases := protected.Group("/databases")
			{
				databases.GET("", opsHandler.ListDatabases)
				databases.GET("/plans", opsHandler.DatabasePlans)
				databases.POST("", opsHandler.CreateDatabase)
				databases.GET("/:id", opsHandler.GetDatabase)
				databases.DELETE("/:id", opsHandler.DeleteDatabase)
				databases.POST("/:id/power/:action", opsHandler.DatabasePower)
				databases.GET("/:id/credentials", opsHandler.DatabaseCredentials)
				databases.GET("/:id/backups", opsHandler.DatabaseBackups)
				databases.POST("/:id/backups", opsHandler.CreateDatabaseBackup)
				databases.POST("/:id/backups/:backupId/restore", opsHandler.RestoreDatabaseBackup)
				databases.DELETE("/:id/backups/:backupId", opsHandler.DeleteDatabaseBackup)
				databases.GET("/:id/logs", opsHandler.DatabaseLogs)
				databases.GET("/:id/metrics", opsHandler.DatabaseMetrics)
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

			// Nevatal Domains: publish a VM port at https://<subdomain>.<PUBLIC_DOMAIN_ROOT>
			domains := protected.Group("/domains")
			domains.Use(middleware.RequireDomains(p.Cfg))
			{
				domains.GET("", domainHandler.List)
				domains.GET("/check", domainHandler.Check)
				domains.POST("", domainHandler.Create)
				domains.PATCH("/:id", domainHandler.Update)
				domains.DELETE("/:id", domainHandler.Release)
			}

			// Observability (Customer / User Observability Center)
			obs := protected.Group("/observability")
			{
				obs.GET("/overview", obsHandler.UserOverview)
				obs.GET("/metrics", obsHandler.UserMetrics)
				obs.GET("/events", obsHandler.UserEvents)
				obs.GET("/alerts", obsHandler.UserAlerts)
				obs.POST("/alerts", obsHandler.CreateUserAlertRule)
				obs.PATCH("/alerts/:id", obsHandler.UpdateUserAlertRule)
				obs.DELETE("/alerts/:id", obsHandler.DeleteUserAlertRule)
				obs.GET("/instances/:id", obsHandler.UserInstanceObservability)
				obs.GET("/instances/:id/metrics", obsHandler.UserInstanceMetrics)
				obs.GET("/instances/:id/events", obsHandler.UserInstanceEvents)
				obs.GET("/stream", obsHandler.Stream)
			}

			// Private Networks
			networks := protected.Group("/networks")
			networks.Use(middleware.RequireScope("networks"))
			{
				networks.GET("", netHandler.ListNetworks)
				networks.POST("", netHandler.CreateNetwork)
				networks.GET("/:id", netHandler.GetNetwork)
				networks.PATCH("/:id", netHandler.UpdateNetwork)
				networks.DELETE("/:id", netHandler.DeleteNetwork)
				networks.POST("/:id/attachments", netHandler.AttachInstance)
				networks.DELETE("/:id/attachments/:attachment_id", netHandler.DetachInstance)
				networks.GET("/:id/traffic", netHandler.GetNetworkTraffic)
			}

			// Cloud Firewalls
			firewalls := protected.Group("/firewalls")
			firewalls.Use(middleware.RequireScope("firewalls"))
			{
				firewalls.GET("", netHandler.ListFirewalls)
				firewalls.POST("", netHandler.CreateFirewall)
				firewalls.GET("/:id", netHandler.GetFirewall)
				firewalls.PATCH("/:id", netHandler.UpdateFirewall)
				firewalls.DELETE("/:id", netHandler.DeleteFirewall)
				firewalls.POST("/:id/rules", netHandler.AddFirewallRule)
				firewalls.PATCH("/:id/rules/:rule_id", netHandler.UpdateFirewallRule)
				firewalls.DELETE("/:id/rules/:rule_id", netHandler.DeleteFirewallRule)
				firewalls.POST("/:id/attachments", netHandler.AttachFirewall)
				firewalls.DELETE("/:id/attachments/:attachment_id", netHandler.DetachFirewall)
			}

			// Floating IPs
			floatingIPs := protected.Group("/floating-ips")
			floatingIPs.Use(middleware.RequireScope("networks"))
			{
				floatingIPs.GET("", netHandler.ListFloatingIPs)
				floatingIPs.POST("", netHandler.AllocateFloatingIP)
				floatingIPs.POST("/:id/assign", netHandler.AssignFloatingIP)
				floatingIPs.POST("/:id/unassign", netHandler.UnassignFloatingIP)
				floatingIPs.DELETE("/:id", netHandler.ReleaseFloatingIP)
			}

			// DNS Management
			dns := protected.Group("/dns")
			dns.Use(middleware.RequireScope("dns"))
			{
				dns.GET("/zones", netHandler.ListDNSZones)
				dns.POST("/zones", netHandler.CreateDNSZone)
				dns.DELETE("/zones/:id", netHandler.DeleteDNSZone)
				dns.GET("/zones/:id/records", netHandler.ListDNSRecords)
				dns.POST("/zones/:id/records", netHandler.CreateDNSRecord)
				dns.PATCH("/zones/:id/records/:record_id", netHandler.UpdateDNSRecord)
				dns.DELETE("/zones/:id/records/:record_id", netHandler.DeleteDNSRecord)
			}

			// Managed Load Balancers
			lbs := protected.Group("/load-balancers")
			lbs.Use(middleware.RequireScope("load_balancers"))
			{
				lbs.GET("", lbHandler.ListLoadBalancers)
				lbs.POST("", lbHandler.CreateLoadBalancer)
				lbs.GET("/certificates", lbHandler.ListCertificates)
				lbs.POST("/certificates", lbHandler.AddCertificate)
				lbs.DELETE("/certificates/:id", lbHandler.DeleteCertificate)
				lbs.GET("/:id", lbHandler.GetLoadBalancer)
				lbs.PATCH("/:id", lbHandler.UpdateLoadBalancer)
				lbs.DELETE("/:id", lbHandler.DeleteLoadBalancer)
				lbs.POST("/:id/listeners", lbHandler.AddListener)
				lbs.DELETE("/:id/listeners/:listener_id", lbHandler.DeleteListener)
				lbs.POST("/:id/pools", lbHandler.AddPool)
				lbs.PATCH("/:id/pools/:pool_id", lbHandler.UpdatePool)
				lbs.DELETE("/:id/pools/:pool_id", lbHandler.DeletePool)
				lbs.POST("/:id/pools/:pool_id/members", lbHandler.AddMember)
				lbs.PATCH("/:id/pools/:pool_id/members/:member_id", lbHandler.UpdateMember)
				lbs.DELETE("/:id/pools/:pool_id/members/:member_id", lbHandler.DeleteMember)
				lbs.GET("/:id/health", lbHandler.GetHealth)
				lbs.GET("/:id/metrics", lbHandler.GetMetrics)
			}

			// Developer API Keys
			apiKeys := protected.Group("/api-keys")
			apiKeys.Use(middleware.RequireScope("api_keys"))
			{
				apiKeys.GET("", devHandler.ListAPIKeys)
				apiKeys.POST("", devHandler.CreateAPIKey)
				apiKeys.DELETE("/:id", devHandler.RevokeAPIKey)
			}

			// Webhooks & Event Integrations
			webhooks := protected.Group("/webhooks")
			webhooks.Use(middleware.RequireScope("webhooks"))
			{
				webhooks.GET("", devHandler.ListWebhooks)
				webhooks.POST("", devHandler.CreateWebhook)
				webhooks.GET("/deliveries", devHandler.ListDeliveries)
				webhooks.POST("/deliveries/:delivery_id/retry", devHandler.RetryDelivery)
				webhooks.GET("/:id", devHandler.GetWebhook)
				webhooks.PATCH("/:id", devHandler.UpdateWebhook)
				webhooks.DELETE("/:id", devHandler.DeleteWebhook)
				webhooks.POST("/:id/rotate-secret", devHandler.RotateSecret)
				webhooks.POST("/:id/ping", devHandler.TestPing)
			}
		}

		// Authenticated administrator endpoints
		admin := v1.Group("/admin")
		admin.Use(middleware.AuthRequired(p.Auth, p.Developer, p.Cfg.CookieSecure), middleware.AdminOnly())
		{
			admin.GET("/overview", adminHandler.Overview)
			admin.GET("/users", adminHandler.ListUsers)
			admin.PATCH("/users/:id", adminHandler.UpdateUserStatus)
			admin.PUT("/users/:id/quotas", adminHandler.UpdateUserQuotas)
			admin.GET("/instances", adminHandler.ListAllInstances)
			admin.GET("/instances/:id/docker", dockerHandler.AdminOverview)
			admin.GET("/instances/:id/docker/containers/:container/logs", dockerHandler.AdminLogs)
			admin.GET("/jobs", adminHandler.ListJobs)
			admin.POST("/jobs/:id/retry", adminHandler.RetryJob)
			admin.GET("/nodes", adminHandler.ListNodes)
			admin.GET("/audit-logs", adminHandler.ListAuditLogs)

			// Admin Observability Center
			adminObs := admin.Group("/observability")
			{
				adminObs.GET("/overview", obsHandler.AdminOverview)
				adminObs.GET("/infrastructure", obsHandler.AdminInfrastructure)
				adminObs.GET("/nodes", obsHandler.AdminNodes)
				adminObs.GET("/nodes/:id", obsHandler.AdminNodeDetail)
				adminObs.GET("/storage", obsHandler.AdminStorage)
				adminObs.GET("/vms", obsHandler.AdminVMs)
				adminObs.GET("/vms/:id", obsHandler.AdminVMDetail)
				adminObs.GET("/api", obsHandler.AdminAPI)
				adminObs.GET("/jobs", obsHandler.AdminJobs)
				adminObs.GET("/tenants", obsHandler.AdminTenants)
				adminObs.GET("/events", obsHandler.AdminEvents)
				adminObs.GET("/logs", obsHandler.AdminLogs)
				adminObs.GET("/alerts", obsHandler.AdminAlerts)
				adminObs.POST("/alerts/:id/acknowledge", obsHandler.AdminAcknowledgeAlert)
				adminObs.POST("/alerts/:id/resolve", obsHandler.AdminResolveAlert)
				adminObs.GET("/incidents", obsHandler.AdminIncidents)
				adminObs.POST("/incidents", obsHandler.AdminCreateIncident)
				adminObs.GET("/incidents/:id", obsHandler.AdminGetIncident)
				adminObs.PATCH("/incidents/:id", obsHandler.AdminUpdateIncident)
				adminObs.GET("/capacity", obsHandler.AdminCapacity)
				adminObs.GET("/ai", obsHandler.AdminAI)
				adminObs.POST("/ai/investigate", obsHandler.AdminAIInvestigate)
			}

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

			// Nevatal Domains moderation
			adminDomains := admin.Group("/domains")
			adminDomains.Use(middleware.RequireDomains(p.Cfg))
			{
				adminDomains.GET("", domainHandler.AdminList)
				adminDomains.PATCH("/:id", domainHandler.AdminUpdate)
			}
		}
	}

	return router
}
