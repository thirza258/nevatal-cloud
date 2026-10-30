package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type AIAssistantMessage struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Role           string    `gorm:"type:varchar(32);not null" json:"role"` // "user", "assistant", "system"
	Content        string    `gorm:"type:text;not null" json:"content"`
	Model          string    `gorm:"type:varchar(100);not null;default:'openai/gpt-4o-mini'" json:"model"`
	ActionType     string    `gorm:"type:varchar(50)" json:"action_type,omitempty"`       // "create_instance", "check_quota", "power_instance"
	ActionPayload  string    `gorm:"type:text" json:"action_payload,omitempty"`           // JSON string of proposed container parameters
	ActionExecuted bool      `gorm:"default:false" json:"action_executed"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`

	User *User `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

func (m *AIAssistantMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

func (AIAssistantMessage) TableName() string {
	return "ai_assistant_messages"
}

type AIAssistantActionProposal struct {
	Action      string      `json:"action"` // "create_instance", "power_instance", etc.
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Params      interface{} `json:"params"`
	Executed    bool        `json:"executed"`
}

type CreateInstanceActionParams struct {
	Name         string `json:"name"`
	Hostname     string `json:"hostname"`
	TemplateID   string `json:"template_id"`
	TemplateName string `json:"template_name"`
	VCPUs        int    `json:"vcpus"`
	MemoryMB     int    `json:"memory_mb"`
	DiskGB       int    `json:"disk_gb"`
	SSHKeyID     string `json:"ssh_key_id,omitempty"`
	SSHKeyName   string `json:"ssh_key_name,omitempty"`
	Reason       string `json:"reason,omitempty"`
}
