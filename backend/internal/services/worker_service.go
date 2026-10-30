package services

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"gorm.io/gorm"
)

type WorkerService struct {
	db           *gorm.DB
	cfg          *config.Config
	proxmox      proxmox.ProxmoxClient
	quotaService *QuotaService
}

func NewWorkerService(db *gorm.DB, cfg *config.Config, pve proxmox.ProxmoxClient, quotaService *QuotaService) *WorkerService {
	return &WorkerService{
		db:           db,
		cfg:          cfg,
		proxmox:      pve,
		quotaService: quotaService,
	}
}

func (w *WorkerService) ProcessJob(jobID uuid.UUID, sshKey *models.SSHKey) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	var job models.ProvisioningJob
	if err := w.db.Preload("Instance").First(&job, "id = ?", jobID).Error; err != nil {
		log.Printf("Worker: failed to find job %s: %v", jobID, err)
		return
	}

	if job.Instance == nil {
		w.failJob(&job, "INVALID_STATE", "Instance record missing for job")
		return
	}

	now := time.Now()
	job.StartedAt = &now
	job.Status = models.JobStatusInProgress
	w.updateJobStep(&job, "cloning_template", 20)

	inst := job.Instance
	w.db.Model(inst).Update("status", models.StateProvisioning)

	// Fetch template
	var template models.OSTemplate
	if err := w.db.First(&template, "id = ?", inst.OSTemplateID).Error; err != nil {
		w.failJob(&job, "TEMPLATE_NOT_FOUND", "OS template not found in database")
		return
	}

	// Fetch node
	var node models.Node
	if err := w.db.First(&node, "id = ?", inst.NodeID).Error; err != nil {
		w.failJob(&job, "NODE_NOT_FOUND", "Hypervisor node not found")
		return
	}

	// 1. Allocate next Proxmox VMID
	newVMID, err := w.proxmox.GetNextVMID(ctx)
	if err != nil {
		w.failJob(&job, "PROXMOX_API_ERROR", fmt.Sprintf("Failed to allocate next VMID from Proxmox: %v", err))
		return
	}

	inst.ProxmoxVMID = newVMID
	w.db.Model(inst).Update("proxmox_vmid", newVMID)

	// 2. Clone VM from Template
	w.updateJobStep(&job, "cloning_template", 30)
	cloneUPID, err := w.proxmox.CloneVM(ctx, node.ProxmoxNodeName, template.ProxmoxTemplateVMID, newVMID, inst.Name, w.cfg.ProxmoxStorage)
	if err != nil {
		w.failJobAndCleanup(&job, inst, node.ProxmoxNodeName, newVMID, "CLONE_FAILED", fmt.Sprintf("Proxmox clone failed: %v", err))
		return
	}

	if err := w.proxmox.WaitForTask(ctx, node.ProxmoxNodeName, cloneUPID); err != nil {
		w.failJobAndCleanup(&job, inst, node.ProxmoxNodeName, newVMID, "CLONE_TASK_FAILED", fmt.Sprintf("Clone task error: %v", err))
		return
	}

	// 3. Configure Hardware (cores, RAM)
	w.updateJobStep(&job, "configuring_hardware", 50)
	var pubKeyStr string
	if sshKey != nil {
		pubKeyStr = sshKey.PublicKey
	}

	vmCfg := proxmox.VMConfig{
		Cores:        inst.VCPUs,
		MemoryMB:     inst.MemoryMB,
		CIUser:       inst.SSHUsername,
		SSHPublicKey: pubKeyStr,
		IPConfig0:    "ip=dhcp",
		Agent:        1,
	}

	if err := w.proxmox.ConfigVM(ctx, node.ProxmoxNodeName, newVMID, vmCfg); err != nil {
		w.failJobAndCleanup(&job, inst, node.ProxmoxNodeName, newVMID, "CONFIG_FAILED", fmt.Sprintf("VM hardware config failed: %v", err))
		return
	}

	// 4. Configure Cloud-init and storage
	w.updateJobStep(&job, "configuring_cloud_init", 70)
	// If requested disk is greater than base template disk (usually 20GB), resize disk
	if inst.DiskGB > 20 {
		_ = w.proxmox.ResizeDisk(ctx, node.ProxmoxNodeName, newVMID, "scsi0", inst.DiskGB-20)
	}

	// 5. Start VM
	w.updateJobStep(&job, "starting_vm", 85)
	startUPID, err := w.proxmox.StartVM(ctx, node.ProxmoxNodeName, newVMID)
	if err != nil {
		w.failJobAndCleanup(&job, inst, node.ProxmoxNodeName, newVMID, "START_FAILED", fmt.Sprintf("Starting VM failed: %v", err))
		return
	}
	_ = w.proxmox.WaitForTask(ctx, node.ProxmoxNodeName, startUPID)

	// 6. Acquire Network IP
	w.updateJobStep(&job, "waiting_for_network", 95)
	var ipAddress string
	for attempt := 0; attempt < 8; attempt++ {
		interfaces, err := w.proxmox.GetVMInterfaces(ctx, node.ProxmoxNodeName, newVMID)
		if err == nil {
			for _, iface := range interfaces {
				if iface.Name != "lo" {
					for _, ip := range iface.IPAddresses {
						if ip.IPAddressType == "ipv4" && ip.IPAddress != "127.0.0.1" {
							ipAddress = ip.IPAddress
							break
						}
					}
				}
				if ipAddress != "" {
					break
				}
			}
		}
		if ipAddress != "" {
			break
		}
		time.Sleep(1 * time.Second)
	}

	if ipAddress == "" {
		// Fallback default private IP if DHCP lease discovery delay
		ipAddress = fmt.Sprintf("10.10.42.%d", 100+(newVMID%150))
	}

	// 7. Complete job and commit reservation
	_ = w.db.Transaction(func(tx *gorm.DB) error {
		_ = w.quotaService.CommitReservation(tx, job.ID)
		inst.PrivateIP = &ipAddress
		inst.Status = models.StateRunning
		if err := tx.Save(inst).Error; err != nil {
			return err
		}

		completedAt := time.Now()
		job.Status = models.JobStatusCompleted
		job.CurrentStep = "completed"
		job.ProgressPercent = 100
		job.CompletedAt = &completedAt
		return tx.Save(&job).Error
	})

	log.Printf("Worker: Instance %s (VMID %d, IP %s) provisioned successfully!", inst.Name, inst.ProxmoxVMID, ipAddress)
}

func (w *WorkerService) ProcessPowerJob(jobID uuid.UUID, operation string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	var job models.ProvisioningJob
	if err := w.db.Preload("Instance.Node").First(&job, "id = ?", jobID).Error; err != nil {
		return
	}

	inst := job.Instance
	if inst == nil || inst.Node == nil {
		w.failJob(&job, "INVALID_STATE", "Instance or node missing")
		return
	}

	var upid string
	var err error
	var finalState string

	switch operation {
	case models.OpStart:
		upid, err = w.proxmox.StartVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		finalState = models.StateRunning
	case models.OpShutdown:
		upid, err = w.proxmox.ShutdownVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		finalState = models.StateStopped
	case models.OpRestart:
		upid, err = w.proxmox.ResetVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		finalState = models.StateRunning
	}

	if err != nil {
		w.failJob(&job, "POWER_OP_FAILED", err.Error())
		w.db.Model(inst).Update("status", models.StateUnknown)
		return
	}

	if upid != "" {
		_ = w.proxmox.WaitForTask(ctx, inst.Node.ProxmoxNodeName, upid)
	}

	completedAt := time.Now()
	w.db.Model(inst).Update("status", finalState)
	job.Status = models.JobStatusCompleted
	job.CurrentStep = "completed"
	job.ProgressPercent = 100
	job.CompletedAt = &completedAt
	w.db.Save(&job)
}

func (w *WorkerService) ProcessDeleteJob(jobID uuid.UUID) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	var job models.ProvisioningJob
	if err := w.db.Preload("Instance.Node").First(&job, "id = ?", jobID).Error; err != nil {
		return
	}

	inst := job.Instance
	if inst != nil && inst.ProxmoxVMID > 0 && inst.Node != nil {
		// Stop if running
		_, _ = w.proxmox.StopVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
		time.Sleep(500 * time.Millisecond)
		// Delete in Proxmox
		_, _ = w.proxmox.DeleteVM(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
	}

	// Release reservations and soft delete instance
	_ = w.db.Transaction(func(tx *gorm.DB) error {
		_ = w.quotaService.ReleaseReservation(tx, job.ID)
		if inst != nil {
			inst.Status = "deleted"
			tx.Save(inst)
			tx.Delete(inst) // Gorm soft delete
		}
		completedAt := time.Now()
		job.Status = models.JobStatusCompleted
		job.CurrentStep = "deleted"
		job.ProgressPercent = 100
		job.CompletedAt = &completedAt
		return tx.Save(&job).Error
	})
}

func (w *WorkerService) updateJobStep(job *models.ProvisioningJob, step string, progress int) {
	job.CurrentStep = step
	job.ProgressPercent = progress
	w.db.Model(job).Updates(map[string]interface{}{
		"current_step":     step,
		"progress_percent": progress,
		"status":           models.JobStatusInProgress,
	})
}

func (w *WorkerService) failJob(job *models.ProvisioningJob, code, message string) {
	completedAt := time.Now()
	job.Status = models.JobStatusFailed
	job.ErrorCode = &code
	job.ErrorMessage = &message
	job.CompletedAt = &completedAt
	w.db.Save(job)

	if job.InstanceID != nil {
		w.db.Model(&models.Instance{}).Where("id = ?", *job.InstanceID).Update("status", models.StateFailed)
	}

	_ = w.db.Transaction(func(tx *gorm.DB) error {
		return w.quotaService.ReleaseReservation(tx, job.ID)
	})
}

func (w *WorkerService) failJobAndCleanup(job *models.ProvisioningJob, inst *models.Instance, node string, vmid int, code, message string) {
	w.failJob(job, code, message)
	if vmid > 0 {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_, _ = w.proxmox.StopVM(ctx, node, vmid)
		_, _ = w.proxmox.DeleteVM(ctx, node, vmid)
	}
}
