package database

import (
	"fmt"
	"log"
	"time"

	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func InitDB(cfg *config.Config) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	// Retry connection a few times if database is still starting up
	for attempts := 1; attempts <= 5; attempts++ {
		db, err = gorm.Open(postgres.Open(cfg.DatabaseURL), gormConfig)
		if err == nil {
			break
		}
		log.Printf("Connecting to database attempt %d/5 failed: %v. Retrying in 2s...", attempts, err)
		time.Sleep(2 * time.Second)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Auto migrate tables
	err = db.AutoMigrate(
		&models.User{},
		&models.ResourceQuota{},
		&models.Node{},
		&models.OSTemplate{},
		&models.ResourcePlan{},
		&models.SSHKey{},
		&models.Instance{},
		&models.InstanceSnapshot{},
		&models.ProvisioningJob{},
		&models.ResourceReservation{},
		&models.AuditLog{},
		&models.Application{},
		&models.Deployment{},
		&models.ApplicationEnvVar{},
		&models.GitConnection{},
		&models.AutomationInstance{},
		&models.AutomationBackup{},
		&models.OpenClawInstance{},
		&models.OpenClawIntegration{},
		&models.OpenClawBackup{},
		&models.OSImage{},
		&models.StorageBucket{},
		&models.StorageObject{},
		&models.StorageAccessKey{},
		&models.StorageSharedLink{},
		&models.AIAssistantMessage{},
		&models.AIConversation{},
		&models.AIMessage{},
		&models.AIConversationState{},
		&models.AIResourcePlan{},
		&models.AIPlanResource{},
		&models.AIApproval{},
		&models.AIToolInvocation{},
		&models.AIAgentRun{},
		&models.MCPServer{},
		&models.MCPConnection{},
		&models.MCPTool{},
		&models.MCPPermission{},
		&models.MCPAuditLog{},
	)
	if err != nil {
		log.Printf("Notice: auto-migration note: %v", err)
	}

	// Schema migrations for OAuth & integrations
	_ = db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_provider VARCHAR(32) DEFAULT 'local'").Error
	_ = db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS auth_id VARCHAR(255)").Error
	_ = db.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS avatar_url VARCHAR(512)").Error
	_ = db.Exec("ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL").Error

	log.Println("Database connection established and migrations applied successfully.")

	// Seed default data if empty
	if err := Seed(db); err != nil {
		log.Printf("Warning: failed to seed database: %v", err)
	}

	return db, nil
}
