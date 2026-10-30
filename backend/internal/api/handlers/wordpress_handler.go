package handlers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/services"
	"gorm.io/gorm"
)

type WordPressHandler struct {
	db              *gorm.DB
	cfg             *config.Config
	authService     *services.AuthService
	instanceService *services.InstanceService
	auditService    *services.AuditService
	proxmox         proxmox.ProxmoxClient
}

func NewWordPressHandler(
	db *gorm.DB,
	cfg *config.Config,
	authService *services.AuthService,
	instanceService *services.InstanceService,
	auditService *services.AuditService,
	pve proxmox.ProxmoxClient,
) *WordPressHandler {
	return &WordPressHandler{
		db:              db,
		cfg:             cfg,
		authService:     authService,
		instanceService: instanceService,
		auditService:    auditService,
		proxmox:         pve,
	}
}

func (h *WordPressHandler) GetStatus(c *gin.Context) {
	var node models.Node
	_ = h.db.First(&node)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	nodeStatus, _ := h.proxmox.GetNodeStatus(ctx, node.ProxmoxNodeName)

	var activeInstances int64
	h.db.Model(&models.Instance{}).Where("deleted_at IS NULL").Count(&activeInstances)

	c.JSON(http.StatusOK, gin.H{
		"status":           "connected",
		"service":          "Nevatal Cloud WordPress Bridge",
		"version":          "1.0.0",
		"hypervisor_node":  node.ProxmoxNodeName,
		"active_instances": activeInstances,
		"node_health":      nodeStatus,
		"timestamp":        time.Now().UTC(),
	})
}

type WPWebhookPayload struct {
	Action      string `json:"action" binding:"required"` // ping, provision_vps, list_instances, control_instance, get_catalog
	UserEmail   string `json:"user_email"`
	UserName    string `json:"user_name"`
	PlanID      string `json:"plan_id"`
	TemplateID  string `json:"template_id"`
	InstanceID  string `json:"instance_id"`
	PowerAction string `json:"power_action"` // start, shutdown, restart, delete
	CustomName  string `json:"custom_name"`
	OrderID     string `json:"order_id"` // WooCommerce Order ID
}

func (h *WordPressHandler) HandleWebhook(c *gin.Context) {
	// 1. Verify API Key
	providedKey := c.GetHeader("X-Nevatal-Key")
	if providedKey == "" {
		providedKey = c.GetHeader("X-WP-Webhook-Secret")
	}
	if providedKey == "" {
		providedKey = c.Query("api_key")
	}

	if providedKey != h.cfg.WordPressAPIKey && h.cfg.WordPressAPIKey != "" {
		c.JSON(http.StatusUnauthorized, gin.H{
			"error": gin.H{"code": "UNAUTHORIZED", "message": "Invalid or missing WordPress API Key"},
		})
		return
	}

	var req WPWebhookPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "BAD_REQUEST", "message": err.Error()},
		})
		return
	}

	switch req.Action {
	case "ping":
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"message": "Nevatal Cloud WordPress integration handshake verified.",
		})

	case "get_catalog":
		var templates []models.OSTemplate
		var plans []models.ResourcePlan
		h.db.Where("enabled = true").Find(&templates)
		h.db.Where("enabled = true").Find(&plans)
		c.JSON(http.StatusOK, gin.H{
			"templates": templates,
			"plans":     plans,
		})

	case "provision_vps":
		if req.UserEmail == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "MISSING_EMAIL", "message": "User email is required"}})
			return
		}

		// Ensure user exists or create them
		oauthUser := services.OAuthUserInput{
			Provider:    "wordpress",
			Email:       req.UserEmail,
			DisplayName: req.UserName,
		}
		user, _, err := h.authService.OAuthLoginOrRegister(oauthUser)
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{"code": "USER_CREATION_FAILED", "message": err.Error()}})
			return
		}

		// Resolve plan and template
		var plan models.ResourcePlan
		if req.PlanID != "" {
			h.db.First(&plan, "id = ?", req.PlanID)
		}
		if plan.ID == uuid.Nil {
			h.db.First(&plan, "name = ?", "Starter")
		}

		var tmpl models.OSTemplate
		if req.TemplateID != "" {
			h.db.First(&tmpl, "id = ?", req.TemplateID)
		}
		if tmpl.ID == uuid.Nil {
			// Default to WordPress template if available, else Ubuntu
			if err := h.db.First(&tmpl, "name ILIKE ?", "%wordpress%").Error; err != nil {
				h.db.First(&tmpl, "distribution = ?", "ubuntu")
			}
		}

		instName := req.CustomName
		if instName == "" {
			instName = fmt.Sprintf("wp-vps-%d", time.Now().Unix()%10000)
		}

		input := services.CreateInstanceInput{
			Name:       instName,
			TemplateID: tmpl.ID,
			PlanID:     &plan.ID,
		}

		job, err := h.instanceService.Create(user.ID, input, c.ClientIP())
		if err != nil {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": gin.H{"code": "PROVISION_FAILED", "message": err.Error()}})
			return
		}

		h.auditService.Log(&user.ID, "wordpress.order.provision", "instance", job.InstanceID, "success", c.ClientIP(), map[string]string{
			"order_id":   req.OrderID,
			"user_email": req.UserEmail,
			"plan":       plan.Name,
		})

		c.JSON(http.StatusAccepted, gin.H{
			"status":      "provisioning",
			"job_id":      job.ID,
			"instance_id": job.InstanceID,
			"user_id":     user.ID,
			"message":     "VPS provisioning enqueued successfully.",
		})

	case "list_instances":
		var instances []models.Instance
		query := h.db.Preload("OSTemplate").Preload("Node").Where("deleted_at IS NULL")

		if req.UserEmail != "" {
			var u models.User
			if err := h.db.First(&u, "email = ?", req.UserEmail).Error; err == nil {
				query = query.Where("owner_id = ?", u.ID)
			}
		}

		query.Order("created_at desc").Find(&instances)
		c.JSON(http.StatusOK, gin.H{"data": instances})

	case "control_instance":
		id, err := uuid.Parse(req.InstanceID)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ID", "message": "Invalid instance ID"}})
			return
		}

		var job *models.ProvisioningJob
		switch req.PowerAction {
		case "start":
			job, err = h.instanceService.Start(id, uuid.Nil, true, c.ClientIP())
		case "shutdown":
			job, err = h.instanceService.Shutdown(id, uuid.Nil, true, c.ClientIP())
		case "restart":
			job, err = h.instanceService.Restart(id, uuid.Nil, true, c.ClientIP())
		case "delete":
			job, err = h.instanceService.Delete(id, uuid.Nil, true, c.ClientIP())
		default:
			c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "INVALID_ACTION", "message": "Supported actions: start, shutdown, restart, delete"}})
			return
		}

		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": gin.H{"code": "ACTION_FAILED", "message": err.Error()}})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{
			"status":  "accepted",
			"job_id":  job.ID,
			"message": fmt.Sprintf("Action %s dispatched successfully.", req.PowerAction),
		})

	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"error": gin.H{"code": "UNKNOWN_ACTION", "message": "Unknown action parameter"},
		})
	}
}
