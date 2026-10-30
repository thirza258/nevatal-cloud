package database

import (
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
				MaxVCPUs:     4,
				MaxMemoryMB:  8192,
				MaxStorageGB: 100,
				MaxInstances: 3,
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

	return nil
}
