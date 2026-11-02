package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nevatal/cloud/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const connectAttempts = 10

// InitDB connects to the database, applies pending migrations and seeds it.
// Only the API calls it; other processes use Connect.
func InitDB(cfg *config.Config) (*gorm.DB, error) {
	db, err := Connect(cfg.DatabaseURL, cfg.DBMaxOpenConns, cfg.DBMaxIdleConns)
	if err != nil {
		return nil, err
	}

	// Migrations and seeding run under one advisory lock so replicas that
	// start together do not race on seed data.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	err = withBootstrapLock(ctx, db, func() error {
		if err := RunMigrations(db); err != nil {
			return fmt.Errorf("failed to apply database migrations: %w", err)
		}
		log.Println("Database connection established and migrations applied successfully.")

		if err := Seed(db, cfg); err != nil {
			if cfg.IsProduction() {
				return fmt.Errorf("failed to seed database: %w", err)
			}
			log.Printf("Warning: failed to seed database: %v", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// Connect opens a connection pool without migrating or seeding, retrying
// while the database is still starting (compose/Kubernetes start order).
func Connect(databaseURL string, maxOpenConns, maxIdleConns int) (*gorm.DB, error) {
	var db *gorm.DB
	var err error

	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn),
	}

	for attempt := 1; attempt <= connectAttempts; attempt++ {
		db, err = gorm.Open(postgres.Open(databaseURL), gormConfig)
		if err == nil {
			break
		}
		wait := time.Duration(attempt) * time.Second
		log.Printf("Connecting to database attempt %d/%d failed: %v. Retrying in %s...", attempt, connectAttempts, err, wait)
		time.Sleep(wait)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to access database pool: %w", err)
	}
	sqlDB.SetMaxOpenConns(maxOpenConns)
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetConnMaxLifetime(30 * time.Minute)
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)
	return db, nil
}
