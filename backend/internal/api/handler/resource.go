package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type ResourceHandler struct {
	resources *usecase.ResourceUsecase
}

func NewResourceHandler(resources *usecase.ResourceUsecase) *ResourceHandler {
	return &ResourceHandler{resources: resources}
}

// ListAll: GET /resources, the unified inventory (PRD Section 34.1).
func (h *ResourceHandler) ListAll(c *gin.Context) {
	summary := h.resources.ListAll(c.Request.Context(), middleware.GetCurrentUserID(c))
	response.OK(c, response.NewResourceList(summary))
}
