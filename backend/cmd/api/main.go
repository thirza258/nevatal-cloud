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
	log.Println("Starting Nevatal Cloud API Service...")

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

	// 4. Initialize Core Services
	authService := services.NewAuthService(db, cfg)
	quotaService := services.NewQuotaService(db)
	sshKeyService := services.NewSSHKeyService(db)
	auditService := services.NewAuditService(db)
	workerService := services.NewWorkerService(db, cfg, pveClient, quotaService)
	instanceService := services.NewInstanceService(db, quotaService, auditService, workerService)

	// 5. Initialize Router & Endpoints
	router := api.SetupRouter(
		cfg,
		db,
		pveClient,
		authService,
		quotaService,
		sshKeyService,
		auditService,
		instanceService,
		workerService,
	)

	// 6. Start HTTP Server
	addr := fmt.Sprintf(":%s", cfg.Port)
	log.Printf("Nevatal Cloud Control Plane listening on http://0.0.0.0%s", addr)
	if err := router.Run(addr); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
