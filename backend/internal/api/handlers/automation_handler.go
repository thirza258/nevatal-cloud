package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type AutomationHandler struct {
	automationService *services.AutomationService
}

func NewAutomationHandler(automationService *services.AutomationService) *AutomationHandler {
	return &AutomationHandler{
		automationService: automationService,
	}
}

func (h *AutomationHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	instances, err := h.automationService.List(userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": instances})
}

func (h *AutomationHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	inst, err := h.automationService.Get(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Automation instance not found"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": inst})
}

func (h *AutomationHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input services.CreateAutomationInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	inst, err := h.automationService.Create(userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": inst})
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
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	if err := h.automationService.UpdatePower(id, userID, isAdmin, action, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "ACTION_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Instance state updated to " + action})
}

func (h *AutomationHandler) CreateBackup(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	backup, err := h.automationService.CreateBackup(id, userID, isAdmin, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "BACKUP_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": backup})
}

func (h *AutomationHandler) GetLogs(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	logs, err := h.automationService.GetLogs(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func (h *AutomationHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
		return
	}

	if err := h.automationService.Delete(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Automation instance deleted successfully"})
}
