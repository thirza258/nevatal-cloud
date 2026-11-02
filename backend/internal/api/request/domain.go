package request

import (
	"strings"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/usecase"
)

// The domain bodies carry no binding tags: presence and ranges are checked
// by the use case, which reports them as VALIDATION_ERROR with readable
// messages and details.field. instance_id is decoded as a string so a
// malformed ID is reported against its field.

// CreateDomainRequest is the body of POST /domains.
type CreateDomainRequest struct {
	Subdomain  string  `json:"subdomain"`
	InstanceID *string `json:"instance_id"`
	TargetPort *int    `json:"target_port"`
}

func (r CreateDomainRequest) ToUsecase() (usecase.CreateDomainInput, error) {
	instanceID, err := optionalInstanceID(r.InstanceID)
	if err != nil {
		return usecase.CreateDomainInput{}, err
	}
	return usecase.CreateDomainInput{Subdomain: r.Subdomain, InstanceID: instanceID, TargetPort: r.TargetPort}, nil
}

// UpdateDomainRequest is the body of PATCH /domains/:id. Absent (or null)
// fields are left unchanged.
type UpdateDomainRequest struct {
	InstanceID *string `json:"instance_id"`
	TargetPort *int    `json:"target_port"`
	Enabled    *bool   `json:"enabled"`
}

func (r UpdateDomainRequest) ToUsecase() (usecase.UpdateDomainInput, error) {
	instanceID, err := optionalInstanceID(r.InstanceID)
	if err != nil {
		return usecase.UpdateDomainInput{}, err
	}
	return usecase.UpdateDomainInput{InstanceID: instanceID, TargetPort: r.TargetPort, Enabled: r.Enabled}, nil
}

// AdminUpdateDomainRequest is the body of PATCH /admin/domains/:id.
type AdminUpdateDomainRequest struct {
	AdminDisabled  *bool   `json:"admin_disabled"`
	DisabledReason *string `json:"disabled_reason"`
}

func (r AdminUpdateDomainRequest) ToUsecase() usecase.AdminDomainInput {
	return usecase.AdminDomainInput{AdminDisabled: r.AdminDisabled, DisabledReason: r.DisabledReason}
}

// optionalInstanceID parses an optional instance_id ("" counts as absent).
func optionalInstanceID(raw *string) (*uuid.UUID, error) {
	if raw == nil || strings.TrimSpace(*raw) == "" {
		return nil, nil
	}
	id, err := uuid.Parse(strings.TrimSpace(*raw))
	if err != nil {
		return nil, usecase.ValidationError("instance_id", "instance_id must be a valid instance ID")
	}
	return &id, nil
}
