package orchestrator

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
)

func mustGeneric(t *testing.T, spec Specification) Specification {
	t.Helper()
	g, err := Generic(spec)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func sampleSpec(tpl uuid.UUID) Specification {
	return Specification{Intent: "stack", Resources: []ResourceSpec{
		{Key: "web-01", Type: TypeVPS, Action: ActionCreate, Config: map[string]any{
			"name": "web-01", "template_id": tpl.String(), "vcpus": 2, "memory_mb": 4096, "disk_gb": 40,
		}},
		{Key: "assets", Type: TypeBucket, Action: ActionCreate, Config: map[string]any{"name": "assets"}},
		{Key: "flows", Type: TypeN8n, Action: ActionCreate, Config: map[string]any{"name": "flows"}},
	}}
}

func TestCompileBuildsDAG(t *testing.T) {
	reg := DefaultRegistry()
	tasks, err := Compile(reg, mustGeneric(t, sampleSpec(uuid.New())))
	if err != nil {
		t.Fatal(err)
	}
	keys := []string{}
	byKey := map[string]TaskSpec{}
	for _, task := range tasks {
		keys = append(keys, task.Key)
		byKey[task.Key] = task
	}
	want := []string{"quota.check", "vps.create:web-01", "vps.wait_ready:web-01", "bucket.create:assets", "n8n.create:flows"}
	if strings.Join(keys, ",") != strings.Join(want, ",") {
		t.Fatalf("tasks = %v, want %v", keys, want)
	}
	if d := byKey["vps.create:web-01"].DependsOn; len(d) != 1 || d[0] != "quota.check" {
		t.Errorf("vps.create depends on %v", d)
	}
	if d := byKey["vps.wait_ready:web-01"].DependsOn; len(d) != 1 || d[0] != "vps.create:web-01" {
		t.Errorf("wait_ready depends on %v", d)
	}
	if len(byKey["bucket.create:assets"].DependsOn) != 0 || len(byKey["n8n.create:flows"].DependsOn) != 0 {
		t.Errorf("independent tasks must not depend on anything")
	}
	if q := byKey["quota.check"].Input; Int(q, "vcpus", 0) != 2 || Int(q, "instances", 0) != 1 {
		t.Errorf("quota input = %v", q)
	}
	if PlanRisk(tasks) != RiskMedium {
		t.Errorf("risk = %s", PlanRisk(tasks))
	}
	order, err := TopoOrder(tasks)
	if err != nil {
		t.Fatal(err)
	}
	pos := map[string]int{}
	for i, k := range order {
		pos[k] = i
	}
	if !(pos["quota.check"] < pos["vps.create:web-01"] && pos["vps.create:web-01"] < pos["vps.wait_ready:web-01"]) {
		t.Errorf("topological order wrong: %v", order)
	}
	if d := Descendants(tasks, "quota.check"); strings.Join(d, ",") != "vps.create:web-01,vps.wait_ready:web-01" {
		t.Errorf("descendants = %v", d)
	}
}

func TestCompileRejectsBadSpecs(t *testing.T) {
	reg := DefaultRegistry()
	cases := []Specification{
		{},
		{Resources: []ResourceSpec{{Key: "a", Type: "vps", Action: "explode", Config: map[string]any{}}}},
		{Resources: []ResourceSpec{{Key: "a", Type: "vps", Action: "start", Config: map[string]any{}}, {Key: "a", Type: "vps", Action: "stop", Config: map[string]any{}}}},
		{Resources: []ResourceSpec{{Key: "", Type: "vps", Action: "start", Config: map[string]any{}}}},
	}
	for i, spec := range cases {
		if _, err := Compile(reg, spec); err == nil {
			t.Errorf("case %d: expected an error", i)
		}
	}
	if _, err := TopoOrder([]TaskSpec{{Key: "a", DependsOn: []string{"b"}}, {Key: "b", DependsOn: []string{"a"}}}); err == nil {
		t.Error("cycle not detected")
	}
}

func TestSpecHashStableAndSensitive(t *testing.T) {
	reg := DefaultRegistry()
	tpl := uuid.New()
	spec := mustGeneric(t, sampleSpec(tpl))
	tasks, _ := Compile(reg, spec)
	tasks, _ = GenericTasks(tasks)
	h1, err := SpecHash(spec, tasks)
	if err != nil || !strings.HasPrefix(h1, "sha256:") || len(h1) != len("sha256:")+64 {
		t.Fatalf("hash = %q, %v", h1, err)
	}
	// Round trip through canonical JSON (as stored in the database).
	specJSON, _ := Canonical(spec)
	tasksJSON, _ := Canonical(tasks)
	var spec2 Specification
	var tasks2 []TaskSpec
	if err := DecodeJSON(string(specJSON), &spec2); err != nil {
		t.Fatal(err)
	}
	if err := DecodeJSON(string(tasksJSON), &tasks2); err != nil {
		t.Fatal(err)
	}
	if h2, _ := SpecHash(spec2, tasks2); h2 != h1 {
		t.Fatalf("hash changed after round trip: %s != %s", h2, h1)
	}
	// Any change of the specification changes the hash.
	changed := sampleSpec(tpl)
	changed.Resources[0].Config["memory_mb"] = 8192
	changed = mustGeneric(t, changed)
	ct, _ := Compile(reg, changed)
	ct, _ = GenericTasks(ct)
	if h3, _ := SpecHash(changed, ct); h3 == h1 {
		t.Fatal("hash did not change with the specification")
	}
}

func TestValidateInput(t *testing.T) {
	reg := DefaultRegistry()
	def, _ := reg.Def(ToolOpenClawCreate)
	if err := ValidateInput(def.InputSchema, map[string]any{"name": "agent", "api_key": "sk-secret"}); err == nil || !strings.Contains(err.Error(), "api_key") {
		t.Errorf("secret field accepted: %v", err)
	}
	vps, _ := reg.Def(ToolVPSCreate)
	for _, in := range []map[string]any{
		{"template_id": uuid.NewString()},
		{"name": "Bad Name", "template_id": uuid.NewString()},
		{"name": "ok-name", "template_id": "not-a-uuid"},
		{"name": "ok-name", "template_id": uuid.NewString(), "vcpus": 1.5},
		{"name": "ok-name", "template_id": uuid.NewString(), "vcpus": 999},
	} {
		if err := ValidateInput(vps.InputSchema, in); err == nil {
			t.Errorf("accepted invalid input %v", in)
		}
	}
	if err := ValidateInput(vps.InputSchema, map[string]any{"name": "ok-name", "template_id": uuid.NewString(), "vcpus": 2, "ssh_key_ids": []any{uuid.NewString()}}); err != nil {
		t.Errorf("rejected valid input: %v", err)
	}
}

// fakeEnv is an in-memory policy environment.
type fakeEnv struct {
	admin     bool
	instances map[uuid.UUID]*InstanceRef
	buckets   map[uuid.UUID]*BucketRef
	quota     QuotaSnapshot
	capacity  CapacitySnapshot
}

func (f *fakeEnv) IsAdmin(context.Context, uuid.UUID) (bool, error) { return f.admin, nil }
func (f *fakeEnv) Normalize(context.Context, uuid.UUID, *ResourceSpec) error {
	return nil
}
func (f *fakeEnv) TemplateEnabled(context.Context, uuid.UUID) (bool, error)        { return true, nil }
func (f *fakeEnv) PlanEnabled(context.Context, uuid.UUID) (bool, error)            { return true, nil }
func (f *fakeEnv) SSHKeyOwned(context.Context, uuid.UUID, uuid.UUID) (bool, error) { return true, nil }
func (f *fakeEnv) Instance(_ context.Context, _ uuid.UUID, id uuid.UUID) (*InstanceRef, error) {
	return f.instances[id], nil
}
func (f *fakeEnv) SnapshotExists(context.Context, uuid.UUID, string) (bool, error) { return false, nil }
func (f *fakeEnv) Bucket(_ context.Context, _ uuid.UUID, id uuid.UUID) (*BucketRef, error) {
	return f.buckets[id], nil
}
func (f *fakeEnv) BucketNameTaken(context.Context, string) (bool, error)          { return false, nil }
func (f *fakeEnv) AndroidImagePublished(context.Context, uuid.UUID) (bool, error) { return true, nil }
func (f *fakeEnv) Quota(context.Context, uuid.UUID) (*QuotaSnapshot, error) {
	q := f.quota
	return &q, nil
}
func (f *fakeEnv) Capacity(context.Context) (*CapacitySnapshot, error) {
	c := f.capacity
	return &c, nil
}

func newFakeEnv() *fakeEnv {
	return &fakeEnv{
		instances: map[uuid.UUID]*InstanceRef{},
		buckets:   map[uuid.UUID]*BucketRef{},
		quota:     QuotaSnapshot{MaxVCPUs: 4, MaxMemoryMB: 8192, MaxStorageGB: 100, MaxInstances: 3},
		capacity:  CapacitySnapshot{NodeOnline: true, NodeName: "pve", AvailableVCPUs: 64, AvailableMemoryMB: 65536},
	}
}

func validate(t *testing.T, reg *Registry, env Env, admin bool, spec Specification) Result {
	t.Helper()
	spec = mustGeneric(t, spec)
	tasks, err := Compile(reg, spec)
	if err != nil {
		t.Fatal(err)
	}
	tasks, _ = GenericTasks(tasks)
	return NewPolicy(reg, env).Validate(context.Background(), uuid.New(), admin, spec, tasks)
}

func hasIssue(list []Issue, code string) bool {
	for _, i := range list {
		if i.Code == code {
			return true
		}
	}
	return false
}

func TestPolicyQuotaAndWarnings(t *testing.T) {
	reg := DefaultRegistry()
	env := newFakeEnv()
	res := validate(t, reg, env, false, sampleSpec(uuid.New()))
	if res.Validation.Status != ValidationWarnings || !hasIssue(res.Validation.Warnings, "NO_SSH_KEY") {
		t.Fatalf("validation = %+v", res.Validation)
	}
	env.quota.UsedVCPUs = 3
	res = validate(t, reg, env, false, sampleSpec(uuid.New()))
	if res.Validation.Status != ValidationFailed || !hasIssue(res.Validation.Errors, "QUOTA_EXCEEDED") {
		t.Fatalf("quota not enforced: %+v", res.Validation)
	}
	env.quota.UsedVCPUs = 0
	env.capacity.AvailableVCPUs = 1
	if res = validate(t, reg, env, false, sampleSpec(uuid.New())); !hasIssue(res.Validation.Errors, "NO_CAPACITY") {
		t.Fatalf("capacity not enforced: %+v", res.Validation)
	}
}

func TestPolicyDestructiveConfirmationComesFromDatabase(t *testing.T) {
	reg := DefaultRegistry()
	env := newFakeEnv()
	id := uuid.New()
	env.instances[id] = &InstanceRef{ID: id, Name: "real-name", Status: "running"}
	spec := Specification{Resources: []ResourceSpec{{Key: "x", Type: TypeVPS, Action: ActionDelete,
		Config: map[string]any{"instance_id": id.String(), "name": "spoofed-name"}}}}
	res := validate(t, reg, env, false, spec)
	if res.Confirmation == nil || len(res.Confirmation.Expected) != 1 || res.Confirmation.Expected[0] != "real-name" {
		t.Fatalf("confirmation = %+v", res.Confirmation)
	}
	// Someone else's (unknown) instance.
	spec.Resources[0].Config["instance_id"] = uuid.NewString()
	if res = validate(t, reg, env, false, spec); !hasIssue(res.Validation.Errors, "RESOURCE_NOT_FOUND") {
		t.Fatalf("ownership not enforced: %+v", res.Validation)
	}
}

func TestPolicyAdminOnlyTool(t *testing.T) {
	reg := DefaultRegistry()
	reg.Register(ToolDef{ID: "test.admin_op", Title: "Admin op", Risk: RiskMedium, RequiresAdmin: true, InputSchema: obj(nil)})
	reg.MapAction("platform", "maintain", "test.admin_op")
	env := newFakeEnv()
	spec := Specification{Resources: []ResourceSpec{{Key: "p", Type: "platform", Action: "maintain", Config: map[string]any{}}}}
	if res := validate(t, reg, env, false, spec); !hasIssue(res.Validation.Errors, "ADMIN_REQUIRED") {
		t.Fatalf("admin-only tool allowed for a customer: %+v", res.Validation)
	}
	if res := validate(t, reg, env, true, spec); hasIssue(res.Validation.Errors, "ADMIN_REQUIRED") {
		t.Fatalf("admin-only tool refused for an admin: %+v", res.Validation)
	}
	// Execute-time check reads the role from the environment, not the JWT.
	reg.Bind("test.admin_op", RunnerFunc(func(context.Context, *TaskContext) (map[string]any, error) { return nil, nil }))
	p := NewPolicy(reg, env)
	if te := p.CheckTask(context.Background(), uuid.New(), "test.admin_op", map[string]any{}); te == nil || te.Code != "ADMIN_REQUIRED" {
		t.Fatalf("CheckTask = %v", te)
	}
	env.admin = true
	if te := p.CheckTask(context.Background(), uuid.New(), "test.admin_op", map[string]any{}); te != nil {
		t.Fatalf("CheckTask for admin = %v", te)
	}
}

func TestConfirmationMatches(t *testing.T) {
	want := []string{"web-01", "db-01"}
	for typed, ok := range map[string]bool{
		"web-01,db-01":    true,
		" db-01 , web-01": true,
		"web-01":          false,
		"WEB-01,db-01":    false,
		"web-01,db-01,x":  false,
		"":                false,
	} {
		if got := ConfirmationMatches(typed, want); got != ok {
			t.Errorf("ConfirmationMatches(%q) = %v, want %v", typed, got, ok)
		}
	}
}

func TestBackoffGrowsAndIsCapped(t *testing.T) {
	e := NewEngine(nil, nil, NewRegistry(), nil, nil, EngineConfig{RetryBase: 100 * time.Millisecond, RetryMax: time.Second})
	prev := time.Duration(0)
	for attempt := 1; attempt <= 3; attempt++ {
		d := e.Backoff(attempt)
		base := 100 * time.Millisecond << (attempt - 1)
		if d < base*8/10 || d > base*12/10 {
			t.Errorf("attempt %d: backoff %s outside [%s, %s]", attempt, d, base*8/10, base*12/10)
		}
		if d <= prev*7/10 {
			t.Errorf("backoff not growing: %s after %s", d, prev)
		}
		prev = d
	}
	if d := e.Backoff(20); d > 1200*time.Millisecond {
		t.Errorf("backoff not capped: %s", d)
	}
}

func TestVerificationSummarize(t *testing.T) {
	cases := []struct {
		checks []string
		want   string
	}{
		{[]string{"pass", "pass"}, Verified},
		{[]string{"pass", "skip"}, Unverified},
		{[]string{"skip", "fail"}, VerifyFail},
		{nil, Unverified},
	}
	for _, c := range cases {
		v := &Verification{}
		for _, s := range c.checks {
			v.Checks = append(v.Checks, Check{Name: "c", Status: s})
		}
		if got := v.Summarize().Status; got != c.want {
			t.Errorf("%v -> %s, want %s", c.checks, got, c.want)
		}
	}
}

func TestRecoveryOptions(t *testing.T) {
	tasks := []models.AIExecutionTask{{Status: models.TaskSucceeded, Risk: RiskMedium}, {Status: models.TaskFailed, Risk: RiskMedium}}
	opts := RecoveryOptions(models.ExecPartiallyCompleted, tasks, true)
	actions := []string{}
	for _, o := range opts {
		actions = append(actions, o.Action)
		if o.Action == RecoveryDeleteCreated && !o.RequiresApproval {
			t.Error("delete_created must require approval")
		}
	}
	if strings.Join(actions, ",") != "retry_failed,keep,delete_created" {
		t.Fatalf("options = %v", actions)
	}
	if len(RecoveryOptions(models.ExecCompleted, tasks, true)) != 0 {
		t.Error("a completed execution has no recovery options")
	}
	// Nothing created (or all of it deleted): no delete_created.
	for _, o := range RecoveryOptions(models.ExecPartiallyCompleted, tasks, false) {
		if o.Action == RecoveryDeleteCreated {
			t.Error("delete_created offered without created resources")
		}
	}
	if got := WithoutDeleteCreated(opts); len(got) != 2 || got[0].Action != RecoveryRetryFailed || got[1].Action != RecoveryKeep {
		t.Errorf("WithoutDeleteCreated = %v", got)
	}
	if failureStatus(tasks) != models.ExecPartiallyCompleted {
		t.Error("a succeeded write task makes the execution partially completed")
	}
	if failureStatus([]models.AIExecutionTask{{Status: models.TaskSucceeded, Risk: RiskRead}, {Status: models.TaskFailed, Risk: RiskMedium}}) != models.ExecFailed {
		t.Error("only read tasks succeeded: the execution failed")
	}
}

func TestSpecFromTool(t *testing.T) {
	reg := DefaultRegistry()
	spec, ok := reg.SpecFromTool(ToolVPSDelete, map[string]any{"instance_id": uuid.NewString(), "name": "web-01"})
	if !ok || len(spec.Resources) != 1 || spec.Resources[0].Type != TypeVPS || spec.Resources[0].Action != ActionDelete || spec.Resources[0].Key != "web-01" {
		t.Fatalf("spec = %+v", spec)
	}
	if _, ok := reg.SpecFromTool(ToolVPSList, nil); ok {
		t.Fatal("read tools are not planned")
	}
}
