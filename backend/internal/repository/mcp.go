package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// MCPRepository persists the MCP integration registry: servers and their
// tools, users' connections with per-tool permissions, the MCP audit trail
// and tool invocation records.
type MCPRepository interface {
	// ListServers returns enabled servers (first-party first, then by name)
	// with their tools.
	ListServers(ctx context.Context) ([]models.MCPServer, error)
	// FindServer returns a server with its tools.
	FindServer(ctx context.Context, id uuid.UUID) (*models.MCPServer, error)
	FindEnabledServer(ctx context.Context, id uuid.UUID) (*models.MCPServer, error)
	// FindServerByName returns a server by its unique name.
	FindServerByName(ctx context.Context, name string) (*models.MCPServer, error)
	ListServerTools(ctx context.Context, serverID uuid.UUID) ([]models.MCPTool, error)
	// ListEnabledTools returns every enabled tool by name.
	ListEnabledTools(ctx context.Context) ([]models.MCPTool, error)
	// FindEnabledToolByName returns an enabled tool with its server.
	FindEnabledToolByName(ctx context.Context, name string) (*models.MCPTool, error)

	// ListConnections returns the user's connections (newest first) with
	// server and permission tools preloaded.
	ListConnections(ctx context.Context, userID uuid.UUID) ([]models.MCPConnection, error)
	// FindConnectionToServer returns the user's connection to a server.
	FindConnectionToServer(ctx context.Context, userID, serverID uuid.UUID) (*models.MCPConnection, error)
	// FindUserConnection returns one of the user's connections.
	FindUserConnection(ctx context.Context, connectionID, userID uuid.UUID) (*models.MCPConnection, error)
	CreateConnection(ctx context.Context, conn *models.MCPConnection) error
	SaveConnection(ctx context.Context, conn *models.MCPConnection) error
	// ReloadConnection refreshes conn with its server and permission tools.
	ReloadConnection(ctx context.Context, conn *models.MCPConnection) error
	// DeleteConnection deletes one of the user's connections.
	DeleteConnection(ctx context.Context, connectionID, userID uuid.UUID) error

	CreatePermission(ctx context.Context, perm *models.MCPPermission) error
	// ListPermissions returns a connection's permissions with their tools.
	ListPermissions(ctx context.Context, connectionID uuid.UUID) ([]models.MCPPermission, error)
	SetPermission(ctx context.Context, connectionID, toolID uuid.UUID, granted bool) error

	// ListAuditLogs returns the user's newest MCP audit entries with servers.
	ListAuditLogs(ctx context.Context, userID uuid.UUID, limit int) ([]models.MCPAuditLog, error)
	CreateAuditLog(ctx context.Context, entry *models.MCPAuditLog) error
	CreateToolInvocation(ctx context.Context, inv *models.AIToolInvocation) error
}

type mcpRepository struct{ base }

var _ MCPRepository = (*mcpRepository)(nil)

func NewMCPRepository(db *gorm.DB) MCPRepository {
	return &mcpRepository{base{db}}
}

func (r *mcpRepository) ListServers(ctx context.Context) ([]models.MCPServer, error) {
	var servers []models.MCPServer
	err := r.conn(ctx).Preload("Tools").Where("enabled = ?", true).Order("is_first_party desc, display_name asc").Find(&servers).Error
	return servers, err
}

func (r *mcpRepository) FindServer(ctx context.Context, id uuid.UUID) (*models.MCPServer, error) {
	var server models.MCPServer
	if err := r.conn(ctx).Preload("Tools").First(&server, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &server, nil
}

func (r *mcpRepository) FindEnabledServer(ctx context.Context, id uuid.UUID) (*models.MCPServer, error) {
	var server models.MCPServer
	if err := r.conn(ctx).First(&server, "id = ? AND enabled = ?", id, true).Error; err != nil {
		return nil, mapErr(err)
	}
	return &server, nil
}

func (r *mcpRepository) FindServerByName(ctx context.Context, name string) (*models.MCPServer, error) {
	var server models.MCPServer
	if err := r.conn(ctx).First(&server, "name = ?", name).Error; err != nil {
		return nil, mapErr(err)
	}
	return &server, nil
}

func (r *mcpRepository) ListServerTools(ctx context.Context, serverID uuid.UUID) ([]models.MCPTool, error) {
	var tools []models.MCPTool
	err := r.conn(ctx).Where("server_id = ?", serverID).Find(&tools).Error
	return tools, err
}

func (r *mcpRepository) ListEnabledTools(ctx context.Context) ([]models.MCPTool, error) {
	var tools []models.MCPTool
	err := r.conn(ctx).Where("enabled = ?", true).Order("name asc").Find(&tools).Error
	return tools, err
}

func (r *mcpRepository) FindEnabledToolByName(ctx context.Context, name string) (*models.MCPTool, error) {
	var tool models.MCPTool
	if err := r.conn(ctx).Preload("Server").First(&tool, "name = ? AND enabled = ?", name, true).Error; err != nil {
		return nil, mapErr(err)
	}
	return &tool, nil
}

func (r *mcpRepository) ListConnections(ctx context.Context, userID uuid.UUID) ([]models.MCPConnection, error) {
	var conns []models.MCPConnection
	err := r.conn(ctx).Preload("Server").Preload("Permissions.Tool").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Find(&conns).Error
	return conns, err
}

func (r *mcpRepository) FindConnectionToServer(ctx context.Context, userID, serverID uuid.UUID) (*models.MCPConnection, error) {
	var conn models.MCPConnection
	if err := r.conn(ctx).First(&conn, "user_id = ? AND server_id = ?", userID, serverID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &conn, nil
}

func (r *mcpRepository) FindUserConnection(ctx context.Context, connectionID, userID uuid.UUID) (*models.MCPConnection, error) {
	var conn models.MCPConnection
	if err := r.conn(ctx).First(&conn, "id = ? AND user_id = ?", connectionID, userID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &conn, nil
}

func (r *mcpRepository) CreateConnection(ctx context.Context, conn *models.MCPConnection) error {
	return r.conn(ctx).Create(conn).Error
}

func (r *mcpRepository) SaveConnection(ctx context.Context, conn *models.MCPConnection) error {
	return r.conn(ctx).Save(conn).Error
}

func (r *mcpRepository) ReloadConnection(ctx context.Context, conn *models.MCPConnection) error {
	return r.conn(ctx).Preload("Server").Preload("Permissions.Tool").First(conn, "id = ?", conn.ID).Error
}

func (r *mcpRepository) DeleteConnection(ctx context.Context, connectionID, userID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND user_id = ?", connectionID, userID).Delete(&models.MCPConnection{}).Error
}

func (r *mcpRepository) CreatePermission(ctx context.Context, perm *models.MCPPermission) error {
	return r.conn(ctx).Create(perm).Error
}

func (r *mcpRepository) ListPermissions(ctx context.Context, connectionID uuid.UUID) ([]models.MCPPermission, error) {
	var perms []models.MCPPermission
	err := r.conn(ctx).Preload("Tool").Where("connection_id = ?", connectionID).Find(&perms).Error
	return perms, err
}

func (r *mcpRepository) SetPermission(ctx context.Context, connectionID, toolID uuid.UUID, granted bool) error {
	return r.conn(ctx).Model(&models.MCPPermission{}).
		Where("connection_id = ? AND tool_id = ?", connectionID, toolID).
		Update("granted", granted).Error
}

func (r *mcpRepository) ListAuditLogs(ctx context.Context, userID uuid.UUID, limit int) ([]models.MCPAuditLog, error) {
	var logs []models.MCPAuditLog
	err := r.conn(ctx).Preload("Server").
		Where("user_id = ?", userID).
		Order("created_at desc").
		Limit(limit).
		Find(&logs).Error
	return logs, err
}

func (r *mcpRepository) CreateAuditLog(ctx context.Context, entry *models.MCPAuditLog) error {
	return r.conn(ctx).Create(entry).Error
}

func (r *mcpRepository) CreateToolInvocation(ctx context.Context, inv *models.AIToolInvocation) error {
	return r.conn(ctx).Create(inv).Error
}
