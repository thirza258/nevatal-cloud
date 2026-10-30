package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

type JobHandler struct {
	db *gorm.DB
}

func NewJobHandler(db *gorm.DB) *JobHandler {
	return &JobHandler{db: db}
}

func (h *JobHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid job ID"}})
		return
	}

	var job models.ProvisioningJob
	query := h.db.Where("id = ?", id)
	if !isAdmin {
		query = query.Where("owner_id = ?", userID)
	}

	if err := query.First(&job).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "JOB_NOT_FOUND", "message": "Provisioning job not found"}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":               job.ID,
			"instance_id":      job.InstanceID,
			"operation_type":   job.OperationType,
			"status":           job.Status,
			"current_step":     job.CurrentStep,
			"progress_percent": job.ProgressPercent,
			"error_code":       job.ErrorCode,
			"error_message":    job.ErrorMessage,
			"started_at":       job.StartedAt,
			"completed_at":     job.CompletedAt,
		},
	})
}
