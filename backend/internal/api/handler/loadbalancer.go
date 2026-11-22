package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

type LoadBalancerHandler struct {
	lbUc *usecase.LoadBalancerUsecase
}

func NewLoadBalancerHandler(lbUc *usecase.LoadBalancerUsecase) *LoadBalancerHandler {
	return &LoadBalancerHandler{lbUc: lbUc}
}

func (h *LoadBalancerHandler) ListLoadBalancers(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbs, err := h.lbUc.ListLoadBalancers(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, lbs)
}

func (h *LoadBalancerHandler) CreateLoadBalancer(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var input usecase.CreateLoadBalancerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	lb, err := h.lbUc.CreateLoadBalancer(c.Request.Context(), tenantID, input)
	if err != nil {
		if err == usecase.ErrInvalidAlgorithm {
			response.Error(c, http.StatusBadRequest, "INVALID_ALGORITHM", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, lb)
}

func (h *LoadBalancerHandler) GetLoadBalancer(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	lb, err := h.lbUc.GetLoadBalancer(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, lb)
}

func (h *LoadBalancerHandler) UpdateLoadBalancer(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	var input usecase.UpdateLoadBalancerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	lb, err := h.lbUc.UpdateLoadBalancer(c.Request.Context(), tenantID, id, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, lb)
}

func (h *LoadBalancerHandler) DeleteLoadBalancer(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	if err := h.lbUc.DeleteLoadBalancer(c.Request.Context(), tenantID, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *LoadBalancerHandler) AddListener(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	var input usecase.CreateListenerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	listener, err := h.lbUc.AddListener(c.Request.Context(), tenantID, lbID, input)
	if err != nil {
		if err == usecase.ErrInvalidListenerPort {
			response.Error(c, http.StatusBadRequest, "INVALID_PORT", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, listener)
}

func (h *LoadBalancerHandler) DeleteListener(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	listenerID, err := uuid.Parse(c.Param("listener_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid listener id")
		return
	}
	if err := h.lbUc.DeleteListener(c.Request.Context(), tenantID, lbID, listenerID); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *LoadBalancerHandler) AddPool(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	var input usecase.CreatePoolInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	pool, err := h.lbUc.AddPool(c.Request.Context(), tenantID, lbID, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, pool)
}

func (h *LoadBalancerHandler) UpdatePool(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	poolID, err := uuid.Parse(c.Param("pool_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid pool id")
		return
	}
	var input usecase.UpdatePoolInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	pool, err := h.lbUc.UpdatePool(c.Request.Context(), tenantID, lbID, poolID, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, pool)
}

func (h *LoadBalancerHandler) DeletePool(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	poolID, err := uuid.Parse(c.Param("pool_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid pool id")
		return
	}
	if err := h.lbUc.DeletePool(c.Request.Context(), tenantID, lbID, poolID); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *LoadBalancerHandler) AddMember(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	poolID, err := uuid.Parse(c.Param("pool_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid pool id")
		return
	}
	var input usecase.AddMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	member, err := h.lbUc.AddMember(c.Request.Context(), tenantID, lbID, poolID, input)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "ADD_MEMBER_FAILED", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, member)
}

func (h *LoadBalancerHandler) UpdateMember(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	poolID, err := uuid.Parse(c.Param("pool_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid pool id")
		return
	}
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member id")
		return
	}
	var input usecase.UpdateMemberInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	member, err := h.lbUc.UpdateMember(c.Request.Context(), tenantID, lbID, poolID, memberID, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, member)
}

func (h *LoadBalancerHandler) DeleteMember(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	poolID, err := uuid.Parse(c.Param("pool_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid pool id")
		return
	}
	memberID, err := uuid.Parse(c.Param("member_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid member id")
		return
	}
	if err := h.lbUc.DeleteMember(c.Request.Context(), tenantID, lbID, poolID, memberID); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *LoadBalancerHandler) GetHealth(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	health, err := h.lbUc.GetLoadBalancerHealth(c.Request.Context(), tenantID, lbID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, health)
}

func (h *LoadBalancerHandler) GetMetrics(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	lbID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid load balancer id")
		return
	}
	metrics, err := h.lbUc.GetLoadBalancerMetrics(c.Request.Context(), tenantID, lbID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, metrics)
}

func (h *LoadBalancerHandler) ListCertificates(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	certs, err := h.lbUc.ListCertificates(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, certs)
}

func (h *LoadBalancerHandler) AddCertificate(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var input struct {
		LoadBalancerID *uuid.UUID `json:"load_balancer_id"`
		Domain         string     `json:"domain" binding:"required"`
		Certificate    string     `json:"certificate" binding:"required"`
		PrivateKey     string     `json:"private_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	cert, err := h.lbUc.AddCertificate(c.Request.Context(), tenantID, input.LoadBalancerID, usecase.UploadCertificateInput{
		Domain:      input.Domain,
		Certificate: input.Certificate,
		PrivateKey:  input.PrivateKey,
	})
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusCreated, cert)
}

func (h *LoadBalancerHandler) DeleteCertificate(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	certID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid certificate id")
		return
	}
	if err := h.lbUc.DeleteCertificate(c.Request.Context(), tenantID, certID); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.JSON(c, http.StatusOK, map[string]string{"status": "deleted"})
}
