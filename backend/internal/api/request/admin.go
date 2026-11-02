package request

import "github.com/nevatal/cloud/internal/usecase"

// The admin bodies were anonymous structs; they stay unnamed (aliases) so
// validation messages are unchanged. The use case inputs have identical
// fields, so they convert directly.

// UpdateUserStatusRequest is the body of PATCH /admin/users/:id.
type UpdateUserStatusRequest = struct {
	Status string `json:"status" binding:"required"` // active, suspended
}

// UpdateUserQuotasRequest is the body of PUT /admin/users/:id/quotas.
type UpdateUserQuotasRequest = struct {
	MaxVCPUs     int `json:"max_vcpus" binding:"required,min=1"`
	MaxMemoryMB  int `json:"max_memory_mb" binding:"required,min=512"`
	MaxStorageGB int `json:"max_storage_gb" binding:"required,min=10"`
	MaxInstances int `json:"max_instances" binding:"required,min=1"`
}

// CreateTemplateRequest is the body of POST /admin/templates.
type CreateTemplateRequest = struct {
	Name                string `json:"name" binding:"required"`
	Distribution        string `json:"distribution" binding:"required"`
	Version             string `json:"version" binding:"required"`
	Description         string `json:"description"`
	ProxmoxTemplateVMID int    `json:"proxmox_template_vmid" binding:"required,min=100"`
	Architecture        string `json:"architecture"`
	CloudInitSupported  *bool  `json:"cloud_init_supported"`
	DefaultUsername     string `json:"default_username"`
	Enabled             *bool  `json:"enabled"`
}

// UpdateTemplateRequest is the body of PUT /admin/templates/:id.
type UpdateTemplateRequest = struct {
	Name                *string `json:"name"`
	Distribution        *string `json:"distribution"`
	Version             *string `json:"version"`
	Description         *string `json:"description"`
	ProxmoxTemplateVMID *int    `json:"proxmox_template_vmid"`
	Architecture        *string `json:"architecture"`
	CloudInitSupported  *bool   `json:"cloud_init_supported"`
	DefaultUsername     *string `json:"default_username"`
	Enabled             *bool   `json:"enabled"`
}

// CreatePlanRequest is the body of POST /admin/plans.
type CreatePlanRequest = struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	VCPUs       int    `json:"vcpus" binding:"required,min=1"`
	MemoryMB    int    `json:"memory_mb" binding:"required,min=256"`
	DiskGB      int    `json:"disk_gb" binding:"required,min=5"`
	Enabled     *bool  `json:"enabled"`
}

// UpdatePlanRequest is the body of PUT /admin/plans/:id.
type UpdatePlanRequest = struct {
	Name        *string `json:"name"`
	Description *string `json:"description"`
	VCPUs       *int    `json:"vcpus"`
	MemoryMB    *int    `json:"memory_mb"`
	DiskGB      *int    `json:"disk_gb"`
	Enabled     *bool   `json:"enabled"`
}

func UserQuotaInput(r UpdateUserQuotasRequest) usecase.UserQuotaInput {
	return usecase.UserQuotaInput(r)
}

func CreateTemplateInput(r CreateTemplateRequest) usecase.CreateTemplateInput {
	return usecase.CreateTemplateInput(r)
}

func UpdateTemplateInput(r UpdateTemplateRequest) usecase.UpdateTemplateInput {
	return usecase.UpdateTemplateInput(r)
}

func CreatePlanInput(r CreatePlanRequest) usecase.CreatePlanInput {
	return usecase.CreatePlanInput(r)
}

func UpdatePlanInput(r UpdatePlanRequest) usecase.UpdatePlanInput {
	return usecase.UpdatePlanInput(r)
}
