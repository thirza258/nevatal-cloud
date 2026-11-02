package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type SSHKeyHandler struct {
	keys *usecase.SSHKeyUsecase
}

func NewSSHKeyHandler(keys *usecase.SSHKeyUsecase) *SSHKeyHandler {
	return &SSHKeyHandler{keys: keys}
}

func (h *SSHKeyHandler) List(c *gin.Context) {
	keys, err := h.keys.ListKeys(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, keys)
}

func (h *SSHKeyHandler) Create(c *gin.Context) {
	var req request.CreateSSHKeyRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	key, err := h.keys.CreateKey(c.Request.Context(), middleware.GetCurrentUserID(c), req.Name, req.PublicKey, c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusUnprocessableEntity, "KEY_INVALID", err.Error())
		return
	}
	response.Created(c, key)
}

func (h *SSHKeyHandler) Rename(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid key ID")
		return
	}

	var req request.RenameSSHKeyRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}

	if err := h.keys.RenameKey(c.Request.Context(), middleware.GetCurrentUserID(c), id, req.Name); err != nil {
		response.Error(c, http.StatusBadRequest, "RENAME_FAILED", err.Error())
		return
	}
	response.OKMessage(c, "Key renamed successfully")
}

func (h *SSHKeyHandler) Delete(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid key ID")
		return
	}

	if err := h.keys.DeleteKey(c.Request.Context(), middleware.GetCurrentUserID(c), id, c.ClientIP()); err != nil {
		response.Error(c, http.StatusNotFound, "DELETE_FAILED", err.Error())
		return
	}
	response.OKMessage(c, "Key removed successfully")
}
