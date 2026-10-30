package services

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
	"gorm.io/gorm"
)

type InstanceService struct {
	db           *gorm.DB
	quotaService *QuotaService
	auditService *AuditService
	worker       *WorkerService
	proxmox      proxmox.ProxmoxClient
}

func NewInstanceService(db *gorm.DB, quotaService *QuotaService, auditService *AuditService, worker *WorkerService, pve proxmox.ProxmoxClient) *InstanceService {
	return &InstanceService{
		db:           db,
		quotaService: quotaService,
		auditService: auditService,
		worker:       worker,
		proxmox:      pve,
	}
}

type CreateInstanceInput struct {
	Name           string     `json:"name" binding:"required"`
	Hostname       string     `json:"hostname"`
	TemplateID     uuid.UUID  `json:"template_id" binding:"required"`
	PlanID         *uuid.UUID `json:"plan_id"`
	VCPUs          int        `json:"vcpus"`
	MemoryMB       int        `json:"memory_mb"`
	DiskGB         int        `json:"disk_gb"`
	SSHKeyID       *uuid.UUID `json:"ssh_key_id"`
	IdempotencyKey string     `json:"idempotency_key"`
}

type ResizeInstanceInput struct {
	VCPUs    int `json:"vcpus" binding:"required,min=1"`
	MemoryMB int `json:"memory_mb" binding:"required,min=512"`
	DiskGB   int `json:"disk_gb" binding:"required,min=10"`
}

type CreateSnapshotInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IncludesRAM bool   `json:"includes_ram"`
}

var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-\.]{1,62}[a-z0-9]$`)
var snapNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-]{1,40}$`)

func (s *InstanceService) List(userID uuid.UUID, isAdmin bool, statusFilter string) ([]models.Instance, error) {
	query := s.db.Model(&models.Instance{}).
		Preload("Node").
		Preload("OSTemplate").
		Where("deleted_at IS NULL")

	if !isAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if statusFilter != "" && statusFilter != "all" {
		query = query.Where("status = ?", statusFilter)
	}

	var instances []models.Instance
	err := query.Order("created_at desc").Find(&instances).Error
	return instances, err
}

func (s *InstanceService) Get(id uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.Instance, error) {
	var inst models.Instance
	query := s.db.Preload("Node").Preload("OSTemplate").Where("id = ? AND deleted_at IS NULL", id)
	if !isAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if err := query.First(&inst).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("instance not found or access denied")
		}
		return nil, err
	}
	return &inst, nil
}

func (s *InstanceService) Create(userID uuid.UUID, input CreateInstanceInput, clientIP string) (*models.ProvisioningJob, error) {
	input.Name = strings.TrimSpace(strings.ToLower(input.Name))
	if !nameRegex.MatchString(input.Name) {
		return nil, errors.New("instance name must be 3-64 lowercase alphanumeric chars or hyphens, starting and ending with alphanumeric")
	}

	if input.Hostname == "" {
		input.Hostname = input.Name
	}

	// 1. Check idempotency
	if input.IdempotencyKey != "" {
		var existingJob models.ProvisioningJob
		if err := s.db.Where("owner_id = ? AND idempotency_key = ?", userID, input.IdempotencyKey).First(&existingJob).Error; err == nil {
			return &existingJob, nil
		}
	} else {
		input.IdempotencyKey = uuid.New().String()
	}

	// 2. Resolve template
	var template models.OSTemplate
	if err := s.db.First(&template, "id = ? AND enabled = true", input.TemplateID).Error; err != nil {
		return nil, errors.New("selected OS template is invalid or not available")
	}

	// 3. Resolve plan or custom resources
	if input.PlanID != nil {
		var plan models.ResourcePlan
		if err := s.db.First(&plan, "id = ? AND enabled = true", *input.PlanID).Error; err == nil {
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
		var key models.SSHKey
		if err := s.db.First(&key, "id = ? AND owner_id = ?", *input.SSHKeyID, userID).Error; err == nil {
			sshKey = &key
		}
	}

	// 5. Select default online node (Dell PowerEdge R640)
	var node models.Node
	if err := s.db.Where("status = ?", "online").Order("created_at asc").First(&node).Error; err != nil {
		return nil, errors.New("no healthy Proxmox hypervisor nodes currently available for scheduling")
	}

	// 6. Transactional Quota Check & Reservation & Job Creation
	var job models.ProvisioningJob
	var instance models.Instance

	err := s.db.Transaction(func(tx *gorm.DB) error {
		job = models.ProvisioningJob{
			OwnerID:         userID,
			IdempotencyKey:  input.IdempotencyKey,
			OperationType:   models.OpCreate,
			Status:          models.JobStatusQueued,
			CurrentStep:     "queued",
			ProgressPercent: 5,
		}
		if err := tx.Create(&job).Error; err != nil {
			return err
		}

		// Reserve resources
		_, err := s.quotaService.CheckAndReserve(tx, userID, job.ID, input.VCPUs, input.MemoryMB, input.DiskGB)
		if err != nil {
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
		if err := tx.Create(&instance).Error; err != nil {
			return err
		}

		// Link instance to job
		job.InstanceID = &instance.ID
		return tx.Model(&job).Update("instance_id", instance.ID).Error
	})

	if err != nil {
		s.auditService.Log(&userID, "instance.create", "instance", nil, "failure", clientIP, map[string]interface{}{
			"error": err.Error(),
			"name":  input.Name,
		})
		return nil, err
	}

	s.auditService.Log(&userID, "instance.create", "instance", &instance.ID, "success", clientIP, map[string]interface{}{
		"job_id": job.ID,
		"name":   input.Name,
		"plan":   input.PlanID,
	})

	// 7. Enqueue worker execution
	go s.worker.ProcessJob(job.ID, sshKey)

	return &job, nil
}

func (s *InstanceService) Resize(instanceID, userID uuid.UUID, isAdmin bool, input ResizeInstanceInput, clientIP string) error {
	inst, err := s.Get(instanceID, userID, isAdmin)
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
		usage, err := s.quotaService.GetUsage(userID)
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Update Proxmox configuration
	if inst.Node != nil && inst.ProxmoxVMID > 0 {
		vmCfg := proxmox.VMConfig{
			Cores:    input.VCPUs,
			MemoryMB: input.MemoryMB,
		}
		if err := s.proxmox.ConfigVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, vmCfg); err != nil {
			return fmt.Errorf("hypervisor hardware update failed: %w", err)
		}

		if deltaDisk > 0 {
			if err := s.proxmox.ResizeDisk(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, "scsi0", deltaDisk); err != nil {
				return fmt.Errorf("hypervisor disk expansion failed: %w", err)
			}
		}
	}

	// Update database record
	inst.VCPUs = input.VCPUs
	inst.MemoryMB = input.MemoryMB
	inst.DiskGB = input.DiskGB
	if err := s.db.Save(inst).Error; err != nil {
		return err
	}

	s.auditService.Log(&userID, "instance.resize", "instance", &inst.ID, "success", clientIP, map[string]interface{}{
		"vcpus":     input.VCPUs,
		"memory_mb": input.MemoryMB,
		"disk_gb":   input.DiskGB,
	})

	return nil
}

func (s *InstanceService) CreateSnapshot(instanceID, userID uuid.UUID, isAdmin bool, input CreateSnapshotInput, clientIP string) (*models.InstanceSnapshot, error) {
	inst, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	input.Name = strings.TrimSpace(input.Name)
	if !snapNameRegex.MatchString(input.Name) {
		return nil, errors.New("snapshot name must be 1-40 alphanumeric characters, hyphens, or underscores")
	}

	// Check duplicate snapshot name
	var count int64
	s.db.Model(&models.InstanceSnapshot{}).Where("instance_id = ? AND name = ?", inst.ID, input.Name).Count(&count)
	if count > 0 {
		return nil, errors.New("a snapshot with this name already exists for this instance")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if inst.Node != nil && inst.ProxmoxVMID > 0 {
		upid, err := s.proxmox.CreateSnapshot(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, input.Name, input.Description, input.IncludesRAM)
		if err != nil {
			return nil, fmt.Errorf("hypervisor snapshot creation failed: %w", err)
		}
		if upid != "" {
			_ = s.proxmox.WaitForTask(ctx, inst.Node.ProxmoxNodeName, upid)
		}
	}

	snap := models.InstanceSnapshot{
		InstanceID:  inst.ID,
		Name:        input.Name,
		Description: input.Description,
		IncludesRAM: input.IncludesRAM,
		CreatedAt:   time.Now(),
	}

	if err := s.db.Create(&snap).Error; err != nil {
		return nil, err
	}

	s.auditService.Log(&userID, "instance.snapshot.create", "instance_snapshot", &snap.ID, "success", clientIP, map[string]string{
		"snapshot_name": snap.Name,
		"instance_name": inst.Name,
	})

	return &snap, nil
}

func (s *InstanceService) ListSnapshots(instanceID, userID uuid.UUID, isAdmin bool) ([]models.InstanceSnapshot, error) {
	_, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	var snapshots []models.InstanceSnapshot
	err = s.db.Where("instance_id = ?", instanceID).Order("created_at desc").Find(&snapshots).Error
	return snapshots, err
}

func (s *InstanceService) RestoreSnapshot(instanceID, userID uuid.UUID, isAdmin bool, snapName string, clientIP string) error {
	inst, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return err
	}

	var snap models.InstanceSnapshot
	if err := s.db.Where("instance_id = ? AND name = ?", inst.ID, snapName).First(&snap).Error; err != nil {
		return errors.New("snapshot not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if inst.Node != nil && inst.ProxmoxVMID > 0 {
		upid, err := s.proxmox.RollbackSnapshot(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, snapName)
		if err != nil {
			return fmt.Errorf("hypervisor snapshot rollback failed: %w", err)
		}
		if upid != "" {
			_ = s.proxmox.WaitForTask(ctx, inst.Node.ProxmoxNodeName, upid)
		}
	}

	s.auditService.Log(&userID, "instance.snapshot.restore", "instance", &inst.ID, "success", clientIP, map[string]string{
		"snapshot_name": snapName,
	})

	return nil
}

func (s *InstanceService) DeleteSnapshot(instanceID, userID uuid.UUID, isAdmin bool, snapName string, clientIP string) error {
	inst, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return err
	}

	var snap models.InstanceSnapshot
	if err := s.db.Where("instance_id = ? AND name = ?", inst.ID, snapName).First(&snap).Error; err != nil {
		return errors.New("snapshot not found")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if inst.Node != nil && inst.ProxmoxVMID > 0 {
		upid, err := s.proxmox.DeleteSnapshot(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID, snapName)
		if err != nil {
			return fmt.Errorf("hypervisor snapshot deletion failed: %w", err)
		}
		if upid != "" {
			_ = s.proxmox.WaitForTask(ctx, inst.Node.ProxmoxNodeName, upid)
		}
	}

	s.db.Delete(&snap)
	s.auditService.Log(&userID, "instance.snapshot.delete", "instance_snapshot", &snap.ID, "success", clientIP, map[string]string{
		"snapshot_name": snapName,
	})

	return nil
}

func (s *InstanceService) GetConsoleTicket(instanceID, userID uuid.UUID, isAdmin bool) (*proxmox.VNCProxyTicket, error) {
	inst, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if inst.Status != models.StateRunning {
		return nil, errors.New("instance must be running to establish interactive console connection")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if inst.Node == nil || inst.ProxmoxVMID <= 0 {
		return nil, errors.New("hypervisor placement not resolved for instance")
	}

	ticket, err := s.proxmox.GetVNCProxy(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
	if err != nil {
		return nil, fmt.Errorf("failed to issue hypervisor console authorization ticket: %w", err)
	}

	return ticket, nil
}

func (s *InstanceService) Start(instanceID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error) {
	inst, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if inst.Status == models.StateRunning {
		return nil, errors.New("instance is already running")
	}
	if inst.Status == models.StateProvisioning || inst.Status == models.StateStarting {
		return nil, errors.New("instance is currently transitioning state")
	}

	job := models.ProvisioningJob{
		OwnerID:         inst.OwnerID,
		InstanceID:      &inst.ID,
		IdempotencyKey:  uuid.New().String(),
		OperationType:   models.OpStart,
		Status:          models.JobStatusQueued,
		CurrentStep:     "starting_vm",
		ProgressPercent: 10,
	}
	if err := s.db.Create(&job).Error; err != nil {
		return nil, err
	}

	s.db.Model(inst).Update("status", models.StateStarting)
	s.auditService.Log(&userID, "instance.start", "instance", &inst.ID, "success", clientIP, nil)

	go s.worker.ProcessPowerJob(job.ID, models.OpStart)
	return &job, nil
}

func (s *InstanceService) Shutdown(instanceID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error) {
	inst, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if inst.Status != models.StateRunning {
		return nil, errors.New("only running instances can be gracefully shut down")
	}

	job := models.ProvisioningJob{
		OwnerID:         inst.OwnerID,
		InstanceID:      &inst.ID,
		IdempotencyKey:  uuid.New().String(),
		OperationType:   models.OpShutdown,
		Status:          models.JobStatusQueued,
		CurrentStep:     "stopping_vm",
		ProgressPercent: 10,
	}
	if err := s.db.Create(&job).Error; err != nil {
		return nil, err
	}

	s.db.Model(inst).Update("status", models.StateStopping)
	s.auditService.Log(&userID, "instance.shutdown", "instance", &inst.ID, "success", clientIP, nil)

	go s.worker.ProcessPowerJob(job.ID, models.OpShutdown)
	return &job, nil
}

func (s *InstanceService) Restart(instanceID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error) {
	inst, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if inst.Status != models.StateRunning {
		return nil, errors.New("instance must be running to perform a graceful restart")
	}

	job := models.ProvisioningJob{
		OwnerID:         inst.OwnerID,
		InstanceID:      &inst.ID,
		IdempotencyKey:  uuid.New().String(),
		OperationType:   models.OpRestart,
		Status:          models.JobStatusQueued,
		CurrentStep:     "restarting_vm",
		ProgressPercent: 10,
	}
	if err := s.db.Create(&job).Error; err != nil {
		return nil, err
	}

	s.db.Model(inst).Update("status", models.StateRestarting)
	s.auditService.Log(&userID, "instance.restart", "instance", &inst.ID, "success", clientIP, nil)

	go s.worker.ProcessPowerJob(job.ID, models.OpRestart)
	return &job, nil
}

func (s *InstanceService) Delete(instanceID, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error) {
	inst, err := s.Get(instanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if inst.Status == models.StateDeleting {
		return nil, errors.New("instance is already being deleted")
	}

	job := models.ProvisioningJob{
		OwnerID:         inst.OwnerID,
		InstanceID:      &inst.ID,
		IdempotencyKey:  uuid.New().String(),
		OperationType:   models.OpDelete,
		Status:          models.JobStatusQueued,
		CurrentStep:     "deleting_vm",
		ProgressPercent: 10,
	}
	if err := s.db.Create(&job).Error; err != nil {
		return nil, err
	}

	s.db.Model(inst).Update("status", models.StateDeleting)
	s.auditService.Log(&userID, "instance.delete", "instance", &inst.ID, "success", clientIP, nil)

	go s.worker.ProcessDeleteJob(job.ID)
	return &job, nil
}
