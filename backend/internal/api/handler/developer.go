package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type DeveloperHandler struct {
	devUc *usecase.DeveloperUsecase
}

func NewDeveloperHandler(devUc *usecase.DeveloperUsecase) *DeveloperHandler {
	return &DeveloperHandler{devUc: devUc}
}

// API Keys

func (h *DeveloperHandler) ListAPIKeys(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	keys, err := h.devUc.ListAPIKeys(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, keys)
}

func (h *DeveloperHandler) CreateAPIKey(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var input usecase.CreateAPIKeyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	res, err := h.devUc.CreateAPIKey(c.Request.Context(), tenantID, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, res)
}

func (h *DeveloperHandler) RevokeAPIKey(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid api key id")
		return
	}
	if err := h.devUc.RevokeAPIKey(c.Request.Context(), tenantID, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"status": "revoked"})
}

// Webhooks

func (h *DeveloperHandler) ListWebhooks(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	eps, err := h.devUc.ListWebhookEndpoints(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, eps)
}

func (h *DeveloperHandler) CreateWebhook(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var input usecase.CreateWebhookEndpointInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	ep, err := h.devUc.CreateWebhookEndpoint(c.Request.Context(), tenantID, input)
	if err != nil {
		if err == usecase.ErrSSRFProtectionBlocked {
			response.Error(c, http.StatusBadRequest, "SSRF_PROTECTION_BLOCKED", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, ep)
}

func (h *DeveloperHandler) GetWebhook(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook id")
		return
	}
	ep, err := h.devUc.GetWebhookEndpoint(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, ep)
}

func (h *DeveloperHandler) UpdateWebhook(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook id")
		return
	}
	var input usecase.UpdateWebhookEndpointInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	ep, err := h.devUc.UpdateWebhookEndpoint(c.Request.Context(), tenantID, id, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, ep)
}

func (h *DeveloperHandler) DeleteWebhook(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook id")
		return
	}
	if err := h.devUc.DeleteWebhookEndpoint(c.Request.Context(), tenantID, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *DeveloperHandler) RotateSecret(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook id")
		return
	}
	newSecret, err := h.devUc.RotateWebhookSecret(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"secret": newSecret})
}

func (h *DeveloperHandler) TestPing(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid webhook id")
		return
	}
	del, err := h.devUc.TestWebhookPing(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, del)
}

func (h *DeveloperHandler) ListDeliveries(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var endpointID *uuid.UUID
	if epParam := c.Query("endpoint_id"); epParam != "" {
		if u, err := uuid.Parse(epParam); err == nil {
			endpointID = &u
		}
	}
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	deliveries, err := h.devUc.ListDeliveries(c.Request.Context(), tenantID, endpointID, limit)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, deliveries)
}

func (h *DeveloperHandler) RetryDelivery(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	deliveryID, err := uuid.Parse(c.Param("delivery_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid delivery id")
		return
	}
	del, err := h.devUc.RetryDelivery(c.Request.Context(), tenantID, deliveryID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, del)
}

// OpenAPI Specification Endpoint

func (h *DeveloperHandler) GetOpenAPISpec(c *gin.Context) {
	spec := map[string]any{
		"openapi": "3.1.0",
		"info": map[string]any{
			"title":       "Nevatal Cloud Developer API",
			"version":     "1.0.0",
			"description": "Public infrastructure control plane API for Nevatal Cloud compute, networking, load balancing, and webhooks.",
		},
		"servers": []map[string]any{
			{"url": "/api/v1", "description": "Nevatal Cloud API v1"},
		},
		"security": []map[string]any{
			{"ApiKeyAuth": []string{}},
			{"BearerAuth": []string{}},
		},
		"components": map[string]any{
			"securitySchemes": map[string]any{
				"ApiKeyAuth": map[string]any{
					"type": "apiKey",
					"in":   "header",
					"name": "X-API-Key",
				},
				"BearerAuth": map[string]any{
					"type":         "http",
					"scheme":       "bearer",
					"bearerFormat": "nv_live_... / JWT",
				},
			},
		},
	}
	response.JSON(c, http.StatusOK, spec)
}
