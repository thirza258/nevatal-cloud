package handler

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

// instanceJobAction is an InstanceUsecase method that queues a job.
type instanceJobAction func(ctx context.Context, id, userID uuid.UUID, isAdmin bool, clientIP string) (*models.ProvisioningJob, error)

type InstanceHandler struct {
	instances *usecase.InstanceUsecase
}

func NewInstanceHandler(instances *usecase.InstanceUsecase) *InstanceHandler {
	return &InstanceHandler{instances: instances}
}

func (h *InstanceHandler) List(c *gin.Context) {
	instances, err := h.instances.List(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.Query("status"))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, instances)
}

func (h *InstanceHandler) Get(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID format")
		return
	}

	inst, err := h.instances.Get(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, inst)
}

func (h *InstanceHandler) Create(c *gin.Context) {
	var req request.CreateInstanceInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}

	if idem := c.GetHeader("X-Idempotency-Key"); idem != "" && req.IdempotencyKey == "" {
		req.IdempotencyKey = idem
	}

	job, err := h.instances.Create(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "PROVISIONING_REJECTED", err.Error())
		return
	}

	response.Accepted(c, response.InstanceCreated{
		InstanceID: job.InstanceID,
		JobID:      job.ID,
		Status:     job.Status,
	})
}

func (h *InstanceHandler) Start(c *gin.Context) {
	h.powerAction(c, h.instances.Start)
}

func (h *InstanceHandler) Shutdown(c *gin.Context) {
	h.powerAction(c, h.instances.Shutdown)
}

func (h *InstanceHandler) Restart(c *gin.Context) {
	h.powerAction(c, h.instances.Restart)
}

func (h *InstanceHandler) Delete(c *gin.Context) {
	h.powerAction(c, h.instances.Delete)
}

// powerAction serves the endpoints that queue a power or delete job.
func (h *InstanceHandler) powerAction(c *gin.Context, action instanceJobAction) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	job, err := action(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusConflict, "ACTION_FAILED", err.Error())
		return
	}

	response.Accepted(c, response.JobQueued{JobID: job.ID, Status: job.Status})
}

// GetActivity: GET /instances/:id/activity. Non-admins only see the activity
// of instances they own (404 otherwise, like the other instance routes).
func (h *InstanceHandler) GetActivity(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	logs, err := h.instances.GetActivity(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, logs)
}

func (h *InstanceHandler) GetMetrics(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	metrics, err := h.instances.GetMetrics(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}

	if !metrics.Available {
		response.OK(c, response.InstanceMetricsUnavailable{Available: false, Reason: metrics.Reason})
		return
	}
	response.OK(c, response.NewInstanceMetrics(metrics.Status))
}

func (h *InstanceHandler) Resize(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	var req request.ResizeInstanceInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	if err := h.instances.Resize(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.ToUsecase(), c.ClientIP()); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "RESIZE_FAILED", err.Error())
		return
	}

	response.OKMessage(c, "Instance resized successfully")
}

func (h *InstanceHandler) ListSnapshots(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	snapshots, err := h.instances.ListSnapshots(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, snapshots)
}

func (h *InstanceHandler) CreateSnapshot(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	var req request.CreateSnapshotInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	snap, err := h.instances.CreateSnapshot(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "SNAPSHOT_FAILED", err.Error())
		return
	}
	response.Created(c, snap)
}

func (h *InstanceHandler) RestoreSnapshot(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	snapName := c.Param("snapname")
	if snapName == "" {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "Snapshot name is required")
		return
	}

	if err := h.instances.RestoreSnapshot(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), snapName, c.ClientIP()); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "RESTORE_FAILED", err.Error())
		return
	}
	response.OKMessage(c, "Snapshot restored successfully")
}

func (h *InstanceHandler) DeleteSnapshot(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	snapName := c.Param("snapname")
	if snapName == "" {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "Snapshot name is required")
		return
	}

	if err := h.instances.DeleteSnapshot(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), snapName, c.ClientIP()); err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "DELETE_FAILED", err.Error())
		return
	}
	response.OKMessage(c, "Snapshot deleted successfully")
}

func (h *InstanceHandler) GetConsoleTicket(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}

	ticket, err := h.instances.GetConsoleTicket(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusConflict, "CONSOLE_ERROR", err.Error())
		return
	}
	response.OK(c, ticket)
}
