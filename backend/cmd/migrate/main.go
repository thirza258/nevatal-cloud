package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/nevatal/cloud/internal/database"
)

func main() {
	_ = godotenv.Load()

	dbURLFlag := flag.String("database", "", "PostgreSQL database URL (defaults to DATABASE_URL env var)")
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		printUsage()
		os.Exit(1)
	}

	dbURL := *dbURLFlag
	if dbURL == "" {
		dbURL = os.Getenv("DATABASE_URL")
	}
	if dbURL == "" {
		log.Fatal("Error: DATABASE_URL must be specified via --database flag or DATABASE_URL environment variable")
	}

	command := args[0]
	switch command {
	case "up":
		log.Println("Applying database migrations (up)...")
		if err := database.RunMigrationsWithURL(dbURL); err != nil {
			log.Fatalf("Migration failed: %v", err)
		}
		log.Println("Migrations applied successfully.")

	case "down":
		steps := 1
		if len(args) > 1 {
			parsedSteps, err := strconv.Atoi(args[1])
			if err != nil || parsedSteps < 1 {
				log.Fatalf("Invalid steps count: %s", args[1])
			}
			steps = parsedSteps
		}
		log.Printf("Rolling back %d database migration step(s)...", steps)
		if err := database.RollbackMigration(dbURL, steps); err != nil {
			log.Fatalf("Rollback failed: %v", err)
		}
		log.Println("Rollback completed successfully.")

	case "version", "status":
		version, dirty, err := database.GetMigrationVersion(dbURL)
		if err != nil {
			log.Fatalf("Failed to get migration version: %v", err)
		}
		fmt.Printf("Current Schema Version: %d (Dirty: %v)\n", version, dirty)

	case "force":
		if len(args) < 2 {
			log.Fatal("Usage: migrate force <version>")
		}
		version, err := strconv.Atoi(args[1])
		if err != nil {
			log.Fatalf("Invalid version: %s", args[1])
		}
		if err := database.ForceMigrationVersion(dbURL, version); err != nil {
			log.Fatalf("Force version failed: %v", err)
		}
		log.Printf("Forced schema version to %d successfully.\n", version)

	case "help":
		printUsage()

	default:
		log.Fatalf("Unknown command: %s. Use 'migrate help' for usage.", command)
	}
}

func printUsage() {
	fmt.Println(`Nevatal Cloud Migration Utility (go-migrate)

Usage:
  nevatal-migrate [flags] <command> [arguments]

Commands:
  up                 Apply all available migrations
  down [steps]       Roll back the latest N migrations (default: 1)
  version            Print current schema version and dirty status
  force <version>    Set database version manually (clears dirty state)
  help               Show this help message

Flags:
  --database <url>   PostgreSQL database connection URL (or DATABASE_URL env var)`)
}
