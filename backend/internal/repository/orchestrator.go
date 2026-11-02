package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ExecutionFilter narrows ListExecutions. Zero values do not filter.
type ExecutionFilter struct {
	ConversationID *uuid.UUID
	PlanID         *uuid.UUID
	Status         string
	Limit          int
}

// OrchestratorRepository persists the AI infrastructure orchestrator: plan
// revisions and approvals (on top of ai_resource_plans), executions, their
// task graphs and leases, execution events and resource relationships.
type OrchestratorRepository interface {
	// Plans
	// LockPlan returns one of the user's plans and locks its row until the
	// transaction in ctx ends.
	LockPlan(ctx context.Context, planID, userID uuid.UUID) (*models.AIResourcePlan, error)
	// FindPlan returns a plan regardless of owner (engine use).
	FindPlan(ctx context.Context, planID uuid.UUID) (*models.AIResourcePlan, error)
	UpdatePlan(ctx context.Context, planID uuid.UUID, fields map[string]any) error
	// ReplacePlanResources replaces the legacy resource rows of a plan.
	ReplacePlanResources(ctx context.Context, planID uuid.UUID, resources []models.AIPlanResource) error
	// SetPlanResourceResult records the outcome on the legacy resource row
	// named name.
	SetPlanResourceResult(ctx context.Context, planID uuid.UUID, name, status, provisionedID, errMsg string) error
	// ListPlansWithoutRevision returns plans that have no revision yet, with
	// their resources (oldest first).
	ListPlansWithoutRevision(ctx context.Context, limit int) ([]models.AIResourcePlan, error)

	// Revisions
	// CreateRevision inserts a revision; ErrDuplicate when the plan already
	// has this version.
	CreateRevision(ctx context.Context, rev *models.AIPlanRevision) error
	// CreateRevisionIfAbsent inserts a revision unless the plan already has
	// this version (backfill; never aborts the surrounding transaction).
	CreateRevisionIfAbsent(ctx context.Context, rev *models.AIPlanRevision) error
	FindRevision(ctx context.Context, planID uuid.UUID, version int) (*models.AIPlanRevision, error)
	// ListRevisions returns a plan's revisions, oldest first.
	ListRevisions(ctx context.Context, planID uuid.UUID) ([]models.AIPlanRevision, error)
	// UpdateRevisionValidation stores the latest policy verdict of a
	// revision (not part of its spec hash).
	UpdateRevisionValidation(ctx context.Context, id uuid.UUID, validation string) error

	// Approvals
	CreateApproval(ctx context.Context, approval *models.AIApproval) error
	FindApproval(ctx context.Context, id, planID uuid.UUID) (*models.AIApproval, error)
	// LatestApproval returns the newest approval of a plan.
	LatestApproval(ctx context.Context, planID uuid.UUID) (*models.AIApproval, error)
	// SupersedeApprovals marks the plan's unconsumed approvals superseded.
	SupersedeApprovals(ctx context.Context, planID uuid.UUID, at time.Time) error
	// RejectApprovals marks the plan's unconsumed approvals rejected.
	RejectApprovals(ctx context.Context, planID uuid.UUID) error
	// ConsumeApproval marks an approved, unexpired approval consumed by an
	// execution. It reports false when the approval was not in that state.
	ConsumeApproval(ctx context.Context, id, executionID uuid.UUID, at time.Time) (bool, error)
	SetApprovalStatus(ctx context.Context, id uuid.UUID, status string) error

	// Executions
	// LockUserExecutions serializes execution creation per user until the
	// transaction in ctx ends (transaction-scoped advisory lock).
	LockUserExecutions(ctx context.Context, userID uuid.UUID) error
	CountActiveExecutions(ctx context.Context, userID uuid.UUID) (int64, error)
	// CreateExecution inserts an execution; ErrDuplicate when the plan
	// already has an active execution (uq_ai_executions_plan_active).
	CreateExecution(ctx context.Context, exec *models.AIExecution) error
	CreateTasks(ctx context.Context, tasks []models.AIExecutionTask) error
	// FindExecution returns an execution; with userID set, executions of
	// other users are not found.
	FindExecution(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*models.AIExecution, error)
	// LockExecution returns an execution and locks its row until the
	// transaction in ctx ends.
	LockExecution(ctx context.Context, id uuid.UUID) (*models.AIExecution, error)
	UpdateExecution(ctx context.Context, id uuid.UUID, fields map[string]any) error
	// ListExecutions returns the user's executions, newest first.
	ListExecutions(ctx context.Context, userID uuid.UUID, filter ExecutionFilter) ([]models.AIExecution, error)
	// LatestExecution returns the newest execution of a plan.
	LatestExecution(ctx context.Context, planID uuid.UUID) (*models.AIExecution, error)
	// TaskCounts returns, per execution, the number of tasks per status.
	TaskCounts(ctx context.Context, executionIDs []uuid.UUID) (map[uuid.UUID]map[string]int, error)

	// Tasks
	// ListTasks returns an execution's tasks in plan order.
	ListTasks(ctx context.Context, executionID uuid.UUID) ([]models.AIExecutionTask, error)
	FindTask(ctx context.Context, executionID, taskID uuid.UUID) (*models.AIExecutionTask, error)
	// UpdateTask writes fields unconditionally (callers hold the execution
	// row lock).
	UpdateTask(ctx context.Context, id uuid.UUID, fields map[string]any) error
	// UpdateTasksStatus moves the execution's tasks with the given keys from
	// one of fromStatuses to status and returns the keys actually moved.
	UpdateTasksStatus(ctx context.Context, executionID uuid.UUID, keys []string, fromStatuses []string, status string, extra map[string]any) ([]string, error)
	// UpdateLeasedTask writes fields only while the task is running under
	// leaseToken (fencing); it reports whether the row was written.
	UpdateLeasedTask(ctx context.Context, id, leaseToken uuid.UUID, fields map[string]any) (bool, error)
	// ClaimTasks leases up to limit runnable tasks to owner: queued tasks,
	// retrying tasks whose backoff elapsed and running tasks whose lease
	// expired (crashed runner), of executions that are queued or running and
	// not being cancelled. Each claim gets a fresh lease token.
	ClaimTasks(ctx context.Context, owner string, limit int, lease time.Duration) ([]models.AIExecutionTask, error)
	// HeartbeatTask extends the lease of a task still held under leaseToken.
	// alive is false when the lease was lost; cancelRequested reports the
	// execution's cancel flag.
	HeartbeatTask(ctx context.Context, id, leaseToken uuid.UUID, lease time.Duration) (alive, cancelRequested bool, err error)
	// ReleaseTaskLease makes a running task immediately claimable again.
	ReleaseTaskLease(ctx context.Context, id, leaseToken uuid.UUID) error
	// ReleaseOwnerLeases makes every running task (and verifying execution)
	// leased by owner claimable again (graceful shutdown).
	ReleaseOwnerLeases(ctx context.Context, owner string) (int64, error)

	// Finalization and maintenance
	// ListStalledExecutions returns executions that need the engine's
	// attention without a claimable task: active ones whose tasks are all
	// finished, verifying ones whose lease expired, and cancelling ones with
	// orphaned running tasks.
	ListStalledExecutions(ctx context.Context, limit int) ([]uuid.UUID, error)
	// CancelOrphanedTasks cancels running tasks of a cancelling execution
	// whose lease expired.
	CancelOrphanedTasks(ctx context.Context, executionID uuid.UUID) ([]string, error)
	// ClaimExecutionLease leases a verifying execution to owner (or renews
	// the lease when leaseToken already holds it).
	ClaimExecutionLease(ctx context.Context, id uuid.UUID, owner string, leaseToken uuid.UUID, lease time.Duration) (bool, error)

	// Events
	// AppendEvent allocates the next sequence number of the execution and
	// inserts the event in one transaction.
	AppendEvent(ctx context.Context, event *models.AIExecutionEvent) error
	// ListEvents returns events with seq > after, oldest first.
	ListEvents(ctx context.Context, executionID uuid.UUID, after int64, limit int) ([]models.AIExecutionEvent, error)

	// Relationships
	// RecordRelationship inserts a relationship once (duplicates ignored).
	RecordRelationship(ctx context.Context, rel *models.ResourceRelationship) error
	ListRelationships(ctx context.Context, executionID uuid.UUID) ([]models.ResourceRelationship, error)
	// FindRelationship returns the relationship of a task with a resource
	// type and relation, if any.
	FindTaskRelationship(ctx context.Context, taskID uuid.UUID, relation, resourceType string) (*models.ResourceRelationship, error)
}

type orchestratorRepository struct{ base }

var _ OrchestratorRepository = (*orchestratorRepository)(nil)

func NewOrchestratorRepository(db *gorm.DB) OrchestratorRepository {
	return &orchestratorRepository{base{db}}
}

var activeTaskStatuses = []string{models.TaskQueued, models.TaskBlocked, models.TaskRunning, models.TaskRetrying}

// Plans

func (r *orchestratorRepository) LockPlan(ctx context.Context, planID, userID uuid.UUID) (*models.AIResourcePlan, error) {
	var plan models.AIResourcePlan
	err := r.conn(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		First(&plan, "id = ? AND user_id = ?", planID, userID).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &plan, nil
}

func (r *orchestratorRepository) FindPlan(ctx context.Context, planID uuid.UUID) (*models.AIResourcePlan, error) {
	var plan models.AIResourcePlan
	if err := r.conn(ctx).First(&plan, "id = ?", planID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &plan, nil
}

func (r *orchestratorRepository) UpdatePlan(ctx context.Context, planID uuid.UUID, fields map[string]any) error {
	return r.conn(ctx).Model(&models.AIResourcePlan{}).Where("id = ?", planID).Updates(fields).Error
}

func (r *orchestratorRepository) ReplacePlanResources(ctx context.Context, planID uuid.UUID, resources []models.AIPlanResource) error {
	return r.inTx(ctx, func(tx *gorm.DB) error {
		if err := tx.Where("plan_id = ?", planID).Delete(&models.AIPlanResource{}).Error; err != nil {
			return err
		}
		if len(resources) == 0 {
			return nil
		}
		return tx.Create(&resources).Error
	})
}

func (r *orchestratorRepository) SetPlanResourceResult(ctx context.Context, planID uuid.UUID, name, status, provisionedID, errMsg string) error {
	fields := map[string]any{"status": status}
	if provisionedID != "" {
		fields["provisioned_id"] = provisionedID
	}
	if errMsg != "" {
		fields["error_message"] = errMsg
	}
	return r.conn(ctx).Model(&models.AIPlanResource{}).
		Where("plan_id = ? AND resource_name = ?", planID, name).
		Updates(fields).Error
}

func (r *orchestratorRepository) ListPlansWithoutRevision(ctx context.Context, limit int) ([]models.AIResourcePlan, error) {
	var plans []models.AIResourcePlan
	err := r.conn(ctx).
		Preload("Resources", func(db *gorm.DB) *gorm.DB { return db.Order("execution_order asc, created_at asc") }).
		Where("NOT EXISTS (SELECT 1 FROM ai_plan_revisions rv WHERE rv.plan_id = ai_resource_plans.id)").
		Order("created_at asc").
		Limit(limit).
		Find(&plans).Error
	return plans, err
}

// Revisions

func (r *orchestratorRepository) CreateRevision(ctx context.Context, rev *models.AIPlanRevision) error {
	err := r.conn(ctx).Create(rev).Error
	if uniqueViolation(r.db, err) {
		return ErrDuplicate
	}
	return err
}

func (r *orchestratorRepository) CreateRevisionIfAbsent(ctx context.Context, rev *models.AIPlanRevision) error {
	return r.conn(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "plan_id"}, {Name: "version"}},
		DoNothing: true,
	}).Create(rev).Error
}

func (r *orchestratorRepository) FindRevision(ctx context.Context, planID uuid.UUID, version int) (*models.AIPlanRevision, error) {
	var rev models.AIPlanRevision
	if err := r.conn(ctx).First(&rev, "plan_id = ? AND version = ?", planID, version).Error; err != nil {
		return nil, mapErr(err)
	}
	return &rev, nil
}

func (r *orchestratorRepository) ListRevisions(ctx context.Context, planID uuid.UUID) ([]models.AIPlanRevision, error) {
	var revs []models.AIPlanRevision
	err := r.conn(ctx).Where("plan_id = ?", planID).Order("version asc").Find(&revs).Error
	return revs, err
}

func (r *orchestratorRepository) UpdateRevisionValidation(ctx context.Context, id uuid.UUID, validation string) error {
	return r.conn(ctx).Model(&models.AIPlanRevision{}).Where("id = ?", id).Update("validation", validation).Error
}

// Approvals

func (r *orchestratorRepository) CreateApproval(ctx context.Context, approval *models.AIApproval) error {
	return r.conn(ctx).Create(approval).Error
}

func (r *orchestratorRepository) FindApproval(ctx context.Context, id, planID uuid.UUID) (*models.AIApproval, error) {
	var a models.AIApproval
	if err := r.conn(ctx).First(&a, "id = ? AND plan_id = ?", id, planID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &a, nil
}

func (r *orchestratorRepository) LatestApproval(ctx context.Context, planID uuid.UUID) (*models.AIApproval, error) {
	var a models.AIApproval
	if err := r.conn(ctx).Where("plan_id = ?", planID).Order("created_at desc").First(&a).Error; err != nil {
		return nil, mapErr(err)
	}
	return &a, nil
}

func (r *orchestratorRepository) SupersedeApprovals(ctx context.Context, planID uuid.UUID, at time.Time) error {
	return r.conn(ctx).Model(&models.AIApproval{}).
		Where("plan_id = ? AND status IN ?", planID, []string{models.ApprovalApproved, models.ApprovalPending}).
		Updates(map[string]any{"status": models.ApprovalSuperseded, "superseded_at": at}).Error
}

func (r *orchestratorRepository) RejectApprovals(ctx context.Context, planID uuid.UUID) error {
	return r.conn(ctx).Model(&models.AIApproval{}).
		Where("plan_id = ? AND status IN ?", planID, []string{models.ApprovalApproved, models.ApprovalPending}).
		Update("status", models.ApprovalRejected).Error
}

func (r *orchestratorRepository) ConsumeApproval(ctx context.Context, id, executionID uuid.UUID, at time.Time) (bool, error) {
	res := r.conn(ctx).Exec(`UPDATE ai_approvals
		SET status = ?, consumed = true, consumed_at = ?, execution_id = ?
		WHERE id = ? AND status = ? AND consumed = false AND expires_at > ?`,
		models.ApprovalConsumed, at, executionID, id, models.ApprovalApproved, at)
	return res.RowsAffected == 1, res.Error
}

func (r *orchestratorRepository) SetApprovalStatus(ctx context.Context, id uuid.UUID, status string) error {
	return r.conn(ctx).Model(&models.AIApproval{}).Where("id = ?", id).Update("status", status).Error
}

// Executions

func (r *orchestratorRepository) LockUserExecutions(ctx context.Context, userID uuid.UUID) error {
	return r.conn(ctx).Exec("SELECT pg_advisory_xact_lock(hashtext(?))", "orch-exec:"+userID.String()).Error
}

func (r *orchestratorRepository) CountActiveExecutions(ctx context.Context, userID uuid.UUID) (int64, error) {
	var n int64
	err := r.conn(ctx).Model(&models.AIExecution{}).
		Where("user_id = ? AND status IN ?", userID, models.ActiveExecutionStatuses).
		Count(&n).Error
	return n, err
}

func (r *orchestratorRepository) CreateExecution(ctx context.Context, exec *models.AIExecution) error {
	err := r.conn(ctx).Create(exec).Error
	if uniqueViolation(r.db, err) {
		return ErrDuplicate
	}
	return err
}

func (r *orchestratorRepository) CreateTasks(ctx context.Context, tasks []models.AIExecutionTask) error {
	if len(tasks) == 0 {
		return nil
	}
	return r.conn(ctx).Create(&tasks).Error
}

func (r *orchestratorRepository) FindExecution(ctx context.Context, id uuid.UUID, userID *uuid.UUID) (*models.AIExecution, error) {
	var e models.AIExecution
	q := r.conn(ctx).Where("id = ?", id)
	if userID != nil {
		q = q.Where("user_id = ?", *userID)
	}
	if err := q.First(&e).Error; err != nil {
		return nil, mapErr(err)
	}
	return &e, nil
}

func (r *orchestratorRepository) LockExecution(ctx context.Context, id uuid.UUID) (*models.AIExecution, error) {
	var e models.AIExecution
	if err := r.conn(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).First(&e, "id = ?", id).Error; err != nil {
		return nil, mapErr(err)
	}
	return &e, nil
}

func (r *orchestratorRepository) UpdateExecution(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return r.conn(ctx).Model(&models.AIExecution{}).Where("id = ?", id).Updates(fields).Error
}

func (r *orchestratorRepository) ListExecutions(ctx context.Context, userID uuid.UUID, filter ExecutionFilter) ([]models.AIExecution, error) {
	q := r.conn(ctx).Where("user_id = ?", userID)
	if filter.ConversationID != nil {
		q = q.Where("conversation_id = ?", *filter.ConversationID)
	}
	if filter.PlanID != nil {
		q = q.Where("plan_id = ?", *filter.PlanID)
	}
	if filter.Status != "" {
		q = q.Where("status = ?", filter.Status)
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 100
	}
	var execs []models.AIExecution
	err := q.Order("created_at desc").Limit(limit).Find(&execs).Error
	return execs, err
}

func (r *orchestratorRepository) LatestExecution(ctx context.Context, planID uuid.UUID) (*models.AIExecution, error) {
	var e models.AIExecution
	if err := r.conn(ctx).Where("plan_id = ?", planID).Order("created_at desc").First(&e).Error; err != nil {
		return nil, mapErr(err)
	}
	return &e, nil
}

func (r *orchestratorRepository) TaskCounts(ctx context.Context, executionIDs []uuid.UUID) (map[uuid.UUID]map[string]int, error) {
	out := make(map[uuid.UUID]map[string]int, len(executionIDs))
	if len(executionIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		ExecutionID uuid.UUID
		Status      string
		N           int
	}
	err := r.conn(ctx).Model(&models.AIExecutionTask{}).
		Select("execution_id, status, count(*) AS n").
		Where("execution_id IN ?", executionIDs).
		Group("execution_id, status").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if out[row.ExecutionID] == nil {
			out[row.ExecutionID] = map[string]int{}
		}
		out[row.ExecutionID][row.Status] = row.N
	}
	return out, nil
}

// Tasks

func (r *orchestratorRepository) ListTasks(ctx context.Context, executionID uuid.UUID) ([]models.AIExecutionTask, error) {
	var tasks []models.AIExecutionTask
	err := r.conn(ctx).Where("execution_id = ?", executionID).Order("position asc").Find(&tasks).Error
	return tasks, err
}

func (r *orchestratorRepository) FindTask(ctx context.Context, executionID, taskID uuid.UUID) (*models.AIExecutionTask, error) {
	var t models.AIExecutionTask
	if err := r.conn(ctx).First(&t, "id = ? AND execution_id = ?", taskID, executionID).Error; err != nil {
		return nil, mapErr(err)
	}
	return &t, nil
}

func (r *orchestratorRepository) UpdateTask(ctx context.Context, id uuid.UUID, fields map[string]any) error {
	return r.conn(ctx).Model(&models.AIExecutionTask{}).Where("id = ?", id).Updates(fields).Error
}

func (r *orchestratorRepository) UpdateTasksStatus(ctx context.Context, executionID uuid.UUID, keys []string, fromStatuses []string, status string, extra map[string]any) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	fields := map[string]any{"status": status, "updated_at": time.Now()}
	for k, v := range extra {
		fields[k] = v
	}
	var moved []models.AIExecutionTask
	err := r.conn(ctx).Model(&moved).
		Clauses(clause.Returning{Columns: []clause.Column{{Name: "task_key"}}}).
		Where("execution_id = ? AND task_key IN ? AND status IN ?", executionID, keys, fromStatuses).
		Updates(fields).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(moved))
	for _, t := range moved {
		out = append(out, t.Key)
	}
	return out, nil
}

func (r *orchestratorRepository) UpdateLeasedTask(ctx context.Context, id, leaseToken uuid.UUID, fields map[string]any) (bool, error) {
	fields["updated_at"] = time.Now()
	res := r.conn(ctx).Model(&models.AIExecutionTask{}).
		Where("id = ? AND lease_token = ? AND status = ?", id, leaseToken, models.TaskRunning).
		Updates(fields)
	return res.RowsAffected == 1, res.Error
}

func (r *orchestratorRepository) ClaimTasks(ctx context.Context, owner string, limit int, lease time.Duration) ([]models.AIExecutionTask, error) {
	if limit <= 0 {
		return nil, nil
	}
	var claimed []models.AIExecutionTask
	err := r.conn(ctx).Raw(`
		WITH candidates AS (
			SELECT t.id
			FROM ai_execution_tasks t
			JOIN ai_executions e ON e.id = t.execution_id
			WHERE e.status IN (?, ?)
			  AND e.cancel_requested = false
			  AND (
			       t.status = ?
			    OR (t.status = ? AND t.next_attempt_at <= now())
			    OR (t.status = ? AND t.lease_expires_at < now())
			  )
			ORDER BY t.created_at, t.position
			LIMIT ?
			FOR UPDATE OF t SKIP LOCKED
		)
		UPDATE ai_execution_tasks t
		SET status = ?,
		    lease_owner = ?,
		    lease_token = gen_random_uuid(),
		    lease_expires_at = now() + make_interval(secs => ?),
		    heartbeat_at = now(),
		    claim_count = CASE WHEN t.status = ? THEN t.claim_count + 1 ELSE 1 END,
		    started_at = COALESCE(t.started_at, now()),
		    next_attempt_at = NULL,
		    updated_at = now()
		FROM candidates c
		WHERE t.id = c.id
		RETURNING t.*`,
		models.ExecQueued, models.ExecRunning,
		models.TaskQueued, models.TaskRetrying, models.TaskRunning,
		limit,
		models.TaskRunning, owner, lease.Seconds(), models.TaskRunning,
	).Scan(&claimed).Error
	return claimed, err
}

func (r *orchestratorRepository) HeartbeatTask(ctx context.Context, id, leaseToken uuid.UUID, lease time.Duration) (bool, bool, error) {
	var flags []bool
	err := r.conn(ctx).Raw(`
		UPDATE ai_execution_tasks t
		SET lease_expires_at = now() + make_interval(secs => ?), heartbeat_at = now()
		FROM ai_executions e
		WHERE t.id = ? AND t.lease_token = ? AND t.status = ? AND e.id = t.execution_id
		RETURNING e.cancel_requested`,
		lease.Seconds(), id, leaseToken, models.TaskRunning,
	).Scan(&flags).Error
	if err != nil {
		return true, false, err
	}
	if len(flags) == 0 {
		return false, false, nil
	}
	return true, flags[0], nil
}

func (r *orchestratorRepository) ReleaseTaskLease(ctx context.Context, id, leaseToken uuid.UUID) error {
	return r.conn(ctx).Exec(`UPDATE ai_execution_tasks SET lease_expires_at = now(), updated_at = now()
		WHERE id = ? AND lease_token = ? AND status = ?`, id, leaseToken, models.TaskRunning).Error
}

func (r *orchestratorRepository) ReleaseOwnerLeases(ctx context.Context, owner string) (int64, error) {
	res := r.conn(ctx).Exec(`UPDATE ai_execution_tasks SET lease_expires_at = now(), updated_at = now()
		WHERE lease_owner = ? AND status = ?`, owner, models.TaskRunning)
	if res.Error != nil {
		return 0, res.Error
	}
	// Verification leases of this owner too.
	if err := r.conn(ctx).Exec(`UPDATE ai_executions SET lease_expires_at = now(), updated_at = now()
		WHERE lease_owner = ? AND status = ?`, owner, models.ExecVerifying).Error; err != nil {
		return res.RowsAffected, err
	}
	return res.RowsAffected, nil
}

// Finalization and maintenance

func (r *orchestratorRepository) ListStalledExecutions(ctx context.Context, limit int) ([]uuid.UUID, error) {
	var ids []uuid.UUID
	err := r.conn(ctx).Raw(`
		SELECT e.id FROM ai_executions e
		WHERE (e.status IN (?, ?, ?) AND NOT EXISTS (
		         SELECT 1 FROM ai_execution_tasks t WHERE t.execution_id = e.id AND t.status IN ?))
		   OR (e.status = ? AND (e.lease_expires_at IS NULL OR e.lease_expires_at < now()))
		   OR (e.status = ? AND EXISTS (
		         SELECT 1 FROM ai_execution_tasks t WHERE t.execution_id = e.id AND t.status = ? AND t.lease_expires_at < now()))
		ORDER BY e.created_at
		LIMIT ?`,
		models.ExecQueued, models.ExecRunning, models.ExecCancelling, activeTaskStatuses,
		models.ExecVerifying,
		models.ExecCancelling, models.TaskRunning,
		limit,
	).Scan(&ids).Error
	return ids, err
}

func (r *orchestratorRepository) CancelOrphanedTasks(ctx context.Context, executionID uuid.UUID) ([]string, error) {
	var keys []string
	err := r.conn(ctx).Raw(`
		UPDATE ai_execution_tasks
		SET status = ?, finished_at = now(), lease_owner = NULL, lease_token = NULL, lease_expires_at = NULL, updated_at = now()
		WHERE execution_id = ? AND status = ? AND lease_expires_at < now()
		RETURNING task_key`,
		models.TaskCancelled, executionID, models.TaskRunning,
	).Scan(&keys).Error
	return keys, err
}

func (r *orchestratorRepository) ClaimExecutionLease(ctx context.Context, id uuid.UUID, owner string, leaseToken uuid.UUID, lease time.Duration) (bool, error) {
	res := r.conn(ctx).Exec(`
		UPDATE ai_executions
		SET lease_owner = ?, lease_token = ?, lease_expires_at = now() + make_interval(secs => ?), updated_at = now()
		WHERE id = ? AND status = ?
		  AND (lease_expires_at IS NULL OR lease_expires_at < now() OR lease_token = ?)`,
		owner, leaseToken, lease.Seconds(), id, models.ExecVerifying, leaseToken)
	return res.RowsAffected == 1, res.Error
}

// Events

func (r *orchestratorRepository) AppendEvent(ctx context.Context, event *models.AIExecutionEvent) error {
	return r.inTx(ctx, func(tx *gorm.DB) error {
		var seqs []int64
		// The row lock taken here is held until commit, so events commit in
		// seq order and a reader polling with ?after= never skips one.
		if err := tx.Raw(`UPDATE ai_executions SET last_event_seq = last_event_seq + 1, updated_at = now()
			WHERE id = ? RETURNING last_event_seq`, event.ExecutionID).Scan(&seqs).Error; err != nil {
			return err
		}
		if len(seqs) == 0 {
			return ErrNotFound
		}
		event.Seq = seqs[0]
		if event.CreatedAt.IsZero() {
			event.CreatedAt = time.Now()
		}
		return tx.Create(event).Error
	})
}

func (r *orchestratorRepository) ListEvents(ctx context.Context, executionID uuid.UUID, after int64, limit int) ([]models.AIExecutionEvent, error) {
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	var events []models.AIExecutionEvent
	err := r.conn(ctx).Where("execution_id = ? AND seq > ?", executionID, after).
		Order("seq asc").Limit(limit).Find(&events).Error
	return events, err
}

// Relationships

func (r *orchestratorRepository) RecordRelationship(ctx context.Context, rel *models.ResourceRelationship) error {
	return r.conn(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(rel).Error
}

func (r *orchestratorRepository) ListRelationships(ctx context.Context, executionID uuid.UUID) ([]models.ResourceRelationship, error) {
	var rels []models.ResourceRelationship
	err := r.conn(ctx).Where("execution_id = ?", executionID).Order("created_at asc").Find(&rels).Error
	return rels, err
}

func (r *orchestratorRepository) FindTaskRelationship(ctx context.Context, taskID uuid.UUID, relation, resourceType string) (*models.ResourceRelationship, error) {
	var rel models.ResourceRelationship
	err := r.conn(ctx).Where("task_id = ? AND relation = ? AND resource_type = ?", taskID, relation, resourceType).
		Order("created_at desc").First(&rel).Error
	if err != nil {
		return nil, mapErr(err)
	}
	return &rel, nil
}
