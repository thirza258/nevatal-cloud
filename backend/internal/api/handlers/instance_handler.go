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

type InstanceHandler struct {
	db              *gorm.DB
	instanceService *services.InstanceService
	auditService    *services.AuditService
	proxmox         proxmox.ProxmoxClient
}

func NewInstanceHandler(db *gorm.DB, instanceService *services.InstanceService, auditService *services.AuditService, pve proxmox.ProxmoxClient) *InstanceHandler {
	return &InstanceHandler{
		db:              db,
		instanceService: instanceService,
		auditService:    auditService,
		proxmox:         pve,
	}
}

func (h *InstanceHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	statusFilter := c.Query("status")

	instances, err := h.instanceService.List(userID, isAdmin, statusFilter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": instances,
	})
}

func (h *InstanceHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	idStr := c.Param("id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID format"}})
		return
	}

	inst, err := h.instanceService.Get(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": inst,
	})
}

func (h *InstanceHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var input services.CreateInstanceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_FAILED", "message": err.Error()}})
		return
	}

	if idem := c.GetHeader("X-Idempotency-Key"); idem != "" && input.IdempotencyKey == "" {
		input.IdempotencyKey = idem
	}

	job, err := h.instanceService.Create(userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": gin.H{"code": "PROVISIONING_REJECTED", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"data": gin.H{
			"instance_id": job.InstanceID,
			"job_id":      job.ID,
			"status":      job.Status,
		},
	})
}

func (h *InstanceHandler) Start(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	job, err := h.instanceService.Start(id, userID, isAdmin, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "ACTION_FAILED", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"data": gin.H{
			"job_id": job.ID,
			"status": job.Status,
		},
	})
}

func (h *InstanceHandler) Shutdown(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	job, err := h.instanceService.Shutdown(id, userID, isAdmin, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "ACTION_FAILED", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"data": gin.H{
			"job_id": job.ID,
			"status": job.Status,
		},
	})
}

func (h *InstanceHandler) Restart(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	job, err := h.instanceService.Restart(id, userID, isAdmin, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "ACTION_FAILED", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"data": gin.H{
			"job_id": job.ID,
			"status": job.Status,
		},
	})
}

func (h *InstanceHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	job, err := h.instanceService.Delete(id, userID, isAdmin, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "ACTION_FAILED", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"data": gin.H{
			"job_id": job.ID,
			"status": job.Status,
		},
	})
}

func (h *InstanceHandler) GetActivity(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	var logs []models.AuditLog
	_ = h.db.Where("resource_type = 'instance' AND resource_id = ?", id).
		Order("created_at desc").
		Limit(20).
		Find(&logs)

	c.JSON(http.StatusOK, gin.H{
		"data": logs,
	})
}

func (h *InstanceHandler) GetMetrics(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	inst, err := h.instanceService.Get(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}})
		return
	}

	if inst.Status != models.StateRunning || inst.ProxmoxVMID <= 0 {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"available": false,
				"reason":    "Metrics are only available while the instance is running.",
			},
		})
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	pveStatus, err := h.proxmox.GetVMStatus(ctx, inst.Node.ProxmoxNodeName, inst.ProxmoxVMID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"data": gin.H{
				"available": false,
				"reason":    "Unable to query Proxmox guest metrics at this time.",
			},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"available":   true,
			"cpu_usage":   pveStatus.CPUUsage,
			"memory_used": pveStatus.MemoryUsed,
			"memory_max":  pveStatus.MemoryMax,
			"disk_used":   pveStatus.DiskUsed,
			"disk_max":    pveStatus.DiskMax,
			"net_in":      pveStatus.NetIn,
			"net_out":     pveStatus.NetOut,
			"uptime":      pveStatus.Uptime,
		},
	})
}
