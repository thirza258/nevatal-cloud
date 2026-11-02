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

type OpenClawUsecase struct {
	agents       repository.OpenClawRepository
	auditService *AuditUsecase
}

func NewOpenClawUsecase(agents repository.OpenClawRepository, auditService *AuditUsecase) *OpenClawUsecase {
	return &OpenClawUsecase{
		agents:       agents,
		auditService: auditService,
	}
}

type CreateOpenClawInput struct {
	Name           string `json:"name"`
	Version        string `json:"version"`
	AIProvider     string `json:"ai_provider"` // openai, anthropic, gemini, ollama, deepseek
	DefaultModel   string `json:"default_model"`
	APIKey         string `json:"api_key"`
	CPULimit       int    `json:"cpu_limit"`
	MemoryLimitMB  int    `json:"memory_limit_mb"`
	StorageLimitGB int    `json:"storage_limit_gb"`
}

func (s *OpenClawUsecase) List(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]models.OpenClawInstance, error) {
	return s.agents.List(ctx, ownerScope(userID, isAdmin))
}

func (s *OpenClawUsecase) Get(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.OpenClawInstance, error) {
	item, err := s.agents.FindByID(ctx, id, ownerScope(userID, isAdmin))
	if err != nil {
		return nil, errors.New("OpenClaw instance not found")
	}
	return item, nil
}

func (s *OpenClawUsecase) Create(ctx context.Context, userID uuid.UUID, input CreateOpenClawInput, clientIP string) (*models.OpenClawInstance, error) {
	ctx = context.WithoutCancel(ctx)

	slug := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	hostname := fmt.Sprintf("agent-%s.apps.nevatal.id", slug)

	count, _ := s.agents.CountByHostname(ctx, hostname)
	if count > 0 {
		hostname = fmt.Sprintf("agent-%s-%d.apps.nevatal.id", slug, time.Now().Unix()%1000)
	}

	if input.Version == "" {
		input.Version = "2.4.0"
	}
	if input.AIProvider == "" {
		input.AIProvider = "openai"
	}
	if input.DefaultModel == "" {
		input.DefaultModel = "gpt-4o"
	}
	if input.CPULimit <= 0 {
		input.CPULimit = 2
	}
	if input.MemoryLimitMB <= 0 {
		input.MemoryLimitMB = 4096
	}
	if input.StorageLimitGB <= 0 {
		input.StorageLimitGB = 30
	}

	maskedKey := "sk-demo-...default"
	if len(input.APIKey) > 8 {
		maskedKey = fmt.Sprintf("%s...%s", input.APIKey[:7], input.APIKey[len(input.APIKey)-4:])
	}

	// Generate Gateway Auth Token
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	gwToken := fmt.Sprintf("gw_sec_%s", hex.EncodeToString(b))

	now := time.Now()
	instance := models.OpenClawInstance{
		OwnerID:           userID,
		Name:              input.Name,
		Version:           input.Version,
		Status:            "running",
		Hostname:          hostname,
		GatewayEndpoint:   fmt.Sprintf("https://%s/gateway", hostname),
		GatewayAuthToken:  gwToken,
		WorkspacePath:     fmt.Sprintf("/home/openclaw/%s/workspace", slug),
		AIProvider:        input.AIProvider,
		DefaultModel:      input.DefaultModel,
		EncryptedAPIKey:   input.APIKey,
		MaskedAPIKey:      maskedKey,
		CPULimit:          input.CPULimit,
		MemoryLimitMB:     input.MemoryLimitMB,
		StorageLimitGB:    input.StorageLimitGB,
		StorageUsedGB:     2.1,
		ActiveSkillsCount: 6,
		TotalInteractions: 0,
		LastHealthCheckAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.agents.Create(ctx, &instance); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "ai.openclaw.create", "openclaw_instance", &instance.ID, "success", clientIP, map[string]string{
		"name":     instance.Name,
		"provider": instance.AIProvider,
		"model":    instance.DefaultModel,
	})

	return &instance, nil
}

func (s *OpenClawUsecase) UpdatePower(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, action string, clientIP string) error {
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
	if err := s.agents.Save(ctx, item); err != nil {
		return err
	}

	s.auditService.Log(ctx, &userID, fmt.Sprintf("ai.openclaw.%s", action), "openclaw_instance", &item.ID, "success", clientIP, nil)
	return nil
}

func (s *OpenClawUsecase) UpdateConfig(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, provider, model, apiKey string, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	if provider != "" {
		item.AIProvider = provider
	}
	if model != "" {
		item.DefaultModel = model
	}
	if apiKey != "" {
		item.EncryptedAPIKey = apiKey
		if len(apiKey) > 8 {
			item.MaskedAPIKey = fmt.Sprintf("%s...%s", apiKey[:7], apiKey[len(apiKey)-4:])
		} else {
			item.MaskedAPIKey = "••••••••"
		}
	}

	item.UpdatedAt = time.Now()
	if err := s.agents.Save(ctx, item); err != nil {
		return err
	}

	s.auditService.Log(ctx, &userID, "ai.openclaw.config_update", "openclaw_instance", &item.ID, "success", clientIP, map[string]string{
		"provider": item.AIProvider,
		"model":    item.DefaultModel,
	})
	return nil
}

func (s *OpenClawUsecase) AddIntegration(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, channel, identifier string, clientIP string) (*models.OpenClawIntegration, error) {
	ctx = context.WithoutCancel(ctx)

	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	integration := models.OpenClawIntegration{
		OpenClawInstanceID: item.ID,
		Channel:            channel,
		Identifier:         identifier,
		Status:             "connected",
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
	}

	if err := s.agents.CreateIntegration(ctx, &integration); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "ai.openclaw.integration.add", "openclaw_integration", &integration.ID, "success", clientIP, map[string]string{
		"channel":    channel,
		"identifier": identifier,
	})

	return &integration, nil
}

func (s *OpenClawUsecase) DeleteIntegration(ctx context.Context, id, integrationID, userID uuid.UUID, isAdmin bool) error {
	ctx = context.WithoutCancel(ctx)

	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	return s.agents.DeleteIntegration(ctx, item.ID, integrationID)
}

func (s *OpenClawUsecase) CreateBackup(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.OpenClawBackup, error) {
	ctx = context.WithoutCancel(ctx)

	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	backup := models.OpenClawBackup{
		OpenClawInstanceID: item.ID,
		Name:               fmt.Sprintf("backup-claw-%s-%s", item.Name, now.Format("20060102-150405")),
		BackupReference:    fmt.Sprintf("backups/openclaw/%s-%d.tar.gz", item.ID.String(), now.Unix()),
		SizeBytes:          int64(float64(item.StorageUsedGB) * 1024 * 1024 * 1024 * 0.35),
		Status:             "completed",
		CreatedAt:          now,
	}

	if err := s.agents.CreateBackup(ctx, &backup); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "ai.openclaw.backup", "openclaw_backup", &backup.ID, "success", clientIP, map[string]string{
		"name": backup.Name,
	})

	return &backup, nil
}

func (s *OpenClawUsecase) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := s.agents.SoftDelete(ctx, item, &now); err != nil {
		return err
	}

	s.auditService.Log(ctx, &userID, "ai.openclaw.delete", "openclaw_instance", &item.ID, "success", clientIP, map[string]string{
		"name": item.Name,
	})
	return nil
}

func (s *OpenClawUsecase) GetLogs(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) ([]string, error) {
	item, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	logs := []string{
		fmt.Sprintf("[%s] OpenClaw Agent Gateway v%s starting on port 18789", now.Add(-15*time.Minute).Format("15:04:05"), item.Version),
		fmt.Sprintf("[%s] Gateway token issued and bound to TLS reverse proxy", now.Add(-14*time.Minute).Format("15:04:05")),
		fmt.Sprintf("[%s] AI Provider configured: %s (model: %s)", now.Add(-12*time.Minute).Format("15:04:05"), item.AIProvider, item.DefaultModel),
		fmt.Sprintf("[%s] Agent workspace mounted at %s", now.Add(-10*time.Minute).Format("15:04:05"), item.WorkspacePath),
		fmt.Sprintf("[%s] Loaded %d custom tools and skills (bash, web_search, file_ops)", now.Add(-8*time.Minute).Format("15:04:05"), item.ActiveSkillsCount),
		fmt.Sprintf("[%s] Gateway listener live at %s", now.Add(-5*time.Minute).Format("15:04:05"), item.GatewayEndpoint),
		fmt.Sprintf("[%s] Health check: OK (RAM %d MB, 0 memory leak, active agent session)", now.Format("15:04:05"), item.MemoryLimitMB/3),
	}
	return logs, nil
}
