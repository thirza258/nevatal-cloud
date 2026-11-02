package response

import (
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/usecase"
)

// AdminOverview is the body of GET /admin/overview.
type AdminOverview struct {
	ActiveJobs       int64                 `json:"active_jobs"`
	FailedJobs       int64                 `json:"failed_jobs"`
	Node             models.Node           `json:"node"`
	NodeStatus       *proxmox.NodeStatus   `json:"node_status"`
	RunningInstances int64                 `json:"running_instances"`
	StoppedInstances int64                 `json:"stopped_instances"`
	Storage          []proxmox.StorageInfo `json:"storage"`
	TotalInstances   int64                 `json:"total_instances"`
	TotalUsers       int64                 `json:"total_users"`
}

func NewAdminOverview(ov *usecase.PlatformOverview) AdminOverview {
	return AdminOverview{
		ActiveJobs:       ov.ActiveJobs,
		FailedJobs:       ov.FailedJobs,
		Node:             ov.Node,
		NodeStatus:       ov.NodeStatus,
		RunningInstances: ov.RunningInstances,
		StoppedInstances: ov.StoppedInstances,
		Storage:          ov.Storage,
		TotalInstances:   ov.TotalInstances,
		TotalUsers:       ov.TotalUsers,
	}
}

// UserWithUsage is an account (all user fields inline) with its quota usage.
type UserWithUsage struct {
	models.User
	Usage *models.QuotaUsage `json:"usage"`
}

func NewUsersWithUsage(users []usecase.UserWithUsage) []UserWithUsage {
	out := make([]UserWithUsage, len(users))
	for i, u := range users {
		out[i] = UserWithUsage{User: u.User, Usage: u.Usage}
	}
	return out
}

// NodeDetail is a hypervisor node (all node fields inline) with its live
// status and storage pools.
type NodeDetail struct {
	models.Node
	LiveStatus *proxmox.NodeStatus   `json:"live_status"`
	Storage    []proxmox.StorageInfo `json:"storage"`
}

func NewNodeDetails(nodes []usecase.NodeDetail) []NodeDetail {
	out := make([]NodeDetail, len(nodes))
	for i, n := range nodes {
		out[i] = NodeDetail{Node: n.Node, LiveStatus: n.LiveStatus, Storage: n.Storage}
	}
	return out
}

// JobRetried is the body of POST /admin/jobs/:id/retry.
type JobRetried struct {
	JobID   uuid.UUID `json:"job_id"`
	Message string    `json:"message"`
}
