package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type AIAssistantHandler struct {
	service *usecase.AIAssistantUsecase
}

func NewAIAssistantHandler(service *usecase.AIAssistantUsecase) *AIAssistantHandler {
	return &AIAssistantHandler{service: service}
}

func (h *AIAssistantHandler) Chat(c *gin.Context) {
	var req request.ChatRequestInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}

	// Check header for custom API key if not in body
	if req.APIKey == "" {
		if headerKey := c.GetHeader("X-OpenRouter-Key"); headerKey != "" {
			req.APIKey = headerKey
		}
	}

	resp, err := h.service.Chat(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "AI_CHAT_ERROR", err.Error())
		return
	}
	response.OK(c, resp)
}

func (h *AIAssistantHandler) Execute(c *gin.Context) {
	var req request.ExecuteActionInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_FAILED", err.Error())
		return
	}

	// The action card becomes an orchestrator plan awaiting approval;
	// nothing is provisioned here.
	result, err := h.service.ExecuteAction(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusUnprocessableEntity, "EXECUTE_ACTION_FAILED")
		return
	}
	response.OK(c, result)
}

func (h *AIAssistantHandler) GetContext(c *gin.Context) {
	ctxData, err := h.service.GetPlatformContext(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "CONTEXT_ERROR", err.Error())
		return
	}
	response.OK(c, ctxData)
}

func (h *AIAssistantHandler) GetHistory(c *gin.Context) {
	messages, err := h.service.ListHistory(c.Request.Context(), middleware.GetCurrentUserID(c), 50)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "HISTORY_ERROR", err.Error())
		return
	}
	response.OK(c, messages)
}

func (h *AIAssistantHandler) ClearHistory(c *gin.Context) {
	if err := h.service.ClearHistory(c.Request.Context(), middleware.GetCurrentUserID(c)); err != nil {
		response.Error(c, http.StatusInternalServerError, "CLEAR_HISTORY_ERROR", err.Error())
		return
	}
	response.OK(c, response.Status{Status: "cleared"})
}

func (h *AIAssistantHandler) ListModels(c *gin.Context) {
	response.OK(c, response.AIModels{Models: usecase.RecommendedOpenRouterModels})
}

func (h *AIAssistantHandler) VerifyKey(c *gin.Context) {
	var req request.VerifyKeyInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_FAILED", "API Key is required")
		return
	}

	check, err := h.service.VerifyOpenRouterKey(c.Request.Context(), req.APIKey)
	if err != nil {
		var ucErr *usecase.Error
		if errors.As(err, &ucErr) {
			writeError(c, err, http.StatusBadGateway, "OPENROUTER_UNREACHABLE")
			return
		}
		// Building the upstream request failed: legacy code-less body.
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}

	if !check.Valid {
		response.JSON(c, http.StatusUnauthorized, response.OpenRouterKeyInvalid{
			Error: response.OpenRouterKeyError{
				Code:    "INVALID_KEY",
				Message: "OpenRouter rejected this API key",
				Raw:     check.RawResponse,
			},
			Valid: false,
		})
		return
	}
	response.JSON(c, http.StatusOK, response.OpenRouterKeyValid{Data: check.Info, Valid: true})
}
