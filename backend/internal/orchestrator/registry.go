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
	Role           string         `json:"role"`
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
	if def.Role == "" {
		def.Role = RoleForTool(def.ID)
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

// Phase 2-4 tool ids.
const (
	ToolServiceList          = "service.list"
	ToolServiceGet           = "service.get"
	ToolServiceLogs          = "service.logs"
	ToolServicePower         = "service.power"
	ToolServiceDelete        = "service.delete"
	ToolServiceBackupCreate  = "service.backup.create"
	ToolServiceBackupRestore = "service.backup.restore"
	ToolDatabaseCreate       = "database.create"
	ToolAppDeploy            = "app.deploy"
	ToolBucketUpdate         = "bucket.update"
	ToolBucketObjectList     = "bucket.objects.list"
	ToolBucketObjectDelete   = "bucket.object.delete"
	ToolDomainList           = "domain.list"
	ToolDomainClaim          = "domain.claim"
	ToolDomainRelease        = "domain.release"
	ToolTemplateList         = "template.list"
	ToolTemplateGet          = "template.get"
	ToolTemplateSave         = "template.save"
	ToolTemplateDeploy       = "template.deploy"
	ToolDiagnosticsResource  = "diagnostics.resource"
	ToolAdvisorOptimize      = "advisor.optimize"
	ToolTopology             = "infra.topology"
)

// Patterns shared by schemas and the planner.
const (
	VPSNamePattern      = `^[a-z0-9][a-z0-9\-\.]{1,62}[a-z0-9]$`
	BucketNamePattern   = `^[a-z0-9][a-z0-9\-]{1,61}[a-z0-9]$`
	ServiceNamePattern  = `^[A-Za-z0-9][A-Za-z0-9 _\.\-]{0,62}[A-Za-z0-9]$`
	SnapshotNamePattern = `^[a-zA-Z0-9_\-]{1,40}$`
	DatabaseNamePattern = `^[a-z][a-z0-9\-]{1,62}$`
	SubdomainPattern    = `^[a-z0-9](?:[a-z0-9-]*[a-z0-9])$`
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

// DefaultRegistry returns the orchestrator tool definitions (no runners bound).
func DefaultRegistry() *Registry {
	r := NewRegistry()
	for _, d := range phase1Tools() {
		r.Register(d)
	}
	for _, d := range phase2Tools() {
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
				"name":            strPattern("Application name", ServiceNamePattern, 64),
				"repository_url":  strPattern("HTTPS Git repository URL", `^https://[^\s]+$`, 512),
				"framework":       enum("Framework", "go", "react", "nextjs", "python", "docker", "static", "nodejs"),
				"branch":          strPattern("Production branch", `^[A-Za-z0-9._/\-]{1,100}$`, 100),
				"database_key":    str("Resource key of a database in the same plan that the application uses"),
				"bucket_keys":     map[string]any{"type": "array", "maxItems": 10, "items": str("Bucket resource key"), "description": "Resource keys of buckets in the same plan that the application uses"},
				"vps_key":         str("Resource key of a VPS in the same plan that hosts the application"),
				"vps_instance_id": uuidStr("Existing VPS id that hosts the application"),
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

// Phase 2-4 tools: operate existing services, manage databases, domains,
// buckets, reusable templates, diagnostics, the advisor and topology. Generic
// service.* tools keep the registry small; the policy engine looks up the
// target resource by type and id and the execution engine enforces the same
// risk, approval and ownership rules as the type-specific Phase 1 tools.
func phase2Tools() []ToolDef {
	serviceTypes := []string{TypeVPS, TypeApp, TypeN8n, TypeOpenClaw, TypeGameServer, TypeAndroid, TypeDatabase}
	backupTypes := []string{TypeN8n, TypeOpenClaw, TypeGameServer, TypeAndroid, TypeDatabase}
	allTypes := append(append([]string{}, serviceTypes...), TypeBucket, TypeDomain)

	serviceListSchema := obj(map[string]any{
		"resource_type": enum("Resource type", allTypes...),
		"status":        str("Optional status filter"),
	})
	serviceGetSchema := obj(map[string]any{
		"resource_type": enum("Resource type", allTypes...),
		"resource_id":   uuidStr("Resource id"),
		"name":          str("Resource name (informational)"),
	}, "resource_type", "resource_id")
	serviceLogsSchema := obj(map[string]any{
		"resource_type": enum("Resource type", serviceTypes...),
		"resource_id":   uuidStr("Resource id"),
		"lines":         integer("Number of log lines", 1, 2000),
	}, "resource_type", "resource_id")
	servicePowerSchema := obj(map[string]any{
		"resource_type": enum("Resource type", serviceTypes...),
		"resource_id":   uuidStr("Resource id"),
		"action":        enum("Power action", ActionStart, ActionStop, ActionRestart),
		"name":          str("Resource name (informational)"),
	}, "resource_type", "resource_id", "action")
	serviceDeleteSchema := obj(map[string]any{
		"resource_type": enum("Resource type", serviceTypes...),
		"resource_id":   uuidStr("Resource id"),
		"name":          str("Resource name (informational)"),
	}, "resource_type", "resource_id")
	serviceBackupCreateSchema := obj(map[string]any{
		"resource_type": enum("Resource type", backupTypes...),
		"resource_id":   uuidStr("Resource id"),
		"name":          str("Backup name (defaults to a timestamped name)"),
	}, "resource_type", "resource_id")
	serviceBackupRestoreSchema := obj(map[string]any{
		"resource_type": enum("Resource type", backupTypes...),
		"resource_id":   uuidStr("Resource id"),
		"backup_id":     uuidStr("Backup id"),
		"name":          str("Resource name (informational)"),
	}, "resource_type", "resource_id", "backup_id")

	templateParamSchema := obj(map[string]any{
		"key":      strPattern("Parameter key", `^[A-Za-z_][A-Za-z0-9_]{0,63}$`, 64),
		"label":    str("Human-readable label"),
		"type":     enum("Parameter type", TemplateParamString, TemplateParamInt, TemplateParamBool, TemplateParamEnum),
		"required": boolean("Required when deploying"),
		"default":  map[string]any{"description": "Default value"},
		"enum":     map[string]any{"type": "array", "maxItems": 50, "items": str("Allowed value")},
	}, "key")

	return []ToolDef{
		// Read tools.
		{ID: ToolServiceList, Title: "List service resources", Description: "List the caller's VPS, applications, n8n, OpenClaw, game servers, Android instances, databases, buckets or domains.", Risk: RiskRead,
			ResourceTypes: allTypes, Idempotent: true, TimeoutSeconds: 30, InputSchema: serviceListSchema},
		{ID: ToolServiceGet, Title: "Get service resource", Description: "Get one of the caller's service resources.", Risk: RiskRead,
			ResourceTypes: allTypes, Idempotent: true, TimeoutSeconds: 30, InputSchema: serviceGetSchema},
		{ID: ToolServiceLogs, Title: "Get service logs", Description: "Read recent logs of an application, n8n instance, OpenClaw agent, game server, Android instance or database.", Risk: RiskRead,
			ResourceTypes: serviceTypes, Idempotent: true, TimeoutSeconds: 60, InputSchema: serviceLogsSchema},
		{ID: ToolBucketObjectList, Title: "List bucket objects", Description: "List the objects and folders of one of the caller's buckets.", Risk: RiskRead,
			ResourceTypes: []string{TypeBucket}, Idempotent: true, TimeoutSeconds: 30, InputSchema: obj(map[string]any{
				"bucket_id": uuidStr("Bucket id"),
				"prefix":    str("Key prefix"),
				"limit":     integer("Maximum objects", 1, 1000),
			}, "bucket_id")},
		{ID: ToolDomainList, Title: "List domains", Description: "List the caller's registered domains and their live status.", Risk: RiskRead,
			ResourceTypes: []string{TypeDomain}, Idempotent: true, TimeoutSeconds: 30, InputSchema: obj(nil)},
		{ID: ToolTemplateList, Title: "List infrastructure templates", Description: "List the caller's templates and public templates.", Risk: RiskRead,
			ResourceTypes: []string{"template"}, Idempotent: true, TimeoutSeconds: 30, InputSchema: obj(nil)},
		{ID: ToolTemplateGet, Title: "Get infrastructure template", Description: "Get one visible infrastructure template.", Risk: RiskRead,
			ResourceTypes: []string{"template"}, Idempotent: true, TimeoutSeconds: 30, InputSchema: obj(map[string]any{
				"template_id": uuidStr("Template id"),
			}, "template_id")},
		{ID: ToolDiagnosticsResource, Title: "Diagnose resource", Description: "Collect authorized metrics, status and logs and rank likely causes with remediation options.", Risk: RiskRead,
			ResourceTypes: allTypes, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"resource_type": enum("Resource type", allTypes...),
				"resource_id":   uuidStr("Resource id"),
			}, "resource_type", "resource_id")},
		{ID: ToolAdvisorOptimize, Title: "Optimization advisor", Description: "Analyze the caller's resources and recommend optimizations with estimated costs. Never changes anything.", Risk: RiskRead,
			ResourceTypes: []string{"account"}, Idempotent: true, TimeoutSeconds: 120, InputSchema: obj(nil)},
		{ID: ToolTopology, Title: "Infrastructure topology", Description: "Return the caller's resources and the relationships between them.", Risk: RiskRead,
			ResourceTypes: []string{"account"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(nil)},

		// Medium tools.
		{ID: ToolServiceBackupCreate, Title: "Back up service", Description: "Create a backup of an n8n instance, OpenClaw agent, game server, Android instance or database.", Risk: RiskMedium,
			ResourceTypes: backupTypes, Idempotent: false, TimeoutSeconds: 600, InputSchema: serviceBackupCreateSchema},
		{ID: ToolDatabaseCreate, Title: "Create database", Description: "Provision a managed PostgreSQL, MySQL, Redis or MongoDB database with generated credentials.", Risk: RiskMedium,
			ResourceTypes: []string{TypeDatabase}, Idempotent: true, TimeoutSeconds: 900, InputSchema: obj(map[string]any{
				"name":            strPattern("Database name", DatabaseNamePattern, 63),
				"engine":          enum("Engine", DatabaseEnginePostgres, DatabaseEngineMySQL, DatabaseEngineRedis, DatabaseEngineMongo),
				"engine_version":  strPattern("Engine version", `^[A-Za-z0-9.\-]{0,32}$`, 32),
				"plan_name":       enum("Sizing plan", "starter", "standard", "power", "custom"),
				"vcpus":           integer("Virtual CPUs", 1, 16),
				"memory_mb":       integer("Memory in MB", 256, 65536),
				"storage_gb":      integer("Storage in GB", 5, 1024),
				"database_name":   strPattern("Logical database name", `^[a-z][a-z0-9_]{0,62}$`, 63),
				"auto_backup":     boolean("Enable automatic backups"),
				"vps_key":         str("Resource key of a VPS in the same plan that hosts the database"),
				"vps_instance_id": uuidStr("Existing VPS id that hosts the database"),
			}, "name", "engine")},
		{ID: ToolAppDeploy, Title: "Redeploy application", Description: "Trigger a new deployment of a Git-backed application.", Risk: RiskMedium,
			ResourceTypes: []string{TypeApp}, Idempotent: false, TimeoutSeconds: 300, InputSchema: obj(map[string]any{
				"app_id":         uuidStr("Application id"),
				"commit_message": str("Deployment message"),
				"name":           str("Application name (informational)"),
			}, "app_id")},
		{ID: ToolBucketUpdate, Title: "Update bucket", Description: "Change the visibility or description of an object storage bucket.", Risk: RiskMedium,
			ResourceTypes: []string{TypeBucket}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"bucket_id":   uuidStr("Bucket id"),
				"is_public":   boolean("Public read access"),
				"description": str("Description"),
			}, "bucket_id")},
		{ID: ToolDomainClaim, Title: "Claim domain", Description: "Publish one port of a VPS at https://<subdomain>.<platform domain>.", Risk: RiskMedium,
			ResourceTypes: []string{TypeDomain}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"subdomain":   strPattern("Subdomain", SubdomainPattern, 63),
				"instance_id": uuidStr("Instance (VPS) id"),
				"target_port": integer("Backend port", 1, 65535),
			}, "subdomain", "instance_id", "target_port")},
		{ID: ToolTemplateSave, Title: "Save infrastructure template", Description: "Save a parameterized infrastructure specification as a reusable template. Secrets are rejected.", Risk: RiskLow,
			ResourceTypes: []string{"template"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"name":          strPattern("Template name", ServiceNamePattern, 120),
				"description":   map[string]any{"type": "string", "maxLength": 2000, "description": "Description"},
				"specification": map[string]any{"type": "object", "description": "Specification with {{parameter}} placeholders"},
				"parameters":    map[string]any{"type": "array", "maxItems": 30, "items": templateParamSchema, "description": "Parameters"},
				"is_public":     boolean("Visible to every user"),
			}, "name", "specification")},
		{ID: ToolTemplateDeploy, Title: "Deploy infrastructure template", Description: "Create a plan from a saved template with parameter values. Nothing runs until the plan is approved.", Risk: RiskMedium,
			ResourceTypes: []string{"template"}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"template_id": uuidStr("Template id"),
				"parameters":  map[string]any{"type": "object", "description": "Parameter values"},
			}, "template_id")},

		// High tools.
		{ID: ToolServicePower, Title: "Power service resource", Description: "Start, stop or restart an application, n8n instance, OpenClaw agent, game server, Android instance or database.", Risk: RiskHigh,
			ResourceTypes: serviceTypes, Idempotent: true, TimeoutSeconds: 300, InputSchema: servicePowerSchema},
		{ID: ToolDomainRelease, Title: "Release domain", Description: "Stop publishing a domain (the site becomes unreachable at that address).", Risk: RiskHigh,
			ResourceTypes: []string{TypeDomain}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"domain_id": uuidStr("Domain id"),
				"name":      str("Hostname (informational)"),
			}, "domain_id")},

		// Destructive tools.
		{ID: ToolServiceDelete, Title: "Delete service resource", Description: "Permanently delete an application, n8n instance, OpenClaw agent, game server, Android instance or database.", Risk: RiskDestructive,
			ResourceTypes: serviceTypes, Idempotent: true, TimeoutSeconds: 600, InputSchema: serviceDeleteSchema},
		{ID: ToolServiceBackupRestore, Title: "Restore service backup", Description: "Restore a game server, Android instance or database from a backup (changes since the backup are lost).", Risk: RiskDestructive,
			ResourceTypes: backupTypes, Idempotent: false, TimeoutSeconds: 900, InputSchema: serviceBackupRestoreSchema},
		{ID: ToolBucketObjectDelete, Title: "Delete bucket object", Description: "Permanently delete an object or folder from an object storage bucket.", Risk: RiskDestructive,
			ResourceTypes: []string{TypeBucket}, Idempotent: true, TimeoutSeconds: 60, InputSchema: obj(map[string]any{
				"bucket_id": uuidStr("Bucket id"),
				"object_id": uuidStr("Object id"),
				"name":      str("Object key (informational)"),
			}, "bucket_id", "object_id")},
	}
}
