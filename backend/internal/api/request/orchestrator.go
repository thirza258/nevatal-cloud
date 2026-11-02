package request

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/usecase"
)

// ApprovePlanInput is the (optional) body of POST /ai/plans/:id/approve.
// Legacy clients send no body, {plan_id} or {version}.
type ApprovePlanInput struct {
	PlanID           *uuid.UUID `json:"plan_id"`
	Version          *int       `json:"version"`
	SpecHash         string     `json:"spec_hash"`
	AcknowledgeRisks bool       `json:"acknowledge_risks"`
	Confirmation     string     `json:"confirmation"`
	Execute          *bool      `json:"execute"`
}

func (r ApprovePlanInput) ToUsecase() usecase.ApproveInput {
	return usecase.ApproveInput{
		Version:          r.Version,
		SpecHash:         r.SpecHash,
		AcknowledgeRisks: r.AcknowledgeRisks,
		Confirmation:     r.Confirmation,
		Execute:          r.Execute,
	}
}

// PatchPlanInput is the body of PATCH /ai/plans/:id.
type PatchPlanInput struct {
	BaseVersion   *int            `json:"base_version"`
	Specification json.RawMessage `json:"specification"`
}

// ToUsecase decodes the specification (numbers kept as written).
func (r PatchPlanInput) ToUsecase() (usecase.PatchPlanInput, error) {
	var spec orchestrator.Specification
	if err := orchestrator.DecodeJSON(string(r.Specification), &spec); err != nil {
		return usecase.PatchPlanInput{}, err
	}
	for i := range spec.Resources {
		if spec.Resources[i].Config == nil {
			spec.Resources[i].Config = map[string]any{}
		}
	}
	base := 0
	if r.BaseVersion != nil {
		base = *r.BaseVersion
	}
	return usecase.PatchPlanInput{BaseVersion: base, Specification: spec}, nil
}

// RejectPlanInput is the optional body of POST /ai/plans/:id/reject.
type RejectPlanInput struct {
	Reason string `json:"reason"`
}

// ExecutePlanInput is the body of POST /ai/plans/:id/execute.
type ExecutePlanInput struct {
	ApprovalID uuid.UUID `json:"approval_id" binding:"required"`
}

// RecoveryInput is the body of POST /ai/executions/:id/recovery.
type RecoveryInput struct {
	Action string `json:"action" binding:"required"`
}
