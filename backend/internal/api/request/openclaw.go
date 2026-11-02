package request

import "github.com/nevatal/cloud/internal/usecase"

// CreateOpenClawInput is the body of POST /ai-agents/openclaw/instances.
type CreateOpenClawInput struct {
	Name           string `json:"name" binding:"required"`
	Version        string `json:"version"`
	AIProvider     string `json:"ai_provider"` // openai, anthropic, gemini, ollama, deepseek
	DefaultModel   string `json:"default_model"`
	APIKey         string `json:"api_key"`
	CPULimit       int    `json:"cpu_limit"`
	MemoryLimitMB  int    `json:"memory_limit_mb"`
	StorageLimitGB int    `json:"storage_limit_gb"`
}

func (r CreateOpenClawInput) ToUsecase() usecase.CreateOpenClawInput {
	return usecase.CreateOpenClawInput(r)
}

// UpdateOpenClawConfigRequest is the body of
// PUT /ai-agents/openclaw/instances/:id/config.
type UpdateOpenClawConfigRequest = struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
}

// AddIntegrationRequest is the body of
// POST /ai-agents/openclaw/instances/:id/integrations.
type AddIntegrationRequest = struct {
	Channel    string `json:"channel" binding:"required"`
	Identifier string `json:"identifier" binding:"required"`
}
