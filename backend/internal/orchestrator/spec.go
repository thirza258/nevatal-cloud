package orchestrator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Resource types of a specification.
const (
	TypeVPS        = "vps"
	TypeBucket     = "bucket"
	TypeApp        = "app"
	TypeN8n        = "n8n"
	TypeOpenClaw   = "openclaw"
	TypeGameServer = "game_server"
	TypeAndroid    = "android"
	TypeDatabase   = "database"
	TypeDomain     = "domain"
)

// Resource actions of a specification.
const (
	ActionCreate          = "create"
	ActionStart           = "start"
	ActionStop            = "stop"
	ActionRestart         = "restart"
	ActionResize          = "resize"
	ActionDelete          = "delete"
	ActionUpdate          = "update"
	ActionDeploy          = "deploy"
	ActionSnapshotCreate  = "snapshot.create"
	ActionSnapshotRestore = "snapshot.restore"
	ActionBackupCreate    = "backup.create"
	ActionBackupRestore   = "backup.restore"
	ActionObjectDelete    = "object.delete"
)

// Template parameter types and managed database engines (shared with the
// registry schemas).
const (
	TemplateParamString = "string"
	TemplateParamInt    = "integer"
	TemplateParamBool   = "boolean"
	TemplateParamEnum   = "enum"
)

const (
	DatabaseEnginePostgres = "postgresql"
	DatabaseEngineMySQL    = "mysql"
	DatabaseEngineRedis    = "redis"
	DatabaseEngineMongo    = "mongodb"
)

// defaultActions maps type/action to the tool that performs it.
var defaultActions = map[string]string{
	TypeVPS + "/" + ActionCreate:          ToolVPSCreate,
	TypeVPS + "/" + ActionStart:           ToolVPSStart,
	TypeVPS + "/" + ActionStop:            ToolVPSStop,
	TypeVPS + "/" + ActionRestart:         ToolVPSRestart,
	TypeVPS + "/" + ActionResize:          ToolVPSResize,
	TypeVPS + "/" + ActionDelete:          ToolVPSDelete,
	TypeVPS + "/" + ActionSnapshotCreate:  ToolVPSSnapshotCreate,
	TypeVPS + "/" + ActionSnapshotRestore: ToolVPSSnapshotRestore,
	TypeBucket + "/" + ActionCreate:       ToolBucketCreate,
	TypeBucket + "/" + ActionDelete:       ToolBucketDelete,
	TypeBucket + "/" + ActionUpdate:       ToolBucketUpdate,
	TypeBucket + "/" + ActionObjectDelete: ToolBucketObjectDelete,
	TypeApp + "/" + ActionCreate:          ToolAppCreate,
	TypeApp + "/" + ActionDeploy:          ToolAppDeploy,
	TypeN8n + "/" + ActionCreate:          ToolN8nCreate,
	TypeOpenClaw + "/" + ActionCreate:     ToolOpenClawCreate,
	TypeGameServer + "/" + ActionCreate:   ToolGameServerCreate,
	TypeAndroid + "/" + ActionCreate:      ToolAndroidCreate,
	TypeDatabase + "/" + ActionCreate:     ToolDatabaseCreate,
	TypeDomain + "/" + ActionCreate:       ToolDomainClaim,
	TypeDomain + "/" + ActionDelete:       ToolDomainRelease,
}

func init() {
	// Power, delete and backup operations on existing service resources all
	// go through the generic, policy-checked service tools.
	for _, typ := range []string{TypeApp, TypeN8n, TypeOpenClaw, TypeGameServer, TypeAndroid, TypeDatabase} {
		defaultActions[typ+"/"+ActionStart] = ToolServicePower
		defaultActions[typ+"/"+ActionStop] = ToolServicePower
		defaultActions[typ+"/"+ActionRestart] = ToolServicePower
		defaultActions[typ+"/"+ActionDelete] = ToolServiceDelete
	}
	for _, typ := range []string{TypeN8n, TypeOpenClaw, TypeGameServer, TypeAndroid, TypeDatabase} {
		defaultActions[typ+"/"+ActionBackupCreate] = ToolServiceBackupCreate
		defaultActions[typ+"/"+ActionBackupRestore] = ToolServiceBackupRestore
	}
}

// SpecFromTool builds the specification of a single tool call (MCP
// tools/call of a write tool). The tool input becomes the resource config.
func (r *Registry) SpecFromTool(toolID string, input map[string]any) (Specification, bool) {
	typ, action := toolToResourceAction(toolID, input)
	if typ == "" {
		return Specification{}, false
	}
	key := String(input, "name")
	if key == "" {
		for _, k := range []string{"instance_id", "bucket_id", "resource_id", "app_id", "domain_id"} {
			if id := String(input, k); id != "" {
				key = id
				break
			}
		}
	}
	if key == "" {
		key = typ
	}
	if !resourceKeyPattern.MatchString(key) {
		key = typ
	}
	cfg := make(map[string]any, len(input))
	for k, v := range input {
		cfg[k] = v
	}
	// A single-tool plan cannot contain the resources these fields point at;
	// only the chat planner builds multi-resource plans.
	for _, k := range []string{"vps_key", "database_key", "bucket_keys"} {
		delete(cfg, k)
	}
	return Specification{
		Intent:    strings.ReplaceAll(toolID, ".", "_"),
		Resources: []ResourceSpec{{Key: key, Type: typ, Action: action, Config: cfg}},
	}, true
}

// toolToResourceAction maps a registry write tool to a resource type/action.
func toolToResourceAction(toolID string, input map[string]any) (string, string) {
	switch toolID {
	case ToolServicePower:
		return String(input, "resource_type"), String(input, "action")
	case ToolServiceDelete:
		return String(input, "resource_type"), ActionDelete
	case ToolServiceBackupCreate:
		return String(input, "resource_type"), ActionBackupCreate
	case ToolServiceBackupRestore:
		return String(input, "resource_type"), ActionBackupRestore
	case ToolDatabaseCreate:
		return TypeDatabase, ActionCreate
	case ToolAppDeploy:
		return TypeApp, ActionDeploy
	case ToolBucketUpdate:
		return TypeBucket, ActionUpdate
	case ToolBucketObjectDelete:
		return TypeBucket, ActionObjectDelete
	case ToolDomainClaim:
		return TypeDomain, ActionCreate
	case ToolDomainRelease:
		return TypeDomain, ActionDelete
	}
	// Phase 1 mapping: "type/action" -> tool.
	for k, t := range defaultActions {
		if t == toolID {
			typ, action, _ := strings.Cut(k, "/")
			return typ, action
		}
	}
	return "", ""
}

// SpecError is a structural problem in a specification (HTTP 400).
type SpecError struct {
	Field   string
	Message string
}

func (e *SpecError) Error() string { return e.Field + ": " + e.Message }

var resourceKeyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._\-]{0,99}$`)

// MaxResources bounds the size of one specification.
const MaxResources = 20

// computeCreates are the creates that consume compute quota.
func consumesQuota(res ResourceSpec) bool {
	if res.Action == ActionResize {
		return true
	}
	if res.Action != ActionCreate {
		return false
	}
	switch res.Type {
	case TypeVPS, TypeGameServer, TypeAndroid, TypeDatabase:
		return true
	}
	return false
}

// CheckStructure rejects specifications the engine cannot plan at all.
func (r *Registry) CheckStructure(spec Specification) error {
	if len(spec.Resources) == 0 {
		return &SpecError{Field: "specification.resources", Message: "at least one resource is required"}
	}
	if len(spec.Resources) > MaxResources {
		return &SpecError{Field: "specification.resources", Message: fmt.Sprintf("at most %d resources are allowed", MaxResources)}
	}
	seen := map[string]bool{}
	for i, res := range spec.Resources {
		field := fmt.Sprintf("specification.resources[%d]", i)
		if !resourceKeyPattern.MatchString(res.Key) {
			return &SpecError{Field: field + ".key", Message: "must be 1-100 letters, digits, spaces, dots, underscores or hyphens"}
		}
		if seen[res.Key] {
			return &SpecError{Field: field + ".key", Message: fmt.Sprintf("duplicate resource key %q", res.Key)}
		}
		seen[res.Key] = true
		tool, ok := r.ToolFor(res.Type, res.Action)
		if !ok {
			return &SpecError{Field: field, Message: fmt.Sprintf("unsupported resource type/action %q/%q", res.Type, res.Action)}
		}
		if res.Config == nil {
			return &SpecError{Field: field + ".config", Message: "is required"}
		}
		// Only fields the tool declares may be stored: anything else (an API
		// key, a password) never reaches plans, revisions or task inputs.
		if def, ok := r.Def(tool); ok {
			props, _ := def.InputSchema["properties"].(map[string]any)
			keys := make([]string, 0, len(res.Config))
			for k := range res.Config {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				if _, known := props[k]; !known && len(props) > 0 {
					return &SpecError{Field: fmt.Sprintf("%s.config.%s", field, k), Message: "is not a field of " + tool}
				}
			}
		}
		if err := checkResourceRefs(res, field, seen); err != nil {
			return err
		}
	}
	// Every dependency must name a resource of this specification, and the
	// resource graph must be acyclic.
	for i, res := range spec.Resources {
		for _, dep := range dependencyKeys(res) {
			if dep == res.Key {
				return &SpecError{Field: fmt.Sprintf("specification.resources[%d].depends_on", i), Message: "a resource cannot depend on itself"}
			}
			if !seen[dep] {
				return &SpecError{Field: fmt.Sprintf("specification.resources[%d].depends_on", i), Message: fmt.Sprintf("unknown dependency %q", dep)}
			}
		}
	}
	if err := checkResourceCycles(spec); err != nil {
		return err
	}
	return nil
}

// checkResourceRefs validates the resource id/config fields an action needs
// before it can even be compiled.
func checkResourceRefs(res ResourceSpec, field string, seen map[string]bool) error {
	require := func(key string) error {
		if String(res.Config, key) == "" {
			return &SpecError{Field: field + ".config." + key, Message: "is required"}
		}
		return nil
	}
	switch res.Type + "/" + res.Action {
	case TypeApp + "/" + ActionDeploy:
		return require("app_id")
	case TypeBucket + "/" + ActionUpdate:
		return require("bucket_id")
	case TypeBucket + "/" + ActionObjectDelete:
		if err := require("bucket_id"); err != nil {
			return err
		}
		return require("object_id")
	case TypeDomain + "/" + ActionDelete:
		return require("domain_id")
	case TypeDatabase + "/" + ActionCreate:
		if err := require("name"); err != nil {
			return err
		}
		return require("engine")
	}
	// Generic service operations target an existing resource.
	if tool, ok := defaultActions[res.Type+"/"+res.Action]; ok {
		switch tool {
		case ToolServicePower, ToolServiceDelete, ToolServiceBackupCreate, ToolServiceBackupRestore:
			if err := require("resource_id"); err != nil {
				return err
			}
		}
	}
	return nil
}

// dependencyKeys returns the resource keys res depends on: explicit
// depends_on entries plus the implicit vps_key, database_key and bucket_keys.
func dependencyKeys(res ResourceSpec) []string {
	var keys []string
	keys = append(keys, res.DependsOn...)
	for _, field := range []string{"vps_key", "database_key"} {
		if k := String(res.Config, field); k != "" {
			keys = append(keys, k)
		}
	}
	keys = append(keys, Strings(res.Config, "bucket_keys")...)
	return dedupe(keys)
}

// checkResourceCycles rejects a resource dependency cycle.
func checkResourceCycles(spec Specification) error {
	indeg := map[string]int{}
	children := map[string][]string{}
	for _, res := range spec.Resources {
		indeg[res.Key] += 0
		for _, dep := range dependencyKeys(res) {
			indeg[res.Key]++
			children[dep] = append(children[dep], res.Key)
		}
	}
	var ready []string
	for _, res := range spec.Resources {
		if indeg[res.Key] == 0 {
			ready = append(ready, res.Key)
		}
	}
	visited := 0
	for len(ready) > 0 {
		k := ready[0]
		ready = ready[1:]
		visited++
		for _, c := range children[k] {
			indeg[c]--
			if indeg[c] == 0 {
				ready = append(ready, c)
			}
		}
	}
	if visited != len(spec.Resources) {
		return &SpecError{Field: "specification.resources", Message: "the resource dependencies contain a cycle"}
	}
	return nil
}

// Estimate totals the compute resources a specification adds (creates
// only; resizes are checked against current sizes by the policy engine).
func Estimate(spec Specification) EstimatedResources {
	var est EstimatedResources
	for _, res := range spec.Resources {
		if res.Action != ActionCreate {
			continue
		}
		switch res.Type {
		case TypeVPS:
			est.VCPUs += Int(res.Config, "vcpus", 0)
			est.MemoryMB += Int(res.Config, "memory_mb", 0)
			est.DiskGB += Int(res.Config, "disk_gb", 0)
			est.Instances++
		case TypeGameServer:
			est.VCPUs += Int(res.Config, "cpu_cores", 0)
			est.MemoryMB += Int(res.Config, "memory_mb", 0)
			est.DiskGB += Int(res.Config, "storage_gb", 0)
		case TypeAndroid:
			est.VCPUs += Int(res.Config, "vcpus", 0)
			est.MemoryMB += Int(res.Config, "memory_mb", 0)
			est.DiskGB += Int(res.Config, "storage_gb", 0)
		case TypeDatabase:
			est.VCPUs += Int(res.Config, "vcpus", 0)
			est.MemoryMB += Int(res.Config, "memory_mb", 0)
			est.DiskGB += Int(res.Config, "storage_gb", 0)
			est.Instances++
		}
	}
	return est
}

// Compile turns a (normalized) specification into its task graph. The result
// is deterministic: the same specification always yields the same tasks.
func Compile(reg *Registry, spec Specification) ([]TaskSpec, error) {
	if err := reg.CheckStructure(spec); err != nil {
		return nil, err
	}
	var tasks []TaskSpec
	quotaKey := ""
	for _, res := range spec.Resources {
		if consumesQuota(res) {
			est := Estimate(spec)
			quotaKey = ToolQuotaCheck
			tasks = append(tasks, TaskSpec{
				Key:       quotaKey,
				Tool:      ToolQuotaCheck,
				Title:     "Validate quota",
				DependsOn: []string{},
				Risk:      riskOf(reg, ToolQuotaCheck),
				Role:      RoleOf(ToolQuotaCheck, ""),
				Input: map[string]any{
					"vcpus":     est.VCPUs,
					"memory_mb": est.MemoryMB,
					"disk_gb":   est.DiskGB,
					"instances": est.Instances,
				},
			})
			break
		}
	}
	afterQuota := func() []string {
		if quotaKey == "" {
			return []string{}
		}
		return []string{quotaKey}
	}

	// Every resource's tasks are known before dependencies are resolved, so a
	// resource may depend on one declared later in the specification.
	type plannedResource struct {
		res      ResourceSpec
		tool     string
		taskKey  string
		finalKey string
	}
	planned := make([]plannedResource, 0, len(spec.Resources))
	finalTask := map[string]string{}
	for _, res := range spec.Resources {
		tool, _ := reg.ToolFor(res.Type, res.Action)
		taskKey := tool + ":" + res.Key
		finalKey := taskKey
		if res.Type == TypeVPS && (res.Action == ActionCreate || res.Action == ActionStart || res.Action == ActionRestart) {
			finalKey = ToolVPSWaitReady + ":" + res.Key
		}
		planned = append(planned, plannedResource{res: res, tool: tool, taskKey: taskKey, finalKey: finalKey})
		finalTask[res.Key] = finalKey
	}

	for _, p := range planned {
		deps := []string{}
		if consumesQuota(p.res) {
			deps = append(deps, afterQuota()...)
		}
		for _, key := range dependencyKeys(p.res) {
			if dep, ok := finalTask[key]; ok {
				deps = append(deps, dep)
			}
		}
		deps = dedupe(deps)
		tasks = append(tasks, TaskSpec{
			Key:       p.taskKey,
			Tool:      p.tool,
			Title:     taskTitle(p.res),
			DependsOn: deps,
			Risk:      riskOf(reg, p.tool),
			Role:      RoleOf(p.tool, p.res.Type),
			Input:     compileInput(p.res, p.tool),
		})
		// Power-on operations are followed by a readiness wait.
		if p.finalKey != p.taskKey {
			waitInput := map[string]any{"name": displayName(p.res)}
			if id := String(p.res.Config, "instance_id"); id != "" {
				waitInput["instance_id"] = id
			}
			tasks = append(tasks, TaskSpec{
				Key:       p.finalKey,
				Tool:      ToolVPSWaitReady,
				Title:     fmt.Sprintf("Wait until %s is running", displayName(p.res)),
				DependsOn: []string{p.taskKey},
				Risk:      riskOf(reg, ToolVPSWaitReady),
				Role:      RoleOf(ToolVPSWaitReady, p.res.Type),
				Input:     waitInput,
			})
		}
	}

	order, err := TopoOrder(tasks)
	if err != nil {
		return nil, err
	}
	byKey := make(map[string]TaskSpec, len(tasks))
	for _, t := range tasks {
		byKey[t.Key] = t
	}
	sorted := make([]TaskSpec, 0, len(tasks))
	for _, key := range order {
		sorted = append(sorted, byKey[key])
	}
	return sorted, nil
}

// compileInput builds a task input from a resource config, injecting the
// generic service tool's resource identity and action.
func compileInput(res ResourceSpec, tool string) map[string]any {
	input := copyMap(res.Config)
	switch tool {
	case ToolServicePower, ToolServiceDelete, ToolServiceBackupCreate, ToolServiceBackupRestore:
		input["resource_type"] = res.Type
		if tool == ToolServicePower {
			input["action"] = res.Action
		}
	}
	return input
}

func riskOf(reg *Registry, tool string) string {
	if reg != nil {
		if d, ok := reg.Def(tool); ok {
			return d.Risk
		}
	}
	return RiskDestructive
}

func copyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func displayName(res ResourceSpec) string {
	if n := String(res.Config, "name"); n != "" {
		return n
	}
	return res.Key
}

func taskTitle(res ResourceSpec) string {
	name := displayName(res)
	switch res.Type + "/" + res.Action {
	case TypeVPS + "/" + ActionCreate:
		return "Provision VPS " + name
	case TypeVPS + "/" + ActionStart:
		return "Start VPS " + name
	case TypeVPS + "/" + ActionStop:
		return "Stop VPS " + name
	case TypeVPS + "/" + ActionRestart:
		return "Restart VPS " + name
	case TypeVPS + "/" + ActionResize:
		return "Resize VPS " + name
	case TypeVPS + "/" + ActionDelete:
		return "Delete VPS " + name
	case TypeVPS + "/" + ActionSnapshotCreate:
		return fmt.Sprintf("Snapshot VPS %s (%s)", name, String(res.Config, "snapshot_name"))
	case TypeVPS + "/" + ActionSnapshotRestore:
		return fmt.Sprintf("Restore VPS %s from snapshot %s", name, String(res.Config, "snapshot_name"))
	case TypeBucket + "/" + ActionCreate:
		return "Create bucket " + name
	case TypeBucket + "/" + ActionDelete:
		return "Delete bucket " + name
	case TypeApp + "/" + ActionCreate:
		return "Deploy application " + name
	case TypeN8n + "/" + ActionCreate:
		return "Provision n8n instance " + name
	case TypeOpenClaw + "/" + ActionCreate:
		return "Deploy OpenClaw agent " + name
	case TypeGameServer + "/" + ActionCreate:
		return "Provision game server " + name
	case TypeAndroid + "/" + ActionCreate:
		return "Provision Android instance " + name
	case TypeDatabase + "/" + ActionCreate:
		return "Provision database " + name
	case TypeDomain + "/" + ActionCreate:
		return "Claim domain " + name
	case TypeDomain + "/" + ActionDelete:
		return "Release domain " + name
	case TypeApp + "/" + ActionDeploy:
		return "Redeploy application " + name
	case TypeBucket + "/" + ActionUpdate:
		return "Update bucket " + name
	case TypeBucket + "/" + ActionObjectDelete:
		return "Delete object from bucket " + name
	}
	switch res.Action {
	case ActionStart:
		return "Start " + typeLabel(res.Type) + " " + name
	case ActionStop:
		return "Stop " + typeLabel(res.Type) + " " + name
	case ActionRestart:
		return "Restart " + typeLabel(res.Type) + " " + name
	case ActionDelete:
		return "Delete " + typeLabel(res.Type) + " " + name
	case ActionBackupCreate:
		return "Back up " + typeLabel(res.Type) + " " + name
	case ActionBackupRestore:
		return "Restore " + typeLabel(res.Type) + " " + name + " from backup"
	case ActionUpdate:
		return "Update " + typeLabel(res.Type) + " " + name
	}
	return res.Type + " " + res.Action + " " + name
}

// PlanRisk is the highest risk of the tasks.
func PlanRisk(tasks []TaskSpec) string {
	risk := RiskRead
	for _, t := range tasks {
		risk = MaxRisk(risk, t.Risk)
	}
	return risk
}

// Summarize describes a specification in one line.
func Summarize(spec Specification) string {
	parts := make([]string, 0, len(spec.Resources))
	for _, res := range spec.Resources {
		name := displayName(res)
		switch {
		case res.Type == TypeVPS && res.Action == ActionCreate:
			parts = append(parts, fmt.Sprintf("Create VPS %s (%d vCPU, %d MB, %d GB)", name,
				Int(res.Config, "vcpus", 0), Int(res.Config, "memory_mb", 0), Int(res.Config, "disk_gb", 0)))
		default:
			verb := map[string]string{
				ActionCreate: "Create", ActionStart: "Start", ActionStop: "Stop", ActionRestart: "Restart",
				ActionResize: "Resize", ActionDelete: "Delete", ActionSnapshotCreate: "Snapshot",
				ActionSnapshotRestore: "Restore snapshot of", ActionDeploy: "Redeploy",
				ActionBackupCreate: "Back up", ActionBackupRestore: "Restore backup of",
				ActionUpdate: "Update", ActionObjectDelete: "Delete object from",
			}[res.Action]
			parts = append(parts, fmt.Sprintf("%s %s %s", verb, typeLabel(res.Type), name))
		}
	}
	return strings.Join(parts, "; ")
}

func typeLabel(t string) string {
	switch t {
	case TypeVPS:
		return "VPS"
	case TypeBucket:
		return "bucket"
	case TypeApp:
		return "application"
	case TypeN8n:
		return "n8n instance"
	case TypeOpenClaw:
		return "OpenClaw agent"
	case TypeGameServer:
		return "game server"
	case TypeAndroid:
		return "Android instance"
	case TypeDatabase:
		return "database"
	case TypeDomain:
		return "domain"
	}
	return t
}

// Descendants returns the keys of every task that depends, directly or
// transitively, on one of keys.
func Descendants(tasks []TaskSpec, keys ...string) []string {
	children := map[string][]string{}
	for _, t := range tasks {
		for _, d := range t.DependsOn {
			children[d] = append(children[d], t.Key)
		}
	}
	seen := map[string]bool{}
	queue := append([]string(nil), keys...)
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		for _, c := range children[k] {
			if !seen[c] {
				seen[c] = true
				queue = append(queue, c)
			}
		}
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// TopoOrder returns the task keys in a dependency-respecting order, or an
// error when the graph has a cycle or an unknown dependency.
func TopoOrder(tasks []TaskSpec) ([]string, error) {
	index := map[string]TaskSpec{}
	for _, t := range tasks {
		index[t.Key] = t
	}
	indeg := map[string]int{}
	children := map[string][]string{}
	for _, t := range tasks {
		indeg[t.Key] += 0
		for _, d := range t.DependsOn {
			if _, ok := index[d]; !ok {
				return nil, fmt.Errorf("task %s depends on unknown task %s", t.Key, d)
			}
			indeg[t.Key]++
			children[d] = append(children[d], t.Key)
		}
	}
	position := map[string]int{}
	for i, t := range tasks {
		position[t.Key] = i
	}
	var ready, order []string
	for _, t := range tasks {
		if indeg[t.Key] == 0 {
			ready = append(ready, t.Key)
		}
	}
	for len(ready) > 0 {
		// Stable Kahn: of the runnable tasks, take the one declared first so
		// the graph keeps the specification's order where dependencies allow.
		pick := 0
		for i := 1; i < len(ready); i++ {
			if position[ready[i]] < position[ready[pick]] {
				pick = i
			}
		}
		k := ready[pick]
		ready = append(ready[:pick], ready[pick+1:]...)
		order = append(order, k)
		for _, c := range children[k] {
			indeg[c]--
			if indeg[c] == 0 {
				ready = append(ready, c)
			}
		}
	}
	if len(order) != len(tasks) {
		return nil, fmt.Errorf("task graph has a cycle")
	}
	return order, nil
}
