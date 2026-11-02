package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// AndroidHandler serves /android and /admin/android. Its errors use the
// code-less body {"error": {"message": ...}}.
type AndroidHandler struct {
	svc *usecase.AndroidUsecase
}

func NewAndroidHandler(svc *usecase.AndroidUsecase) *AndroidHandler {
	return &AndroidHandler{svc: svc}
}

// instanceID parses :id or writes the 400 reply.
func instanceID(c *gin.Context) (uuid.UUID, bool) {
	return childID(c, "id", "invalid instance id")
}

// imageID parses :id of an image or writes the 400 reply.
func imageID(c *gin.Context) (uuid.UUID, bool) {
	return childID(c, "id", "invalid image id")
}

// Instances

func (h *AndroidHandler) List(c *gin.Context) {
	instances, err := h.svc.List(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.Query("status"))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, instances)
}

func (h *AndroidHandler) Create(c *gin.Context) {
	var req request.CreateAndroidInstanceInput
	if !bindMessage(c, &req) {
		return
	}

	instance, err := h.svc.Create(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, instance)
}

func (h *AndroidHandler) Get(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	instance, err := h.svc.Get(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, instance)
}

func (h *AndroidHandler) Update(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	var req request.UpdateAndroidInstanceInput
	if !bindMessage(c, &req) {
		return
	}

	instance, err := h.svc.Update(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.ToUsecase())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, instance)
}

// androidAction is an AndroidUsecase lifecycle method.
type androidAction func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error

// lifecycle serves the endpoints that act on an instance and answer with a
// bare {"message": ...} body; failures are 500 with the message.
func (h *AndroidHandler) lifecycle(c *gin.Context, message string, action androidAction) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	if err := action(c, id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c)); err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: message})
}

func (h *AndroidHandler) Delete(c *gin.Context) {
	h.lifecycle(c, "android instance deleted successfully", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.Delete(c.Request.Context(), id, userID, isAdmin, c.ClientIP())
	})
}

// Lifecycle

func (h *AndroidHandler) Start(c *gin.Context) {
	h.lifecycle(c, "android instance started", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.Start(c.Request.Context(), id, userID, isAdmin, c.ClientIP())
	})
}

func (h *AndroidHandler) Stop(c *gin.Context) {
	h.lifecycle(c, "android instance stopped", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.Stop(c.Request.Context(), id, userID, isAdmin, c.ClientIP())
	})
}

func (h *AndroidHandler) Restart(c *gin.Context) {
	h.lifecycle(c, "android instance restarted", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.Restart(c.Request.Context(), id, userID, isAdmin, c.ClientIP())
	})
}

func (h *AndroidHandler) Suspend(c *gin.Context) {
	h.lifecycle(c, "android instance suspended", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.Suspend(c.Request.Context(), id, userID, isAdmin, c.ClientIP())
	})
}

func (h *AndroidHandler) Resume(c *gin.Context) {
	h.lifecycle(c, "android instance resumed", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.Resume(c.Request.Context(), id, userID, isAdmin, c.ClientIP())
	})
}

func (h *AndroidHandler) Reinstall(c *gin.Context) {
	h.lifecycle(c, "android instance reinstalled", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.Reinstall(c.Request.Context(), id, userID, isAdmin, c.ClientIP())
	})
}

// Telemetry & Logs

func (h *AndroidHandler) Metrics(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	metrics, err := h.svc.GetMetrics(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, metrics)
}

func (h *AndroidHandler) Logs(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	lines := 100
	if lStr := c.Query("lines"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			lines = l
		}
	}

	logs, err := h.svc.GetLogs(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), lines)
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, response.AndroidLogs{Logs: logs})
}

// Plans & Images

func (h *AndroidHandler) ListPlans(c *gin.Context) {
	response.OK(c, h.svc.ListPlans())
}

func (h *AndroidHandler) ListImages(c *gin.Context) {
	h.listImages(c, true)
}

func (h *AndroidHandler) listImages(c *gin.Context, publishedOnly bool) {
	images, err := h.svc.ListImages(c.Request.Context(), publishedOnly)
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, images)
}

func (h *AndroidHandler) GetImage(c *gin.Context) {
	id, ok := imageID(c)
	if !ok {
		return
	}
	get := h.svc.GetPublishedImage
	if middleware.IsAdmin(c) {
		get = h.svc.GetImage
	}
	image, err := get(c.Request.Context(), id)
	if err != nil {
		response.ErrorMessage(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, image)
}

// Applications

func (h *AndroidHandler) ListApplications(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	apps, err := h.svc.ListApplications(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, apps)
}

func (h *AndroidHandler) InstallAPK(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	var req request.InstallAPKInput
	if !bindMessage(c, &req) {
		return
	}

	app, err := h.svc.InstallAPK(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, app)
}

func (h *AndroidHandler) UninstallApplication(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	if err := h.svc.UninstallApplication(c.Request.Context(), id, c.Param("packageName"), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP()); err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "application uninstalled"})
}

func (h *AndroidHandler) GetApplication(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	app, err := h.svc.GetApplication(c.Request.Context(), id, c.Param("packageName"), middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, app)
}

// Remote Sessions

func (h *AndroidHandler) CreateSession(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	session, err := h.svc.CreateSession(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP(), c.Request.UserAgent())
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Created(c, session)
}

func (h *AndroidHandler) ListSessions(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	sessions, err := h.svc.ListSessions(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, sessions)
}

func (h *AndroidHandler) TerminateSession(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}
	sessionID, ok := childID(c, "sessionId", "invalid session id")
	if !ok {
		return
	}

	if err := h.svc.TerminateSession(c.Request.Context(), id, sessionID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c)); err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "session terminated"})
}

// Backups

func (h *AndroidHandler) ListBackups(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	backups, err := h.svc.ListBackups(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, backups)
}

func (h *AndroidHandler) CreateBackup(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}

	var req request.CreateAndroidBackupRequest
	_ = c.ShouldBindJSON(&req)

	backup, err := h.svc.CreateBackup(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.BackupName, c.ClientIP())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, backup)
}

func (h *AndroidHandler) RestoreBackup(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}
	backupID, ok := childID(c, "backupId", "invalid backup id")
	if !ok {
		return
	}

	if err := h.svc.RestoreBackup(c.Request.Context(), id, backupID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP()); err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "backup restored successfully, instance restarting"})
}

func (h *AndroidHandler) DeleteBackup(c *gin.Context) {
	id, ok := instanceID(c)
	if !ok {
		return
	}
	backupID, ok := childID(c, "backupId", "invalid backup id")
	if !ok {
		return
	}

	if err := h.svc.DeleteBackup(c.Request.Context(), id, backupID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c)); err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "backup deleted"})
}

// Admin Image Registry per PRD Section 69

func (h *AndroidHandler) AdminListImages(c *gin.Context) {
	h.listImages(c, false)
}

func (h *AndroidHandler) AdminCreateImage(c *gin.Context) {
	var req request.CreateAndroidImageInput
	if !bindMessage(c, &req) {
		return
	}
	image, err := h.svc.CreateImage(c.Request.Context(), req.ToUsecase())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, image)
}

func (h *AndroidHandler) AdminUpdateImage(c *gin.Context) {
	id, ok := imageID(c)
	if !ok {
		return
	}
	var req request.UpdateAndroidImageInput
	if !bindMessage(c, &req) {
		return
	}
	image, err := h.svc.UpdateImage(c.Request.Context(), id, req.ToUsecase())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, image)
}

func (h *AndroidHandler) AdminDeleteImage(c *gin.Context) {
	id, ok := imageID(c)
	if !ok {
		return
	}
	if err := h.svc.DeleteImage(c.Request.Context(), id); err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "android image deleted"})
}

func (h *AndroidHandler) AdminPublishImage(c *gin.Context) {
	h.setPublished(c, true)
}

func (h *AndroidHandler) AdminUnpublishImage(c *gin.Context) {
	h.setPublished(c, false)
}

func (h *AndroidHandler) setPublished(c *gin.Context, publish bool) {
	id, ok := imageID(c)
	if !ok {
		return
	}
	image, err := h.svc.PublishImage(c.Request.Context(), id, publish)
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, image)
}

func (h *AndroidHandler) AdminVerifyImage(c *gin.Context) {
	id, ok := imageID(c)
	if !ok {
		return
	}
	image, err := h.svc.VerifyImage(c.Request.Context(), id)
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, image)
}
