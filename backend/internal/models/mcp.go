package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// MCPServer represents a registered first-party or external MCP server
type MCPServer struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	Name         string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"name"`
	DisplayName  string    `gorm:"type:varchar(128);not null" json:"display_name"`
	Description  string    `gorm:"type:text" json:"description"`
	Transport    string    `gorm:"type:varchar(32);not null;default:'in_process'" json:"transport"` // in_process, sse, stdio, http
	EndpointURL  string    `gorm:"type:varchar(512)" json:"endpoint_url,omitempty"`
	IsFirstParty bool      `gorm:"default:false" json:"is_first_party"`
	Enabled      bool      `gorm:"default:true" json:"enabled"`
	AuthType     string    `gorm:"type:varchar(32);default:'none'" json:"auth_type"` // none, bearer, api_key
	Icon         string    `gorm:"type:varchar(64)" json:"icon,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	Tools []MCPTool `gorm:"foreignKey:ServerID" json:"tools,omitempty"`
}

func (s *MCPServer) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (MCPServer) TableName() string {
	return "mcp_servers"
}

// MCPConnection represents a user/tenant's authorized connection to an MCP server
type MCPConnection struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID          uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	ServerID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"server_id"`
	Status          string     `gorm:"type:varchar(32);not null;default:'connected'" json:"status"` // connected, disconnected, error
	ConfigJSON      string     `gorm:"type:text" json:"config_json,omitempty"`                      // encrypted credentials / settings
	LastHealthCheck *time.Time `json:"last_health_check,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`

	Server      *MCPServer      `gorm:"foreignKey:ServerID" json:"server,omitempty"`
	Permissions []MCPPermission `gorm:"foreignKey:ConnectionID" json:"permissions,omitempty"`
}

func (c *MCPConnection) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	return nil
}

func (MCPConnection) TableName() string {
	return "mcp_connections"
}

// MCPTool defines an individual tool provided by an MCP server
type MCPTool struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ServerID    uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	Name        string    `gorm:"type:varchar(128);not null;index" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	RiskLevel   string    `gorm:"type:varchar(32);not null;default:'read'" json:"risk_level"` // read, write, destructive
	InputSchema string    `gorm:"type:text" json:"input_schema"`                              // JSON Schema
	Enabled     bool      `gorm:"default:true" json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`

	Server *MCPServer `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (t *MCPTool) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (MCPTool) TableName() string {
	return "mcp_tools"
}

// MCPPermission records granted tool permissions for a customer connection
type MCPPermission struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ConnectionID uuid.UUID `gorm:"type:uuid;not null;index" json:"connection_id"`
	ToolID       uuid.UUID `gorm:"type:uuid;not null;index" json:"tool_id"`
	Granted      bool      `gorm:"default:true" json:"granted"`
	CreatedAt    time.Time `json:"created_at"`

	Tool *MCPTool `gorm:"foreignKey:ToolID" json:"tool,omitempty"`
}

func (p *MCPPermission) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (MCPPermission) TableName() string {
	return "mcp_permissions"
}

// MCPAuditLog records all MCP tool calls with risk level, tenant authorization, and outcome
type MCPAuditLog struct {
	ID           uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID       uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	ServerID     uuid.UUID `gorm:"type:uuid;not null;index" json:"server_id"`
	ToolName     string    `gorm:"type:varchar(128);not null;index" json:"tool_name"`
	RiskLevel    string    `gorm:"type:varchar(32);not null" json:"risk_level"`
	Confirmed    bool      `gorm:"default:false" json:"confirmed"`
	InputSummary string    `gorm:"type:text" json:"input_summary"`
	ResultStatus string    `gorm:"type:varchar(32);not null" json:"result_status"` // success, error, denied
	IPAddress    string    `gorm:"type:varchar(45)" json:"ip_address,omitempty"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`

	User   *User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Server *MCPServer `gorm:"foreignKey:ServerID" json:"server,omitempty"`
}

func (l *MCPAuditLog) BeforeCreate(tx *gorm.DB) error {
	if l.ID == uuid.Nil {
		l.ID = uuid.New()
	}
	return nil
}

func (MCPAuditLog) TableName() string {
	return "mcp_audit_logs"
}
