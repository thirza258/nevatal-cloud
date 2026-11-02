package usecase

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
)

// AdminRepos are the repositories AdminUsecase reads and writes.
type AdminRepos struct {
	Users     repository.UserRepository
	Instances repository.InstanceRepository
	Jobs      repository.JobRepository
	Nodes     repository.NodeRepository
	Templates repository.TemplateRepository
	Plans     repository.PlanRepository
}

// AdminUsecase implements the platform administration console: tenants and
// their quotas, all instances and provisioning jobs, hypervisor nodes, the
// audit trail and the VM catalog (templates and plans).
type AdminUsecase struct {
	repos        AdminRepos
	quotaService *QuotaUsecase
	auditService *AuditUsecase
	worker       *WorkerUsecase
	proxmox      proxmox.ProxmoxClient
}

func NewAdminUsecase(repos AdminRepos, quotaService *QuotaUsecase, auditService *AuditUsecase, worker *WorkerUsecase, pve proxmox.ProxmoxClient) *AdminUsecase {
	return &AdminUsecase{
		repos:        repos,
		quotaService: quotaService,
		auditService: auditService,
		worker:       worker,
		proxmox:      pve,
	}
}

var (
	errAdminUserNotFound  = newError(KindNotFound, "USER_NOT_FOUND", "User not found")
	errAdminJobNotFound   = newError(KindNotFound, "JOB_NOT_FOUND", "Job not found")
	errAdminJobNotFailed  = newError(KindInvalid, "INVALID_STATE", "Only failed jobs can be retried")
	errTemplateNotFound   = newError(KindNotFound, "NOT_FOUND", "Template not found")
	errResourcePlanAbsent = newError(KindNotFound, "NOT_FOUND", "Plan not found")
)

// PlatformOverview is the admin dashboard summary.
type PlatformOverview struct {
	TotalUsers       int64
	TotalInstances   int64
	RunningInstances int64
	StoppedInstances int64
	ActiveJobs       int64
	FailedJobs       int64
	Node             models.Node
	NodeStatus       *proxmox.NodeStatus
	Storage          []proxmox.StorageInfo
}

// UserWithUsage is a tenant with its current quota usage (nil when it could
// not be computed).
type UserWithUsage struct {
	User  models.User
	Usage *models.QuotaUsage
}

// NodeDetail is a hypervisor node with its live status and storage pools
// (nil when the hypervisor did not answer).
type NodeDetail struct {
	Node       models.Node
	LiveStatus *proxmox.NodeStatus
	Storage    []proxmox.StorageInfo
}

// UserQuotaInput replaces a tenant's limits. Its JSON form is recorded in the
// audit trail.
type UserQuotaInput struct {
	MaxVCPUs     int `json:"max_vcpus"`
	MaxMemoryMB  int `json:"max_memory_mb"`
	MaxStorageGB int `json:"max_storage_gb"`
	MaxInstances int `json:"max_instances"`
}

// CreateTemplateInput registers an OS template. Its JSON form is recorded in
// the audit trail.
type CreateTemplateInput struct {
	Name                string `json:"name"`
	Distribution        string `json:"distribution"`
	Version             string `json:"version"`
	Description         string `json:"description"`
	ProxmoxTemplateVMID int    `json:"proxmox_template_vmid"`
	Architecture        string `json:"architecture"`
	CloudInitSupported  *bool  `json:"cloud_init_supported"`
	DefaultUsername     string `json:"default_username"`
	Enabled             *bool  `json:"enabled"`
}

// UpdateTemplateInput changes the set fields of an OS template. Its JSON form
// is recorded in the audit trail.
type UpdateTemplateInput struct {
	Name                *string `json:"name"`
	Distribution        *string `json:"distribution"`
	Version             *string `json:"version"`
	Description         *string `json:"description"`
	ProxmoxTemplateVMID *int    `json:"proxmox_template_vmid"`
	Architecture        *string `json:"architecture"`
	CloudInitSupported  *bool   `json:"cloud_init_supported"`
	DefaultUsername     *string `json:"default_username"`
	Enabled             *bool   `json:"enabled"`
}

// CreatePlanInput registers a resource plan. Its JSON form is recorded in the
// audit trail.
type CreatePlanInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	VCPUs       int    `json:"vcpus"`
	MemoryMB    int    `json:"memory_mb"`
	DiskGB      int    `json:"disk_gb"`
	Enabled     *bool  `json:"enabled"`
}

// UpdatePlanInput changes the set fields of a resource plan. Its JSON form is
// recorded in the audit trail.
type UpdatePlanInput struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	VCPUs       *int    `json:"vcpus"`
	MemoryMB    *int    `json:"memory_mb"`
	DiskGB      *int    `json:"disk_gb"`
	Enabled     *bool   `json:"enabled"`
}

// IsValidUserStatus reports whether status can be assigned to an account.
func IsValidUserStatus(status string) bool {
	return status == models.UserStatusActive || status == models.UserStatusSuspended
}

// Overview counts users, instances and jobs and reads the primary node's
// live status. Failures count as zero / nil.
func (s *AdminUsecase) Overview(ctx context.Context) *PlatformOverview {
	var ov PlatformOverview
	ov.TotalUsers, _ = s.repos.Users.Count(ctx)
	ov.TotalInstances, _ = s.repos.Instances.CountActive(ctx)
	ov.RunningInstances, _ = s.repos.Instances.CountActiveByStatus(ctx, models.StateRunning)
	ov.StoppedInstances, _ = s.repos.Instances.CountActiveByStatus(ctx, models.StateStopped)
	ov.ActiveJobs, _ = s.repos.Jobs.CountByStatuses(ctx, []string{models.JobStatusQueued, models.JobStatusInProgress})
	ov.FailedJobs, _ = s.repos.Jobs.CountByStatus(ctx, models.JobStatusFailed)

	// Fetch primary node status
	node, _ := s.repos.Nodes.First(ctx)
	ov.Node = *node

	pveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	ov.NodeStatus, _ = s.proxmox.GetNodeStatus(pveCtx, node.ProxmoxNodeName)
	ov.Storage, _ = s.proxmox.GetStorage(pveCtx, node.ProxmoxNodeName)
	return &ov
}

// ListUsers returns every account (newest first) with quota and usage.
func (s *AdminUsecase) ListUsers(ctx context.Context) ([]UserWithUsage, error) {
	users, err := s.repos.Users.ListWithQuota(ctx)
	if err != nil {
		return nil, err
	}

	results := make([]UserWithUsage, len(users))
	for i, u := range users {
		usage, _ := s.quotaService.GetUsage(ctx, u.ID)
		results[i] = UserWithUsage{User: u, Usage: usage}
	}
	return results, nil
}

// UpdateUserStatus activates or suspends an account.
func (s *AdminUsecase) UpdateUserStatus(ctx context.Context, adminID, targetID uuid.UUID, status, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	rows, err := s.repos.Users.UpdateStatus(ctx, targetID, status)
	if err != nil || rows == 0 {
		return errAdminUserNotFound
	}

	s.auditService.Log(ctx, &adminID, "admin.user.status_update", "user", &targetID, "success", clientIP, map[string]string{"new_status": status})
	return nil
}

// UpdateUserQuotas replaces a tenant's limits.
func (s *AdminUsecase) UpdateUserQuotas(ctx context.Context, adminID, targetID uuid.UUID, input UserQuotaInput, clientIP string) (*models.ResourceQuota, error) {
	ctx = context.WithoutCancel(ctx)

	quota, err := s.quotaService.UpdateUserQuota(ctx, targetID, input.MaxVCPUs, input.MaxMemoryMB, input.MaxStorageGB, input.MaxInstances)
	if err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &adminID, "admin.quota.update", "resource_quota", &quota.ID, "success", clientIP, input)
	return quota, nil
}

// ListAllInstances returns every live instance with owner and placement.
func (s *AdminUsecase) ListAllInstances(ctx context.Context) ([]models.Instance, error) {
	return s.repos.Instances.ListAllWithOwner(ctx)
}

// ListJobs returns the 50 newest provisioning jobs.
func (s *AdminUsecase) ListJobs(ctx context.Context) ([]models.ProvisioningJob, error) {
	return s.repos.Jobs.ListRecentWithInstance(ctx, 50)
}

// RetryJob re-queues a failed provisioning job and runs it again in the
// background.
func (s *AdminUsecase) RetryJob(ctx context.Context, adminID, jobID uuid.UUID, clientIP string) (*models.ProvisioningJob, error) {
	ctx = context.WithoutCancel(ctx)

	job, err := s.repos.Jobs.FindByID(ctx, jobID, nil)
	if err != nil {
		return nil, errAdminJobNotFound
	}

	if job.Status != models.JobStatusFailed {
		return nil, errAdminJobNotFailed
	}

	job.Status = models.JobStatusQueued
	job.CurrentStep = "retry_requested"
	job.ErrorCode = nil
	job.ErrorMessage = nil
	_ = s.repos.Jobs.Save(ctx, job)

	s.auditService.Log(ctx, &adminID, "admin.job.retry", "provisioning_job", &jobID, "success", clientIP, nil)

	retryID := job.ID
	s.worker.Go(func() { s.worker.ProcessJob(retryID, nil) })
	return job, nil
}

// ListNodes returns every hypervisor node with live status and storage.
func (s *AdminUsecase) ListNodes(ctx context.Context) []NodeDetail {
	nodes, _ := s.repos.Nodes.List(ctx)

	pveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	details := make([]NodeDetail, len(nodes))
	for i, n := range nodes {
		live, _ := s.proxmox.GetNodeStatus(pveCtx, n.ProxmoxNodeName)
		storage, _ := s.proxmox.GetStorage(pveCtx, n.ProxmoxNodeName)
		details[i] = NodeDetail{Node: n, LiveStatus: live, Storage: storage}
	}
	return details
}

// ListAuditLogs returns the 100 newest platform-wide audit entries.
func (s *AdminUsecase) ListAuditLogs(ctx context.Context) ([]models.AuditLog, error) {
	return s.auditService.ListRecentActivity(ctx, nil, 100)
}

// OS Templates Management

func (s *AdminUsecase) ListTemplates(ctx context.Context) ([]models.OSTemplate, error) {
	return s.repos.Templates.ListAll(ctx)
}

func (s *AdminUsecase) CreateTemplate(ctx context.Context, adminID uuid.UUID, input CreateTemplateInput, clientIP string) (*models.OSTemplate, error) {
	ctx = context.WithoutCancel(ctx)

	tmpl := models.OSTemplate{
		Name:                input.Name,
		Distribution:        input.Distribution,
		Version:             input.Version,
		Description:         input.Description,
		ProxmoxTemplateVMID: input.ProxmoxTemplateVMID,
		Architecture:        "x86_64",
		CloudInitSupported:  true,
		DefaultUsername:     "ubuntu",
		Enabled:             true,
	}
	if input.Architecture != "" {
		tmpl.Architecture = input.Architecture
	}
	if input.CloudInitSupported != nil {
		tmpl.CloudInitSupported = *input.CloudInitSupported
	}
	if input.DefaultUsername != "" {
		tmpl.DefaultUsername = input.DefaultUsername
	}
	if input.Enabled != nil {
		tmpl.Enabled = *input.Enabled
	}

	if err := s.repos.Templates.Create(ctx, &tmpl); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &adminID, "admin.template.create", "os_template", &tmpl.ID, "success", clientIP, input)
	return &tmpl, nil
}

// FindTemplate returns a template or NOT_FOUND "Template not found".
func (s *AdminUsecase) FindTemplate(ctx context.Context, id uuid.UUID) (*models.OSTemplate, error) {
	tmpl, err := s.repos.Templates.FindByID(ctx, id)
	if err != nil {
		return nil, errTemplateNotFound
	}
	return tmpl, nil
}

// UpdateTemplate applies the set fields of input to tmpl (from FindTemplate).
func (s *AdminUsecase) UpdateTemplate(ctx context.Context, adminID uuid.UUID, tmpl *models.OSTemplate, input UpdateTemplateInput, clientIP string) (*models.OSTemplate, error) {
	ctx = context.WithoutCancel(ctx)

	if input.Name != nil {
		tmpl.Name = *input.Name
	}
	if input.Distribution != nil {
		tmpl.Distribution = *input.Distribution
	}
	if input.Version != nil {
		tmpl.Version = *input.Version
	}
	if input.Description != nil {
		tmpl.Description = *input.Description
	}
	if input.ProxmoxTemplateVMID != nil {
		tmpl.ProxmoxTemplateVMID = *input.ProxmoxTemplateVMID
	}
	if input.Architecture != nil {
		tmpl.Architecture = *input.Architecture
	}
	if input.CloudInitSupported != nil {
		tmpl.CloudInitSupported = *input.CloudInitSupported
	}
	if input.DefaultUsername != nil {
		tmpl.DefaultUsername = *input.DefaultUsername
	}
	if input.Enabled != nil {
		tmpl.Enabled = *input.Enabled
	}

	if err := s.repos.Templates.Save(ctx, tmpl); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &adminID, "admin.template.update", "os_template", &tmpl.ID, "success", clientIP, input)
	return tmpl, nil
}

func (s *AdminUsecase) DeleteTemplate(ctx context.Context, adminID, id uuid.UUID, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	if err := s.repos.Templates.DeleteByID(ctx, id); err != nil {
		return err
	}

	s.auditService.Log(ctx, &adminID, "admin.template.delete", "os_template", &id, "success", clientIP, nil)
	return nil
}

// Resource Plans Management

func (s *AdminUsecase) ListPlans(ctx context.Context) ([]models.ResourcePlan, error) {
	return s.repos.Plans.ListAll(ctx)
}

func (s *AdminUsecase) CreatePlan(ctx context.Context, adminID uuid.UUID, input CreatePlanInput, clientIP string) (*models.ResourcePlan, error) {
	ctx = context.WithoutCancel(ctx)

	plan := models.ResourcePlan{
		Name:        input.Name,
		Description: input.Description,
		VCPUs:       input.VCPUs,
		MemoryMB:    input.MemoryMB,
		DiskGB:      input.DiskGB,
		Enabled:     true,
	}
	if input.Enabled != nil {
		plan.Enabled = *input.Enabled
	}

	if err := s.repos.Plans.Create(ctx, &plan); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &adminID, "admin.plan.create", "resource_plan", &plan.ID, "success", clientIP, input)
	return &plan, nil
}

// FindPlan returns a plan or NOT_FOUND "Plan not found".
func (s *AdminUsecase) FindPlan(ctx context.Context, id uuid.UUID) (*models.ResourcePlan, error) {
	plan, err := s.repos.Plans.FindByID(ctx, id)
	if err != nil {
		return nil, errResourcePlanAbsent
	}
	return plan, nil
}

// UpdatePlan applies the set fields of input to plan (from FindPlan).
func (s *AdminUsecase) UpdatePlan(ctx context.Context, adminID uuid.UUID, plan *models.ResourcePlan, input UpdatePlanInput, clientIP string) (*models.ResourcePlan, error) {
	ctx = context.WithoutCancel(ctx)

	if input.Name != nil {
		plan.Name = *input.Name
	}
	if input.Description != nil {
		plan.Description = *input.Description
	}
	if input.VCPUs != nil {
		plan.VCPUs = *input.VCPUs
	}
	if input.MemoryMB != nil {
		plan.MemoryMB = *input.MemoryMB
	}
	if input.DiskGB != nil {
		plan.DiskGB = *input.DiskGB
	}
	if input.Enabled != nil {
		plan.Enabled = *input.Enabled
	}

	if err := s.repos.Plans.Save(ctx, plan); err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &adminID, "admin.plan.update", "resource_plan", &plan.ID, "success", clientIP, input)
	return plan, nil
}

func (s *AdminUsecase) DeletePlan(ctx context.Context, adminID, id uuid.UUID, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	if err := s.repos.Plans.DeleteByID(ctx, id); err != nil {
		return err
	}

	s.auditService.Log(ctx, &adminID, "admin.plan.delete", "resource_plan", &id, "success", clientIP, nil)
	return nil
}
