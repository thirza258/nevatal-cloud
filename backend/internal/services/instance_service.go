package services

import (
	"errors"
	"regexp"
	"strings"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type InstanceService struct {
	db           *gorm.DB
	quotaService *QuotaService
	auditService *AuditService
	worker       *WorkerService
}

func NewInstanceService(db *gorm.DB, quotaService *QuotaService, auditService *AuditService, worker *WorkerService) *InstanceService {
	return &InstanceService{
		db:           db,
		quotaService: quotaService,
		auditService: auditService,
		worker:       worker,
	}
}

type CreateInstanceInput struct {
	Name           string    `json:"name" binding:"required"`
	Hostname       string    `json:"hostname"`
	TemplateID     uuid.UUID `json:"template_id" binding:"required"`
	PlanID         *uuid.UUID `json:"plan_id"`
	VCPUs          int       `json:"vcpus"`
	MemoryMB       int       `json:"memory_mb"`
	DiskGB         int       `json:"disk_gb"`
	SSHKeyID       *uuid.UUID `json:"ssh_key_id"`
	IdempotencyKey string    `json:"idempotency_key"`
}

var nameRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9\-\.]{1,62}[a-z0-9]$`)

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
			ProxmoxVMID:  0, // Allocated by worker during clone
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
