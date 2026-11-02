package orchestrator

import (
	"sort"
	"strings"
	"sync"
	"time"
)

// ToolDef describes a registry tool (contract §6). The registry is the single
// source of truth for GET /ai/tools and MCP tools/list.
type ToolDef struct {
	ID             string         `json:"id"`
	Title          string         `json:"title"`
	Description    string         `json:"description"`
	Risk           string         `json:"risk"`
	ResourceTypes  []string       `json:"resource_types"`
	Idempotent     bool           `json:"idempotent"`
	TimeoutSeconds int            `json:"timeout_seconds"`
	RequiresAdmin  bool           `json:"requires_admin"`
	InputSchema    map[string]any `json:"input_schema"`
	// MaxAttempts bounds automatic retries of a task using this tool
	// (always 1 for a tool that is not idempotent).
	MaxAttempts int `json:"-"`
}

// Timeout returns the per-attempt timeout.
func (d ToolDef) Timeout() time.Duration {
	if d.TimeoutSeconds <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(d.TimeoutSeconds) * time.Second
}

// Registry holds tool definitions, the runners bound to them and the
// mapping of specification resource type/action pairs to tools.
type Registry struct {
	mu      sync.RWMutex
	defs    map[string]ToolDef
	runners map[string]Runner
	actions map[string]string // "type/action" -> tool id
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{defs: map[string]ToolDef{}, runners: map[string]Runner{}, actions: map[string]string{}}
}

// MapAction makes resources of resourceType with action run tool.
func (r *Registry) MapAction(resourceType, action, tool string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.actions[resourceType+"/"+action] = tool
}

// ToolFor returns the tool performing action on a resource type.
func (r *Registry) ToolFor(resourceType, action string) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.actions[resourceType+"/"+action]
	return t, ok
}

// Register adds or replaces a tool definition. A tool that is not
// idempotent gets one attempt: the engine never runs it again on its own
// (only a manual retry does).
func (r *Registry) Register(def ToolDef) {
	if !def.Idempotent {
		def.MaxAttempts = 1
	}
	if def.MaxAttempts <= 0 {
		def.MaxAttempts = 3
	}
	if def.InputSchema == nil {
		def.InputSchema = obj(nil)
	}
	if def.ResourceTypes == nil {
		def.ResourceTypes = []string{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.defs[def.ID] = def
}

// Bind attaches the implementation of a registered tool.
func (r *Registry) Bind(id string, runner Runner) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runners[id] = runner
}

// Def returns a tool definition.
func (r *Registry) Def(id string) (ToolDef, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.defs[id]
	return d, ok
}

// Runner returns the implementation of a tool.
func (r *Registry) Runner(id string) (Runner, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, ok := r.runners[id]
	return run, ok
}

// List returns the tool definitions ordered by risk, then id.
func (r *Registry) List() []ToolDef {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]ToolDef, 0, len(r.defs))
	for _, d := range r.defs {
		out = append(out, d)
	}
	sort.Slice(out, func(i, j int) bool {
		if RiskRank(out[i].Risk) != RiskRank(out[j].Risk) {
			return RiskRank(out[i].Risk) < RiskRank(out[j].Risk)
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// Phase 1 tool ids.
const (
	ToolVPSList            = "vps.list"
	ToolVPSGet             = "vps.get"
	ToolVPSStatus          = "vps.status"
	ToolQuotaCheck         = "quota.check"
	ToolCatalogTemplates   = "catalog.templates"
	ToolCatalogPlans       = "catalog.plans"
	ToolVPSWaitReady       = "vps.wait_ready"
	ToolVPSVerify          = "vps.verify"
	ToolVPSCreate          = "vps.create"
	ToolVPSStart           = "vps.start"
	ToolVPSRestart         = "vps.restart"
	ToolVPSSnapshotCreate  = "vps.snapshot.create"
	ToolBucketCreate       = "bucket.create"
	ToolAppCreate          = "app.create"
	ToolN8nCreate          = "n8n.create"
	ToolOpenClawCreate     = "openclaw.create"
	ToolGameServerCreate   = "game_server.create"
	ToolAndroidCreate      = "android.create"
	ToolVPSStop            = "vps.stop"
	ToolVPSResize          = "vps.resize"
	ToolVPSDelete          = "vps.delete"
	ToolVPSSnapshotRestore = "vps.snapshot.restore"
	ToolBucketDelete       = "bucket.delete"
)

// Patterns shared by schemas and the planner.
const (
	VPSNamePattern      = `^[a-z0-9][a-z0-9\-\.]{1,62}[a-z0-9]$`
	BucketNamePattern   = `^[a-z0-9][a-z0-9\-]{1,61}[a-z0-9]$`
	ServiceNamePattern  = `^[A-Za-z0-9][A-Za-z0-9 _\.\-]{0,62}[A-Za-z0-9]$`
	SnapshotNamePattern = `^[a-zA-Z0-9_\-]{1,40}$`
)

func obj(props map[string]any, required ...string) map[string]any {
	if props == nil {
		props = map[string]any{}
	}
	if required == nil {
		required = []string{}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             required,
		"additionalProperties": false,
	}
}

func str(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }

func strPattern(desc, pattern string, maxLen int) map[string]any {
	return map[string]any{"type": "string", "description": desc, "pattern": pattern, "maxLength": maxLen}
}

func uuidStr(desc string) map[string]any {
	return map[string]any{"type": "string", "format": "uuid", "description": desc}
}

func integer(desc string, min, max int) map[string]any {
	return map[string]any{"type": "integer", "description": desc, "minimum": min, "maximum": max}
}

func boolean(desc string) map[string]any {
	return map[string]any{"type": "boolean", "description": desc}
}

func enum(desc string, values ...string) map[string]any {
	return map[string]any{"type": "string", "description": desc, "enum": values}
}

func instanceRef() map[string]any {
	return obj(map[string]any{
		"instance_id": uuidStr("Instance (VPS) id"),
		"name":        str("Instance name (informational)"),
	}, "instance_id")
}

// DefaultRegistry returns the Phase 1 tool definitions (no runners bound).
func DefaultRegistry() *Registry {
	r := NewRegistry()
	for _, d := range phase1Tools() {
		r.Register(d)
	}
	for k, tool := range defaultActions {
		typ, action, _ := strings.Cut(k, "/")
		r.MapAction(typ, action, tool)
	}
	return r
}

func phase1Tools() []ToolDef {
	vpsCreateSchema := obj(map[string]any{
		"name":        strPattern("Instance name (lowercase letters, digits, hyphens, dots)", VPSNamePattern, 64),
		"hostname":    strPattern("Guest hostname (defaults to the name)", VPSNamePattern, 64),
		"template_id": uuidStr("OS template id (catalog.templates)"),
		"plan_id":     uuidStr("Sizing plan id (catalog.plans); overrides vcpus/memory_mb/disk_gb"),
		"vcpus":       integer("Virtual CPUs", 1, 64),
		"memory_mb":   integer("Memory in MB", 512, 262144),
		"disk_gb":     integer("Disk in GB", 10, 4096),
		"ssh_key_ids": map[string]any{"type": "array", "maxItems": 10, "items": uuidStr("SSH key id"), "description": "SSH keys to install (the first one is used)"},
	}, "name", "template_id")

	return []ToolDef{
		// read
		{ID: ToolVPSList, Title: "List VPS", Description: "List the caller's virtual machines.", Risk: RiskRead,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 30, InputSchema: obj(nil)},
		{ID: ToolVPSGet, Title: "Get VPS", Description: "Get one of the caller's virtual machines.", Risk: RiskRead,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 30, InputSchema: instanceRef()},
		{ID: ToolVPSStatus, Title: "VPS status", Description: "Report a virtual machine's recorded and hypervisor power state.", Risk: RiskRead,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 30, InputSchema: instanceRef()},
		{ID: ToolQuotaCheck, Title: "Validate quota", Description: "Check that the caller's quota (including active reservations) fits the requested resources.", Risk: RiskRead,
			ResourceTypes: []string{"vps", "game_server", "android"}, Idempotent: true, TimeoutSeconds: 30, InputSchema: obj(map[string]any{
				"vcpus":     integer("Virtual CPUs to add", 0, 1024),
				"memory_mb": integer("Memory to add in MB", 0, 4194304),
				"disk_gb":   integer("Disk to add in GB", 0, 65536),
				"instances": integer("Instances to add", 0, 100),
			})},
		{ID: ToolCatalogTemplates, Title: "List OS templates", Description: "List the enabled operating system templates.", Risk: RiskRead,
			ResourceTypes: []string{"catalog"}, Idempotent: true, TimeoutSeconds: 30, InputSchema: obj(nil)},
		{ID: ToolCatalogPlans, Title: "List sizing plans", Description: "List the enabled VPS sizing plans.", Risk: RiskRead,
			ResourceTypes: []string{"catalog"}, Idempotent: true, TimeoutSeconds: 30, InputSchema: obj(nil)},
		{ID: ToolVPSWaitReady, Title: "Wait until running", Description: "Wait until a virtual machine is running and has a private IP address.", Risk: RiskRead,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 600, InputSchema: obj(map[string]any{
				"instance_id": uuidStr("Instance id (defaults to the instance of the task this one depends on)"),
				"name":        str("Instance name (informational)"),
			})},
		{ID: ToolVPSVerify, Title: "Verify VPS", Description: "Compare a virtual machine's desired and actual state (record, hypervisor power state, private IP).", Risk: RiskRead,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: instanceRef()},

		// medium
		{ID: ToolVPSCreate, Title: "Create VPS", Description: "Provision a virtual machine from an OS template (quota reserved transactionally).", Risk: RiskMedium,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 900, InputSchema: vpsCreateSchema},
		{ID: ToolVPSStart, Title: "Start VPS", Description: "Power on a stopped virtual machine.", Risk: RiskMedium,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 300, InputSchema: instanceRef()},
		{ID: ToolVPSRestart, Title: "Restart VPS", Description: "Restart a running virtual machine.", Risk: RiskMedium,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 300, InputSchema: instanceRef()},
		{ID: ToolVPSSnapshotCreate, Title: "Create VPS snapshot", Description: "Take a snapshot of a virtual machine.", Risk: RiskMedium,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 300, InputSchema: obj(map[string]any{
				"instance_id":   uuidStr("Instance id"),
				"name":          str("Instance name (informational)"),
				"snapshot_name": strPattern("Snapshot name", SnapshotNamePattern, 40),
				"description":   map[string]any{"type": "string", "maxLength": 255, "description": "Snapshot description"},
				"include_ram":   boolean("Include the RAM state"),
			}, "instance_id", "snapshot_name")},
		{ID: ToolBucketCreate, Title: "Create bucket", Description: "Create an S3-compatible object storage bucket.", Risk: RiskMedium,
			ResourceTypes: []string{"bucket"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"name":        strPattern("Bucket name (globally unique)", BucketNamePattern, 63),
				"is_public":   boolean("Public read access"),
				"description": map[string]any{"type": "string", "maxLength": 255, "description": "Description"},
			}, "name")},
		{ID: ToolAppCreate, Title: "Create application", Description: "Create a web application deployment from a Git repository.", Risk: RiskMedium,
			ResourceTypes: []string{"app"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"name":           strPattern("Application name", ServiceNamePattern, 64),
				"repository_url": strPattern("HTTPS Git repository URL", `^https://[^\s]+$`, 512),
				"framework":      enum("Framework", "go", "react", "nextjs", "python", "docker", "static", "nodejs"),
				"branch":         strPattern("Production branch", `^[A-Za-z0-9._/\-]{1,100}$`, 100),
			}, "name", "repository_url")},
		{ID: ToolN8nCreate, Title: "Create n8n instance", Description: "Provision a dedicated n8n workflow automation instance.", Risk: RiskMedium,
			ResourceTypes: []string{"n8n"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"name": strPattern("Instance name", ServiceNamePattern, 64),
			}, "name")},
		{ID: ToolOpenClawCreate, Title: "Create OpenClaw agent", Description: "Deploy an OpenClaw AI agent container. Provider API keys are configured afterwards in the OpenClaw settings, never in a plan.", Risk: RiskMedium,
			ResourceTypes: []string{"openclaw"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"name":     strPattern("Agent name", ServiceNamePattern, 64),
				"provider": enum("AI provider", "openai", "anthropic", "gemini", "ollama", "deepseek"),
				"model":    strPattern("Default model", `^[A-Za-z0-9._:/\-]{1,100}$`, 100),
			}, "name")},
		{ID: ToolGameServerCreate, Title: "Create game server", Description: "Provision a managed game server (Minecraft Java/Bedrock, Terraria).", Risk: RiskMedium,
			ResourceTypes: []string{"game_server"}, Idempotent: true, TimeoutSeconds: 120, InputSchema: obj(map[string]any{
				"name":            strPattern("Server name", ServiceNamePattern, 64),
				"game_type":       enum("Game", "minecraft_java", "minecraft_bedrock", "terraria"),
				"game_version":    strPattern("Game version", `^[0-9A-Za-z.\-]{1,32}$`, 32),
				"server_software": strPattern("Server software", `^[a-z_]{1,32}$`, 32),
				"plan_name":       enum("Sizing plan", "starter", "basic", "standard", "advanced", "custom"),
				"cpu_cores":       integer("CPU cores", 1, 32),
				"memory_mb":       integer("Memory in MB", 512, 131072),
				"storage_gb":      integer("Storage in GB", 1, 2048),
				"player_slots":    integer("Player slots", 1, 1000),
				"difficulty":      strPattern("Difficulty", `^[a-z]{1,16}$`, 16),
				"world_size":      map[string]any{"type": "string", "enum": []string{"", "small", "medium", "large"}, "description": "Terraria world size"},
				"motd":            map[string]any{"type": "string", "maxLength": 128, "description": "Message of the day"},
			}, "name", "game_type")},
		{ID: ToolAndroidCreate, Title: "Create Android instance", Description: "Provision a Waydroid Android cloud instance.", Risk: RiskMedium,
			ResourceTypes: []string{"android"}, Idempotent: true, TimeoutSeconds: 120, InputSchema: obj(map[string]any{
				"name":               strPattern("Instance name", ServiceNamePattern, 64),
				"image_id":           uuidStr("Published Android image id"),
				"plan_name":          enum("Sizing plan", "testing", "standard", "developer", "power", "custom"),
				"vcpus":              integer("Virtual CPUs", 1, 32),
				"memory_mb":          integer("Memory in MB", 1024, 65536),
				"storage_gb":         integer("Storage in GB", 8, 1024),
				"display_resolution": strPattern("Display resolution", `^[0-9]{3,4}x[0-9]{3,4}$`, 9),
				"display_density":    integer("Display density (DPI)", 120, 640),
				"orientation":        enum("Orientation", "portrait", "landscape"),
				"adb_enabled":        boolean("Enable ADB"),
			}, "name", "image_id")},

		// high
		{ID: ToolVPSStop, Title: "Stop VPS", Description: "Gracefully shut down a running virtual machine (services become unavailable).", Risk: RiskHigh,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 300, InputSchema: instanceRef()},
		{ID: ToolVPSResize, Title: "Resize VPS", Description: "Change the vCPU, memory and disk size of a stopped virtual machine (disks only grow).", Risk: RiskHigh,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 180, InputSchema: obj(map[string]any{
				"instance_id": uuidStr("Instance id"),
				"name":        str("Instance name (informational)"),
				"vcpus":       integer("Virtual CPUs", 1, 64),
				"memory_mb":   integer("Memory in MB", 512, 262144),
				"disk_gb":     integer("Disk in GB", 10, 4096),
			}, "instance_id", "vcpus", "memory_mb", "disk_gb")},

		// destructive
		{ID: ToolVPSDelete, Title: "Delete VPS", Description: "Permanently delete a virtual machine and its disks.", Risk: RiskDestructive,
			ResourceTypes: []string{"vps"}, Idempotent: true, TimeoutSeconds: 300, InputSchema: instanceRef()},
		{ID: ToolVPSSnapshotRestore, Title: "Restore VPS snapshot", Description: "Roll a virtual machine back to a snapshot (changes since the snapshot are lost).", Risk: RiskDestructive,
			ResourceTypes: []string{"vps"}, Idempotent: false, TimeoutSeconds: 300, InputSchema: obj(map[string]any{
				"instance_id":   uuidStr("Instance id"),
				"name":          str("Instance name (informational)"),
				"snapshot_name": strPattern("Snapshot name", SnapshotNamePattern, 40),
			}, "instance_id", "snapshot_name")},
		{ID: ToolBucketDelete, Title: "Delete bucket", Description: "Delete an empty object storage bucket.", Risk: RiskDestructive,
			ResourceTypes: []string{"bucket"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"bucket_id": uuidStr("Bucket id"),
				"name":      str("Bucket name (informational)"),
			}, "bucket_id")},
	}
}
