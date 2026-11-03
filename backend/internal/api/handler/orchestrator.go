package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/usecase"
)

// OrchestratorHandler serves the AI infrastructure orchestrator (plans,
// approvals, executions, events, tools). Errors use the coded body of the
// orchestrator contract (docs/ORCHESTRATOR_API.md §7).
type OrchestratorHandler struct {
	service *usecase.OrchestratorUsecase
	// streams is cancelled when the HTTP server shuts down, so open SSE
	// streams end and http.Server.Shutdown does not wait for them.
	streams context.Context
}

func NewOrchestratorHandler(service *usecase.OrchestratorUsecase, streams context.Context) *OrchestratorHandler {
	if streams == nil {
		streams = context.Background()
	}
	return &OrchestratorHandler{service: service, streams: streams}
}

// SSE timings.
const (
	sseHeartbeat = 20 * time.Second
	ssePoll      = time.Second
)

func uuidParam(c *gin.Context, name, what string) (uuid.UUID, bool) {
	id, err := request.UUIDParam(c, name)
	if err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid "+what, response.FieldDetails{Field: name})
		return uuid.Nil, false
	}
	return id, true
}

func optionalUUIDQuery(c *gin.Context, name string) (*uuid.UUID, bool) {
	raw := c.Query(name)
	if raw == "" {
		return nil, true
	}
	id, err := uuid.Parse(raw)
	if err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid "+name, response.FieldDetails{Field: name})
		return nil, false
	}
	return &id, true
}

// bindOptionalJSON decodes the body into dst; an empty body leaves dst as is.
func bindOptionalJSON(c *gin.Context, dst any) bool {
	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "could not read the request body")
		return false
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return true
	}
	if err := json.Unmarshal(body, dst); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body: "+err.Error())
		return false
	}
	return true
}

func bindRequiredJSON(c *gin.Context, dst any) bool {
	if err := request.BindJSON(c, dst); err != nil {
		response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return false
	}
	return true
}

// Plans

func (h *OrchestratorHandler) GetPlan(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	plan, err := h.service.GetPlan(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, plan)
}

func (h *OrchestratorHandler) ListRevisions(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	revs, err := h.service.ListRevisions(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, revs)
}

func (h *OrchestratorHandler) GetRevision(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	version, err := strconv.Atoi(c.Param("version"))
	if err != nil || version <= 0 {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid version", response.FieldDetails{Field: "version"})
		return
	}
	rev, err := h.service.GetRevision(c.Request.Context(), id, middleware.GetCurrentUserID(c), version)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, rev)
}

func (h *OrchestratorHandler) PatchPlan(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	var req request.PatchPlanInput
	if !bindRequiredJSON(c, &req) {
		return
	}
	if req.BaseVersion == nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "base_version is required", response.FieldDetails{Field: "base_version"})
		return
	}
	if len(req.Specification) == 0 || string(req.Specification) == "null" {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "specification is required", response.FieldDetails{Field: "specification"})
		return
	}
	in, err := req.ToUsecase()
	if err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "specification: "+err.Error(), response.FieldDetails{Field: "specification"})
		return
	}
	plan, err := h.service.PatchPlan(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), in)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, plan)
}

func (h *OrchestratorHandler) ValidatePlan(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	res, err := h.service.ValidatePlan(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, res)
}

// ApprovePlan is the only way to approve a plan.
func (h *OrchestratorHandler) ApprovePlan(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	var req request.ApprovePlanInput
	if !bindOptionalJSON(c, &req) {
		return
	}
	if req.PlanID != nil && *req.PlanID != id {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "plan_id does not match the URL", response.FieldDetails{Field: "plan_id"})
		return
	}
	res, err := h.service.ApprovePlan(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, res)
}

func (h *OrchestratorHandler) RejectPlan(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	var req request.RejectPlanInput
	if !bindOptionalJSON(c, &req) {
		return
	}
	plan, err := h.service.RejectPlan(c.Request.Context(), id, middleware.GetCurrentUserID(c), req.Reason, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, plan)
}

// CancelPlan keeps the legacy reply ({"status": "cancelled"}) and also
// cancels the plan's active execution.
func (h *OrchestratorHandler) CancelPlan(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	if err := h.service.CancelPlan(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.ClientIP()); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, response.Status{Status: "cancelled"})
}

func (h *OrchestratorHandler) ExecutePlan(c *gin.Context) {
	id, ok := uuidParam(c, "id", "plan id")
	if !ok {
		return
	}
	var req request.ExecutePlanInput
	if !bindRequiredJSON(c, &req) {
		return
	}
	exec, err := h.service.ExecutePlan(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.ApprovalID, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Accepted(c, exec)
}

// Executions

func (h *OrchestratorHandler) ListExecutions(c *gin.Context) {
	conv, ok := optionalUUIDQuery(c, "conversation_id")
	if !ok {
		return
	}
	plan, ok := optionalUUIDQuery(c, "plan_id")
	if !ok {
		return
	}
	list, err := h.service.ListExecutions(c.Request.Context(), middleware.GetCurrentUserID(c), usecase.ExecutionListFilter{
		ConversationID: conv, PlanID: plan, Status: c.Query("status"),
	})
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, list)
}

func (h *OrchestratorHandler) GetExecution(c *gin.Context) {
	id, ok := uuidParam(c, "id", "execution id")
	if !ok {
		return
	}
	exec, err := h.service.GetExecution(c.Request.Context(), id, middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, exec)
}

func (h *OrchestratorHandler) CancelExecution(c *gin.Context) {
	id, ok := uuidParam(c, "id", "execution id")
	if !ok {
		return
	}
	exec, err := h.service.CancelExecution(c.Request.Context(), id, middleware.GetCurrentUserID(c), c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Accepted(c, exec)
}

func (h *OrchestratorHandler) RetryTask(c *gin.Context) {
	id, ok := uuidParam(c, "id", "execution id")
	if !ok {
		return
	}
	taskID, ok := uuidParam(c, "taskId", "task id")
	if !ok {
		return
	}
	exec, err := h.service.RetryTask(c.Request.Context(), id, taskID, middleware.GetCurrentUserID(c), c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.Accepted(c, exec)
}

func (h *OrchestratorHandler) Recovery(c *gin.Context) {
	id, ok := uuidParam(c, "id", "execution id")
	if !ok {
		return
	}
	var req request.RecoveryInput
	if !bindRequiredJSON(c, &req) {
		return
	}
	res, err := h.service.Recover(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Action, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	switch {
	case res.Diagnostics != nil:
		response.OK(c, res.Diagnostics)
	case res.PlanID != nil:
		response.Created(c, response.RecoveryPlan{PlanID: *res.PlanID})
	case req.Action == orchestrator.RecoveryRetryFailed:
		response.Accepted(c, res.Execution)
	default:
		response.OK(c, res.Execution)
	}
}

func (h *OrchestratorHandler) ListTools(c *gin.Context) {
	response.OK(c, h.service.ListTools())
}

// Events serves GET /ai/executions/:id/events as a JSON page or, with
// Accept: text/event-stream, as a Server-Sent Events stream.
func (h *OrchestratorHandler) Events(c *gin.Context) {
	id, ok := uuidParam(c, "id", "execution id")
	if !ok {
		return
	}
	after := int64(0)
	if raw := c.Query("after"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "invalid after", response.FieldDetails{Field: "after"})
			return
		}
		after = n
	}
	if raw := c.GetHeader("Last-Event-ID"); raw != "" {
		if n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil && n > after {
			after = n
		}
	}
	limit := 200
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n <= 0 || n > 500 {
			response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "limit must be 1-500", response.FieldDetails{Field: "limit"})
			return
		}
		limit = n
	}
	userID := middleware.GetCurrentUserID(c)
	if strings.Contains(c.GetHeader("Accept"), "text/event-stream") {
		h.stream(c, id, userID, after)
		return
	}
	page, err := h.service.ListEvents(c.Request.Context(), id, userID, after, limit)
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	response.OK(c, page)
}

func (h *OrchestratorHandler) stream(c *gin.Context, execID, userID uuid.UUID, after int64) {
	ctx := c.Request.Context()
	// Ownership check (and 404) before the stream starts.
	if _, err := h.service.ListEvents(ctx, execID, userID, after, 1); err != nil {
		writeError(c, err, http.StatusInternalServerError, "INTERNAL_ERROR")
		return
	}
	w := c.Writer
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	w.Flush()

	heartbeat := time.NewTicker(sseHeartbeat)
	defer heartbeat.Stop()
	poll := time.NewTicker(ssePoll)
	defer poll.Stop()
	notifier := h.service.Notifier()
	for {
		wake := notifier.Wait(execID)
		page, err := h.service.EventsSince(context.WithoutCancel(ctx), execID, after, 200)
		if err != nil {
			return
		}
		for _, ev := range page.Events {
			data, _ := json.Marshal(ev)
			if _, err := fmt.Fprintf(w, "id: %d\nevent: %s\ndata: %s\n\n", ev.Seq, ev.Type, data); err != nil {
				return
			}
			after = ev.Seq
			if ev.Type == orchestrator.EventExecutionFinished {
				w.Flush()
				return
			}
		}
		if len(page.Events) > 0 {
			w.Flush()
			if len(page.Events) == 200 {
				continue
			}
		} else if h.service.ExecutionFinished(ctx, execID, userID, after) {
			// Reconnected after the end: nothing more will come.
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-h.streams.Done():
			return
		case <-wake:
		case <-poll.C:
		case <-heartbeat.C:
			if _, err := fmt.Fprint(w, ": ping\n\n"); err != nil {
				return
			}
			w.Flush()
		}
	}
}
