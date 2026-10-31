package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WorkflowState represents the conversation workflow state (PRD 34.2)
type WorkflowState string

const (
	StateUnderstandingRequest   WorkflowState = "UnderstandingRequest"
	StateClarifyingRequirements WorkflowState = "ClarifyingRequirements"
	StatePlanning               WorkflowState = "Planning"
	StateAwaitingConfirmation   WorkflowState = "AwaitingConfirmation"
	StateAIProvisioning         WorkflowState = "Provisioning"
	StateValidating             WorkflowState = "Validating"
	StateRecovery               WorkflowState = "Recovery"
	StateCompleted              WorkflowState = "Completed"
	StateAIFailed               WorkflowState = "Failed"
	StateCancelled              WorkflowState = "Cancelled"
)

// AIConversation maintains conversation metadata and current status
type AIConversation struct {
	ID            uuid.UUID      `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Title         string         `gorm:"type:varchar(255);not null" json:"title"`
	State         WorkflowState  `gorm:"type:varchar(64);not null;default:'UnderstandingRequest'" json:"state"`
	CurrentIntent string         `gorm:"type:varchar(64)" json:"current_intent,omitempty"`
	Summary       string         `gorm:"type:text" json:"summary,omitempty"`
	CreatedAt     time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`

	User     *User             `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Messages []AIMessage       `gorm:"foreignKey:ConversationID" json:"messages,omitempty"`
	Plans    []AIResourcePlan  `gorm:"foreignKey:ConversationID" json:"plans,omitempty"`
	StateObj *AIConversationState `gorm:"foreignKey:ConversationID" json:"state_obj,omitempty"`
}

func (c *AIConversation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == uuid.Nil {
		c.ID = uuid.New()
	}
	if c.State == "" {
		c.State = StateUnderstandingRequest
	}
	return nil
}

func (AIConversation) TableName() string {
	return "ai_conversations"
}

// AIMessage represents user or assistant messages with structured UI metadata
type AIMessage struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ConversationID uuid.UUID `gorm:"type:uuid;not null;index" json:"conversation_id"`
	UserID         uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Role           string    `gorm:"type:varchar(32);not null" json:"role"` // "user", "assistant", "system"
	Content        string    `gorm:"type:text;not null" json:"content"`
	UIComponents   string    `gorm:"type:text" json:"ui_components,omitempty"` // JSON string for interactive UI cards
	RawInput       string    `gorm:"type:text" json:"raw_input,omitempty"`
	CreatedAt      time.Time `gorm:"index" json:"created_at"`
}

func (m *AIMessage) BeforeCreate(tx *gorm.DB) error {
	if m.ID == uuid.Nil {
		m.ID = uuid.New()
	}
	return nil
}

func (AIMessage) TableName() string {
	return "ai_messages"
}

// AIConversationState tracks runtime gathering progress and questions
type AIConversationState struct {
	ID                   uuid.UUID     `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ConversationID       uuid.UUID     `gorm:"type:uuid;not null;uniqueIndex" json:"conversation_id"`
	State                WorkflowState `gorm:"type:varchar(64);not null;default:'UnderstandingRequest'" json:"state"`
	CurrentIntent        string        `gorm:"type:varchar(64)" json:"current_intent"`
	PendingQuestions     string        `gorm:"type:text" json:"pending_questions"`     // JSON array of StructuredQuestion
	GatheredRequirements string        `gorm:"type:text" json:"gathered_requirements"` // JSON object of requirements
	ActivePlanID         *uuid.UUID    `gorm:"type:uuid" json:"active_plan_id,omitempty"`
	UpdatedAt            time.Time     `json:"updated_at"`
}

func (s *AIConversationState) BeforeCreate(tx *gorm.DB) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	return nil
}

func (AIConversationState) TableName() string {
	return "ai_conversation_state"
}

// AIResourcePlan represents a validated multi-resource plan
type AIResourcePlan struct {
	ID                   uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ConversationID       uuid.UUID `gorm:"type:uuid;not null;index" json:"conversation_id"`
	UserID               uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Version              int       `gorm:"not null;default:1" json:"version"`
	Intent               string    `gorm:"type:varchar(64);not null" json:"intent"`
	Status               string    `gorm:"type:varchar(32);not null;default:'draft'" json:"status"` // draft, awaiting_confirmation, approved, provisioning, validating, completed, failed, cancelled
	Title                string    `gorm:"type:varchar(255);not null" json:"title"`
	Description          string    `gorm:"type:text" json:"description"`
	PlanJSON             string    `gorm:"type:text;not null" json:"plan_json"`
	EstimatedCosts       string    `gorm:"type:text" json:"estimated_costs,omitempty"`
	ValidationErrors     string    `gorm:"type:text" json:"validation_errors,omitempty"`
	RequiresConfirmation bool      `gorm:"default:true" json:"requires_confirmation"`
	CreatedAt            time.Time `json:"created_at"`
	UpdatedAt            time.Time `json:"updated_at"`

	Resources []AIPlanResource `gorm:"foreignKey:PlanID" json:"resources,omitempty"`
	Approvals []AIApproval     `gorm:"foreignKey:PlanID" json:"approvals,omitempty"`
}

func (p *AIResourcePlan) BeforeCreate(tx *gorm.DB) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	return nil
}

func (AIResourcePlan) TableName() string {
	return "ai_resource_plans"
}

// AIPlanResource details each individual resource in a multi-resource plan
type AIPlanResource struct {
	ID             uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID         uuid.UUID `gorm:"type:uuid;not null;index" json:"plan_id"`
	ResourceType   string    `gorm:"type:varchar(64);not null" json:"resource_type"` // virtual_machine, application, n8n, openclaw, storage_bucket
	ResourceName   string    `gorm:"type:varchar(128);not null" json:"resource_name"`
	ConfigJSON     string    `gorm:"type:text;not null" json:"config_json"`
	Status         string    `gorm:"type:varchar(32);not null;default:'pending'" json:"status"` // pending, provisioning, validating, completed, failed, cancelled
	ProvisionedID  string    `gorm:"type:varchar(128)" json:"provisioned_id,omitempty"`
	ExecutionOrder int       `gorm:"not null;default:1" json:"execution_order"`
	ErrorMessage   string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
}

func (r *AIPlanResource) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (AIPlanResource) TableName() string {
	return "ai_plan_resources"
}

// AIApproval binds an approval strictly to a plan version
type AIApproval struct {
	ID          uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"plan_id"`
	UserID      uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Version     int        `gorm:"not null" json:"version"`
	Status      string     `gorm:"type:varchar(32);not null;default:'pending'" json:"status"` // pending, approved, rejected, cancelled, expired
	ExpiresAt   time.Time  `gorm:"not null" json:"expires_at"`
	ApprovedAt  *time.Time `json:"approved_at,omitempty"`
	CancelledAt *time.Time `json:"cancelled_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
}

func (a *AIApproval) BeforeCreate(tx *gorm.DB) error {
	if a.ID == uuid.Nil {
		a.ID = uuid.New()
	}
	if a.ExpiresAt.IsZero() {
		a.ExpiresAt = time.Now().Add(24 * time.Hour)
	}
	return nil
}

func (AIApproval) TableName() string {
	return "ai_approvals"
}

// AIToolInvocation records every MCP tool execution for auditability
type AIToolInvocation struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ConversationID  *uuid.UUID `gorm:"type:uuid;index" json:"conversation_id,omitempty"`
	RunID           *uuid.UUID `gorm:"type:uuid;index" json:"run_id,omitempty"`
	ToolName        string     `gorm:"type:varchar(128);not null" json:"tool_name"`
	ServerName      string     `gorm:"type:varchar(128);not null" json:"server_name"`
	InputJSON       string     `gorm:"type:text" json:"input_json"`
	OutputJSON      string     `gorm:"type:text" json:"output_json"`
	Status          string     `gorm:"type:varchar(32);not null" json:"status"` // success, error, blocked
	RiskLevel       string     `gorm:"type:varchar(32);not null;default:'read'" json:"risk_level"`
	ExecutionTimeMs int64      `gorm:"not null;default:0" json:"execution_time_ms"`
	CreatedAt       time.Time  `gorm:"index" json:"created_at"`
}

func (t *AIToolInvocation) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (AIToolInvocation) TableName() string {
	return "ai_tool_invocations"
}

// AIAgentRun tracks an agent LLM execution cycle
type AIAgentRun struct {
	ID               uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ConversationID   uuid.UUID  `gorm:"type:uuid;not null;index" json:"conversation_id"`
	UserID           uuid.UUID  `gorm:"type:uuid;not null;index" json:"user_id"`
	Status           string     `gorm:"type:varchar(32);not null;default:'running'" json:"status"` // running, completed, failed, cancelled
	Model            string     `gorm:"type:varchar(128)" json:"model"`
	PromptTokens     int        `gorm:"default:0" json:"prompt_tokens"`
	CompletionTokens int        `gorm:"default:0" json:"completion_tokens"`
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

func (r *AIAgentRun) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (AIAgentRun) TableName() string {
	return "ai_agent_runs"
}

// Structured UI Component Types for rich interactive chat interactions

type UIComponentType string

const (
	UIComponentOptionsCard   UIComponentType = "options_card"
	UIComponentPlanReview    UIComponentType = "plan_review"
	UIComponentProgressSteps UIComponentType = "progress_steps"
	UIComponentSuccessReport UIComponentType = "success_report"
	UIComponentErrorReport   UIComponentType = "error_report"
)

type StructuredQuestionOption struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Value       string `json:"value"`
	IsDefault   bool   `json:"is_default,omitempty"`
}

type StructuredQuestion struct {
	ID           string                     `json:"id"`
	Key          string                     `json:"key"`
	Title        string                     `json:"title"`
	Description  string                     `json:"description,omitempty"`
	Type         string                     `json:"type"` // "single_choice", "multi_choice", "text", "slider"
	Options      []StructuredQuestionOption `json:"options,omitempty"`
	Required     bool                       `json:"required"`
	DefaultValue string                     `json:"default_value,omitempty"`
	Answered     bool                       `json:"answered"`
	Answer       string                     `json:"answer,omitempty"`
}

type UIProgressStep struct {
	StepID      string `json:"step_id"`
	Title       string `json:"title"`
	Status      string `json:"status"` // "pending", "in_progress", "completed", "failed"
	Message     string `json:"message,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

type UIComponentsPayload struct {
	Questions     []StructuredQuestion `json:"questions,omitempty"`
	PlanProposal  *StructuredPlanView  `json:"plan_proposal,omitempty"`
	ProgressSteps []UIProgressStep     `json:"progress_steps,omitempty"`
	SuccessData   map[string]any       `json:"success_data,omitempty"`
	ErrorData     map[string]any       `json:"error_data,omitempty"`
}

type StructuredPlanView struct {
	PlanID               string                 `json:"plan_id"`
	Version              int                    `json:"version"`
	Title                string                 `json:"title"`
	Intent               string                 `json:"intent"`
	Summary              string                 `json:"summary"`
	Resources            []StructuredResourceView `json:"resources"`
	EstimatedAllocations map[string]any         `json:"estimated_allocations"`
	EstimatedPricing     string                 `json:"estimated_pricing,omitempty"`
	Reversible           bool                   `json:"reversible"`
	RequiresConfirmation bool                   `json:"requires_confirmation"`
	Status               string                 `json:"status"`
}

type StructuredResourceView struct {
	Type        string         `json:"type"` // VM, Application, n8n, OpenClaw, Bucket
	Name        string         `json:"name"`
	Summary     string         `json:"summary"`
	Details     map[string]any `json:"details"`
	Order       int            `json:"order"`
	Status      string         `json:"status"`
}
