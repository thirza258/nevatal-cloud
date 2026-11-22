package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/usecase"
)

type NetworkHandler struct {
	netUc *usecase.NetworkUsecase
}

func NewNetworkHandler(netUc *usecase.NetworkUsecase) *NetworkHandler {
	return &NetworkHandler{netUc: netUc}
}

// Networks

func (h *NetworkHandler) ListNetworks(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	nets, err := h.netUc.ListNetworks(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if nets == nil {
		nets = []models.Network{}
	}
	response.OK(c, nets)
}

func (h *NetworkHandler) CreateNetwork(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var input usecase.CreateNetworkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	net, err := h.netUc.CreateNetwork(c.Request.Context(), tenantID, input)
	if err != nil {
		if err == usecase.ErrCIDROverlap {
			response.Error(c, http.StatusConflict, "NETWORK_CIDR_CONFLICT", err.Error())
			return
		}
		if err == usecase.ErrInvalidCIDR {
			response.Error(c, http.StatusBadRequest, "INVALID_CIDR", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Created(c, net)
}

func (h *NetworkHandler) GetNetwork(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid network id")
		return
	}
	net, err := h.netUc.GetNetwork(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, net)
}

func (h *NetworkHandler) UpdateNetwork(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid network id")
		return
	}
	var input usecase.UpdateNetworkInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	net, err := h.netUc.UpdateNetwork(c.Request.Context(), tenantID, id, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, net)
}

func (h *NetworkHandler) DeleteNetwork(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid network id")
		return
	}
	if err := h.netUc.DeleteNetwork(c.Request.Context(), tenantID, id); err != nil {
		response.Error(c, http.StatusBadRequest, "DELETE_FAILED", err.Error())
		return
	}
	response.OK(c, map[string]string{"status": "deleted"})
}

func (h *NetworkHandler) AttachInstance(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	networkID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid network id")
		return
	}
	var input struct {
		InstanceID uuid.UUID `json:"instance_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	att, err := h.netUc.AttachInstance(c.Request.Context(), tenantID, networkID, input.InstanceID)
	if err != nil {
		if err == usecase.ErrInstanceNotOwned {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		if err == usecase.ErrAttachmentConflict {
			response.Error(c, http.StatusConflict, "CONFLICT", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Created(c, att)
}

func (h *NetworkHandler) DetachInstance(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	networkID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid network id")
		return
	}
	attachmentID, err := uuid.Parse(c.Param("attachment_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid attachment id")
		return
	}
	if err := h.netUc.DetachInstance(c.Request.Context(), tenantID, networkID, attachmentID); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, map[string]string{"status": "detached"})
}

func (h *NetworkHandler) GetNetworkTraffic(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	networkID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid network id")
		return
	}
	timeRange := c.DefaultQuery("range", "1h")
	metrics, err := h.netUc.GetNetworkTraffic(c.Request.Context(), tenantID, networkID, timeRange)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, metrics)
}

// Firewalls

func (h *NetworkHandler) ListFirewalls(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	fws, err := h.netUc.ListFirewalls(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if fws == nil {
		fws = []models.Firewall{}
	}
	response.OK(c, fws)
}

func (h *NetworkHandler) CreateFirewall(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var input usecase.CreateFirewallInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	fw, err := h.netUc.CreateFirewall(c.Request.Context(), tenantID, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Created(c, fw)
}

func (h *NetworkHandler) GetFirewall(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid firewall id")
		return
	}
	fw, err := h.netUc.GetFirewall(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, fw)
}

func (h *NetworkHandler) UpdateFirewall(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid firewall id")
		return
	}
	var input usecase.UpdateFirewallInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	fw, err := h.netUc.UpdateFirewall(c.Request.Context(), tenantID, id, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, fw)
}

func (h *NetworkHandler) DeleteFirewall(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid firewall id")
		return
	}
	if err := h.netUc.DeleteFirewall(c.Request.Context(), tenantID, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, map[string]string{"status": "deleted"})
}

func (h *NetworkHandler) AddFirewallRule(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	firewallID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid firewall id")
		return
	}
	var input usecase.CreateFirewallRuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	rule, err := h.netUc.AddFirewallRule(c.Request.Context(), tenantID, firewallID, input)
	if err != nil {
		if err == usecase.ErrInvalidFirewallRule {
			response.Error(c, http.StatusBadRequest, "INVALID_FIREWALL_RULE", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Created(c, rule)
}

func (h *NetworkHandler) UpdateFirewallRule(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	firewallID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid firewall id")
		return
	}
	ruleID, err := uuid.Parse(c.Param("rule_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid rule id")
		return
	}
	var input usecase.UpdateFirewallRuleInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	rule, err := h.netUc.UpdateFirewallRule(c.Request.Context(), tenantID, firewallID, ruleID, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, rule)
}

func (h *NetworkHandler) DeleteFirewallRule(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	firewallID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid firewall id")
		return
	}
	ruleID, err := uuid.Parse(c.Param("rule_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid rule id")
		return
	}
	if err := h.netUc.DeleteFirewallRule(c.Request.Context(), tenantID, firewallID, ruleID); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, map[string]string{"status": "deleted"})
}

func (h *NetworkHandler) AttachFirewall(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	firewallID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid firewall id")
		return
	}
	var input usecase.AttachFirewallInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	att, err := h.netUc.AttachFirewall(c.Request.Context(), tenantID, firewallID, input)
	if err != nil {
		if err == usecase.ErrInstanceNotOwned {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Created(c, att)
}

func (h *NetworkHandler) DetachFirewall(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	firewallID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid firewall id")
		return
	}
	attachmentID, err := uuid.Parse(c.Param("attachment_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid attachment id")
		return
	}
	if err := h.netUc.DetachFirewall(c.Request.Context(), tenantID, firewallID, attachmentID); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, map[string]string{"status": "detached"})
}

// Floating IPs

func (h *NetworkHandler) ListFloatingIPs(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	fips, err := h.netUc.ListFloatingIPs(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, fips)
}

func (h *NetworkHandler) AllocateFloatingIP(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var input usecase.AllocateFloatingIPInput
	if err := c.ShouldBindJSON(&input); err != nil {
		// allow empty body
		input = usecase.AllocateFloatingIPInput{}
	}
	fip, err := h.netUc.AllocateFloatingIP(c.Request.Context(), tenantID, input)
	if err != nil {
		if err == usecase.ErrInstanceNotOwned {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Created(c, fip)
}

func (h *NetworkHandler) AssignFloatingIP(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid floating IP id")
		return
	}
	var input struct {
		InstanceID uuid.UUID `json:"instance_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	fip, err := h.netUc.AssignFloatingIP(c.Request.Context(), tenantID, id, input.InstanceID)
	if err != nil {
		if err == usecase.ErrInstanceNotOwned {
			response.Error(c, http.StatusForbidden, "FORBIDDEN", err.Error())
			return
		}
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, fip)
}

func (h *NetworkHandler) UnassignFloatingIP(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid floating IP id")
		return
	}
	fip, err := h.netUc.UnassignFloatingIP(c.Request.Context(), tenantID, id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, fip)
}

func (h *NetworkHandler) ReleaseFloatingIP(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid floating IP id")
		return
	}
	if err := h.netUc.ReleaseFloatingIP(c.Request.Context(), tenantID, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, map[string]string{"status": "released"})
}

// DNS

func (h *NetworkHandler) ListDNSZones(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	zones, err := h.netUc.ListDNSZones(c.Request.Context(), tenantID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if zones == nil {
		zones = []models.DNSZone{}
	}
	response.OK(c, zones)
}

func (h *NetworkHandler) CreateDNSZone(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	var input usecase.CreateDNSZoneInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	zone, err := h.netUc.CreateDNSZone(c.Request.Context(), tenantID, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.Created(c, zone)
}

func (h *NetworkHandler) DeleteDNSZone(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid DNS zone id")
		return
	}
	if err := h.netUc.DeleteDNSZone(c.Request.Context(), tenantID, id); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, map[string]string{"status": "deleted"})
}

func (h *NetworkHandler) ListDNSRecords(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	zoneID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid DNS zone id")
		return
	}
	records, err := h.netUc.ListDNSRecords(c.Request.Context(), tenantID, zoneID)
	if err != nil {
		response.Error(c, http.StatusNotFound, "NOT_FOUND", err.Error())
		return
	}
	response.OK(c, records)
}

func (h *NetworkHandler) CreateDNSRecord(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	zoneID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid DNS zone id")
		return
	}
	var input usecase.CreateDNSRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	rec, err := h.netUc.CreateDNSRecord(c.Request.Context(), tenantID, zoneID, input)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	response.Created(c, rec)
}

func (h *NetworkHandler) UpdateDNSRecord(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	zoneID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid DNS zone id")
		return
	}
	recordID, err := uuid.Parse(c.Param("record_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid DNS record id")
		return
	}
	var input usecase.UpdateDNSRecordInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", err.Error())
		return
	}
	rec, err := h.netUc.UpdateDNSRecord(c.Request.Context(), tenantID, zoneID, recordID, input)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, rec)
}

func (h *NetworkHandler) DeleteDNSRecord(c *gin.Context) {
	tenantID := middleware.GetCurrentUserID(c)
	zoneID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid DNS zone id")
		return
	}
	recordID, err := uuid.Parse(c.Param("record_id"))
	if err != nil {
		response.Error(c, http.StatusBadRequest, "BAD_REQUEST", "invalid DNS record id")
		return
	}
	if err := h.netUc.DeleteDNSRecord(c.Request.Context(), tenantID, zoneID, recordID); err != nil {
		response.Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	response.OK(c, map[string]string{"status": "deleted"})
}
