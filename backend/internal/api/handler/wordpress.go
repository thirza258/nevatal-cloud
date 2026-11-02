package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// WordPressHandler serves the WordPress bridge plugin. Except for errors and
// list_instances, its bodies are not enveloped.
type WordPressHandler struct {
	wp *usecase.WordPressUsecase
}

func NewWordPressHandler(wp *usecase.WordPressUsecase) *WordPressHandler {
	return &WordPressHandler{wp: wp}
}

func (h *WordPressHandler) GetStatus(c *gin.Context) {
	status := h.wp.Status(c.Request.Context())
	response.JSON(c, http.StatusOK, response.WordPressStatus{
		ActiveInstances: status.ActiveInstances,
		HypervisorNode:  status.HypervisorNode,
		NodeHealth:      status.NodeHealth,
		Service:         "Nevatal Cloud WordPress Bridge",
		Status:          "connected",
		Timestamp:       time.Now().UTC(),
		Version:         "1.0.0",
	})
}

func (h *WordPressHandler) HandleWebhook(c *gin.Context) {
	// 1. Verify API Key
	providedKey := c.GetHeader("X-Nevatal-Key")
	if providedKey == "" {
		providedKey = c.GetHeader("X-WP-Webhook-Secret")
	}
	if providedKey == "" {
		providedKey = c.Query("api_key")
	}
	if err := h.wp.VerifyWebhookKey(providedKey); err != nil {
		writeError(c, err, http.StatusUnauthorized, "UNAUTHORIZED")
		return
	}

	var req request.WPWebhookPayload
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	ctx := c.Request.Context()
	switch req.Action {
	case "ping":
		response.JSON(c, http.StatusOK, response.StatusMessage{
			Message: "Nevatal Cloud WordPress integration handshake verified.",
			Status:  "ok",
		})

	case "get_catalog":
		templates, plans := h.wp.Catalog(ctx)
		response.JSON(c, http.StatusOK, response.WordPressCatalog{Plans: plans, Templates: templates})

	case "provision_vps":
		result, err := h.wp.ProvisionVPS(ctx, req.ProvisionInput(), c.ClientIP())
		if err != nil {
			writeError(c, err, http.StatusUnprocessableEntity, "PROVISION_FAILED")
			return
		}
		response.JSON(c, http.StatusAccepted, response.WordPressProvisioned{
			InstanceID: result.Job.InstanceID,
			JobID:      result.Job.ID,
			Message:    "VPS provisioning enqueued successfully.",
			Status:     "provisioning",
			UserID:     result.UserID,
		})

	case "list_instances":
		response.OK(c, h.wp.ListInstances(ctx, req.UserEmail))

	case "control_instance":
		job, err := h.wp.ControlInstance(ctx, req.InstanceID, req.PowerAction, c.ClientIP())
		if err != nil {
			writeError(c, err, http.StatusConflict, "ACTION_FAILED")
			return
		}
		response.JSON(c, http.StatusAccepted, response.WordPressActionAccepted{
			JobID:   job.ID,
			Message: fmt.Sprintf("Action %s dispatched successfully.", req.PowerAction),
			Status:  "accepted",
		})

	default:
		response.Error(c, http.StatusBadRequest, "UNKNOWN_ACTION", "Unknown action parameter")
	}
}
