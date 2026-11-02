package handler

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// DomainHandler serves Nevatal Domains (/domains and /admin/domains). When
// the feature is off, middleware.RequireDomains answers 503 DOMAINS_DISABLED
// before these handlers run (the use case checks again).
type DomainHandler struct {
	domains *usecase.DomainUsecase
}

func NewDomainHandler(domains *usecase.DomainUsecase) *DomainHandler {
	return &DomainHandler{domains: domains}
}

// List: GET /domains (the caller's active domains).
func (h *DomainHandler) List(c *gin.Context) {
	views, err := h.domains.List(c.Request.Context(), middleware.GetCurrentUserID(c))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.OK(c, response.NewDomains(views, false))
}

// Check: GET /domains/check?subdomain=foo.
func (h *DomainHandler) Check(c *gin.Context) {
	result, err := h.domains.Check(c.Request.Context(), middleware.GetCurrentUserID(c), c.Query("subdomain"))
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.OK(c, response.NewSubdomainAvailability(result))
}

// Create: POST /domains.
func (h *DomainHandler) Create(c *gin.Context) {
	var req request.CreateDomainRequest
	if err := request.BindJSON(c, &req); err != nil {
		h.invalidBody(c, err)
		return
	}
	input, err := req.ToUsecase()
	if err != nil {
		writeError(c, err, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	view, err := h.domains.Create(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), input, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.Created(c, response.NewDomain(*view, false))
}

// Update: PATCH /domains/:id.
func (h *DomainHandler) Update(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid domain ID", response.FieldDetails{Field: "id"})
		return
	}
	var req request.UpdateDomainRequest
	if err := request.BindJSON(c, &req); err != nil {
		h.invalidBody(c, err)
		return
	}
	input, err := req.ToUsecase()
	if err != nil {
		writeError(c, err, http.StatusBadRequest, "VALIDATION_ERROR")
		return
	}
	view, err := h.domains.Update(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), input, c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.OK(c, response.NewDomain(*view, false))
}

// Release: DELETE /domains/:id.
func (h *DomainHandler) Release(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid domain ID", response.FieldDetails{Field: "id"})
		return
	}
	if err := h.domains.Release(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP()); err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.OKMessage(c, "Domain released")
}

// AdminList: GET /admin/domains (every active domain, with its owner).
func (h *DomainHandler) AdminList(c *gin.Context) {
	views, err := h.domains.AdminList(c.Request.Context())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.OK(c, response.NewDomains(views, true))
}

// AdminUpdate: PATCH /admin/domains/:id ({admin_disabled, disabled_reason}).
func (h *DomainHandler) AdminUpdate(c *gin.Context) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid domain ID", response.FieldDetails{Field: "id"})
		return
	}
	var req request.AdminUpdateDomainRequest
	if err := request.BindJSON(c, &req); err != nil {
		h.invalidBody(c, err)
		return
	}
	view, err := h.domains.AdminSetDisabled(c.Request.Context(), middleware.GetCurrentUserID(c), id, req.ToUsecase(), c.ClientIP())
	if err != nil {
		writeError(c, err, http.StatusInternalServerError, "SERVER_ERROR")
		return
	}
	response.OK(c, response.NewDomain(*view, true))
}

// invalidBody reports a body that is not valid JSON for the endpoint, with
// details.field when a value has the wrong type.
func (h *DomainHandler) invalidBody(c *gin.Context, err error) {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		response.ErrorWithDetails(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body: "+typeErr.Field+" has the wrong type",
			response.FieldDetails{Field: typeErr.Field})
		return
	}
	response.Error(c, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid request body: "+err.Error())
}
