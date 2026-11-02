package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/rand/v2"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// Event types (contract §5).
const (
	EventExecutionCreated            = "execution.created"
	EventExecutionStarted            = "execution.started"
	EventApprovalRecorded            = "approval.recorded"
	EventTaskQueued                  = "task.queued"
	EventTaskStarted                 = "task.started"
	EventTaskProgress                = "task.progress"
	EventTaskRetrying                = "task.retrying"
	EventTaskSucceeded               = "task.succeeded"
	EventTaskFailed                  = "task.failed"
	EventTaskSkipped                 = "task.skipped"
	EventTaskCancelled               = "task.cancelled"
	EventVerificationStarted         = "verification.started"
	EventVerificationResult          = "verification.result"
	EventExecutionCompleted          = "execution.completed"
	EventExecutionFailed             = "execution.failed"
	EventExecutionPartiallyCompleted = "execution.partially_completed"
	EventExecutionCancelled          = "execution.cancelled"
	EventExecutionFinished           = "execution.finished"
)

// Event severities.
const (
	SeverityInfo    = "info"
	SeverityWarning = "warning"
	SeverityError   = "error"
)

// Hooks connect the engine to the use case layer.
type Hooks interface {
	// CheckTask is the execute-time policy check of one task.
	CheckTask(ctx context.Context, exec *models.AIExecution, task *models.AIExecutionTask, input map[string]any) *TaskError
	// Verify compares desired and actual state after a write task.
	Verify(ctx context.Context, exec *models.AIExecution, task *models.AIExecutionTask, output map[string]any) *Verification
	// Report builds the final report of a finished execution.
	Report(ctx context.Context, exec *models.AIExecution, tasks []models.AIExecutionTask, status string) *Report
	// CanDeleteCreated reports whether the execution created resources that
	// still exist and that a delete_created plan can remove.
	CanDeleteCreated(ctx context.Context, exec *models.AIExecution) bool
	// StatusChanged runs inside the transaction that changed the execution
	// status (plan status, conversation state and chat messages follow it).
	StatusChanged(ctx context.Context, exec *models.AIExecution, tasks []models.AIExecutionTask, status string)
}

// EngineConfig tunes the engine.
type EngineConfig struct {
	// Owner identifies this replica in leases (default host-pid-random).
	Owner string
	// Concurrency is the number of tasks run at once (default 4).
	Concurrency int
	// Lease is how long a claim lasts without a heartbeat (default 30s).
	Lease time.Duration
	// Heartbeat is the lease renewal interval (default Lease/3).
	Heartbeat time.Duration
	// PollInterval is how often runnable tasks are claimed (default 1s).
	PollInterval time.Duration
	// MaintenanceInterval is how often stalled executions are advanced
	// (default 5s).
	MaintenanceInterval time.Duration
	// RetryBase and RetryMax bound the exponential retry backoff (defaults
	// 2s and 60s).
	RetryBase time.Duration
	RetryMax  time.Duration
	// MaxClaims fails a task whose attempt was claimed this many times
	// without finishing (a task that crashes its runner; default 5).
	MaxClaims int
	// Faults injects failures: the first N attempts of tasks using a tool
	// fail with FAULT_INJECTED. Keys are "tool" or "tool@key-substring".
	Faults map[string]int
}

func (c *EngineConfig) defaults() {
	if c.Owner == "" {
		host, _ := os.Hostname()
		c.Owner = fmt.Sprintf("%s-%d-%s", host, os.Getpid(), uuid.NewString()[:8])
	}
	if c.Concurrency <= 0 {
		c.Concurrency = 4
	}
	if c.Lease <= 0 {
		c.Lease = 30 * time.Second
	}
	if c.Heartbeat <= 0 || c.Heartbeat*2 > c.Lease {
		c.Heartbeat = c.Lease / 3
	}
	if c.PollInterval <= 0 {
		c.PollInterval = time.Second
	}
	if c.MaintenanceInterval <= 0 {
		c.MaintenanceInterval = 5 * time.Second
	}
	if c.RetryBase <= 0 {
		c.RetryBase = 2 * time.Second
	}
	if c.RetryMax <= 0 {
		c.RetryMax = time.Minute
	}
	if c.MaxClaims <= 0 {
		c.MaxClaims = 5
	}
}

// Cancellation causes of a running task's context.
var (
	errLeaseLost     = errors.New("task lease lost")
	errShutdown      = errors.New("engine shutting down")
	errExecCancelled = errors.New("execution cancelled")
	// ErrNotActive is returned when an execution is already finished.
	ErrNotActive = errors.New("execution is not active")
)

type runningTask struct {
	executionID uuid.UUID
	cancel      context.CancelCauseFunc
}

// Engine runs execution task graphs. Every API replica runs one; tasks are
// leased in Postgres, so replicas share the work and pick up each other's
// tasks after a crash.
type Engine struct {
	repo   repository.OrchestratorRepository
	tx     repository.Transactor
	reg    *Registry
	hooks  Hooks
	cfg    EngineConfig
	notify *Notifier

	mu        sync.Mutex
	running   map[uuid.UUID]*runningTask // task id -> runner
	verifying map[uuid.UUID]context.CancelCauseFunc

	startOnce sync.Once
	started   atomic.Bool
	stopOnce  sync.Once
	stop      chan struct{}
	loopDone  chan struct{}
	wake      chan struct{}
	wg        sync.WaitGroup
}

// NewEngine returns an engine; call Start to run it.
func NewEngine(repo repository.OrchestratorRepository, tx repository.Transactor, reg *Registry, hooks Hooks, notify *Notifier, cfg EngineConfig) *Engine {
	cfg.defaults()
	if notify == nil {
		notify = NewNotifier()
	}
	return &Engine{
		repo:      repo,
		tx:        tx,
		reg:       reg,
		hooks:     hooks,
		cfg:       cfg,
		notify:    notify,
		running:   map[uuid.UUID]*runningTask{},
		verifying: map[uuid.UUID]context.CancelCauseFunc{},
		stop:      make(chan struct{}),
		loopDone:  make(chan struct{}),
		wake:      make(chan struct{}, 1),
	}
}

// Owner is the lease owner id of this engine.
func (e *Engine) Owner() string { return e.cfg.Owner }

// Notifier is the engine's event notifier (SSE streams wait on it).
func (e *Engine) Notifier() *Notifier { return e.notify }

// SetHooks sets the hooks (the use case that implements them is built after
// the engine).
func (e *Engine) SetHooks(h Hooks) { e.hooks = h }

// Start begins claiming tasks.
func (e *Engine) Start() {
	e.startOnce.Do(func() {
		e.started.Store(true)
		go e.loop()
	})
}

// Nudge makes the engine look for runnable tasks now.
func (e *Engine) Nudge() {
	select {
	case e.wake <- struct{}{}:
	default:
	}
}

func (e *Engine) loop() {
	defer close(e.loopDone)
	poll := time.NewTicker(e.cfg.PollInterval)
	defer poll.Stop()
	maint := time.NewTicker(e.cfg.MaintenanceInterval)
	defer maint.Stop()
	e.maintenance()
	for {
		select {
		case <-e.stop:
			return
		case <-poll.C:
			e.claimAndRun()
		case <-e.wake:
			e.claimAndRun()
		case <-maint.C:
			e.maintenance()
		}
	}
}

func (e *Engine) runningCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.running)
}

func (e *Engine) claimAndRun() {
	free := e.cfg.Concurrency - e.runningCount()
	if free <= 0 {
		return
	}
	tasks, err := e.repo.ClaimTasks(context.Background(), e.cfg.Owner, free, e.cfg.Lease)
	if err != nil {
		log.Printf("Orchestrator: claim failed: %v", err)
		return
	}
	for i := range tasks {
		e.startRunner(tasks[i])
	}
}

func (e *Engine) startRunner(task models.AIExecutionTask) {
	if task.LeaseToken == nil {
		return
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	e.mu.Lock()
	e.running[task.ID] = &runningTask{executionID: task.ExecutionID, cancel: cancel}
	e.mu.Unlock()
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.running, task.ID)
			e.mu.Unlock()
			cancel(nil)
			e.Nudge()
		}()
		defer func() {
			if r := recover(); r != nil {
				log.Printf("Orchestrator: task %s panicked: %v", task.Key, r)
				e.failTask(&task, *task.LeaseToken, Permanent("TOOL_PANIC", "the tool crashed"))
			}
		}()
		e.runTask(ctx, cancel, &task)
	}()
}

// heartbeat renews the task lease until done is closed. A lost lease cancels
// the task with errLeaseLost (the runner then writes nothing); a cancel
// request on the execution cancels it with errExecCancelled.
func (e *Engine) heartbeat(cancel context.CancelCauseFunc, taskID, token uuid.UUID, done <-chan struct{}) {
	ticker := time.NewTicker(e.cfg.Heartbeat)
	defer ticker.Stop()
	cancelled := false
	for {
		select {
		case <-done:
			return
		case <-ticker.C:
			alive, cancelReq, err := e.repo.HeartbeatTask(context.Background(), taskID, token, e.cfg.Lease)
			if err != nil {
				log.Printf("Orchestrator: heartbeat of task %s failed: %v", taskID, err)
				continue
			}
			if !alive {
				cancel(errLeaseLost)
				return
			}
			if cancelReq && !cancelled {
				cancelled = true
				cancel(errExecCancelled)
			}
		}
	}
}

func (e *Engine) faultFor(task *models.AIExecutionTask) int {
	for k, n := range e.cfg.Faults {
		tool, sub, hasSub := strings.Cut(k, "@")
		if tool != task.Tool {
			continue
		}
		if hasSub && !strings.Contains(task.Key, sub) {
			continue
		}
		return n
	}
	return 0
}

func (e *Engine) runTask(ctx context.Context, cancel context.CancelCauseFunc, task *models.AIExecutionTask) {
	bg := context.Background()
	token := *task.LeaseToken
	hbDone := make(chan struct{})
	go e.heartbeat(cancel, task.ID, token, hbDone)
	defer close(hbDone)

	exec, err := e.repo.FindExecution(bg, task.ExecutionID, nil)
	if err != nil {
		log.Printf("Orchestrator: execution of task %s not found: %v", task.Key, err)
		_ = e.repo.ReleaseTaskLease(bg, task.ID, token)
		return
	}
	if task.ClaimCount > e.cfg.MaxClaims {
		e.failTask(task, token, Permanent("LEASE_EXHAUSTED", fmt.Sprintf("attempt %d was interrupted %d times", task.Attempt, task.ClaimCount-1)))
		return
	}
	if exec.CancelRequested {
		e.finishTask(task, token, models.TaskCancelled, nil, nil)
		return
	}

	if exec.Status == models.ExecQueued {
		e.markRunning(exec.ID)
	}
	resumed := task.ClaimCount > 1
	if def, ok := e.reg.Def(task.Tool); ok && !def.Idempotent && resumed {
		// The interrupted run may already have applied the change (a
		// snapshot restore rolled the VM back): running it again could apply
		// it twice. Only the user may retry it.
		e.failTask(task, token, Permanent(ErrCodeInterruptedNotIdempotent, fmt.Sprintf(
			"%s was interrupted after it started and is not retried automatically because it is not idempotent (running it again could apply it twice). Check the resource, then retry the task manually if needed.", task.Title)))
		return
	}
	msg := fmt.Sprintf("%s (attempt %d of %d)", task.Title, task.Attempt, task.MaxAttempts)
	if resumed {
		msg += " resumed after an interrupted run"
	}
	e.event(bg, exec.ID, task.Key, EventTaskStarted, SeverityInfo, msg, map[string]any{"attempt": task.Attempt, "resumed": resumed})

	var input map[string]any
	if err := DecodeJSON(task.Input, &input); err != nil || input == nil {
		input = map[string]any{}
	}
	if e.hooks != nil {
		if te := e.hooks.CheckTask(bg, exec, task, input); te != nil {
			e.handleFailure(task, token, te)
			return
		}
	}
	def, ok := e.reg.Def(task.Tool)
	runner, bound := e.reg.Runner(task.Tool)
	if !ok || !bound {
		e.failTask(task, token, Permanent("TOOL_NOT_FOUND", "tool "+task.Tool+" is not available"))
		return
	}

	var out map[string]any
	var runErr error
	if n := e.faultFor(task); n > 0 && task.Attempt <= n {
		runErr = Retryable("FAULT_INJECTED", fmt.Sprintf("injected failure (attempt %d of the first %d)", task.Attempt, n))
	} else {
		tc := &TaskContext{
			ExecutionID:        exec.ID,
			ExecutionCreatedAt: exec.CreatedAt,
			PlanID:             exec.PlanID,
			ConversationID:     exec.ConversationID,
			TaskID:             task.ID,
			TaskKey:            task.Key,
			Tool:               task.Tool,
			UserID:             exec.UserID,
			ClientIP:           exec.ClientIP,
			Attempt:            task.Attempt,
			IdempotencyKey:     task.IdempotencyKey,
			Resumed:            resumed,
			Input:              input,
			DependencyOutputs:  e.dependencyOutputs(task),
		}
		tc.progress = func(message string, metadata map[string]any) {
			e.event(bg, exec.ID, task.Key, EventTaskProgress, SeverityInfo, message, metadata)
		}
		tc.record = func(relation, resourceType string, id uuid.UUID, name string) {
			execID, taskID, planID := exec.ID, task.ID, exec.PlanID
			if err := e.repo.RecordRelationship(bg, &models.ResourceRelationship{
				OwnerID: exec.UserID, ExecutionID: &execID, TaskID: &taskID, PlanID: &planID,
				Relation: relation, ResourceType: resourceType, ResourceID: id, ResourceName: name,
			}); err != nil {
				log.Printf("Orchestrator: record relationship: %v", err)
			}
		}
		toolCtx, cancelTool := context.WithTimeout(ctx, def.Timeout())
		out, runErr = runner.Run(toolCtx, tc)
		if runErr != nil && errors.Is(toolCtx.Err(), context.DeadlineExceeded) && ctx.Err() == nil {
			runErr = &TaskError{Code: "TOOL_TIMEOUT", Message: fmt.Sprintf("%s did not finish within %s", task.Tool, def.Timeout()), Retryable: def.Idempotent}
		}
		cancelTool()
	}

	switch cause := context.Cause(ctx); {
	case errors.Is(cause, errLeaseLost):
		log.Printf("Orchestrator: lost the lease of task %s (attempt %d); another runner owns it", task.Key, task.Attempt)
		return
	case errors.Is(cause, errShutdown):
		return // the lease is released by Shutdown
	case errors.Is(cause, errExecCancelled):
		e.finishTask(task, token, models.TaskCancelled, nil, nil)
		return
	}
	if runErr != nil {
		e.handleFailure(task, token, AsTaskError(runErr, def.Idempotent))
		return
	}
	if out == nil {
		out = map[string]any{}
	}
	e.finishTask(task, token, models.TaskSucceeded, out, nil)
}

func (e *Engine) dependencyOutputs(task *models.AIExecutionTask) map[string]map[string]any {
	var deps []string
	_ = json.Unmarshal([]byte(task.DependsOn), &deps)
	out := map[string]map[string]any{}
	if len(deps) == 0 {
		return out
	}
	tasks, err := e.repo.ListTasks(context.Background(), task.ExecutionID)
	if err != nil {
		return out
	}
	for _, t := range tasks {
		for _, d := range deps {
			if t.Key == d && t.Output != nil {
				var m map[string]any
				if DecodeJSON(*t.Output, &m) == nil {
					out[d] = m
				}
			}
		}
	}
	return out
}

// handleFailure retries a retryable failure while attempts remain, except
// for tools that are not idempotent: they are never retried automatically
// (whatever the error, including TOOL_TIMEOUT), only by a manual retry.
func (e *Engine) handleFailure(task *models.AIExecutionTask, token uuid.UUID, te *TaskError) {
	if def, ok := e.reg.Def(task.Tool); ok && !def.Idempotent && te.Retryable {
		te = &TaskError{Code: te.Code, Message: te.Message + " (not retried automatically: the tool is not idempotent; retry manually)"}
	}
	if te.Retryable && task.Attempt < task.MaxAttempts {
		e.retryTask(task, token, te)
		return
	}
	e.failTask(task, token, te)
}

// Backoff returns the delay before attempt+1: RetryBase * 2^(attempt-1),
// capped at RetryMax, with +-20% jitter.
func (e *Engine) Backoff(attempt int) time.Duration {
	d := e.cfg.RetryBase
	for i := 1; i < attempt && d < e.cfg.RetryMax; i++ {
		d *= 2
	}
	if d > e.cfg.RetryMax {
		d = e.cfg.RetryMax
	}
	jitter := 0.8 + 0.4*rand.Float64()
	return time.Duration(float64(d) * jitter)
}

func errorJSON(te *TaskError) string {
	return MustJSON(map[string]any{"code": te.Code, "message": te.Message})
}

// event appends an execution event (outside or inside a transaction) and
// wakes local SSE streams.
func (e *Engine) event(ctx context.Context, executionID uuid.UUID, taskKey, typ, severity, message string, metadata map[string]any) {
	if metadata == nil {
		metadata = map[string]any{}
	}
	ev := &models.AIExecutionEvent{
		ExecutionID: executionID,
		Type:        typ,
		Severity:    severity,
		Message:     message,
		Metadata:    MustJSON(metadata),
	}
	if taskKey != "" {
		k := taskKey
		ev.TaskKey = &k
	}
	if err := e.repo.AppendEvent(ctx, ev); err != nil {
		log.Printf("Orchestrator: append event %s: %v", typ, err)
	}
	e.notify.Notify(executionID)
}

// Event appends an event for the use case layer (approval, creation).
func (e *Engine) Event(ctx context.Context, executionID uuid.UUID, taskKey, typ, severity, message string, metadata map[string]any) {
	e.event(ctx, executionID, taskKey, typ, severity, message, metadata)
}

// markRunning moves a queued execution to running on its first task.
func (e *Engine) markRunning(executionID uuid.UUID) {
	err := e.tx.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		exec, err := e.repo.LockExecution(txCtx, executionID)
		if err != nil || exec.Status != models.ExecQueued {
			return err
		}
		now := time.Now()
		if err := e.repo.UpdateExecution(txCtx, executionID, map[string]any{"status": models.ExecRunning, "started_at": now}); err != nil {
			return err
		}
		exec.Status, exec.StartedAt = models.ExecRunning, &now
		e.event(txCtx, executionID, "", EventExecutionStarted, SeverityInfo, "Execution started", nil)
		if e.hooks != nil {
			tasks, _ := e.repo.ListTasks(txCtx, executionID)
			e.hooks.StatusChanged(txCtx, exec, tasks, models.ExecRunning)
		}
		return nil
	})
	e.notify.Notify(executionID)
	if err != nil {
		log.Printf("Orchestrator: start execution %s: %v", executionID, err)
	}
}

// finishTask records a terminal task status under the task lease and moves
// the graph on: dependents whose dependencies all succeeded are queued (or
// cancelled when a cancel was requested).
func (e *Engine) finishTask(task *models.AIExecutionTask, token uuid.UUID, status string, output map[string]any, te *TaskError) {
	now := time.Now()
	err := e.tx.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		exec, err := e.repo.LockExecution(txCtx, task.ExecutionID)
		if err != nil {
			return err
		}
		fields := map[string]any{
			"status": status, "finished_at": now,
			"lease_owner": nil, "lease_token": nil, "lease_expires_at": nil,
		}
		if output != nil {
			fields["output"] = MustJSON(output)
		}
		if te != nil {
			fields["error"] = errorJSON(te)
		} else if status == models.TaskSucceeded {
			fields["error"] = nil
		}
		ok, err := e.repo.UpdateLeasedTask(txCtx, task.ID, token, fields)
		if err != nil {
			return err
		}
		if !ok {
			return errLeaseLost
		}
		switch status {
		case models.TaskSucceeded:
			e.event(txCtx, exec.ID, task.Key, EventTaskSucceeded, SeverityInfo, task.Title+" succeeded", map[string]any{"attempt": task.Attempt})
		case models.TaskCancelled:
			e.event(txCtx, exec.ID, task.Key, EventTaskCancelled, SeverityWarning, task.Title+" cancelled", nil)
		}
		return e.propagate(txCtx, exec)
	})
	if err != nil && !errors.Is(err, errLeaseLost) {
		log.Printf("Orchestrator: finish task %s: %v", task.Key, err)
	}
	e.advance(task.ExecutionID)
	e.Nudge()
}

// propagate re-evaluates blocked tasks (caller holds the execution lock):
// all dependencies succeeded -> queued (cancelled if a cancel is pending);
// a dependency failed, was skipped or cancelled -> skipped.
func (e *Engine) propagate(txCtx context.Context, exec *models.AIExecution) error {
	for {
		tasks, err := e.repo.ListTasks(txCtx, exec.ID)
		if err != nil {
			return err
		}
		status := map[string]string{}
		for _, t := range tasks {
			status[t.Key] = t.Status
		}
		changed := false
		for _, t := range tasks {
			if t.Status != models.TaskBlocked {
				continue
			}
			var deps []string
			_ = json.Unmarshal([]byte(t.DependsOn), &deps)
			allOK, anyDead := true, false
			for _, d := range deps {
				switch status[d] {
				case models.TaskSucceeded:
				case models.TaskFailed, models.TaskSkipped, models.TaskCancelled:
					anyDead = true
					allOK = false
				default:
					allOK = false
				}
			}
			switch {
			case anyDead:
				if moved, err := e.repo.UpdateTasksStatus(txCtx, exec.ID, []string{t.Key}, []string{models.TaskBlocked}, models.TaskSkipped, map[string]any{"finished_at": time.Now()}); err != nil {
					return err
				} else if len(moved) > 0 {
					e.event(txCtx, exec.ID, t.Key, EventTaskSkipped, SeverityWarning, t.Title+" skipped: a task it depends on did not succeed", nil)
					changed = true
				}
			case allOK && exec.CancelRequested:
				if moved, err := e.repo.UpdateTasksStatus(txCtx, exec.ID, []string{t.Key}, []string{models.TaskBlocked}, models.TaskCancelled, map[string]any{"finished_at": time.Now()}); err != nil {
					return err
				} else if len(moved) > 0 {
					e.event(txCtx, exec.ID, t.Key, EventTaskCancelled, SeverityWarning, t.Title+" cancelled", nil)
					changed = true
				}
			case allOK:
				if moved, err := e.repo.UpdateTasksStatus(txCtx, exec.ID, []string{t.Key}, []string{models.TaskBlocked}, models.TaskQueued, nil); err != nil {
					return err
				} else if len(moved) > 0 {
					e.event(txCtx, exec.ID, t.Key, EventTaskQueued, SeverityInfo, t.Title+" queued", nil)
					changed = true
				}
			}
		}
		if !changed {
			return nil
		}
	}
}

func (e *Engine) retryTask(task *models.AIExecutionTask, token uuid.UUID, te *TaskError) {
	delay := e.Backoff(task.Attempt)
	next := time.Now().Add(delay)
	attempt := task.Attempt + 1
	err := e.tx.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		exec, err := e.repo.LockExecution(txCtx, task.ExecutionID)
		if err != nil {
			return err
		}
		ok, err := e.repo.UpdateLeasedTask(txCtx, task.ID, token, map[string]any{
			"status": models.TaskRetrying, "attempt": attempt, "next_attempt_at": next,
			"idempotency_key": IdempotencyKey(exec.ID, task.Key, attempt),
			"error":           errorJSON(te),
			"lease_owner":     nil, "lease_token": nil, "lease_expires_at": nil,
		})
		if err != nil {
			return err
		}
		if !ok {
			return errLeaseLost
		}
		e.event(txCtx, exec.ID, task.Key, EventTaskRetrying, SeverityWarning,
			fmt.Sprintf("%s failed (%s: %s); retrying in %s (attempt %d of %d)", task.Title, te.Code, te.Message, delay.Round(100*time.Millisecond), attempt, task.MaxAttempts),
			map[string]any{"attempt": attempt, "next_attempt_at": next.UTC().Format(time.RFC3339Nano), "error": map[string]any{"code": te.Code, "message": te.Message}})
		return nil
	})
	if err != nil && !errors.Is(err, errLeaseLost) {
		log.Printf("Orchestrator: schedule retry of %s: %v", task.Key, err)
	}
	e.notify.Notify(task.ExecutionID)
}

// failTask fails a task permanently; its descendants are skipped.
func (e *Engine) failTask(task *models.AIExecutionTask, token uuid.UUID, te *TaskError) {
	now := time.Now()
	err := e.tx.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		exec, err := e.repo.LockExecution(txCtx, task.ExecutionID)
		if err != nil {
			return err
		}
		ok, err := e.repo.UpdateLeasedTask(txCtx, task.ID, token, map[string]any{
			"status": models.TaskFailed, "finished_at": now, "error": errorJSON(te),
			"lease_owner": nil, "lease_token": nil, "lease_expires_at": nil,
		})
		if err != nil {
			return err
		}
		if !ok {
			return errLeaseLost
		}
		e.event(txCtx, exec.ID, task.Key, EventTaskFailed, SeverityError,
			fmt.Sprintf("%s failed: %s", task.Title, te.Message),
			map[string]any{"attempt": task.Attempt, "error": map[string]any{"code": te.Code, "message": te.Message}})
		return e.propagate(txCtx, exec)
	})
	if err != nil && !errors.Is(err, errLeaseLost) {
		log.Printf("Orchestrator: fail task %s: %v", task.Key, err)
	}
	e.advance(task.ExecutionID)
}

func isTerminalExec(status string) bool {
	switch status {
	case models.ExecCompleted, models.ExecFailed, models.ExecPartiallyCompleted, models.ExecCancelled:
		return true
	}
	return false
}

func isActiveTask(status string) bool {
	switch status {
	case models.TaskQueued, models.TaskBlocked, models.TaskRunning, models.TaskRetrying, models.TaskCompensating:
		return true
	}
	return false
}

// advance finishes an execution whose tasks are all done: cancelled when a
// cancel was requested, verification when every task succeeded, otherwise
// failed or partially completed. It is idempotent and safe to call from any
// replica.
func (e *Engine) advance(executionID uuid.UUID) {
	var verifyToken *uuid.UUID
	err := e.tx.WithinTransaction(context.Background(), func(txCtx context.Context) error {
		exec, err := e.repo.LockExecution(txCtx, executionID)
		if err != nil {
			return err
		}
		if isTerminalExec(exec.Status) || exec.Status == models.ExecVerifying {
			return nil
		}
		tasks, err := e.repo.ListTasks(txCtx, executionID)
		if err != nil {
			return err
		}
		for _, t := range tasks {
			if isActiveTask(t.Status) {
				return nil
			}
		}
		if exec.CancelRequested {
			return e.finalize(txCtx, exec, tasks, models.ExecCancelled)
		}
		allOK := true
		for _, t := range tasks {
			if t.Status != models.TaskSucceeded {
				allOK = false
			}
		}
		if !allOK {
			return e.finalize(txCtx, exec, tasks, failureStatus(tasks))
		}
		token := uuid.New()
		if err := e.repo.UpdateExecution(txCtx, executionID, map[string]any{
			"status": models.ExecVerifying, "lease_owner": e.cfg.Owner, "lease_token": token,
			"lease_expires_at": time.Now().Add(e.cfg.Lease),
		}); err != nil {
			return err
		}
		exec.Status = models.ExecVerifying
		e.event(txCtx, executionID, "", EventVerificationStarted, SeverityInfo, "Verifying the desired state", nil)
		if e.hooks != nil {
			e.hooks.StatusChanged(txCtx, exec, tasks, models.ExecVerifying)
		}
		verifyToken = &token
		return nil
	})
	e.notify.Notify(executionID)
	if err != nil {
		log.Printf("Orchestrator: advance execution %s: %v", executionID, err)
		return
	}
	if verifyToken != nil {
		e.startVerification(executionID, *verifyToken)
	}
}

// failureStatus is partially_completed when at least one write task
// succeeded (something changed), failed otherwise.
func failureStatus(tasks []models.AIExecutionTask) string {
	for _, t := range tasks {
		if t.Status == models.TaskSucceeded && IsWrite(t.Risk) {
			return models.ExecPartiallyCompleted
		}
	}
	return models.ExecFailed
}

// finalize writes the terminal status, report and recovery options and the
// closing events (caller holds the execution lock).
func (e *Engine) finalize(txCtx context.Context, exec *models.AIExecution, tasks []models.AIExecutionTask, status string) error {
	var report *Report
	if e.hooks != nil {
		report = e.hooks.Report(txCtx, exec, tasks, status)
	}
	if report == nil {
		report = &Report{Summary: "Execution " + status, Resources: []ReportResource{}, Warnings: []string{}, NextActions: []string{}}
	}
	canDelete := false
	if e.hooks != nil && (status == models.ExecFailed || status == models.ExecPartiallyCompleted) {
		canDelete = e.hooks.CanDeleteCreated(txCtx, exec)
	}
	recovery := RecoveryOptions(status, tasks, canDelete)
	now := time.Now()
	if err := e.repo.UpdateExecution(txCtx, exec.ID, map[string]any{
		"status": status, "finished_at": now, "report": MustJSON(report),
		"recovery_options": MustJSON(recovery),
		"lease_owner":      nil, "lease_token": nil, "lease_expires_at": nil,
	}); err != nil {
		return err
	}
	exec.Status, exec.FinishedAt = status, &now
	typ, severity := EventExecutionCompleted, SeverityInfo
	switch status {
	case models.ExecFailed:
		typ, severity = EventExecutionFailed, SeverityError
	case models.ExecPartiallyCompleted:
		typ, severity = EventExecutionPartiallyCompleted, SeverityWarning
	case models.ExecCancelled:
		typ, severity = EventExecutionCancelled, SeverityWarning
	}
	e.event(txCtx, exec.ID, "", typ, severity, report.Summary, map[string]any{"status": status})
	if e.hooks != nil {
		e.hooks.StatusChanged(txCtx, exec, tasks, status)
	}
	e.event(txCtx, exec.ID, "", EventExecutionFinished, severity, "Execution finished: "+status, map[string]any{"status": status})
	return nil
}

// RecoveryOptions lists the actions offered on a failed or partial
// execution. delete_created is offered only when canDeleteCreated (the
// execution has "created" resources that are not deleted and that a plan
// can remove); it always goes through a new destructive plan that needs its
// own approval.
func RecoveryOptions(status string, tasks []models.AIExecutionTask, canDeleteCreated bool) []RecoveryOption {
	out := []RecoveryOption{}
	if status != models.ExecFailed && status != models.ExecPartiallyCompleted {
		return out
	}
	for _, t := range tasks {
		if t.Status == models.TaskFailed {
			out = append(out, RecoveryOption{Action: RecoveryRetryFailed, Label: "Retry failed tasks", RequiresApproval: false})
			break
		}
	}
	if status == models.ExecPartiallyCompleted {
		out = append(out, RecoveryOption{Action: RecoveryKeep, Label: "Keep what was created", RequiresApproval: false})
	}
	if canDeleteCreated {
		out = append(out, RecoveryOption{Action: RecoveryDeleteCreated, Label: "Delete the resources this execution created", RequiresApproval: true})
	}
	return out
}

// WithoutDeleteCreated drops the delete_created option (the created
// resources are gone since the options were computed).
func WithoutDeleteCreated(opts []RecoveryOption) []RecoveryOption {
	out := make([]RecoveryOption, 0, len(opts))
	for _, o := range opts {
		if o.Action != RecoveryDeleteCreated {
			out = append(out, o)
		}
	}
	return out
}

func (e *Engine) startVerification(executionID, token uuid.UUID) {
	ctx, cancel := context.WithCancelCause(context.Background())
	e.mu.Lock()
	e.verifying[executionID] = cancel
	e.mu.Unlock()
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		defer func() {
			e.mu.Lock()
			delete(e.verifying, executionID)
			e.mu.Unlock()
			cancel(nil)
		}()
		e.verify(ctx, executionID, token)
	}()
}

// verify runs the verification hooks for every succeeded write task, then
// finalizes the execution. The execution lease is renewed meanwhile; when
// it is lost (or the engine shuts down) another replica redoes the work.
func (e *Engine) verify(ctx context.Context, executionID, token uuid.UUID) {
	bg := context.Background()
	done := make(chan struct{})
	defer close(done)
	go func() {
		ticker := time.NewTicker(e.cfg.Heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if ok, err := e.repo.ClaimExecutionLease(bg, executionID, e.cfg.Owner, token, e.cfg.Lease); err == nil && !ok {
					return
				}
			}
		}
	}()

	exec, err := e.repo.FindExecution(bg, executionID, nil)
	if err != nil {
		return
	}
	tasks, err := e.repo.ListTasks(bg, executionID)
	if err != nil {
		return
	}
	anyFailed := false
	for i := range tasks {
		t := &tasks[i]
		if t.Status != models.TaskSucceeded || !IsWrite(t.Risk) {
			continue
		}
		if ctx.Err() != nil {
			return
		}
		var out map[string]any
		if t.Output != nil {
			_ = DecodeJSON(*t.Output, &out)
		}
		var v *Verification
		if e.hooks != nil {
			v = e.hooks.Verify(ctx, exec, t, out)
		}
		if v == nil {
			v = (&Verification{Checks: []Check{{Name: "verifier", Status: "skip", Detail: "no verifier for " + t.Tool}}}).Summarize()
		}
		if v.Status == VerifyFail {
			anyFailed = true
		}
		vj := MustJSON(v)
		t.Verification = &vj
		_ = e.repo.UpdateTask(bg, t.ID, map[string]any{"verification": vj})
		severity := SeverityInfo
		switch v.Status {
		case Unverified:
			severity = SeverityWarning
		case VerifyFail:
			severity = SeverityError
		}
		e.event(bg, executionID, t.Key, EventVerificationResult, severity,
			fmt.Sprintf("%s: %s", t.Title, v.Status), map[string]any{"status": v.Status, "checks": v.Checks})
	}
	if ctx.Err() != nil {
		return
	}
	err = e.tx.WithinTransaction(bg, func(txCtx context.Context) error {
		locked, err := e.repo.LockExecution(txCtx, executionID)
		if err != nil {
			return err
		}
		if locked.Status != models.ExecVerifying || locked.LeaseToken == nil || *locked.LeaseToken != token {
			return nil // another replica took over
		}
		fresh, err := e.repo.ListTasks(txCtx, executionID)
		if err != nil {
			return err
		}
		status := models.ExecCompleted
		if anyFailed {
			status = models.ExecPartiallyCompleted
		}
		return e.finalize(txCtx, locked, fresh, status)
	})
	e.notify.Notify(executionID)
	if err != nil {
		log.Printf("Orchestrator: finalize execution %s: %v", executionID, err)
	}
}

// maintenance advances executions that have no claimable task left:
// finished task graphs that were not finalized (crash between the last task
// and finalization), verifications whose lease expired, and orphaned
// running tasks of cancelling executions.
func (e *Engine) maintenance() {
	bg := context.Background()
	ids, err := e.repo.ListStalledExecutions(bg, 20)
	if err != nil {
		log.Printf("Orchestrator: maintenance scan: %v", err)
		return
	}
	for _, id := range ids {
		exec, err := e.repo.FindExecution(bg, id, nil)
		if err != nil {
			continue
		}
		switch exec.Status {
		case models.ExecVerifying:
			token := uuid.New()
			if ok, err := e.repo.ClaimExecutionLease(bg, id, e.cfg.Owner, token, e.cfg.Lease); err == nil && ok {
				e.startVerification(id, token)
			}
			continue
		case models.ExecCancelling:
			if keys, err := e.repo.CancelOrphanedTasks(bg, id); err == nil {
				for _, k := range keys {
					e.event(bg, id, k, EventTaskCancelled, SeverityWarning, "Task cancelled (its runner stopped)", nil)
				}
			}
		}
		e.advance(id)
	}
}

// Cancel requests cancellation: pending tasks are cancelled at once, running
// tasks are cancelled by their runners (locally right away, on other
// replicas at their next heartbeat), and the execution ends as cancelled.
// ctx may carry a transaction.
func (e *Engine) Cancel(ctx context.Context, executionID uuid.UUID) error {
	err := e.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		exec, err := e.repo.LockExecution(txCtx, executionID)
		if err != nil {
			return err
		}
		if isTerminalExec(exec.Status) {
			return ErrNotActive
		}
		if exec.CancelRequested {
			return nil
		}
		if err := e.repo.UpdateExecution(txCtx, executionID, map[string]any{"cancel_requested": true, "status": models.ExecCancelling}); err != nil {
			return err
		}
		exec.CancelRequested, exec.Status = true, models.ExecCancelling
		tasks, err := e.repo.ListTasks(txCtx, executionID)
		if err != nil {
			return err
		}
		var pending []string
		for _, t := range tasks {
			switch t.Status {
			case models.TaskQueued, models.TaskBlocked, models.TaskRetrying:
				pending = append(pending, t.Key)
			}
		}
		moved, err := e.repo.UpdateTasksStatus(txCtx, executionID, pending,
			[]string{models.TaskQueued, models.TaskBlocked, models.TaskRetrying}, models.TaskCancelled,
			map[string]any{"finished_at": time.Now(), "next_attempt_at": nil})
		if err != nil {
			return err
		}
		for _, k := range moved {
			e.event(txCtx, executionID, k, EventTaskCancelled, SeverityWarning, "Task cancelled", nil)
		}
		if e.hooks != nil {
			e.hooks.StatusChanged(txCtx, exec, tasks, models.ExecCancelling)
		}
		return nil
	})
	if err != nil {
		return err
	}
	e.mu.Lock()
	for _, rt := range e.running {
		if rt.executionID == executionID {
			rt.cancel(errExecCancelled)
		}
	}
	e.mu.Unlock()
	e.advance(executionID)
	return nil
}

// RetryInput selects the failed tasks to run again.
type RetryInput struct {
	// TaskID retries one failed task; nil retries every failed task.
	TaskID *uuid.UUID
	// Activate is called inside the transaction before an inactive
	// execution becomes active again (rate limits).
	Activate func(txCtx context.Context, exec *models.AIExecution) error
}

// ErrTaskNotRetryable is returned when the selected task is not failed.
var ErrTaskNotRetryable = errors.New("task is not failed")

// Retry re-queues failed tasks (with a fresh attempt budget and a new
// idempotency key) and their skipped descendants. ctx may carry a
// transaction.
func (e *Engine) Retry(ctx context.Context, executionID uuid.UUID, in RetryInput) error {
	err := e.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		exec, err := e.repo.LockExecution(txCtx, executionID)
		if err != nil {
			return err
		}
		switch exec.Status {
		case models.ExecRunning, models.ExecQueued, models.ExecFailed, models.ExecPartiallyCompleted:
		default:
			return ErrNotActive
		}
		tasks, err := e.repo.ListTasks(txCtx, executionID)
		if err != nil {
			return err
		}
		specs := make([]TaskSpec, len(tasks))
		byKey := map[string]*models.AIExecutionTask{}
		var targets []*models.AIExecutionTask
		for i := range tasks {
			t := &tasks[i]
			byKey[t.Key] = t
			var deps []string
			_ = json.Unmarshal([]byte(t.DependsOn), &deps)
			specs[i] = TaskSpec{Key: t.Key, DependsOn: deps}
			if in.TaskID != nil && t.ID == *in.TaskID {
				if t.Status != models.TaskFailed {
					return ErrTaskNotRetryable
				}
				targets = append(targets, t)
			}
			if in.TaskID == nil && t.Status == models.TaskFailed {
				targets = append(targets, t)
			}
		}
		if len(targets) == 0 {
			return ErrTaskNotRetryable
		}
		if isTerminalExec(exec.Status) && in.Activate != nil {
			if err := in.Activate(txCtx, exec); err != nil {
				return err
			}
		}
		var keys []string
		for _, t := range targets {
			keys = append(keys, t.Key)
			def, _ := e.reg.Def(t.Tool)
			budget := def.MaxAttempts
			if budget <= 0 {
				budget = 3
			}
			attempt := t.Attempt + 1
			if err := e.repo.UpdateTask(txCtx, t.ID, map[string]any{
				"status": models.TaskBlocked, "attempt": attempt, "max_attempts": attempt + budget - 1,
				"idempotency_key": IdempotencyKey(executionID, t.Key, attempt),
				"next_attempt_at": nil, "finished_at": nil, "error": nil, "claim_count": 0,
				"lease_owner": nil, "lease_token": nil, "lease_expires_at": nil, "updated_at": time.Now(),
			}); err != nil {
				return err
			}
			e.event(txCtx, executionID, t.Key, EventTaskRetrying, SeverityInfo,
				fmt.Sprintf("%s: manual retry requested (attempt %d)", t.Title, attempt), map[string]any{"attempt": attempt, "manual_retry": true})
		}
		// Skipped descendants wait again for their dependencies.
		if _, err := e.repo.UpdateTasksStatus(txCtx, executionID, Descendants(specs, keys...),
			[]string{models.TaskSkipped}, models.TaskBlocked, map[string]any{"finished_at": nil}); err != nil {
			return err
		}
		if isTerminalExec(exec.Status) {
			if err := e.repo.UpdateExecution(txCtx, executionID, map[string]any{
				"status": models.ExecRunning, "finished_at": nil, "report": nil,
				"recovery_options": "[]", "kept_at": nil,
			}); err != nil {
				return err
			}
			exec.Status, exec.FinishedAt = models.ExecRunning, nil
			e.event(txCtx, executionID, "", EventExecutionStarted, SeverityInfo, "Execution resumed for a retry", map[string]any{"retry": true})
		}
		// Retried tasks are blocked until propagate queues them (their
		// dependencies already succeeded) or skips them again.
		if err := e.propagate(txCtx, exec); err != nil {
			return err
		}
		if e.hooks != nil {
			fresh, _ := e.repo.ListTasks(txCtx, executionID)
			e.hooks.StatusChanged(txCtx, exec, fresh, models.ExecRunning)
		}
		return nil
	})
	if err != nil {
		return err
	}
	e.notify.Notify(executionID)
	e.Nudge()
	return nil
}

// StopClaiming stops the claim loop (no new tasks start on this replica).
func (e *Engine) StopClaiming() {
	e.stopOnce.Do(func() { close(e.stop) })
	if !e.started.Load() {
		return
	}
	select {
	case <-e.loopDone:
	case <-time.After(5 * time.Second):
	}
}

// Shutdown stops claiming, lets running tasks and verifications finish
// until ctx is done, then stops the remaining ones without recording a
// result and releases their leases so another replica re-claims them
// immediately (the re-claimed attempt keeps its idempotency key).
func (e *Engine) Shutdown(ctx context.Context) {
	e.StopClaiming()
	done := make(chan struct{})
	go func() {
		e.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return
	case <-ctx.Done():
	}
	e.mu.Lock()
	n := len(e.running)
	for _, rt := range e.running {
		rt.cancel(errShutdown)
	}
	for _, cancel := range e.verifying {
		cancel(errShutdown)
	}
	e.mu.Unlock()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
	}
	released, err := e.repo.ReleaseOwnerLeases(context.Background(), e.cfg.Owner)
	if err != nil {
		log.Printf("Orchestrator: releasing task leases at shutdown failed: %v", err)
		return
	}
	log.Printf("Orchestrator: shutdown interrupted %d running task(s); released %d lease(s) for re-claim", n, released)
}
