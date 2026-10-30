package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type StorageHandler struct {
	storageService *services.StorageService
}

func NewStorageHandler(storageService *services.StorageService) *StorageHandler {
	return &StorageHandler{
		storageService: storageService,
	}
}

func (h *StorageHandler) GetOverview(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	overview, err := h.storageService.GetOverview(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": overview})
}

func (h *StorageHandler) ListBuckets(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	buckets, err := h.storageService.ListBuckets(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": buckets})
}

func (h *StorageHandler) GetBucket(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid bucket ID"}})
		return
	}

	bucket, err := h.storageService.GetBucket(id, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Bucket not found"}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bucket})
}

func (h *StorageHandler) CreateBucket(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input services.CreateBucketInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	bucket, err := h.storageService.CreateBucket(userID, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "CREATE_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": bucket})
}

func (h *StorageHandler) UpdateBucket(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid bucket ID"}})
		return
	}

	var input services.UpdateBucketInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	bucket, err := h.storageService.UpdateBucket(id, userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": bucket})
}

func (h *StorageHandler) DeleteBucket(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid bucket ID"}})
		return
	}

	if err := h.storageService.DeleteBucket(id, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "DELETE_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Bucket deleted successfully"})
}

func (h *StorageHandler) ListObjects(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	bucketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid bucket ID"}})
		return
	}

	// Verify bucket ownership
	if _, err := h.storageService.GetBucket(bucketID, userID); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Bucket not found"}})
		return
	}

	prefix := c.Query("prefix")
	objects, err := h.storageService.ListObjects(bucketID, prefix)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": objects})
}

func (h *StorageHandler) UploadObject(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	bucketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid bucket ID"}})
		return
	}

	bucket, err := h.storageService.GetBucket(bucketID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Bucket not found"}})
		return
	}

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "FILE_REQUIRED", "message": "File is required"}})
		return
	}
	defer file.Close()

	folderPrefix := c.PostForm("folder_prefix")
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	obj, err := h.storageService.SaveObject(bucket, folderPrefix, header.Filename, contentType, file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "UPLOAD_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": obj})
}

func (h *StorageHandler) CreateFolder(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	bucketID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid bucket ID"}})
		return
	}

	bucket, err := h.storageService.GetBucket(bucketID, userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Bucket not found"}})
		return
	}

	var body struct {
		FolderPrefix string `json:"folder_prefix"`
		FolderName   string `json:"folder_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	obj, err := h.storageService.CreateFolder(bucket, body.FolderPrefix, body.FolderName)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "FOLDER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"data": obj})
}

func (h *StorageHandler) DownloadObject(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	objectID, err := uuid.Parse(c.Param("object_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid object ID"}})
		return
	}

	obj, err := h.storageService.GetObject(objectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Object not found"}})
		return
	}

	// Verify bucket ownership
	if _, err := h.storageService.GetBucket(obj.BucketID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	file, err := h.storageService.OpenObjectFile(obj)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "FILE_NOT_FOUND", "message": "File not found on storage"}})
		return
	}
	defer file.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(obj.Filename)))
	c.Header("Content-Type", obj.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", obj.SizeBytes))
	_, _ = io.Copy(c.Writer, file)
}

func (h *StorageHandler) DeleteObject(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	objectID, err := uuid.Parse(c.Param("object_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid object ID"}})
		return
	}

	obj, err := h.storageService.GetObject(objectID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": "Object not found"}})
		return
	}

	if _, err := h.storageService.GetBucket(obj.BucketID, userID); err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": gin.H{"code": "FORBIDDEN", "message": "Access denied"}})
		return
	}

	if err := h.storageService.DeleteObject(objectID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Object deleted successfully"})
}

func (h *StorageHandler) ListAccessKeys(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	keys, err := h.storageService.ListAccessKeys(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": keys})
}

func (h *StorageHandler) CreateAccessKey(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input services.CreateAccessKeyInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	key, err := h.storageService.CreateAccessKey(userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": key})
}

func (h *StorageHandler) RevokeAccessKey(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid key ID"}})
		return
	}

	if err := h.storageService.RevokeAccessKey(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Access key revoked successfully"})
}

func (h *StorageHandler) CreateSharedLink(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input services.CreateSharedLinkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "VALIDATION_ERROR", "message": err.Error()}})
		return
	}

	link, err := h.storageService.CreateSharedLink(userID, input)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": link})
}

func (h *StorageHandler) ListSharedLinks(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	links, err := h.storageService.ListSharedLinks(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": links})
}

func (h *StorageHandler) DeleteSharedLink(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid link ID"}})
		return
	}

	if err := h.storageService.DeleteSharedLink(id, userID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"code": "SERVER_ERROR", "message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Shared link deleted successfully"})
}

// Public Shared Link Endpoint
func (h *StorageHandler) GetSharedLink(c *gin.Context) {
	token := c.Param("token")
	link, err := h.storageService.GetSharedLink(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": gin.H{
			"id":           link.ID,
			"has_password": link.HasPassword,
			"access_type":  link.AccessType,
			"expires_at":   link.ExpiresAt,
			"filename":     link.Object.Filename,
			"content_type": link.Object.ContentType,
			"size_bytes":   link.Object.SizeBytes,
		},
	})
}

func (h *StorageHandler) DownloadSharedLink(c *gin.Context) {
	token := c.Param("token")
	link, err := h.storageService.GetSharedLink(token)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "NOT_FOUND", "message": err.Error()}})
		return
	}

	if link.HasPassword {
		providedPass := c.Query("password")
		if providedPass == "" {
			providedPass = c.GetHeader("X-Share-Password")
		}
		if providedPass == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "PASSWORD_REQUIRED", "message": "This shared link requires a password"}})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(link.Password), []byte(providedPass)); err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "INVALID_PASSWORD", "message": "Invalid link password"}})
			return
		}
	}

	file, err := h.storageService.OpenObjectFile(link.Object)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "FILE_NOT_FOUND", "message": "File not found"}})
		return
	}
	defer file.Close()

	h.storageService.IncrementSharedDownload(link.ID)

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filepath.Base(link.Object.Filename)))
	c.Header("Content-Type", link.Object.ContentType)
	c.Header("Content-Length", fmt.Sprintf("%d", link.Object.SizeBytes))
	_, _ = io.Copy(c.Writer, file)
}
