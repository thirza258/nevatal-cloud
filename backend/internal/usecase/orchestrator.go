package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
)

// MaxActiveExecutions is the number of executions one user may have queued,
// running, verifying or cancelling at the same time.
const MaxActiveExecutions = 3

// OrchestratorRepos are the repositories the orchestrator reads and writes.
type OrchestratorRepos struct {
	Orch      repository.OrchestratorRepository
	Agent     repository.AIAgentRepository
	Users     repository.UserRepository
	Templates repository.TemplateRepository
	Plans     repository.PlanRepository
	SSHKeys   repository.SSHKeyRepository
	Nodes     repository.NodeRepository
	Instances repository.InstanceRepository
	Snapshots repository.SnapshotRepository
	Jobs      repository.JobRepository
	Android   repository.AndroidRepository
	Storage   repository.StorageRepository
}

// OrchestratorServices are the use cases the orchestrator's tools drive.
type OrchestratorServices struct {
	Instance   *InstanceUsecase
	App        *AppUsecase
	Automation *AutomationUsecase
	OpenClaw   *OpenClawUsecase
	Storage    *StorageUsecase
	GameServer *GameServerUsecase
	Android    *AndroidUsecase
	Quota      *QuotaUsecase
	Audit      *AuditUsecase
}

// OrchestratorUsecase is the approval manager and the API of the AI
// infrastructure orchestrator: it turns planner proposals into versioned
// plan revisions, validates them with the policy engine, records approvals
// bound to a revision, starts executions on the engine and serves their
// state, events and recovery actions.
type OrchestratorUsecase struct {
	tx     repository.Transactor
	repos  OrchestratorRepos
	svc    OrchestratorServices
	cfg    *config.Config
	pve    proxmox.ProxmoxClient
	reg    *orchestrator.Registry
	policy *orchestrator.Policy
	engine *orchestrator.Engine

	// jobPoll is how often tools poll provisioning jobs.
	jobPoll time.Duration
}

// NewOrchestratorUsecase builds the orchestrator with the Phase 1 registry
// and an engine configured from cfg. Call Engine().Start() to run tasks.
func NewOrchestratorUsecase(tx repository.Transactor, repos OrchestratorRepos, svc OrchestratorServices, cfg *config.Config, pve proxmox.ProxmoxClient) *OrchestratorUsecase {
	engineCfg := orchestrator.EngineConfig{}
	if cfg != nil {
		engineCfg = orchestrator.EngineConfig{
			Concurrency:  cfg.OrchConcurrency,
			Lease:        cfg.OrchLease,
			Heartbeat:    cfg.OrchHeartbeat,
			PollInterval: cfg.OrchPollInterval,
			RetryBase:    cfg.OrchRetryBase,
			Faults:       cfg.OrchFaults,
		}
	}
	return NewOrchestratorUsecaseWith(tx, repos, svc, cfg, pve, orchestrator.DefaultRegistry(), engineCfg)
}

// NewOrchestratorUsecaseWith is NewOrchestratorUsecase with an explicit
// registry and engine configuration (tests add tools and shorten leases).
// The Phase 1 tool implementations are bound to reg.
func NewOrchestratorUsecaseWith(tx repository.Transactor, repos OrchestratorRepos, svc OrchestratorServices, cfg *config.Config, pve proxmox.ProxmoxClient, reg *orchestrator.Registry, engineCfg orchestrator.EngineConfig) *OrchestratorUsecase {
	if cfg == nil {
		cfg = &config.Config{}
	}
	s := &OrchestratorUsecase{tx: tx, repos: repos, svc: svc, cfg: cfg, pve: pve, reg: reg, jobPoll: time.Second}
	s.policy = orchestrator.NewPolicy(reg, &policyEnv{s: s})
	s.bindTools()
	s.engine = orchestrator.NewEngine(repos.Orch, tx, reg, s, nil, engineCfg)
	return s
}

// Engine returns the execution engine.
func (s *OrchestratorUsecase) Engine() *orchestrator.Engine { return s.engine }

// Registry returns the tool registry.
func (s *OrchestratorUsecase) Registry() *orchestrator.Registry { return s.reg }

// SetJobPollInterval changes how often tools poll provisioning jobs (tests).
func (s *OrchestratorUsecase) SetJobPollInterval(d time.Duration) { s.jobPoll = d }

// Errors (contract §7).
var (
	errPlanNotFound      = newError(KindNotFound, "PLAN_NOT_FOUND", "plan not found")
	errExecutionNotFound = newError(KindNotFound, "EXECUTION_NOT_FOUND", "execution not found")
	errTaskNotFound      = newError(KindNotFound, "TASK_NOT_FOUND", "task not found")
	errAdminRequired     = newError(KindForbidden, "ADMIN_REQUIRED", "this plan uses an administrator-only tool")
	errExecutionActive   = newError(KindConflict, "EXECUTION_ACTIVE", "the plan already has an active execution")
	errTooMany           = newError(KindTooManyRequests, "TOO_MANY_EXECUTIONS", fmt.Sprintf("at most %d executions may be active at once", MaxActiveExecutions))
	errRiskAck           = newError(KindUnprocessable, "RISK_ACK_REQUIRED", "this plan is high risk: set acknowledge_risks to true to approve it")
	errConfirmRequired   = newError(KindUnprocessable, "CONFIRMATION_REQUIRED", "this plan is destructive: type the name(s) of the affected resource(s) to confirm")
	errConfirmMismatch   = newError(KindUnprocessable, "CONFIRMATION_MISMATCH", "the confirmation does not match the affected resource name(s)")
	errTaskNotRetryable  = newError(KindConflict, "TASK_NOT_RETRYABLE", "only failed tasks can be retried")
	errNotActive         = newError(KindConflict, "EXECUTION_NOT_ACTIVE", "the execution has already finished")
)

func versionMismatch(msg string) *Error {
	return newError(KindConflict, "PLAN_VERSION_MISMATCH", msg)
}

// ValidationIssues are the details of a 422 VALIDATION_FAILED error.
type ValidationIssues struct {
	Errors   []orchestrator.Issue `json:"errors"`
	Warnings []orchestrator.Issue `json:"warnings"`
}

func validationFailed(v orchestrator.Validation) *Error {
	e := newError(KindUnprocessable, "VALIDATION_FAILED", "the plan does not pass policy validation")
	if len(v.Errors) > 0 {
		e.Message = "the plan does not pass policy validation: " + v.Errors[0].Message
	}
	e.Details = ValidationIssues{Errors: v.Errors, Warnings: v.Warnings}
	return e
}

// Views (contract §2, §4, §5).

// ApprovalView is the approval object of a plan.
type ApprovalView struct {
	ID        uuid.UUID `json:"id"`
	Version   int       `json:"version"`
	SpecHash  string    `json:"spec_hash"`
	Status    string    `json:"status"`
	RiskLevel string    `json:"risk_level"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// PlanDetail is GET /ai/plans/:id: the plan's existing fields followed by
// the orchestrator fields.
type PlanDetail struct {
	models.AIResourcePlan
	CurrentVersion    int                    `json:"current_version"`
	Revision          *orchestrator.Revision `json:"revision"`
	Approval          *ApprovalView          `json:"approval"`
	LatestExecutionID *uuid.UUID             `json:"latest_execution_id"`
}

// RevisionSummary is one item of GET /ai/plans/:id/revisions.
type RevisionSummary struct {
	Version   int       `json:"version"`
	SpecHash  string    `json:"spec_hash"`
	CreatedAt time.Time `json:"created_at"`
	CreatedBy string    `json:"created_by"`
	Summary   string    `json:"summary"`
	RiskLevel string    `json:"risk_level"`
}

// TaskView is one task of an execution.
type TaskView struct {
	ID            uuid.UUID       `json:"id"`
	Key           string          `json:"key"`
	Tool          string          `json:"tool"`
	Title         string          `json:"title"`
	Risk          string          `json:"risk"`
	DependsOn     []string        `json:"depends_on"`
	Status        string          `json:"status"`
	Attempt       int             `json:"attempt"`
	MaxAttempts   int             `json:"max_attempts"`
	NextAttemptAt *time.Time      `json:"next_attempt_at"`
	StartedAt     *time.Time      `json:"started_at"`
	FinishedAt    *time.Time      `json:"finished_at"`
	Output        json.RawMessage `json:"output"`
	Error         json.RawMessage `json:"error"`
	Verification  json.RawMessage `json:"verification"`
}

type executionBase struct {
	ID              uuid.UUID  `json:"id"`
	PlanID          uuid.UUID  `json:"plan_id"`
	PlanVersion     int        `json:"plan_version"`
	ConversationID  uuid.UUID  `json:"conversation_id"`
	Status          string     `json:"status"`
	CancelRequested bool       `json:"cancel_requested"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
}

// ExecutionView is an execution with its tasks (contract §4).
type ExecutionView struct {
	executionBase
	Tasks           []TaskView                    `json:"tasks"`
	Report          json.RawMessage               `json:"report"`
	RecoveryOptions []orchestrator.RecoveryOption `json:"recovery_options"`
}

// ExecutionSummaryView is an execution without tasks, with task counts.
type ExecutionSummaryView struct {
	executionBase
	Report          json.RawMessage               `json:"report"`
	RecoveryOptions []orchestrator.RecoveryOption `json:"recovery_options"`
	TaskCounts      map[string]int                `json:"task_counts"`
}

// EventView is one execution event (contract §5).
type EventView struct {
	Seq       int64           `json:"seq"`
	ID        uuid.UUID       `json:"id"`
	Type      string          `json:"type"`
	Severity  string          `json:"severity"`
	Message   string          `json:"message"`
	TaskKey   *string         `json:"task_key"`
	Metadata  json.RawMessage `json:"metadata"`
	CreatedAt time.Time       `json:"created_at"`
}

// EventsPage is GET /ai/executions/:id/events.
type EventsPage struct {
	Events    []EventView `json:"events"`
	NextAfter int64       `json:"next_after"`
}

// ValidateResult is POST /ai/plans/:id/validate: the legacy fields plus the
// policy verdict.
type ValidateResult struct {
	PlanID     uuid.UUID               `json:"plan_id"`
	Status     string                  `json:"status"`
	Valid      bool                    `json:"valid"`
	Validation orchestrator.Validation `json:"validation"`
}

// ApproveInput is the body of POST /ai/plans/:id/approve.
type ApproveInput struct {
	Version          *int
	SpecHash         string
	AcknowledgeRisks bool
	Confirmation     string
	// Execute defaults to true.
	Execute *bool
}

// ApproveResult is the reply of POST /ai/plans/:id/approve (legacy fields
// kept; JSON keys in alphabetical order).
type ApproveResult struct {
	Approval      *ApprovalView           `json:"approval"`
	Execution     *ExecutionView          `json:"execution"`
	Message       string                  `json:"message"`
	PlanID        uuid.UUID               `json:"plan_id"`
	ProgressSteps []models.UIProgressStep `json:"progress_steps"`
	ResultDetails map[string]any          `json:"result_details"`
	Status        string                  `json:"status"`
}

// PatchPlanInput is the body of PATCH /ai/plans/:id.
type PatchPlanInput struct {
	BaseVersion   int
	Specification orchestrator.Specification
}

// RecoveryResult is the reply of POST /ai/executions/:id/recovery: an
// execution (retry_failed, keep) or the new plan (delete_created).
type RecoveryResult struct {
	Execution *ExecutionView
	PlanID    *uuid.UUID
}

// ListTools returns the registry (GET /ai/tools, MCP tools/list).
func (s *OrchestratorUsecase) ListTools() []orchestrator.ToolDef {
	return s.reg.List()
}

// Notifier exposes the event notifier for SSE streams.
func (s *OrchestratorUsecase) Notifier() *orchestrator.Notifier { return s.engine.Notifier() }

// Revisions

func revisionView(rev *models.AIPlanRevision) (*orchestrator.Revision, error) {
	v := &orchestrator.Revision{
		Version:          rev.Version,
		SpecHash:         rev.SpecHash,
		CreatedAt:        rev.CreatedAt.UTC(),
		CreatedBy:        rev.CreatedBy,
		Summary:          rev.Summary,
		RiskLevel:        rev.RiskLevel,
		RequiresApproval: rev.RequiresApproval,
	}
	if err := orchestrator.DecodeJSON(rev.Specification, &v.Specification); err != nil {
		return nil, fmt.Errorf("decode specification: %w", err)
	}
	if err := orchestrator.DecodeJSON(rev.Tasks, &v.Tasks); err != nil {
		return nil, fmt.Errorf("decode tasks: %w", err)
	}
	if v.Tasks == nil {
		v.Tasks = []orchestrator.TaskSpec{}
	}
	for i := range v.Tasks {
		if v.Tasks[i].DependsOn == nil {
			v.Tasks[i].DependsOn = []string{}
		}
	}
	if v.Specification.Resources == nil {
		v.Specification.Resources = []orchestrator.ResourceSpec{}
	}
	_ = orchestrator.DecodeJSON(rev.EstimatedResources, &v.EstimatedResources)
	_ = orchestrator.DecodeJSON(rev.Validation, &v.Validation)
	if v.Validation.Errors == nil {
		v.Validation.Errors = []orchestrator.Issue{}
	}
	if v.Validation.Warnings == nil {
		v.Validation.Warnings = []orchestrator.Issue{}
	}
	if rev.ConfirmationRequired != nil && *rev.ConfirmationRequired != "" && *rev.ConfirmationRequired != "null" {
		var c orchestrator.Confirmation
		if orchestrator.DecodeJSON(*rev.ConfirmationRequired, &c) == nil {
			v.ConfirmationRequired = &c
		}
	}
	return v, nil
}

// buildRevision normalizes spec, compiles its task graph, validates it and
// computes its hash. A specification that cannot be compiled yields a
// revision whose validation failed (only CreatePlan and PatchPlan reject it).
func (s *OrchestratorUsecase) buildRevision(ctx context.Context, userID uuid.UUID, isAdmin bool, spec orchestrator.Specification, createdBy, summary string) (*models.AIPlanRevision, error) {
	norm, err := s.policy.Normalize(ctx, userID, spec)
	if err != nil {
		return nil, err
	}
	var validation orchestrator.Validation
	var confirmation *orchestrator.Confirmation
	var estimate orchestrator.EstimatedResources
	tasks, compileErr := orchestrator.Compile(s.reg, norm)
	if compileErr == nil {
		if tasks, err = orchestrator.GenericTasks(tasks); err != nil {
			return nil, err
		}
		res := s.policy.Validate(ctx, userID, isAdmin, norm, tasks)
		validation, confirmation, estimate = res.Validation, res.Confirmation, res.Estimate
	} else {
		tasks = []orchestrator.TaskSpec{}
		validation = orchestrator.Validation{
			Status: orchestrator.ValidationFailed, CheckedAt: time.Now().UTC(),
			Errors:   []orchestrator.Issue{{Code: "SPEC_INVALID", Message: compileErr.Error()}},
			Warnings: []orchestrator.Issue{},
		}
		estimate = orchestrator.Estimate(norm)
	}
	hash, err := orchestrator.SpecHash(norm, tasks)
	if err != nil {
		return nil, err
	}
	risk := orchestrator.PlanRisk(tasks)
	specJSON, err := orchestrator.Canonical(norm)
	if err != nil {
		return nil, err
	}
	tasksJSON, err := orchestrator.Canonical(tasks)
	if err != nil {
		return nil, err
	}
	if summary == "" {
		summary = orchestrator.Summarize(norm)
	}
	rev := &models.AIPlanRevision{
		SpecHash:           hash,
		CreatedBy:          createdBy,
		Summary:            summary,
		Specification:      string(specJSON),
		Tasks:              string(tasksJSON),
		EstimatedResources: orchestrator.MustJSON(estimate),
		RiskLevel:          risk,
		RequiresApproval:   orchestrator.IsWrite(risk),
		Validation:         orchestrator.MustJSON(validation),
		CreatedAt:          time.Now(),
	}
	if confirmation != nil {
		c := orchestrator.MustJSON(confirmation)
		rev.ConfirmationRequired = &c
	}
	return rev, nil
}

// verifyRevisionHash recomputes the hash of a stored revision.
func verifyRevisionHash(view *orchestrator.Revision) bool {
	hash, err := orchestrator.SpecHash(view.Specification, view.Tasks)
	return err == nil && hash == view.SpecHash
}

// currentRevision returns the plan's current revision, creating revision 1
// for plans written before the orchestrator existed (lazy backfill).
func (s *OrchestratorUsecase) currentRevision(ctx context.Context, plan *models.AIResourcePlan) (*models.AIPlanRevision, *orchestrator.Revision, error) {
	rev, err := s.repos.Orch.FindRevision(ctx, plan.ID, plan.Version)
	if errors.Is(err, repository.ErrNotFound) {
		if err := s.backfillPlan(ctx, plan); err != nil {
			return nil, nil, err
		}
		rev, err = s.repos.Orch.FindRevision(ctx, plan.ID, plan.Version)
	}
	if err != nil {
		return nil, nil, err
	}
	view, err := revisionView(rev)
	if err != nil {
		return nil, nil, err
	}
	return rev, view, nil
}

// PlanDraft is a plan proposed by a planner (chat agent, MCP, recovery).
type PlanDraft struct {
	ConversationID uuid.UUID
	UserID         uuid.UUID
	IsAdmin        bool
	Intent         string
	Title          string
	Description    string
	Summary        string
	EstimatedCosts string
	Spec           orchestrator.Specification
}

// CreatePlan stores a planner proposal as a plan with revision 1. The plan
// awaits approval when the policy engine accepts it and stays a draft
// otherwise. No approval is created: only POST /ai/plans/:id/approve
// approves.
func (s *OrchestratorUsecase) CreatePlan(ctx context.Context, d PlanDraft) (*models.AIResourcePlan, *orchestrator.Revision, error) {
	ctx = context.WithoutCancel(ctx)
	if err := s.reg.CheckStructure(d.Spec); err != nil {
		return nil, nil, specError(err)
	}
	rev, err := s.buildRevision(ctx, d.UserID, d.IsAdmin, d.Spec, "planner", d.Summary)
	if err != nil {
		return nil, nil, specError(err)
	}
	view, err := revisionView(rev)
	if err != nil {
		return nil, nil, err
	}
	status := models.PlanStatusAwaitingApproval
	if view.Validation.Status == orchestrator.ValidationFailed {
		status = models.PlanStatusDraft
	}
	if d.Intent == "" {
		d.Intent = view.Specification.Intent
	}
	if d.Title == "" {
		d.Title = view.Summary
	}
	if len(d.Title) > 250 {
		d.Title = d.Title[:250]
	}
	plan := models.AIResourcePlan{
		ConversationID:       d.ConversationID,
		UserID:               d.UserID,
		Version:              1,
		Intent:               d.Intent,
		Status:               status,
		Title:                d.Title,
		Description:          d.Description,
		PlanJSON:             legacyPlanJSON(view.Specification),
		EstimatedCosts:       d.EstimatedCosts,
		ValidationErrors:     issueMessages(view.Validation.Errors),
		RequiresConfirmation: view.RequiresApproval,
		CreatedAt:            time.Now(),
		UpdatedAt:            time.Now(),
	}
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.repos.Agent.CreatePlan(txCtx, &plan); err != nil {
			return err
		}
		for _, res := range legacyResources(plan.ID, view.Specification) {
			r := res
			if err := s.repos.Agent.CreatePlanResource(txCtx, &r); err != nil {
				return err
			}
		}
		rev.PlanID = plan.ID
		rev.Version = 1
		return s.repos.Orch.CreateRevision(txCtx, rev)
	})
	if err != nil {
		return nil, nil, err
	}
	view.Version = 1
	view.CreatedAt = rev.CreatedAt.UTC()
	return &plan, view, nil
}

func specError(err error) error {
	var se *orchestrator.SpecError
	if errors.As(err, &se) {
		return ValidationError(se.Field, se.Error())
	}
	var ue *Error
	if errors.As(err, &ue) {
		return ue
	}
	return err
}

func issueMessages(issues []orchestrator.Issue) string {
	msgs := make([]string, 0, len(issues))
	for _, i := range issues {
		msgs = append(msgs, i.Message)
	}
	b, _ := json.Marshal(msgs)
	return string(b)
}

// Plans

func (s *OrchestratorUsecase) findPlan(ctx context.Context, planID, userID uuid.UUID) (*models.AIResourcePlan, error) {
	plan, err := s.repos.Agent.FindPlanDetails(ctx, planID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errPlanNotFound
		}
		return nil, err
	}
	return plan, nil
}

func approvalView(a *models.AIApproval) *ApprovalView {
	if a == nil {
		return nil
	}
	status := a.Status
	switch status {
	case models.ApprovalApproved:
		if time.Now().After(a.ExpiresAt) {
			status = models.ApprovalExpired
		}
	case models.ApprovalPending, models.ApprovalCancelled:
		// Legacy rows: they can never be used to execute.
		status = models.ApprovalExpired
	}
	version := a.Version
	if a.PlanVersion != nil {
		version = *a.PlanVersion
	}
	return &ApprovalView{
		ID: a.ID, Version: version, SpecHash: a.SpecHash, Status: status, RiskLevel: a.RiskLevel,
		ExpiresAt: a.ExpiresAt.UTC(), CreatedAt: a.CreatedAt.UTC(),
	}
}

// GetPlan returns a plan with its current revision, latest approval and
// latest execution.
func (s *OrchestratorUsecase) GetPlan(ctx context.Context, planID, userID uuid.UUID) (*PlanDetail, error) {
	plan, err := s.findPlan(ctx, planID, userID)
	if err != nil {
		return nil, err
	}
	return s.planDetail(ctx, plan)
}

func (s *OrchestratorUsecase) planDetail(ctx context.Context, plan *models.AIResourcePlan) (*PlanDetail, error) {
	_, view, err := s.currentRevision(ctx, plan)
	if err != nil {
		return nil, err
	}
	detail := &PlanDetail{AIResourcePlan: *plan, CurrentVersion: plan.Version, Revision: view}
	if a, err := s.repos.Orch.LatestApproval(ctx, plan.ID); err == nil {
		detail.Approval = approvalView(a)
	}
	if e, err := s.repos.Orch.LatestExecution(ctx, plan.ID); err == nil {
		id := e.ID
		detail.LatestExecutionID = &id
	}
	return detail, nil
}

// ListRevisions returns the plan's revisions, oldest first.
func (s *OrchestratorUsecase) ListRevisions(ctx context.Context, planID, userID uuid.UUID) ([]RevisionSummary, error) {
	plan, err := s.findPlan(ctx, planID, userID)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.currentRevision(ctx, plan); err != nil {
		return nil, err
	}
	revs, err := s.repos.Orch.ListRevisions(ctx, plan.ID)
	if err != nil {
		return nil, err
	}
	out := make([]RevisionSummary, 0, len(revs))
	for _, r := range revs {
		out = append(out, RevisionSummary{Version: r.Version, SpecHash: r.SpecHash, CreatedAt: r.CreatedAt.UTC(),
			CreatedBy: r.CreatedBy, Summary: r.Summary, RiskLevel: r.RiskLevel})
	}
	return out, nil
}

// GetRevision returns one revision of a plan.
func (s *OrchestratorUsecase) GetRevision(ctx context.Context, planID, userID uuid.UUID, version int) (*orchestrator.Revision, error) {
	plan, err := s.findPlan(ctx, planID, userID)
	if err != nil {
		return nil, err
	}
	if _, _, err := s.currentRevision(ctx, plan); err != nil {
		return nil, err
	}
	rev, err := s.repos.Orch.FindRevision(ctx, plan.ID, version)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, newError(KindNotFound, "PLAN_NOT_FOUND", fmt.Sprintf("plan has no revision %d", version))
		}
		return nil, err
	}
	return revisionView(rev)
}

func isAwaiting(status string) bool {
	switch status {
	case models.PlanStatusDraft, models.PlanStatusValidating, models.PlanStatusAwaitingApproval, "awaiting_confirmation":
		return true
	}
	return false
}

// editable reports whether a plan may still be changed, approved or
// rejected: before approval, or approved while no execution has started.
func (s *OrchestratorUsecase) editable(ctx context.Context, plan *models.AIResourcePlan) bool {
	if isAwaiting(plan.Status) {
		return true
	}
	if plan.Status != models.PlanStatusApproved {
		return false
	}
	latest, err := s.repos.Orch.LatestExecution(ctx, plan.ID)
	if errors.Is(err, repository.ErrNotFound) {
		return true
	}
	return err == nil && isTerminalExecStatus(latest.Status)
}

func isTerminalPlan(status string) bool {
	switch status {
	case models.PlanStatusCompleted, models.PlanStatusRejected, models.PlanStatusCancelled,
		models.PlanStatusFailed, models.PlanStatusPartiallyCompleted:
		return true
	}
	return false
}

// PatchPlan stores a user-edited specification as a new revision. Tasks,
// risk, confirmation and hash are recomputed server-side; every earlier
// approval is superseded.
func (s *OrchestratorUsecase) PatchPlan(ctx context.Context, planID, userID uuid.UUID, isAdmin bool, in PatchPlanInput) (*PlanDetail, error) {
	ctx = context.WithoutCancel(ctx)
	if err := s.reg.CheckStructure(in.Specification); err != nil {
		return nil, specError(err)
	}
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		plan, err := s.repos.Orch.LockPlan(txCtx, planID, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return errPlanNotFound
			}
			return err
		}
		if !s.editable(txCtx, plan) {
			return newError(KindConflict, "PLAN_NOT_EDITABLE", fmt.Sprintf("the plan is %s and can no longer be edited", plan.Status))
		}
		if in.BaseVersion != plan.Version {
			return newError(KindConflict, "PLAN_VERSION_CONFLICT", fmt.Sprintf("base_version %d is not the current version %d", in.BaseVersion, plan.Version))
		}
		_, err = s.addRevision(txCtx, plan, userID, isAdmin, in.Specification, "user")
		return err
	})
	if err != nil {
		return nil, err
	}
	return s.GetPlan(ctx, planID, userID)
}

// RevisePlan stores spec as a new planner revision of an open plan (a
// follow-up chat message refining the same request). It reports false, and
// changes nothing, when the plan has another intent or can no longer be
// edited: the caller then creates a new plan.
func (s *OrchestratorUsecase) RevisePlan(ctx context.Context, planID, userID uuid.UUID, intent string, spec orchestrator.Specification) (*models.AIResourcePlan, *orchestrator.Revision, bool, error) {
	ctx = context.WithoutCancel(ctx)
	if err := s.reg.CheckStructure(spec); err != nil {
		return nil, nil, false, specError(err)
	}
	var plan *models.AIResourcePlan
	var view *orchestrator.Revision
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		p, err := s.repos.Orch.LockPlan(txCtx, planID, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil
			}
			return err
		}
		if p.Intent != intent || !s.editable(txCtx, p) {
			return nil
		}
		v, err := s.addRevision(txCtx, p, userID, false, spec, "planner")
		if err != nil {
			return err
		}
		plan, view = p, v
		return nil
	})
	if err != nil || plan == nil {
		return nil, nil, false, err
	}
	fresh, err := s.repos.Orch.FindPlan(ctx, plan.ID)
	if err != nil {
		return nil, nil, false, err
	}
	return fresh, view, true, nil
}

// addRevision appends a revision built from spec to a locked plan (no-op
// when spec yields the current hash), supersedes earlier approvals and
// brings the plan row and its legacy resources in line.
func (s *OrchestratorUsecase) addRevision(txCtx context.Context, plan *models.AIResourcePlan, userID uuid.UUID, isAdmin bool, spec orchestrator.Specification, createdBy string) (*orchestrator.Revision, error) {
	_, current, err := s.currentRevision(txCtx, plan)
	if err != nil {
		return nil, err
	}
	rev, err := s.buildRevision(txCtx, userID, isAdmin, spec, createdBy, "")
	if err != nil {
		return nil, specError(err)
	}
	if createdBy == "planner" && rev.SpecHash == current.SpecHash {
		// A chat follow-up that changes nothing keeps the current revision.
		// A user PATCH always creates a new revision and supersedes.
		return current, nil
	}
	view, err := revisionView(rev)
	if err != nil {
		return nil, err
	}
	rev.PlanID = plan.ID
	rev.Version = plan.Version + 1
	if err := s.repos.Orch.CreateRevision(txCtx, rev); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, newError(KindConflict, "PLAN_VERSION_CONFLICT", "the plan was changed concurrently")
		}
		return nil, err
	}
	view.Version = rev.Version
	if err := s.repos.Orch.SupersedeApprovals(txCtx, plan.ID, time.Now()); err != nil {
		return nil, err
	}
	status := models.PlanStatusAwaitingApproval
	if view.Validation.Status == orchestrator.ValidationFailed {
		status = models.PlanStatusDraft
	}
	if err := s.repos.Orch.UpdatePlan(txCtx, plan.ID, map[string]any{
		"version":               rev.Version,
		"status":                status,
		"description":           view.Summary,
		"plan_json":             legacyPlanJSON(view.Specification),
		"validation_errors":     issueMessages(view.Validation.Errors),
		"requires_confirmation": view.RequiresApproval,
	}); err != nil {
		return nil, err
	}
	plan.Version, plan.Status = rev.Version, status
	if err := s.repos.Orch.ReplacePlanResources(txCtx, plan.ID, legacyResources(plan.ID, view.Specification)); err != nil {
		return nil, err
	}
	return view, nil
}

// ValidatePlan re-runs the policy engine on the current revision and stores
// the verdict on it.
func (s *OrchestratorUsecase) ValidatePlan(ctx context.Context, planID, userID uuid.UUID, isAdmin bool) (*ValidateResult, error) {
	ctx = context.WithoutCancel(ctx)
	plan, err := s.findPlan(ctx, planID, userID)
	if err != nil {
		return nil, err
	}
	rev, view, err := s.currentRevision(ctx, plan)
	if err != nil {
		return nil, err
	}
	res := s.policy.Validate(ctx, userID, isAdmin, view.Specification, view.Tasks)
	_ = s.repos.Orch.UpdateRevisionValidation(ctx, rev.ID, orchestrator.MustJSON(res.Validation))
	if isAwaiting(plan.Status) {
		status := models.PlanStatusAwaitingApproval
		if res.Validation.Status == orchestrator.ValidationFailed {
			status = models.PlanStatusDraft
		}
		if status != plan.Status {
			_ = s.repos.Orch.UpdatePlan(ctx, plan.ID, map[string]any{"status": status})
			plan.Status = status
		}
	}
	return &ValidateResult{
		PlanID: plan.ID, Status: plan.Status,
		Valid:      res.Validation.Status != orchestrator.ValidationFailed,
		Validation: res.Validation,
	}, nil
}

func (s *OrchestratorUsecase) requiresAdmin(tasks []orchestrator.TaskSpec) bool {
	for _, t := range tasks {
		if def, ok := s.reg.Def(t.Tool); ok && def.RequiresAdmin {
			return true
		}
	}
	return false
}

func (s *OrchestratorUsecase) userIsAdmin(ctx context.Context, userID uuid.UUID) bool {
	u, err := s.repos.Users.FindByID(ctx, userID)
	return err == nil && u.Role == models.RoleAdmin
}

// ApprovePlan is the approval manager: it binds an approval to the current
// revision (version + spec hash), enforces the risk gates, re-runs the policy
// engine and, unless execute is false, starts the execution that consumes
// the approval. Nothing else approves a plan.
func (s *OrchestratorUsecase) ApprovePlan(ctx context.Context, planID, userID uuid.UUID, isAdmin bool, in ApproveInput, clientIP string) (*ApproveResult, error) {
	ctx = context.WithoutCancel(ctx)
	execute := in.Execute == nil || *in.Execute
	var result *ApproveResult
	var execID *uuid.UUID
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		plan, err := s.repos.Orch.LockPlan(txCtx, planID, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return errPlanNotFound
			}
			return err
		}
		if err := s.checkApprovable(txCtx, plan); err != nil {
			return err
		}
		rev, view, err := s.currentRevision(txCtx, plan)
		if err != nil {
			return err
		}
		if !verifyRevisionHash(view) {
			return versionMismatch("the stored plan revision does not match its spec hash")
		}
		// Version binding.
		if in.Version != nil && *in.Version != plan.Version {
			return versionMismatch(fmt.Sprintf("version %d is not the current version %d", *in.Version, plan.Version))
		}
		if in.SpecHash != "" {
			if in.SpecHash != view.SpecHash {
				return versionMismatch("spec_hash does not match the current revision")
			}
		} else if orchestrator.RiskRank(view.RiskLevel) > orchestrator.RiskRank(orchestrator.RiskMedium) {
			return ValidationError("spec_hash", "version and spec_hash are required to approve a high-risk or destructive plan")
		}
		if s.requiresAdmin(view.Tasks) && !(isAdmin && s.userIsAdmin(txCtx, userID)) {
			return errAdminRequired
		}
		res := s.policy.Validate(txCtx, userID, isAdmin, view.Specification, view.Tasks)
		_ = s.repos.Orch.UpdateRevisionValidation(txCtx, rev.ID, orchestrator.MustJSON(res.Validation))
		if res.Validation.Status == orchestrator.ValidationFailed {
			return validationFailed(res.Validation)
		}
		if orchestrator.RiskRank(view.RiskLevel) >= orchestrator.RiskRank(orchestrator.RiskHigh) && !in.AcknowledgeRisks {
			return errRiskAck
		}
		if view.RiskLevel == orchestrator.RiskDestructive {
			var expected []string
			if res.Confirmation != nil {
				expected = res.Confirmation.Expected
			}
			if strings.TrimSpace(in.Confirmation) == "" {
				return errConfirmRequired
			}
			if !orchestrator.ConfirmationMatches(in.Confirmation, expected) {
				return errConfirmMismatch
			}
		}

		now := time.Now()
		if err := s.repos.Orch.SupersedeApprovals(txCtx, plan.ID, now); err != nil {
			return err
		}
		version := plan.Version
		approval := &models.AIApproval{
			PlanID: plan.ID, UserID: userID, Version: version, PlanVersion: &version,
			SpecHash: view.SpecHash, RiskLevel: view.RiskLevel, Status: models.ApprovalApproved,
			ExpiresAt: now.Add(orchestrator.ApprovalTTLFor(view.RiskLevel)), ApprovedAt: &now,
			AcknowledgedRisks: in.AcknowledgeRisks, CreatedAt: now,
		}
		if err := s.repos.Orch.CreateApproval(txCtx, approval); err != nil {
			return err
		}
		result = &ApproveResult{
			Approval: approvalView(approval), PlanID: plan.ID, Status: models.ApprovalApproved,
			ProgressSteps: []models.UIProgressStep{}, ResultDetails: map[string]any{},
			Message: "Plan approved. Execute it with the approval id before the approval expires.",
		}
		if !execute {
			// Approved but not executing: still editable (a PATCH supersedes
			// the approval and returns the plan to awaiting_approval).
			return s.repos.Orch.UpdatePlan(txCtx, plan.ID, map[string]any{"status": models.PlanStatusApproved})
		}
		exec, err := s.startExecution(txCtx, plan, view, approval, clientIP)
		if err != nil {
			return err
		}
		approval.Status, approval.Consumed = models.ApprovalConsumed, true
		result.Approval = approvalView(approval)
		result.Status = "provisioning"
		result.Message = fmt.Sprintf("Plan approved. Execution %s started with %d task(s).", exec.ID, len(view.Tasks))
		result.ProgressSteps = progressSteps(view.Tasks)
		id := exec.ID
		execID = &id
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.svc.Audit.Log(ctx, &userID, "ai.plan.approve", "ai_plan", &planID, "success", clientIP, map[string]any{
		"version": result.Approval.Version, "spec_hash": result.Approval.SpecHash,
		"risk_level": result.Approval.RiskLevel, "execute": execute,
	})
	if execID != nil {
		s.auditExecutionStart(ctx, userID, planID, *execID, clientIP)
		s.engine.Nudge()
		view, err := s.GetExecution(ctx, *execID, userID)
		if err == nil {
			result.Execution = view
		}
	}
	return result, nil
}

// checkApprovable rejects approvals of plans that are executing or finished.
func (s *OrchestratorUsecase) checkApprovable(ctx context.Context, plan *models.AIResourcePlan) error {
	if s.editable(ctx, plan) {
		return nil
	}
	if latest, err := s.repos.Orch.LatestExecution(ctx, plan.ID); err == nil && !isTerminalExecStatus(latest.Status) {
		return errExecutionActive
	}
	return newError(KindConflict, "PLAN_NOT_EDITABLE", fmt.Sprintf("the plan is %s and can no longer be approved or executed", plan.Status))
}

func isTerminalExecStatus(status string) bool {
	switch status {
	case models.ExecCompleted, models.ExecFailed, models.ExecPartiallyCompleted, models.ExecCancelled:
		return true
	}
	return false
}

func progressSteps(tasks []orchestrator.TaskSpec) []models.UIProgressStep {
	steps := make([]models.UIProgressStep, 0, len(tasks))
	for _, t := range tasks {
		steps = append(steps, models.UIProgressStep{StepID: t.Key, Title: t.Title, Status: "pending"})
	}
	return steps
}

// startExecution consumes the approval and creates the execution and its
// tasks (caller holds the plan lock in txCtx).
func (s *OrchestratorUsecase) startExecution(txCtx context.Context, plan *models.AIResourcePlan, view *orchestrator.Revision, approval *models.AIApproval, clientIP string) (*models.AIExecution, error) {
	if err := s.repos.Orch.LockUserExecutions(txCtx, plan.UserID); err != nil {
		return nil, err
	}
	active, err := s.repos.Orch.CountActiveExecutions(txCtx, plan.UserID)
	if err != nil {
		return nil, err
	}
	if active >= MaxActiveExecutions {
		return nil, errTooMany
	}
	approvalID := approval.ID
	exec := &models.AIExecution{
		PlanID: plan.ID, PlanVersion: view.Version, ApprovalID: &approvalID,
		ConversationID: plan.ConversationID, UserID: plan.UserID,
		Status: models.ExecQueued, ClientIP: clientIP, RecoveryOptions: "[]",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	if err := s.repos.Orch.CreateExecution(txCtx, exec); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, errExecutionActive
		}
		return nil, err
	}
	ok, err := s.repos.Orch.ConsumeApproval(txCtx, approval.ID, exec.ID, time.Now())
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, approvalStateError(approval)
	}
	tasks := make([]models.AIExecutionTask, 0, len(view.Tasks))
	for i, t := range view.Tasks {
		def, _ := s.reg.Def(t.Tool)
		status := models.TaskQueued
		if len(t.DependsOn) > 0 {
			status = models.TaskBlocked
		}
		deps := t.DependsOn
		if deps == nil {
			deps = []string{}
		}
		input := t.Input
		if input == nil {
			input = map[string]any{}
		}
		tasks = append(tasks, models.AIExecutionTask{
			ExecutionID: exec.ID, Position: i, Key: t.Key, Tool: t.Tool, Title: t.Title, Risk: t.Risk,
			DependsOn: orchestrator.MustJSON(deps), Input: orchestrator.MustJSON(input),
			Status: status, Attempt: 1, MaxAttempts: def.MaxAttempts, TimeoutSeconds: def.TimeoutSeconds,
			IdempotencyKey: orchestrator.IdempotencyKey(exec.ID, t.Key, 1),
			CreatedAt:      exec.CreatedAt.Add(time.Duration(i) * time.Microsecond), UpdatedAt: time.Now(),
		})
	}
	if err := s.repos.Orch.CreateTasks(txCtx, tasks); err != nil {
		return nil, err
	}
	if err := s.repos.Orch.UpdatePlan(txCtx, plan.ID, map[string]any{"status": models.PlanStatusApproved}); err != nil {
		return nil, err
	}
	_ = s.repos.Agent.SetConversationState(txCtx, plan.ConversationID, models.StateAIProvisioning)
	if state, err := s.repos.Agent.FindState(txCtx, plan.ConversationID); err == nil {
		state.State = models.StateAIProvisioning
		_ = s.repos.Agent.SaveState(txCtx, state)
	}
	s.engine.Event(txCtx, exec.ID, "", orchestrator.EventExecutionCreated, orchestrator.SeverityInfo,
		fmt.Sprintf("Execution of plan version %d created with %d task(s)", view.Version, len(tasks)),
		map[string]any{"plan_id": plan.ID.String(), "plan_version": view.Version})
	s.engine.Event(txCtx, exec.ID, "", orchestrator.EventApprovalRecorded, orchestrator.SeverityInfo,
		fmt.Sprintf("Approval of version %d (%s risk) consumed", view.Version, view.RiskLevel),
		map[string]any{"approval_id": approval.ID.String(), "spec_hash": view.SpecHash, "risk_level": view.RiskLevel})
	for _, t := range tasks {
		if t.Status == models.TaskQueued {
			s.engine.Event(txCtx, exec.ID, t.Key, orchestrator.EventTaskQueued, orchestrator.SeverityInfo, t.Title+" queued", nil)
		}
	}
	return exec, nil
}

// auditExecutionStart is written after the transaction that started the
// execution committed (an audit failure must never abort it).
func (s *OrchestratorUsecase) auditExecutionStart(ctx context.Context, userID, planID, execID uuid.UUID, clientIP string) {
	s.svc.Audit.Log(ctx, &userID, "ai.execution.start", "ai_execution", &execID, "success", clientIP, map[string]any{"plan_id": planID})
}

func approvalStateError(a *models.AIApproval) *Error {
	switch {
	case a.Status == models.ApprovalConsumed || a.Consumed:
		return newError(KindConflict, "APPROVAL_CONSUMED", "the approval was already used by an execution")
	case a.Status == models.ApprovalSuperseded:
		return newError(KindConflict, "APPROVAL_SUPERSEDED", "the plan changed after this approval; approve the current version")
	case a.Status == models.ApprovalExpired || (a.Status == models.ApprovalApproved && time.Now().After(a.ExpiresAt)):
		return newError(KindConflict, "APPROVAL_EXPIRED", "the approval expired; approve the plan again")
	}
	return newError(KindConflict, "APPROVAL_REQUIRED", "the plan needs a valid approval")
}

// ExecutePlan starts an execution with an approval recorded earlier
// (approve with execute=false). The approval must be unused, unexpired and
// bound to the current revision.
func (s *OrchestratorUsecase) ExecutePlan(ctx context.Context, planID, userID uuid.UUID, isAdmin bool, approvalID uuid.UUID, clientIP string) (*ExecutionView, error) {
	ctx = context.WithoutCancel(ctx)
	var execID uuid.UUID
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		plan, err := s.repos.Orch.LockPlan(txCtx, planID, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return errPlanNotFound
			}
			return err
		}
		approval, err := s.repos.Orch.FindApproval(txCtx, approvalID, plan.ID)
		if err != nil || approval.UserID != userID {
			return newError(KindConflict, "APPROVAL_REQUIRED", "no approval with this id for this plan")
		}
		if approval.Status != models.ApprovalApproved || approval.Consumed || time.Now().After(approval.ExpiresAt) {
			return approvalStateError(approval)
		}
		if err := s.checkApprovable(txCtx, plan); err != nil {
			return err
		}
		_, view, err := s.currentRevision(txCtx, plan)
		if err != nil {
			return err
		}
		if approval.PlanVersion == nil || *approval.PlanVersion != plan.Version || approval.SpecHash != view.SpecHash || !verifyRevisionHash(view) {
			return newError(KindConflict, "APPROVAL_SUPERSEDED", "the plan changed after this approval; approve the current version")
		}
		if s.requiresAdmin(view.Tasks) && !(isAdmin && s.userIsAdmin(txCtx, userID)) {
			return errAdminRequired
		}
		res := s.policy.Validate(txCtx, userID, isAdmin, view.Specification, view.Tasks)
		if res.Validation.Status == orchestrator.ValidationFailed {
			return validationFailed(res.Validation)
		}
		exec, err := s.startExecution(txCtx, plan, view, approval, clientIP)
		if err != nil {
			return err
		}
		execID = exec.ID
		return nil
	})
	if err != nil {
		return nil, err
	}
	s.auditExecutionStart(ctx, userID, planID, execID, clientIP)
	s.engine.Nudge()
	return s.GetExecution(ctx, execID, userID)
}

// RejectPlan rejects a plan that has not started executing.
func (s *OrchestratorUsecase) RejectPlan(ctx context.Context, planID, userID uuid.UUID, reason, clientIP string) (*PlanDetail, error) {
	ctx = context.WithoutCancel(ctx)
	var convID uuid.UUID
	err := s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		plan, err := s.repos.Orch.LockPlan(txCtx, planID, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return errPlanNotFound
			}
			return err
		}
		if err := s.checkApprovable(txCtx, plan); err != nil {
			return err
		}
		convID = plan.ConversationID
		if err := s.repos.Orch.RejectApprovals(txCtx, plan.ID); err != nil {
			return err
		}
		return s.repos.Orch.UpdatePlan(txCtx, plan.ID, map[string]any{"status": models.PlanStatusRejected})
	})
	if err != nil {
		return nil, err
	}
	msg := "🛑 **Plan rejected.** Nothing was provisioned or changed."
	if strings.TrimSpace(reason) != "" {
		msg += "\n\nReason: " + truncate(strings.TrimSpace(reason), 500)
	}
	_ = s.repos.Agent.CreateMessage(ctx, &models.AIMessage{ConversationID: convID, UserID: userID, Role: "assistant", Content: msg, CreatedAt: time.Now()})
	_ = s.repos.Agent.SetConversationState(ctx, convID, models.StateCancelled)
	s.svc.Audit.Log(ctx, &userID, "ai.plan.reject", "ai_plan", &planID, "success", clientIP, nil)
	return s.GetPlan(ctx, planID, userID)
}

// CancelPlan cancels a plan and its active execution, if any.
func (s *OrchestratorUsecase) CancelPlan(ctx context.Context, planID, userID uuid.UUID, clientIP string) error {
	ctx = context.WithoutCancel(ctx)
	plan, err := s.findPlan(ctx, planID, userID)
	if err != nil {
		return err
	}
	if latest, err := s.repos.Orch.LatestExecution(ctx, plan.ID); err == nil && !isTerminalExecStatus(latest.Status) {
		if err := s.engine.Cancel(ctx, latest.ID); err != nil && !errors.Is(err, orchestrator.ErrNotActive) {
			return err
		}
	}
	now := time.Now()
	_ = s.repos.Agent.CancelApprovals(ctx, plan.ID, userID, &now)
	if !isTerminalPlan(plan.Status) || plan.Status == models.PlanStatusCancelled {
		_ = s.repos.Orch.UpdatePlan(ctx, plan.ID, map[string]any{"status": models.PlanStatusCancelled})
	}
	if state, err := s.repos.Agent.FindState(ctx, plan.ConversationID); err == nil {
		state.State = models.StateCancelled
		_ = s.repos.Agent.SaveState(ctx, state)
	}
	_ = s.repos.Agent.SetConversationState(ctx, plan.ConversationID, models.StateCancelled)
	_ = s.repos.Agent.CreateMessage(ctx, &models.AIMessage{
		ConversationID: plan.ConversationID,
		UserID:         userID,
		Role:           "assistant",
		Content:        "🛑 **Plan Cancelled**\n\nThe proposed infrastructure plan was cancelled. Tasks that had not started will not run; resources already created by a running execution are listed in its report. Feel free to describe a new configuration anytime!",
		CreatedAt:      time.Now(),
	})
	s.svc.Audit.Log(ctx, &userID, "ai.plan.cancel", "ai_plan", &planID, "success", clientIP, nil)
	return nil
}

// Executions

func rawJSON(s *string) json.RawMessage {
	if s == nil || *s == "" {
		return json.RawMessage("null")
	}
	return json.RawMessage(*s)
}

func baseView(e *models.AIExecution) executionBase {
	b := executionBase{
		ID: e.ID, PlanID: e.PlanID, PlanVersion: e.PlanVersion, ConversationID: e.ConversationID,
		Status: e.Status, CancelRequested: e.CancelRequested, CreatedAt: e.CreatedAt.UTC(),
		StartedAt: e.StartedAt, FinishedAt: e.FinishedAt,
	}
	if b.StartedAt != nil {
		t := b.StartedAt.UTC()
		b.StartedAt = &t
	}
	if b.FinishedAt != nil {
		t := b.FinishedAt.UTC()
		b.FinishedAt = &t
	}
	return b
}

func recoveryView(e *models.AIExecution) []orchestrator.RecoveryOption {
	out := []orchestrator.RecoveryOption{}
	_ = json.Unmarshal([]byte(e.RecoveryOptions), &out)
	if out == nil {
		out = []orchestrator.RecoveryOption{}
	}
	return out
}

func taskView(t models.AIExecutionTask) TaskView {
	deps := []string{}
	_ = json.Unmarshal([]byte(t.DependsOn), &deps)
	v := TaskView{
		ID: t.ID, Key: t.Key, Tool: t.Tool, Title: t.Title, Risk: t.Risk, DependsOn: deps,
		Status: t.Status, Attempt: t.Attempt, MaxAttempts: t.MaxAttempts,
		NextAttemptAt: utcPtr(t.NextAttemptAt), StartedAt: utcPtr(t.StartedAt), FinishedAt: utcPtr(t.FinishedAt),
		Output: rawJSON(t.Output), Error: rawJSON(t.Error), Verification: rawJSON(t.Verification),
	}
	if t.Status != models.TaskFailed && t.Status != models.TaskRetrying {
		// A retrying task shows the error of its previous attempt; other
		// statuses have none.
		v.Error = json.RawMessage("null")
	}
	return v
}

func utcPtr(t *time.Time) *time.Time {
	if t == nil {
		return nil
	}
	u := t.UTC()
	return &u
}

func (s *OrchestratorUsecase) findExecution(ctx context.Context, execID, userID uuid.UUID) (*models.AIExecution, error) {
	e, err := s.repos.Orch.FindExecution(ctx, execID, &userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errExecutionNotFound
		}
		return nil, err
	}
	return e, nil
}

// GetExecution returns an execution with its tasks.
func (s *OrchestratorUsecase) GetExecution(ctx context.Context, execID, userID uuid.UUID) (*ExecutionView, error) {
	e, err := s.findExecution(ctx, execID, userID)
	if err != nil {
		return nil, err
	}
	tasks, err := s.repos.Orch.ListTasks(ctx, e.ID)
	if err != nil {
		return nil, err
	}
	views := make([]TaskView, 0, len(tasks))
	for _, t := range tasks {
		views = append(views, taskView(t))
	}
	return &ExecutionView{executionBase: baseView(e), Tasks: views, Report: rawJSON(e.Report), RecoveryOptions: s.recoveryOptions(ctx, e)}, nil
}

// ExecutionListFilter filters ListExecutions.
type ExecutionListFilter struct {
	ConversationID *uuid.UUID
	PlanID         *uuid.UUID
	Status         string
}

// ListExecutions returns the user's executions, newest first.
func (s *OrchestratorUsecase) ListExecutions(ctx context.Context, userID uuid.UUID, f ExecutionListFilter) ([]ExecutionSummaryView, error) {
	execs, err := s.repos.Orch.ListExecutions(ctx, userID, repository.ExecutionFilter{
		ConversationID: f.ConversationID, PlanID: f.PlanID, Status: f.Status,
	})
	if err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(execs))
	for _, e := range execs {
		ids = append(ids, e.ID)
	}
	counts, err := s.repos.Orch.TaskCounts(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]ExecutionSummaryView, 0, len(execs))
	for i := range execs {
		e := &execs[i]
		c := counts[e.ID]
		if c == nil {
			c = map[string]int{}
		}
		out = append(out, ExecutionSummaryView{executionBase: baseView(e), Report: rawJSON(e.Report), RecoveryOptions: s.recoveryOptions(ctx, e), TaskCounts: c})
	}
	return out, nil
}

// ListEvents returns the execution's events after seq after.
func (s *OrchestratorUsecase) ListEvents(ctx context.Context, execID, userID uuid.UUID, after int64, limit int) (*EventsPage, error) {
	if _, err := s.findExecution(ctx, execID, userID); err != nil {
		return nil, err
	}
	return s.EventsSince(ctx, execID, after, limit)
}

// ExecutionFinished reports whether an execution has finished and every
// event up to seq after has been delivered (an SSE stream can end).
func (s *OrchestratorUsecase) ExecutionFinished(ctx context.Context, execID, userID uuid.UUID, after int64) bool {
	e, err := s.repos.Orch.FindExecution(ctx, execID, &userID)
	if err != nil {
		return true
	}
	return isTerminalExecStatus(e.Status) && e.LastEventSeq <= after
}

// EventsSince is ListEvents without the ownership check (SSE streams check
// ownership once, when they start).
func (s *OrchestratorUsecase) EventsSince(ctx context.Context, execID uuid.UUID, after int64, limit int) (*EventsPage, error) {
	events, err := s.repos.Orch.ListEvents(ctx, execID, after, limit)
	if err != nil {
		return nil, err
	}
	page := &EventsPage{Events: make([]EventView, 0, len(events)), NextAfter: after}
	for _, ev := range events {
		md := ev.Metadata
		if md == "" {
			md = "{}"
		}
		page.Events = append(page.Events, EventView{
			Seq: ev.Seq, ID: ev.ID, Type: ev.Type, Severity: ev.Severity, Message: ev.Message,
			TaskKey: ev.TaskKey, Metadata: json.RawMessage(md), CreatedAt: ev.CreatedAt.UTC(),
		})
		page.NextAfter = ev.Seq
	}
	return page, nil
}

// CancelExecution requests cancellation of an active execution.
func (s *OrchestratorUsecase) CancelExecution(ctx context.Context, execID, userID uuid.UUID, clientIP string) (*ExecutionView, error) {
	ctx = context.WithoutCancel(ctx)
	if _, err := s.findExecution(ctx, execID, userID); err != nil {
		return nil, err
	}
	if err := s.engine.Cancel(ctx, execID); err != nil {
		if errors.Is(err, orchestrator.ErrNotActive) {
			return nil, errNotActive
		}
		return nil, err
	}
	s.svc.Audit.Log(ctx, &userID, "ai.execution.cancel", "ai_execution", &execID, "success", clientIP, nil)
	return s.GetExecution(ctx, execID, userID)
}

// retry re-queues failed tasks; a finished execution becomes active again
// (subject to the per-user limit).
func (s *OrchestratorUsecase) retry(ctx context.Context, execID, userID uuid.UUID, taskID *uuid.UUID, clientIP string) (*ExecutionView, error) {
	ctx = context.WithoutCancel(ctx)
	exec, err := s.findExecution(ctx, execID, userID)
	if err != nil {
		return nil, err
	}
	if taskID != nil {
		if _, err := s.repos.Orch.FindTask(ctx, execID, *taskID); err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, errTaskNotFound
			}
			return nil, err
		}
	}
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.engine.Retry(txCtx, execID, orchestrator.RetryInput{
			TaskID: taskID,
			Activate: func(txCtx context.Context, e *models.AIExecution) error {
				if err := s.repos.Orch.LockUserExecutions(txCtx, exec.UserID); err != nil {
					return err
				}
				n, err := s.repos.Orch.CountActiveExecutions(txCtx, exec.UserID)
				if err != nil {
					return err
				}
				if n >= MaxActiveExecutions {
					return errTooMany
				}
				return nil
			},
		})
	})
	switch {
	case errors.Is(err, orchestrator.ErrTaskNotRetryable):
		return nil, errTaskNotRetryable
	case errors.Is(err, orchestrator.ErrNotActive):
		return nil, errTaskNotRetryable
	case err != nil:
		return nil, err
	}
	s.svc.Audit.Log(ctx, &userID, "ai.execution.retry", "ai_execution", &execID, "success", clientIP, map[string]any{"task_id": taskID})
	return s.GetExecution(ctx, execID, userID)
}

// RetryTask re-queues one failed task and its skipped descendants.
func (s *OrchestratorUsecase) RetryTask(ctx context.Context, execID, taskID, userID uuid.UUID, clientIP string) (*ExecutionView, error) {
	return s.retry(ctx, execID, userID, &taskID, clientIP)
}

// Recover runs a recovery action on a failed or partially completed
// execution: retry_failed, keep, or delete_created (which only creates a new
// destructive plan; nothing is deleted without its own approval).
func (s *OrchestratorUsecase) Recover(ctx context.Context, execID, userID uuid.UUID, isAdmin bool, action, clientIP string) (*RecoveryResult, error) {
	ctx = context.WithoutCancel(ctx)
	exec, err := s.findExecution(ctx, execID, userID)
	if err != nil {
		return nil, err
	}
	switch action {
	case orchestrator.RecoveryRetryFailed:
		view, err := s.retry(ctx, execID, userID, nil, clientIP)
		if err != nil {
			return nil, err
		}
		return &RecoveryResult{Execution: view}, nil
	case orchestrator.RecoveryKeep:
		if exec.Status != models.ExecPartiallyCompleted {
			return nil, newError(KindConflict, "RECOVERY_NOT_AVAILABLE", "keep is only available on a partially completed execution")
		}
		now := time.Now()
		if err := s.repos.Orch.UpdateExecution(ctx, execID, map[string]any{"kept_at": now, "recovery_options": "[]"}); err != nil {
			return nil, err
		}
		s.svc.Audit.Log(ctx, &userID, "ai.execution.keep", "ai_execution", &execID, "success", clientIP, nil)
		view, err := s.GetExecution(ctx, execID, userID)
		if err != nil {
			return nil, err
		}
		return &RecoveryResult{Execution: view}, nil
	case orchestrator.RecoveryDeleteCreated:
		if exec.Status != models.ExecPartiallyCompleted && exec.Status != models.ExecFailed {
			return nil, newError(KindConflict, "RECOVERY_NOT_AVAILABLE", "delete_created is only available on a failed or partially completed execution")
		}
		planID, err := s.deleteCreatedPlan(ctx, exec, isAdmin)
		if err != nil {
			return nil, err
		}
		s.svc.Audit.Log(ctx, &userID, "ai.execution.delete_created", "ai_execution", &execID, "success", clientIP, map[string]any{"plan_id": planID})
		return &RecoveryResult{PlanID: &planID}, nil
	}
	return nil, ValidationError("action", "action must be retry_failed, keep or delete_created")
}

// deletableCreated lists the resources the execution created ("created"
// relations without a "deleted" one) that still exist and that a plan can
// delete (VPS, bucket). skipped names created resources without a delete
// tool in Phase 1.
func (s *OrchestratorUsecase) deletableCreated(ctx context.Context, exec *models.AIExecution) ([]orchestrator.ResourceSpec, []string, error) {
	rels, err := s.repos.Orch.ListRelationships(ctx, exec.ID)
	if err != nil {
		return nil, nil, err
	}
	deleted := map[uuid.UUID]bool{}
	for _, r := range rels {
		if r.Relation == models.RelationDeleted {
			deleted[r.ResourceID] = true
		}
	}
	var resources []orchestrator.ResourceSpec
	var skipped []string
	seen := map[uuid.UUID]bool{}
	for _, r := range rels {
		if r.Relation != models.RelationCreated || deleted[r.ResourceID] || seen[r.ResourceID] {
			continue
		}
		seen[r.ResourceID] = true
		switch r.ResourceType {
		case orchestrator.TypeVPS:
			inst, err := s.repos.Instances.FindByID(ctx, r.ResourceID, &exec.UserID)
			if err != nil {
				continue // already gone
			}
			resources = append(resources, orchestrator.ResourceSpec{Key: inst.Name, Type: orchestrator.TypeVPS, Action: orchestrator.ActionDelete,
				Config: map[string]any{"instance_id": inst.ID.String(), "name": inst.Name}})
		case orchestrator.TypeBucket:
			b, err := s.repos.Storage.FindBucket(ctx, r.ResourceID, exec.UserID)
			if err != nil {
				continue
			}
			resources = append(resources, orchestrator.ResourceSpec{Key: b.Name, Type: orchestrator.TypeBucket, Action: orchestrator.ActionDelete,
				Config: map[string]any{"bucket_id": b.ID.String(), "name": b.Name}})
		default:
			skipped = append(skipped, fmt.Sprintf("%s %s", r.ResourceType, r.ResourceName))
		}
	}
	return resources, skipped, nil
}

// CanDeleteCreated implements orchestrator.Hooks: delete_created is only
// offered when it would produce a plan.
func (s *OrchestratorUsecase) CanDeleteCreated(ctx context.Context, exec *models.AIExecution) bool {
	resources, _, err := s.deletableCreated(ctx, exec)
	return err == nil && len(resources) > 0
}

// recoveryOptions returns the stored recovery options of an execution,
// without delete_created once nothing it created is left to delete (a
// delete plan ran, or the resources were removed from their pages).
func (s *OrchestratorUsecase) recoveryOptions(ctx context.Context, e *models.AIExecution) []orchestrator.RecoveryOption {
	opts := recoveryView(e)
	for _, o := range opts {
		if o.Action == orchestrator.RecoveryDeleteCreated {
			if !s.CanDeleteCreated(ctx, e) {
				return orchestrator.WithoutDeleteCreated(opts)
			}
			break
		}
	}
	return opts
}

// deleteCreatedPlan proposes a destructive plan removing what the execution
// created and that still exists.
func (s *OrchestratorUsecase) deleteCreatedPlan(ctx context.Context, exec *models.AIExecution, isAdmin bool) (uuid.UUID, error) {
	resources, skipped, err := s.deletableCreated(ctx, exec)
	if err != nil {
		return uuid.Nil, err
	}
	if len(resources) == 0 {
		return uuid.Nil, newError(KindConflict, "RECOVERY_NOT_AVAILABLE", "this execution created no resources that can be deleted by a plan")
	}
	description := "Delete the resources created by execution " + exec.ID.String()
	if len(skipped) > 0 {
		description += ". Not included (no delete tool in Phase 1; remove them from their pages): " + strings.Join(skipped, ", ")
	}
	plan, _, err := s.CreatePlan(ctx, PlanDraft{
		ConversationID: exec.ConversationID, UserID: exec.UserID, IsAdmin: isAdmin,
		Intent: "delete_created", Title: "Delete resources created by a failed execution",
		Description: description,
		Spec:        orchestrator.Specification{Intent: "delete_created", Resources: resources},
	})
	if err != nil {
		return uuid.Nil, err
	}
	_ = s.repos.Agent.CreateMessage(ctx, &models.AIMessage{
		ConversationID: exec.ConversationID, UserID: exec.UserID, Role: "assistant",
		Content:   fmt.Sprintf("I prepared a **destructive plan** to delete %d resource(s) created by the failed execution. Review it, type the resource names to confirm, and approve it to proceed. Nothing has been deleted yet.", len(resources)),
		CreatedAt: time.Now(),
	})
	return plan.ID, nil
}

// MCP and other planners

// MCPConversationTitle is the conversation MCP-created plans are filed in.
const MCPConversationTitle = "MCP requests"

// AssistantConversationTitle is the conversation plans proposed by the
// OpenRouter AI assistant (POST /ai-assistant/execute) are filed in.
const AssistantConversationTitle = "AI assistant requests"

// PlanURL is the AI workspace deep link to a plan, relative to the
// control-plane origin (APP_BASE_URL). It is the format of
// frontend/src/lib/aiRoutes.ts: /dashboard/ai/conversations/<conversationId>?plan=<planId>,
// or /dashboard/ai?plan=<planId> when the plan has no conversation (the
// workspace then opens the plan's conversation itself).
func PlanURL(conversationID, planID uuid.UUID) string {
	if conversationID == uuid.Nil {
		return "/dashboard/ai?plan=" + planID.String()
	}
	return "/dashboard/ai/conversations/" + conversationID.String() + "?plan=" + planID.String()
}

// ToolCallResult is the outcome of an MCP tools/call on a registry tool:
// Data for read tools, the plan (awaiting approval, or a draft the policy
// engine rejected) for write tools.
type ToolCallResult struct {
	Risk           string
	Data           any
	PlanID         uuid.UUID
	Version        int
	PlanStatus     string
	ConversationID uuid.UUID
	// URL is the workspace deep link to the plan (PlanURL).
	URL string
}

// planSource describes who proposed a plan built from a single registry
// tool call: where the plan is filed and how it is labelled and audited.
type planSource struct {
	conversationTitle string
	titlePrefix       string
	description       func(toolID string) string
	message           func(def orchestrator.ToolDef, risk string, planID uuid.UUID, version int) string
	auditAction       string
}

var mcpPlanSource = planSource{
	conversationTitle: MCPConversationTitle,
	titlePrefix:       "MCP: ",
	description:       func(toolID string) string { return "Requested by an MCP client through " + toolID },
	message: func(def orchestrator.ToolDef, risk string, planID uuid.UUID, version int) string {
		return fmt.Sprintf("An MCP client requested **%s** (%s risk). I prepared plan `%s` (version %d). Nothing runs until you review and approve it here.",
			def.Title, risk, planID, version)
	},
	auditAction: "ai.plan.create_from_mcp",
}

var assistantPlanSource = planSource{
	conversationTitle: AssistantConversationTitle,
	titlePrefix:       "AI assistant: ",
	description:       func(toolID string) string { return "Proposed by the AI assistant through " + toolID },
	message: func(def orchestrator.ToolDef, risk string, planID uuid.UUID, version int) string {
		return fmt.Sprintf("The AI assistant proposed **%s** (%s risk). I prepared plan `%s` (version %d). Nothing is created until you review and approve it here.",
			def.Title, risk, planID, version)
	},
	auditAction: "ai.plan.create_from_assistant",
}

// CallTool runs an MCP tools/call on a registry tool. Read tools run now;
// write, high and destructive tools never run: they become a plan the user
// must approve in the UI.
func (s *OrchestratorUsecase) CallTool(ctx context.Context, userID uuid.UUID, isAdmin bool, toolID string, args map[string]any, clientIP string) (*ToolCallResult, error) {
	def, ok := s.reg.Def(toolID)
	if !ok {
		return nil, fmt.Errorf("tool %q is not in the registry", toolID)
	}
	if args == nil {
		args = map[string]any{}
	}
	generic, err := genericMap(args)
	if err != nil {
		return nil, err
	}
	if !orchestrator.IsWrite(def.Risk) {
		if te := s.policy.CheckTask(ctx, userID, toolID, generic); te != nil {
			return nil, te
		}
		runner, _ := s.reg.Runner(toolID)
		runCtx, cancel := context.WithTimeout(ctx, def.Timeout())
		defer cancel()
		tc := orchestrator.NewTaskContext(orchestrator.TaskContext{
			TaskKey: toolID, Tool: toolID, UserID: userID, ClientIP: clientIP, Attempt: 1, Input: generic,
			DependencyOutputs: map[string]map[string]any{},
		}, nil, nil)
		out, err := runner.Run(runCtx, tc)
		if err != nil {
			return nil, err
		}
		return &ToolCallResult{Risk: def.Risk, Data: out}, nil
	}
	return s.planFromTool(ctx, userID, isAdmin, def, generic, clientIP, mcpPlanSource, "")
}

// ProposeFromAssistant turns an AI assistant action card (a write registry
// tool and its arguments, proposed by the model and sent by the client)
// into a plan awaiting approval, through the same path as MCP write tools.
// It never runs the tool: only POST /ai/plans/:id/approve authorizes
// execution. messageID (optional) is the assistant message holding the
// card; FindAssistantPlan finds the plan by it.
func (s *OrchestratorUsecase) ProposeFromAssistant(ctx context.Context, userID uuid.UUID, isAdmin bool, toolID string, args map[string]any, messageID uuid.UUID, clientIP string) (*ToolCallResult, error) {
	def, ok := s.reg.Def(toolID)
	if !ok || !orchestrator.IsWrite(def.Risk) {
		return nil, fmt.Errorf("tool %q cannot be planned", toolID)
	}
	if args == nil {
		args = map[string]any{}
	}
	generic, err := genericMap(args)
	if err != nil {
		return nil, err
	}
	suffix := ""
	if messageID != uuid.Nil {
		suffix = assistantMessageMarker(messageID)
	}
	return s.planFromTool(ctx, userID, isAdmin, def, generic, clientIP, assistantPlanSource, suffix)
}

// FindAssistantPlan returns the plan the AI assistant created for one of
// the user's assistant messages (its description names the message), so a
// second click on the same action card opens that plan instead of creating
// another one.
func (s *OrchestratorUsecase) FindAssistantPlan(ctx context.Context, userID, messageID uuid.UUID) (*ToolCallResult, error) {
	conv, err := s.repos.Agent.FindConversationByTitle(ctx, userID, AssistantConversationTitle)
	if err != nil {
		return nil, err
	}
	plans, err := s.repos.Agent.ListPlans(ctx, conv.ID, userID)
	if err != nil {
		return nil, err
	}
	marker := assistantMessageMarker(messageID)
	for _, p := range plans {
		if !strings.Contains(p.Description, marker) {
			continue
		}
		detail, err := s.GetPlan(ctx, p.ID, userID)
		if err != nil {
			return nil, err
		}
		res := &ToolCallResult{PlanID: p.ID, Version: detail.CurrentVersion, PlanStatus: detail.Status,
			ConversationID: conv.ID, URL: PlanURL(conv.ID, p.ID)}
		if detail.Revision != nil {
			res.Risk = detail.Revision.RiskLevel
		}
		return res, nil
	}
	return nil, repository.ErrNotFound
}

func assistantMessageMarker(messageID uuid.UUID) string {
	return "(assistant message " + messageID.String() + ")"
}

// planFromTool stores a single write tool call as a plan (revision 1) in the
// source's conversation. The plan awaits approval, or stays a draft listing
// its validation errors when the policy engine rejects the request.
// descriptionSuffix is appended to the plan description.
func (s *OrchestratorUsecase) planFromTool(ctx context.Context, userID uuid.UUID, isAdmin bool, def orchestrator.ToolDef, generic map[string]any, clientIP string, src planSource, descriptionSuffix string) (*ToolCallResult, error) {
	ctx = context.WithoutCancel(ctx)
	spec, ok := s.reg.SpecFromTool(def.ID, generic)
	if !ok {
		return nil, fmt.Errorf("tool %q cannot be planned", def.ID)
	}
	convID, err := s.requestConversation(ctx, userID, src.conversationTitle)
	if err != nil {
		return nil, err
	}
	description := src.description(def.ID)
	if descriptionSuffix != "" {
		description += " " + descriptionSuffix
	}
	plan, rev, err := s.CreatePlan(ctx, PlanDraft{
		ConversationID: convID, UserID: userID, IsAdmin: isAdmin,
		Intent: spec.Intent, Title: src.titlePrefix + def.Title, Description: description,
		Spec: spec,
	})
	if err != nil {
		return nil, err
	}
	_ = s.repos.Agent.CreateMessage(ctx, &models.AIMessage{
		ConversationID: convID, UserID: userID, Role: "assistant",
		Content:   src.message(def, rev.RiskLevel, plan.ID, rev.Version),
		CreatedAt: time.Now(),
	})
	s.svc.Audit.Log(ctx, &userID, src.auditAction, "ai_plan", &plan.ID, "success", clientIP, map[string]any{"tool": def.ID})
	// awaiting_approval, or draft when the policy engine rejected the request
	// (the plan then lists its validation errors).
	return &ToolCallResult{Risk: rev.RiskLevel, PlanID: plan.ID, Version: rev.Version, PlanStatus: plan.Status,
		ConversationID: convID, URL: PlanURL(convID, plan.ID)}, nil
}

func genericMap(m map[string]any) (map[string]any, error) {
	b, err := orchestrator.Canonical(m)
	if err != nil {
		return nil, err
	}
	var out map[string]any
	if err := orchestrator.DecodeJSON(string(b), &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]any{}
	}
	return out, nil
}

// requestConversation returns (creating it once) the user's conversation
// with the given title, in which plans from one planner (MCP, the AI
// assistant) are filed.
func (s *OrchestratorUsecase) requestConversation(ctx context.Context, userID uuid.UUID, title string) (uuid.UUID, error) {
	if conv, err := s.repos.Agent.FindConversationByTitle(ctx, userID, title); err == nil {
		return conv.ID, nil
	}
	conv := models.AIConversation{UserID: userID, Title: title, State: models.StateAwaitingConfirmation, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	if err := s.repos.Agent.CreateConversation(ctx, &conv); err != nil {
		return uuid.Nil, err
	}
	_ = s.repos.Agent.CreateState(ctx, &models.AIConversationState{
		ConversationID: conv.ID, State: models.StateAwaitingConfirmation, PendingQuestions: "[]", GatheredRequirements: "{}", UpdatedAt: time.Now(),
	})
	return conv.ID, nil
}

// BackfillRevisions writes revision 1 of plans created before the
// orchestrator (startup; also done lazily on first read). Safe to run from
// several replicas: a revision that already exists is skipped.
func (s *OrchestratorUsecase) BackfillRevisions(ctx context.Context) (int, error) {
	total := 0
	for {
		plans, err := s.repos.Orch.ListPlansWithoutRevision(ctx, 100)
		if err != nil {
			return total, err
		}
		if len(plans) == 0 {
			return total, nil
		}
		progress := 0
		for i := range plans {
			if err := s.backfillPlan(ctx, &plans[i]); err != nil {
				log.Printf("Orchestrator: backfill of plan %s failed: %v", plans[i].ID, err)
				continue
			}
			progress++
		}
		total += progress
		if progress == 0 {
			return total, nil
		}
	}
}

func (s *OrchestratorUsecase) backfillPlan(ctx context.Context, plan *models.AIResourcePlan) error {
	resources := plan.Resources
	if resources == nil {
		if full, err := s.repos.Agent.FindPlanDetails(ctx, plan.ID, plan.UserID); err == nil {
			resources = full.Resources
		}
	}
	spec := specFromLegacy(plan, resources)
	rev, err := s.buildRevision(ctx, plan.UserID, false, spec, "planner", plan.Description)
	if err != nil {
		return err
	}
	rev.PlanID = plan.ID
	rev.Version = plan.Version
	if rev.Version <= 0 {
		rev.Version = 1
	}
	rev.CreatedAt = plan.CreatedAt
	return s.repos.Orch.CreateRevisionIfAbsent(ctx, rev)
}
