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
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
)

// ErrInstanceNotFound is returned when an instance does not exist, is deleted
// or belongs to another tenant.
var ErrInstanceNotFound = errors.New("instance not found or access denied")

// InstanceRepos are the repositories InstanceUsecase works with.
type InstanceRepos struct {
	Instances repository.InstanceRepository
	Jobs      repository.JobRepository
	Templates repository.TemplateRepository
	Plans     repository.PlanRepository
	SSHKeys   repository.SSHKeyRepository
	Nodes     repository.NodeRepository
	Snapshots repository.SnapshotRepository
	Audits    repository.AuditRepository
}

type InstanceUsecase struct {
	tx           repository.Transactor
	repos        InstanceRepos
	quotaService *QuotaUsecase
	auditService *AuditUsecase
	worker       *WorkerUsecase
	proxmox      proxmox.ProxmoxClient
}

func NewInstanceUsecase(tx repository.Transactor, repos InstanceRepos, quotaService *QuotaUsecase, auditService *AuditUsecase, worker *WorkerUsecase, pve proxmox.ProxmoxClient) *InstanceUsecase {
	return &InstanceUsecase{
		tx:           tx,
		repos:        repos,
		quotaService: quotaService,
		auditService: auditService,
		worker:       worker,
		proxmox:      pve,
	}
}

type CreateInstanceInput struct {
	Name           string     `json:"name"`
	Hostname       string     `json:"hostname"`
	TemplateID     uuid.UUID  `json:"template_id"`
	PlanID         *uuid.UUID `json:"plan_id"`
	VCPUs          int        `json:"vcpus"`
	MemoryMB       int        `json:"memory_mb"`
	DiskGB         int        `json:"disk_gb"`
	SSHKeyID       *uuid.UUID `json:"ssh_key_id"`
	IdempotencyKey string     `json:"idempotency_key"`
}

type ResizeInstanceInput struct {
	VCPUs    int `json:"vcpus"`
	MemoryMB int `json:"memory_mb"`
	DiskGB   int `json:"disk_gb"`
}

type CreateSnapshotInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	IncludesRAM bool   `json:"includes_ram"`
}

// InstanceMetrics is a live hypervisor reading. When Available is false,
// Reason explains why and Status is nil.
type InstanceMetrics struct {
	Available bool
	Reason    string
	Status    *proxmox.VMCurrentStatus
}

var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-\.]{1,62}[a-z0-9]$`)
var snapNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,40}$`)

func ownerScope(userID uuid.UUID, isAdmin bool) *uuid.UUID {
	if isAdmin {
		return nil
	}
	return &userID
}

func (s *InstanceUsecase) List(ctx context.Context, userID uuid.UUID, isAdmin bool, statusFilter string) ([]models.Instance, error) {
	if statusFilter == "all" {
		statusFilter = ""
	}
	return s.repos.Instances.ListWithPlacement(ctx, ownerScope(userID, isAdmin), statusFilter)
}

func (s *InstanceUsecase) Get(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Instance, error) {
	inst, err := s.repos.Instances.FindByID(ctx, id, ownerScope(userID, isAdmin))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrInstanceNotFound
		}
		return nil, err
	}
	return inst, nil
}

// GetActivity returns the 20 newest audit entries about an instance.
// Non-admins only see instances they own (deleted ones included, so owners
// keep their history); anything else is ErrInstanceNotFound.
func (s *InstanceUsecase) GetActivity(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) ([]models.AuditLog, error) {
	if !isAdmin {
		owned, err := s.repos.Instances.IsOwnedBy(ctx, id, userID)
		if err != nil || !owned {
			return nil, ErrInstanceNotFound
		}
	}
	logs, _ := s.repos.Audits.ListForInstance(ctx, id, 20)
	return logs, nil
}

// GetMetrics reads live CPU, memory, disk and network counters from the
// hypervisor. A stopped VM or an unreachable hypervisor is not an error.
func (s *InstanceUsecase) GetMetrics(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (*InstanceMetrics, error) {
	inst, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if inst.Status != models.StateRunning || inst.ProxmoxVMID <= 0 {
		return &InstanceMetrics{Reason: "Metrics are only available while the instance is running."}, nil
	}

	pveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	pveStatus, err := s.proxmox.GetVMStatus(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
	if err != nil {
		return &InstanceMetrics{Reason: "Unable to query Proxmox guest metrics at this time."}, nil
	}
	return &InstanceMetrics{Available: true, Status: pveStatus}, nil
}

func (s *InstanceUsecase) Create(ctx context.Context, userID uuid.UUID, input CreateInstanceInput, clientIP string) (*models.ProvisioningJob, error) {
	ctx = context.WithoutCancel(ctx)

	input.Name = strings.TrimSpace(strings.ToLower(input.Name))
	if !nameRegex.MatchString(input.Name) {
		return nil, errors.New("instance name must be 3-64 lowercase alphanumeric chars or hyphens, starting and ending with alphanumeric")
	}

	if input.Hostname == "" {
		input.Hostname = input.Name
	}

	// 1. Check idempotency. The lookup is only a shortcut: the unique index
	// on (owner_id, idempotency_key) is what makes a key create one job (see
	// the ErrDuplicate handling below).
	keyProvided := input.IdempotencyKey != ""
	if keyProvided {
		if existingJob, err := s.repos.Jobs.FindByIdempotencyKey(ctx, userID, input.IdempotencyKey); err == nil {
			return existingJob, nil
		}
	} else {
		input.IdempotencyKey = uuid.New().String()
	}

	// 2. Resolve template
	template, err := s.repos.Templates.FindEnabledByID(ctx, input.TemplateID)
	if err != nil {
		return nil, errors.New("selected OS template is invalid or not available")
	}

	// 3. Resolve plan or custom resources
	if input.PlanID != nil {
		if plan, err := s.repos.Plans.FindEnabledByID(ctx, *input.PlanID); err == nil {
			input.VCPUs = plan.VCPUs
			input.MemoryMB = plan.MemoryMB
			input.DiskGB = plan.DiskGB
		}
	}

	if input.VCPUs <= 0 || input.MemoryMB <= 0 || input.DiskGB <= 0 {
		return nil, errors.New("vCPU, RAM, and Disk specifications must be positive integers")
	}

	// 4. Resolve SSH key if provided
	var sshKey *models.SSHKey
	if input.SSHKeyID != nil {
		if key, err := s.repos.SSHKeys.FindByIDAndOwner(ctx, *input.SSHKeyID, userID); err == nil {
			sshKey = key
		}
	}

	// 5. Select default online node (Dell PowerEdge R640)
	node, err := s.repos.Nodes.FirstOnline(ctx)
	if err != nil {
		return nil, errors.New("no healthy Proxmox hypervisor nodes currently available for scheduling")
	}

	// 6. Transactional Quota Check & Reservation & Job Creation
	var job models.ProvisioningJob
	var instance models.Instance

	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		job = models.ProvisioningJob{
			OwnerID:         userID,
			IdempotencyKey:  input.IdempotencyKey,
			OperationType:   models.OpCreate,
			Status:          models.JobStatusQueued,
			CurrentStep:     "queued",
			ProgressPercent: 5,
		}
		if err := s.repos.Jobs.Create(txCtx, &job); err != nil {
			return err
		}

		// Reserve resources. Current usage is read with ctx, i.e. outside the
		// transaction, exactly as before the layered refactor.
		usage, err := s.quotaService.GetUsage(ctx, userID)
		if err != nil {
			return err
		}
		if _, err := s.quotaService.Reserve(txCtx, usage, userID, job.ID, input.VCPUs, input.MemoryMB, input.DiskGB); err != nil {
			return err
		}

		// Create instance shell record in queued state
		instance = models.Instance{
			OwnerID:      userID,
			NodeID:       node.ID,
			OSTemplateID: template.ID,
			ProxmoxVMID:  0,
			Name:         input.Name,
			Hostname:     input.Hostname,
			Status:       models.StateQueued,
			VCPUs:        input.VCPUs,
			MemoryMB:     input.MemoryMB,
			DiskGB:       input.DiskGB,
			SSHUsername:  template.DefaultUsername,
		}
		if err := s.repos.Instances.Create(txCtx, &instance); err != nil {
			return err
		}

		// Link instance to job
		job.InstanceID = &instance.ID
		return s.repos.Jobs.SetInstance(txCtx, &job, instance.ID)
	})

	if err != nil {
		if keyProvided && errors.Is(err, repository.ErrDuplicate) {
			// A concurrent request with the same key won the insert; its
			// transaction is committed, ours rolled back (no second
			// instance or reservation).
			if existingJob, findErr := s.repos.Jobs.FindByIdempotencyKey(ctx, userID, input.IdempotencyKey); findErr == nil {
				return existingJob, nil
			}
		}
		s.auditService.Log(ctx, &userID, "instance.create", "instance", nil, "failure", clientIP, map[string]interface{}{
			"error": err.Error(),
			"name":  input.Name,
		})
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "instance.create", "instance", &instance.ID, "success", clientIP, map[string]interface{}{
		"job_id": job.ID,
		"name":   input.Name,
		"plan":   input.PlanID,
	})

	// 7. Enqueue worker execution
	s.worker.Go(func() { s.worker.ProcessJob(job.ID, sshKey) })

	return &job, nil
}

func (s *InstanceUsecase) Resize(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, input ResizeInstanceInput, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return err
	}

	// PRD Section 10.6 Requirement: VM must be stopped before resizing
	if inst.Status != models.StateStopped {
		return errors.New("instance must be stopped before modifying hardware configuration")
	}

	// PRD Section 10.6: Disk shrinking is strictly prohibited
	if input.DiskGB < inst.DiskGB {
		return fmt.Errorf("shrinking disk storage is not supported (current: %d GB, requested: %d GB)", inst.DiskGB, input.DiskGB)
	}

	// Check resource quota deltas
	deltaVCPUs := input.VCPUs - inst.VCPUs
	deltaMemory := input.MemoryMB - inst.MemoryMB
	deltaDisk := input.DiskGB - inst.DiskGB

	if deltaVCPUs > 0 || deltaMemory > 0 || deltaDisk > 0 {
		usage, err := s.quotaService.GetUsage(ctx, userID)
		if err != nil {
			return err
		}

		if deltaVCPUs > 0 && usage.UsedVCPUs+deltaVCPUs > usage.MaxVCPUs {
			return fmt.Errorf("requested resize exceeds vCPU quota (%d needed, %d available)", deltaVCPUs, usage.MaxVCPUs-usage.UsedVCPUs)
		}
		if deltaMemory > 0 && usage.UsedMemoryMB+deltaMemory > usage.MaxMemoryMB {
			return fmt.Errorf("requested resize exceeds memory quota (%d MB needed, %d MB available)", deltaMemory, usage.MaxMemoryMB-usage.UsedMemoryMB)
		}
		if deltaDisk > 0 && usage.UsedStorageGB+deltaDisk > usage.MaxStorageGB {
			return fmt.Errorf("requested resize exceeds storage quota (%d GB needed, %d GB available)", deltaDisk, usage.MaxStorageGB-usage.UsedStorageGB)
		}
	}

	pveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Update Proxmox configuration
	if inst.Node != nil && inst.ProxmoxVMID > 0 {
		vmCfg := proxmox.VMConfig{
			Cores:    input.VCPUs,
			MemoryMB: input.MemoryMB,
		}
		if err := s.proxmox.ConfigVM(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, vmCfg); err != nil {
			return fmt.Errorf("hypervisor hardware update failed: %w", err)
		}

		if deltaDisk > 0 {
			if err := s.proxmox.ResizeDisk(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, "scsi0", deltaDisk); err != nil {
				return fmt.Errorf("hypervisor disk expansion failed: %w", err)
			}
		}
	}

	// Update database record
	inst.VCPUs = input.VCPUs
	inst.MemoryMB = input.MemoryMB
	inst.DiskGB = input.DiskGB
	if err := s.repos.Instances.Save(ctx, inst); err != nil {
		return err
	}

	s.auditService.Log(ctx, &userID, "instance.resize", "instance", &inst.ID, "success", clientIP, map[string]interface{}{
		"vcpus":     input.VCPUs,
		"memory_mb": input.MemoryMB,
		"disk_gb":   input.DiskGB,
	})

	return nil
}

func (s *InstanceUsecase) CreateSnapshot(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, input CreateSnapshotInput, clientIP string) (*models.InstanceSnapshot, error) {
	ctx = context.WithoutCancel(ctx)

	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	input.Name = strings.TrimSpace(input.Name)
	if !snapNameRegex.MatchString(input.Name) {
		return nil, errors.New("snapshot name must be 1-40 alphanumeric characters, hyphens, or underscores")
	}

	// Check duplicate snapshot name
	count, _ := s.repos.Snapshots.CountByName(ctx, inst.ID, input.Name)
	if count > 0 {
		return nil, errors.New("a snapshot with this name already exists for this instance")
	}

	pveCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if inst.Node != nil && inst.ProxmoxVMID > 0 {
		upid, err := s.proxmox.CreateSnapshot(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, input.Name, input.Description, input.IncludesRAM)
		if err != nil {
			return nil, fmt.Errorf("hypervisor snapshot creation failed: %w", err)
		}
		if upid != "" {
			_ = s.proxmox.WaitForTask(pveCtx, inst.Node.ProxmoxNodeName, upid)
		}
	}

	snap := models.InstanceSnapshot{
		InstanceID:  inst.ID,
		Name:        input.Name,
		Description: input.Description,
		IncludesRAM: input.IncludesRAM,
		CreatedAt:   time.Now(),
	}

	if err := s.repos.Snapshots.Create(ctx, &snap); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "instance.snapshot.create", "instance_snapshot", &snap.ID, "success", clientIP, map[string]string{
		"snapshot_name": snap.Name,
		"instance_name": inst.Name,
	})

	return &snap, nil
}

func (s *InstanceUsecase) ListSnapshots(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool) ([]models.InstanceSnapshot, error) {
	_, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	return s.repos.Snapshots.ListByInstance(ctx, instanceID)
}

func (s *InstanceUsecase) RestoreSnapshot(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, snapName string, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return err
	}

	if _, err := s.repos.Snapshots.FindByName(ctx, inst.ID, snapName); err != nil {
		return errors.New("snapshot not found")
	}

	pveCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if inst.Node != nil && inst.ProxmoxVMID > 0 {
		upid, err := s.proxmox.RollbackSnapshot(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, snapName)
		if err != nil {
			return fmt.Errorf("hypervisor snapshot rollback failed: %w", err)
		}
		if upid != "" {
			_ = s.proxmox.WaitForTask(pveCtx, inst.Node.ProxmoxNodeName, upid)
		}
	}

	s.auditService.Log(ctx, &userID, "instance.snapshot.restore", "instance", &inst.ID, "success", clientIP, map[string]string{
		"snapshot_name": snapName,
	})

	return nil
}

func (s *InstanceUsecase) DeleteSnapshot(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, snapName string, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return err
	}

	snap, err := s.repos.Snapshots.FindByName(ctx, inst.ID, snapName)
	if err != nil {
		return errors.New("snapshot not found")
	}

	pveCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if inst.Node != nil && inst.ProxmoxVMID > 0 {
		upid, err := s.proxmox.DeleteSnapshot(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, snapName)
		if err != nil {
			return fmt.Errorf("hypervisor snapshot deletion failed: %w", err)
		}
		if upid != "" {
			_ = s.proxmox.WaitForTask(pveCtx, inst.Node.ProxmoxNodeName, upid)
		}
	}

	_ = s.repos.Snapshots.Delete(ctx, snap)
	s.auditService.Log(ctx, &userID, "instance.snapshot.delete", "instance_snapshot", &snap.ID, "success", clientIP, map[string]string{
		"snapshot_name": snapName,
	})

	return nil
}

func (s *InstanceUsecase) GetConsoleTicket(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool) (*proxmox.VNCProxyTicket, error) {
	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if inst.Status != models.StateRunning {
		return nil, errors.New("instance must be running to establish interactive console connection")
	}

	pveCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if inst.Node == nil || inst.ProxmoxVMID <= 0 {
		return nil, errors.New("hypervisor placement not resolved for instance")
	}

	ticket, err := s.proxmox.GetVNCProxy(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
	if err != nil {
		return nil, fmt.Errorf("failed to issue hypervisor console authorization ticket: %w", err)
	}

	return ticket, nil
}

// queuePowerJob records a power or delete job for inst, moves the instance to
// transitionState and audits the action. The caller starts the worker. An
// empty key gets a random one.
func (s *InstanceUsecase) queuePowerJob(ctx context.Context, inst *models.Instance, userID uuid.UUID, op, step, transitionState, action, key, clientIP string) (*models.ProvisioningJob, error) {
	if key == "" {
		key = uuid.New().String()
	}
	job := models.ProvisioningJob{
		OwnerID:         inst.OwnerID,
		InstanceID:      &inst.ID,
		IdempotencyKey:  key,
		OperationType:   op,
		Status:          models.JobStatusQueued,
		CurrentStep:     step,
		ProgressPercent: 10,
	}
	if err := s.repos.Jobs.Create(ctx, &job); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			if existing, findErr := s.repos.Jobs.FindByIdempotencyKey(ctx, inst.OwnerID, key); findErr == nil {
				return existing, errDuplicateOperation
			}
		}
		return nil, err
	}

	_ = s.repos.Instances.UpdateStatus(ctx, inst, transitionState)
	s.auditService.Log(ctx, &userID, action, "instance", &inst.ID, "success", clientIP, nil)
	return &job, nil
}

// errDuplicateOperation tells queueOperation that another caller already
// queued the operation under the same idempotency key.
var errDuplicateOperation = errors.New("operation already queued")

// Instance operations run as provisioning jobs.
const (
	InstanceOpStart    = models.OpStart
	InstanceOpShutdown = models.OpShutdown
	InstanceOpRestart  = models.OpRestart
	InstanceOpDelete   = models.OpDelete
)

// checkOperation returns the error a user sees when op is not allowed in the
// instance's current state.
func checkOperation(inst *models.Instance, op string) error {
	switch op {
	case models.OpStart:
		if inst.Status == models.StateRunning {
			return errors.New("instance is already running")
		}
		if inst.Status == models.StateProvisioning || inst.Status == models.StateStarting {
			return errors.New("instance is currently transitioning state")
		}
	case models.OpShutdown:
		if inst.Status != models.StateRunning {
			return errors.New("only running instances can be gracefully shut down")
		}
	case models.OpRestart:
		if inst.Status != models.StateRunning {
			return errors.New("instance must be running to perform a graceful restart")
		}
	case models.OpDelete:
		if inst.Status == models.StateDeleting {
			return errors.New("instance is already being deleted")
		}
	default:
		return fmt.Errorf("unsupported instance operation %q", op)
	}
	return nil
}

// queueOperation checks op against the instance state, records its job under
// key (random when empty) and starts the worker.
func (s *InstanceUsecase) queueOperation(ctx context.Context, inst *models.Instance, userID uuid.UUID, op, key, clientIP string) (*models.ProvisioningJob, error) {
	if err := checkOperation(inst, op); err != nil {
		return nil, err
	}
	var step, transition, action string
	switch op {
	case models.OpStart:
		step, transition, action = "starting_vm", models.StateStarting, "instance.start"
	case models.OpShutdown:
		step, transition, action = "stopping_vm", models.StateStopping, "instance.shutdown"
	case models.OpRestart:
		step, transition, action = "restarting_vm", models.StateRestarting, "instance.restart"
	case models.OpDelete:
		step, transition, action = "deleting_vm", models.StateDeleting, "instance.delete"
	}
	job, err := s.queuePowerJob(ctx, inst, userID, op, step, transition, action, key, clientIP)
	if errors.Is(err, errDuplicateOperation) {
		return job, nil
	}
	if err != nil {
		return nil, err
	}
	jobID := job.ID
	if op == models.OpDelete {
		s.worker.Go(func() { s.worker.ProcessDeleteJob(jobID) })
	} else {
		s.worker.Go(func() { s.worker.ProcessPowerJob(jobID, op) })
	}
	return job, nil
}

// QueueOperation runs start, shutdown, restart or delete as a job with an
// idempotency key: a second call with the same key returns the job of the
// first call (whatever its state) instead of queuing another operation. Used
// by the AI orchestrator, whose tasks may be re-driven after a crash.
func (s *InstanceUsecase) QueueOperation(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, op, idempotencyKey, clientIP string) (*models.ProvisioningJob, error) {
	ctx = context.WithoutCancel(ctx)
	if idempotencyKey != "" {
		if job, err := s.repos.Jobs.FindByIdempotencyKey(ctx, userID, idempotencyKey); err == nil {
			if job.InstanceID == nil || *job.InstanceID != instanceID || job.OperationType != op {
				return nil, errors.New("idempotency key already used for another operation")
			}
			return job, nil
		}
	}
	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	return s.queueOperation(ctx, inst, userID, op, idempotencyKey, clientIP)
}

// DiscardFailedInstance removes the leftovers of a failed create: the VM (when
// its VMID is known) and the instance row (soft delete), so a failed
// orchestrator attempt neither shows up as an instance nor counts toward the
// quota. Instances that are not in state failed are left alone.
func (s *InstanceUsecase) DiscardFailedInstance(ctx context.Context, instanceID, userID uuid.UUID, clientIP string) error {
	ctx = context.WithoutCancel(ctx)
	inst, err := s.repos.Instances.FindByID(ctx, instanceID, &userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil // already gone
		}
		return err
	}
	if inst.Status != models.StateFailed {
		return fmt.Errorf("instance %s is %s, not failed", inst.Name, inst.Status)
	}
	if inst.ProxmoxVMID > 0 && inst.Node != nil {
		pveCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		_, _ = s.proxmox.StopVM(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		_, _ = s.proxmox.DeleteVM(pveCtx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		cancel()
	}
	inst.Status = "deleted"
	if err := s.repos.Instances.Save(ctx, inst); err != nil {
		return err
	}
	if err := s.repos.Instances.SoftDelete(ctx, inst); err != nil {
		return err
	}
	s.auditService.Log(ctx, &userID, "instance.discard_failed", "instance", &inst.ID, "success", clientIP, map[string]interface{}{
		"name": inst.Name,
	})
	return nil
}

func (s *InstanceUsecase) Start(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error) {
	ctx = context.WithoutCancel(ctx)

	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	return s.queueOperation(ctx, inst, userID, models.OpStart, "", clientIP)
}

func (s *InstanceUsecase) Shutdown(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error) {
	ctx = context.WithoutCancel(ctx)

	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	return s.queueOperation(ctx, inst, userID, models.OpShutdown, "", clientIP)
}

func (s *InstanceUsecase) Restart(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error) {
	ctx = context.WithoutCancel(ctx)

	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	return s.queueOperation(ctx, inst, userID, models.OpRestart, "", clientIP)
}

func (s *InstanceUsecase) Delete(ctx context.Context, instanceID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error) {
	ctx = context.WithoutCancel(ctx)

	inst, err := s.Get(ctx, instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}
	return s.queueOperation(ctx, inst, userID, models.OpDelete, "", clientIP)
}
