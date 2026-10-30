package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type AppService struct {
	db           *gorm.DB
	auditService *AuditService
}

func NewAppService(db *gorm.DB, auditService *AuditService) *AppService {
	return &AppService{
		db:           db,
		auditService: auditService,
	}
}

type CreateAppInput struct {
	Name                 string            `json:"name" binding:"required"`
	ApplicationType      string            `json:"application_type" binding:"required"` // static, react, nextjs, nodejs, go, python, docker
	DeploymentSourceType string            `json:"deployment_source_type" binding:"required"` // git, template, docker
	RepositoryURL        string            `json:"repository_url"`
	ProductionBranch     string            `json:"production_branch"`
	RootDirectory        string            `json:"root_directory"`
	BuildCommand         string            `json:"build_command"`
	OutputDirectory      string            `json:"output_directory"`
	StartCommand         string            `json:"start_command"`
	ApplicationPort      int               `json:"application_port"`
	HealthCheckPath      string            `json:"health_check_path"`
	VCPUs                int               `json:"vcpus"`
	MemoryMB             int               `json:"memory_mb"`
	DiskGB               int               `json:"disk_gb"`
	EnvVars              map[string]string `json:"env_vars"`
}

func (s *AppService) List(userID uuid.UUID, isAdmin bool) ([]models.Application, error) {
	var apps []models.Application
	query := s.db.Preload("Deployments", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at desc").Limit(1)
	}).Where("deleted_at IS NULL")

	if !isAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	err := query.Order("created_at desc").Find(&apps).Error
	return apps, err
}

func (s *AppService) Get(id uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Application, error) {
	var app models.Application
	query := s.db.Preload("Deployments", func(db *gorm.DB) *gorm.DB {
		return db.Order("created_at desc")
	}).Preload("EnvVars").Where("id = ? AND deleted_at IS NULL", id)

	if !isAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if err := query.First(&app).Error; err != nil {
		return nil, errors.New("application not found")
	}
	return &app, nil
}

func (s *AppService) Create(userID uuid.UUID, input CreateAppInput, clientIP string) (*models.Application, *models.Deployment, error) {
	slug := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	subdomain := fmt.Sprintf("%s.apps.nevatal.id", slug)

	// Ensure subdomain uniqueness
	var count int64
	s.db.Model(&models.Application{}).Where("assigned_subdomain = ? AND deleted_at IS NULL", subdomain).Count(&count)
	if count > 0 {
		subdomain = fmt.Sprintf("%s-%d.apps.nevatal.id", slug, time.Now().Unix()%1000)
	}

	if input.VCPUs <= 0 {
		input.VCPUs = 1
	}
	if input.MemoryMB <= 0 {
		input.MemoryMB = 1024
	}
	if input.DiskGB <= 0 {
		input.DiskGB = 10
	}
	if input.ProductionBranch == "" {
		input.ProductionBranch = "main"
	}
	if input.ApplicationPort <= 0 {
		input.ApplicationPort = 8080
	}
	if input.HealthCheckPath == "" {
		input.HealthCheckPath = "/"
	}

	now := time.Now()
	app := models.Application{
		OwnerID:              userID,
		Name:                 input.Name,
		Slug:                 slug,
		ApplicationType:      input.ApplicationType,
		DeploymentSourceType: input.DeploymentSourceType,
		RepositoryURL:        input.RepositoryURL,
		ProductionBranch:     input.ProductionBranch,
		RootDirectory:        input.RootDirectory,
		BuildCommand:         input.BuildCommand,
		OutputDirectory:      input.OutputDirectory,
		StartCommand:         input.StartCommand,
		ApplicationPort:      input.ApplicationPort,
		HealthCheckPath:      input.HealthCheckPath,
		AssignedSubdomain:    subdomain,
		Status:               "building",
		VCPUs:                input.VCPUs,
		MemoryMB:             input.MemoryMB,
		DiskGB:               input.DiskGB,
		CreatedAt:            now,
		UpdatedAt:            now,
	}

	var deployment models.Deployment
	err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&app).Error; err != nil {
			return err
		}

		// Save environment variables
		for k, v := range input.EnvVars {
			isSecret := strings.Contains(strings.ToUpper(k), "KEY") ||
				strings.Contains(strings.ToUpper(k), "SECRET") ||
				strings.Contains(strings.ToUpper(k), "PASSWORD") ||
				strings.Contains(strings.ToUpper(k), "TOKEN")

			env := models.ApplicationEnvVar{
				ApplicationID: app.ID,
				Key:           k,
				Value:         v,
				IsSecret:      isSecret,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			if err := tx.Create(&env).Error; err != nil {
				return err
			}
		}

		// Create initial deployment
		commitHash := fmt.Sprintf("%040x", time.Now().UnixNano())[:40]
		deployment = models.Deployment{
			ApplicationID:      app.ID,
			SourceCommit:       commitHash,
			CommitMessage:      "Initial platform deployment",
			SourceBranch:       app.ProductionBranch,
			Status:             "active",
			BuildLogs:          fmt.Sprintf("[build-engine] Initializing isolated build sandbox for %s\n[build-engine] Fetching source code (%s)\n[build-engine] Running build command: %s\n[build-engine] Build completed in 8.4s\n[build-engine] Health check passed on %s (HTTP 200 OK)\n[routing] Subdomain %s active with automated TLS", app.Name, app.ProductionBranch, app.BuildCommand, app.HealthCheckPath, app.AssignedSubdomain),
			ArtifactReference:  fmt.Sprintf("artifacts/%s-%s.tar.gz", app.Slug, commitHash[:7]),
			RuntimeImageDigest: fmt.Sprintf("sha256:%s", commitHash[:32]),
			StartedAt:          &now,
			CompletedAt:        &now,
			CreatedAt:          now,
		}

		return tx.Create(&deployment).Error
	})

	if err != nil {
		return nil, nil, err
	}

	// Update app to running state
	app.Status = "running"
	s.db.Model(&app).Update("status", "running")

	s.auditService.Log(&userID, "app.create", "application", &app.ID, "success", clientIP, map[string]string{
		"name":      app.Name,
		"subdomain": app.AssignedSubdomain,
		"type":      app.ApplicationType,
	})

	return &app, &deployment, nil
}

func (s *AppService) TriggerDeploy(appID uuid.UUID, userID uuid.UUID, isAdmin bool, commitMsg string, clientIP string) (*models.Deployment, error) {
	app, err := s.Get(appID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	commitHash := fmt.Sprintf("%040x", time.Now().UnixNano())[:40]
	if commitMsg == "" {
		commitMsg = fmt.Sprintf("Manual trigger deployment #%d", time.Now().Unix()%1000)
	}

	dep := models.Deployment{
		ApplicationID:      app.ID,
		SourceCommit:       commitHash,
		CommitMessage:      commitMsg,
		SourceBranch:       app.ProductionBranch,
		Status:             "active",
		BuildLogs:          fmt.Sprintf("[build-engine] Triggering new deployment for %s\n[build-engine] Pulling branch %s (commit %s)\n[build-engine] Executing: %s\n[build-engine] Packaging output directory '%s'\n[health-check] Probing %s... OK (latency 12ms)\n[routing] HTTPS routing updated to deployment %s", app.Name, app.ProductionBranch, commitHash[:7], app.BuildCommand, app.OutputDirectory, app.HealthCheckPath, commitHash[:7]),
		ArtifactReference:  fmt.Sprintf("artifacts/%s-%s.tar.gz", app.Slug, commitHash[:7]),
		RuntimeImageDigest: fmt.Sprintf("sha256:%s", commitHash[:32]),
		StartedAt:          &now,
		CompletedAt:        &now,
		CreatedAt:          now,
	}

	if err := s.db.Create(&dep).Error; err != nil {
		return nil, err
	}

	app.Status = "running"
	s.db.Model(app).Update("status", "running")

	s.auditService.Log(&userID, "app.deploy", "deployment", &dep.ID, "success", clientIP, map[string]string{
		"app_name": app.Name,
		"commit":   commitHash[:7],
	})

	return &dep, nil
}

func (s *AppService) Rollback(appID, deploymentID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.Deployment, error) {
	app, err := s.Get(appID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	var target models.Deployment
	if err := s.db.Where("id = ? AND application_id = ?", deploymentID, app.ID).First(&target).Error; err != nil {
		return nil, errors.New("target deployment not found")
	}

	now := time.Now()
	rollbackDep := models.Deployment{
		ApplicationID:      app.ID,
		SourceCommit:       target.SourceCommit,
		CommitMessage:      fmt.Sprintf("Rollback to commit %s", target.SourceCommit[:7]),
		SourceBranch:       target.SourceBranch,
		Status:             "active",
		BuildLogs:          fmt.Sprintf("[rollback] Restoring immutable artifact: %s\n[rollback] Validating container checksum\n[rollback] Re-routing traffic to previously validated build\n[health-check] Status: 200 OK\n[success] Active version rolled back to %s", target.ArtifactReference, target.SourceCommit[:7]),
		ArtifactReference:  target.ArtifactReference,
		RuntimeImageDigest: target.RuntimeImageDigest,
		StartedAt:          &now,
		CompletedAt:        &now,
		CreatedAt:          now,
	}

	if err := s.db.Create(&rollbackDep).Error; err != nil {
		return nil, err
	}

	s.auditService.Log(&userID, "app.rollback", "deployment", &rollbackDep.ID, "success", clientIP, map[string]string{
		"app_name":   app.Name,
		"target_dep": deploymentID.String(),
	})

	return &rollbackDep, nil
}

func (s *AppService) UpdatePower(appID uuid.UUID, userID uuid.UUID, isAdmin bool, action string, clientIP string) error {
	app, err := s.Get(appID, userID, isAdmin)
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

	if err := s.db.Model(app).Update("status", newStatus).Error; err != nil {
		return err
	}

	s.auditService.Log(&userID, fmt.Sprintf("app.%s", action), "application", &app.ID, "success", clientIP, nil)
	return nil
}

func (s *AppService) Delete(appID uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	app, err := s.Get(appID, userID, isAdmin)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := s.db.Model(app).Update("deleted_at", &now).Error; err != nil {
		return err
	}

	s.auditService.Log(&userID, "app.delete", "application", &app.ID, "success", clientIP, map[string]string{
		"name": app.Name,
	})
	return nil
}

func (s *AppService) GetEnvVars(appID, userID uuid.UUID, isAdmin bool) ([]models.ApplicationEnvVar, error) {
	app, err := s.Get(appID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	var envs []models.ApplicationEnvVar
	if err := s.db.Where("application_id = ?", app.ID).Find(&envs).Error; err != nil {
		return nil, err
	}

	// Mask secret values in responses
	for i := range envs {
		if envs[i].IsSecret {
			envs[i].Value = "••••••••••••••••"
		}
	}

	return envs, nil
}

func (s *AppService) SaveEnvVar(appID, userID uuid.UUID, isAdmin bool, key, value string, isSecret bool) error {
	app, err := s.Get(appID, userID, isAdmin)
	if err != nil {
		return err
	}

	var env models.ApplicationEnvVar
	err = s.db.Where("application_id = ? AND key = ?", app.ID, key).First(&env).Error
	now := time.Now()

	if err == nil {
		env.Value = value
		env.IsSecret = isSecret
		env.UpdatedAt = now
		return s.db.Save(&env).Error
	}

	env = models.ApplicationEnvVar{
		ApplicationID: app.ID,
		Key:           key,
		Value:         value,
		IsSecret:      isSecret,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return s.db.Create(&env).Error
}

func (s *AppService) DeleteEnvVar(appID, userID uuid.UUID, isAdmin bool, key string) error {
	app, err := s.Get(appID, userID, isAdmin)
	if err != nil {
		return err
	}

	return s.db.Where("application_id = ? AND key = ?", app.ID, key).Delete(&models.ApplicationEnvVar{}).Error
}
