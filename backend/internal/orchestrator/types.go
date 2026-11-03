// Package orchestrator is the deterministic core of the AI infrastructure
// orchestrator: the tool registry, plan specifications and their task graphs
// (with spec hashes), the policy engine, and the execution engine that runs
// task graphs from Postgres leases.
//
// The planner (chat or MCP) only proposes specifications. The policy engine
// validates them, the approval manager (usecase) authorizes one revision, the
// execution engine runs it and the verification hooks confirm the result.
//
// Like the use case layer, this package never imports gorm: persistence goes
// through repository interfaces, and tool implementations are bound at the
// composition root (they live in package usecase).
package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Risk levels (task risk = tool risk; plan risk = max of its tasks).
const (
	RiskRead        = "read"
	RiskLow         = "low"
	RiskMedium      = "medium"
	RiskHigh        = "high"
	RiskDestructive = "destructive"
)

// RiskRank orders risk levels; unknown levels rank highest.
func RiskRank(risk string) int {
	switch risk {
	case RiskRead:
		return 0
	case RiskLow:
		return 1
	case RiskMedium:
		return 2
	case RiskHigh:
		return 3
	case RiskDestructive:
		return 4
	}
	return 5
}

// MaxRisk returns the higher of two risk levels.
func MaxRisk(a, b string) string {
	if a == "" {
		return b
	}
	if RiskRank(b) > RiskRank(a) {
		return b
	}
	return a
}

// IsWrite reports whether a risk level changes infrastructure (needs approval).
func IsWrite(risk string) bool { return RiskRank(risk) >= RiskRank(RiskMedium) }

// Approval lifetimes.
const (
	ApprovalTTL            = 60 * time.Minute
	DestructiveApprovalTTL = 15 * time.Minute
)

// ApprovalTTLFor returns how long an approval of a plan with this risk lasts.
func ApprovalTTLFor(risk string) time.Duration {
	if risk == RiskDestructive {
		return DestructiveApprovalTTL
	}
	return ApprovalTTL
}

// Specification is the desired change a plan revision describes.
type Specification struct {
	Intent    string         `json:"intent"`
	Resources []ResourceSpec `json:"resources"`
}

// ResourceSpec is one resource of a specification. DependsOn names other
// resource keys of the same specification whose tasks must finish first.
type ResourceSpec struct {
	Key       string         `json:"key"`
	Type      string         `json:"type"`
	Action    string         `json:"action"`
	DependsOn []string       `json:"depends_on,omitempty"`
	Config    map[string]any `json:"config"`
}

// TaskSpec is one node of a revision's task graph.
type TaskSpec struct {
	Key       string         `json:"key"`
	Tool      string         `json:"tool"`
	Title     string         `json:"title"`
	DependsOn []string       `json:"depends_on"`
	Risk      string         `json:"risk"`
	Role      string         `json:"role"`
	Input     map[string]any `json:"input,omitempty"`
}

// EstimatedResources are the compute resources a revision adds.
type EstimatedResources struct {
	VCPUs     int `json:"vcpus"`
	MemoryMB  int `json:"memory_mb"`
	DiskGB    int `json:"disk_gb"`
	Instances int `json:"instances"`
}

// Issue is one validation finding.
type Issue struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	TaskKey string `json:"task_key,omitempty"`
}

// Validation statuses.
const (
	ValidationPassed   = "passed"
	ValidationWarnings = "warnings"
	ValidationFailed   = "failed"
)

// Validation is the policy engine's verdict on a revision.
type Validation struct {
	Status    string    `json:"status"`
	CheckedAt time.Time `json:"checked_at"`
	Errors    []Issue   `json:"errors"`
	Warnings  []Issue   `json:"warnings"`
}

// Confirmation is what a destructive plan requires the user to type.
type Confirmation struct {
	Type     string   `json:"type"`
	Expected []string `json:"expected"`
}

// Revision is the API view of a plan revision (contract §2).
type Revision struct {
	Version              int                `json:"version"`
	SpecHash             string             `json:"spec_hash"`
	CreatedAt            time.Time          `json:"created_at"`
	CreatedBy            string             `json:"created_by"`
	Summary              string             `json:"summary"`
	Specification        Specification      `json:"specification"`
	Tasks                []TaskSpec         `json:"tasks"`
	EstimatedResources   EstimatedResources `json:"estimated_resources"`
	RiskLevel            string             `json:"risk_level"`
	RequiresApproval     bool               `json:"requires_approval"`
	ConfirmationRequired *Confirmation      `json:"confirmation_required"`
	Validation           Validation         `json:"validation"`
}

// Check is one verification check.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"` // pass | fail | skip
	Detail string `json:"detail"`
}

// Verification statuses.
const (
	Verified   = "verified"
	Unverified = "unverified"
	VerifyFail = "failed"
)

// Verification is the desired-vs-actual verdict for one task.
type Verification struct {
	Status string  `json:"status"`
	Checks []Check `json:"checks"`
}

// Summarize derives the status from the checks: any failed check fails the
// verification; any skipped check (simulated or unreachable backend) leaves
// it unverified; only all-passed checks verify it.
func (v *Verification) Summarize() *Verification {
	status := Verified
	for _, c := range v.Checks {
		switch c.Status {
		case "fail":
			status = VerifyFail
		case "skip":
			if status != VerifyFail {
				status = Unverified
			}
		}
	}
	if len(v.Checks) == 0 {
		status = Unverified
	}
	v.Status = status
	return v
}

// ReportResource is one resource in an execution report.
type ReportResource struct {
	Type   string            `json:"type"`
	ID     string            `json:"id"`
	Name   string            `json:"name"`
	Status string            `json:"status"`
	Access map[string]string `json:"access"`
}

// Report is the final report of a finished execution (no secrets).
type Report struct {
	Summary     string           `json:"summary"`
	Resources   []ReportResource `json:"resources"`
	Warnings    []string         `json:"warnings"`
	NextActions []string         `json:"next_actions"`
}

// RecoveryOption is an action offered on a failed or partial execution.
type RecoveryOption struct {
	Action           string `json:"action"`
	Label            string `json:"label"`
	RequiresApproval bool   `json:"requires_approval"`
}

// Recovery actions.
const (
	RecoveryRetryFailed   = "retry_failed"
	RecoveryKeep          = "keep"
	RecoveryDeleteCreated = "delete_created"
	// RecoveryDiagnose returns diagnostics for the failed tasks' resources.
	RecoveryDiagnose = "diagnose"
	// RecoveryCompensate proposes a destructive compensation plan (the same
	// safe mechanism as delete_created; it never runs without its own
	// approval). It is only offered when the execution created resources.
	RecoveryCompensate = "compensate"
)

// ErrCodeInterruptedNotIdempotent fails a re-claimed task whose tool is not
// idempotent: the interrupted run may already have applied its change.
const ErrCodeInterruptedNotIdempotent = "INTERRUPTED_NOT_IDEMPOTENT"

// TaskError is a tool failure with a stable code. Retryable failures are
// retried with backoff while attempts remain (never for tools that are not
// idempotent).
type TaskError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	Retryable bool   `json:"-"`
}

func (e *TaskError) Error() string { return e.Code + ": " + e.Message }

// Permanent returns a failure that is not retried.
func Permanent(code, message string) *TaskError {
	return &TaskError{Code: code, Message: message}
}

// Retryable returns a failure that is retried while attempts remain.
func Retryable(code, message string) *TaskError {
	return &TaskError{Code: code, Message: message, Retryable: true}
}

// AsTaskError converts err; plain errors become TOOL_ERROR with the given
// retryability.
func AsTaskError(err error, retryable bool) *TaskError {
	var te *TaskError
	if errors.As(err, &te) {
		return te
	}
	return &TaskError{Code: "TOOL_ERROR", Message: err.Error(), Retryable: retryable}
}

// IdempotencyKey is the key of one attempt of a task.
func IdempotencyKey(executionID uuid.UUID, taskKey string, attempt int) string {
	return fmt.Sprintf("orch:%s:%s:%d", executionID, taskKey, attempt)
}

// TaskContext is what a tool receives for one attempt of a task.
type TaskContext struct {
	ExecutionID        uuid.UUID
	ExecutionCreatedAt time.Time
	PlanID             uuid.UUID
	ConversationID     uuid.UUID
	TaskID             uuid.UUID
	TaskKey            string
	Tool               string
	UserID             uuid.UUID
	ClientIP           string
	Attempt            int
	IdempotencyKey     string
	// Resumed is true when this attempt was already started by a runner
	// that lost its lease (crash, restart): the tool must re-drive the same
	// job/resource instead of starting a new one.
	Resumed           bool
	Input             map[string]any
	DependencyOutputs map[string]map[string]any

	progress func(message string, metadata map[string]any)
	record   func(relation, resourceType string, id uuid.UUID, name string)
}

// Progress appends a task.progress event.
func (tc *TaskContext) Progress(message string, metadata map[string]any) {
	if tc.progress != nil {
		tc.progress(message, metadata)
	}
}

// RecordResource records that this task created, modified or deleted a
// resource (resource_relationships).
func (tc *TaskContext) RecordResource(relation, resourceType string, id uuid.UUID, name string) {
	if tc.record != nil {
		tc.record(relation, resourceType, id, name)
	}
}

// KeyForAttempt returns the idempotency key of another attempt of this task.
func (tc *TaskContext) KeyForAttempt(attempt int) string {
	return IdempotencyKey(tc.ExecutionID, tc.TaskKey, attempt)
}

// NewTaskContext builds a TaskContext with hooks (tests and synchronous MCP
// read calls).
func NewTaskContext(tc TaskContext, progress func(string, map[string]any), record func(string, string, uuid.UUID, string)) *TaskContext {
	tc.progress = progress
	tc.record = record
	return &tc
}

// Runner executes one attempt of a task.
type Runner interface {
	Run(ctx context.Context, tc *TaskContext) (map[string]any, error)
}

// RunnerFunc adapts a function to Runner.
type RunnerFunc func(ctx context.Context, tc *TaskContext) (map[string]any, error)

func (f RunnerFunc) Run(ctx context.Context, tc *TaskContext) (map[string]any, error) {
	return f(ctx, tc)
}

// Input helpers: values decoded with UseNumber are json.Number.

// String returns m[key] as a string ("" when absent).
func String(m map[string]any, key string) string {
	switch v := m[key].(type) {
	case string:
		return v
	case json.Number:
		return v.String()
	case nil:
		return ""
	default:
		return fmt.Sprint(v)
	}
}

// Int returns m[key] as an int (def when absent or not a number).
func Int(m map[string]any, key string, def int) int {
	switch v := m[key].(type) {
	case json.Number:
		if n, err := v.Int64(); err == nil {
			return int(n)
		}
		if f, err := v.Float64(); err == nil {
			return int(f)
		}
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return def
}

// Bool returns m[key] as a bool.
func Bool(m map[string]any, key string) bool {
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		b, _ := strconv.ParseBool(v)
		return b
	}
	return false
}

// UUID returns m[key] parsed as a UUID.
func UUID(m map[string]any, key string) (uuid.UUID, bool) {
	id, err := uuid.Parse(String(m, key))
	return id, err == nil
}

// Strings returns m[key] as a string slice.
func Strings(m map[string]any, key string) []string {
	switch v := m[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok {
				out = append(out, s)
			}
		}
		return out
	case []string:
		return v
	}
	return nil
}
