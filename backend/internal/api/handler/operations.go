package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// OperationsHandler serves the Phase 2-4 surfaces: managed databases,
// notifications, scheduled checks, automation policies, templates,
// diagnostics, the advisor, topology and agent roles.
type OperationsHandler struct {
	databases     *usecase.DatabaseUsecase
	notifications *usecase.NotificationUsecase
	monitoring    *usecase.MonitoringUsecase
	policies      *usecase.AutomationPolicyUsecase
	orchestrator  *usecase.OrchestratorUsecase
}

func NewOperationsHandler(databases *usecase.DatabaseUsecase, notifications *usecase.NotificationUsecase,
	monitoring *usecase.MonitoringUsecase, policies *usecase.AutomationPolicyUsecase,
	orchestrator *usecase.OrchestratorUsecase) *OperationsHandler {
	return &OperationsHandler{databases: databases, notifications: notifications, monitoring: monitoring, policies: policies, orchestrator: orchestrator}
}

// --- Managed databases -----------------------------------------------------

func (h *OperationsHandler) DatabasePlans(c *gin.Context) {
	response.OK(c, h.databases.Plans())
}

func (h *OperationsHandler) ListDatabases(c *gin.Context) {
	list, err := h.databases.List(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, list)
}

func (h *OperationsHandler) GetDatabase(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	db, err := h.databases.Get(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, db)
}

// CreateDatabase returns the generated password exactly once.
func (h *OperationsHandler) CreateDatabase(c *gin.Context) {
	var in usecase.CreateDatabaseInput
	if !bindRequiredJSON(c, &in) {
		return
	}
	db, password, err := h.databases.Create(c.Request.Context(), middleware.GetCurrentUserID(c), in, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Created(c, map[string]any{"database": db, "password": password})
}

func (h *OperationsHandler) DeleteDatabase(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	if err := h.databases.Delete(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.ClientIP()); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.NoContent(c)
}

func (h *OperationsHandler) DatabasePower(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	db, err := h.databases.UpdatePower(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.Param("action"), c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, db)
}

func (h *OperationsHandler) DatabaseCredentials(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	creds, err := h.databases.Credentials(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, creds)
}

func (h *OperationsHandler) DatabaseBackups(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	list, err := h.databases.ListBackups(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, list)
}

func (h *OperationsHandler) CreateDatabaseBackup(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	var in struct {
		Name string `json:"name"`
	}
	if !bindOptionalJSON(c, &in) {
		return
	}
	backup, err := h.databases.CreateBackup(c.Request.Context(), id, middleware.GetCurrentUserID(c), in.Name, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Created(c, backup)
}

func (h *OperationsHandler) RestoreDatabaseBackup(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	backupID, ok := uuidParam(c, "backupId", "backup id")
	if !ok {
		return
	}
	if err := h.databases.RestoreBackup(c.Request.Context(), id, backupID, middleware.GetCurrentUserID(c), c.ClientIP()); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OKMessage(c, "backup restored")
}

func (h *OperationsHandler) DeleteDatabaseBackup(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	backupID, ok := uuidParam(c, "backupId", "backup id")
	if !ok {
		return
	}
	if err := h.databases.DeleteBackup(c.Request.Context(), id, backupID, middleware.GetCurrentUserID(c), c.ClientIP()); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.NoContent(c)
}

func (h *OperationsHandler) DatabaseLogs(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	lines := 100
	if raw := c.Query("lines"); raw != "" {
		if n, err := parseInt(raw); err == nil {
			lines = n
		}
	}
	logs, err := h.databases.Logs(c.Request.Context(), id, middleware.GetCurrentUserID(c), lines)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, map[string]any{"logs": logs})
}

func (h *OperationsHandler) DatabaseMetrics(c *gin.Context) {
	id, ok := uuidParam(c, "id", "database id")
	if !ok {
		return
	}
	m, err := h.databases.Metrics(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, m)
}

// --- Notifications ---------------------------------------------------------

func (h *OperationsHandler) ListNotifications(c *gin.Context) {
	unread := c.Query("unread") == "true"
	limit := 50
	if raw := c.Query("limit"); raw != "" {
		if n, err := parseInt(raw); err == nil {
			limit = n
		}
	}
	list, err := h.notifications.List(c.Request.Context(), middleware.GetCurrentUserID(c), unread, limit)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	count, _ := h.notifications.UnreadCount(c.Request.Context(), middleware.GetCurrentUserID(c))
	response.OK(c, map[string]any{"unread_count": count, "notifications": list})
}

func (h *OperationsHandler) NotificationUnreadCount(c *gin.Context) {
	count, err := h.notifications.UnreadCount(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, map[string]any{"unread_count": count})
}

func (h *OperationsHandler) MarkNotificationRead(c *gin.Context) {
	id, ok := uuidParam(c, "id", "notification id")
	if !ok {
		return
	}
	if err := h.notifications.MarkRead(c.Request.Context(), id, middleware.GetCurrentUserID(c)); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OKMessage(c, "notification marked read")
}

func (h *OperationsHandler) MarkAllNotificationsRead(c *gin.Context) {
	n, err := h.notifications.MarkAllRead(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, map[string]any{"marked": n})
}

// --- Scheduled checks ------------------------------------------------------

func (h *OperationsHandler) ListChecks(c *gin.Context) {
	list, err := h.monitoring.ListChecks(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, list)
}

func (h *OperationsHandler) CreateCheck(c *gin.Context) {
	var in usecase.CreateCheckInput
	if !bindRequiredJSON(c, &in) {
		return
	}
	view, err := h.monitoring.CreateCheck(c.Request.Context(), middleware.GetCurrentUserID(c), in, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Created(c, view)
}

func (h *OperationsHandler) UpdateCheck(c *gin.Context) {
	id, ok := uuidParam(c, "id", "check id")
	if !ok {
		return
	}
	var in usecase.UpdateCheckInput
	if !bindRequiredJSON(c, &in) {
		return
	}
	view, err := h.monitoring.UpdateCheck(c.Request.Context(), id, middleware.GetCurrentUserID(c), in, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, view)
}

func (h *OperationsHandler) DeleteCheck(c *gin.Context) {
	id, ok := uuidParam(c, "id", "check id")
	if !ok {
		return
	}
	if err := h.monitoring.DeleteCheck(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.ClientIP()); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.NoContent(c)
}

func (h *OperationsHandler) RunCheck(c *gin.Context) {
	id, ok := uuidParam(c, "id", "check id")
	if !ok {
		return
	}
	view, err := h.monitoring.RunNow(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, view)
}

// --- Automation policies ---------------------------------------------------

func (h *OperationsHandler) ListPolicies(c *gin.Context) {
	list, err := h.policies.List(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, list)
}

func (h *OperationsHandler) CreatePolicy(c *gin.Context) {
	var in usecase.CreatePolicyInput
	if !bindRequiredJSON(c, &in) {
		return
	}
	view, err := h.policies.Create(c.Request.Context(), middleware.GetCurrentUserID(c), in, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Created(c, view)
}

func (h *OperationsHandler) UpdatePolicy(c *gin.Context) {
	id, ok := uuidParam(c, "id", "policy id")
	if !ok {
		return
	}
	var in usecase.CreatePolicyInput
	if !bindRequiredJSON(c, &in) {
		return
	}
	view, err := h.policies.Update(c.Request.Context(), id, middleware.GetCurrentUserID(c), in, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, view)
}

func (h *OperationsHandler) DeletePolicy(c *gin.Context) {
	id, ok := uuidParam(c, "id", "policy id")
	if !ok {
		return
	}
	if err := h.policies.Delete(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.ClientIP()); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.NoContent(c)
}

// --- Templates -------------------------------------------------------------

func (h *OperationsHandler) ListTemplates(c *gin.Context) {
	list, err := h.orchestrator.ListTemplates(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, list)
}

func (h *OperationsHandler) GetTemplate(c *gin.Context) {
	id, ok := uuidParam(c, "id", "template id")
	if !ok {
		return
	}
	view, err := h.orchestrator.GetTemplate(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, view)
}

func (h *OperationsHandler) CreateTemplate(c *gin.Context) {
	var in usecase.SaveTemplateInput
	if !bindRequiredJSON(c, &in) {
		return
	}
	view, err := h.orchestrator.SaveTemplate(c.Request.Context(), middleware.GetCurrentUserID(c), in, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Created(c, view)
}

func (h *OperationsHandler) UpdateTemplate(c *gin.Context) {
	id, ok := uuidParam(c, "id", "template id")
	if !ok {
		return
	}
	var in usecase.SaveTemplateInput
	if !bindRequiredJSON(c, &in) {
		return
	}
	view, err := h.orchestrator.UpdateTemplate(c.Request.Context(), id, middleware.GetCurrentUserID(c), in, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, view)
}

func (h *OperationsHandler) DeleteTemplate(c *gin.Context) {
	id, ok := uuidParam(c, "id", "template id")
	if !ok {
		return
	}
	if err := h.orchestrator.DeleteTemplate(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.ClientIP()); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.NoContent(c)
}

func (h *OperationsHandler) DeployTemplate(c *gin.Context) {
	id, ok := uuidParam(c, "id", "template id")
	if !ok {
		return
	}
	var in struct {
		Parameters map[string]any `json:"parameters"`
	}
	if !bindOptionalJSON(c, &in) {
		return
	}
	plan, err := h.orchestrator.DeployTemplate(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), id, in.Parameters)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Created(c, map[string]any{"plan_id": plan.ID, "conversation_id": plan.ConversationID, "status": plan.Status})
}

// --- Diagnostics, advisor, topology, roles ---------------------------------

func (h *OperationsHandler) Diagnose(c *gin.Context) {
	id, ok := uuidParam(c, "id", "resource id")
	if !ok {
		return
	}
	res, err := h.orchestrator.Diagnose(c.Request.Context(), middleware.GetCurrentUserID(c), c.Param("resourceType"), id)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, res)
}

func (h *OperationsHandler) Remediate(c *gin.Context) {
	id, ok := uuidParam(c, "id", "resource id")
	if !ok {
		return
	}
	var in usecase.RemediationInput
	if !bindRequiredJSON(c, &in) {
		return
	}
	plan, err := h.orchestrator.Remediate(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.Param("resourceType"), id, in)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Created(c, plan)
}

func (h *OperationsHandler) Optimize(c *gin.Context) {
	report, err := h.orchestrator.Optimize(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, report)
}

func (h *OperationsHandler) Costs(c *gin.Context) {
	report, err := h.orchestrator.Costs(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, report)
}

func (h *OperationsHandler) Topology(c *gin.Context) {
	view, err := h.orchestrator.Topology(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, view)
}

func (h *OperationsHandler) AgentRoles(c *gin.Context) {
	response.OK(c, h.orchestrator.AgentRoles())
}

func parseInt(raw string) (int, error) {
	return strconv.Atoi(raw)
}
