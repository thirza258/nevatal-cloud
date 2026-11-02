package orchestrator

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
)

// InstanceRef is what the policy engine knows about an instance.
type InstanceRef struct {
	ID       uuid.UUID
	Name     string
	Status   string
	VCPUs    int
	MemoryMB int
	DiskGB   int
}

// BucketRef is what the policy engine knows about a bucket.
type BucketRef struct {
	ID      uuid.UUID
	Name    string
	Objects int64
}

// QuotaSnapshot is a user's quota and current usage (instances plus active
// reservations).
type QuotaSnapshot struct {
	MaxVCPUs, UsedVCPUs         int
	MaxMemoryMB, UsedMemoryMB   int
	MaxStorageGB, UsedStorageGB int
	MaxInstances, UsedInstances int
}

// CapacitySnapshot is the free capacity of the scheduling node.
type CapacitySnapshot struct {
	NodeOnline        bool
	NodeName          string
	AvailableVCPUs    int64
	AvailableMemoryMB int64
}

// Env answers the policy engine's questions about the platform. The use case
// layer implements it on top of the repositories. Lookups scoped to a user
// return nil (no error) for resources that are missing or owned by someone
// else.
type Env interface {
	IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error)
	// Normalize fills defaults and sizes into a resource config (for
	// example plan_id -> vcpus/memory_mb/disk_gb), so estimates, quota checks
	// and the spec hash all see the same values.
	Normalize(ctx context.Context, userID uuid.UUID, res *ResourceSpec) error
	TemplateEnabled(ctx context.Context, id uuid.UUID) (bool, error)
	PlanEnabled(ctx context.Context, id uuid.UUID) (bool, error)
	SSHKeyOwned(ctx context.Context, userID, id uuid.UUID) (bool, error)
	Instance(ctx context.Context, userID, id uuid.UUID) (*InstanceRef, error)
	SnapshotExists(ctx context.Context, instanceID uuid.UUID, name string) (bool, error)
	Bucket(ctx context.Context, userID, id uuid.UUID) (*BucketRef, error)
	BucketNameTaken(ctx context.Context, name string) (bool, error)
	AndroidImagePublished(ctx context.Context, id uuid.UUID) (bool, error)
	Quota(ctx context.Context, userID uuid.UUID) (*QuotaSnapshot, error)
	Capacity(ctx context.Context) (*CapacitySnapshot, error)
}

// Policy is the deterministic policy engine. Model output never reaches the
// platform without passing it: it runs when a revision is created, on
// validate, on approve and again on execute (per task).
type Policy struct {
	reg *Registry
	env Env
	now func() time.Time
}

// NewPolicy returns a policy engine over reg and env.
func NewPolicy(reg *Registry, env Env) *Policy {
	return &Policy{reg: reg, env: env, now: time.Now}
}

// Result is the policy verdict on a revision.
type Result struct {
	Validation   Validation
	Confirmation *Confirmation
	Estimate     EstimatedResources
}

// Normalize returns spec with every resource normalized by the environment
// and every value converted to its generic JSON form.
func (p *Policy) Normalize(ctx context.Context, userID uuid.UUID, spec Specification) (Specification, error) {
	out := Specification{Intent: spec.Intent, Resources: make([]ResourceSpec, len(spec.Resources))}
	for i, res := range spec.Resources {
		res.Config = copyMap(res.Config)
		if err := p.env.Normalize(ctx, userID, &res); err != nil {
			return Specification{}, err
		}
		out.Resources[i] = res
	}
	return Generic(out)
}

// Generic round-trips spec through canonical JSON so its values have the
// types a decoded document has (json.Number, []any, map[string]any).
func Generic(spec Specification) (Specification, error) {
	b, err := Canonical(spec)
	if err != nil {
		return Specification{}, err
	}
	var out Specification
	if err := DecodeJSON(string(b), &out); err != nil {
		return Specification{}, err
	}
	for i := range out.Resources {
		if out.Resources[i].Config == nil {
			out.Resources[i].Config = map[string]any{}
		}
	}
	return out, nil
}

// GenericTasks is Generic for a task list.
func GenericTasks(tasks []TaskSpec) ([]TaskSpec, error) {
	b, err := Canonical(tasks)
	if err != nil {
		return nil, err
	}
	var out []TaskSpec
	if err := DecodeJSON(string(b), &out); err != nil {
		return nil, err
	}
	for i := range out {
		if out[i].DependsOn == nil {
			out[i].DependsOn = []string{}
		}
	}
	return out, nil
}

type issues struct {
	errs, warns []Issue
}

func (is *issues) err(code, msg, task string)  { is.errs = append(is.errs, Issue{code, msg, task}) }
func (is *issues) warn(code, msg, task string) { is.warns = append(is.warns, Issue{code, msg, task}) }

// Validate checks a revision: every tool exists, every input matches its
// schema, referenced resources exist and belong to the user, catalog
// entries are valid, quota and node capacity suffice, admin-only tools are
// only used by admins. It also derives the typed confirmation destructive
// tasks require, from the database rows the tasks reference (never from
// names in the specification).
func (p *Policy) Validate(ctx context.Context, userID uuid.UUID, isAdmin bool, spec Specification, tasks []TaskSpec) Result {
	var is issues
	est := Estimate(spec)
	var expected []string

	if err := p.reg.CheckStructure(spec); err != nil {
		is.err("SPEC_INVALID", err.Error(), "")
	}
	if _, err := TopoOrder(tasks); err != nil {
		is.err("TASK_GRAPH_INVALID", err.Error(), "")
	}

	resizeDelta := EstimatedResources{}
	for _, t := range tasks {
		def, ok := p.reg.Def(t.Tool)
		if !ok {
			is.err("TOOL_NOT_FOUND", fmt.Sprintf("tool %s is not in the registry", t.Tool), t.Key)
			continue
		}
		if def.Risk != t.Risk {
			is.err("RISK_MISMATCH", fmt.Sprintf("task risk %s does not match tool risk %s", t.Risk, def.Risk), t.Key)
		}
		if def.RequiresAdmin && !isAdmin {
			is.err("ADMIN_REQUIRED", fmt.Sprintf("tool %s requires an administrator", t.Tool), t.Key)
		}
		input := t.Input
		if input == nil {
			input = map[string]any{}
		}
		if err := ValidateInput(def.InputSchema, input); err != nil {
			is.err("INPUT_INVALID", err.Error(), t.Key)
			continue
		}
		p.checkSemantics(ctx, userID, t, input, &is, &expected, &resizeDelta)
	}

	p.checkQuotaAndCapacity(ctx, userID, est, resizeDelta, &is)

	v := Validation{Status: ValidationPassed, CheckedAt: p.now().UTC(), Errors: is.errs, Warnings: is.warns}
	if v.Errors == nil {
		v.Errors = []Issue{}
	}
	if v.Warnings == nil {
		v.Warnings = []Issue{}
	}
	switch {
	case len(v.Errors) > 0:
		v.Status = ValidationFailed
	case len(v.Warnings) > 0:
		v.Status = ValidationWarnings
	}

	var conf *Confirmation
	if PlanRisk(tasks) == RiskDestructive {
		conf = &Confirmation{Type: "typed_name", Expected: dedupe(expected)}
		if len(conf.Expected) == 0 {
			conf.Expected = []string{}
		}
	}
	return Result{Validation: v, Confirmation: conf, Estimate: est}
}

func (p *Policy) instance(ctx context.Context, userID uuid.UUID, input map[string]any, task string, is *issues) *InstanceRef {
	id, ok := UUID(input, "instance_id")
	if !ok {
		is.err("INPUT_INVALID", "instance_id: must be a UUID", task)
		return nil
	}
	inst, err := p.env.Instance(ctx, userID, id)
	if err != nil {
		is.err("LOOKUP_FAILED", "could not look up the instance: "+err.Error(), task)
		return nil
	}
	if inst == nil {
		is.err("RESOURCE_NOT_FOUND", "instance not found or access denied", task)
		return nil
	}
	return inst
}

func (p *Policy) checkSemantics(ctx context.Context, userID uuid.UUID, t TaskSpec, input map[string]any, is *issues, expected *[]string, resizeDelta *EstimatedResources) {
	switch t.Tool {
	case ToolVPSCreate:
		if id, ok := UUID(input, "template_id"); ok {
			if enabled, err := p.env.TemplateEnabled(ctx, id); err != nil || !enabled {
				is.err("TEMPLATE_INVALID", "selected OS template is invalid or not available", t.Key)
			}
		}
		if id, ok := UUID(input, "plan_id"); ok {
			if enabled, err := p.env.PlanEnabled(ctx, id); err != nil || !enabled {
				is.err("PLAN_INVALID", "selected sizing plan is invalid or not available", t.Key)
			}
		}
		if Int(input, "vcpus", 0) <= 0 || Int(input, "memory_mb", 0) <= 0 || Int(input, "disk_gb", 0) <= 0 {
			is.err("SIZE_REQUIRED", "vcpus, memory_mb and disk_gb (or a plan_id) are required", t.Key)
		}
		keys := Strings(input, "ssh_key_ids")
		for _, k := range keys {
			id, err := uuid.Parse(k)
			owned := false
			if err == nil {
				owned, _ = p.env.SSHKeyOwned(ctx, userID, id)
			}
			if !owned {
				is.err("SSH_KEY_INVALID", "SSH key not found or access denied", t.Key)
			}
		}
		if len(keys) == 0 {
			is.warn("NO_SSH_KEY", "No SSH key selected: the VM is reachable through the web console only until you add one.", t.Key)
		}
	case ToolVPSGet, ToolVPSStatus, ToolVPSVerify:
		p.instance(ctx, userID, input, t.Key, is)
	case ToolVPSWaitReady:
		if String(input, "instance_id") != "" {
			p.instance(ctx, userID, input, t.Key, is)
		}
	case ToolVPSStart:
		if inst := p.instance(ctx, userID, input, t.Key, is); inst != nil && inst.Status == "running" {
			is.warn("ALREADY_RUNNING", fmt.Sprintf("%s is already running", inst.Name), t.Key)
		}
	case ToolVPSStop:
		if inst := p.instance(ctx, userID, input, t.Key, is); inst != nil && inst.Status != "running" {
			is.warn("NOT_RUNNING", fmt.Sprintf("%s is %s", inst.Name, inst.Status), t.Key)
		}
	case ToolVPSRestart:
		if inst := p.instance(ctx, userID, input, t.Key, is); inst != nil && inst.Status != "running" {
			is.err("INVALID_STATE", fmt.Sprintf("%s must be running to restart (it is %s)", inst.Name, inst.Status), t.Key)
		}
	case ToolVPSResize:
		if inst := p.instance(ctx, userID, input, t.Key, is); inst != nil {
			if Int(input, "disk_gb", 0) < inst.DiskGB {
				is.err("DISK_SHRINK", fmt.Sprintf("disks cannot shrink (current %d GB)", inst.DiskGB), t.Key)
			}
			if inst.Status != "stopped" {
				is.warn("INSTANCE_RUNNING", fmt.Sprintf("%s must be stopped when the resize runs (it is %s)", inst.Name, inst.Status), t.Key)
			}
			if d := Int(input, "vcpus", 0) - inst.VCPUs; d > 0 {
				resizeDelta.VCPUs += d
			}
			if d := Int(input, "memory_mb", 0) - inst.MemoryMB; d > 0 {
				resizeDelta.MemoryMB += d
			}
			if d := Int(input, "disk_gb", 0) - inst.DiskGB; d > 0 {
				resizeDelta.DiskGB += d
			}
		}
	case ToolVPSDelete:
		if inst := p.instance(ctx, userID, input, t.Key, is); inst != nil {
			if inst.Status == "deleting" {
				is.err("INVALID_STATE", fmt.Sprintf("%s is already being deleted", inst.Name), t.Key)
			}
			*expected = append(*expected, inst.Name)
		}
	case ToolVPSSnapshotCreate:
		if inst := p.instance(ctx, userID, input, t.Key, is); inst != nil {
			if exists, _ := p.env.SnapshotExists(ctx, inst.ID, String(input, "snapshot_name")); exists {
				is.err("SNAPSHOT_EXISTS", "a snapshot with this name already exists for this instance", t.Key)
			}
		}
	case ToolVPSSnapshotRestore:
		if inst := p.instance(ctx, userID, input, t.Key, is); inst != nil {
			if exists, _ := p.env.SnapshotExists(ctx, inst.ID, String(input, "snapshot_name")); !exists {
				is.err("SNAPSHOT_NOT_FOUND", "snapshot not found", t.Key)
			}
			*expected = append(*expected, inst.Name)
		}
	case ToolBucketCreate:
		if taken, err := p.env.BucketNameTaken(ctx, strings.ToLower(String(input, "name"))); err == nil && taken {
			is.err("NAME_TAKEN", fmt.Sprintf("bucket name %q is already taken", String(input, "name")), t.Key)
		}
	case ToolBucketDelete:
		id, ok := UUID(input, "bucket_id")
		if !ok {
			is.err("INPUT_INVALID", "bucket_id: must be a UUID", t.Key)
			return
		}
		b, err := p.env.Bucket(ctx, userID, id)
		switch {
		case err != nil:
			is.err("LOOKUP_FAILED", "could not look up the bucket: "+err.Error(), t.Key)
		case b == nil:
			is.err("RESOURCE_NOT_FOUND", "bucket not found or access denied", t.Key)
		default:
			if b.Objects > 0 {
				is.err("BUCKET_NOT_EMPTY", fmt.Sprintf("bucket %s is not empty (%d objects)", b.Name, b.Objects), t.Key)
			}
			*expected = append(*expected, b.Name)
		}
	case ToolAndroidCreate:
		if id, ok := UUID(input, "image_id"); ok {
			if published, err := p.env.AndroidImagePublished(ctx, id); err != nil || !published {
				is.err("IMAGE_INVALID", "selected Android image is unavailable or unpublished", t.Key)
			}
		}
	}
}

func (p *Policy) checkQuotaAndCapacity(ctx context.Context, userID uuid.UUID, est, resize EstimatedResources, is *issues) {
	add := EstimatedResources{
		VCPUs:     est.VCPUs + resize.VCPUs,
		MemoryMB:  est.MemoryMB + resize.MemoryMB,
		DiskGB:    est.DiskGB + resize.DiskGB,
		Instances: est.Instances,
	}
	if add == (EstimatedResources{}) {
		return
	}
	q, err := p.env.Quota(ctx, userID)
	if err != nil {
		is.err("LOOKUP_FAILED", "could not read the quota: "+err.Error(), ToolQuotaCheck)
		return
	}
	if msg := QuotaShortfall(q, add); msg != "" {
		is.err("QUOTA_EXCEEDED", msg, ToolQuotaCheck)
	}
	if add.VCPUs == 0 && add.MemoryMB == 0 {
		return
	}
	c, err := p.env.Capacity(ctx)
	switch {
	case err != nil:
		is.err("LOOKUP_FAILED", "could not read node capacity: "+err.Error(), ToolQuotaCheck)
	case !c.NodeOnline:
		is.err("NO_CAPACITY", "no healthy hypervisor node is available for scheduling", ToolQuotaCheck)
	case int64(add.VCPUs) > c.AvailableVCPUs:
		is.err("NO_CAPACITY", fmt.Sprintf("node %s has %d vCPUs available, %d requested", c.NodeName, c.AvailableVCPUs, add.VCPUs), ToolQuotaCheck)
	case int64(add.MemoryMB) > c.AvailableMemoryMB:
		is.err("NO_CAPACITY", fmt.Sprintf("node %s has %d MB memory available, %d MB requested", c.NodeName, c.AvailableMemoryMB, add.MemoryMB), ToolQuotaCheck)
	}
}

// QuotaShortfall describes why add does not fit q ("" when it fits).
func QuotaShortfall(q *QuotaSnapshot, add EstimatedResources) string {
	switch {
	case q.UsedVCPUs+add.VCPUs > q.MaxVCPUs:
		return fmt.Sprintf("exceeds vCPU quota: %d requested, %d available", add.VCPUs, q.MaxVCPUs-q.UsedVCPUs)
	case q.UsedMemoryMB+add.MemoryMB > q.MaxMemoryMB:
		return fmt.Sprintf("exceeds memory quota: %d MB requested, %d MB available", add.MemoryMB, q.MaxMemoryMB-q.UsedMemoryMB)
	case q.UsedStorageGB+add.DiskGB > q.MaxStorageGB:
		return fmt.Sprintf("exceeds storage quota: %d GB requested, %d GB available", add.DiskGB, q.MaxStorageGB-q.UsedStorageGB)
	case q.UsedInstances+add.Instances > q.MaxInstances:
		return fmt.Sprintf("exceeds maximum instance count: limit is %d instances", q.MaxInstances)
	}
	return ""
}

// CheckTask is the execute-time policy check of one task: the tool exists,
// admin-only tools run only for admins (role read from the database), the
// input matches the schema and referenced resources still belong to the
// user. Quota is enforced again by the tools (the transactional reservation
// of instance creation is the final authority).
func (p *Policy) CheckTask(ctx context.Context, userID uuid.UUID, tool string, input map[string]any) *TaskError {
	def, ok := p.reg.Def(tool)
	if !ok {
		return Permanent("TOOL_NOT_FOUND", fmt.Sprintf("tool %s is not in the registry", tool))
	}
	if _, bound := p.reg.Runner(tool); !bound {
		return Permanent("TOOL_NOT_FOUND", fmt.Sprintf("tool %s has no implementation", tool))
	}
	if def.RequiresAdmin {
		admin, err := p.env.IsAdmin(ctx, userID)
		if err != nil {
			return Retryable("LOOKUP_FAILED", "could not read the user's role")
		}
		if !admin {
			return Permanent("ADMIN_REQUIRED", fmt.Sprintf("tool %s requires an administrator", tool))
		}
	}
	if input == nil {
		input = map[string]any{}
	}
	if err := ValidateInput(def.InputSchema, input); err != nil {
		return Permanent("INPUT_INVALID", err.Error())
	}
	if id, ok := UUID(input, "instance_id"); ok && tool != ToolVPSDelete {
		inst, err := p.env.Instance(ctx, userID, id)
		if err != nil {
			return Retryable("LOOKUP_FAILED", "could not look up the instance")
		}
		if inst == nil {
			return Permanent("RESOURCE_NOT_FOUND", "instance not found or access denied")
		}
	}
	if id, ok := UUID(input, "bucket_id"); ok && tool != ToolBucketDelete {
		b, err := p.env.Bucket(ctx, userID, id)
		if err != nil {
			return Retryable("LOOKUP_FAILED", "could not look up the bucket")
		}
		if b == nil {
			return Permanent("RESOURCE_NOT_FOUND", "bucket not found or access denied")
		}
	}
	return nil
}

// ConfirmationMatches compares a typed confirmation with the expected names:
// comma-separated, surrounding spaces ignored, order-insensitive,
// case-sensitive.
func ConfirmationMatches(typed string, expected []string) bool {
	var got []string
	for _, part := range strings.Split(typed, ",") {
		if p := strings.TrimSpace(part); p != "" {
			got = append(got, p)
		}
	}
	want := dedupe(expected)
	got = dedupe(got)
	if len(got) != len(want) {
		return false
	}
	sort.Strings(got)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(in))
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
