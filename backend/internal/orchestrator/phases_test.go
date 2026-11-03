package orchestrator

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func taskByKey(tasks []TaskSpec) map[string]TaskSpec {
	out := map[string]TaskSpec{}
	for _, t := range tasks {
		out[t.Key] = t
	}
	return out
}

func contains(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func multiServiceSpec() Specification {
	return Specification{Intent: "deploy_application", Resources: []ResourceSpec{
		{Key: "web", Type: TypeVPS, Action: ActionCreate, Config: map[string]any{
			"name": "web", "template_id": uuid.NewString(), "vcpus": 2, "memory_mb": 4096, "disk_gb": 40,
		}},
		{Key: "db", Type: TypeDatabase, Action: ActionCreate, Config: map[string]any{
			"name": "db", "engine": "postgresql", "plan_name": "starter", "vps_key": "web",
		}},
		{Key: "assets", Type: TypeBucket, Action: ActionCreate, Config: map[string]any{"name": "assets"}},
		{Key: "app", Type: TypeApp, Action: ActionCreate, Config: map[string]any{
			"name": "app", "repository_url": "https://github.com/example/app", "database_key": "db", "bucket_keys": []any{"assets"},
		}},
	}}
}

func TestCompileMultiServiceDependencies(t *testing.T) {
	tasks, err := Compile(DefaultRegistry(), multiServiceSpec())
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	byKey := taskByKey(tasks)
	db, ok := byKey["database.create:db"]
	if !ok {
		t.Fatalf("database task missing: %v", tasks)
	}
	if !contains(db.DependsOn, "vps.wait_ready:web") || !contains(db.DependsOn, "quota.check") {
		t.Fatalf("database deps = %v", db.DependsOn)
	}
	app, ok := byKey["app.create:app"]
	if !ok {
		t.Fatalf("application task missing")
	}
	if !contains(app.DependsOn, "database.create:db") || !contains(app.DependsOn, "bucket.create:assets") {
		t.Fatalf("application deps = %v", app.DependsOn)
	}
	// The VPS readiness wait must come before anything hosted on it.
	vpsIdx, dbIdx := -1, -1
	for i, task := range tasks {
		switch task.Key {
		case "vps.wait_ready:web":
			vpsIdx = i
		case "database.create:db":
			dbIdx = i
		}
	}
	if vpsIdx < 0 || dbIdx < 0 || vpsIdx > dbIdx {
		t.Fatalf("readiness wait not ordered before the hosted database: %v", tasks)
	}
	if app.Role != RoleApplication || db.Role != RoleDatabase {
		t.Fatalf("roles = %s/%s", app.Role, db.Role)
	}
}

func TestCompileRejectsUnknownAndCyclicDependencies(t *testing.T) {
	spec := Specification{Resources: []ResourceSpec{
		{Key: "app", Type: TypeApp, Action: ActionCreate, DependsOn: []string{"missing"}, Config: map[string]any{
			"name": "app", "repository_url": "https://github.com/example/app",
		}},
	}}
	if _, err := Compile(DefaultRegistry(), spec); err == nil {
		t.Fatal("unknown dependency was accepted")
	}
	cyclic := Specification{Resources: []ResourceSpec{
		{Key: "a", Type: TypeBucket, Action: ActionCreate, DependsOn: []string{"b"}, Config: map[string]any{"name": "one"}},
		{Key: "b", Type: TypeBucket, Action: ActionCreate, DependsOn: []string{"a"}, Config: map[string]any{"name": "two"}},
	}}
	if _, err := Compile(DefaultRegistry(), cyclic); err == nil {
		t.Fatal("cyclic dependency was accepted")
	}
}

func TestRoles(t *testing.T) {
	if got := RoleOf(ToolServicePower, TypeApp); got != RoleApplication {
		t.Fatalf("service.power on app role = %s", got)
	}
	if got := RoleOf(ToolServicePower, TypeDatabase); got != RoleDatabase {
		t.Fatalf("service.power on database role = %s", got)
	}
	if got := RoleForTool(ToolDomainClaim); got != RoleNetwork {
		t.Fatalf("domain.claim role = %s", got)
	}
	roles := AgentRoles(DefaultRegistry())
	found := false
	for _, r := range roles {
		if r.ID == RolePlanner && len(r.Tools) > 0 {
			found = true
		}
	}
	if !found {
		t.Fatal("planner role has no tools")
	}
}

func TestDiagnoseStoppedAndSaturated(t *testing.T) {
	now := time.Now()
	stopped := Diagnose(DiagnosticInput{
		ResourceType: TypeVPS, ResourceID: uuid.New(), Name: "web", Status: "stopped",
	}, now)
	if stopped.Health != "critical" {
		t.Fatalf("stopped health = %s", stopped.Health)
	}
	if len(stopped.Recommendations) == 0 || stopped.Recommendations[0].Action != "start" {
		t.Fatalf("recommendations = %+v", stopped.Recommendations)
	}
	busy := Diagnose(DiagnosticInput{
		ResourceType: TypeVPS, ResourceID: uuid.New(), Name: "web", Status: "running", Running: true,
		PrivateIP: "10.0.0.5", HasMetrics: true, CPUPercent: 96, MemoryPercent: 60, DiskPercent: 40,
		LastBackupAt: ptrTime(now.Add(-24 * time.Hour)),
	}, now)
	if busy.Health != "critical" {
		t.Fatalf("busy health = %s", busy.Health)
	}
	foundCPU := false
	for _, h := range busy.Hypotheses {
		if strings.Contains(strings.ToLower(h.Cause), "cpu") && h.Confidence == "high" {
			foundCPU = true
		}
	}
	if !foundCPU {
		t.Fatalf("no high-confidence CPU hypothesis: %+v", busy.Hypotheses)
	}
	healthy := Diagnose(DiagnosticInput{
		ResourceType: TypeVPS, ResourceID: uuid.New(), Name: "web", Status: "running", Running: true,
		PrivateIP: "10.0.0.5", HasMetrics: true, CPUPercent: 10, MemoryPercent: 20, DiskPercent: 20,
		LastBackupAt: ptrTime(now.Add(-time.Hour)),
	}, now)
	if healthy.Health != "healthy" {
		t.Fatalf("healthy health = %s", healthy.Health)
	}
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestAdvisorRecommendations(t *testing.T) {
	items := []AdvisorResource{
		{ResourceType: TypeVPS, ID: uuid.NewString(), Name: "old", Status: "stopped", VCPUs: 2, MemoryMB: 2048, DiskGB: 40, AgeDays: 20},
		{ResourceType: TypeVPS, ID: uuid.NewString(), Name: "idle", Status: "running", VCPUs: 4, MemoryMB: 4096, DiskGB: 80,
			CPUPercent: 4, MemoryPercent: 20, DiskPercent: 30, HasMetrics: true, AgeDays: 10},
		{ResourceType: TypeBucket, ID: uuid.NewString(), Name: "empty", Status: "active", Objects: 0, SizeBytes: 0, AgeDays: 30},
	}
	recs := GenerateOptimizations(items)
	if len(recs) != 3 {
		t.Fatalf("recommendations = %+v", recs)
	}
	if recs[0].Category != "waste" || !recs[0].RequiresApproval {
		t.Fatalf("first recommendation = %+v", recs[0])
	}
	foundRightsize := false
	for _, r := range recs {
		if r.Category == "rightsize" && r.ResourceName == "idle" {
			foundRightsize = true
		}
	}
	if !foundRightsize {
		t.Fatalf("no rightsize recommendation: %+v", recs)
	}
}

func TestEstimateCosts(t *testing.T) {
	report := EstimateCosts([]CostItem{
		{ResourceType: TypeVPS, Name: "web", VCPUs: 2, MemoryMB: 4096, DiskGB: 40, Status: "running"},
		{ResourceType: TypeVPS, Name: "idle", VCPUs: 2, MemoryMB: 4096, DiskGB: 40, Status: "stopped"},
		{ResourceType: TypeBucket, Name: "assets", SizeBytes: 2 * 1024 * 1024 * 1024, Status: "active"},
	}, CostRate{Currency: "USD", VCPUPerMonth: 5, MemoryGBPerMonth: 2, DiskGBPerMonth: 0.1, BucketGBPerMonth: 0.5})
	// Running: 10 + 8 + 4 = 22. Stopped: 4 (disk only). Bucket: 2 * 0.5 = 1.
	if report.MonthlyTotal != 27 {
		t.Fatalf("total = %v (%+v)", report.MonthlyTotal, report.Resources)
	}
}
