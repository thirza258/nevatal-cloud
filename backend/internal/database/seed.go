package database

import (
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

func Seed(db *gorm.DB) error {
	// 1. Seed Nodes (Dell PowerEdge R640)
	var nodeCount int64
	db.Model(&models.Node{}).Count(&nodeCount)
	var defaultNode models.Node
	if nodeCount == 0 {
		now := time.Now()
		defaultNode = models.Node{
			Name:              "Dell PowerEdge R640 Node 01",
			ProxmoxNodeName:   "pve-dell-r640",
			Status:            "online",
			TotalCPUThreads:   64,
			TotalMemoryMB:     262144, // 256 GB
			TotalDiskGB:       3840,   // ~3.8 TB
			LastHealthCheckAt: &now,
		}
		if err := db.Create(&defaultNode).Error; err != nil {
			return err
		}
		log.Println("Seeded primary Dell PowerEdge R640 Proxmox node.")
	} else {
		db.First(&defaultNode)
	}

	// 2. Seed OS Templates
	var templateCount int64
	db.Model(&models.OSTemplate{}).Count(&templateCount)
	var ubuntuTemplate models.OSTemplate
	if templateCount == 0 {
		templates := []models.OSTemplate{
			{
				Name:                "Ubuntu Server 24.04 LTS (Noble Numbat)",
				Distribution:        "ubuntu",
				Version:             "24.04 LTS",
				Description:         "Official Ubuntu Cloud image with cloud-init pre-installed and optimized for cloud workloads.",
				ProxmoxTemplateVMID: 9000,
				Architecture:        "x86_64",
				CloudInitSupported:  true,
				DefaultUsername:     "ubuntu",
				Enabled:             true,
			},
			{
				Name:                "Debian 12 / 13 (Bookworm / Trixie)",
				Distribution:        "debian",
				Version:             "12 / 13",
				Description:         "Minimal, rock-solid Debian Linux server image with cloud-init automation enabled.",
				ProxmoxTemplateVMID: 9001,
				Architecture:        "x86_64",
				CloudInitSupported:  true,
				DefaultUsername:     "debian",
				Enabled:             true,
			},
			{
				Name:                "Alpine Linux 3.20 Standard",
				Distribution:        "alpine",
				Version:             "3.20",
				Description:         "Ultra-lightweight Linux distribution oriented around security, simplicity, and resource efficiency.",
				ProxmoxTemplateVMID: 9002,
				Architecture:        "x86_64",
				CloudInitSupported:  true,
				DefaultUsername:     "alpine",
				Enabled:             true,
			},
		}
		for _, t := range templates {
			if err := db.Create(&t).Error; err != nil {
				return err
			}
		}
		log.Println("Seeded official OS templates.")
		ubuntuTemplate = templates[0]
	} else {
		db.First(&ubuntuTemplate)
	}

	// Ensure WordPress 6.7 LEMP stack template is seeded
	var wpCount int64
	db.Model(&models.OSTemplate{}).Where("name ILIKE ?", "%wordpress%").Count(&wpCount)
	if wpCount == 0 {
		wpTemplate := models.OSTemplate{
			Name:                "WordPress 6.7 (Ubuntu 24.04 LEMP Stack)",
			Distribution:        "ubuntu",
			Version:             "WordPress 6.7 / PHP 8.3 / MariaDB",
			Description:         "Pre-configured WordPress production stack with Nginx, PHP 8.3-FPM, MariaDB database, and WP-CLI for instant publishing.",
			ProxmoxTemplateVMID: 9003,
			Architecture:        "x86_64",
			CloudInitSupported:  true,
			DefaultUsername:     "ubuntu",
			Enabled:             true,
		}
		if err := db.Create(&wpTemplate).Error; err == nil {
			log.Println("Seeded WordPress 6.7 LEMP Application Template.")
		}
	}

	// 3. Seed Resource Plans
	var planCount int64
	db.Model(&models.ResourcePlan{}).Count(&planCount)
	if planCount == 0 {
		plans := []models.ResourcePlan{
			{
				Name:        "Nano",
				Description: "Lightweight sandbox for microservices, learning, and testing.",
				VCPUs:       1,
				MemoryMB:    1024,
				DiskGB:      20,
				Enabled:     true,
			},
			{
				Name:        "Starter",
				Description: "Great for basic web servers, small bots, and staging environments.",
				VCPUs:       2,
				MemoryMB:    2048,
				DiskGB:      40,
				Enabled:     true,
			},
			{
				Name:        "Developer",
				Description: "Recommended for full-stack applications, databases, and continuous workloads.",
				VCPUs:       2,
				MemoryMB:    4096,
				DiskGB:      60,
				Enabled:     true,
			},
			{
				Name:        "Pro",
				Description: "High-performance compute for production workloads and busy APIs.",
				VCPUs:       4,
				MemoryMB:    8192,
				DiskGB:      100,
				Enabled:     true,
			},
		}
		for _, p := range plans {
			if err := db.Create(&p).Error; err != nil {
				return err
			}
		}
		log.Println("Seeded resource plans (Nano, Starter, Developer, Pro).")
	}

	// 4. Seed Admin User
	var adminUser models.User
	if err := db.Where("email = ?", "admin@nevatal.id").First(&adminUser).Error; err != nil {
		adminPass, _ := bcrypt.GenerateFromPassword([]byte("AdminPass123!"), bcrypt.DefaultCost)
		now := time.Now()
		adminUser = models.User{
			Email:           "admin@nevatal.id",
			PasswordHash:    string(adminPass),
			DisplayName:     "Platform Administrator",
			Role:            models.RoleAdmin,
			Status:          models.UserStatusActive,
			EmailVerifiedAt: &now,
		}
		if err := db.Create(&adminUser).Error; err == nil {
			adminQuota := models.ResourceQuota{
				UserID:       adminUser.ID,
				MaxVCPUs:     32,
				MaxMemoryMB:  65536,
				MaxStorageGB: 1000,
				MaxInstances: 20,
			}
			_ = db.Create(&adminQuota)
			log.Println("Seeded admin user: admin@nevatal.id")
		}
	}

	// 5. Seed Demo Customer User
	var demoUser models.User
	if err := db.Where("email = ?", "demo@nevatal.id").First(&demoUser).Error; err != nil {
		demoPass, _ := bcrypt.GenerateFromPassword([]byte("DemoPass123!"), bcrypt.DefaultCost)
		now := time.Now()
		demoUser = models.User{
			Email:           "demo@nevatal.id",
			PasswordHash:    string(demoPass),
			DisplayName:     "Dev User",
			Role:            models.RoleCustomer,
			Status:          models.UserStatusActive,
			EmailVerifiedAt: &now,
		}
		if err := db.Create(&demoUser).Error; err == nil {
			demoQuota := models.ResourceQuota{
				UserID:       demoUser.ID,
				MaxVCPUs:     16,
				MaxMemoryMB:  32768,
				MaxStorageGB: 500,
				MaxInstances: 10,
			}
			_ = db.Create(&demoQuota)

			// Add an initial SSH key for demo user
			demoKey := models.SSHKey{
				OwnerID:     demoUser.ID,
				Name:        "macbook-pro",
				PublicKey:   "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIExampleNevatalCloudDevKeyForDemoPurposesOnly thirzq@macbook",
				Fingerprint: "SHA256:dGhpcml6cUBuZXZhdGFsLWNsb3VkLWRlbW8ta2V5LXNoYQ",
			}
			_ = db.Create(&demoKey)

			// Create 1 initial demo instance
			privateIP := "10.10.42.105"
			demoInstance := models.Instance{
				OwnerID:      demoUser.ID,
				NodeID:       defaultNode.ID,
				OSTemplateID: ubuntuTemplate.ID,
				ProxmoxVMID:  1001,
				Name:         "web-app-production",
				Hostname:     "web-app-prod",
				Status:       models.StateRunning,
				VCPUs:        2,
				MemoryMB:     4096,
				DiskGB:       60,
				PrivateIP:    &privateIP,
				SSHUsername:  "ubuntu",
			}
			_ = db.Create(&demoInstance)
			log.Println("Seeded demo user: demo@nevatal.id")
		}
	}

	// 5. Seed OS Images (ISO Catalog per Section 14)
	var isoCount int64
	db.Model(&models.OSImage{}).Count(&isoCount)
	if isoCount == 0 {
		isos := []models.OSImage{
			{
				Name:           "Ubuntu Server",
				Version:        "24.04 LTS (Noble Numbat)",
				Architecture:   "x86_64",
				Category:       "Linux",
				Description:    "Standard Ubuntu Server ISO media for bare-metal and custom VM installations.",
				Filename:       "ubuntu-24.04-live-server-amd64.iso",
				FileSizeBytes:  2791728742,
				SHA256:         "8762f7e13e51025263fde003c53d9bc297046aa7e474b19b3e682502279450c7",
				ProxmoxStorage: "local",
				ProxmoxVolume:  "local:iso/ubuntu-24.04-live-server-amd64.iso",
				Visibility:     "published",
				Status:         "available",
			},
			{
				Name:           "Debian GNU/Linux",
				Version:        "13 (Trixie)",
				Architecture:   "x86_64",
				Category:       "Linux",
				Description:    "Debian Netinst installation image for lean custom server deployments.",
				Filename:       "debian-13-amd64-netinst.iso",
				FileSizeBytes:  658505728,
				SHA256:         "3f9b2361ac864de086a987ef1c0e357904834ff2497645155949cb3cf3526154",
				ProxmoxStorage: "local",
				ProxmoxVolume:  "local:iso/debian-13-amd64-netinst.iso",
				Visibility:     "published",
				Status:         "available",
			},
			{
				Name:           "Windows Server",
				Version:        "2025 Evaluation",
				Architecture:   "x86_64",
				Category:       "Windows",
				Description:    "Official Microsoft Windows Server 2025 180-day evaluation ISO.",
				Filename:       "windows-server-2025-x64-eval.iso",
				FileSizeBytes:  5497558138,
				SHA256:         "a952d1921356bb7889e1b213b1940a049448102431fa2a74c7ffbc01831846b1",
				ProxmoxStorage: "local",
				ProxmoxVolume:  "local:iso/windows-server-2025-x64-eval.iso",
				Visibility:     "published",
				Status:         "available",
			},
		}
		for _, img := range isos {
			_ = db.Create(&img)
		}
		log.Println("Seeded published OS ISO images.")
	}

	// 6. Seed Demo Application (Git-based Website Deployment per Section 3)
	var appCount int64
	db.Model(&models.Application{}).Count(&appCount)
	if appCount == 0 && demoUser.ID != uuid.Nil {
		now := time.Now()
		app := models.Application{
			OwnerID:              demoUser.ID,
			Name:                 "Next.js Portfolio",
			Slug:                 "nextjs-portfolio",
			ApplicationType:      "nextjs",
			DeploymentSourceType: "git",
			RepositoryURL:        "https://github.com/nevatal/portfolio-demo",
			ProductionBranch:     "main",
			RootDirectory:        "/",
			BuildCommand:         "npm run build",
			OutputDirectory:      ".next",
			StartCommand:         "npm run start",
			ApplicationPort:      3000,
			HealthCheckPath:      "/",
			AssignedSubdomain:    "portfolio.apps.nevatal.id",
			Status:               "running",
			VCPUs:                2,
			MemoryMB:             2048,
			DiskGB:               15,
			CreatedAt:            now.Add(-24 * time.Hour),
			UpdatedAt:            now,
		}
		if err := db.Create(&app).Error; err == nil {
			dep := models.Deployment{
				ApplicationID:      app.ID,
				SourceCommit:       "7f2081a94bc1230e95ff8129034821f00821543a",
				CommitMessage:      "feat: update landing page and responsive layout",
				SourceBranch:       "main",
				Status:             "active",
				BuildLogs:          "[info] Cloning repository\n[info] Installing dependencies with npm ci\n[info] Building Next.js production bundle\n[info] Compiled successfully in 14.2s\n[info] Health check passed at / (HTTP 200)\n[success] Deployment active at https://portfolio.apps.nevatal.id",
				ArtifactReference:  "artifacts/apps/nextjs-portfolio-7f2081a.tar.gz",
				RuntimeImageDigest: "sha256:88fa2b109312014e75741a2fe9e6",
				StartedAt:          &now,
				CompletedAt:        &now,
				CreatedAt:          now,
			}
			_ = db.Create(&dep)

			env1 := models.ApplicationEnvVar{
				ApplicationID: app.ID,
				Key:           "NODE_ENV",
				Value:         "production",
				IsSecret:      false,
			}
			env2 := models.ApplicationEnvVar{
				ApplicationID: app.ID,
				Key:           "DATABASE_URL",
				Value:         "postgresql://user:secret@localhost:5432/app",
				IsSecret:      true,
			}
			_ = db.Create(&env1)
			_ = db.Create(&env2)
			log.Println("Seeded sample Git-deployed application: Next.js Portfolio")
		}
	}

	// 7. Seed Demo n8n Automation Instance (per Section 7)
	var n8nCount int64
	db.Model(&models.AutomationInstance{}).Count(&n8nCount)
	if n8nCount == 0 && demoUser.ID != uuid.Nil {
		now := time.Now()
		n8n := models.AutomationInstance{
			OwnerID:              demoUser.ID,
			Name:                 "Marketing & CRM Workflows",
			N8nVersion:           "1.71.0",
			Status:               "running",
			Hostname:             "automation.apps.nevatal.id",
			DashboardURL:         "https://automation.apps.nevatal.id",
			EncryptionKey:        "n8n-enc-key-stable-production-grade-2026",
			DatabaseName:         "n8n_marketing",
			CPULimit:             2,
			MemoryLimitMB:        2048,
			StorageLimitGB:       20,
			StorageUsedGB:        3.4,
			ActiveWorkflowsCount: 6,
			TotalExecutions:      1428,
			LastHealthCheckAt:    &now,
			CreatedAt:            now.Add(-48 * time.Hour),
			UpdatedAt:            now,
		}
		if err := db.Create(&n8n).Error; err == nil {
			backup := models.AutomationBackup{
				AutomationInstanceID: n8n.ID,
				Name:                 "weekly-automated-backup-2026-10-28",
				BackupReference:      "backups/n8n/marketing-20261028.dump",
				SizeBytes:            45219800,
				Status:               "completed",
				CreatedAt:            now.Add(-2 * 24 * time.Hour),
			}
			_ = db.Create(&backup)
			log.Println("Seeded n8n automation instance.")
		}
	}

	// 8. Seed Demo OpenClaw AI Agent Instance (per Section 2.3)
	var ocCount int64
	db.Model(&models.OpenClawInstance{}).Count(&ocCount)
	if ocCount == 0 && demoUser.ID != uuid.Nil {
		now := time.Now()
		oc := models.OpenClawInstance{
			OwnerID:           demoUser.ID,
			Name:              "Executive AI Assistant",
			Version:           "2.4.0",
			Status:            "running",
			Hostname:          "agent.apps.nevatal.id",
			GatewayEndpoint:   "https://agent.apps.nevatal.id/gateway",
			GatewayAuthToken:  "gw_sec_live_9941a87b204910",
			WorkspacePath:     "/home/openclaw/workspace",
			AIProvider:        "openai",
			DefaultModel:      "gpt-4o",
			MaskedAPIKey:      "sk-proj-...88F1",
			CPULimit:          2,
			MemoryLimitMB:     4096,
			StorageLimitGB:    30,
			StorageUsedGB:     5.2,
			ActiveSkillsCount: 8,
			TotalInteractions: 312,
			LastHealthCheckAt: &now,
			CreatedAt:         now.Add(-72 * time.Hour),
			UpdatedAt:         now,
		}
		if err := db.Create(&oc).Error; err == nil {
			tg := models.OpenClawIntegration{
				OpenClawInstanceID: oc.ID,
				Channel:            "telegram",
				Identifier:         "@NevatalChiefBot",
				Status:             "connected",
			}
			discord := models.OpenClawIntegration{
				OpenClawInstanceID: oc.ID,
				Channel:            "discord",
				Identifier:         "NevatalAgent#4012",
				Status:             "connected",
			}
			_ = db.Create(&tg)
			_ = db.Create(&discord)
			log.Println("Seeded OpenClaw AI Agent instance.")
		}
	}

	// 9. Seed Demo Object Storage (MinIO S3 Buckets & Files per Section 21-32)
	var bucketCount int64
	db.Model(&models.StorageBucket{}).Count(&bucketCount)
	if bucketCount == 0 && demoUser.ID != uuid.Nil {
		now := time.Now()
		b1 := models.StorageBucket{
			OwnerID:           demoUser.ID,
			Name:              "customer-assets",
			Description:       "Public static website media, images, and videos",
			Visibility:        "public",
			VersioningEnabled: true,
			Region:            "id-central-1",
			TotalObjects:      3,
			TotalSizeBytes:    92485120, // ~92 MB
			CreatedAt:         now.Add(-5 * 24 * time.Hour),
			UpdatedAt:         now,
		}
		if err := db.Create(&b1).Error; err == nil {
			objs := []models.StorageObject{
				{
					BucketID:     b1.ID,
					Key:          "branding/logo.png",
					Filename:     "logo.png",
					FolderPrefix: "branding",
					ContentType:  "image/png",
					SizeBytes:    2457600, // 2.4 MB
					ETag:         "\"9b2cf51a23620e9f1938\"",
					PublicURL:    "https://s3.nevatal.id/customer-assets/branding/logo.png",
					CreatedAt:    now.Add(-4 * 24 * time.Hour),
					UpdatedAt:    now.Add(-4 * 24 * time.Hour),
				},
				{
					BucketID:     b1.ID,
					Key:          "media/campaign.mp4",
					Filename:     "campaign.mp4",
					FolderPrefix: "media",
					ContentType:  "video/mp4",
					SizeBytes:    89128960, // 85 MB
					ETag:         "\"881fa0491029ba7e2123\"",
					PublicURL:    "https://s3.nevatal.id/customer-assets/media/campaign.mp4",
					CreatedAt:    now.Add(-2 * 24 * time.Hour),
					UpdatedAt:    now.Add(-2 * 24 * time.Hour),
				},
				{
					BucketID:     b1.ID,
					Key:          "docs/architecture-proposal.pdf",
					Filename:     "architecture-proposal.pdf",
					FolderPrefix: "docs",
					ContentType:  "application/pdf",
					SizeBytes:    1258240, // 1.2 MB
					ETag:         "\"331fa99001ba7e449201\"",
					PublicURL:    "https://s3.nevatal.id/customer-assets/docs/architecture-proposal.pdf",
					CreatedAt:    now.Add(-1 * 24 * time.Hour),
					UpdatedAt:    now.Add(-1 * 24 * time.Hour),
				},
			}
			for _, obj := range objs {
				_ = db.Create(&obj)
			}

			// Seed Developer S3 Access Key
			key := models.StorageAccessKey{
				OwnerID:         demoUser.ID,
				AccessKeyID:     "NVTLAK98A123049F10",
				SecretAccessKey: "nvtlsk_live_9488a10b42f99014881023ba",
				Description:     "Production application backup key",
				Permissions:     "read_write",
				BucketScope:     "*",
				Status:          "active",
				LastUsedAt:      &now,
				CreatedAt:       now.Add(-3 * 24 * time.Hour),
			}
			_ = db.Create(&key)
			log.Println("Seeded MinIO S3 Object Storage bucket and files.")
		}
	}

	// 8. Seed MCP Servers & Tools (PRD Section 37 & 38)
	if err := seedMCPServers(db); err != nil {
		log.Printf("Warning: failed to seed MCP servers: %v", err)
	}

	// 9. Seed Game Server Templates, Versions & Demo Instances (PRD Section 56 & 65)
	if err := seedGameServers(db); err != nil {
		log.Printf("Warning: failed to seed game servers: %v", err)
	}

	// 10. Seed Android Waydroid Hosting (PRD Section 66-83)
	if err := seedAndroidCloud(db); err != nil {
		log.Printf("Warning: failed to seed android cloud: %v", err)
	}

	return nil
}

func seedMCPServers(db *gorm.DB) error {
	now := time.Now()

	// 1. First-Party Nevatal Cloud MCP Server
	var coreServer models.MCPServer
	if err := db.Where("name = ?", "nevatal-cloud").First(&coreServer).Error; err != nil {
		coreServer = models.MCPServer{
			Name:         "nevatal-cloud",
			DisplayName:  "Nevatal Cloud Control Plane (First-Party)",
			Description:  "First-party Model Context Protocol server exposing approved Nevatal Cloud infrastructure capabilities and hardware orchestration.",
			Transport:    "in_process",
			EndpointURL:  "/api/v1/mcp/rpc",
			IsFirstParty: true,
			Enabled:      true,
			AuthType:     "bearer",
			Icon:         "Zap",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		if err := db.Create(&coreServer).Error; err != nil {
			return err
		}
	}

	coreTools := []models.MCPTool{
		// Resource discovery
		{ServerID: coreServer.ID, Name: "cloud.list_available_os_images", Description: "List available approved Linux OS templates and ISOs", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_instance_plans", Description: "List standardized compute sizing plans (Nano, Small, Medium, Large)", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_resource_quotas", Description: "Inspect user quota limits and remaining allocations", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_available_capacity", Description: "Check Dell PowerEdge R640 cluster capacity", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_regions", Description: "List available virtualization nodes and regions", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_network_profiles", Description: "List network bridges and public/private IP configurations", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},

		// VPS management
		{ServerID: coreServer.ID, Name: "cloud.list_vms", Description: "List all compute instances owned by the user", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_vm", Description: "Get details and status of a specific virtual machine", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.plan_vm_creation", Description: "Prepare and dry-run validate a virtual machine creation plan", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"template_id":{"type":"string"},"vcpus":{"type":"integer"},"memory_mb":{"type":"integer"},"disk_gb":{"type":"integer"}},"required":["name","template_id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.create_vm", Description: "Provision a new virtual machine from an approved OS template", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"template_id":{"type":"string"},"vcpus":{"type":"integer"},"memory_mb":{"type":"integer"},"disk_gb":{"type":"integer"},"ssh_key_id":{"type":"string"}},"required":["name","template_id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.start_vm", Description: "Power on a virtual machine", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.stop_vm", Description: "Gracefully shut down a virtual machine", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.restart_vm", Description: "Reboot an active virtual machine", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_vm_metrics", Description: "Fetch live CPU, RAM, disk, and network metrics", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.delete_vm", Description: "Permanently destroy a virtual machine and its attached disks", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"confirm_name":{"type":"string"}},"required":["id","confirm_name"]}`, Enabled: true, CreatedAt: now},

		// Application deployment
		{ServerID: coreServer.ID, Name: "cloud.list_applications", Description: "List deployed web applications and git repositories", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.create_application", Description: "Configure a new application repository and runtime", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"framework":{"type":"string"},"git_repository":{"type":"string"},"branch":{"type":"string"},"domain":{"type":"string"}},"required":["name","git_repository"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.deploy_application", Description: "Trigger an automated build and deployment", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_deployment_status", Description: "Check application container health and uptime", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_deployment_logs", Description: "Retrieve build, runtime, and reverse-proxy logs", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"lines":{"type":"integer"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.rollback_deployment", Description: "Rollback application to a previous deployment release", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"deployment_id":{"type":"string"}},"required":["id","deployment_id"]}`, Enabled: true, CreatedAt: now},

		// Automation & AI agents
		{ServerID: coreServer.ID, Name: "cloud.list_n8n_instances", Description: "List running n8n workflow automation instances", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.create_n8n_instance", Description: "Provision a dedicated n8n workflow automation server", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"domain":{"type":"string"}},"required":["name"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_n8n_status", Description: "Check n8n execution engine health", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_openclaw_instances", Description: "List OpenClaw autonomous AI agent instances", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.create_openclaw_instance", Description: "Deploy an OpenClaw AI agent container", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"ai_provider":{"type":"string"},"model_name":{"type":"string"}},"required":["name"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_openclaw_status", Description: "Inspect OpenClaw agent runtime and channels", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.restart_openclaw_instance", Description: "Restart an OpenClaw agent instance", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},

		// Object storage
		{ServerID: coreServer.ID, Name: "cloud.list_buckets", Description: "List MinIO S3-compatible storage buckets", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.create_bucket", Description: "Create a new private or public S3 bucket", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"is_public":{"type":"boolean"}},"required":["name"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_storage_usage", Description: "Inspect bucket disk usage and object count", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"bucket_id":{"type":"string"}},"required":["bucket_id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.generate_presigned_upload", Description: "Generate temporary presigned upload URL", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"bucket_id":{"type":"string"},"filename":{"type":"string"}},"required":["bucket_id","filename"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.generate_presigned_download", Description: "Generate temporary presigned download link", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"object_id":{"type":"string"}},"required":["object_id"]}`, Enabled: true, CreatedAt: now},

		// Monitoring
		{ServerID: coreServer.ID, Name: "cloud.get_resource_usage", Description: "Get cluster-wide or tenant-wide resource utilization", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_service_health", Description: "Inspect service health and ping status", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_recent_events", Description: "Inspect audit and provisioning events", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"limit":{"type":"integer"}}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_provisioning_status", Description: "Track in-progress provisioning job status", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"job_id":{"type":"string"}},"required":["job_id"]}`, Enabled: true, CreatedAt: now},

		// Game server hosting per PRD Section 58
		{ServerID: coreServer.ID, Name: "cloud.list_game_types", Description: "List supported game types and software engines", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_game_versions", Description: "List supported game versions for a game type", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"game_type":{"type":"string"}}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_game_templates", Description: "List published game server deployment templates", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"game_type":{"type":"string"}}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_game_server_plans", Description: "List predefined game server resource tiers", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_game_server_capacity", Description: "Check cluster port allocation and memory capacity", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_game_servers", Description: "List customer game server instances", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"game_type":{"type":"string"}}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_game_server", Description: "Get details and status of a game server", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.plan_game_server_creation", Description: "Prepare and dry-run validate a game server proposal", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"game_type":{"type":"string"},"plan":{"type":"string"}},"required":["name","game_type"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.create_game_server", Description: "Provision a new multiplayer game server", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"game_type":{"type":"string"},"plan_name":{"type":"string"},"difficulty":{"type":"string"}},"required":["name","game_type"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.start_game_server", Description: "Start an inactive game server", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.stop_game_server", Description: "Gracefully shut down a game server", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.restart_game_server", Description: "Restart a game server instance", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.delete_game_server", Description: "Permanently delete a game server and release ports", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_game_server_status", Description: "Check online player count and status", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_game_server_metrics", Description: "Fetch live TPS, MSPT, CPU, and RAM telemetry", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.get_players", Description: "Get current players and whitelist for Minecraft", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.get_configuration", Description: "Fetch Minecraft server.properties settings", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.update_configuration", Description: "Update Minecraft server.properties settings", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"settings":{"type":"object"}},"required":["id","settings"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.get_console_logs", Description: "Fetch live console logs from Minecraft server", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.execute_command", Description: "Execute admin command in Minecraft console", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"command":{"type":"string"}},"required":["id","command"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.get_plugins", Description: "List installed plugins on Paper/Purpur server", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.install_plugin", Description: "Install approved plugin JAR onto Minecraft server", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"name":{"type":"string"}},"required":["id","name"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.remove_plugin", Description: "Remove a plugin from Minecraft server", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"plugin_id":{"type":"string"}},"required":["id","plugin_id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.create_world_backup", Description: "Create persistent world snapshot for Minecraft", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"backup_name":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.minecraft.restore_world_backup", Description: "Restore Minecraft world from a backup archive", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"backup_id":{"type":"string"}},"required":["id","backup_id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.terraria.get_players", Description: "List connected players on Terraria server", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.terraria.get_configuration", Description: "Fetch Terraria server config", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.terraria.update_configuration", Description: "Update Terraria server config", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"settings":{"type":"object"}},"required":["id","settings"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.terraria.get_console_logs", Description: "Fetch live console logs from Terraria server", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.terraria.execute_command", Description: "Execute admin command in Terraria server", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"command":{"type":"string"}},"required":["id","command"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.terraria.create_world_backup", Description: "Create persistent world snapshot for Terraria", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"backup_name":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.terraria.restore_world_backup", Description: "Restore Terraria world from a backup archive", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"backup_id":{"type":"string"}},"required":["id","backup_id"]}`, Enabled: true, CreatedAt: now},

		// Waydroid Android Hosting per PRD Section 77
		{ServerID: coreServer.ID, Name: "cloud.list_android_images", Description: "List published Waydroid system and vendor images", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_android_plans", Description: "List Android hardware sizing tiers (Standard, Dev, QA, Power)", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_android_capacity", Description: "Inspect Waydroid worker node capacity and binder driver status", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.list_android_instances", Description: "List customer Android container instances", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"status":{"type":"string"}}}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.get_android_instance", Description: "Get details and status of an Android container instance", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.plan_android_instance", Description: "Validate and dry-run proposal for an Android instance", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"image_id":{"type":"string"},"plan":{"type":"string"}},"required":["name"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.create_android_instance", Description: "Provision a new Waydroid Android container instance", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"name":{"type":"string"},"image_id":{"type":"string"},"plan_name":{"type":"string"}},"required":["name","image_id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.start_android_instance", Description: "Start a stopped Android instance", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.stop_android_instance", Description: "Stop an active Android instance", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.restart_android_instance", Description: "Restart an Android container instance", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.delete_android_instance", Description: "Permanently delete an Android instance and persistent storage", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.list_applications", Description: "List installed Android apps and packages", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.install_apk", Description: "Install an APK package onto the Android environment", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"app_name":{"type":"string"},"apk_url":{"type":"string"}},"required":["id","app_name"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.uninstall_application", Description: "Uninstall an APK package from the Android instance", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"package_name":{"type":"string"}},"required":["id","package_name"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.get_application_info", Description: "Inspect metadata and permissions of an installed APK", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"package_name":{"type":"string"}},"required":["id","package_name"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.get_metrics", Description: "Fetch live CPU, RAM, storage and FPS metrics for Android", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.get_health", Description: "Check Waydroid container health and IP endpoint", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.get_logs", Description: "Fetch live logcat and container logs from Android instance", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"lines":{"type":"integer"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.get_session_status", Description: "Inspect active browser WebRTC/canvas remote display sessions", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.create_backup", Description: "Create snapshot backup of Android persistent user data", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"backup_name":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.list_backups", Description: "List snapshot backups for an Android instance", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"id":{"type":"string"}},"required":["id"]}`, Enabled: true, CreatedAt: now},
		{ServerID: coreServer.ID, Name: "cloud.android.restore_backup", Description: "Restore persistent user data from a snapshot backup", RiskLevel: "destructive", InputSchema: `{"type":"object","properties":{"id":{"type":"string"},"backup_id":{"type":"string"}},"required":["id","backup_id"]}`, Enabled: true, CreatedAt: now},
	}

	for _, tool := range coreTools {
		tool.ServerID = coreServer.ID
		var existing models.MCPTool
		if err := db.Where("server_id = ? AND name = ?", coreServer.ID, tool.Name).First(&existing).Error; err != nil {
			_ = db.Create(&tool)
		}
	}

	// 2. Curated External MCP Servers
	extServers := []struct {
		Server models.MCPServer
		Tools  []models.MCPTool
	}{
		{
			Server: models.MCPServer{
				Name:         "github",
				DisplayName:  "GitHub Provider",
				Description:  "Inspect repositories, branches, review commits, and trigger approved deployment workflows.",
				Transport:    "http",
				EndpointURL:  "https://api.github.com/mcp",
				IsFirstParty: false,
				Enabled:      true,
				AuthType:     "api_key",
				Icon:         "Github",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			Tools: []models.MCPTool{
				{Name: "github.list_repos", Description: "List authorized repositories and organizations", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
				{Name: "github.get_branches", Description: "List branches and tags in a repository", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"repo":{"type":"string"}},"required":["repo"]}`, Enabled: true, CreatedAt: now},
				{Name: "github.get_latest_commit", Description: "Inspect latest commit hash and author details", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"repo":{"type":"string"},"branch":{"type":"string"}},"required":["repo"]}`, Enabled: true, CreatedAt: now},
				{Name: "github.trigger_workflow", Description: "Dispatch a GitHub Actions deployment workflow", RiskLevel: "write", InputSchema: `{"type":"object","properties":{"repo":{"type":"string"},"workflow_id":{"type":"string"}},"required":["repo","workflow_id"]}`, Enabled: true, CreatedAt: now},
			},
		},
		{
			Server: models.MCPServer{
				Name:         "gitlab",
				DisplayName:  "GitLab CI/CD Integration",
				Description:  "Review GitLab projects, pipeline statuses, and triggers.",
				Transport:    "http",
				EndpointURL:  "https://gitlab.com/api/v4/mcp",
				IsFirstParty: false,
				Enabled:      true,
				AuthType:     "api_key",
				Icon:         "Gitlab",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			Tools: []models.MCPTool{
				{Name: "gitlab.list_projects", Description: "List authorized GitLab repositories", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
				{Name: "gitlab.get_pipelines", Description: "Check CI/CD pipeline build status", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"project_id":{"type":"string"}},"required":["project_id"]}`, Enabled: true, CreatedAt: now},
			},
		},
		{
			Server: models.MCPServer{
				Name:         "datadog-monitoring",
				DisplayName:  "Datadog & Cloud Observability",
				Description:  "Retrieve external cluster telemetry and active incident alerts.",
				Transport:    "http",
				EndpointURL:  "https://api.datadoghq.com/mcp",
				IsFirstParty: false,
				Enabled:      true,
				AuthType:     "api_key",
				Icon:         "Activity",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			Tools: []models.MCPTool{
				{Name: "datadog.get_metrics", Description: "Retrieve host CPU, RAM, and network metrics", RiskLevel: "read", InputSchema: `{"type":"object","properties":{"query":{"type":"string"}},"required":["query"]}`, Enabled: true, CreatedAt: now},
				{Name: "datadog.list_active_alerts", Description: "Check currently firing monitoring alerts", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
			},
		},
		{
			Server: models.MCPServer{
				Name:         "postgres-inspector",
				DisplayName:  "PostgreSQL Schema Inspector",
				Description:  "Inspect database schemas, query table structures, and verify connection health safely.",
				Transport:    "in_process",
				EndpointURL:  "/api/v1/mcp/postgres",
				IsFirstParty: false,
				Enabled:      true,
				AuthType:     "none",
				Icon:         "Database",
				CreatedAt:    now,
				UpdatedAt:    now,
			},
			Tools: []models.MCPTool{
				{Name: "postgres.list_tables", Description: "List public tables and view schema details", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
				{Name: "postgres.check_health", Description: "Test database connectivity and connection pool status", RiskLevel: "read", InputSchema: `{"type":"object","properties":{}}`, Enabled: true, CreatedAt: now},
			},
		},
	}

	for _, ext := range extServers {
		s := ext.Server
		var existing models.MCPServer
		if err := db.Where("name = ?", s.Name).First(&existing).Error; err != nil {
			if err := db.Create(&s).Error; err == nil {
				for _, t := range ext.Tools {
					t.ServerID = s.ID
					_ = db.Create(&t)
				}
			}
		}
	}

	// 3. Auto-connect customer users to core server and github
	var targetUsers []models.User
	db.Where("email IN ?", []string{"customer@nevatal.local", "demo@nevatal.id"}).Find(&targetUsers)
	for _, targetUser := range targetUsers {
		var coreConn models.MCPConnection
		if err := db.Where("user_id = ? AND server_id = ?", targetUser.ID, coreServer.ID).First(&coreConn).Error; err != nil {
			coreConn = models.MCPConnection{
				UserID:          targetUser.ID,
				ServerID:        coreServer.ID,
				Status:          "connected",
				ConfigJSON:      `{"token":"internal-tenant-session"}`,
				LastHealthCheck: &now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			_ = db.Create(&coreConn)
		}

		// Grant permissions for all core tools
		var tools []models.MCPTool
		db.Where("server_id = ?", coreServer.ID).Find(&tools)
		for _, t := range tools {
			var perm models.MCPPermission
			if err := db.Where("connection_id = ? AND tool_id = ?", coreConn.ID, t.ID).First(&perm).Error; err != nil {
				_ = db.Create(&models.MCPPermission{
					ConnectionID: coreConn.ID,
					ToolID:       t.ID,
					Granted:      true,
					CreatedAt:    now,
				})
			}
		}
		// Connect GitHub
		var ghServer models.MCPServer
		if err := db.First(&ghServer, "name = ?", "github").Error; err == nil {
			ghConn := models.MCPConnection{
				UserID:          targetUser.ID,
				ServerID:        ghServer.ID,
				Status:          "connected",
				ConfigJSON:      `{"connected_account":"customer-demo","scopes":["repo","read:user"]}`,
				LastHealthCheck: &now,
				CreatedAt:       now,
				UpdatedAt:       now,
			}
			if err := db.Create(&ghConn).Error; err == nil {
				var ghTools []models.MCPTool
				db.Where("server_id = ?", ghServer.ID).Find(&ghTools)
				for _, t := range ghTools {
					_ = db.Create(&models.MCPPermission{
						ConnectionID: ghConn.ID,
						ToolID:       t.ID,
						Granted:      true,
						CreatedAt:    now,
					})
				}
			}
		}
	}

	log.Println("Seeded first-party & external MCP servers, tool definitions, and default connections.")
	return nil
}

func seedGameServers(db *gorm.DB) error {
	var count int64
	db.Model(&models.GameServerTemplate{}).Count(&count)
	if count > 0 {
		return nil
	}

	now := time.Now()

	// 1. Seed Templates per PRD Section 56
	templates := []models.GameServerTemplate{
		// Minecraft Templates
		{
			GameType:          models.GameTypeMinecraftJava,
			Name:              "Vanilla Survival",
			Slug:              "vanilla-survival",
			ServerSoftware:    models.SoftwareVanilla,
			Description:       "Pure official Mojang Minecraft Java server experience without modifications.",
			MinCPU:            1,
			MinRAMMB:          2048,
			MinStorageGB:      10,
			SupportedVersions: "1.21.4, 1.21.1, 1.20.4",
			IsModded:          false,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeMinecraftJava,
			Name:              "Paper Survival",
			Slug:              "paper-survival",
			ServerSoftware:    models.SoftwarePaper,
			Description:       "High-performance Minecraft server with Bukkit, Spigot, and Paper plugin compatibility.",
			MinCPU:            2,
			MinRAMMB:          4096,
			MinStorageGB:      20,
			SupportedVersions: "1.21.4, 1.21.1, 1.20.4",
			IsModded:          false,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeMinecraftJava,
			Name:              "Creative World",
			Slug:              "minecraft-creative",
			ServerSoftware:    models.SoftwarePaper,
			Description:       "Optimized creative mode world template with flight enabled and builder utilities.",
			MinCPU:            2,
			MinRAMMB:          4096,
			MinStorageGB:      20,
			SupportedVersions: "1.21.4, 1.21.1",
			IsModded:          false,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeMinecraftJava,
			Name:              "Fabric Modded",
			Slug:              "fabric-modded",
			ServerSoftware:    models.SoftwareFabric,
			Description:       "Modern, lightweight modded Minecraft platform with fast startup times and extensive mod catalog.",
			MinCPU:            3,
			MinRAMMB:          6144,
			MinStorageGB:      40,
			SupportedVersions: "1.21.4, 1.21.1, 1.20.4",
			IsModded:          true,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeMinecraftJava,
			Name:              "Forge Modded",
			Slug:              "forge-modded",
			ServerSoftware:    models.SoftwareForge,
			Description:       "Classic Minecraft Forge environment designed for heavy technical and RPG modpacks.",
			MinCPU:            4,
			MinRAMMB:          8192,
			MinStorageGB:      80,
			SupportedVersions: "1.20.4, 1.19.4",
			IsModded:          true,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeMinecraftBedrock,
			Name:              "Bedrock Dedicated Server",
			Slug:              "minecraft-bedrock",
			ServerSoftware:    models.SoftwareBedrock,
			Description:       "Official Bedrock Dedicated Server supporting Windows 10/11, iOS, Android, and consoles.",
			MinCPU:            2,
			MinRAMMB:          4096,
			MinStorageGB:      20,
			SupportedVersions: "1.21.51, 1.21.50",
			IsModded:          false,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},

		// Terraria Templates
		{
			GameType:          models.GameTypeTerraria,
			Name:              "Classic Survival",
			Slug:              "terraria-classic",
			ServerSoftware:    models.SoftwareTerraria,
			Description:       "Standard Terraria multiplayer server for relaxed exploration, building, and progression.",
			MinCPU:            1,
			MinRAMMB:          2048,
			MinStorageGB:      10,
			SupportedVersions: "1.4.4.9",
			IsModded:          false,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeTerraria,
			Name:              "Expert World",
			Slug:              "terraria-expert",
			ServerSoftware:    models.SoftwareTerraria,
			Description:       "Challenging multiplayer world with smarter enemy AI, increased boss stats, and unique expert loot drops.",
			MinCPU:            2,
			MinRAMMB:          4096,
			MinStorageGB:      20,
			SupportedVersions: "1.4.4.9",
			IsModded:          false,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeTerraria,
			Name:              "Master World",
			Slug:              "terraria-master",
			ServerSoftware:    models.SoftwareTerraria,
			Description:       "Highest standard Terraria difficulty level with exclusive master relics and gold trophies.",
			MinCPU:            2,
			MinRAMMB:          4096,
			MinStorageGB:      20,
			SupportedVersions: "1.4.4.9",
			IsModded:          false,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeTerraria,
			Name:              "Large World Explorer",
			Slug:              "terraria-large-world",
			ServerSoftware:    models.SoftwareTerraria,
			Description:       "Massive procedural map generation for larger teams and extensive subterranean mining.",
			MinCPU:            3,
			MinRAMMB:          6144,
			MinStorageGB:      40,
			SupportedVersions: "1.4.4.9",
			IsModded:          false,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			GameType:          models.GameTypeTerraria,
			Name:              "tModLoader Modded",
			Slug:              "terraria-tmodloader",
			ServerSoftware:    models.SoftwareTModLoader,
			Description:       "Extensible tModLoader server supporting Calamity, Thorium, and Steam Workshop mod packages.",
			MinCPU:            3,
			MinRAMMB:          6144,
			MinStorageGB:      40,
			SupportedVersions: "1.4.4.9",
			IsModded:          true,
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}

	for _, tpl := range templates {
		_ = db.Create(&tpl)
	}

	// 2. Seed Supported Versions
	versions := []models.GameServerVersion{
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwarePaper, VersionString: "1.21.4", IsLatestStable: true, ReleaseDate: "2024-12-03", JavaVersion: "21", Enabled: true},
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwarePaper, VersionString: "1.21.1", IsLatestStable: false, ReleaseDate: "2024-08-08", JavaVersion: "21", Enabled: true},
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwarePaper, VersionString: "1.20.4", IsLatestStable: false, ReleaseDate: "2023-12-07", JavaVersion: "17", Enabled: true},
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwareVanilla, VersionString: "1.21.4", IsLatestStable: true, ReleaseDate: "2024-12-03", JavaVersion: "21", Enabled: true},
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwareVanilla, VersionString: "1.21.1", IsLatestStable: false, ReleaseDate: "2024-08-08", JavaVersion: "21", Enabled: true},
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwareFabric, VersionString: "1.21.4", IsLatestStable: true, ReleaseDate: "2024-12-03", JavaVersion: "21", Enabled: true},
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwareFabric, VersionString: "1.21.1", IsLatestStable: false, ReleaseDate: "2024-08-08", JavaVersion: "21", Enabled: true},
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwareForge, VersionString: "1.20.4", IsLatestStable: true, ReleaseDate: "2023-12-07", JavaVersion: "17", Enabled: true},
		{GameType: models.GameTypeMinecraftJava, ServerSoftware: models.SoftwareForge, VersionString: "1.19.4", IsLatestStable: false, ReleaseDate: "2023-03-14", JavaVersion: "17", Enabled: true},
		{GameType: models.GameTypeMinecraftBedrock, ServerSoftware: models.SoftwareBedrock, VersionString: "1.21.51", IsLatestStable: true, ReleaseDate: "2024-12-10", Enabled: true},
		{GameType: models.GameTypeMinecraftBedrock, ServerSoftware: models.SoftwareBedrock, VersionString: "1.21.50", IsLatestStable: false, ReleaseDate: "2024-11-12", Enabled: true},
		{GameType: models.GameTypeTerraria, ServerSoftware: models.SoftwareTerraria, VersionString: "1.4.4.9", IsLatestStable: true, ReleaseDate: "2022-11-17", Enabled: true},
		{GameType: models.GameTypeTerraria, ServerSoftware: models.SoftwareTModLoader, VersionString: "1.4.4.9", IsLatestStable: true, ReleaseDate: "2023-06-01", Enabled: true},
	}

	for _, ver := range versions {
		_ = db.Create(&ver)
	}

	// 3. Seed Demo Minecraft Server for Customer Demo User
	var demoUser models.User
	if err := db.First(&demoUser, "email = ?", "demo@nevatal.id").Error; err == nil {
		mcID := uuid.New()
		mcPort := 25565
		mcVMID := 25565

		mcServer := models.GameServer{
			ID:                mcID,
			OwnerID:           demoUser.ID,
			Name:              "Nevatal SMP Community",
			Slug:              "nevatal-smp-community",
			GameType:          models.GameTypeMinecraftJava,
			GameVersion:       "1.21.4",
			ServerSoftware:    models.SoftwarePaper,
			Status:            models.GameServerStatusRunning,
			PlanName:          "basic",
			CPUCores:          2,
			MemoryMB:          4096,
			StorageGB:         20,
			PlayerSlots:       20,
			AssignedPort:      mcPort,
			QueryPort:         mcPort,
			RCONPort:          25575,
			RCONPassword:      "nevatal-rcon-secret",
			PublicAddress:     "cloud.nevatal.id:25565",
			MOTD:              "§bNevatal Cloud§r | Survival SMP §a[1.21.4 Paper]§r",
			ProxmoxVMID:       &mcVMID,
			NodeName:          "pve-dell-r640",
			UptimeSeconds:     86400 * 3, // 3 days
			TPS:               20.0,
			MSPT:              12.4,
			OnlinePlayerCount: 3,
			AutoBackupEnabled: true,
			LastHealthCheckAt: &now,
			CreatedAt:         now.Add(-72 * time.Hour),
			UpdatedAt:         now,
		}
		if err := db.Create(&mcServer).Error; err == nil {
			// Port allocation
			_ = db.Create(&models.GameServerPort{
				ServerID:    mcID,
				PortNumber:  mcPort,
				Protocol:    "tcp",
				ServiceType: "game",
				NodeName:    "pve-dell-r640",
				IsAllocated: true,
				AllocatedAt: now,
			})

			// Active World
			_ = db.Create(&models.GameServerWorld{
				ServerID:   mcID,
				WorldName:  "world",
				IsActive:   true,
				SizeBytes:  1024 * 1024 * 145, // 145 MB
				Seed:       "-7492048219401284",
				Difficulty: "normal",
				CreatedAt:  now.Add(-72 * time.Hour),
				UpdatedAt:  now,
			})

			// Demo Players
			demoPlayers := []models.GameServerPlayer{
				{ServerID: mcID, PlayerName: "AdminOwner", IsOnline: true, IsOperator: true, IsWhitelisted: true, FirstJoinedAt: &now, LastSeenAt: &now},
				{ServerID: mcID, PlayerName: "Alex_Builder", IsOnline: true, IsOperator: false, IsWhitelisted: true, FirstJoinedAt: &now, LastSeenAt: &now},
				{ServerID: mcID, PlayerName: "SteveMiner99", IsOnline: true, IsOperator: false, IsWhitelisted: true, FirstJoinedAt: &now, LastSeenAt: &now},
				{ServerID: mcID, PlayerName: "GrieferBot", IsOnline: false, IsBanned: true, BanReason: "Attempted automated block disruption", FirstJoinedAt: &now, LastSeenAt: &now},
			}
			for _, p := range demoPlayers {
				_ = db.Create(&p)
			}

			// Demo Plugins
			demoPlugins := []models.GameServerPlugin{
				{ServerID: mcID, Name: "EssentialsX", FileName: "EssentialsX-2.20.1.jar", Version: "2.20.1", Author: "EssentialsX Team", IsEnabled: true, IsCompatible: true, FileSizeBytes: 1024 * 1024 * 3, CreatedAt: now},
				{ServerID: mcID, Name: "CoreProtect", FileName: "CoreProtect-22.4.jar", Version: "22.4", Author: "Intelli", IsEnabled: true, IsCompatible: true, FileSizeBytes: 1024 * 1024 * 2, CreatedAt: now},
				{ServerID: mcID, Name: "LuckPerms", FileName: "LuckPerms-5.4.102.jar", Version: "5.4.102", Author: "Luck", IsEnabled: true, IsCompatible: true, FileSizeBytes: 1024 * 1024 * 4, CreatedAt: now},
			}
			for _, pl := range demoPlugins {
				_ = db.Create(&pl)
			}

			// Demo Backup
			_ = db.Create(&models.GameServerBackup{
				ServerID:      mcID,
				UserID:        demoUser.ID,
				BackupName:    "backup-nevatal-smp-daily-01",
				FileSizeBytes: 1024 * 1024 * 128,
				StoragePath:   fmt.Sprintf("/storage/backups/game-servers/%s/backup-01.tar.gz", mcID),
				WorldName:     "world",
				IsAutomated:   true,
				Checksum:      "sha256-e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				Status:        "completed",
				CreatedAt:     now.Add(-24 * time.Hour),
			})
		}

		// 4. Seed Demo Terraria Server
		terID := uuid.New()
		terPort := 7777
		terVMID := 27777

		terServer := models.GameServer{
			ID:                terID,
			OwnerID:           demoUser.ID,
			Name:              "Terraria Expert Realm",
			Slug:              "terraria-expert-realm",
			GameType:          models.GameTypeTerraria,
			GameVersion:       "1.4.4.9",
			ServerSoftware:    models.SoftwareTerraria,
			Status:            models.GameServerStatusRunning,
			PlanName:          "basic",
			CPUCores:          2,
			MemoryMB:          4096,
			StorageGB:         20,
			PlayerSlots:       8,
			AssignedPort:      terPort,
			QueryPort:         terPort,
			PublicAddress:     "cloud.nevatal.id:7777",
			MOTD:              "Nevatal Cloud | Terraria Expert World",
			ProxmoxVMID:       &terVMID,
			NodeName:          "pve-dell-r640",
			UptimeSeconds:     86400 * 2,
			TPS:               60.0,
			MSPT:              8.1,
			OnlinePlayerCount: 2,
			AutoBackupEnabled: true,
			LastHealthCheckAt: &now,
			CreatedAt:         now.Add(-48 * time.Hour),
			UpdatedAt:         now,
		}
		if err := db.Create(&terServer).Error; err == nil {
			_ = db.Create(&models.GameServerPort{
				ServerID:    terID,
				PortNumber:  terPort,
				Protocol:    "tcp",
				ServiceType: "game",
				NodeName:    "pve-dell-r640",
				IsAllocated: true,
				AllocatedAt: now,
			})

			_ = db.Create(&models.GameServerWorld{
				ServerID:   terID,
				WorldName:  "Aethelgard",
				IsActive:   true,
				SizeBytes:  1024 * 1024 * 42,
				Difficulty: "expert",
				WorldSize:  "large",
				CreatedAt:  now.Add(-48 * time.Hour),
				UpdatedAt:  now,
			})

			_ = db.Create(&models.GameServerPlayer{
				ServerID:      terID,
				PlayerName:    "TerrarianWarrior",
				IsOnline:      true,
				IsOperator:    true,
				FirstJoinedAt: &now,
				LastSeenAt:    &now,
			})
		}
	}

	log.Println("Seeded game server templates, versions, and sample instances.")
	return nil
}

func seedAndroidCloud(db *gorm.DB) error {
	now := time.Now()
	var demoUser models.User
	db.Where("email = ?", "demo@nevatal.id").First(&demoUser)

	// 5. Seed Android Waydroid Hosting (PRD Section 66-83)
	var androidWorkerCount int64
	db.Model(&models.AndroidWorkerNode{}).Count(&androidWorkerCount)
	var androidWorker models.AndroidWorkerNode
	if androidWorkerCount == 0 {
		androidWorker = models.AndroidWorkerNode{
			Name:                 "Waydroid Worker Node 01",
			ProxmoxNodeName:      "pve-dell-r640",
			IPAddress:            "192.168.10.10",
			Status:               "ready",
			KernelVersion:        "6.8.12-waydroid-pve",
			BinderStatus:         "active",
			TotalCPUCores:        32,
			TotalMemoryMB:        131072,
			TotalStorageGB:       2048,
			ActiveInstancesCount: 1,
			LastHealthCheckAt:    now,
			CreatedAt:            now,
			UpdatedAt:            now,
		}
		_ = db.Create(&androidWorker)
		log.Println("Seeded Android Waydroid worker node.")
	} else {
		db.First(&androidWorker)
	}

	var androidImgCount int64
	db.Model(&models.AndroidImage{}).Count(&androidImgCount)
	var defaultAndroidImg models.AndroidImage
	if androidImgCount == 0 {
		img1 := models.AndroidImage{
			Name:           "LineageOS 20 (Android 13)",
			Slug:           "lineageos-20-android-13",
			AndroidVersion: "13",
			Architecture:   "x86_64",
			ImageType:      models.AndroidImageTypeStandard,
			SystemImageURL: "https://images.nevatal.id/waydroid/system-20.0-vanilla.img",
			VendorImageURL: "https://images.nevatal.id/waydroid/vendor-20.0-mainline.img",
			ImageSource:    "official",
			ChecksumSHA256: "a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef0",
			MinCPU:         2,
			MinRAMMB:       4096,
			MinStorageGB:   32,
			IsPublished:    true,
			IsVerified:     true,
			Description:    "Modern Android 13 LineageOS 20 container image optimized for cloud desktop and high-performance testing.",
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		_ = db.Create(&img1)
		defaultAndroidImg = img1

		_ = db.Create(&models.AndroidImageVersion{
			ImageID:        img1.ID,
			VersionTag:     "v20.0-20240901-VANILLA",
			Channel:        "stable",
			ReleaseNotes:   "Official LineageOS 20 Waydroid mainline release with Linux binder integration.",
			SystemChecksum: img1.ChecksumSHA256,
			VendorChecksum: img1.ChecksumSHA256,
			IsActive:       true,
			CreatedAt:      now,
		})

		img2 := models.AndroidImage{
			Name:           "LineageOS 18.1 (Android 11)",
			Slug:           "lineageos-18-android-11",
			AndroidVersion: "11",
			Architecture:   "x86_64",
			ImageType:      models.AndroidImageTypeStandard,
			SystemImageURL: "https://images.nevatal.id/waydroid/system-18.1-vanilla.img",
			VendorImageURL: "https://images.nevatal.id/waydroid/vendor-18.1-mainline.img",
			ImageSource:    "official",
			ChecksumSHA256: "b2c3d4e5f6a17890123456789abcdef0123456789abcdef0123456789abcdef1",
			MinCPU:         1,
			MinRAMMB:       2048,
			MinStorageGB:   16,
			IsPublished:    true,
			IsVerified:     true,
			Description:    "Lightweight Android 11 container image for enterprise QA and backwards compatibility testing.",
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		_ = db.Create(&img2)
		_ = db.Create(&models.AndroidImageVersion{
			ImageID:        img2.ID,
			VersionTag:     "v18.1-20240815-VANILLA",
			Channel:        "stable",
			ReleaseNotes:   "Official LineageOS 18.1 Waydroid release for legacy app validation.",
			SystemChecksum: img2.ChecksumSHA256,
			VendorChecksum: img2.ChecksumSHA256,
			IsActive:       true,
			CreatedAt:      now,
		})

		img3 := models.AndroidImage{
			Name:           "Android 12 Developer & Testing",
			Slug:           "android-12-developer",
			AndroidVersion: "12",
			Architecture:   "x86_64",
			ImageType:      models.AndroidImageTypeDeveloper,
			SystemImageURL: "https://images.nevatal.id/waydroid/system-12-dev.img",
			VendorImageURL: "https://images.nevatal.id/waydroid/vendor-12-dev.img",
			ImageSource:    "approved_custom",
			ChecksumSHA256: "c3d4e5f6a1b27890123456789abcdef0123456789abcdef0123456789abcdef2",
			MinCPU:         2,
			MinRAMMB:       4096,
			MinStorageGB:   32,
			IsPublished:    true,
			IsVerified:     true,
			Description:    "Developer preview build with root access, pre-configured ADB over TLS, and debug instrumentation.",
			CreatedAt:      now,
			UpdatedAt:      now,
		}
		_ = db.Create(&img3)
		_ = db.Create(&models.AndroidImageVersion{
			ImageID:        img3.ID,
			VersionTag:     "v12.0-developer-preview",
			Channel:        "beta",
			ReleaseNotes:   "Developer tooling release with root privileges and ADB pre-authorized.",
			SystemChecksum: img3.ChecksumSHA256,
			VendorChecksum: img3.ChecksumSHA256,
			IsActive:       true,
			CreatedAt:      now,
		})

		log.Println("Seeded official Waydroid Android images and versions.")
	} else {
		db.First(&defaultAndroidImg)
	}

	// Seed Demo Android Instance for demoUser
	var androidInstCount int64
	db.Model(&models.AndroidInstance{}).Count(&androidInstCount)
	if androidInstCount == 0 && demoUser.ID != uuid.Nil && defaultAndroidImg.ID != uuid.Nil {
		instID := uuid.New()
		sampleToken := "demo-sess-tok-" + uuid.New().String()[:12]
		workerID := androidWorker.ID

		demoInstance := models.AndroidInstance{
			ID:                 instID,
			OwnerID:            demoUser.ID,
			ImageID:            defaultAndroidImg.ID,
			WorkerNodeID:       &workerID,
			Name:               "Demo Android Device",
			Slug:               "demo-android-device",
			Status:             models.AndroidStatusRunning,
			PlanName:           "developer",
			VCPUs:              2,
			MemoryMB:           4096,
			StorageGB:          32,
			DisplayResolution:  "1280x720",
			DisplayDensity:     240,
			Orientation:        "portrait",
			RemoteAccessType:   "browser",
			AccessURL:          fmt.Sprintf("https://cloud.nevatal.id/dashboard/android/%s/console", instID),
			SessionToken:       sampleToken,
			IPAddress:          "10.192.0.15",
			GatewayPort:        6080,
			ADBEnabled:         true,
			ADBPort:            5555,
			PersistenceEnabled: true,
			AutoBackupEnabled:  true,
			BackupSchedule:     "daily",
			UptimeSeconds:      86400 * 3,
			CreatedAt:          now.Add(-72 * time.Hour),
			UpdatedAt:          now,
		}
		if err := db.Create(&demoInstance).Error; err == nil {
			// Persistent Data Volume
			_ = db.Create(&models.AndroidDataVolume{
				InstanceID:  instID,
				VolumeName:  "data-volume-userdata",
				MountPath:   "/data",
				SizeBytes:   1024 * 1024 * 1024 * 8, // 8 GB used
				ZFSDataset:  "zfs-pool/android/" + instID.String(),
				CreatedAt:   now.Add(-72 * time.Hour),
				UpdatedAt:   now,
			})

			// Pre-installed sample applications
			sampleApps := []models.AndroidApplication{
				{InstanceID: instID, PackageName: "com.android.chrome", AppName: "Google Chrome", VersionName: "128.0.6613.88", FileSizeBytes: 1024 * 1024 * 85, IsSystemApp: false, InstalledAt: now.Add(-70 * time.Hour)},
				{InstanceID: instID, PackageName: "com.termux", AppName: "Termux Terminal", VersionName: "0.118.0", FileSizeBytes: 1024 * 1024 * 35, IsSystemApp: false, InstalledAt: now.Add(-60 * time.Hour)},
				{InstanceID: instID, PackageName: "com.nevatal.cloudagent", AppName: "Nevatal Mobile QA Companion", VersionName: "1.0.4", FileSizeBytes: 1024 * 1024 * 12, IsSystemApp: false, InstalledAt: now.Add(-48 * time.Hour)},
			}
			for _, app := range sampleApps {
				_ = db.Create(&app)
			}

			// Active Session
			_ = db.Create(&models.AndroidSession{
				InstanceID:   instID,
				UserID:       demoUser.ID,
				SessionToken: sampleToken,
				AccessType:   "browser",
				ClientIP:     "127.0.0.1",
				IsActive:     true,
				ExpiresAt:    now.Add(2 * time.Hour),
				StartedAt:    now,
			})

			// Sample Backup
			_ = db.Create(&models.AndroidBackup{
				InstanceID:     instID,
				UserID:         demoUser.ID,
				BackupName:     "demo-android-full-backup-01",
				BackupType:     "snapshot",
				FileSizeBytes:  1024 * 1024 * 450, // 450 MB
				StoragePath:    fmt.Sprintf("/storage/backups/android/%s/backup-01.tar.zst", instID),
				ChecksumSHA256: "d4e5f6a1b2c37890123456789abcdef0123456789abcdef0123456789abcdef3",
				IsAutomated:    true,
				Status:         "completed",
				RetentionDays:  30,
				CreatedAt:      now.Add(-24 * time.Hour),
			})
		}
	}

	log.Println("Seeded Android Waydroid images, worker nodes, and demo instance.")
	return nil
}

