package request

import "github.com/nevatal/cloud/internal/usecase"

// CreateConversationInput is the optional body of POST /ai/conversations.
type CreateConversationInput struct {
	Title string `json:"title"`
}

// SendMessageInput is the body of POST /ai/conversations/:id/messages.
type SendMessageInput struct {
	Message   string                 `json:"message" binding:"required"`
	AnswerKey string                 `json:"answer_key,omitempty"` // If user clicked an interactive answer chip
	AnswerVal string                 `json:"answer_val,omitempty"` // Value for the answered key
	Answers   map[string]interface{} `json:"answers,omitempty"`    // Map of key-value answers
	APIKey    string                 `json:"api_key,omitempty"`    // Optional user OpenRouter API Key
	Model     string                 `json:"model,omitempty"`      // Model preference
}

func (r SendMessageInput) ToUsecase() usecase.SendMessageInput {
	return usecase.SendMessageInput(r)
}
