package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Plan statuses of the orchestrator contract (docs/ORCHESTRATOR_API.md §1).
const (
	PlanStatusDraft              = "draft"
	PlanStatusValidating         = "validating"
	PlanStatusAwaitingApproval   = "awaiting_approval"
	PlanStatusApproved           = "approved"
	PlanStatusExecuting          = "executing"
	PlanStatusVerifying          = "verifying"
	PlanStatusCompleted          = "completed"
	PlanStatusRejected           = "rejected"
	PlanStatusCancelled          = "cancelled"
	PlanStatusFailed             = "failed"
	PlanStatusPartiallyCompleted = "partially_completed"
)

// Approval statuses.
const (
	ApprovalApproved   = "approved"
	ApprovalSuperseded = "superseded"
	ApprovalExpired    = "expired"
	ApprovalRejected   = "rejected"
	ApprovalConsumed   = "consumed"
	// Legacy statuses (rows written before migration 000007).
	ApprovalPending   = "pending"
	ApprovalCancelled = "cancelled"
)

// Execution statuses.
const (
	ExecQueued             = "queued"
	ExecRunning            = "running"
	ExecVerifying          = "verifying"
	ExecCompleted          = "completed"
	ExecFailed             = "failed"
	ExecPartiallyCompleted = "partially_completed"
	ExecCancelling         = "cancelling"
	ExecCancelled          = "cancelled"
)

// ActiveExecutionStatuses are the statuses of an execution that still has work
// to do (they count toward the per-user limit and block a second execution of
// the same plan).
var ActiveExecutionStatuses = []string{ExecQueued, ExecRunning, ExecVerifying, ExecCancelling}

// Task statuses.
const (
	TaskQueued       = "queued"
	TaskBlocked      = "blocked"
	TaskRunning      = "running"
	TaskRetrying     = "retrying"
	TaskSucceeded    = "succeeded"
	TaskFailed       = "failed"
	TaskCancelled    = "cancelled"
	TaskSkipped      = "skipped"
	TaskCompensating = "compensating"
)

// AIPlanRevision is one immutable version of a plan. Specification and Tasks
// hold canonical JSON; SpecHash is computed over exactly that content.
type AIPlanRevision struct {
	ID                   uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID               uuid.UUID `gorm:"type:uuid;not null" json:"plan_id"`
	Version              int       `gorm:"not null" json:"version"`
	SpecHash             string    `gorm:"type:varchar(80);not null" json:"spec_hash"`
	CreatedBy            string    `gorm:"type:varchar(16);not null" json:"created_by"`
	Summary              string    `gorm:"type:text;not null" json:"summary"`
	Specification        string    `gorm:"type:text;not null" json:"specification"`
	Tasks                string    `gorm:"type:text;not null" json:"tasks"`
	EstimatedResources   string    `gorm:"type:text;not null" json:"estimated_resources"`
	RiskLevel            string    `gorm:"type:varchar(16);not null" json:"risk_level"`
	RequiresApproval     bool      `gorm:"not null" json:"requires_approval"`
	ConfirmationRequired *string   `gorm:"type:text" json:"confirmation_required"`
	Validation           string    `gorm:"type:text;not null" json:"validation"`
	CreatedAt            time.Time `json:"created_at"`
}

func (r *AIPlanRevision) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (AIPlanRevision) TableName() string { return "ai_plan_revisions" }

// AIExecution is one run of an approved plan revision.
type AIExecution struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	PlanID          uuid.UUID  `gorm:"type:uuid;not null" json:"plan_id"`
	PlanVersion     int        `gorm:"not null" json:"plan_version"`
	ApprovalID      *uuid.UUID `gorm:"type:uuid" json:"approval_id"`
	ConversationID  uuid.UUID  `gorm:"type:uuid;not null" json:"conversation_id"`
	UserID          uuid.UUID  `gorm:"type:uuid;not null" json:"user_id"`
	Status          string     `gorm:"type:varchar(32);not null" json:"status"`
	CancelRequested bool       `gorm:"not null" json:"cancel_requested"`
	LastEventSeq    int64      `gorm:"not null" json:"-"`
	LeaseOwner      *string    `gorm:"type:varchar(128)" json:"-"`
	LeaseToken      *uuid.UUID `gorm:"type:uuid" json:"-"`
	LeaseExpiresAt  *time.Time `json:"-"`
	ClientIP        string     `gorm:"type:varchar(64);not null" json:"-"`
	Report          *string    `gorm:"type:text" json:"-"`
	RecoveryOptions string     `gorm:"type:text;not null" json:"-"`
	KeptAt          *time.Time `json:"-"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

func (e *AIExecution) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

func (AIExecution) TableName() string { return "ai_executions" }

// AIExecutionTask is one node of an execution's task graph.
type AIExecutionTask struct {
	ID             uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExecutionID    uuid.UUID  `gorm:"type:uuid;not null" json:"execution_id"`
	Position       int        `gorm:"not null" json:"position"`
	Key            string     `gorm:"column:task_key;type:varchar(160);not null" json:"key"`
	Tool           string     `gorm:"type:varchar(64);not null" json:"tool"`
	Title          string     `gorm:"type:varchar(255);not null" json:"title"`
	Risk           string     `gorm:"type:varchar(16);not null" json:"risk"`
	DependsOn      string     `gorm:"type:text;not null" json:"depends_on"`
	Input          string     `gorm:"type:text;not null" json:"input"`
	Status         string     `gorm:"type:varchar(16);not null" json:"status"`
	Attempt        int        `gorm:"not null" json:"attempt"`
	MaxAttempts    int        `gorm:"not null" json:"max_attempts"`
	NextAttemptAt  *time.Time `json:"next_attempt_at"`
	TimeoutSeconds int        `gorm:"not null" json:"timeout_seconds"`
	IdempotencyKey string     `gorm:"type:varchar(255);not null" json:"idempotency_key"`
	LeaseOwner     *string    `gorm:"type:varchar(128)" json:"-"`
	LeaseToken     *uuid.UUID `gorm:"type:uuid" json:"-"`
	LeaseExpiresAt *time.Time `json:"-"`
	HeartbeatAt    *time.Time `json:"-"`
	ClaimCount     int        `gorm:"not null" json:"-"`
	Output         *string    `gorm:"type:text" json:"output"`
	Error          *string    `gorm:"type:text" json:"error"`
	Verification   *string    `gorm:"type:text" json:"verification"`
	StartedAt      *time.Time `json:"started_at"`
	FinishedAt     *time.Time `json:"finished_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

func (t *AIExecutionTask) BeforeCreate(tx *gorm.DB) error {
	if t.ID == uuid.Nil {
		t.ID = uuid.New()
	}
	return nil
}

func (AIExecutionTask) TableName() string { return "ai_execution_tasks" }

// AIExecutionEvent is an append-only execution event; Seq increases strictly
// per execution.
type AIExecutionEvent struct {
	ID          uuid.UUID `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	ExecutionID uuid.UUID `gorm:"type:uuid;not null" json:"execution_id"`
	Seq         int64     `gorm:"not null" json:"seq"`
	Type        string    `gorm:"type:varchar(48);not null" json:"type"`
	Severity    string    `gorm:"type:varchar(16);not null" json:"severity"`
	Message     string    `gorm:"type:text;not null" json:"message"`
	TaskKey     *string   `gorm:"type:varchar(160)" json:"task_key"`
	Metadata    string    `gorm:"type:text;not null" json:"metadata"`
	CreatedAt   time.Time `json:"created_at"`
}

func (e *AIExecutionEvent) BeforeCreate(tx *gorm.DB) error {
	if e.ID == uuid.Nil {
		e.ID = uuid.New()
	}
	return nil
}

func (AIExecutionEvent) TableName() string { return "ai_execution_events" }

// Resource relations recorded by the orchestrator.
const (
	RelationCreated  = "created"
	RelationDeleted  = "deleted"
	RelationModified = "modified"
)

// ResourceRelationship records that an execution (task) created, modified or
// deleted a resource.
type ResourceRelationship struct {
	ID           uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID      uuid.UUID  `gorm:"type:uuid;not null" json:"owner_id"`
	ExecutionID  *uuid.UUID `gorm:"type:uuid" json:"execution_id"`
	TaskID       *uuid.UUID `gorm:"type:uuid" json:"task_id"`
	PlanID       *uuid.UUID `gorm:"type:uuid" json:"plan_id"`
	Relation     string     `gorm:"type:varchar(32);not null" json:"relation"`
	ResourceType string     `gorm:"type:varchar(32);not null" json:"resource_type"`
	ResourceID   uuid.UUID  `gorm:"type:uuid;not null" json:"resource_id"`
	ResourceName string     `gorm:"type:varchar(255);not null" json:"resource_name"`
	CreatedAt    time.Time  `json:"created_at"`
}

func (r *ResourceRelationship) BeforeCreate(tx *gorm.DB) error {
	if r.ID == uuid.Nil {
		r.ID = uuid.New()
	}
	return nil
}

func (ResourceRelationship) TableName() string { return "resource_relationships" }
