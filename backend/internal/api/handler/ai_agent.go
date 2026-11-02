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

// AIAgentHandler serves the AI cloud agent (/ai). Its errors use the
// code-less body.
type AIAgentHandler struct {
	service *usecase.AIAgentUsecase
}

func NewAIAgentHandler(service *usecase.AIAgentUsecase) *AIAgentHandler {
	return &AIAgentHandler{service: service}
}

// conversationID parses :id of a conversation or writes the 400 reply.
func conversationID(c *gin.Context) (uuid.UUID, bool) {
	return childID(c, "id", "invalid conversation id")
}

// Conversation Endpoints

func (h *AIAgentHandler) ListConversations(c *gin.Context) {
	convs, err := h.service.ListConversations(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, convs)
}

func (h *AIAgentHandler) CreateConversation(c *gin.Context) {
	var req request.CreateConversationInput
	_ = c.ShouldBindJSON(&req)

	conv, err := h.service.CreateConversation(c.Request.Context(), middleware.GetCurrentUserID(c), req.Title)
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Created(c, conv)
}

func (h *AIAgentHandler) GetConversation(c *gin.Context) {
	id, ok := conversationID(c)
	if !ok {
		return
	}

	conv, err := h.service.GetConversation(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, conv)
}

func (h *AIAgentHandler) DeleteConversation(c *gin.Context) {
	id, ok := conversationID(c)
	if !ok {
		return
	}

	if err := h.service.DeleteConversation(c.Request.Context(), id, middleware.GetCurrentUserID(c)); err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, response.Status{Status: "deleted"})
}

func (h *AIAgentHandler) SendMessage(c *gin.Context) {
	id, ok := conversationID(c)
	if !ok {
		return
	}

	var req request.SendMessageInput
	if err := request.BindJSON(c, &req); err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, "invalid message input")
		return
	}

	resp, err := h.service.ProcessUserMessage(c.Request.Context(), id, middleware.GetCurrentUserID(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, resp)
}

func (h *AIAgentHandler) ListMessages(c *gin.Context) {
	id, ok := conversationID(c)
	if !ok {
		return
	}

	msgs, err := h.service.ListMessages(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, msgs)
}

func (h *AIAgentHandler) GetState(c *gin.Context) {
	id, ok := conversationID(c)
	if !ok {
		return
	}

	state, err := h.service.GetConversationState(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, state)
}

// Plan & Approval Endpoints

func (h *AIAgentHandler) ListPlans(c *gin.Context) {
	id, ok := conversationID(c)
	if !ok {
		return
	}

	plans, err := h.service.ListPlans(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, plans)
}

// GET /ai/plans/:id and the plan actions (validate, approve, reject,
// cancel, execute) are served by OrchestratorHandler.

// Runs (legacy agent-run endpoints; executions are served by
// OrchestratorHandler)

func (h *AIAgentHandler) GetRun(c *gin.Context) {
	response.OK(c, response.AgentRun{ID: c.Param("id"), Status: "completed"})
}

func (h *AIAgentHandler) GetRunEvents(c *gin.Context) {
	response.OK(c, []response.AgentRunEvent{
		{Event: "understanding_request", Timestamp: "now"},
		{Event: "planning", Timestamp: "now"},
		{Event: "awaiting_confirmation", Timestamp: "now"},
	})
}

func (h *AIAgentHandler) CancelRun(c *gin.Context) {
	response.OK(c, response.Status{Status: "cancelled"})
}
