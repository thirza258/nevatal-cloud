package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type OpenClawHandler struct {
	openclawService *services.OpenClawService
}

func NewOpenClawHandler(openclawService *services.OpenClawService) *OpenClawHandler {
	return &OpenClawHandler{
		openclawService: openclawService,
	}
}

func (h *OpenClawHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	instances, err := h.openclawService.List(userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": instances})
}

func (h *OpenClawHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	inst, err := h.openclawService.Get(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "OpenClaw instance not found"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": inst})
}

func (h *OpenClawHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input services.CreateOpenClawInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	inst, err := h.openclawService.Create(userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": inst})
}

func (h *OpenClawHandler) UpdateConfig(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	var input struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		APIKey   string `json:"api_key"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	if err := h.openclawService.UpdateConfig(id, userID, isAdmin, input.Provider, input.Model, input.APIKey, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Agent configuration updated successfully"})
}

func (h *OpenClawHandler) ConfigureIntegration(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	var input struct {
		Channel    string `json:"channel" binding:"required"`
		Identifier string `json:"identifier" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	integ, err := h.openclawService.AddIntegration(id, userID, isAdmin, input.Channel, input.Identifier, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": integ})
}

func (h *OpenClawHandler) DeleteIntegration(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}
	integrationID, err := uuid.Parse(c.Param("channel"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_INTEGRATION_ID", "message": "Invalid integration ID"}})
		return
	}

	if err := h.openclawService.DeleteIntegration(id, integrationID, userID, isAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Integration removed successfully"})
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
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	if err := h.openclawService.UpdatePower(id, userID, isAdmin, action, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "ACTION_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Instance state updated to " + action})
}

func (h *OpenClawHandler) CreateBackup(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	backup, err := h.openclawService.CreateBackup(id, userID, isAdmin, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "BACKUP_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": backup})
}

func (h *OpenClawHandler) GetLogs(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	logs, err := h.openclawService.GetLogs(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func (h *OpenClawHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	if err := h.openclawService.Delete(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "OpenClaw instance deleted successfully"})
}
