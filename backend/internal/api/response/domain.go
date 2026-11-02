package response

import (
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/usecase"
)

// Domain is a Nevatal Domain (GET/POST/PATCH /domains, /admin/domains).
// Owner is only set on the admin endpoints.
type Domain struct {
	AdminDisabled  bool         `json:"admin_disabled"`
	CreatedAt      time.Time    `json:"created_at"`
	DisabledReason string       `json:"disabled_reason"`
	Enabled        bool         `json:"enabled"`
	Hostname       string       `json:"hostname"`
	ID             uuid.UUID    `json:"id"`
	InstanceID     *uuid.UUID   `json:"instance_id"`
	InstanceName   string       `json:"instance_name"`
	InstanceStatus string       `json:"instance_status"`
	LiveStatus     string       `json:"live_status"`
	Owner          *DomainOwner `json:"owner,omitempty"`
	Subdomain      string       `json:"subdomain"`
	TargetIP       *string      `json:"target_ip"`
	TargetPort     int          `json:"target_port"`
	UpdatedAt      time.Time    `json:"updated_at"`
	URL            string       `json:"url"`
}

// DomainOwner identifies the account that owns a domain (admin endpoints).
type DomainOwner struct {
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	ID          uuid.UUID `json:"id"`
}

// NewDomain maps a use case view. withOwner adds the owner (admin endpoints).
func NewDomain(v usecase.DomainView, withOwner bool) Domain {
	d := v.Domain
	out := Domain{
		AdminDisabled:  d.AdminDisabled,
		CreatedAt:      d.CreatedAt,
		DisabledReason: d.DisabledReason,
		Enabled:        d.Enabled,
		Hostname:       d.Hostname,
		ID:             d.ID,
		InstanceID:     d.InstanceID,
		LiveStatus:     string(v.LiveStatus),
		Subdomain:      d.Subdomain,
		TargetIP:       v.TargetIP,
		TargetPort:     d.TargetPort,
		UpdatedAt:      d.UpdatedAt,
		URL:            "https://" + d.Hostname,
	}
	if d.Instance != nil {
		out.InstanceName = d.Instance.Name
		out.InstanceStatus = d.Instance.Status
	}
	if withOwner && d.Owner != nil {
		out.Owner = &DomainOwner{DisplayName: d.Owner.DisplayName, Email: d.Owner.Email, ID: d.Owner.ID}
	}
	return out
}

// NewDomains maps a list of use case views.
func NewDomains(views []usecase.DomainView, withOwner bool) []Domain {
	out := make([]Domain, len(views))
	for i, v := range views {
		out[i] = NewDomain(v, withOwner)
	}
	return out
}

// SubdomainAvailability is the body of GET /domains/check.
type SubdomainAvailability struct {
	Available  bool   `json:"available"`
	Hostname   string `json:"hostname"`
	Reason     string `json:"reason"`
	ReasonCode string `json:"reason_code"`
	Subdomain  string `json:"subdomain"`
}

func NewSubdomainAvailability(a *usecase.SubdomainAvailability) SubdomainAvailability {
	return SubdomainAvailability{
		Available:  a.Available,
		Hostname:   a.Hostname,
		Reason:     a.Reason,
		ReasonCode: a.ReasonCode,
		Subdomain:  a.Subdomain,
	}
}
