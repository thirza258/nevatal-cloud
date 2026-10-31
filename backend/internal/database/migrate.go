package database

import (
	"errors"
	"fmt"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/nevatal/cloud/migrations"
	"gorm.io/gorm"
)

// RunMigrations applies all pending database migrations using the active GORM database connection.
func RunMigrations(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	driver, err := postgres.WithInstance(sqlDB, &postgres.Config{
		MigrationsTable: "schema_migrations",
	})
	if err != nil {
		return fmt.Errorf("failed to create postgres migration driver: %w", err)
	}

	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", sourceDriver, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to initialize migrate instance: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		log.Printf("Warning: failed to read migration version: %v", err)
	} else if errors.Is(err, migrate.ErrNilVersion) {
		log.Println("Database migrations applied. (No migrations version recorded)")
	} else {
		log.Printf("Database migrations up to date (version: %d, dirty: %v).", version, dirty)
	}

	return nil
}

// RunMigrationsWithURL applies all pending database migrations by connecting directly to the given database URL.
func RunMigrationsWithURL(databaseURL string) error {
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database for migration: %w", err)
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			log.Printf("Notice closing source driver: %v", srcErr)
		}
		if dbErr != nil {
			log.Printf("Notice closing db driver: %v", dbErr)
		}
	}()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to apply migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		log.Printf("Warning: failed to read migration version: %v", err)
	} else {
		log.Printf("Migrations applied successfully. Database is at version %d (dirty: %v).", version, dirty)
	}

	return nil
}

// RollbackMigration rolls back N migrations.
func RollbackMigration(databaseURL string, steps int) error {
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database for rollback: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-steps); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("failed to rollback %d step(s): %w", steps, err)
	}

	version, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		log.Printf("Warning: failed to read migration version: %v", err)
	} else {
		log.Printf("Rollback complete. Database is at version %d (dirty: %v).", version, dirty)
	}

	return nil
}

// GetMigrationVersion returns the current version and dirty status of the database.
func GetMigrationVersion(databaseURL string) (uint, bool, error) {
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return 0, false, fmt.Errorf("failed to load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURL)
	if err != nil {
		return 0, false, fmt.Errorf("failed to connect to database: %w", err)
	}
	defer m.Close()

	version, dirty, err := m.Version()
	if err != nil {
		return 0, false, err
	}

	return version, dirty, nil
}

// ForceMigrationVersion forces the database to a specific version (useful if a migration failed dirty).
func ForceMigrationVersion(databaseURL string, version int) error {
	sourceDriver, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return fmt.Errorf("failed to load embedded migrations: %w", err)
	}

	m, err := migrate.NewWithSourceInstance("iofs", sourceDriver, databaseURL)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer m.Close()

	if err := m.Force(version); err != nil {
		return fmt.Errorf("failed to force version %d: %w", version, err)
	}

	log.Printf("Successfully forced schema version to %d", version)
	return nil
}
