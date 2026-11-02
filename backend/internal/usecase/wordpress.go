package usecase

import (
	"context"
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
)

// WordPressRepos are the repositories WordPressUsecase reads.
type WordPressRepos struct {
	Nodes     repository.NodeRepository
	Instances repository.InstanceRepository
	Templates repository.TemplateRepository
	Plans     repository.PlanRepository
	Users     repository.UserRepository
}

// WordPressUsecase backs the WordPress/WooCommerce bridge plugin. The plugin
// authenticates with the shared WORDPRESS_API_KEY and acts with platform
// (administrator) rights.
type WordPressUsecase struct {
	cfg          *config.Config
	repos        WordPressRepos
	authService  *AuthUsecase
	instances    *InstanceUsecase
	auditService *AuditUsecase
	proxmox      proxmox.ProxmoxClient
}

func NewWordPressUsecase(cfg *config.Config, repos WordPressRepos, authService *AuthUsecase, instances *InstanceUsecase, auditService *AuditUsecase, pve proxmox.ProxmoxClient) *WordPressUsecase {
	return &WordPressUsecase{
		cfg:          cfg,
		repos:        repos,
		authService:  authService,
		instances:    instances,
		auditService: auditService,
		proxmox:      pve,
	}
}

// WordPressStatus is the bridge health report.
type WordPressStatus struct {
	HypervisorNode  string
	ActiveInstances int64
	NodeHealth      *proxmox.NodeStatus
}

// WordPressProvisionInput is a WooCommerce order for a VPS.
type WordPressProvisionInput struct {
	UserEmail  string
	UserName   string
	PlanID     string
	TemplateID string
	CustomName string
	OrderID    string
}

// WordPressProvisioned is the outcome of a VPS order.
type WordPressProvisioned struct {
	Job    *models.ProvisioningJob
	UserID uuid.UUID
}

// Status reports the primary node and how many instances are live.
func (s *WordPressUsecase) Status(ctx context.Context) *WordPressStatus {
	node, _ := s.repos.Nodes.First(ctx)

	pveCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	nodeStatus, _ := s.proxmox.GetNodeStatus(pveCtx, node.ProxmoxNodeName)
	activeInstances, _ := s.repos.Instances.CountActive(ctx)

	return &WordPressStatus{
		HypervisorNode:  node.ProxmoxNodeName,
		ActiveInstances: activeInstances,
		NodeHealth:      nodeStatus,
	}
}

// VerifyWebhookKey checks the key the plugin presented. An empty
// WORDPRESS_API_KEY disables the bridge entirely (production refuses the old
// public default key).
func (s *WordPressUsecase) VerifyWebhookKey(providedKey string) error {
	if !s.cfg.WordPressEnabled() {
		return newError(KindUnavailable, "WORDPRESS_DISABLED", "WordPress integration is not configured on this server")
	}
	if subtle.ConstantTimeCompare([]byte(providedKey), []byte(s.cfg.WordPressAPIKey)) != 1 {
		return newError(KindUnauthorized, "UNAUTHORIZED", "Invalid or missing WordPress API Key")
	}
	return nil
}

// Catalog returns the enabled templates and plans in database order.
func (s *WordPressUsecase) Catalog(ctx context.Context) ([]models.OSTemplate, []models.ResourcePlan) {
	templates, _ := s.repos.Templates.ListEnabledUnordered(ctx)
	plans, _ := s.repos.Plans.ListEnabledUnordered(ctx)
	return templates, plans
}

// ProvisionVPS signs up (or finds) the ordering customer and queues a VPS.
// The plan defaults to "Starter" and the template to a WordPress image, else
// Ubuntu.
func (s *WordPressUsecase) ProvisionVPS(ctx context.Context, input WordPressProvisionInput, clientIP string) (*WordPressProvisioned, error) {
	ctx = context.WithoutCancel(ctx)

	if input.UserEmail == "" {
		return nil, newError(KindInvalid, "MISSING_EMAIL", "User email is required")
	}

	// Ensure user exists or create them
	user, _, err := s.authService.OAuthLoginOrRegister(ctx, OAuthUserInput{
		Provider:    "wordpress",
		Email:       input.UserEmail,
		DisplayName: input.UserName,
	})
	if err != nil {
		return nil, newError(KindUnprocessable, "USER_CREATION_FAILED", err.Error())
	}

	// Resolve plan and template
	var plan models.ResourcePlan
	if input.PlanID != "" {
		if p, err := s.repos.Plans.FindByRawID(ctx, input.PlanID); err == nil {
			plan = *p
		}
	}
	if plan.ID == uuid.Nil {
		if p, err := s.repos.Plans.FindByName(ctx, "Starter"); err == nil {
			plan = *p
		}
	}

	var tmpl models.OSTemplate
	if input.TemplateID != "" {
		if t, err := s.repos.Templates.FindByRawID(ctx, input.TemplateID); err == nil {
			tmpl = *t
		}
	}
	if tmpl.ID == uuid.Nil {
		// Default to WordPress template if available, else Ubuntu
		if t, err := s.repos.Templates.FindByNameLike(ctx, "%wordpress%"); err == nil {
			tmpl = *t
		} else if t, err := s.repos.Templates.FindByDistribution(ctx, "ubuntu"); err == nil {
			tmpl = *t
		}
	}

	instName := input.CustomName
	if instName == "" {
		instName = fmt.Sprintf("wp-vps-%d", time.Now().Unix()%10000)
	}

	job, err := s.instances.Create(ctx, user.ID, CreateInstanceInput{
		Name:       instName,
		TemplateID: tmpl.ID,
		PlanID:     &plan.ID,
	}, clientIP)
	if err != nil {
		return nil, newError(KindUnprocessable, "PROVISION_FAILED", err.Error())
	}

	s.auditService.Log(ctx, &user.ID, "wordpress.order.provision", "instance", job.InstanceID, "success", clientIP, map[string]string{
		"order_id":   input.OrderID,
		"user_email": input.UserEmail,
		"plan":       plan.Name,
	})

	return &WordPressProvisioned{Job: job, UserID: user.ID}, nil
}

// ListInstances returns live instances: all of them for an empty userEmail
// (site-wide view of the bridge), only that account's for a known email, and
// none for an unknown email.
func (s *WordPressUsecase) ListInstances(ctx context.Context, userEmail string) []models.Instance {
	var owner *uuid.UUID
	if userEmail != "" {
		u, err := s.repos.Users.FindByEmail(ctx, userEmail)
		if err != nil {
			return []models.Instance{}
		}
		owner = &u.ID
	}
	instances, _ := s.repos.Instances.ListWithPlacement(ctx, owner, "")
	return instances
}

// ControlInstance runs a power or delete action on any instance.
func (s *WordPressUsecase) ControlInstance(ctx context.Context, instanceID, powerAction, clientIP string) (*models.ProvisioningJob, error) {
	id, err := uuid.Parse(instanceID)
	if err != nil {
		return nil, newError(KindInvalid, "INVALID_ID", "Invalid instance ID")
	}

	var job *models.ProvisioningJob
	switch powerAction {
	case "start":
		job, err = s.instances.Start(ctx, id, uuid.Nil, true, clientIP)
	case "shutdown":
		job, err = s.instances.Shutdown(ctx, id, uuid.Nil, true, clientIP)
	case "restart":
		job, err = s.instances.Restart(ctx, id, uuid.Nil, true, clientIP)
	case "delete":
		job, err = s.instances.Delete(ctx, id, uuid.Nil, true, clientIP)
	default:
		return nil, newError(KindInvalid, "INVALID_ACTION", "Supported actions: start, shutdown, restart, delete")
	}

	if err != nil {
		return nil, newError(KindConflict, "ACTION_FAILED", err.Error())
	}
	return job, nil
}
