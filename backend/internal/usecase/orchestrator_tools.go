package usecase

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/repository"
)

// Tool implementations. Every tool is idempotent per attempt: the attempt's
// idempotency key (tc.IdempotencyKey) re-drives the same provisioning job, a
// resource created by an earlier run of the same task is adopted, and a new
// attempt starts only after the previous attempt's job is terminal and its
// leftovers are cleaned up. Outputs are whitelisted fields: never secrets
// (gateway tokens, RCON passwords, session tokens, encryption keys).

func (s *OrchestratorUsecase) bindTools() {
	bind := func(id string, fn func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error)) {
		s.reg.Bind(id, orchestrator.RunnerFunc(fn))
	}
	bind(orchestrator.ToolVPSList, s.toolVPSList)
	bind(orchestrator.ToolVPSGet, s.toolVPSGet)
	bind(orchestrator.ToolVPSStatus, s.toolVPSStatus)
	bind(orchestrator.ToolQuotaCheck, s.toolQuotaCheck)
	bind(orchestrator.ToolCatalogTemplates, s.toolCatalogTemplates)
	bind(orchestrator.ToolCatalogPlans, s.toolCatalogPlans)
	bind(orchestrator.ToolVPSWaitReady, s.toolVPSWaitReady)
	bind(orchestrator.ToolVPSVerify, s.toolVPSVerify)
	bind(orchestrator.ToolVPSCreate, s.toolVPSCreate)
	bind(orchestrator.ToolVPSStart, s.instanceOpTool(models.OpStart))
	bind(orchestrator.ToolVPSRestart, s.instanceOpTool(models.OpRestart))
	bind(orchestrator.ToolVPSStop, s.instanceOpTool(models.OpShutdown))
	bind(orchestrator.ToolVPSDelete, s.instanceOpTool(models.OpDelete))
	bind(orchestrator.ToolVPSResize, s.toolVPSResize)
	bind(orchestrator.ToolVPSSnapshotCreate, s.toolSnapshotCreate)
	bind(orchestrator.ToolVPSSnapshotRestore, s.toolSnapshotRestore)
	bind(orchestrator.ToolBucketCreate, s.toolBucketCreate)
	bind(orchestrator.ToolBucketDelete, s.toolBucketDelete)
	bind(orchestrator.ToolAppCreate, s.toolAppCreate)
	bind(orchestrator.ToolN8nCreate, s.toolN8nCreate)
	bind(orchestrator.ToolOpenClawCreate, s.toolOpenClawCreate)
	bind(orchestrator.ToolGameServerCreate, s.toolGameServerCreate)
	bind(orchestrator.ToolAndroidCreate, s.toolAndroidCreate)
	s.bindToolsExt()
}

var (
	urlPattern  = regexp.MustCompile(`https?://\S+`)
	ipv4Pattern = regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}(?::\d+)?\b`)
)

// sanitize removes URLs and addresses from messages that reach clients
// (hypervisor errors can name internal hosts).
func sanitize(msg string) string {
	msg = urlPattern.ReplaceAllString(msg, "[endpoint]")
	msg = ipv4Pattern.ReplaceAllString(msg, "[address]")
	return truncate(msg, 500)
}

// classify maps a service error to a task error: requests the platform
// rejects (validation, quota, state) are permanent, everything else is
// retried.
func classify(err error) *orchestrator.TaskError {
	var te *orchestrator.TaskError
	if errors.As(err, &te) {
		return te
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "quota") || strings.Contains(lower, "exceeds"):
		return orchestrator.Permanent("QUOTA_EXCEEDED", sanitize(msg))
	case strings.Contains(lower, "no healthy"):
		return orchestrator.Retryable("NO_CAPACITY", sanitize(msg))
	case strings.Contains(lower, "not found") || strings.Contains(lower, "access denied"):
		return orchestrator.Permanent("RESOURCE_NOT_FOUND", sanitize(msg))
	case strings.Contains(lower, "already taken") || strings.Contains(lower, "already exists"):
		return orchestrator.Permanent("NAME_TAKEN", sanitize(msg))
	case strings.Contains(lower, "invalid") || strings.Contains(lower, "must be") || strings.Contains(lower, "not available") ||
		strings.Contains(lower, "unsupported") || strings.Contains(lower, "required") || strings.Contains(lower, "not supported") ||
		strings.Contains(lower, "only running") || strings.Contains(lower, "already being"):
		return orchestrator.Permanent("REQUEST_REJECTED", sanitize(msg))
	}
	return orchestrator.Retryable("TOOL_ERROR", sanitize(msg))
}

func isJobTerminal(job *models.ProvisioningJob) bool {
	return job.Status == models.JobStatusCompleted || job.Status == models.JobStatusFailed
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// waitJob polls a provisioning job until it is terminal, reporting step
// changes as task progress.
func (s *OrchestratorUsecase) waitJob(ctx context.Context, tc *orchestrator.TaskContext, jobID uuid.UUID) (*models.ProvisioningJob, error) {
	lastStep := ""
	ticker := time.NewTicker(s.jobPoll)
	defer ticker.Stop()
	for {
		job, err := s.repos.Jobs.FindByID(context.Background(), jobID, &tc.UserID)
		if err == nil {
			if job.CurrentStep != lastStep && !isJobTerminal(job) {
				lastStep = job.CurrentStep
				tc.Progress(fmt.Sprintf("Provisioning job: %s (%d%%)", strings.ReplaceAll(job.CurrentStep, "_", " "), job.ProgressPercent),
					map[string]any{"job_id": job.ID.String(), "step": job.CurrentStep, "progress": job.ProgressPercent})
			}
			if isJobTerminal(job) {
				return job, nil
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-ticker.C:
		}
	}
}

// settlePreviousAttempts waits for the jobs of earlier attempts of this task
// to finish. It returns the completed job of an earlier attempt, if any
// (then the work is already done), and cleans up failed creates.
func (s *OrchestratorUsecase) settlePreviousAttempts(ctx context.Context, tc *orchestrator.TaskContext, cleanupFailedCreate bool) (*models.ProvisioningJob, error) {
	for a := 1; a < tc.Attempt; a++ {
		job, err := s.repos.Jobs.FindByIdempotencyKey(context.Background(), tc.UserID, tc.KeyForAttempt(a))
		if err != nil {
			continue
		}
		if !isJobTerminal(job) {
			tc.Progress(fmt.Sprintf("Waiting for attempt %d to finish before starting attempt %d", a, tc.Attempt), map[string]any{"job_id": job.ID.String()})
			if job, err = s.waitJob(ctx, tc, job.ID); err != nil {
				return nil, err
			}
		}
		if job.Status == models.JobStatusCompleted {
			return job, nil
		}
		if cleanupFailedCreate && job.InstanceID != nil {
			if err := s.svc.Instance.DiscardFailedInstance(context.Background(), *job.InstanceID, tc.UserID, tc.ClientIP); err != nil {
				return nil, orchestrator.Retryable("CLEANUP_FAILED", sanitize(err.Error()))
			}
			tc.RecordResource(models.RelationDeleted, orchestrator.TypeVPS, *job.InstanceID, "")
		}
	}
	return nil, nil
}

func vpsOutput(inst *models.Instance, job *models.ProvisioningJob) map[string]any {
	out := map[string]any{
		"instance_id": inst.ID.String(),
		"name":        inst.Name,
		"status":      inst.Status,
		"vcpus":       inst.VCPUs,
		"memory_mb":   inst.MemoryMB,
		"disk_gb":     inst.DiskGB,
		"private_ip":  nil,
	}
	if inst.PrivateIP != nil && *inst.PrivateIP != "" {
		out["private_ip"] = *inst.PrivateIP
		out["ssh"] = fmt.Sprintf("ssh %s@%s", inst.SSHUsername, *inst.PrivateIP)
	}
	if job != nil {
		out["job_id"] = job.ID.String()
	}
	return out
}

func (s *OrchestratorUsecase) toolVPSCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	in := tc.Input
	done, err := s.settlePreviousAttempts(ctx, tc, true)
	if err != nil {
		return nil, err
	}
	if done != nil && done.InstanceID != nil {
		if inst, err := s.repos.Instances.FindByID(bg, *done.InstanceID, &tc.UserID); err == nil && inst.Status != models.StateFailed {
			tc.RecordResource(models.RelationCreated, orchestrator.TypeVPS, inst.ID, inst.Name)
			return vpsOutput(inst, done), nil
		}
	}

	templateID, _ := orchestrator.UUID(in, "template_id")
	var planID *uuid.UUID
	if id, ok := orchestrator.UUID(in, "plan_id"); ok {
		planID = &id
	}
	var sshKey *uuid.UUID
	if keys := orchestrator.Strings(in, "ssh_key_ids"); len(keys) > 0 {
		if id, err := uuid.Parse(keys[0]); err == nil {
			sshKey = &id
		}
	}
	job, err := s.svc.Instance.Create(bg, tc.UserID, CreateInstanceInput{
		Name:           orchestrator.String(in, "name"),
		Hostname:       orchestrator.String(in, "hostname"),
		TemplateID:     templateID,
		PlanID:         planID,
		VCPUs:          orchestrator.Int(in, "vcpus", 0),
		MemoryMB:       orchestrator.Int(in, "memory_mb", 0),
		DiskGB:         orchestrator.Int(in, "disk_gb", 0),
		SSHKeyID:       sshKey,
		IdempotencyKey: tc.IdempotencyKey,
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	if job.InstanceID != nil {
		tc.RecordResource(models.RelationCreated, orchestrator.TypeVPS, *job.InstanceID, orchestrator.String(in, "name"))
	}
	tc.Progress("Provisioning job queued", map[string]any{"job_id": job.ID.String(), "resumed": tc.Resumed})
	job, err = s.waitJob(ctx, tc, job.ID)
	if err != nil {
		return nil, err
	}
	if job.Status == models.JobStatusFailed {
		code := deref(job.ErrorCode)
		if code == "" {
			code = "PROVISIONING_FAILED"
		}
		msg := sanitize(deref(job.ErrorMessage))
		// Leave nothing behind before a new attempt may start.
		if job.InstanceID != nil {
			if err := s.svc.Instance.DiscardFailedInstance(bg, *job.InstanceID, tc.UserID, tc.ClientIP); err == nil {
				tc.RecordResource(models.RelationDeleted, orchestrator.TypeVPS, *job.InstanceID, orchestrator.String(in, "name"))
			}
		}
		return nil, orchestrator.Retryable(code, msg)
	}
	if job.InstanceID == nil {
		return nil, orchestrator.Retryable("INVALID_STATE", "the provisioning job has no instance")
	}
	inst, err := s.repos.Instances.FindByID(bg, *job.InstanceID, &tc.UserID)
	if err != nil {
		return nil, orchestrator.Retryable("LOOKUP_FAILED", "the provisioned instance could not be read")
	}
	return vpsOutput(inst, job), nil
}

// instanceID returns the instance a task works on: its input, or the output
// of the task it depends on.
func instanceID(tc *orchestrator.TaskContext) (uuid.UUID, bool) {
	if id, ok := orchestrator.UUID(tc.Input, "instance_id"); ok {
		return id, true
	}
	for _, out := range tc.DependencyOutputs {
		if id, ok := orchestrator.UUID(out, "instance_id"); ok {
			return id, true
		}
	}
	return uuid.Nil, false
}

// readyIPWait is how long vps.wait_ready waits for a guest address once the
// instance runs.
const readyIPWait = 2 * time.Minute

func (s *OrchestratorUsecase) toolVPSWaitReady(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, ok := instanceID(tc)
	if !ok {
		return nil, orchestrator.Permanent("INPUT_INVALID", "no instance to wait for")
	}
	var runningSince time.Time
	reported := ""
	for {
		inst, err := s.repos.Instances.FindByID(context.Background(), id, &tc.UserID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, orchestrator.Permanent("RESOURCE_NOT_FOUND", "instance not found or access denied")
			}
		} else {
			if inst.Status != reported {
				reported = inst.Status
				tc.Progress(fmt.Sprintf("%s is %s", inst.Name, inst.Status), map[string]any{"status": inst.Status})
			}
			switch inst.Status {
			case models.StateFailed, models.StateDeleting, "deleted":
				return nil, orchestrator.Permanent("INSTANCE_FAILED", fmt.Sprintf("%s is %s", inst.Name, inst.Status))
			case models.StateRunning:
				if runningSince.IsZero() {
					runningSince = time.Now()
				}
				hasIP := inst.PrivateIP != nil && *inst.PrivateIP != ""
				if hasIP || time.Since(runningSince) > readyIPWait {
					out := vpsOutput(inst, nil)
					if !hasIP {
						out["warning"] = "the guest agent has not reported an address yet"
					}
					return out, nil
				}
			}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(2 * s.jobPoll):
		}
	}
}

func (s *OrchestratorUsecase) instanceOpTool(op string) func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	return func(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
		bg := context.Background()
		id, ok := instanceID(tc)
		if !ok {
			return nil, orchestrator.Permanent("INPUT_INVALID", "instance_id is required")
		}
		// Jobs of earlier attempts must be finished; a completed one means
		// the operation already happened.
		job, err := s.settlePreviousAttempts(ctx, tc, false)
		if err != nil {
			return nil, err
		}
		if job == nil {
			// A run of this same attempt may already have queued the job.
			if existing, err := s.repos.Jobs.FindByIdempotencyKey(bg, tc.UserID, tc.IdempotencyKey); err == nil {
				job = existing
			}
		}
		// Placement survives the soft delete (verification of a delete).
		var name, node string
		var vmid int
		if rec, err := s.repos.Instances.FindAnyByID(bg, id, &tc.UserID); err == nil {
			name, vmid = rec.Name, rec.ProxmoxVMID
			if rec.Node != nil {
				node = rec.Node.ProxmoxNodeName
			}
		}
		if job == nil {
			inst, findErr := s.repos.Instances.FindByID(bg, id, &tc.UserID)
			switch {
			case op == models.OpDelete && errors.Is(findErr, repository.ErrNotFound):
				return map[string]any{"instance_id": id.String(), "name": name, "status": "deleted", "already_absent": true}, nil
			case findErr != nil:
				return nil, classify(findErr)
			case op == models.OpStart && inst.Status == models.StateRunning,
				op == models.OpShutdown && inst.Status == models.StateStopped:
				// Desired state already reached.
				out := vpsOutput(inst, nil)
				out["already"] = true
				return out, nil
			}
			if job, err = s.svc.Instance.QueueOperation(bg, id, tc.UserID, false, op, tc.IdempotencyKey, tc.ClientIP); err != nil {
				return nil, classify(err)
			}
			tc.Progress(fmt.Sprintf("%s job queued for %s", op, name), map[string]any{"job_id": job.ID.String()})
		}
		if !isJobTerminal(job) {
			if job, err = s.waitJob(ctx, tc, job.ID); err != nil {
				return nil, err
			}
		}
		if job.Status == models.JobStatusFailed {
			code := deref(job.ErrorCode)
			if code == "" {
				code = "OPERATION_FAILED"
			}
			return nil, orchestrator.Retryable(code, sanitize(deref(job.ErrorMessage)))
		}
		if op == models.OpDelete {
			tc.RecordResource(models.RelationDeleted, orchestrator.TypeVPS, id, name)
			out := map[string]any{"instance_id": id.String(), "name": name, "status": "deleted", "job_id": job.ID.String()}
			if vmid > 0 && node != "" {
				out["vmid"], out["node"] = vmid, node
			}
			return out, nil
		}
		fresh, err := s.repos.Instances.FindByID(bg, id, &tc.UserID)
		if err != nil {
			return nil, classify(err)
		}
		tc.RecordResource(models.RelationModified, orchestrator.TypeVPS, id, fresh.Name)
		return vpsOutput(fresh, job), nil
	}
}

func (s *OrchestratorUsecase) toolVPSResize(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	id, _ := orchestrator.UUID(tc.Input, "instance_id")
	want := ResizeInstanceInput{
		VCPUs:    orchestrator.Int(tc.Input, "vcpus", 0),
		MemoryMB: orchestrator.Int(tc.Input, "memory_mb", 0),
		DiskGB:   orchestrator.Int(tc.Input, "disk_gb", 0),
	}
	inst, err := s.repos.Instances.FindByID(bg, id, &tc.UserID)
	if err != nil {
		return nil, classify(err)
	}
	if inst.VCPUs != want.VCPUs || inst.MemoryMB != want.MemoryMB || inst.DiskGB != want.DiskGB {
		if err := s.svc.Instance.Resize(bg, id, tc.UserID, false, want, tc.ClientIP); err != nil {
			return nil, classify(err)
		}
		if inst, err = s.repos.Instances.FindByID(bg, id, &tc.UserID); err != nil {
			return nil, classify(err)
		}
	}
	tc.RecordResource(models.RelationModified, orchestrator.TypeVPS, id, inst.Name)
	return vpsOutput(inst, nil), nil
}

func (s *OrchestratorUsecase) toolSnapshotCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	id, _ := orchestrator.UUID(tc.Input, "instance_id")
	name := orchestrator.String(tc.Input, "snapshot_name")
	if snap, err := s.repos.Snapshots.FindByName(bg, id, name); err == nil {
		return map[string]any{"instance_id": id.String(), "snapshot_id": snap.ID.String(), "snapshot_name": snap.Name, "already": true}, nil
	}
	snap, err := s.svc.Instance.CreateSnapshot(bg, id, tc.UserID, false, CreateSnapshotInput{
		Name: name, Description: orchestrator.String(tc.Input, "description"), IncludesRAM: orchestrator.Bool(tc.Input, "include_ram"),
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	return map[string]any{"instance_id": id.String(), "snapshot_id": snap.ID.String(), "snapshot_name": snap.Name}, nil
}

func (s *OrchestratorUsecase) toolSnapshotRestore(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, _ := orchestrator.UUID(tc.Input, "instance_id")
	name := orchestrator.String(tc.Input, "snapshot_name")
	if err := s.svc.Instance.RestoreSnapshot(context.Background(), id, tc.UserID, false, name, tc.ClientIP); err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationModified, orchestrator.TypeVPS, id, orchestrator.String(tc.Input, "name"))
	return map[string]any{"instance_id": id.String(), "snapshot_name": name, "restored": true}, nil
}

// adopted returns the resource id this task already created (an earlier run
// of the same task that crashed before recording its result).
func (s *OrchestratorUsecase) adopted(tc *orchestrator.TaskContext, resourceType string) (uuid.UUID, bool) {
	if tc.TaskID == uuid.Nil {
		return uuid.Nil, false
	}
	rel, err := s.repos.Orch.FindTaskRelationship(context.Background(), tc.TaskID, models.RelationCreated, resourceType)
	if err != nil {
		return uuid.Nil, false
	}
	return rel.ResourceID, true
}

// createdByThisExecution reports whether a resource found by name was
// created after the execution started (the crash window between creating a
// resource and recording it).
func createdByThisExecution(tc *orchestrator.TaskContext, createdAt time.Time) bool {
	return tc.TaskID != uuid.Nil && !tc.ExecutionCreatedAt.IsZero() && !createdAt.Before(tc.ExecutionCreatedAt.Add(-time.Second))
}

func bucketOutput(b *models.StorageBucket) map[string]any {
	return map[string]any{
		"bucket_id":  b.ID.String(),
		"name":       b.Name,
		"visibility": b.Visibility,
		"endpoint":   "https://s3.nevatal.id/" + b.Name,
	}
}

func (s *OrchestratorUsecase) toolBucketCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	name := strings.ToLower(strings.TrimSpace(orchestrator.String(tc.Input, "name")))
	if id, ok := s.adopted(tc, orchestrator.TypeBucket); ok {
		if b, err := s.repos.Storage.FindBucket(bg, id, tc.UserID); err == nil {
			return bucketOutput(b), nil
		}
	}
	if b, err := s.repos.Storage.FindBucketByName(bg, name); err == nil {
		if b.OwnerID == tc.UserID && createdByThisExecution(tc, b.CreatedAt) {
			tc.RecordResource(models.RelationCreated, orchestrator.TypeBucket, b.ID, b.Name)
			return bucketOutput(b), nil
		}
		return nil, orchestrator.Permanent("NAME_TAKEN", fmt.Sprintf("bucket name %q is already taken", name))
	}
	vis := "private"
	if orchestrator.Bool(tc.Input, "is_public") {
		vis = "public"
	}
	b, err := s.svc.Storage.CreateBucket(bg, tc.UserID, CreateBucketInput{Name: name, Visibility: vis, Description: orchestrator.String(tc.Input, "description")})
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationCreated, orchestrator.TypeBucket, b.ID, b.Name)
	return bucketOutput(b), nil
}

func (s *OrchestratorUsecase) toolBucketDelete(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	id, _ := orchestrator.UUID(tc.Input, "bucket_id")
	b, err := s.repos.Storage.FindBucket(bg, id, tc.UserID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return map[string]any{"bucket_id": id.String(), "status": "deleted", "already_absent": true}, nil
		}
		return nil, classify(err)
	}
	if err := s.svc.Storage.DeleteBucket(bg, id, tc.UserID); err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationDeleted, orchestrator.TypeBucket, b.ID, b.Name)
	return map[string]any{"bucket_id": b.ID.String(), "name": b.Name, "status": "deleted"}, nil
}

func (s *OrchestratorUsecase) toolAppCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	out := func(a *models.Application) map[string]any {
		return map[string]any{"application_id": a.ID.String(), "name": a.Name, "status": a.Status, "url": "https://" + a.AssignedSubdomain}
	}
	name := orchestrator.String(tc.Input, "name")
	if id, ok := s.adopted(tc, orchestrator.TypeApp); ok {
		if a, err := s.svc.App.Get(bg, id, tc.UserID, false); err == nil {
			return out(a), nil
		}
	}
	if list, err := s.svc.App.List(bg, tc.UserID, false); err == nil {
		for i := range list {
			if list[i].Name == name && createdByThisExecution(tc, list[i].CreatedAt) {
				tc.RecordResource(models.RelationCreated, orchestrator.TypeApp, list[i].ID, name)
				return out(&list[i]), nil
			}
		}
	}
	app, dep, err := s.svc.App.Create(bg, tc.UserID, CreateAppInput{
		Name: name, ApplicationType: orchestrator.String(tc.Input, "framework"), DeploymentSourceType: "git",
		RepositoryURL: orchestrator.String(tc.Input, "repository_url"), ProductionBranch: orchestrator.String(tc.Input, "branch"),
		VCPUs: 1, MemoryMB: 1024, DiskGB: 10,
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationCreated, orchestrator.TypeApp, app.ID, app.Name)
	s.linkAppDependencies(bg, tc, app)
	result := out(app)
	if dep != nil {
		result["deployment_id"] = dep.ID.String()
	}
	return result, nil
}

func (s *OrchestratorUsecase) toolN8nCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	out := func(a *models.AutomationInstance) map[string]any {
		return map[string]any{"automation_id": a.ID.String(), "name": a.Name, "status": a.Status, "url": a.DashboardURL}
	}
	name := orchestrator.String(tc.Input, "name")
	if id, ok := s.adopted(tc, orchestrator.TypeN8n); ok {
		if a, err := s.svc.Automation.Get(bg, id, tc.UserID, false); err == nil {
			return out(a), nil
		}
	}
	if list, err := s.svc.Automation.List(bg, tc.UserID, false); err == nil {
		for i := range list {
			if list[i].Name == name && createdByThisExecution(tc, list[i].CreatedAt) {
				tc.RecordResource(models.RelationCreated, orchestrator.TypeN8n, list[i].ID, name)
				return out(&list[i]), nil
			}
		}
	}
	inst, err := s.svc.Automation.Create(bg, tc.UserID, CreateAutomationInput{
		Name: name, N8nVersion: "latest", CPULimit: 1, MemoryLimitMB: 1024, StorageLimitGB: 15,
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationCreated, orchestrator.TypeN8n, inst.ID, inst.Name)
	s.linkHosted(tc, orchestrator.TypeN8n, inst.ID, inst.Name)
	return out(inst), nil
}

func (s *OrchestratorUsecase) toolOpenClawCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	out := func(a *models.OpenClawInstance) map[string]any {
		return map[string]any{"agent_id": a.ID.String(), "name": a.Name, "status": a.Status,
			"provider": a.AIProvider, "model": a.DefaultModel, "gateway_endpoint": a.GatewayEndpoint}
	}
	name := orchestrator.String(tc.Input, "name")
	if id, ok := s.adopted(tc, orchestrator.TypeOpenClaw); ok {
		if a, err := s.svc.OpenClaw.Get(bg, id, tc.UserID, false); err == nil {
			return out(a), nil
		}
	}
	if list, err := s.svc.OpenClaw.List(bg, tc.UserID, false); err == nil {
		for i := range list {
			if list[i].Name == name && createdByThisExecution(tc, list[i].CreatedAt) {
				tc.RecordResource(models.RelationCreated, orchestrator.TypeOpenClaw, list[i].ID, name)
				return out(&list[i]), nil
			}
		}
	}
	inst, err := s.svc.OpenClaw.Create(bg, tc.UserID, CreateOpenClawInput{
		Name: name, Version: "latest", AIProvider: orchestrator.String(tc.Input, "provider"),
		DefaultModel: orchestrator.String(tc.Input, "model"), CPULimit: 1, MemoryLimitMB: 1024, StorageLimitGB: 10,
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationCreated, orchestrator.TypeOpenClaw, inst.ID, inst.Name)
	s.linkHosted(tc, orchestrator.TypeOpenClaw, inst.ID, inst.Name)
	return out(inst), nil
}

func (s *OrchestratorUsecase) toolGameServerCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	out := func(g *models.GameServer) map[string]any {
		return map[string]any{"game_server_id": g.ID.String(), "name": g.Name, "status": g.Status,
			"game_type": g.GameType, "public_address": g.PublicAddress}
	}
	in := tc.Input
	name := orchestrator.String(in, "name")
	if id, ok := s.adopted(tc, orchestrator.TypeGameServer); ok {
		if g, err := s.svc.GameServer.Get(bg, id, tc.UserID, false); err == nil {
			return out(g), nil
		}
	}
	if list, err := s.svc.GameServer.List(bg, tc.UserID, false, ""); err == nil {
		for i := range list {
			if list[i].Name == name && createdByThisExecution(tc, list[i].CreatedAt) {
				tc.RecordResource(models.RelationCreated, orchestrator.TypeGameServer, list[i].ID, name)
				return out(&list[i]), nil
			}
		}
	}
	gs, err := s.svc.GameServer.Create(bg, tc.UserID, CreateGameServerInput{
		Name: name, GameType: orchestrator.String(in, "game_type"), GameVersion: orchestrator.String(in, "game_version"),
		ServerSoftware: orchestrator.String(in, "server_software"), PlanName: orchestrator.String(in, "plan_name"),
		CPUCores: orchestrator.Int(in, "cpu_cores", 0), MemoryMB: orchestrator.Int(in, "memory_mb", 0),
		StorageGB: orchestrator.Int(in, "storage_gb", 0), PlayerSlots: orchestrator.Int(in, "player_slots", 0),
		MOTD: orchestrator.String(in, "motd"), Difficulty: orchestrator.String(in, "difficulty"), WorldSize: orchestrator.String(in, "world_size"),
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationCreated, orchestrator.TypeGameServer, gs.ID, gs.Name)
	s.linkHosted(tc, orchestrator.TypeGameServer, gs.ID, gs.Name)
	return out(gs), nil
}

func (s *OrchestratorUsecase) toolAndroidCreate(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	bg := context.Background()
	out := func(a *models.AndroidInstance) map[string]any {
		// No access URL (it embeds a session token) and no ADB key.
		return map[string]any{"android_instance_id": a.ID.String(), "name": a.Name, "status": a.Status,
			"vcpus": a.VCPUs, "memory_mb": a.MemoryMB, "storage_gb": a.StorageGB, "adb_enabled": a.ADBEnabled}
	}
	in := tc.Input
	name := orchestrator.String(in, "name")
	if id, ok := s.adopted(tc, orchestrator.TypeAndroid); ok {
		if a, err := s.svc.Android.Get(bg, id, tc.UserID, false); err == nil {
			return out(a), nil
		}
	}
	if list, err := s.svc.Android.List(bg, tc.UserID, false, ""); err == nil {
		for i := range list {
			if list[i].Name == name && createdByThisExecution(tc, list[i].CreatedAt) {
				tc.RecordResource(models.RelationCreated, orchestrator.TypeAndroid, list[i].ID, name)
				return out(&list[i]), nil
			}
		}
	}
	inst, err := s.svc.Android.Create(bg, tc.UserID, CreateAndroidInstanceInput{
		Name: name, ImageID: orchestrator.String(in, "image_id"), PlanName: orchestrator.String(in, "plan_name"),
		VCPUs: orchestrator.Int(in, "vcpus", 0), MemoryMB: orchestrator.Int(in, "memory_mb", 0), StorageGB: orchestrator.Int(in, "storage_gb", 0),
		DisplayResolution: orchestrator.String(in, "display_resolution"), DisplayDensity: orchestrator.Int(in, "display_density", 0),
		Orientation: orchestrator.String(in, "orientation"), ADBEnabled: orchestrator.Bool(in, "adb_enabled"),
	}, tc.ClientIP)
	if err != nil {
		return nil, classify(err)
	}
	tc.RecordResource(models.RelationCreated, orchestrator.TypeAndroid, inst.ID, inst.Name)
	s.linkHosted(tc, orchestrator.TypeAndroid, inst.ID, inst.Name)
	return out(inst), nil
}

// Read tools.

func (s *OrchestratorUsecase) toolQuotaCheck(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	usage, err := s.svc.Quota.GetUsage(context.Background(), tc.UserID)
	if err != nil {
		return nil, orchestrator.Retryable("LOOKUP_FAILED", "could not read the quota")
	}
	q := &orchestrator.QuotaSnapshot{
		MaxVCPUs: usage.MaxVCPUs, UsedVCPUs: usage.UsedVCPUs, MaxMemoryMB: usage.MaxMemoryMB, UsedMemoryMB: usage.UsedMemoryMB,
		MaxStorageGB: usage.MaxStorageGB, UsedStorageGB: usage.UsedStorageGB, MaxInstances: usage.MaxInstances, UsedInstances: usage.UsedInstances,
	}
	add := orchestrator.EstimatedResources{
		VCPUs: orchestrator.Int(tc.Input, "vcpus", 0), MemoryMB: orchestrator.Int(tc.Input, "memory_mb", 0),
		DiskGB: orchestrator.Int(tc.Input, "disk_gb", 0), Instances: orchestrator.Int(tc.Input, "instances", 0),
	}
	if msg := orchestrator.QuotaShortfall(q, add); msg != "" {
		return nil, orchestrator.Permanent("QUOTA_EXCEEDED", msg)
	}
	return map[string]any{
		"ok":                  true,
		"vcpus_available":     q.MaxVCPUs - q.UsedVCPUs,
		"memory_mb_available": q.MaxMemoryMB - q.UsedMemoryMB,
		"disk_gb_available":   q.MaxStorageGB - q.UsedStorageGB,
		"instances_available": q.MaxInstances - q.UsedInstances,
	}, nil
}

func (s *OrchestratorUsecase) toolCatalogTemplates(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	tpls, err := s.repos.Templates.ListEnabledByName(ctx)
	if err != nil {
		return nil, orchestrator.Retryable("LOOKUP_FAILED", "could not list templates")
	}
	items := make([]map[string]any, 0, len(tpls))
	for _, t := range tpls {
		items = append(items, map[string]any{"id": t.ID.String(), "name": t.Name, "distribution": t.Distribution, "version": t.Version})
	}
	return map[string]any{"templates": items}, nil
}

func (s *OrchestratorUsecase) toolCatalogPlans(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	plans, err := s.repos.Plans.ListEnabledBySortOrder(ctx)
	if err != nil {
		return nil, orchestrator.Retryable("LOOKUP_FAILED", "could not list plans")
	}
	items := make([]map[string]any, 0, len(plans))
	for _, p := range plans {
		items = append(items, map[string]any{"id": p.ID.String(), "name": p.Name, "vcpus": p.VCPUs, "memory_mb": p.MemoryMB, "disk_gb": p.DiskGB})
	}
	return map[string]any{"plans": items}, nil
}

func (s *OrchestratorUsecase) toolVPSList(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	list, err := s.svc.Instance.List(ctx, tc.UserID, false, "")
	if err != nil {
		return nil, orchestrator.Retryable("LOOKUP_FAILED", "could not list instances")
	}
	items := make([]map[string]any, 0, len(list))
	for i := range list {
		items = append(items, vpsOutput(&list[i], nil))
	}
	return map[string]any{"instances": items}, nil
}

func (s *OrchestratorUsecase) toolVPSGet(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, _ := orchestrator.UUID(tc.Input, "instance_id")
	inst, err := s.svc.Instance.Get(ctx, id, tc.UserID, false)
	if err != nil {
		return nil, classify(err)
	}
	out := vpsOutput(inst, nil)
	out["created_at"] = inst.CreatedAt.UTC().Format(time.RFC3339)
	if inst.OSTemplate != nil {
		out["template"] = inst.OSTemplate.Name
	}
	return out, nil
}

func (s *OrchestratorUsecase) toolVPSStatus(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, _ := orchestrator.UUID(tc.Input, "instance_id")
	inst, err := s.svc.Instance.Get(ctx, id, tc.UserID, false)
	if err != nil {
		return nil, classify(err)
	}
	out := vpsOutput(inst, nil)
	out["hypervisor_status"] = s.hypervisorStatus(ctx, inst)
	return out, nil
}

func (s *OrchestratorUsecase) toolVPSVerify(ctx context.Context, tc *orchestrator.TaskContext) (map[string]any, error) {
	id, _ := orchestrator.UUID(tc.Input, "instance_id")
	v := s.verifyVPS(ctx, tc.UserID, id, models.StateRunning, true)
	return map[string]any{"instance_id": id.String(), "verification": v}, nil
}

// hypervisorStatus is the VM power state seen by the hypervisor, "simulated"
// in mock mode or "unreachable".
func (s *OrchestratorUsecase) hypervisorStatus(ctx context.Context, inst *models.Instance) string {
	if s.cfg.ProxmoxMock {
		return "simulated"
	}
	if inst.Node == nil || inst.ProxmoxVMID <= 0 {
		return "unplaced"
	}
	pveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	st, err := s.pve.GetVMStatus(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
	if err != nil {
		return "unreachable"
	}
	return st.Status
}
