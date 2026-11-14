package response

// Health is the body of GET /health (not enveloped).
type Health struct {
	Features []string `json:"features"`
	Product  string   `json:"product"`
	Status   string   `json:"status"`
	Version  string   `json:"version"`
}

// SystemConfig is the public runtime configuration (GET /system/config).
type SystemConfig struct {
	Auth        SystemAuthConfig     `json:"auth"`
	DemoMode    bool                 `json:"demo_mode"`
	Domains     SystemDomainsConfig  `json:"domains"`
	Environment string               `json:"environment"`
	Features    SystemFeaturesConfig `json:"features"`
	ProxmoxMock bool                 `json:"proxmox_mock"`
	Telemetry   *SystemTelemetry     `json:"telemetry,omitempty"`
	Version     string               `json:"version"`
}

type SystemTelemetry struct {
	NodeName        string               `json:"node_name"`
	ProxmoxNodeName string               `json:"proxmox_node_name"`
	CPUThreads      int                  `json:"cpu_threads"`
	CPUCores        int                  `json:"cpu_cores"`
	CPUModel        string               `json:"cpu_model"`
	MemoryTotalMB   int                  `json:"memory_total_mb"`
	MemoryTotalGB   int                  `json:"memory_total_gb"`
	MemoryUsedGB    int                  `json:"memory_used_gb"`
	DiskTotalGB     int                  `json:"disk_total_gb"`
	DiskTotalTB     string               `json:"disk_total_tb"`
	DiskUsedGB      int                  `json:"disk_used_gb"`
	PveVersion      string               `json:"pve_version"`
	KernelVersion   string               `json:"kernel_version"`
	StoragePools    []StoragePoolSummary `json:"storage_pools"`
	Status          string               `json:"status"`
}

type StoragePoolSummary struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	TotalGB int64  `json:"total_gb"`
	AvailGB int64  `json:"avail_gb"`
	UsedGB  int64  `json:"used_gb"`
}

type SystemAuthConfig struct {
	PasswordLogin bool `json:"password_login"`
	Registration  bool `json:"registration"`
	SocialLogin   bool `json:"social_login"`
}

type SystemFeaturesConfig struct {
	DockerViewer bool `json:"docker_viewer"`
	Domains      bool `json:"domains"`
}

// SystemDomainsConfig describes Nevatal Domains. Root is empty while the
// feature is disabled.
type SystemDomainsConfig struct {
	Enabled    bool   `json:"enabled"`
	MaxPerUser int    `json:"max_per_user"`
	Root       string `json:"root"`
}
