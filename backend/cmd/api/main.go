// Command api is the Nevatal Cloud control plane HTTP API. main is the
// composition root: config -> database -> repositories -> use cases ->
// handlers (router). Nothing else constructs dependencies.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nevatal/cloud/internal/api"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/database"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
	"github.com/nevatal/cloud/internal/secret"
	"github.com/nevatal/cloud/internal/usecase"
)

func main() {
	log.Println("Starting Nevatal Cloud Control Plane API...")

	// 1. Load and validate configuration (production fails fast on unsafe settings)
	cfg := config.Load()
	log.Printf("Loaded configuration: Version=%s, Environment=%s, DemoMode=%t, Port=%s, ProxmoxMock=%t, ProxmoxNode=%s",
		config.Version, cfg.Env, cfg.DemoMode, cfg.Port, cfg.ProxmoxMock, cfg.ProxmoxNodeName)
	for _, w := range cfg.Warnings {
		log.Printf("Config warning: %s", w)
	}
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Refusing to start: %v", err)
	}
	if cfg.DemoMode {
		log.Println("DEMO_MODE is on: public demo accounts, role switcher and mock social login are enabled; the hypervisor is simulated.")
	}

	// 2. Initialize Database and Auto-Migrations
	db, err := database.InitDB(cfg)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}

	// 3. Initialize Proxmox Client (Real API or Simulation Mode)
	pveClient := proxmox.NewClient(cfg)
	if cfg.ProxmoxMock {
		log.Println("Proxmox VE integration running in SIMULATION/MOCK mode. (Set PROXMOX_MOCK=false and supply credentials for live hypervisor).")
	} else {
		log.Printf("Proxmox VE integration configured for real host: %s:%s (Node: %s)", cfg.ProxmoxHost, cfg.ProxmoxPort, cfg.ProxmoxNodeName)
	}

	// 4. Repositories (persistence)
	tx := repository.NewTransactor(db)
	auditRepo := repository.NewAuditRepository(db)
	userRepo := repository.NewUserRepository(db)
	quotaRepo := repository.NewQuotaRepository(db)
	reservationRepo := repository.NewReservationRepository(db)
	instanceRepo := repository.NewInstanceRepository(db)
	jobRepo := repository.NewJobRepository(db)
	templateRepo := repository.NewTemplateRepository(db)
	planRepo := repository.NewPlanRepository(db)
	sshKeyRepo := repository.NewSSHKeyRepository(db)
	nodeRepo := repository.NewNodeRepository(db)
	snapshotRepo := repository.NewSnapshotRepository(db)
	isoRepo := repository.NewISORepository(db)
	storageRepo := repository.NewStorageRepository(db)
	portRepo := repository.NewPortRepository(db)
	gameServerRepo := repository.NewGameServerRepository(db)
	androidRepo := repository.NewAndroidRepository(db)
	appRepo := repository.NewAppRepository(db)
	automationRepo := repository.NewAutomationRepository(db)
	openClawRepo := repository.NewOpenClawRepository(db)
	aiAssistantRepo := repository.NewAIAssistantRepository(db)
	mcpRepo := repository.NewMCPRepository(db)
	aiAgentRepo := repository.NewAIAgentRepository(db)
	domainRepo := repository.NewDomainRepository(db)
	orchRepo := repository.NewOrchestratorRepository(db)
	infraTemplateRepo := repository.NewInfraTemplateRepository(db)
	managedDBRepo := repository.NewDatabaseRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	infraCheckRepo := repository.NewInfraCheckRepository(db)
	policyRepo := repository.NewAutomationPolicyRepository(db)

	// 5. Use cases (business logic)
	auditUC := usecase.NewAuditUsecase(auditRepo)
	authUC := usecase.NewAuthUsecase(tx, userRepo, quotaRepo, auditUC, cfg)
	quotaUC := usecase.NewQuotaUsecase(quotaRepo, instanceRepo, reservationRepo)
	sshKeyUC := usecase.NewSSHKeyUsecase(sshKeyRepo, auditUC)
	catalogUC := usecase.NewCatalogUsecase(templateRepo, planRepo)
	dashboardUC := usecase.NewDashboardUsecase(instanceRepo, quotaUC, auditUC)
	jobUC := usecase.NewJobUsecase(jobRepo)
	workerUC := usecase.NewWorkerUsecase(tx, usecase.WorkerRepos{
		Jobs:      jobRepo,
		Instances: instanceRepo,
		Templates: templateRepo,
		Nodes:     nodeRepo,
	}, cfg, pveClient, quotaUC)
	instanceUC := usecase.NewInstanceUsecase(tx, usecase.InstanceRepos{
		Instances: instanceRepo,
		Jobs:      jobRepo,
		Templates: templateRepo,
		Plans:     planRepo,
		SSHKeys:   sshKeyRepo,
		Nodes:     nodeRepo,
		Snapshots: snapshotRepo,
		Audits:    auditRepo,
	}, quotaUC, auditUC, workerUC, pveClient)
	dockerUC := usecase.NewDockerUsecase(instanceUC, pveClient, auditUC)
	appUC := usecase.NewAppUsecase(tx, appRepo, auditUC)
	automationUC := usecase.NewAutomationUsecase(automationRepo, auditUC)
	openClawUC := usecase.NewOpenClawUsecase(openClawRepo, auditUC)
	isoUC := usecase.NewISOUsecase(isoRepo, auditUC)
	storageUC := usecase.NewStorageUsecase(storageRepo, cfg.StoragePath)
	portUC := usecase.NewPortUsecase(portRepo)
	gameServerUC := usecase.NewGameServerUsecase(gameServerRepo, cfg, portUC, auditUC, quotaUC, pveClient)
	androidUC := usecase.NewAndroidUsecase(androidRepo, cfg, auditUC, quotaUC, pveClient)
	wordPressUC := usecase.NewWordPressUsecase(cfg, usecase.WordPressRepos{
		Nodes:     nodeRepo,
		Instances: instanceRepo,
		Templates: templateRepo,
		Plans:     planRepo,
		Users:     userRepo,
	}, authUC, instanceUC, auditUC, pveClient)
	domainUC := usecase.NewDomainUsecase(tx, usecase.DomainRepos{
		Domains:   domainRepo,
		Instances: instanceRepo,
	}, cfg, auditUC)
	notificationUC := usecase.NewNotificationUsecase(notificationRepo)
	secretCipher := secret.New(cfg.JWTSecret)
	databaseUC := usecase.NewDatabaseUsecase(managedDBRepo, auditUC, quotaUC, cfg, secretCipher)
	resourceUC := usecase.NewResourceUsecase(instanceUC, appUC, automationUC, openClawUC, storageUC, gameServerUC, androidUC, databaseUC, quotaUC)
	policiesUC := usecase.NewAutomationPolicyUsecase(policyRepo, notificationUC, auditUC)
	orchUC := usecase.NewOrchestratorUsecase(tx, usecase.OrchestratorRepos{
		Orch:           orchRepo,
		Agent:          aiAgentRepo,
		Users:          userRepo,
		Templates:      templateRepo,
		Plans:          planRepo,
		SSHKeys:        sshKeyRepo,
		Nodes:          nodeRepo,
		Instances:      instanceRepo,
		Snapshots:      snapshotRepo,
		Jobs:           jobRepo,
		Android:        androidRepo,
		Storage:        storageRepo,
		InfraTemplates: infraTemplateRepo,
		Databases:      managedDBRepo,
		Domains:        domainRepo,
	}, usecase.OrchestratorServices{
		Instance:      instanceUC,
		App:           appUC,
		Automation:    automationUC,
		OpenClaw:      openClawUC,
		Storage:       storageUC,
		GameServer:    gameServerUC,
		Android:       androidUC,
		Database:      databaseUC,
		Domain:        domainUC,
		Quota:         quotaUC,
		Audit:         auditUC,
		Notifications: notificationUC,
		Policies:      policiesUC,
	}, cfg, pveClient)
	policiesUC.SetPlans(orchUC)
	monitoringUC := usecase.NewMonitoringUsecase(infraCheckRepo, notificationUC, policiesUC,
		instanceUC, appUC, gameServerUC, androidUC, databaseUC, quotaUC, cfg)
	monitoringUC.SetInterval(cfg.MonitorInterval)
	databaseUC.SetBackupInterval(cfg.OrchDBBackupInterval)
	// The assistant's action cards become orchestrator plans (never direct
	// provisioning), so it is built after the orchestrator.
	aiAssistantUC := usecase.NewAIAssistantUsecase(usecase.AIAssistantRepos{
		Messages:  aiAssistantRepo,
		Users:     userRepo,
		Templates: templateRepo,
		Plans:     planRepo,
		SSHKeys:   sshKeyRepo,
	}, cfg, instanceUC, quotaUC, auditUC, orchUC)
	mcpUC := usecase.NewMCPUsecase(usecase.MCPRepos{
		MCP:       mcpRepo,
		Templates: templateRepo,
		Plans:     planRepo,
		Nodes:     nodeRepo,
		Instances: instanceRepo,
		Audits:    auditRepo,
		Jobs:      jobRepo,
	}, instanceUC, appUC, automationUC, openClawUC, storageUC, gameServerUC, androidUC, quotaUC, orchUC, pveClient)
	aiAgentUC := usecase.NewAIAgentUsecase(usecase.AIAgentRepos{
		Agent:     aiAgentRepo,
		Templates: templateRepo,
		SSHKeys:   sshKeyRepo,
		Android:   androidRepo,
	}, cfg, instanceUC, appUC, automationUC, openClawUC, storageUC, gameServerUC, androidUC, quotaUC, mcpUC, orchUC, pveClient)
	aiAgentUC.SetDatabaseService(databaseUC)
	adminUC := usecase.NewAdminUsecase(usecase.AdminRepos{
		Users:     userRepo,
		Instances: instanceRepo,
		Jobs:      jobRepo,
		Nodes:     nodeRepo,
		Templates: templateRepo,
		Plans:     planRepo,
	}, quotaUC, auditUC, workerUC, pveClient)
	if cfg.DomainsEnabled() {
		log.Printf("Nevatal Domains enabled: customers publish at https://<subdomain>.%s (edge targets: %v)", cfg.PublicDomainRoot, cfg.EdgeTargetCIDRs)
	}

	// AI orchestrator: revision 1 for plans created before it existed, then
	// the execution engine (every replica runs one; tasks are leased in
	// Postgres) and the stale provisioning-job reaper.
	if n, err := orchUC.BackfillRevisions(context.Background()); err != nil {
		log.Printf("Warning: AI plan revision backfill: %v", err)
	} else if n > 0 {
		log.Printf("AI orchestrator: wrote revision 1 of %d existing plan(s)", n)
	}
	engine := orchUC.Engine()
	engine.Start()
	log.Printf("AI orchestrator engine started (owner %s, concurrency %d, lease %s, heartbeat %s)", engine.Owner(), cfg.OrchConcurrency, cfg.OrchLease, cfg.OrchHeartbeat)
	reaperCtx, stopReaper := context.WithCancel(context.Background())
	defer stopReaper()
	workerUC.StartReaper(reaperCtx)
	log.Printf("Provisioning jobs: heartbeat every %s, failed as %s after %s without one", cfg.JobHeartbeatInterval, models.JobErrorStale, cfg.JobStaleAfter)
	monitorCtx, stopMonitor := context.WithCancel(context.Background())
	defer stopMonitor()
	monitoringUC.Start(monitorCtx)
	log.Printf("Infrastructure checks: monitor loop every %s", cfg.MonitorInterval)
	databaseUC.StartBackups(monitorCtx)
	log.Printf("Managed databases: automatic backup cadence %s", cfg.OrchDBBackupInterval)
	notificationUC.StartRetention(monitorCtx, cfg.NotificationRetention)
	if cfg.NotificationRetention > 0 {
		log.Printf("Notifications: retention %s", cfg.NotificationRetention)
	}

	// Synchronize hypervisor hardware capacity (CPU threads, RAM, storage)
	// from Proxmox VE into node records and admin quotas.
	syncCtx, cancelSync := context.WithTimeout(context.Background(), 15*time.Second)
	if err := adminUC.SyncClusterCapacity(syncCtx); err != nil {
		log.Printf("Warning: cluster capacity sync: %v", err)
	} else {
		log.Printf("Hypervisor node capacity synchronized with Proxmox host")
	}
	cancelSync()

	// Periodic hardware capacity sync (detects physical CPU/RAM/storage upgrades)
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-monitorCtx.Done():
				return
			case <-ticker.C:
				_ = adminUC.SyncClusterCapacity(monitorCtx)
			}
		}
	}()

	// Long-lived responses (SSE) end when the HTTP server starts shutting
	// down, so Shutdown does not wait for them.
	streamCtx, cancelStreams := context.WithCancel(context.Background())
	defer cancelStreams()

	// 6. HTTP router (handlers)
	router := api.SetupRouter(api.RouterParams{
		Cfg:          cfg,
		Auth:         authUC,
		Quota:        quotaUC,
		SSHKey:       sshKeyUC,
		Audit:        auditUC,
		Catalog:      catalogUC,
		Dashboard:    dashboardUC,
		Job:          jobUC,
		Instance:     instanceUC,
		Docker:       dockerUC,
		App:          appUC,
		Automation:   automationUC,
		OpenClaw:     openClawUC,
		ISO:          isoUC,
		Storage:      storageUC,
		GameServer:   gameServerUC,
		Android:      androidUC,
		AIAssistant:  aiAssistantUC,
		MCP:          mcpUC,
		AIAgent:      aiAgentUC,
		Resource:     resourceUC,
		WordPress:    wordPressUC,
		Admin:        adminUC,
		Domain:       domainUC,
		Database:     databaseUC,
		Notification: notificationUC,
		Monitoring:   monitoringUC,
		Policies:     policiesUC,

		Orchestrator: orchUC,
		Streams:      streamCtx,
	})

	// 7. Start HTTP Server with graceful shutdown (SIGTERM from Docker/Kubernetes)
	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	srv.RegisterOnShutdown(cancelStreams)

	serverErr := make(chan error, 1)
	go func() {
		log.Printf("Nevatal Cloud Control Plane listening on http://0.0.0.0%s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-serverErr:
		log.Fatalf("Server failed to start: %v", err)
	case sig := <-stop:
		log.Printf("Received %s, shutting down gracefully...", sig)
	}

	// Stop claiming orchestrator tasks at once; let running tasks, in-flight
	// requests and background provisioning jobs finish within the budget.
	// Keep the total below the container grace period (docker
	// stop_grace_period / terminationGracePeriodSeconds).
	shutdownStart := time.Now()
	engine.StopClaiming()
	stopReaper()
	stopMonitor()
	engineDone := make(chan struct{})
	go func() {
		// Tasks still running after 50 s are stopped without a result and
		// their leases released, so another replica re-claims them at once
		// (same attempt, same idempotency key).
		engineCtx, cancelEngine := context.WithTimeout(context.Background(), 50*time.Second)
		defer cancelEngine()
		engine.Shutdown(engineCtx)
		close(engineDone)
	}()

	httpCtx, cancelHTTP := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancelHTTP()
	if err := srv.Shutdown(httpCtx); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}

	jobsCtx, cancelJobs := context.WithDeadline(context.Background(), shutdownStart.Add(55*time.Second))
	defer cancelJobs()
	if !workerUC.Wait(jobsCtx) {
		log.Println("Warning: provisioning jobs were still running at shutdown and were interrupted; the stale-job reaper of another replica fails them (JOB_STALE) and the orchestrator retries.")
	}
	<-engineDone

	if sqlDB, err := db.DB(); err == nil {
		_ = sqlDB.Close()
	}
	log.Println("Shutdown complete.")
}
