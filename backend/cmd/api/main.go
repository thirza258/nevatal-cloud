package main

import (
	"fmt"
	"log"

	"github.com/nevatal/cloud/internal/api"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/database"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/services"
)

func main() {
	log.Println("Starting Nevatal Cloud Control Plane API...")

	// 1. Load Configuration
	cfg := config.Load()
	log.Printf("Loaded configuration: Environment=%s, Port=%s, ProxmoxMock=%t, ProxmoxNode=%s",
		cfg.Env, cfg.Port, cfg.ProxmoxMock, cfg.ProxmoxNodeName)

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

	// 4. Initialize Core & Extended Services
	authService := services.NewAuthService(db, cfg)
	quotaService := services.NewQuotaService(db)
	sshKeyService := services.NewSSHKeyService(db)
	auditService := services.NewAuditService(db)
	workerService := services.NewWorkerService(db, cfg, pveClient, quotaService)
	instanceService := services.NewInstanceService(db, quotaService, auditService, workerService, pveClient)
	appService := services.NewAppService(db, auditService)
	automationService := services.NewAutomationService(db, auditService)
	openClawService := services.NewOpenClawService(db, auditService)
	isoService := services.NewISOService(db, auditService)
	storageService := services.NewStorageService(db, "")
	portService := services.NewPortService(db)
	gameServerService := services.NewGameServerService(db, cfg, portService, auditService, quotaService, pveClient)
	androidService := services.NewAndroidService(db, cfg, auditService, quotaService, pveClient)
	aiAssistantService := services.NewAIAssistantService(db, cfg, instanceService, quotaService, auditService)
	mcpService := services.NewMCPService(db, instanceService, appService, automationService, openClawService, storageService, gameServerService, androidService, quotaService, pveClient)
	aiAgentService := services.NewAIAgentService(db, cfg, instanceService, appService, automationService, openClawService, storageService, gameServerService, androidService, quotaService, mcpService, pveClient)

	// 5. Initialize Router & Endpoints
	router := api.SetupRouter(api.RouterParams{
		Cfg:                cfg,
		DB:                 db,
		PVE:                pveClient,
		AuthService:        authService,
		QuotaService:       quotaService,
		SSHKeyService:      sshKeyService,
		AuditService:       auditService,
		InstanceService:    instanceService,
		WorkerService:      workerService,
		AppService:         appService,
		AutomationService:  automationService,
		OpenClawService:    openClawService,
		ISOService:         isoService,
		StorageService:     storageService,
		GameServerService:  gameServerService,
		AndroidService:     androidService,
		AIAssistantService: aiAssistantService,
		MCPService:         mcpService,
		AIAgentService:     aiAgentService,
	})

	// 6. Start HTTP Server
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Nevatal Cloud Control Plane listening on http://0.0.0.0%s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
