package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type AIAssistantHandler struct {
	service *services.AIAssistantService
}

func NewAIAssistantHandler(service *services.AIAssistantService) *AIAssistantHandler {
	return &AIAssistantHandler{
		service: service,
	}
}

func (h *AIAssistantHandler) Chat(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var input services.ChatRequestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "VALIDATION_FAILED", "message": err.Error()},
		})
		return
	}

	// Check header for custom API key if not in body
	if input.APIKey == "" {
		if headerKey := c.GetHeader("X-OpenRouter-Key"); headerKey != "" {
			input.APIKey = headerKey
		}
	}

	resp, err := h.service.Chat(c.Request.Context(), userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "AI_CHAT_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": resp,
	})
}

func (h *AIAssistantHandler) Execute(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var input services.ExecuteActionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "VALIDATION_FAILED", "message": err.Error()},
		})
		return
	}

	result, err := h.service.ExecuteAction(userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error": gin.H{"code": "EXECUTE_ACTION_FAILED", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": result,
	})
}

func (h *AIAssistantHandler) GetContext(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	ctxData, err := h.service.GetPlatformContext(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "CONTEXT_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": ctxData,
	})
}

func (h *AIAssistantHandler) GetHistory(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	messages, err := h.service.ListHistory(userID, 50)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "HISTORY_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": messages,
	})
}

func (h *AIAssistantHandler) ClearHistory(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	if err := h.service.ClearHistory(userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": gin.H{"code": "CLEAR_HISTORY_ERROR", "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{"status": "cleared"},
	})
}

func (h *AIAssistantHandler) ListModels(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"models": services.RecommendedOpenRouterModels,
		},
	})
}

type VerifyKeyInput struct {
	APIKey string `json:"api_key" binding:"required"`
}

func (h *AIAssistantHandler) VerifyKey(c *gin.Context) {
	var input VerifyKeyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "VALIDATION_FAILED", "message": "API Key is required"},
		})
		return
	}

	// Ping OpenRouter key check endpoint
	req, err := http.NewRequestWithContext(c.Request.Context(), "GET", "https://openrouter.ai/api/v1/auth/key", nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", input.APIKey))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{
			"error": gin.H{"code": "OPENROUTER_UNREACHABLE", "message": "Failed to connect to OpenRouter: " + err.Error()},
		})
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "INVALID_KEY", "message": "OpenRouter rejected this API key", "raw": string(bodyBytes)},
			"valid": false,
		})
		return
	}

	var keyInfo map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &keyInfo)

	c.JSON(http.StatusOK, gin.H{
		"valid": true,
		"data":  keyInfo,
	})
}
