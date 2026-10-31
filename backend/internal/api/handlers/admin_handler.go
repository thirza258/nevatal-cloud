package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/services"
	"gorm.io/gorm"
)

type AdminHandler struct {
	db           *gorm.DB
	quotaService *services.QuotaService
	auditService *services.AuditService
	worker       *services.WorkerService
	proxmox      proxmox.ProxmoxClient
}

func NewAdminHandler(db *gorm.DB, quotaService *services.QuotaService, auditService *services.AuditService, worker *services.WorkerService, pve proxmox.ProxmoxClient) *AdminHandler {
	return &AdminHandler{
		db:           db,
		quotaService: quotaService,
		auditService: auditService,
		worker:       worker,
		proxmox:      pve,
	}
}

func (h *AdminHandler) Overview(c *gin.Context) {
	var totalUsers int64
	var totalInstances int64
	var runningInstances int64
	var stoppedInstances int64
	var activeJobs int64
	var failedJobs int64

	h.db.Model(&models.User{}).Count(&totalUsers)
	h.db.Model(&models.Instance{}).Where("deleted_at IS NULL").Count(&totalInstances)
	h.db.Model(&models.Instance{}).Where("status = ? AND deleted_at IS NULL", models.StateRunning).Count(&runningInstances)
	h.db.Model(&models.Instance{}).Where("status = ? AND deleted_at IS NULL", models.StateStopped).Count(&stoppedInstances)
	h.db.Model(&models.ProvisioningJob{}).Where("status IN ?", []string{models.JobStatusQueued, models.JobStatusInProgress}).Count(&activeJobs)
	h.db.Model(&models.ProvisioningJob{}).Where("status = ?", models.JobStatusFailed).Count(&failedJobs)

	// Fetch primary node status
	var node models.Node
	_ = h.db.First(&node)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nodeStatus, _ := h.proxmox.GetNodeStatus(ctx, node.ProxmoxNodeName)
	storageList, _ := h.proxmox.GetStorage(ctx, node.ProxmoxNodeName)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"total_users":       totalUsers,
			"total_instances":   totalInstances,
			"running_instances": runningInstances,
			"stopped_instances": stoppedInstances,
			"active_jobs":       activeJobs,
			"failed_jobs":       failedJobs,
			"node":              node,
			"node_status":       nodeStatus,
			"storage":           storageList,
		},
	})
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	var users []models.User
	err := h.db.Preload("Quota").Order("created_at desc").Find(&users).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	type UserWithUsage struct {
		models.User
		Usage *models.QuotaUsage `json:"usage"`
	}

	results := make([]UserWithUsage, len(users))
	for i, u := range users {
		usage, _ := h.quotaService.GetUsage(u.ID)
		results[i] = UserWithUsage{
			User:  u,
			Usage: usage,
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": results})
}

func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)
	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid user ID"}})
		return
	}

	var req struct {
		Status string `json:"status" binding:"required"` // active, suspended
	}
	if err := c.ShouldBindJSON(&req); err != nil || (req.Status != models.UserStatusActive && req.Status != models.UserStatusSuspended) {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "BAD_REQUEST", "message": "Status must be 'active' or 'suspended'"}})
		return
	}

	res := h.db.Model(&models.User{}).Where("id = ?", targetID).Update("status", req.Status)
	if res.Error != nil || res.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "USER_NOT_FOUND", "message": "User not found"}})
		return
	}

	h.auditService.Log(&adminID, "admin.user.status_update", "user", &targetID, "success", c.ClientIP(), map[string]string{"new_status": req.Status})

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "User status updated successfully"}})
}

func (h *AdminHandler) UpdateUserQuotas(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)
	targetID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid user ID"}})
		return
	}

	var req struct {
		MaxVCPUs     int `json:"max_vcpus" binding:"required,min=1"`
		MaxMemoryMB  int `json:"max_memory_mb" binding:"required,min=512"`
		MaxStorageGB int `json:"max_storage_gb" binding:"required,min=10"`
		MaxInstances int `json:"max_instances" binding:"required,min=1"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
		return
	}

	var quota models.ResourceQuota
	if err := h.db.Where("user_id = ?", targetID).First(&quota).Error; err != nil {
		quota = models.ResourceQuota{
			UserID: targetID,
		}
	}

	quota.MaxVCPUs = req.MaxVCPUs
	quota.MaxMemoryMB = req.MaxMemoryMB
	quota.MaxStorageGB = req.MaxStorageGB
	quota.MaxInstances = req.MaxInstances

	if err := h.db.Save(&quota).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	h.auditService.Log(&adminID, "admin.quota.update", "resource_quota", &quota.ID, "success", c.ClientIP(), req)

	c.JSON(http.StatusOK, gin.H{"data": quota})
}

func (h *AdminHandler) ListAllInstances(c *gin.Context) {
	var instances []models.Instance
	err := h.db.Preload("Owner").Preload("Node").Preload("OSTemplate").
		Where("deleted_at IS NULL").
		Order("created_at desc").
		Find(&instances).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": instances})
}

func (h *AdminHandler) ListJobs(c *gin.Context) {
	var jobs []models.ProvisioningJob
	err := h.db.Preload("Instance").Order("created_at desc").Limit(50).Find(&jobs).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": jobs})
}

func (h *AdminHandler) RetryJob(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)
	jobID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid job ID"}})
		return
	}

	var job models.ProvisioningJob
	if err := h.db.First(&job, "id = ?", jobID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "JOB_NOT_FOUND", "message": "Job not found"}})
		return
	}

	if job.Status != models.JobStatusFailed {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_STATE", "message": "Only failed jobs can be retried"}})
		return
	}

	job.Status = models.JobStatusQueued
	job.CurrentStep = "retry_requested"
	job.ErrorCode = nil
	job.ErrorMessage = nil
	h.db.Save(&job)

	h.auditService.Log(&adminID, "admin.job.retry", "provisioning_job", &jobID, "success", c.ClientIP(), nil)

	go h.worker.ProcessJob(job.ID, nil)

	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "Job retried successfully", "job_id": job.ID}})
}

func (h *AdminHandler) ListNodes(c *gin.Context) {
	var nodes []models.Node
	_ = h.db.Find(&nodes)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	type NodeDetail struct {
		models.Node
		LiveStatus *proxmox.NodeStatus   `json:"live_status"`
		Storage    []proxmox.StorageInfo `json:"storage"`
	}

	details := make([]NodeDetail, len(nodes))
	for i, n := range nodes {
		live, _ := h.proxmox.GetNodeStatus(ctx, n.ProxmoxNodeName)
		storage, _ := h.proxmox.GetStorage(ctx, n.ProxmoxNodeName)
		details[i] = NodeDetail{
			Node:       n,
			LiveStatus: live,
			Storage:    storage,
		}
	}

	c.JSON(http.StatusOK, gin.H{"data": details})
}

func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	logs, err := h.auditService.ListRecentActivity(nil, 100)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// OS Templates Management
func (h *AdminHandler) ListTemplates(c *gin.Context) {
	var templates []models.OSTemplate
	if err := h.db.Order("created_at desc").Find(&templates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

func (h *AdminHandler) CreateTemplate(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)

	var req struct {
		Name                string `json:"name" binding:"required"`
		Distribution        string `json:"distribution" binding:"required"`
		Version             string `json:"version" binding:"required"`
		Description         string `json:"description"`
		ProxmoxTemplateVMID int    `json:"proxmox_template_vmid" binding:"required,min=100"`
		Architecture        string `json:"architecture"`
		CloudInitSupported  *bool  `json:"cloud_init_supported"`
		DefaultUsername     string `json:"default_username"`
		Enabled             *bool  `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
		return
	}

	tmpl := models.OSTemplate{
		Name:                req.Name,
		Distribution:        req.Distribution,
		Version:             req.Version,
		Description:         req.Description,
		ProxmoxTemplateVMID: req.ProxmoxTemplateVMID,
		Architecture:        "x86_64",
		CloudInitSupported:  true,
		DefaultUsername:     "ubuntu",
		Enabled:             true,
	}
	if req.Architecture != "" {
		tmpl.Architecture = req.Architecture
	}
	if req.CloudInitSupported != nil {
		tmpl.CloudInitSupported = *req.CloudInitSupported
	}
	if req.DefaultUsername != "" {
		tmpl.DefaultUsername = req.DefaultUsername
	}
	if req.Enabled != nil {
		tmpl.Enabled = *req.Enabled
	}

	if err := h.db.Create(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	h.auditService.Log(&adminID, "admin.template.create", "os_template", &tmpl.ID, "success", c.ClientIP(), req)
	c.JSON(http.StatusCreated, gin.H{"data": tmpl})
}

func (h *AdminHandler) UpdateTemplate(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid template ID"}})
		return
	}

	var tmpl models.OSTemplate
	if err := h.db.First(&tmpl, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Template not found"}})
		return
	}

	var req struct {
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

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
		return
	}

	if req.Name != nil {
		tmpl.Name = *req.Name
	}
	if req.Distribution != nil {
		tmpl.Distribution = *req.Distribution
	}
	if req.Version != nil {
		tmpl.Version = *req.Version
	}
	if req.Description != nil {
		tmpl.Description = *req.Description
	}
	if req.ProxmoxTemplateVMID != nil {
		tmpl.ProxmoxTemplateVMID = *req.ProxmoxTemplateVMID
	}
	if req.Architecture != nil {
		tmpl.Architecture = *req.Architecture
	}
	if req.CloudInitSupported != nil {
		tmpl.CloudInitSupported = *req.CloudInitSupported
	}
	if req.DefaultUsername != nil {
		tmpl.DefaultUsername = *req.DefaultUsername
	}
	if req.Enabled != nil {
		tmpl.Enabled = *req.Enabled
	}

	if err := h.db.Save(&tmpl).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	h.auditService.Log(&adminID, "admin.template.update", "os_template", &tmpl.ID, "success", c.ClientIP(), req)
	c.JSON(http.StatusOK, gin.H{"data": tmpl})
}

func (h *AdminHandler) DeleteTemplate(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid template ID"}})
		return
	}

	if err := h.db.Delete(&models.OSTemplate{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	h.auditService.Log(&adminID, "admin.template.delete", "os_template", &id, "success", c.ClientIP(), nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "Template deleted successfully"}})
}

// Resource Plans Management
func (h *AdminHandler) ListPlans(c *gin.Context) {
	var plans []models.ResourcePlan
	if err := h.db.Order("vcpus asc, memory_mb asc").Find(&plans).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plans})
}

func (h *AdminHandler) CreatePlan(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)

	var req struct {
		Name        string `json:"name" binding:"required"`
		Description string `json:"description"`
		VCPUs       int    `json:"vcpus" binding:"required,min=1"`
		MemoryMB    int    `json:"memory_mb" binding:"required,min=256"`
		DiskGB      int    `json:"disk_gb" binding:"required,min=5"`
		Enabled     *bool  `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
		return
	}

	plan := models.ResourcePlan{
		Name:        req.Name,
		Description: req.Description,
		VCPUs:       req.VCPUs,
		MemoryMB:    req.MemoryMB,
		DiskGB:      req.DiskGB,
		Enabled:     true,
	}
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}

	if err := h.db.Create(&plan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	h.auditService.Log(&adminID, "admin.plan.create", "resource_plan", &plan.ID, "success", c.ClientIP(), req)
	c.JSON(http.StatusCreated, gin.H{"data": plan})
}

func (h *AdminHandler) UpdatePlan(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid plan ID"}})
		return
	}

	var plan models.ResourcePlan
	if err := h.db.First(&plan, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Plan not found"}})
		return
	}

	var req struct {
		Name        *string `json:"name"`
		Description *string `json:"description"`
		VCPUs       *int    `json:"vcpus"`
		MemoryMB    *int    `json:"memory_mb"`
		DiskGB      *int    `json:"disk_gb"`
		Enabled     *bool   `json:"enabled"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
		return
	}

	if req.Name != nil {
		plan.Name = *req.Name
	}
	if req.Description != nil {
		plan.Description = *req.Description
	}
	if req.VCPUs != nil {
		plan.VCPUs = *req.VCPUs
	}
	if req.MemoryMB != nil {
		plan.MemoryMB = *req.MemoryMB
	}
	if req.DiskGB != nil {
		plan.DiskGB = *req.DiskGB
	}
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}

	if err := h.db.Save(&plan).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	h.auditService.Log(&adminID, "admin.plan.update", "resource_plan", &plan.ID, "success", c.ClientIP(), req)
	c.JSON(http.StatusOK, gin.H{"data": plan})
}

func (h *AdminHandler) DeletePlan(c *gin.Context) {
	adminID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid plan ID"}})
		return
	}

	if err := h.db.Delete(&models.ResourcePlan{}, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	h.auditService.Log(&adminID, "admin.plan.delete", "resource_plan", &id, "success", c.ClientIP(), nil)
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"message": "Plan deleted successfully"}})
}
