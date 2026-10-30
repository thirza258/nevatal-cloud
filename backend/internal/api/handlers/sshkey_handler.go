package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type SSHKeyHandler struct {
	sshService   *services.SSHKeyService
	auditService *services.AuditService
}

func NewSSHKeyHandler(sshService *services.SSHKeyService, auditService *services.AuditService) *SSHKeyHandler {
	return &SSHKeyHandler{
		sshService:   sshService,
		auditService: auditService,
	}
}

type CreateSSHKeyRequest struct {
	Name      string `json:"name" binding:"required"`
	PublicKey string `json:"public_key" binding:"required"`
}

type RenameSSHKeyRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *SSHKeyHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	keys, err := h.sshService.ListKeys(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": keys,
	})
}

func (h *SSHKeyHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var req CreateSSHKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
		return
	}

	key, err := h.sshService.CreateKey(userID, req.Name, req.PublicKey)
	if err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{"code": "KEY_INVALID", "message": err.Error()}})
		return
	}

	h.auditService.Log(&userID, "ssh_key.create", "ssh_key", &key.ID, "success", c.ClientIP(), map[string]string{"name": key.Name})

	c.JSON(http.StatusCreated, gin.H{
		"data": key,
	})
}

func (h *SSHKeyHandler) Rename(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid key ID"}})
		return
	}

	var req RenameSSHKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()}})
		return
	}

	if err := h.sshService.RenameKey(userID, id, req.Name); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "RENAME_FAILED", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{"message": "Key renamed successfully"},
	})
}

func (h *SSHKeyHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid key ID"}})
		return
	}

	if err := h.sshService.DeleteKey(userID, id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "DELETE_FAILED", "message": err.Error()}})
		return
	}

	h.auditService.Log(&userID, "ssh_key.delete", "ssh_key", &id, "success", c.ClientIP(), nil)

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{"message": "Key removed successfully"},
	})
}
