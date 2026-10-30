package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/services"
	"gorm.io/gorm"
)

type TemplatePlanHandler struct {
	db           *gorm.DB
	quotaService *services.QuotaService
	auditService *services.AuditService
}

func NewTemplatePlanHandler(db *gorm.DB, quotaService *services.QuotaService, auditService *services.AuditService) *TemplatePlanHandler {
	return &TemplatePlanHandler{
		db:           db,
		quotaService: quotaService,
		auditService: auditService,
	}
}

func (h *TemplatePlanHandler) ListTemplates(c *gin.Context) {
	var templates []models.OSTemplate
	err := h.db.Where("enabled = true").Order("created_at asc").Find(&templates).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": templates})
}

func (h *TemplatePlanHandler) ListPlans(c *gin.Context) {
	var plans []models.ResourcePlan
	err := h.db.Where("enabled = true").Order("vcpus asc, memory_mb asc").Find(&plans).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plans})
}

func (h *TemplatePlanHandler) GetQuotas(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	usage, err := h.quotaService.GetUsage(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": usage})
}

func (h *TemplatePlanHandler) GetUsage(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	usage, err := h.quotaService.GetUsage(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": usage})
}

func (h *TemplatePlanHandler) GetActivity(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	logs, err := h.auditService.ListRecentActivity(&userID, 30)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}
