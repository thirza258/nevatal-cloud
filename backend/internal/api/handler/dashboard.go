package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type DashboardHandler struct {
	dashboard *usecase.DashboardUsecase
}

func NewDashboardHandler(dashboard *usecase.DashboardUsecase) *DashboardHandler {
	return &DashboardHandler{dashboard: dashboard}
}

func (h *DashboardHandler) GetSummary(c *gin.Context) {
	summary, err := h.dashboard.GetSummary(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", "Failed to calculate quota usage")
		return
	}
	response.OK(c, response.NewDashboardSummary(summary))
}
