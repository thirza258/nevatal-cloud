package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type AppHandler struct {
	appService *services.AppService
}

func NewAppHandler(appService *services.AppService) *AppHandler {
	return &AppHandler{
		appService: appService,
	}
}

func (h *AppHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	apps, err := h.appService.List(userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": apps})
}

func (h *AppHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid application ID"}})
		return
	}

	app, err := h.appService.Get(appID, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Application not found"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": app})
}

func (h *AppHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input services.CreateAppInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	app, deployment, err := h.appService.Create(userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": app, "deployment": deployment})
}

func (h *AppHandler) Deploy(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid application ID"}})
		return
	}

	var body struct {
		CommitMessage string `json:"commit_message"`
	}
	_ = c.ShouldBindJSON(&body)

	deployment, err := h.appService.TriggerDeploy(appID, userID, isAdmin, body.CommitMessage, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "DEPLOY_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"data": deployment})
}

func (h *AppHandler) Rollback(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid application ID"}})
		return
	}

	deploymentID, err := uuid.Parse(c.Param("deployment_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_DEPLOYMENT_ID", "message": "Invalid deployment ID"}})
		return
	}

	deployment, err := h.appService.Rollback(appID, deploymentID, userID, isAdmin, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "ROLLBACK_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": deployment})
}

func (h *AppHandler) Power(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid application ID"}})
		return
	}

	action := c.Param("action")
	if err := h.appService.UpdatePower(appID, userID, isAdmin, action, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Application power action executed: " + action})
}

func (h *AppHandler) GetEnvVars(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid application ID"}})
		return
	}

	envs, err := h.appService.GetEnvVars(appID, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": envs})
}

func (h *AppHandler) SaveEnvVar(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid application ID"}})
		return
	}

	var body struct {
		Key      string `json:"key" binding:"required"`
		Value    string `json:"value" binding:"required"`
		IsSecret bool   `json:"is_secret"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	if err := h.appService.SaveEnvVar(appID, userID, isAdmin, body.Key, body.Value, body.IsSecret); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Environment variable saved"})
}

func (h *AppHandler) DeleteEnvVar(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid application ID"}})
		return
	}

	key := c.Param("key")
	if key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_KEY", "message": "Key cannot be empty"}})
		return
	}

	if err := h.appService.DeleteEnvVar(appID, userID, isAdmin, key); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Environment variable deleted successfully"})
}

func (h *AppHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid application ID"}})
		return
	}

	if err := h.appService.Delete(appID, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Application deleted successfully"})
}
