package response

import (
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
)

// The WordPress bridge predates the envelope: these bodies are written as is.

// WordPressStatus is the body of GET /wordpress/status.
type WordPressStatus struct {
	ActiveInstances int64               `json:"active_instances"`
	HypervisorNode  string              `json:"hypervisor_node"`
	NodeHealth      *proxmox.NodeStatus `json:"node_health"`
	Service         string              `json:"service"`
	Status          string              `json:"status"`
	Timestamp       time.Time           `json:"timestamp"`
	Version         string              `json:"version"`
}

// StatusMessage is a {"message", "status"} acknowledgement.
type StatusMessage struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

// WordPressCatalog is the get_catalog webhook reply.
type WordPressCatalog struct {
	Plans     []models.ResourcePlan `json:"plans"`
	Templates []models.OSTemplate   `json:"templates"`
}

// WordPressProvisioned is the provision_vps webhook reply (202).
type WordPressProvisioned struct {
	InstanceID *uuid.UUID `json:"instance_id"`
	JobID      uuid.UUID  `json:"job_id"`
	Message    string     `json:"message"`
	Status     string     `json:"status"`
	UserID     uuid.UUID  `json:"user_id"`
}

// WordPressActionAccepted is the control_instance webhook reply (202).
type WordPressActionAccepted struct {
	JobID   uuid.UUID `json:"job_id"`
	Message string    `json:"message"`
	Status  string    `json:"status"`
}
