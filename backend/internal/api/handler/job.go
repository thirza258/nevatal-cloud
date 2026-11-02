package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type JobHandler struct {
	jobs *usecase.JobUsecase
}

func NewJobHandler(jobs *usecase.JobUsecase) *JobHandler {
	return &JobHandler{jobs: jobs}
}

func (h *JobHandler) Get(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid job ID")
		return
	}

	job, err := h.jobs.Get(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.Error(c, http.StatusNotFound, "JOB_NOT_FOUND", "Provisioning job not found")
		return
	}
	response.OK(c, response.NewJobStatus(job))
}
