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

	// Auto migrate tables if not already initialized
	if !db.Migrator().HasTable(&models.User{}) {
		err = db.AutoMigrate(
			&models.User{},
			&models.ResourceQuota{},
			&models.Node{},
			&models.OSTemplate{},
			&models.ResourcePlan{},
			&models.SSHKey{},
			&models.Instance{},
			&models.ProvisioningJob{},
			&models.ResourceReservation{},
			&models.AuditLog{},
		)
		if err != nil {
			return nil, fmt.Errorf("database auto-migration failed: %w", err)
		}
	}

	log.Println("Database connection established and migrations applied successfully.")

	// Seed default data if empty
	if err := Seed(db); err != nil {
		log.Printf("Warning: failed to seed database: %v", err)
	}

	return db, nil
}
