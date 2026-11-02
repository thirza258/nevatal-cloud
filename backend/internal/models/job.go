package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	JobStatusQueued     = "queued"
	JobStatusInProgress = "in_progress"
	JobStatusCompleted  = "completed"
	JobStatusFailed     = "failed"

	// JobErrorStale is the error code of a job failed by the stale-job
	// reaper (its worker stopped heartbeating, e.g. the replica crashed).
	JobErrorStale = "JOB_STALE"

	OpCreate   = "create"
	OpDelete   = "delete"
	OpStart    = "start"
	OpShutdown = "shutdown"
	OpRestart  = "restart"
)

type ProvisioningJob struct {
	ID              uuid.UUID  `gorm:"type:uuid;primary_key;default:gen_random_uuid()" json:"id"`
	OwnerID         uuid.UUID  `gorm:"type:uuid;index;not null" json:"owner_id"`
	InstanceID      *uuid.UUID `gorm:"type:uuid;index" json:"instance_id,omitempty"`
	IdempotencyKey  string     `gorm:"type:varchar(255);not null" json:"idempotency_key"`
	OperationType   string     `gorm:"type:varchar(32);not null" json:"operation_type"` // create, delete, start, shutdown, restart
	Status          string     `gorm:"type:varchar(32);index;not null;default:'queued'" json:"status"`
	CurrentStep     string     `gorm:"type:varchar(64);default:'queued'" json:"current_step"`
	ProgressPercent int        `gorm:"default:0" json:"progress_percent"`
	ErrorCode       *string    `gorm:"type:varchar(64)" json:"error_code,omitempty"`
	ErrorMessage    *string    `gorm:"type:text" json:"error_message,omitempty"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	CompletedAt     *time.Time `json:"completed_at,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	// HeartbeatAt is refreshed by the worker while the job runs; the
	// stale-job reaper fails active jobs whose heartbeat stopped. Not part of
	// the job JSON.
	HeartbeatAt *time.Time `gorm:"column:heartbeat_at" json:"-"`

	Instance *Instance `gorm:"foreignKey:InstanceID" json:"instance,omitempty"`
}

func (j *ProvisioningJob) BeforeCreate(tx *gorm.DB) error {
	if j.ID == uuid.Nil {
		j.ID = uuid.New()
	}
	return nil
}
