package handlers

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type MCPHandler struct {
	service *services.MCPService
}

func NewMCPHandler(service *services.MCPService) *MCPHandler {
	return &MCPHandler{
		service: service,
	}
}

// Server Registry

func (h *MCPHandler) ListServers(c *gin.Context) {
	servers, err := h.service.ListServers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": servers})
}

// Connections

func (h *MCPHandler) ListConnections(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	conns, err := h.service.ListConnections(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": conns})
}

type ConnectServerInput struct {
	ServerID   uuid.UUID `json:"server_id" binding:"required"`
	ConfigJSON string    `json:"config_json"`
}

func (h *MCPHandler) Connect(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input ConnectServerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid input"}})
		return
	}

	conn, err := h.service.ConnectServer(userID, input.ServerID, input.ConfigJSON)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": conn})
}

func (h *MCPHandler) Disconnect(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid connection id"}})
		return
	}

	if err := h.service.DisconnectServer(userID, id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "disconnected"}})
}

func (h *MCPHandler) GetConnectionTools(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid connection id"}})
		return
	}

	perms, err := h.service.GetConnectionTools(userID, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": perms})
}

type UpdatePermissionInput struct {
	ToolID  uuid.UUID `json:"tool_id" binding:"required"`
	Granted bool      `json:"granted"`
}

func (h *MCPHandler) UpdatePermission(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid connection id"}})
		return
	}

	var input UpdatePermissionInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid permission input"}})
		return
	}

	if err := h.service.UpdateToolPermission(userID, id, input.ToolID, input.Granted); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"status": "updated"}})
}

func (h *MCPHandler) ListAuditLogs(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	limit := 50
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil {
			limit = l
		}
	}

	logs, err := h.service.ListAuditLogs(userID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

// First-Party MCP JSON-RPC 2.0 Endpoint (PRD Section 37)

func (h *MCPHandler) HandleRPC(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	resp, err := h.service.HandleJSONRPC(c.Request.Context(), userID, c.ClientIP(), body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, resp)
}
