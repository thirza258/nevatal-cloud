package handler

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"

	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

type StorageHandler struct {
	storage *usecase.StorageUsecase
}

func NewStorageHandler(storage *usecase.StorageUsecase) *StorageHandler {
	return &StorageHandler{storage: storage}
}

func (h *StorageHandler) GetOverview(c *gin.Context) {
	overview, err := h.storage.GetOverview(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, overview)
}

func (h *StorageHandler) ListBuckets(c *gin.Context) {
	buckets, err := h.storage.ListBuckets(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, buckets)
}

func (h *StorageHandler) GetBucket(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid bucket ID")
		return
	}

	bucket, err := h.storage.GetOwnedBucket(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}
	response.OK(c, bucket)
}

func (h *StorageHandler) CreateBucket(c *gin.Context) {
	var req request.CreateBucketInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	bucket, err := h.storage.CreateBucket(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase())
	if err != nil {
		response.Error(c, http.StatusBadRequest, "CREATE_ERROR", err.Error())
		return
	}
	response.Created(c, bucket)
}

func (h *StorageHandler) UpdateBucket(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid bucket ID")
		return
	}

	var req request.UpdateBucketInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	bucket, err := h.storage.UpdateBucket(c.Request.Context(), id, middleware.GetCurrentUserID(c), req.ToUsecase())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, bucket)
}

func (h *StorageHandler) DeleteBucket(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid bucket ID")
		return
	}

	if err := h.storage.DeleteBucket(c.Request.Context(), id, middleware.GetCurrentUserID(c)); err != nil {
		response.Error(c, http.StatusBadRequest, "DELETE_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Bucket deleted successfully"})
}

func (h *StorageHandler) ListObjects(c *gin.Context) {
	bucketID, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid bucket ID")
		return
	}

	objects, err := h.storage.ListObjects(c.Request.Context(), bucketID, middleware.GetCurrentUserID(c), c.Query("prefix"))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.OK(c, objects)
}

func (h *StorageHandler) UploadObject(c *gin.Context) {
	bucketID, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid bucket ID")
		return
	}

	bucket, err := h.storage.GetOwnedBucket(c.Request.Context(), bucketID, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "FILE_REQUIRED", "File is required")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	obj, err := h.storage.SaveObject(c.Request.Context(), bucket, c.PostForm("folder_prefix"), header.Filename, contentType, file)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "UPLOAD_ERROR", err.Error())
		return
	}
	response.Created(c, obj)
}

func (h *StorageHandler) CreateFolder(c *gin.Context) {
	bucketID, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid bucket ID")
		return
	}

	bucket, err := h.storage.GetOwnedBucket(c.Request.Context(), bucketID, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}

	var req request.CreateFolderRequest
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	obj, err := h.storage.CreateFolder(c.Request.Context(), bucket, req.FolderPrefix, req.FolderName)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "FOLDER_ERROR", err.Error())
		return
	}
	response.Created(c, obj)
}

func (h *StorageHandler) DownloadObject(c *gin.Context) {
	objectID, err := request.UUIDParam(c, "object_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	obj, file, err := h.storage.OpenObject(c.Request.Context(), objectID, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	defer file.Close()

	streamAttachment(c, obj, file)
}

func (h *StorageHandler) DeleteObject(c *gin.Context) {
	objectID, err := request.UUIDParam(c, "object_id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid object ID")
		return
	}

	if err := h.storage.DeleteObject(c.Request.Context(), objectID, middleware.GetCurrentUserID(c)); err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Object deleted successfully"})
}

func (h *StorageHandler) ListAccessKeys(c *gin.Context) {
	keys, err := h.storage.ListAccessKeys(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, keys)
}

func (h *StorageHandler) CreateAccessKey(c *gin.Context) {
	var req request.CreateAccessKeyInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	key, err := h.storage.CreateAccessKey(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase())
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.Created(c, key)
}

func (h *StorageHandler) RevokeAccessKey(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid key ID")
		return
	}

	if err := h.storage.RevokeAccessKey(c.Request.Context(), id, middleware.GetCurrentUserID(c)); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Access key revoked successfully"})
}

func (h *StorageHandler) CreateSharedLink(c *gin.Context) {
	var req request.CreateSharedLinkInput
	if err := request.BindJSON(c, &req); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	link, err := h.storage.CreateSharedLink(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.Created(c, link)
}

func (h *StorageHandler) ListSharedLinks(c *gin.Context) {
	links, err := h.storage.ListSharedLinks(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.OK(c, links)
}

func (h *StorageHandler) DeleteSharedLink(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.Error(c, http.StatusBadRequest, "INVALID_ID", "Invalid link ID")
		return
	}

	if err := h.storage.DeleteSharedLink(c.Request.Context(), id, middleware.GetCurrentUserID(c)); err != nil {
		response.Error(c, http.StatusInternalServerError, "SERVER_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Shared link deleted successfully"})
}

// GetSharedLink: public metadata of a shared link.
func (h *StorageHandler) GetSharedLink(c *gin.Context) {
	link, err := h.storage.GetSharedLink(c.Request.Context(), c.Param("token"))
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, response.NewSharedLinkInfo(link))
}

// DownloadSharedLink: public download; a password-protected link takes the
// password from ?password= or the X-Share-Password header.
func (h *StorageHandler) DownloadSharedLink(c *gin.Context) {
	password := c.Query("password")
	if password == "" {
		password = c.GetHeader("X-Share-Password")
	}

	link, file, err := h.storage.OpenSharedLink(c.Request.Context(), c.Param("token"), password)
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}
	defer file.Close()

	streamAttachment(c, link.Object, file)
}

// streamAttachment writes an object as a file download.
func streamAttachment(c *gin.Context, obj *models.StorageObject, file io.Reader) {
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(obj.Filename)))
	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", obj.SizeBytes))
	_, _ = io.Copy(c.Writer, file)
}
