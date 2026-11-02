package response

import "github.com/google/uuid"

// PlanValidation is the body of POST /ai/plans/:id/validate.
type PlanValidation struct {
	PlanID uuid.UUID `json:"plan_id"`
	Status string    `json:"status"`
	Valid  bool      `json:"valid"`
}

// AgentRun is the body of GET /ai/runs/:id (runs are synchronous today).
type AgentRun struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// AgentRunEvent is one item of GET /ai/runs/:id/events.
type AgentRunEvent struct {
	Event     string `json:"event"`
	Timestamp string `json:"timestamp"`
}

// StringError is the legacy {"error": "..."} body of the MCP JSON-RPC
// endpoint.
type StringError struct {
	Error string `json:"error"`
}
