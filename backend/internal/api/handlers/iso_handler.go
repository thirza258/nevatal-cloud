package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/services"
)

type ISOHandler struct {
	isoService *services.ISOService
}

func NewISOHandler(isoService *services.ISOService) *ISOHandler {
	return &ISOHandler{
		isoService: isoService,
	}
}

func (h *ISOHandler) List(c *gin.Context) {
	var images []models.OSImage
	var err error

	if c.Query("all") == "true" && middleware.IsAdmin(c) {
		images, err = h.isoService.ListAllAdmin()
	} else {
		images, err = h.isoService.ListPublished()
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": images})
}

func (h *ISOHandler) Upload(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	var input services.UploadISOInput
	if err := c.ShouldBind(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "FILE_REQUIRED", "message": "ISO file is required"}})
		return
	}

	img, err := h.isoService.ProcessUpload(userID, input, file, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "UPLOAD_FAILED", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": img})
}

func (h *ISOHandler) SetPublished(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid ISO ID"}})
		return
	}

	var body struct {
		Visibility string `json:"visibility" binding:"required"` // published, unpublished
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	if err := h.isoService.UpdateVisibility(id, userID, body.Visibility, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Visibility updated successfully to " + body.Visibility})
}

func (h *ISOHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid ISO ID"}})
		return
	}

	if err := h.isoService.Delete(id, userID, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "ISO image deleted successfully"})
}
