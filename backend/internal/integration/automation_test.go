package integration

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
	"github.com/nevatal/cloud/internal/usecase"
)

// TestClaimDueBackups proves the automatic backup claim is atomic: a database
// is claimed once, its next_backup_at moves forward, and auto_backup=false is
// never claimed.
func TestClaimDueBackups(t *testing.T) {
	needDB(t)
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	ctx := context.Background()
	repo := repository.NewDatabaseRepository(testDB)

	now := time.Now()
	dueAt := now.Add(-time.Minute)
	makeDB := func(name string, auto bool) *models.ManagedDatabase {
		return &models.ManagedDatabase{
			OwnerID: u.id, Name: name, Engine: models.DatabaseEnginePostgres, EngineVersion: "16",
			Status: models.DatabaseStatusRunning, PlanName: "starter",
			VCPUs: 1, MemoryMB: 1024, StorageGB: 10,
			Host: "db-" + name + ".nevatal.internal", Port: 5432, DatabaseName: name,
			AutoBackup: auto, NextBackupAt: &dueAt, CreatedAt: now, UpdatedAt: now,
		}
	}
	on := makeDB("it-auto-backup", true)
	off := makeDB("it-auto-backup-off", false)
	for _, db := range []*models.ManagedDatabase{on, off} {
		if err := repo.Create(ctx, db); err != nil {
			t.Fatalf("create database: %v", err)
		}
	}

	claimed, err := repo.ClaimDueBackups(ctx, now, 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	if !containsDB(claimed, on.ID) {
		t.Fatal("a due auto-backup database was not claimed")
	}
	if containsDB(claimed, off.ID) {
		t.Fatal("auto_backup=false was claimed")
	}

	// The claim advanced next_backup_at, so a second replica does not take a
	// duplicate backup.
	again, err := repo.ClaimDueBackups(ctx, now.Add(time.Second), 24*time.Hour, 10)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if containsDB(again, on.ID) {
		t.Fatal("the same database was claimed twice")
	}

	// Clearing the row proves next_backup_at is persisted, not only returned.
	reloaded, err := repo.FindByID(ctx, on.ID, u.id)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.NextBackupAt == nil || !reloaded.NextBackupAt.After(now) {
		t.Fatalf("next_backup_at was not advanced: %v", reloaded.NextBackupAt)
	}
}

// TestNotificationPrune deletes only notifications older than the retention
// window.
func TestNotificationPrune(t *testing.T) {
	needDB(t)
	s := newStack(t, stackOpts{noEngine: true})
	u := s.newUser()
	ctx := context.Background()
	repo := repository.NewNotificationRepository(testDB)
	uc := usecase.NewNotificationUsecase(repo)

	old := &models.Notification{UserID: u.id, Type: models.NotificationTypeSystem,
		Severity: models.NotificationSeverityInfo, Title: "old", CreatedAt: time.Now().Add(-48 * time.Hour)}
	fresh := &models.Notification{UserID: u.id, Type: models.NotificationTypeSystem,
		Severity: models.NotificationSeverityInfo, Title: "fresh", CreatedAt: time.Now()}
	for _, n := range []*models.Notification{old, fresh} {
		if err := repo.Create(ctx, n); err != nil {
			t.Fatalf("create notification: %v", err)
		}
	}

	if n := uc.Prune(ctx, 24*time.Hour); n != 1 {
		t.Fatalf("pruned %d rows, want 1", n)
	}
	list, err := repo.List(ctx, u.id, false, 50)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].ID != fresh.ID {
		t.Fatalf("retention kept the wrong rows: %+v", list)
	}
}

func containsDB(list []models.ManagedDatabase, id uuid.UUID) bool {
	for _, db := range list {
		if db.ID == id {
			return true
		}
	}
	return false
}
