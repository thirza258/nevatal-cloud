package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type AndroidHandler struct {
	svc *services.AndroidService
}

func NewAndroidHandler(svc *services.AndroidService) *AndroidHandler {
	return &AndroidHandler{svc: svc}
}

// Instances

func (h *AndroidHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	status := c.Query("status")

	instances, err := h.svc.List(userID, isAdmin, status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": instances})
}

func (h *AndroidHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input services.CreateAndroidInstanceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	instance, err := h.svc.Create(userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": instance})
}

func (h *AndroidHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	instance, err := h.svc.Get(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": instance})
}

func (h *AndroidHandler) Update(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	var input services.UpdateAndroidInstanceInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	instance, err := h.svc.Update(id, userID, isAdmin, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": instance})
}

func (h *AndroidHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	if err := h.svc.Delete(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "android instance deleted successfully"})
}

// Lifecycle

func (h *AndroidHandler) Start(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	if err := h.svc.Start(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "android instance started"})
}

func (h *AndroidHandler) Stop(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	if err := h.svc.Stop(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "android instance stopped"})
}

func (h *AndroidHandler) Restart(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	if err := h.svc.Restart(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "android instance restarted"})
}

func (h *AndroidHandler) Suspend(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	if err := h.svc.Suspend(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "android instance suspended"})
}

func (h *AndroidHandler) Resume(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	if err := h.svc.Resume(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "android instance resumed"})
}

func (h *AndroidHandler) Reinstall(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	if err := h.svc.Reinstall(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "android instance reinstalled"})
}

// Telemetry & Logs

func (h *AndroidHandler) Metrics(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	metrics, err := h.svc.GetMetrics(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

func (h *AndroidHandler) Logs(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	lines := 100
	if lStr := c.Query("lines"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			lines = l
		}
	}

	logs, err := h.svc.GetLogs(id, userID, isAdmin, lines)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"logs": logs}})
}

// Plans & Images

func (h *AndroidHandler) ListPlans(c *gin.Context) {
	plans := h.svc.ListPlans()
	c.JSON(http.StatusOK, gin.H{"data": plans})
}

func (h *AndroidHandler) ListImages(c *gin.Context) {
	images, err := h.svc.ListImages(true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": images})
}

func (h *AndroidHandler) GetImage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid image id"}})
		return
	}
	image, err := h.svc.GetImage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": image})
}

// Applications

func (h *AndroidHandler) ListApplications(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	apps, err := h.svc.ListApplications(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": apps})
}

func (h *AndroidHandler) InstallAPK(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	var input services.InstallAPKInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	app, err := h.svc.InstallAPK(id, userID, isAdmin, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": app})
}

func (h *AndroidHandler) UninstallApplication(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}
	packageName := c.Param("packageName")

	if err := h.svc.UninstallApplication(id, packageName, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "application uninstalled"})
}

func (h *AndroidHandler) GetApplication(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}
	packageName := c.Param("packageName")

	app, err := h.svc.GetApplication(id, packageName, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": app})
}

// Remote Sessions

func (h *AndroidHandler) CreateSession(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	session, err := h.svc.CreateSession(id, userID, isAdmin, c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": session})
}

func (h *AndroidHandler) ListSessions(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	sessions, err := h.svc.ListSessions(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": sessions})
}

func (h *AndroidHandler) TerminateSession(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}
	sessionID, err := uuid.Parse(c.Param("sessionId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid session id"}})
		return
	}

	if err := h.svc.TerminateSession(id, sessionID, userID, isAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "session terminated"})
}

// Backups

func (h *AndroidHandler) ListBackups(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	backups, err := h.svc.ListBackups(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": backups})
}

func (h *AndroidHandler) CreateBackup(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}

	var body struct {
		BackupName string `json:"backup_name"`
	}
	_ = c.ShouldBindJSON(&body)

	backup, err := h.svc.CreateBackup(id, userID, isAdmin, body.BackupName, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": backup})
}

func (h *AndroidHandler) RestoreBackup(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}
	backupID, err := uuid.Parse(c.Param("backupId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid backup id"}})
		return
	}

	if err := h.svc.RestoreBackup(id, backupID, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "backup restored successfully, instance restarting"})
}

func (h *AndroidHandler) DeleteBackup(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid instance id"}})
		return
	}
	backupID, err := uuid.Parse(c.Param("backupId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid backup id"}})
		return
	}

	if err := h.svc.DeleteBackup(id, backupID, userID, isAdmin); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "backup deleted"})
}

// Admin Image Registry per PRD Section 69

func (h *AndroidHandler) AdminListImages(c *gin.Context) {
	images, err := h.svc.ListImages(false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": images})
}

func (h *AndroidHandler) AdminCreateImage(c *gin.Context) {
	var input services.CreateAndroidImageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	image, err := h.svc.CreateImage(input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": image})
}

func (h *AndroidHandler) AdminUpdateImage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid image id"}})
		return
	}
	var input services.UpdateAndroidImageInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	image, err := h.svc.UpdateImage(id, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": image})
}

func (h *AndroidHandler) AdminDeleteImage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid image id"}})
		return
	}
	if err := h.svc.DeleteImage(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "android image deleted"})
}

func (h *AndroidHandler) AdminPublishImage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid image id"}})
		return
	}
	image, err := h.svc.PublishImage(id, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": image})
}

func (h *AndroidHandler) AdminUnpublishImage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid image id"}})
		return
	}
	image, err := h.svc.PublishImage(id, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": image})
}

func (h *AndroidHandler) AdminVerifyImage(c *gin.Context) {
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid image id"}})
		return
	}
	image, err := h.svc.VerifyImage(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": image})
}
