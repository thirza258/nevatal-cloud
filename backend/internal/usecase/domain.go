package usecase

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/domainroute"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// Nevatal Domains: a customer publishes one port of a VM at
// https://<subdomain>.<PUBLIC_DOMAIN_ROOT>. nevatal-edge (internal/edge)
// proxies the traffic; this use case owns the claim rules. Whether a domain
// is actually routable (live_status) is decided by domainroute.Policy, the
// same function the edge uses.

// Reason codes of GET /domains/check.
const (
	DomainReasonInvalid     = "INVALID"
	DomainReasonReserved    = "RESERVED"
	DomainReasonTaken       = "TAKEN"
	DomainReasonQuarantined = "QUARANTINED"
)

const maxDisabledReasonLength = 255

// subdomainPattern is a DNS label of 2+ characters: lowercase letters,
// digits and inner hyphens. Length (3-63) is checked separately.
var subdomainPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]*[a-z0-9])$`)

var (
	errDomainsDisabled      = newError(KindUnavailable, "DOMAINS_DISABLED", "Nevatal Domains is not enabled on this server")
	errDomainNotFound       = newError(KindNotFound, "DOMAIN_NOT_FOUND", "Domain not found")
	errDomainInstance       = newError(KindNotFound, "INSTANCE_NOT_FOUND", "Instance not found or access denied")
	errSubdomainTaken       = newError(KindConflict, "SUBDOMAIN_TAKEN", "This subdomain is already in use")
	errDomainNothingChanged = ValidationError("", "Provide at least one of instance_id, target_port or enabled")
)

// DomainRepos are the repositories DomainUsecase works with.
type DomainRepos struct {
	Domains   repository.DomainRepository
	Instances repository.InstanceRepository
}

type DomainUsecase struct {
	tx           repository.Transactor
	repos        DomainRepos
	cfg          *config.Config
	auditService *AuditUsecase
	policy       domainroute.Policy
	reserved     map[string]bool
	now          func() time.Time
}

func NewDomainUsecase(tx repository.Transactor, repos DomainRepos, cfg *config.Config, auditService *AuditUsecase) *DomainUsecase {
	reserved := make(map[string]bool, len(cfg.ReservedSubdomains))
	for _, name := range cfg.ReservedSubdomains {
		reserved[name] = true
	}
	return &DomainUsecase{
		tx:           tx,
		repos:        repos,
		cfg:          cfg,
		auditService: auditService,
		policy:       domainroute.Policy{TargetCIDRs: cfg.EdgeTargetCIDRs, DenyIPs: cfg.EdgeDenyIPs},
		reserved:     reserved,
		now:          time.Now,
	}
}

// DomainView is a domain with its routing state. Domain has Owner and
// Instance loaded (Instance may be soft-deleted).
type DomainView struct {
	Domain     models.Domain
	LiveStatus domainroute.Status
	TargetIP   *string // instance private address, when the instance exists
}

// SubdomainAvailability answers GET /domains/check. ReasonCode is "" when
// Available, otherwise one of the DomainReason* codes.
type SubdomainAvailability struct {
	Subdomain  string
	Hostname   string
	Available  bool
	ReasonCode string
	Reason     string
}

// CreateDomainInput claims Subdomain for TargetPort of InstanceID.
type CreateDomainInput struct {
	Subdomain  string
	InstanceID *uuid.UUID
	TargetPort *int
}

// UpdateDomainInput changes the fields that are set.
type UpdateDomainInput struct {
	InstanceID *uuid.UUID
	TargetPort *int
	Enabled    *bool
}

// AdminDomainInput disables or re-enables a domain. AdminDisabled is
// required.
type AdminDomainInput struct {
	AdminDisabled  *bool
	DisabledReason *string
}

// Enabled reports whether the feature is on (PUBLIC_DOMAIN_ROOT set and not
// in demo mode).
func (s *DomainUsecase) Enabled() bool {
	return s.cfg.DomainsEnabled()
}

func (s *DomainUsecase) ensureEnabled() error {
	if !s.Enabled() {
		return errDomainsDisabled
	}
	return nil
}

// List returns the caller's active domains, newest first.
func (s *DomainUsecase) List(ctx context.Context, userID uuid.UUID) ([]DomainView, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	domains, err := s.repos.Domains.ListActiveByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, domains)
}

// Check reports whether userID could claim subdomain right now. Claiming
// can still fail later (another account may claim it first, or the
// caller's domain limit applies).
func (s *DomainUsecase) Check(ctx context.Context, userID uuid.UUID, subdomain string) (*SubdomainAvailability, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}

	sub := normalizeSubdomain(subdomain)
	out := &SubdomainAvailability{Subdomain: sub}
	if code, message := s.classifySubdomain(sub); code != "" {
		out.ReasonCode, out.Reason = code, message
		if code != DomainReasonInvalid {
			out.Hostname = s.hostname(sub)
		}
		return out, nil
	}
	out.Hostname = s.hostname(sub)

	taken, err := s.repos.Domains.HostnameActive(ctx, out.Hostname)
	if err != nil {
		return nil, err
	}
	if taken {
		out.ReasonCode, out.Reason = DomainReasonTaken, "This subdomain is already in use"
		return out, nil
	}

	quarantined, err := s.repos.Domains.HostnameQuarantined(ctx, out.Hostname, userID, s.quarantineSince())
	if err != nil {
		return nil, err
	}
	if quarantined {
		out.ReasonCode, out.Reason = DomainReasonQuarantined, s.quarantineMessage()
		return out, nil
	}

	out.Available = true
	return out, nil
}

// Create claims a subdomain for a port of one of the caller's instances
// (admins: any instance). Uniqueness is enforced by the database: the unique
// index on the active hostname turns a concurrent duplicate claim into
// SUBDOMAIN_TAKEN.
func (s *DomainUsecase) Create(ctx context.Context, userID uuid.UUID, isAdmin bool, input CreateDomainInput, clientIP string) (*DomainView, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx)

	sub := normalizeSubdomain(input.Subdomain)
	switch code, message := s.classifySubdomain(sub); code {
	case DomainReasonInvalid:
		return nil, ValidationError("subdomain", message)
	case DomainReasonReserved:
		return nil, newError(KindConflict, "SUBDOMAIN_RESERVED", message)
	}
	if input.InstanceID == nil || *input.InstanceID == uuid.Nil {
		return nil, ValidationError("instance_id", "instance_id is required")
	}
	if input.TargetPort == nil {
		return nil, ValidationError("target_port", "target_port is required")
	}
	if err := validateTargetPort(*input.TargetPort); err != nil {
		return nil, err
	}
	inst, err := s.findInstance(ctx, *input.InstanceID, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	domain := models.Domain{
		OwnerID:    userID,
		InstanceID: &inst.ID,
		Subdomain:  sub,
		Hostname:   s.hostname(sub),
		TargetPort: *input.TargetPort,
		Enabled:    true,
	}
	err = s.tx.WithinTransaction(ctx, func(txCtx context.Context) error {
		// Serialize this owner's claims so two parallel requests cannot both
		// pass the limit check.
		if err := s.repos.Domains.LockOwner(txCtx, userID); err != nil {
			return err
		}
		active, err := s.repos.Domains.CountActiveByOwner(txCtx, userID)
		if err != nil {
			return err
		}
		if active >= int64(s.cfg.MaxDomainsPerUser) {
			return newError(KindConflict, "DOMAIN_LIMIT_REACHED", fmt.Sprintf("You can publish at most %d domains. Release one to claim another.", s.cfg.MaxDomainsPerUser))
		}
		quarantined, err := s.repos.Domains.HostnameQuarantined(txCtx, domain.Hostname, userID, s.quarantineSince())
		if err != nil {
			return err
		}
		if quarantined {
			return newError(KindConflict, "SUBDOMAIN_QUARANTINED", s.quarantineMessage())
		}
		if err := s.repos.Domains.Create(txCtx, &domain); err != nil {
			if errors.Is(err, repository.ErrHostnameTaken) {
				return errSubdomainTaken
			}
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	s.auditService.Log(ctx, &userID, "domain.create", "domain", &domain.ID, "success", clientIP, map[string]any{
		"hostname":    domain.Hostname,
		"instance_id": inst.ID,
		"target_port": domain.TargetPort,
	})
	return s.view(ctx, domain.ID)
}

// Update changes the target instance, the port or the owner's enabled
// toggle of a domain. Non-admins only reach their own domains and
// instances.
func (s *DomainUsecase) Update(ctx context.Context, id, userID uuid.UUID, isAdmin bool, input UpdateDomainInput, clientIP string) (*DomainView, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx)

	if input.InstanceID == nil && input.TargetPort == nil && input.Enabled == nil {
		return nil, errDomainNothingChanged
	}
	if input.InstanceID != nil && *input.InstanceID == uuid.Nil {
		return nil, ValidationError("instance_id", "instance_id must be a valid instance ID")
	}
	if input.TargetPort != nil {
		if err := validateTargetPort(*input.TargetPort); err != nil {
			return nil, err
		}
	}

	domain, err := s.find(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	changes := map[string]any{}
	if input.InstanceID != nil {
		inst, err := s.findInstance(ctx, *input.InstanceID, userID, isAdmin)
		if err != nil {
			return nil, err
		}
		domain.InstanceID = &inst.ID
		changes["instance_id"] = inst.ID
	}
	if input.TargetPort != nil {
		domain.TargetPort = *input.TargetPort
		changes["target_port"] = domain.TargetPort
	}
	if input.Enabled != nil {
		domain.Enabled = *input.Enabled
		changes["enabled"] = domain.Enabled
	}
	if err := s.repos.Domains.UpdateSettings(ctx, domain); err != nil {
		return nil, err
	}

	changes["hostname"] = domain.Hostname
	s.auditService.Log(ctx, &userID, "domain.update", "domain", &domain.ID, "success", clientIP, changes)
	return s.view(ctx, domain.ID)
}

// Release gives a domain up. The hostname is quarantined for other accounts
// for DOMAIN_QUARANTINE_DAYS; the same owner may claim it again at once
// unless an administrator had disabled it.
func (s *DomainUsecase) Release(ctx context.Context, id, userID uuid.UUID, isAdmin bool, clientIP string) error {
	if err := s.ensureEnabled(); err != nil {
		return err
	}
	ctx = context.WithoutCancel(ctx)

	domain, err := s.find(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if err := s.repos.Domains.Release(ctx, domain, s.now()); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return errDomainNotFound
		}
		return err
	}

	s.auditService.Log(ctx, &userID, "domain.release", "domain", &domain.ID, "success", clientIP, map[string]any{
		"hostname":       domain.Hostname,
		"owner_id":       domain.OwnerID,
		"admin_disabled": domain.AdminDisabled,
	})
	return nil
}

// AdminList returns every active domain with its owner, newest first.
func (s *DomainUsecase) AdminList(ctx context.Context) ([]DomainView, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	domains, err := s.repos.Domains.ListActive(ctx)
	if err != nil {
		return nil, err
	}
	return s.views(ctx, domains)
}

// AdminSetDisabled disables a domain (for example for abuse) or re-enables
// it. Re-enabling clears the reason.
func (s *DomainUsecase) AdminSetDisabled(ctx context.Context, adminID, id uuid.UUID, input AdminDomainInput, clientIP string) (*DomainView, error) {
	if err := s.ensureEnabled(); err != nil {
		return nil, err
	}
	ctx = context.WithoutCancel(ctx)

	if input.AdminDisabled == nil {
		return nil, ValidationError("admin_disabled", "admin_disabled is required")
	}
	reason := ""
	if input.DisabledReason != nil {
		reason = strings.TrimSpace(*input.DisabledReason)
		if utf8.RuneCountInString(reason) > maxDisabledReasonLength {
			return nil, ValidationError("disabled_reason", fmt.Sprintf("disabled_reason must be at most %d characters", maxDisabledReasonLength))
		}
	}

	domain, err := s.find(ctx, id, adminID, true)
	if err != nil {
		return nil, err
	}
	domain.AdminDisabled = *input.AdminDisabled
	switch {
	case !domain.AdminDisabled:
		domain.DisabledReason = ""
	case input.DisabledReason != nil:
		domain.DisabledReason = reason
	}
	if err := s.repos.Domains.UpdateAdminState(ctx, domain); err != nil {
		return nil, err
	}

	action := "domain.admin_enable"
	if domain.AdminDisabled {
		action = "domain.admin_disable"
	}
	s.auditService.Log(ctx, &adminID, action, "domain", &domain.ID, "success", clientIP, map[string]any{
		"hostname": domain.Hostname,
		"owner_id": domain.OwnerID,
		"reason":   domain.DisabledReason,
	})
	return s.view(ctx, domain.ID)
}

// find returns an active domain, scoped to the owner unless isAdmin.
func (s *DomainUsecase) find(ctx context.Context, id, userID uuid.UUID, isAdmin bool) (*models.Domain, error) {
	domain, err := s.repos.Domains.FindActiveByID(ctx, id, ownerScope(userID, isAdmin))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errDomainNotFound
		}
		return nil, err
	}
	return domain, nil
}

// findInstance returns a live instance the caller may publish (admins: any).
func (s *DomainUsecase) findInstance(ctx context.Context, id, userID uuid.UUID, isAdmin bool) (*models.Instance, error) {
	inst, err := s.repos.Instances.FindByID(ctx, id, ownerScope(userID, isAdmin))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errDomainInstance
		}
		return nil, err
	}
	return inst, nil
}

// view reloads one domain with its relations and routing state.
func (s *DomainUsecase) view(ctx context.Context, id uuid.UUID) (*DomainView, error) {
	domain, err := s.repos.Domains.FindActiveByID(ctx, id, nil)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, errDomainNotFound
		}
		return nil, err
	}
	views, err := s.views(ctx, []models.Domain{*domain})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// views computes the routing state of domains with the edge's policy.
func (s *DomainUsecase) views(ctx context.Context, domains []models.Domain) ([]DomainView, error) {
	ips, err := s.repos.Instances.ListPrivateIPs(ctx)
	if err != nil {
		return nil, err
	}
	ipUsers := domainroute.CountAddrs(ips)
	out := make([]DomainView, len(domains))
	for i := range domains {
		out[i] = s.evaluate(domains[i], ipUsers)
	}
	return out, nil
}

func (s *DomainUsecase) evaluate(domain models.Domain, ipUsers map[netip.Addr]int) DomainView {
	target := domainroute.Target{
		Enabled:       domain.Enabled,
		AdminDisabled: domain.AdminDisabled,
		OwnerActive:   domain.Owner != nil && domain.Owner.Status == models.UserStatusActive,
	}
	var targetIP *string
	if inst := domain.Instance; inst != nil && !inst.DeletedAt.Valid {
		target.InstanceFound = true
		target.InstanceStatus = inst.Status
		if inst.PrivateIP != nil && *inst.PrivateIP != "" {
			target.PrivateIP = *inst.PrivateIP
			ip := *inst.PrivateIP
			targetIP = &ip
		}
	}
	status, _ := s.policy.Evaluate(target, ipUsers)
	return DomainView{Domain: domain, LiveStatus: status, TargetIP: targetIP}
}

func (s *DomainUsecase) hostname(sub string) string {
	return sub + "." + s.cfg.PublicDomainRoot
}

func (s *DomainUsecase) quarantineSince() time.Time {
	return s.now().Add(-time.Duration(s.cfg.DomainQuarantineDays) * 24 * time.Hour)
}

func (s *DomainUsecase) quarantineMessage() string {
	return fmt.Sprintf("This subdomain was released recently and is quarantined for %d days", s.cfg.DomainQuarantineDays)
}

// classifySubdomain checks a normalized subdomain against the naming rules.
// It returns "" when the name may be claimed, otherwise DomainReasonInvalid
// or DomainReasonReserved and a message.
//
// Rules: 3-63 characters; lowercase letters, digits and inner hyphens;
// no "--" in positions 3-4 (reserved for IDN A-labels such as xn--, which
// allow homograph look-alikes); not a reserved name; nothing that reads as
// "nevatal" (brand protection, including common look-alike spellings).
func (s *DomainUsecase) classifySubdomain(sub string) (code, message string) {
	switch {
	case sub == "":
		return DomainReasonInvalid, "Enter a subdomain"
	case len(sub) < 3 || len(sub) > 63:
		return DomainReasonInvalid, "A subdomain must be 3 to 63 characters long"
	case !subdomainPattern.MatchString(sub):
		return DomainReasonInvalid, "Use lowercase letters, digits and hyphens, starting and ending with a letter or digit"
	case len(sub) >= 4 && sub[2:4] == "--":
		return DomainReasonInvalid, "Subdomains with \"--\" in the third and fourth positions (such as xn--) are not allowed"
	case len(sub)+1+len(s.cfg.PublicDomainRoot) > 253:
		return DomainReasonInvalid, "The full hostname would be longer than 253 characters"
	case s.reserved[sub]:
		return DomainReasonReserved, "This subdomain is reserved"
	case strings.Contains(brandSkeleton(sub), "nevatal"):
		return DomainReasonReserved, "Subdomains containing \"nevatal\" are reserved"
	}
	return "", ""
}

// normalizeSubdomain trims and lower-cases a requested subdomain. A full
// hostname is not accepted: the dot fails validation.
func normalizeSubdomain(raw string) string {
	return strings.ToLower(strings.TrimSpace(raw))
}

// brandSkeleton folds common look-alike spellings (n3vata1, neva-tal,
// nevatai) so the brand check catches them.
func brandSkeleton(sub string) string {
	var b strings.Builder
	for _, r := range sub {
		switch r {
		case '-':
			continue
		case '0':
			r = 'o'
		case '1', 'i':
			r = 'l'
		case '3':
			r = 'e'
		case '4':
			r = 'a'
		case '5':
			r = 's'
		case '7':
			r = 't'
		}
		b.WriteRune(r)
	}
	return b.String()
}

func validateTargetPort(port int) error {
	if port < 1 || port > 65535 {
		return ValidationError("target_port", "target_port must be between 1 and 65535")
	}
	return nil
}
