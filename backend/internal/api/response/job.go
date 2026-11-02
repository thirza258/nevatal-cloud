package response

import (
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
)

// JobStatus is the polling view of a provisioning job (GET /jobs/:id). Unlike
// the model, unset optional fields are rendered as null.
type JobStatus struct {
	CompletedAt     *time.Time `json:"completed_at"`
	CurrentStep     string     `json:"current_step"`
	ErrorCode       *string    `json:"error_code"`
	ErrorMessage    *string    `json:"error_message"`
	ID              uuid.UUID  `json:"id"`
	InstanceID      *uuid.UUID `json:"instance_id"`
	OperationType   string     `json:"operation_type"`
	ProgressPercent int        `json:"progress_percent"`
	StartedAt       *time.Time `json:"started_at"`
	Status          string     `json:"status"`
}

func NewJobStatus(job *models.ProvisioningJob) JobStatus {
	return JobStatus{
		CompletedAt:     job.CompletedAt,
		CurrentStep:     job.CurrentStep,
		ErrorCode:       job.ErrorCode,
		ErrorMessage:    job.ErrorMessage,
		ID:              job.ID,
		InstanceID:      job.InstanceID,
		OperationType:   job.OperationType,
		ProgressPercent: job.ProgressPercent,
		StartedAt:       job.StartedAt,
		Status:          job.Status,
	}
}
