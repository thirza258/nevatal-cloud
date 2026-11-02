package request

import "github.com/nevatal/cloud/internal/usecase"

// CreateAutomationInput is the body of POST /automation/instances.
type CreateAutomationInput struct {
	Name           string `json:"name" binding:"required"`
	N8nVersion     string `json:"n8n_version"`
	CPULimit       int    `json:"cpu_limit"`
	MemoryLimitMB  int    `json:"memory_limit_mb"`
	StorageLimitGB int    `json:"storage_limit_gb"`
}

func (r CreateAutomationInput) ToUsecase() usecase.CreateAutomationInput {
	return usecase.CreateAutomationInput(r)
}
