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
)

// Resource actions of a specification.
const (
	ActionCreate          = "create"
	ActionStart           = "start"
	ActionStop            = "stop"
	ActionRestart         = "restart"
	ActionResize          = "resize"
	ActionDelete          = "delete"
	ActionSnapshotCreate  = "snapshot.create"
	ActionSnapshotRestore = "snapshot.restore"
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
	TypeApp + "/" + ActionCreate:          ToolAppCreate,
	TypeN8n + "/" + ActionCreate:          ToolN8nCreate,
	TypeOpenClaw + "/" + ActionCreate:     ToolOpenClawCreate,
	TypeGameServer + "/" + ActionCreate:   ToolGameServerCreate,
	TypeAndroid + "/" + ActionCreate:      ToolAndroidCreate,
}

// SpecFromTool builds the specification of a single tool call (MCP
// tools/call of a write tool). The tool input becomes the resource config.
func (r *Registry) SpecFromTool(toolID string, input map[string]any) (Specification, bool) {
	r.mu.RLock()
	var typ, action string
	for k, t := range r.actions {
		if t == toolID {
			typ, action, _ = strings.Cut(k, "/")
			break
		}
	}
	r.mu.RUnlock()
	if typ == "" {
		return Specification{}, false
	}
	key := String(input, "name")
	if key == "" {
		if id := String(input, "instance_id"); id != "" {
			key = id
		} else if id := String(input, "bucket_id"); id != "" {
			key = id
		} else {
			key = typ
		}
	}
	if !resourceKeyPattern.MatchString(key) {
		key = typ
	}
	cfg := make(map[string]any, len(input))
	for k, v := range input {
		cfg[k] = v
	}
	return Specification{
		Intent:    strings.ReplaceAll(toolID, ".", "_"),
		Resources: []ResourceSpec{{Key: key, Type: typ, Action: action, Config: cfg}},
	}, true
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
	case TypeVPS, TypeGameServer, TypeAndroid:
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

	for _, res := range spec.Resources {
		tool, _ := reg.ToolFor(res.Type, res.Action)
		key := tool + ":" + res.Key
		input := copyMap(res.Config)
		deps := []string{}
		if consumesQuota(res) {
			deps = afterQuota()
		}
		tasks = append(tasks, TaskSpec{
			Key:       key,
			Tool:      tool,
			Title:     taskTitle(res),
			DependsOn: deps,
			Risk:      riskOf(reg, tool),
			Input:     input,
		})
		// Power-on operations are followed by a readiness wait.
		if res.Type == TypeVPS && (res.Action == ActionCreate || res.Action == ActionStart || res.Action == ActionRestart) {
			waitInput := map[string]any{"name": displayName(res)}
			if id := String(res.Config, "instance_id"); id != "" {
				waitInput["instance_id"] = id
			}
			tasks = append(tasks, TaskSpec{
				Key:       ToolVPSWaitReady + ":" + res.Key,
				Tool:      ToolVPSWaitReady,
				Title:     fmt.Sprintf("Wait until %s is running", displayName(res)),
				DependsOn: []string{key},
				Risk:      riskOf(reg, ToolVPSWaitReady),
				Input:     waitInput,
			})
		}
	}
	return tasks, nil
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
				ActionSnapshotRestore: "Restore snapshot of",
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
	var ready, order []string
	for _, t := range tasks {
		if indeg[t.Key] == 0 {
			ready = append(ready, t.Key)
		}
	}
	for len(ready) > 0 {
		k := ready[0]
		ready = ready[1:]
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
