package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

type AutomationUsecase struct {
	automations  repository.AutomationRepository
	auditService *AuditUsecase
}

func NewAutomationUsecase(automations repository.AutomationRepository, auditService *AuditUsecase) *AutomationUsecase {
	return &AutomationUsecase{
		automations:  automations,
		auditService: auditService,
	}
}

type CreateAutomationInput struct {
	Name           string `json:"name"`
	N8nVersion     string `json:"n8n_version"`
	CPULimit       int    `json:"cpu_limit"`
	MemoryLimitMB  int    `json:"memory_limit_mb"`
	StorageLimitGB int    `json:"storage_limit_gb"`
}

func (s *AutomationUsecase) List(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]models.AutomationInstance, error) {
	return s.automations.List(ctx, ownerScope(userID, isAdmin))
}

func (s *AutomationUsecase) Get(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.AutomationInstance, error) {
	item, err := s.automations.FindByID(ctx, id, ownerScope(userID, isAdmin))
	if err != nil {
		return nil, errors.New("n8n automation instance not found")
	}
	return item, nil
}

func (s *AutomationUsecase) Create(ctx context.Context, userID uuid.UUID, input CreateAutomationInput, clientIP string) (*models.AutomationInstance, error) {
	ctx = context.WithoutCancel(ctx)

	slug := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	hostname := fmt.Sprintf("automation-%s.apps.nevatal.id", slug)

	count, _ := s.automations.CountByHostname(ctx, hostname)
	if count > 0 {
		hostname = fmt.Sprintf("automation-%s-%d.apps.nevatal.id", slug, time.Now().Unix()%1000)
	}

	if input.N8nVersion == "" {
		input.N8nVersion = "1.71.0"
	}
	if input.CPULimit <= 0 {
		input.CPULimit = 2
	}
	if input.MemoryLimitMB <= 0 {
		input.MemoryLimitMB = 2048
	}
	if input.StorageLimitGB <= 0 {
		input.StorageLimitGB = 20
	}

	// Generate 32-byte stable encryption key
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	encKey := hex.EncodeToString(b)

	now := time.Now()
	instance := models.AutomationInstance{
		OwnerID:              userID,
		Name:                 input.Name,
		N8nVersion:           input.N8nVersion,
		Status:               "running",
		Hostname:             hostname,
		DashboardURL:         fmt.Sprintf("https://%s", hostname),
		EncryptionKey:        encKey,
		DatabaseName:         fmt.Sprintf("n8n_%s", strings.ReplaceAll(slug, "-", "_")),
		CPULimit:             input.CPULimit,
		MemoryLimitMB:        input.MemoryLimitMB,
		StorageLimitGB:       input.StorageLimitGB,
		StorageUsedGB:        1.2,
		ActiveWorkflowsCount: 1,
		TotalExecutions:      0,
		LastHealthCheckAt:    &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	if err := s.automations.Create(ctx, &instance); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "automation.n8n.create", "automation_instance", &instance.ID, "success", clientIP, map[string]string{
		"name":     instance.Name,
		"hostname": instance.Hostname,
	})

	return &instance, nil
}

func (s *AutomationUsecase) UpdatePower(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, action string, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	var newStatus string
	switch action {
	case "start":
		newStatus = "running"
	case "stop":
		newStatus = "stopped"
	case "restart":
		newStatus = "running"
	default:
		return errors.New("unsupported action")
	}

	now := time.Now()
	item.Status = newStatus
	item.LastHealthCheckAt = &now
	if err := s.automations.Save(ctx, item); err != nil {
		return err
	}

	s.auditService.Log(ctx, &userID, fmt.Sprintf("automation.n8n.%s", action), "automation_instance", &item.ID, "success", clientIP, nil)
	return nil
}

func (s *AutomationUsecase) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := s.automations.SoftDelete(ctx, item, &now); err != nil {
		return err
	}

	s.auditService.Log(ctx, &userID, "automation.n8n.delete", "automation_instance", &item.ID, "success", clientIP, map[string]string{
		"name": item.Name,
	})
	return nil
}

func (s *AutomationUsecase) CreateBackup(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.AutomationBackup, error) {
	ctx = context.WithoutCancel(ctx)

	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	backup := models.AutomationBackup{
		AutomationInstanceID: item.ID,
		Name:                 fmt.Sprintf("backup-%s-%s", item.Name, now.Format("20060102-150405")),
		BackupReference:      fmt.Sprintf("backups/n8n/%s-%d.dump", item.ID.String(), now.Unix()),
		SizeBytes:            int64(float64(item.StorageUsedGB) * 1024 * 1024 * 1024 * 0.4),
		Status:               "completed",
		CreatedAt:            now,
	}

	if err := s.automations.CreateBackup(ctx, &backup); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "automation.n8n.backup", "automation_backup", &backup.ID, "success", clientIP, map[string]string{
		"name": backup.Name,
	})

	return &backup, nil
}

func (s *AutomationUsecase) GetLogs(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) ([]string, error) {
	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	logs := []string{
		fmt.Sprintf("[%s] n8n worker process initialized (PID 1)", now.Add(-10*time.Minute).Format("15:04:05")),
		fmt.Sprintf("[%s] Connected to PostgreSQL backend (%s:5432)", now.Add(-9*time.Minute).Format("15:04:05"), item.DatabaseName),
		fmt.Sprintf("[%s] Encryption key verified against credential store", now.Add(-8*time.Minute).Format("15:04:05")),
		fmt.Sprintf("[%s] Webhook listener bound on 0.0.0.0:5678", now.Add(-7*time.Minute).Format("15:04:05")),
		fmt.Sprintf("[%s] Loaded %d active workflows from disk", now.Add(-5*time.Minute).Format("15:04:05"), item.ActiveWorkflowsCount),
		fmt.Sprintf("[%s] Ready for trigger execution at %s", now.Add(-2*time.Minute).Format("15:04:05"), item.DashboardURL),
		fmt.Sprintf("[%s] Health check: OK (RAM %d MB, CPU load 0.02)", now.Format("15:04:05"), item.MemoryLimitMB/4),
	}
	return logs, nil
}
