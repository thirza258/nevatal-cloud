package database

import (
	"log"
	"time"

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

	return nil
}
