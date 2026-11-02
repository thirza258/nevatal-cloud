package request

import (
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/usecase"
)

// CreateInstanceInput is the body of POST /instances.
type CreateInstanceInput struct {
	Name           string     `json:"name" binding:"required"`
	Hostname       string     `json:"hostname"`
	TemplateID     uuid.UUID  `json:"template_id" binding:"required"`
	PlanID         *uuid.UUID `json:"plan_id"`
	VCPUs          int        `json:"vcpus"`
	MemoryMB       int        `json:"memory_mb"`
	DiskGB         int        `json:"disk_gb"`
	SSHKeyID       *uuid.UUID `json:"ssh_key_id"`
	IdempotencyKey string     `json:"idempotency_key"`
}

func (r CreateInstanceInput) ToUsecase() usecase.CreateInstanceInput {
	return usecase.CreateInstanceInput(r)
}

// ResizeInstanceInput is the body of POST /instances/:id/resize.
type ResizeInstanceInput struct {
	VCPUs    int `json:"vcpus" binding:"required,min=1"`
	MemoryMB int `json:"memory_mb" binding:"required,min=512"`
	DiskGB   int `json:"disk_gb" binding:"required,min=10"`
}

func (r ResizeInstanceInput) ToUsecase() usecase.ResizeInstanceInput {
	return usecase.ResizeInstanceInput(r)
}

// CreateSnapshotInput is the body of POST /instances/:id/snapshots.
type CreateSnapshotInput struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	IncludesRAM bool   `json:"includes_ram"`
}

func (r CreateSnapshotInput) ToUsecase() usecase.CreateSnapshotInput {
	return usecase.CreateSnapshotInput(r)
}
