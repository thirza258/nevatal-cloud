package response

import "github.com/nevatal/cloud/internal/usecase"

// Status is a {"status": ...} acknowledgement.
type Status struct {
	Status string `json:"status"`
}

// AIModels is the body of GET /ai-assistant/models.
type AIModels struct {
	Models []usecase.OpenRouterModelDefinition `json:"models"`
}

// OpenRouterKeyValid is the 200 body of POST /ai-assistant/verify-key (not
// enveloped: "valid" sits beside "data").
type OpenRouterKeyValid struct {
	Data  map[string]interface{} `json:"data"`
	Valid bool                   `json:"valid"`
}

// OpenRouterKeyInvalid is the 401 body of POST /ai-assistant/verify-key.
type OpenRouterKeyInvalid struct {
	Error OpenRouterKeyError `json:"error"`
	Valid bool               `json:"valid"`
}

// OpenRouterKeyError carries OpenRouter's raw rejection.
type OpenRouterKeyError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Raw     string `json:"raw"`
}
