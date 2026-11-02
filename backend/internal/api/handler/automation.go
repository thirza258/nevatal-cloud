package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type AutomationHandler struct {
	automationService *usecase.AutomationUsecase
}

func NewAutomationHandler(automationService *usecase.AutomationUsecase) *AutomationHandler {
	return &AutomationHandler{automationService: automationService}
}

// managedInstanceID parses :id of an n8n or OpenClaw instance or writes the
// 400 reply.
func managedInstanceID(c *gin.Context) (uuid.UUID, bool) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return uuid.Nil, false
	}
	return id, true
}

func (h *AutomationHandler) List(c *gin.Context) {
	instances, err := h.automationService.List(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, instances)
}

func (h *AutomationHandler) Get(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	inst, err := h.automationService.Get(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "Automation instance not found")
		return
	}
	response.OK(c, inst)
}

func (h *AutomationHandler) Create(c *gin.Context) {
	var req request.CreateAutomationInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	inst, err := h.automationService.Create(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.Created(c, inst)
}

func (h *AutomationHandler) Start(c *gin.Context) {
	h.controlAction(c, "start")
}

func (h *AutomationHandler) Stop(c *gin.Context) {
	h.controlAction(c, "stop")
}

func (h *AutomationHandler) Restart(c *gin.Context) {
	h.controlAction(c, "restart")
}

func (h *AutomationHandler) controlAction(c *gin.Context, action string) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	if err := h.automationService.UpdatePower(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), action, c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "ACTION_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Instance state updated to " + action})
}

func (h *AutomationHandler) CreateBackup(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	backup, err := h.automationService.CreateBackup(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "BACKUP_ERROR", err.Error())
		return
	}
	response.Created(c, backup)
}

func (h *AutomationHandler) GetLogs(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	logs, err := h.automationService.GetLogs(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, logs)
}

func (h *AutomationHandler) Delete(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	if err := h.automationService.Delete(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Automation instance deleted successfully"})
}
