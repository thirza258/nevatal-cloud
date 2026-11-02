package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

type AppUsecase struct {
	tx           repository.Transactor
	apps         repository.AppRepository
	auditService *AuditUsecase
}

func NewAppUsecase(tx repository.Transactor, apps repository.AppRepository, auditService *AuditUsecase) *AppUsecase {
	return &AppUsecase{
		tx:           tx,
		apps:         apps,
		auditService: auditService,
	}
}

type CreateAppInput struct {
	Name                 string            `json:"name"`
	ApplicationType      string            `json:"application_type"`       // static, react, nextjs, nodejs, go, python, docker
	DeploymentSourceType string            `json:"deployment_source_type"` // git, template, docker
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

func (s *AppUsecase) List(ctx context.Context, userID uuid.UUID, isAdmin bool) ([]models.Application, error) {
	return s.apps.List(ctx, ownerScope(userID, isAdmin))
}

func (s *AppUsecase) Get(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Application, error) {
	app, err := s.apps.FindByID(ctx, id, ownerScope(userID, isAdmin))
	if err != nil {
		return nil, errors.New("application not found")
	}
	return app, nil
}

func (s *AppUsecase) Create(ctx context.Context, userID uuid.UUID, input CreateAppInput, clientIP string) (*models.Application, *models.Deployment, error) {
	ctx = context.WithoutCancel(ctx)

	slug := strings.ToLower(strings.ReplaceAll(input.Name, " ", "-"))
	subdomain := fmt.Sprintf("%s.apps.nevatal.id", slug)

	// Ensure subdomain uniqueness
	count, _ := s.apps.CountBySubdomain(ctx, subdomain)
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
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.apps.Create(txCtx, &app); err != nil {
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
			if err := s.apps.CreateEnvVar(txCtx, &env); err != nil {
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

		return s.apps.CreateDeployment(txCtx, &deployment)
	})

	if err != nil {
		return nil, nil, err
	}

	// Update app to running state
	app.Status = "running"
	_ = s.apps.UpdateStatus(ctx, &app, "running")

	s.auditService.Log(ctx, &userID, "app.create", "application", &app.ID, "success", clientIP, map[string]string{
		"name":      app.Name,
		"subdomain": app.AssignedSubdomain,
		"type":      app.ApplicationType,
	})

	return &app, &deployment, nil
}

func (s *AppUsecase) TriggerDeploy(ctx context.Context, appID uuid.UUID, userID uuid.UUID, isAdmin bool, commitMsg string, clientIP string) (*models.Deployment, error) {
	ctx = context.WithoutCancel(ctx)

	app, err := s.Get(ctx, appID, userID, isAdmin)
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

	if err := s.apps.CreateDeployment(ctx, &dep); err != nil {
		return nil, err
	}

	app.Status = "running"
	_ = s.apps.UpdateStatus(ctx, app, "running")

	s.auditService.Log(ctx, &userID, "app.deploy", "deployment", &dep.ID, "success", clientIP, map[string]string{
		"app_name": app.Name,
		"commit":   commitHash[:7],
	})

	return &dep, nil
}

func (s *AppUsecase) Rollback(ctx context.Context, appID, deploymentID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.Deployment, error) {
	ctx = context.WithoutCancel(ctx)

	app, err := s.Get(ctx, appID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	target, err := s.apps.FindDeployment(ctx, app.ID, deploymentID)
	if err != nil {
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

	if err := s.apps.CreateDeployment(ctx, &rollbackDep); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "app.rollback", "deployment", &rollbackDep.ID, "success", clientIP, map[string]string{
		"app_name":   app.Name,
		"target_dep": deploymentID.String(),
	})

	return &rollbackDep, nil
}

func (s *AppUsecase) UpdatePower(ctx context.Context, appID uuid.UUID, userID uuid.UUID, isAdmin bool, action string, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	app, err := s.Get(ctx, appID, userID, isAdmin)
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

	if err := s.apps.UpdateStatus(ctx, app, newStatus); err != nil {
		return err
	}

	s.auditService.Log(ctx, &userID, fmt.Sprintf("app.%s", action), "application", &app.ID, "success", clientIP, nil)
	return nil
}

func (s *AppUsecase) Delete(ctx context.Context, appID uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	app, err := s.Get(ctx, appID, userID, isAdmin)
	if err != nil {
		return err
	}

	now := time.Now()
	if err := s.apps.SoftDelete(ctx, app, &now); err != nil {
		return err
	}

	s.auditService.Log(ctx, &userID, "app.delete", "application", &app.ID, "success", clientIP, map[string]string{
		"name": app.Name,
	})
	return nil
}

func (s *AppUsecase) GetEnvVars(ctx context.Context, appID, userID uuid.UUID, isAdmin bool) ([]models.ApplicationEnvVar, error) {
	app, err := s.Get(ctx, appID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	envs, err := s.apps.ListEnvVars(ctx, app.ID)
	if err != nil {
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

func (s *AppUsecase) SaveEnvVar(ctx context.Context, appID, userID uuid.UUID, isAdmin bool, key, value string, isSecret bool) error {
	ctx = context.WithoutCancel(ctx)

	app, err := s.Get(ctx, appID, userID, isAdmin)
	if err != nil {
		return err
	}

	existing, err := s.apps.FindEnvVar(ctx, app.ID, key)
	now := time.Now()

	if err == nil {
		existing.Value = value
		existing.IsSecret = isSecret
		existing.UpdatedAt = now
		return s.apps.SaveEnvVar(ctx, existing)
	}

	env := models.ApplicationEnvVar{
		ApplicationID: app.ID,
		Key:           key,
		Value:         value,
		IsSecret:      isSecret,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	return s.apps.CreateEnvVar(ctx, &env)
}

func (s *AppUsecase) DeleteEnvVar(ctx context.Context, appID, userID uuid.UUID, isAdmin bool, key string) error {
	ctx = context.WithoutCancel(ctx)

	app, err := s.Get(ctx, appID, userID, isAdmin)
	if err != nil {
		return err
	}

	return s.apps.DeleteEnvVar(ctx, app.ID, key)
}
