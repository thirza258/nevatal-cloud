package request

import "github.com/google/uuid"

// ConnectServerInput is the body of POST /integrations/mcp/connections.
type ConnectServerInput struct {
	ServerID   uuid.UUID `json:"server_id" binding:"required"`
	ConfigJSON string    `json:"config_json"`
}

// UpdatePermissionInput is the body of
// PATCH /integrations/mcp/connections/:id/permissions.
type UpdatePermissionInput struct {
	ToolID  uuid.UUID `json:"tool_id" binding:"required"`
	Granted bool      `json:"granted"`
}
