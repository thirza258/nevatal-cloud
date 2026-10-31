package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type AIAgentHandler struct {
	service *services.AIAgentService
}

func NewAIAgentHandler(service *services.AIAgentService) *AIAgentHandler {
	return &AIAgentHandler{
		service: service,
	}
}

// Conversation Endpoints

func (h *AIAgentHandler) ListConversations(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	convs, err := h.service.ListConversations(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": convs})
}

type CreateConversationInput struct {
	Title string `json:"title"`
}

func (h *AIAgentHandler) CreateConversation(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input CreateConversationInput
	_ = c.ShouldBindJSON(&input)

	conv, err := h.service.CreateConversation(userID, input.Title)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": conv})
}

func (h *AIAgentHandler) GetConversation(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid conversation id"}})
		return
	}

	conv, err := h.service.GetConversation(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": conv})
}

func (h *AIAgentHandler) DeleteConversation(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid conversation id"}})
		return
	}

	if err := h.service.DeleteConversation(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "deleted"}})
}

func (h *AIAgentHandler) SendMessage(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid conversation id"}})
		return
	}

	var input services.SendMessageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid message input"}})
		return
	}

	resp, err := h.service.ProcessUserMessage(c.Request.Context(), id, userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (h *AIAgentHandler) ListMessages(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid conversation id"}})
		return
	}

	msgs, err := h.service.ListMessages(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": msgs})
}

func (h *AIAgentHandler) GetState(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid conversation id"}})
		return
	}

	state, err := h.service.GetConversationState(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": state})
}

// Plan & Approval Endpoints

func (h *AIAgentHandler) ListPlans(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid conversation id"}})
		return
	}

	plans, err := h.service.ListPlans(id, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plans})
}

func (h *AIAgentHandler) GetPlan(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid plan id"}})
		return
	}

	plan, err := h.service.GetPlan(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plan})
}

func (h *AIAgentHandler) ValidatePlan(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid plan id"}})
		return
	}

	plan, err := h.service.GetPlan(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"valid":   true,
			"plan_id": plan.ID,
			"status":  plan.Status,
		},
	})
}

func (h *AIAgentHandler) ApprovePlan(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid plan id"}})
		return
	}

	plan, err := h.service.GetPlan(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	resp, err := h.service.ApprovePlan(c.Request.Context(), plan.ConversationID, userID, services.ApprovePlanInput{
		PlanID:  plan.ID,
		Version: plan.Version,
	}, c.ClientIP())

	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": resp})
}

func (h *AIAgentHandler) CancelPlan(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid plan id"}})
		return
	}

	plan, err := h.service.GetPlan(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	if err := h.service.CancelPlan(plan.ConversationID, plan.ID, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "cancelled"}})
}

// Runs

func (h *AIAgentHandler) GetRun(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":     c.Param("id"),
			"status": "completed",
		},
	})
}

func (h *AIAgentHandler) GetRunEvents(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": []gin.H{
			{"event": "understanding_request", "timestamp": "now"},
			{"event": "planning", "timestamp": "now"},
			{"event": "awaiting_confirmation", "timestamp": "now"},
		},
	})
}

func (h *AIAgentHandler) CancelRun(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "cancelled"}})
}
