package request

import "github.com/nevatal/cloud/internal/usecase"

// CreateAppInput is the body of POST /apps.
type CreateAppInput struct {
	Name                 string            `json:"name" binding:"required"`
	ApplicationType      string            `json:"application_type" binding:"required"`       // static, react, nextjs, nodejs, go, python, docker
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

func (r CreateAppInput) ToUsecase() usecase.CreateAppInput {
	return usecase.CreateAppInput(r)
}

// DeployAppRequest is the optional body of POST /apps/:id/deploy.
type DeployAppRequest = struct {
	CommitMessage string `json:"commit_message"`
}

// SaveEnvVarRequest is the body of POST /apps/:id/env.
type SaveEnvVarRequest = struct {
	Key      string `json:"key" binding:"required"`
	Value    string `json:"value" binding:"required"`
	IsSecret bool   `json:"is_secret"`
}
