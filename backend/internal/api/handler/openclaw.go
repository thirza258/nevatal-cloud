package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type OpenClawHandler struct {
	openclawService *usecase.OpenClawUsecase
}

func NewOpenClawHandler(openclawService *usecase.OpenClawUsecase) *OpenClawHandler {
	return &OpenClawHandler{openclawService: openclawService}
}

func (h *OpenClawHandler) List(c *gin.Context) {
	instances, err := h.openclawService.List(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, instances)
}

func (h *OpenClawHandler) Get(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	inst, err := h.openclawService.Get(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "OpenClaw instance not found")
		return
	}
	response.OK(c, inst)
}

func (h *OpenClawHandler) Create(c *gin.Context) {
	var req request.CreateOpenClawInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	inst, err := h.openclawService.Create(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.Created(c, inst)
}

func (h *OpenClawHandler) UpdateConfig(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	var req request.UpdateOpenClawConfigRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.openclawService.UpdateConfig(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Provider, req.Model, req.APIKey, c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Agent configuration updated successfully"})
}

func (h *OpenClawHandler) ConfigureIntegration(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	var req request.AddIntegrationRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	integ, err := h.openclawService.AddIntegration(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Channel, req.Identifier, c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, integ)
}

func (h *OpenClawHandler) DeleteIntegration(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}
	// The route parameter is named :channel but carries the integration id.
	integrationID, err := request.UUIDParam(c, "channel")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_INTEGRATION_ID", "Invalid integration ID")
		return
	}

	if err := h.openclawService.DeleteIntegration(c.Request.Context(), id, integrationID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c)); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Integration removed successfully"})
}

func (h *OpenClawHandler) Start(c *gin.Context) {
	h.controlAction(c, "start")
}

func (h *OpenClawHandler) Stop(c *gin.Context) {
	h.controlAction(c, "stop")
}

func (h *OpenClawHandler) Restart(c *gin.Context) {
	h.controlAction(c, "restart")
}

func (h *OpenClawHandler) controlAction(c *gin.Context, action string) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	if err := h.openclawService.UpdatePower(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), action, c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "ACTION_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Instance state updated to " + action})
}

func (h *OpenClawHandler) CreateBackup(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	backup, err := h.openclawService.CreateBackup(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "BACKUP_ERROR", err.Error())
		return
	}
	response.Created(c, backup)
}

func (h *OpenClawHandler) GetLogs(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	logs, err := h.openclawService.GetLogs(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, logs)
}

func (h *OpenClawHandler) Delete(c *gin.Context) {
	id, ok := managedInstanceID(c)
	if !ok {
		return
	}

	if err := h.openclawService.Delete(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "OpenClaw instance deleted successfully"})
}
