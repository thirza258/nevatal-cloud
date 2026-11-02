package request

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/usecase"
)

// ChatRequestInput is the body of POST /ai-assistant/chat.
type ChatRequestInput struct {
	Message    string `json:"message" binding:"required"`
	Model      string `json:"model"`
	APIKey     string `json:"api_key"`
	ActionOnly bool   `json:"action_only"`
}

func (r ChatRequestInput) ToUsecase() usecase.ChatRequestInput {
	return usecase.ChatRequestInput(r)
}

// ExecuteActionInput is the body of POST /ai-assistant/execute.
type ExecuteActionInput struct {
	MessageID uuid.UUID       `json:"message_id"`
	Action    string          `json:"action" binding:"required"`
	Params    json.RawMessage `json:"params" binding:"required"`
}

func (r ExecuteActionInput) ToUsecase() usecase.ExecuteActionInput {
	return usecase.ExecuteActionInput(r)
}

// VerifyKeyInput is the body of POST /ai-assistant/verify-key.
type VerifyKeyInput struct {
	APIKey string `json:"api_key" binding:"required"`
}
