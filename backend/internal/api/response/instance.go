package response

import (
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/proxmox"
)

// InstanceCreated is returned by POST /instances (202).
type InstanceCreated struct {
	InstanceID *uuid.UUID `json:"instance_id"`
	JobID      uuid.UUID  `json:"job_id"`
	Status     string     `json:"status"`
}

// JobQueued is returned when an action was queued as a background job (202).
type JobQueued struct {
	JobID  uuid.UUID `json:"job_id"`
	Status string    `json:"status"`
}

// InstanceMetricsUnavailable is returned by GET /instances/:id/metrics when no
// live reading exists.
type InstanceMetricsUnavailable struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason"`
}

// InstanceMetrics is a live hypervisor reading for GET /instances/:id/metrics.
type InstanceMetrics struct {
	Available  bool    `json:"available"`
	CPUUsage   float64 `json:"cpu_usage"`
	DiskMax    int64   `json:"disk_max"`
	DiskUsed   int64   `json:"disk_used"`
	MemoryMax  int64   `json:"memory_max"`
	MemoryUsed int64   `json:"memory_used"`
	NetIn      int64   `json:"net_in"`
	NetOut     int64   `json:"net_out"`
	Uptime     int64   `json:"uptime"`
}

// NewInstanceMetrics maps a hypervisor status to the metrics DTO.
func NewInstanceMetrics(s *proxmox.VMCurrentStatus) InstanceMetrics {
	return InstanceMetrics{
		Available:  true,
		CPUUsage:   s.CPUUsage,
		DiskMax:    s.DiskMax,
		DiskUsed:   s.DiskUsed,
		MemoryMax:  s.MemoryMax,
		MemoryUsed: s.MemoryUsed,
		NetIn:      s.NetIn,
		NetOut:     s.NetOut,
		Uptime:     s.Uptime,
	}
}
