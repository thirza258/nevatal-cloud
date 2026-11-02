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
	Version     string               `json:"version"`
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
