package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

type ISOHandler struct {
	isos *usecase.ISOUsecase
}

func NewISOHandler(isos *usecase.ISOUsecase) *ISOHandler {
	return &ISOHandler{isos: isos}
}

// List serves both the public catalog (GET /operating-systems) and the admin
// list (GET /admin/isos); ?all=true from an admin includes unpublished images.
func (h *ISOHandler) List(c *gin.Context) {
	var images []models.OSImage
	var err error

	if c.Query("all") == "true" && middleware.IsAdmin(c) {
		images, err = h.isos.ListAllAdmin(c.Request.Context())
	} else {
		images, err = h.isos.ListPublished(c.Request.Context())
	}

	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, images)
}

func (h *ISOHandler) Upload(c *gin.Context) {
	var req request.UploadISOInput
	if err := c.ShouldBind(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	file, err := c.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "FILE_REQUIRED", "ISO file is required")
		return
	}

	img, err := h.isos.ProcessUpload(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase(), file, c.ClientIP())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "UPLOAD_FAILED", err.Error())
		return
	}
	response.Created(c, img)
}

// SetPublished answers with a bare {"message": ...} body (no data envelope).
func (h *ISOHandler) SetPublished(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid ISO ID")
		return
	}

	var req request.SetISOVisibilityRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	if err := h.isos.UpdateVisibility(c.Request.Context(), id, middleware.GetCurrentUserID(c), req.Visibility, c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Visibility updated successfully to " + req.Visibility})
}

// Delete answers with a bare {"message": ...} body (no data envelope).
func (h *ISOHandler) Delete(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid ISO ID")
		return
	}

	if err := h.isos.Delete(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.ClientIP()); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "ISO image deleted successfully"})
}
