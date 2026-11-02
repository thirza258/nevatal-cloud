package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type AdminHandler struct {
	admin *usecase.AdminUsecase
}

func NewAdminHandler(admin *usecase.AdminUsecase) *AdminHandler {
	return &AdminHandler{admin: admin}
}

func (h *AdminHandler) Overview(c *gin.Context) {
	response.OK(c, response.NewAdminOverview(h.admin.Overview(c.Request.Context())))
}

func (h *AdminHandler) ListUsers(c *gin.Context) {
	users, err := h.admin.ListUsers(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, response.NewUsersWithUsage(users))
}

func (h *AdminHandler) UpdateUserStatus(c *gin.Context) {
	targetID, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid user ID")
		return
	}

	var req request.UpdateUserStatusRequest
	if err := request.BindJSON(c, &req); err != nil || !usecase.IsValidUserStatus(req.Status) {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "Status must be 'active' or 'suspended'")
		return
	}

	if err := h.admin.UpdateUserStatus(c.Request.Context(), middleware.GetCurrentUserID(c), targetID, req.Status, c.ClientIP()); err != nil {
		writeError(c, err, http.StatusNotFound, "USER_NOT_FOUND")
		return
	}
	response.OKMessage(c, "User status updated successfully")
}

func (h *AdminHandler) UpdateUserQuotas(c *gin.Context) {
	targetID, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid user ID")
		return
	}

	var req request.UpdateUserQuotasRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	quota, err := h.admin.UpdateUserQuotas(c.Request.Context(), middleware.GetCurrentUserID(c), targetID, request.UserQuotaInput(req), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, quota)
}

func (h *AdminHandler) ListAllInstances(c *gin.Context) {
	instances, err := h.admin.ListAllInstances(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, instances)
}

func (h *AdminHandler) ListJobs(c *gin.Context) {
	jobs, err := h.admin.ListJobs(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, jobs)
}

func (h *AdminHandler) RetryJob(c *gin.Context) {
	jobID, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid job ID")
		return
	}

	job, err := h.admin.RetryJob(c.Request.Context(), middleware.GetCurrentUserID(c), jobID, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.OK(c, response.JobRetried{JobID: job.ID, Message: "Job retried successfully"})
}

func (h *AdminHandler) ListNodes(c *gin.Context) {
	response.OK(c, response.NewNodeDetails(h.admin.ListNodes(c.Request.Context())))
}

func (h *AdminHandler) ListAuditLogs(c *gin.Context) {
	logs, err := h.admin.ListAuditLogs(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, logs)
}

// OS Templates Management
func (h *AdminHandler) ListTemplates(c *gin.Context) {
	templates, err := h.admin.ListTemplates(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, templates)
}

func (h *AdminHandler) CreateTemplate(c *gin.Context) {
	var req request.CreateTemplateRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	tmpl, err := h.admin.CreateTemplate(c.Request.Context(), middleware.GetCurrentUserID(c), request.CreateTemplateInput(req), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.Created(c, tmpl)
}

func (h *AdminHandler) UpdateTemplate(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid template ID")
		return
	}

	tmpl, err := h.admin.FindTemplate(c.Request.Context(), id)
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}

	var req request.UpdateTemplateRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	tmpl, err = h.admin.UpdateTemplate(c.Request.Context(), middleware.GetCurrentUserID(c), tmpl, request.UpdateTemplateInput(req), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, tmpl)
}

func (h *AdminHandler) DeleteTemplate(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid template ID")
		return
	}

	if err := h.admin.DeleteTemplate(c.Request.Context(), middleware.GetCurrentUserID(c), id, c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OKMessage(c, "Template deleted successfully")
}

// Resource Plans Management
func (h *AdminHandler) ListPlans(c *gin.Context) {
	plans, err := h.admin.ListPlans(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, plans)
}

func (h *AdminHandler) CreatePlan(c *gin.Context) {
	var req request.CreatePlanRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	plan, err := h.admin.CreatePlan(c.Request.Context(), middleware.GetCurrentUserID(c), request.CreatePlanInput(req), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.Created(c, plan)
}

func (h *AdminHandler) UpdatePlan(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid plan ID")
		return
	}

	plan, err := h.admin.FindPlan(c.Request.Context(), id)
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}

	var req request.UpdatePlanRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	plan, err = h.admin.UpdatePlan(c.Request.Context(), middleware.GetCurrentUserID(c), plan, request.UpdatePlanInput(req), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, plan)
}

func (h *AdminHandler) DeletePlan(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid plan ID")
		return
	}

	if err := h.admin.DeletePlan(c.Request.Context(), middleware.GetCurrentUserID(c), id, c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OKMessage(c, "Plan deleted successfully")
}
