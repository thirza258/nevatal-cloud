package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// TemplatePlanHandler serves the public catalog and the caller's quota,
// usage and recent activity.
type TemplatePlanHandler struct {
	catalog      *usecase.CatalogUsecase
	quotaService *usecase.QuotaUsecase
	auditService *usecase.AuditUsecase
}

func NewTemplatePlanHandler(catalog *usecase.CatalogUsecase, quotaService *usecase.QuotaUsecase, auditService *usecase.AuditUsecase) *TemplatePlanHandler {
	return &TemplatePlanHandler{
		catalog:      catalog,
		quotaService: quotaService,
		auditService: auditService,
	}
}

func (h *TemplatePlanHandler) ListTemplates(c *gin.Context) {
	templates, err := h.catalog.ListTemplates(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, templates)
}

func (h *TemplatePlanHandler) ListPlans(c *gin.Context) {
	plans, err := h.catalog.ListPlans(c.Request.Context())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, plans)
}

func (h *TemplatePlanHandler) GetQuotas(c *gin.Context) {
	h.usage(c)
}

func (h *TemplatePlanHandler) GetUsage(c *gin.Context) {
	h.usage(c)
}

func (h *TemplatePlanHandler) usage(c *gin.Context) {
	usage, err := h.quotaService.GetUsage(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, usage)
}

func (h *TemplatePlanHandler) GetActivity(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	logs, err := h.auditService.ListRecentActivity(c.Request.Context(), &userID, 30)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, logs)
}
