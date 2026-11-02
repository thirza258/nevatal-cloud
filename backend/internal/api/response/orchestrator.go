package response

import "github.com/google/uuid"

// RecoveryPlan is the reply of the delete_created recovery action.
type RecoveryPlan struct {
	PlanID uuid.UUID `json:"plan_id"`
}
