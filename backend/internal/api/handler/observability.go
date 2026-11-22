package handler

import (
	"context"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

type ObservabilityHandler struct {
	obs     *usecase.ObservabilityUsecase
	adminUC *usecase.AdminUsecase
	streams context.Context
}

func NewObservabilityHandler(obs *usecase.ObservabilityUsecase, adminUC *usecase.AdminUsecase, streams context.Context) *ObservabilityHandler {
	return &ObservabilityHandler{
		obs:     obs,
		adminUC: adminUC,
		streams: streams,
	}
}

// UserOverview: GET /api/v1/observability/overview
func (h *ObservabilityHandler) UserOverview(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	ov, err := h.obs.GetUserOverview(c.Request.Context(), tenantID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, ov)
}

// UserMetrics: GET /api/v1/observability/metrics
func (h *ObservabilityHandler) UserMetrics(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	ov, err := h.obs.GetUserOverview(c.Request.Context(), tenantID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, map[string]any{
		"cpu_usage_pct":     ov.CPUUsagePct,
		"ram_usage_pct":     ov.RAMUsagePct,
		"storage_usage_pct": ov.StorageUsagePct,
		"network_mbps":      ov.NetworkBandwidthMbps,
		"health_score":      ov.OverallHealthScore,
	})
}

// UserEvents: GET /api/v1/observability/events
func (h *ObservabilityHandler) UserEvents(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	eventType := c.Query("event_type")
	severity := c.Query("severity")
	corrID := c.Query("correlation_id")

	events, count, err := h.obs.QueryEvents(c.Request.Context(), &tenantID, nil, eventType, severity, corrID, limit, offset)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	if events == nil {
		events = []models.ObservabilityEvent{}
	}
	response.OK(c, map[string]any{
		"events": events,
		"total":  count,
	})
}

// UserAlerts: GET /api/v1/observability/alerts
func (h *ObservabilityHandler) UserAlerts(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	status := c.Query("status")
	alerts, err := h.obs.ListAlerts(c.Request.Context(), &tenantID, status)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	rules, err := h.obs.ListAlertRules(c.Request.Context(), &tenantID)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	if alerts == nil {
		alerts = []models.ObservabilityAlert{}
	}
	if rules == nil {
		rules = []models.ObservabilityAlertRule{}
	}
	var activeAlerts []models.ObservabilityAlert
	for _, a := range alerts {
		if a.Status != "resolved" {
			activeAlerts = append(activeAlerts, a)
		}
	}
	if activeAlerts == nil {
		activeAlerts = []models.ObservabilityAlert{}
	}
	response.OK(c, map[string]any{
		"alerts":        alerts,
		"all_alerts":    alerts,
		"active_alerts": activeAlerts,
		"rules":         rules,
	})
}

// CreateUserAlertRule: POST /api/v1/observability/alerts
func (h *ObservabilityHandler) CreateUserAlertRule(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var in request.CreateAlertRuleRequest
	if !bindRequiredJSON(c, &in) {
		return
	}
	rule, err := h.obs.CreateAlertRule(c.Request.Context(), &tenantID, in.ToInput())
	if err != nil {
		writeError(c, err, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	response.Created(c, rule)
}

// UpdateUserAlertRule: PATCH /api/v1/observability/alerts/:id
func (h *ObservabilityHandler) UpdateUserAlertRule(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, ok := uuidParam(c, "id", "alert rule id")
	if !ok {
		return
	}
	var in request.UpdateAlertRuleRequest
	if !bindRequiredJSON(c, &in) {
		return
	}
	rule, err := h.obs.UpdateAlertRule(c.Request.Context(), id, &tenantID, in.ToInput())
	if err != nil {
		writeError(c, err, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	response.OK(c, rule)
}

// DeleteUserAlertRule: DELETE /api/v1/observability/alerts/:id
func (h *ObservabilityHandler) DeleteUserAlertRule(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, ok := uuidParam(c, "id", "alert rule id")
	if !ok {
		return
	}
	if err := h.obs.DeleteAlertRule(c.Request.Context(), id, &tenantID); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.NoContent(c)
}

// UserInstanceObservability: GET /api/v1/observability/instances/:id
func (h *ObservabilityHandler) UserInstanceObservability(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, ok := uuidParam(c, "id", "instance id")
	if !ok {
		return
	}
	detail, err := h.obs.GetVMDetailObservability(c.Request.Context(), id, &tenantID)
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}
	response.OK(c, detail)
}

// UserInstanceMetrics: GET /api/v1/observability/instances/:id/metrics
func (h *ObservabilityHandler) UserInstanceMetrics(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, ok := uuidParam(c, "id", "instance id")
	if !ok {
		return
	}
	detail, err := h.obs.GetVMDetailObservability(c.Request.Context(), id, &tenantID)
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}
	response.OK(c, map[string]any{
		"cpu_usage_pct":        detail.CPUUsagePct,
		"memory_usage_pct":     detail.MemoryUsagePct,
		"memory_used_mb":       detail.MemoryUsedMB,
		"memory_total_mb":      detail.MemoryTotalMB,
		"disk_usage_pct":       detail.DiskUsagePct,
		"disk_used_gb":         detail.DiskUsedGB,
		"disk_total_gb":        detail.DiskTotalGB,
		"disk_read_bytes_sec":  detail.DiskReadBytesSec,
		"disk_write_bytes_sec": detail.DiskWriteBytesSec,
		"network_rx_mbps":      detail.NetworkRxMbps,
		"network_tx_mbps":      detail.NetworkTxMbps,
		"uptime_seconds":       detail.UptimeSeconds,
		"guest_agent_status":   detail.GuestAgentStatus,
		"health_score":         detail.HealthScore,
	})
}

// UserInstanceEvents: GET /api/v1/observability/instances/:id/events
func (h *ObservabilityHandler) UserInstanceEvents(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, ok := uuidParam(c, "id", "instance id")
	if !ok {
		return
	}
	events, count, err := h.obs.QueryEvents(c.Request.Context(), &tenantID, &id, "", "", "", 50, 0)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, map[string]any{
		"events": events,
		"total":  count,
	})
}

// Stream: GET /api/v1/observability/stream (SSE)
func (h *ObservabilityHandler) Stream(c *gin.Context) {
	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	sub, cancel := h.obs.Subscribe(c.Request.Context())
	defer cancel()

	c.SSEvent("connected", map[string]string{"status": "listening"})
	c.Writer.Flush()

	notify := c.Request.Context().Done()
	var serverStop <-chan struct{}
	if h.streams != nil {
		serverStop = h.streams.Done()
	}

	c.Stream(func(w io.Writer) bool {
		select {
		case <-notify:
			return false
		case <-serverStop:
			return false
		case msg, ok := <-sub:
			if !ok {
				return false
			}
			c.SSEvent(msg.Event, msg.Data)
			return true
		}
	})
}

// --- Admin Endpoints ---

// AdminOverview: GET /api/v1/admin/observability/overview
func (h *ObservabilityHandler) AdminOverview(c *gin.Context) {
	ov, err := h.obs.GetAdminOverview(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, ov)
}

// AdminInfrastructure: GET /api/v1/admin/observability/infrastructure
func (h *ObservabilityHandler) AdminInfrastructure(c *gin.Context) {
	inf, err := h.obs.GetInfrastructureHealth(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, inf)
}

// AdminNodes: GET /api/v1/admin/observability/nodes
func (h *ObservabilityHandler) AdminNodes(c *gin.Context) {
	if h.adminUC == nil {
		response.OK(c, []map[string]any{})
		return
	}
	nodes := h.adminUC.ListNodes(c.Request.Context())
	response.OK(c, nodes)
}

// AdminNodeDetail: GET /api/v1/admin/observability/nodes/:id
func (h *ObservabilityHandler) AdminNodeDetail(c *gin.Context) {
	id, ok := uuidParam(c, "id", "node id")
	if !ok {
		return
	}
	if h.adminUC == nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", "Node not found")
		return
	}
	nodes := h.adminUC.ListNodes(c.Request.Context())
	for _, n := range nodes {
		if n.Node.ID == id {
			inf, _ := h.obs.GetInfrastructureHealth(c.Request.Context())
			response.OK(c, map[string]any{
				"node":           n,
				"infrastructure": inf,
			})
			return
		}
	}
	response.Error(c, http.StatusNotFound, "NOT_FOUND", "Node not found")
}

// AdminStorage: GET /api/v1/admin/observability/storage
func (h *ObservabilityHandler) AdminStorage(c *gin.Context) {
	inf, err := h.obs.GetInfrastructureHealth(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	thresholds := map[string]string{
		"healthy":  "< 70%",
		"warning":  "70–80%",
		"high":     "80–90%",
		"critical": "> 90%",
	}
	response.OK(c, map[string]any{
		"storage_trend": inf.Storage,
		"zfs_pool":      inf.ZFS,
		"thresholds":    thresholds,
	})
}

// AdminVMs: GET /api/v1/admin/observability/vms
func (h *ObservabilityHandler) AdminVMs(c *gin.Context) {
	if h.adminUC == nil {
		response.OK(c, []map[string]any{})
		return
	}
	instances, err := h.adminUC.ListAllInstances(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}

	res := make([]map[string]any, 0, len(instances))
	for _, inst := range instances {
		hs := h.obs.CalculateVMHealthScore(inst, 24.0, 62.0, 54.0, inst.Status == models.StateRunning)
		res = append(res, map[string]any{
			"instance":     inst,
			"health_score": hs,
		})
	}
	response.OK(c, res)
}

// AdminVMDetail: GET /api/v1/admin/observability/vms/:id
func (h *ObservabilityHandler) AdminVMDetail(c *gin.Context) {
	id, ok := uuidParam(c, "id", "vm id")
	if !ok {
		return
	}
	detail, err := h.obs.GetVMDetailObservability(c.Request.Context(), id, nil)
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}
	response.OK(c, detail)
}

// AdminAPI: GET /api/v1/admin/observability/api
func (h *ObservabilityHandler) AdminAPI(c *gin.Context) {
	apiSummary, err := h.obs.GetAPIObservability(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, apiSummary)
}

// AdminJobs: GET /api/v1/admin/observability/jobs
func (h *ObservabilityHandler) AdminJobs(c *gin.Context) {
	summary, err := h.obs.GetJobObservability(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, summary)
}

// AdminTenants: GET /api/v1/admin/observability/tenants
func (h *ObservabilityHandler) AdminTenants(c *gin.Context) {
	tenants, err := h.obs.GetTenantsObservability(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, tenants)
}

// AdminEvents: GET /api/v1/admin/observability/events
func (h *ObservabilityHandler) AdminEvents(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	eventType := c.Query("event_type")
	severity := c.Query("severity")
	corrID := c.Query("correlation_id")

	var tenantID *uuid.UUID
	if tStr := c.Query("tenant_id"); tStr != "" {
		if tid, err := uuid.Parse(tStr); err == nil {
			tenantID = &tid
		}
	}
	var resID *uuid.UUID
	if rStr := c.Query("resource_id"); rStr != "" {
		if rid, err := uuid.Parse(rStr); err == nil {
			resID = &rid
		}
	}

	events, count, err := h.obs.QueryEvents(c.Request.Context(), tenantID, resID, eventType, severity, corrID, limit, offset)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	if events == nil {
		events = []models.ObservabilityEvent{}
	}
	response.OK(c, map[string]any{
		"events": events,
		"total":  count,
	})
}

// AdminLogs: GET /api/v1/admin/observability/logs
func (h *ObservabilityHandler) AdminLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	service := c.Query("service")
	level := c.Query("level")
	corrID := c.Query("correlation_id")
	search := c.Query("search")

	var tenantID *uuid.UUID
	if tStr := c.Query("tenant_id"); tStr != "" {
		if tid, err := uuid.Parse(tStr); err == nil {
			tenantID = &tid
		}
	}
	var resID *uuid.UUID
	if rStr := c.Query("resource_id"); rStr != "" {
		if rid, err := uuid.Parse(rStr); err == nil {
			resID = &rid
		}
	}
	var jobID *uuid.UUID
	if jStr := c.Query("job_id"); jStr != "" {
		if jid, err := uuid.Parse(jStr); err == nil {
			jobID = &jid
		}
	}

	logs, count, err := h.obs.QueryLogs(c.Request.Context(), tenantID, resID, jobID, service, level, corrID, search, limit, offset)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	if logs == nil {
		logs = []models.ObservabilityLog{}
	}
	response.OK(c, map[string]any{
		"logs":  logs,
		"total": count,
	})
}

// AdminAlerts: GET /api/v1/admin/observability/alerts
func (h *ObservabilityHandler) AdminAlerts(c *gin.Context) {
	status := c.Query("status")
	alerts, err := h.obs.ListAlerts(c.Request.Context(), nil, status)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	rules, err := h.obs.ListAlertRules(c.Request.Context(), nil)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	if alerts == nil {
		alerts = []models.ObservabilityAlert{}
	}
	if rules == nil {
		rules = []models.ObservabilityAlertRule{}
	}
	var activeAlerts []models.ObservabilityAlert
	for _, a := range alerts {
		if a.Status != "resolved" {
			activeAlerts = append(activeAlerts, a)
		}
	}
	if activeAlerts == nil {
		activeAlerts = []models.ObservabilityAlert{}
	}
	response.OK(c, map[string]any{
		"alerts":        alerts,
		"all_alerts":    alerts,
		"active_alerts": activeAlerts,
		"rules":         rules,
	})
}

// AdminAcknowledgeAlert: POST /api/v1/admin/observability/alerts/:id/acknowledge
func (h *ObservabilityHandler) AdminAcknowledgeAlert(c *gin.Context) {
	id, ok := uuidParam(c, "id", "alert id")
	if !ok {
		return
	}
	actor := c.GetString("email")
	if actor == "" {
		actor = "admin"
	}
	if err := h.obs.AcknowledgeAlert(c.Request.Context(), id, nil, actor); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, map[string]string{"status": "acknowledged"})
}

// AdminResolveAlert: POST /api/v1/admin/observability/alerts/:id/resolve
func (h *ObservabilityHandler) AdminResolveAlert(c *gin.Context) {
	id, ok := uuidParam(c, "id", "alert id")
	if !ok {
		return
	}
	var in request.ResolveAlertRequest
	_ = c.ShouldBindJSON(&in)
	res := in.Resolution
	if res == "" {
		res = "Resolved by administrator"
	}
	if err := h.obs.ResolveAlert(c.Request.Context(), id, nil, res); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, map[string]string{"status": "resolved"})
}

// AdminIncidents: GET /api/v1/admin/observability/incidents
func (h *ObservabilityHandler) AdminIncidents(c *gin.Context) {
	status := c.Query("status")
	incs, err := h.obs.ListIncidents(c.Request.Context(), status)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	if incs == nil {
		incs = []models.ObservabilityIncident{}
	}
	response.OK(c, incs)
}

// AdminCreateIncident: POST /api/v1/admin/observability/incidents
func (h *ObservabilityHandler) AdminCreateIncident(c *gin.Context) {
	var in request.CreateIncidentRequest
	if !bindRequiredJSON(c, &in) {
		return
	}
	creator := c.GetString("email")
	if creator == "" {
		creator = "admin"
	}
	inc, err := h.obs.CreateIncident(c.Request.Context(), in.ToInput(), creator)
	if err != nil {
		writeError(c, err, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	response.Created(c, inc)
}

// AdminGetIncident: GET /api/v1/admin/observability/incidents/:id
func (h *ObservabilityHandler) AdminGetIncident(c *gin.Context) {
	id, ok := uuidParam(c, "id", "incident id")
	if !ok {
		return
	}
	inc, err := h.obs.GetIncident(c.Request.Context(), id)
	if err != nil {
		writeError(c, err, http.StatusNotFound, "NOT_FOUND")
		return
	}
	response.OK(c, inc)
}

// AdminUpdateIncident: PATCH /api/v1/admin/observability/incidents/:id
func (h *ObservabilityHandler) AdminUpdateIncident(c *gin.Context) {
	id, ok := uuidParam(c, "id", "incident id")
	if !ok {
		return
	}
	var in request.UpdateIncidentRequest
	if !bindRequiredJSON(c, &in) {
		return
	}
	inc, err := h.obs.UpdateIncident(c.Request.Context(), id, in.ToInput())
	if err != nil {
		writeError(c, err, http.StatusBadRequest, "BAD_REQUEST")
		return
	}
	response.OK(c, inc)
}

// AdminCapacity: GET /api/v1/admin/observability/capacity
func (h *ObservabilityHandler) AdminCapacity(c *gin.Context) {
	forecast, err := h.obs.GetCapacityForecast(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, forecast)
}

// AdminAI: GET /api/v1/admin/observability/ai
func (h *ObservabilityHandler) AdminAI(c *gin.Context) {
	summary, err := h.obs.GetAIObservability(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, summary)
}

// AdminAIInvestigate: POST /api/v1/admin/observability/ai/investigate
func (h *ObservabilityHandler) AdminAIInvestigate(c *gin.Context) {
	var in request.AIInvestigateRequest
	_ = c.ShouldBindJSON(&in)
	result, err := h.obs.AIIncidentInvestigation(c.Request.Context(), in.ToInput())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, result)
}
