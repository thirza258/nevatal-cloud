package handler

import (
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// MCPHandler serves the MCP integration registry and the first-party MCP
// JSON-RPC endpoint. Its errors use the code-less body.
type MCPHandler struct {
	service *usecase.MCPUsecase
}

func NewMCPHandler(service *usecase.MCPUsecase) *MCPHandler {
	return &MCPHandler{service: service}
}

// Server Registry

func (h *MCPHandler) ListServers(c *gin.Context) {
	servers, err := h.service.ListServers(c.Request.Context())
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, servers)
}

// Connections

func (h *MCPHandler) ListConnections(c *gin.Context) {
	conns, err := h.service.ListConnections(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, conns)
}

func (h *MCPHandler) Connect(c *gin.Context) {
	var req request.ConnectServerInput
	if err := request.BindJSON(c, &req); err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, "invalid input")
		return
	}

	conn, err := h.service.ConnectServer(c.Request.Context(), middleware.GetCurrentUserID(c), req.ServerID, req.ConfigJSON)
	if err != nil {
		response.ErrorMessage(c, http.StatusUnprocessableEntity, err.Error())
		return
	}
	response.Created(c, conn)
}

func (h *MCPHandler) Disconnect(c *gin.Context) {
	id, ok := childID(c, "id", "invalid connection id")
	if !ok {
		return
	}

	if err := h.service.DisconnectServer(c.Request.Context(), middleware.GetCurrentUserID(c), id); err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, response.Status{Status: "disconnected"})
}

func (h *MCPHandler) GetConnectionTools(c *gin.Context) {
	id, ok := childID(c, "id", "invalid connection id")
	if !ok {
		return
	}

	perms, err := h.service.GetConnectionTools(c.Request.Context(), middleware.GetCurrentUserID(c), id)
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, perms)
}

func (h *MCPHandler) UpdatePermission(c *gin.Context) {
	id, ok := childID(c, "id", "invalid connection id")
	if !ok {
		return
	}

	var req request.UpdatePermissionInput
	if err := request.BindJSON(c, &req); err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, "invalid permission input")
		return
	}

	if err := h.service.UpdateToolPermission(c.Request.Context(), middleware.GetCurrentUserID(c), id, req.ToolID, req.Granted); err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, response.Status{Status: "updated"})
}

func (h *MCPHandler) ListAuditLogs(c *gin.Context) {
	limit := 50
	if lStr := c.Query("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil {
			limit = l
		}
	}

	logs, err := h.service.ListAuditLogs(c.Request.Context(), middleware.GetCurrentUserID(c), limit)
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, logs)
}

// HandleRPC serves the first-party MCP JSON-RPC 2.0 endpoint (PRD Section
// 37). Replies are raw JSON-RPC objects, not enveloped.
func (h *MCPHandler) HandleRPC(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		response.JSON(c, http.StatusBadRequest, response.StringError{Error: "failed to read request body"})
		return
	}

	resp, err := h.service.HandleJSONRPC(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP(), body)
	if err != nil {
		response.JSON(c, http.StatusInternalServerError, response.StringError{Error: err.Error()})
		return
	}
	if resp == nil {
		// JSON-RPC notification (e.g. notifications/initialized): no body.
		c.Status(http.StatusAccepted)
		return
	}
	response.JSON(c, http.StatusOK, resp)
}
