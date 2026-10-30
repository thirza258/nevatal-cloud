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

func SetupRouter(
	cfg *config.Config,
	db *gorm.DB,
	pve proxmox.ProxmoxClient,
	authService *services.AuthService,
	quotaService *services.QuotaService,
	sshKeyService *services.SSHKeyService,
	auditService *services.AuditService,
	instanceService *services.InstanceService,
	workerService *services.WorkerService,
) *gin.Engine {
	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	router := gin.Default()
	router.Use(middleware.SetupCORS())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "healthy",
			"product": "Nevatal Cloud",
			"version": "1.0.0",
		})
	})

	// Handlers initialization
	authHandler := handlers.NewAuthHandler(authService, auditService)
	dashboardHandler := handlers.NewDashboardHandler(db, quotaService, auditService)
	instanceHandler := handlers.NewInstanceHandler(db, instanceService, auditService, pve)
	sshKeyHandler := handlers.NewSSHKeyHandler(sshKeyService, auditService)
	jobHandler := handlers.NewJobHandler(db)
	templatePlanHandler := handlers.NewTemplatePlanHandler(db, quotaService, auditService)
	adminHandler := handlers.NewAdminHandler(db, quotaService, auditService, workerService, pve)

	// API v1 group
	v1 := router.Group("/api/v1")
	{
		// Authentication endpoints
		auth := v1.Group("/auth")
		{
			auth.POST("/register", authHandler.Register)
			auth.POST("/login", authHandler.Login)
			auth.POST("/logout", authHandler.Logout)
			auth.GET("/me", middleware.AuthRequired(authService), authHandler.Me)
		}

		// Public/Semi-public catalogs
		v1.GET("/templates", templatePlanHandler.ListTemplates)
		v1.GET("/plans", templatePlanHandler.ListPlans)

		// Authenticated customer endpoints
		protected := v1.Group("")
		protected.Use(middleware.AuthRequired(authService))
		{
			// Dashboard & Quotas
			protected.GET("/dashboard", dashboardHandler.GetSummary)
			protected.GET("/quotas", templatePlanHandler.GetQuotas)
			protected.GET("/usage", templatePlanHandler.GetUsage)
			protected.GET("/activity", templatePlanHandler.GetActivity)

			// Instances
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
		admin.Use(middleware.AuthRequired(authService), middleware.AdminOnly())
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
		}
	}

	return router
}
