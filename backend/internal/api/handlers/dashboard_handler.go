package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/services"
	"gorm.io/gorm"
)

type DashboardHandler struct {
	db           *gorm.DB
	quotaService *services.QuotaService
	auditService *services.AuditService
}

func NewDashboardHandler(db *gorm.DB, quotaService *services.QuotaService, auditService *services.AuditService) *DashboardHandler {
	return &DashboardHandler{
		db:           db,
		quotaService: quotaService,
		auditService: auditService,
	}
}

func (h *DashboardHandler) GetSummary(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	// 1. Quota & usage
	usage, err := h.quotaService.GetUsage(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "SERVER_ERROR", "message": "Failed to calculate quota usage"},
		})
		return
	}

	// 2. Instance state breakdown
	var instances []models.Instance
	_ = h.db.Model(&models.Instance{}).
		Preload("OSTemplate").
		Where("owner_id = ? AND deleted_at IS NULL", userID).
		Order("created_at desc").
		Find(&instances)

	var running, stopped, provisioning int
	for _, inst := range instances {
		switch inst.Status {
		case models.StateRunning:
			running++
		case models.StateStopped:
			stopped++
		case models.StateProvisioning, models.StateQueued, models.StateStarting, models.StateRestarting:
			provisioning++
		}
	}

	// 3. Recent activity
	recentActivity, _ := h.auditService.ListRecentActivity(&userID, 10)

	// 4. Return summary payload
	recentInstances := instances
	if len(recentInstances) > 5 {
		recentInstances = recentInstances[:5]
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"instances": gin.H{
				"total":        len(instances),
				"running":      running,
				"stopped":      stopped,
				"provisioning": provisioning,
			},
			"usage":            usage,
			"recent_instances": recentInstances,
			"recent_activity":  recentActivity,
		},
	})
}
