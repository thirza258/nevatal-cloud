package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// DockerHandler serves the per-VM Docker viewer for customers
// (/api/v1/instances/:id/docker...) and administrators
// (/api/v1/admin/instances/:id/docker...).
type DockerHandler struct {
	docker *usecase.DockerUsecase
}

func NewDockerHandler(docker *usecase.DockerUsecase) *DockerHandler {
	return &DockerHandler{docker: docker}
}

// Overview: GET /instances/:id/docker?stats=true
func (h *DockerHandler) Overview(c *gin.Context) {
	h.overview(c, middleware.IsAdmin(c))
}

// Logs: GET /instances/:id/docker/containers/:container/logs?tail=200&since=1h
func (h *DockerHandler) Logs(c *gin.Context) {
	h.logs(c, middleware.IsAdmin(c))
}

// AdminOverview: GET /admin/instances/:id/docker (any tenant's VM).
func (h *DockerHandler) AdminOverview(c *gin.Context) {
	h.overview(c, true)
}

// AdminLogs: GET /admin/instances/:id/docker/containers/:container/logs.
func (h *DockerHandler) AdminLogs(c *gin.Context) {
	h.logs(c, true)
}

func (h *DockerHandler) overview(c *gin.Context, isAdmin bool) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}
	withStats, _ := strconv.ParseBool(c.DefaultQuery("stats", "false"))

	ov, err := h.docker.Overview(c.Request.Context(), id, middleware.GetCurrentUserID(c), isAdmin, withStats, c.ClientIP())
	if err != nil {
		writeDockerError(c, err)
		return
	}
	response.OK(c, ov)
}

func (h *DockerHandler) logs(c *gin.Context, isAdmin bool) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid instance ID")
		return
	}
	tail := 0
	if raw := c.Query("tail"); raw != "" {
		tail, err = strconv.Atoi(raw)
		if err != nil {
			response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "tail must be an integer")
			return
		}
	}

	logs, err := h.docker.Logs(c.Request.Context(), id, middleware.GetCurrentUserID(c), isAdmin,
		c.Param("container"), tail, c.Query("since"), c.ClientIP())
	if err != nil {
		writeDockerError(c, err)
		return
	}
	response.OK(c, logs)
}

func writeDockerError(c *gin.Context, err error) {
	var dErr *usecase.DockerError
	if errors.As(err, &dErr) {
		response.Error(c, dErr.Status, dErr.Code, dErr.Message)
		return
	}
	response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "Unexpected error while contacting the VM")
}
