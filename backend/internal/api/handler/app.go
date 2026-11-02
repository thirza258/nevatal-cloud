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

type AppHandler struct {
	appService *usecase.AppUsecase
}

func NewAppHandler(appService *usecase.AppUsecase) *AppHandler {
	return &AppHandler{appService: appService}
}

// appID parses :id or writes the 400 reply.
func appID(c *gin.Context) (uuid.UUID, bool) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid application ID")
		return uuid.Nil, false
	}
	return id, true
}

func (h *AppHandler) List(c *gin.Context) {
	apps, err := h.appService.List(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, apps)
}

func (h *AppHandler) Get(c *gin.Context) {
	id, ok := appID(c)
	if !ok {
		return
	}

	app, err := h.appService.Get(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "Application not found")
		return
	}
	response.OK(c, app)
}

func (h *AppHandler) Create(c *gin.Context) {
	var req request.CreateAppInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	app, deployment, err := h.appService.Create(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, response.AppCreated{Data: app, Deployment: deployment})
}

func (h *AppHandler) Deploy(c *gin.Context) {
	id, ok := appID(c)
	if !ok {
		return
	}

	var req request.DeployAppRequest
	_ = c.ShouldBindJSON(&req)

	deployment, err := h.appService.TriggerDeploy(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.CommitMessage, c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "DEPLOY_ERROR", err.Error())
		return
	}
	response.Accepted(c, deployment)
}

func (h *AppHandler) Rollback(c *gin.Context) {
	id, ok := appID(c)
	if !ok {
		return
	}

	deploymentID, err := request.UUIDParam(c, "deployment_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_DEPLOYMENT_ID", "Invalid deployment ID")
		return
	}

	deployment, err := h.appService.Rollback(c.Request.Context(), id, deploymentID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "ROLLBACK_ERROR", err.Error())
		return
	}
	response.OK(c, deployment)
}

func (h *AppHandler) Power(c *gin.Context) {
	id, ok := appID(c)
	if !ok {
		return
	}

	action := c.Param("action")
	if err := h.appService.UpdatePower(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), action, c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Application power action executed: " + action})
}

func (h *AppHandler) GetEnvVars(c *gin.Context) {
	id, ok := appID(c)
	if !ok {
		return
	}

	envs, err := h.appService.GetEnvVars(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, envs)
}

func (h *AppHandler) SaveEnvVar(c *gin.Context) {
	id, ok := appID(c)
	if !ok {
		return
	}

	var req request.SaveEnvVarRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.appService.SaveEnvVar(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Key, req.Value, req.IsSecret); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Environment variable saved"})
}

func (h *AppHandler) DeleteEnvVar(c *gin.Context) {
	id, ok := appID(c)
	if !ok {
		return
	}

	key := c.Param("key")
	if key == "" {
		response.Error(c, http.StatusBadRequest, "INVALID_KEY", "Key cannot be empty")
		return
	}

	if err := h.appService.DeleteEnvVar(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), key); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Environment variable deleted successfully"})
}

func (h *AppHandler) Delete(c *gin.Context) {
	id, ok := appID(c)
	if !ok {
		return
	}

	if err := h.appService.Delete(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Application deleted successfully"})
}
