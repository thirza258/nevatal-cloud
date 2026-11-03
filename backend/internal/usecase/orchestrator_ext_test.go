package usecase

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
)

func TestSubstituteTemplateTypedAndEmbedded(t *testing.T) {
	spec := orchestrator.Specification{Resources: []orchestrator.ResourceSpec{{
		Key: "web", Type: orchestrator.TypeVPS, Action: orchestrator.ActionCreate,
		Config: map[string]any{
			"name":        "{{name}}",
			"vcpus":       "{{size}}",
			"memory_mb":   4096,
			"disk_gb":     40,
			"template_id": "00000000-0000-0000-0000-000000000001",
			"hostname":    "{{name}}-host",
		},
	}}}
	params := []TemplateParameter{
		{Key: "name", Type: models.TemplateParamString, Required: true},
		{Key: "size", Type: models.TemplateParamInt, Default: float64(2), Min: intPtr(1), Max: intPtr(8)},
	}
	resolved, err := substituteTemplate(spec, params, map[string]any{"name": "demo"})
	if err != nil {
		t.Fatalf("substitute: %v", err)
	}
	cfg := resolved.Resources[0].Config
	if orchestrator.String(cfg, "name") != "demo" {
		t.Fatalf("name = %v", cfg["name"])
	}
	if orchestrator.Int(cfg, "vcpus", 0) != 2 {
		t.Fatalf("vcpus = %v", cfg["vcpus"])
	}
	if orchestrator.String(cfg, "hostname") != "demo-host" {
		t.Fatalf("hostname = %v", cfg["hostname"])
	}
}

func TestSubstituteTemplateErrors(t *testing.T) {
	spec := orchestrator.Specification{Resources: []orchestrator.ResourceSpec{{
		Key: "web", Type: orchestrator.TypeVPS, Action: orchestrator.ActionCreate,
		Config: map[string]any{"name": "{{name}}", "vcpus": 1, "memory_mb": 512, "disk_gb": 10,
			"template_id": "00000000-0000-0000-0000-000000000001"},
	}}}
	params := []TemplateParameter{{Key: "name", Type: models.TemplateParamString, Required: true}}
	if _, err := substituteTemplate(spec, params, map[string]any{}); err == nil {
		t.Fatal("missing required parameter was accepted")
	}
	if _, err := substituteTemplate(spec, params, map[string]any{"name": "x", "unknown": "y"}); err == nil {
		t.Fatal("unknown parameter was accepted")
	}
	// A placeholder without a declared parameter stays unresolved and fails.
	bad := orchestrator.Specification{Resources: []orchestrator.ResourceSpec{{
		Key: "web", Type: orchestrator.TypeVPS, Action: orchestrator.ActionCreate,
		Config: map[string]any{"name": "{{missing}}", "vcpus": 1, "memory_mb": 512, "disk_gb": 10,
			"template_id": "00000000-0000-0000-0000-000000000001"},
	}}}
	if _, err := substituteTemplate(bad, params, map[string]any{"name": "x"}); err == nil {
		t.Fatal("unresolved placeholder was accepted")
	}
}

func TestRejectSecrets(t *testing.T) {
	if err := rejectSecrets(map[string]any{"password": "hunter2"}, "config"); err == nil {
		t.Fatal("a password key was accepted")
	}
	if err := rejectSecrets(map[string]any{"repository_url": "https://user:pass@github.com/x/y"}, "config"); err == nil {
		t.Fatal("credentials embedded in a URL were accepted")
	}
	if err := rejectSecrets(map[string]any{"repository_url": "https://github.com/x/y", "name": "app"}, "config"); err != nil {
		t.Fatalf("safe config rejected: %v", err)
	}
}

func TestValidateTemplateParameters(t *testing.T) {
	dup := []TemplateParameter{{Key: "name", Type: "string"}, {Key: "name", Type: "string"}}
	if err := validateTemplateParameters(dup); err == nil {
		t.Fatal("duplicate parameter keys were accepted")
	}
	badEnum := []TemplateParameter{{Key: "engine", Type: models.TemplateParamEnum}}
	if err := validateTemplateParameters(badEnum); err == nil {
		t.Fatal("enum without values was accepted")
	}
	good := []TemplateParameter{{Key: "size", Type: models.TemplateParamInt, Min: intPtr(1), Max: intPtr(4)}}
	if err := validateTemplateParameters(good); err != nil {
		t.Fatalf("valid parameters rejected: %v", err)
	}
}

func TestAdvisorPolicyEvents(t *testing.T) {
	events := advisorPolicyEvents([]orchestrator.Optimization{
		{ID: "reliability:vps:1", Category: "reliability", Title: "Failed resource needs attention",
			ResourceType: orchestrator.TypeVPS, ResourceID: "1", ResourceName: "web-01",
			Proposed: "restart", Risk: orchestrator.RiskMedium},
		{ID: "waste:vps:2", Category: "waste", Title: "Stopped resource still allocated",
			ResourceType: orchestrator.TypeVPS, ResourceID: "2", ResourceName: "web-02",
			Proposed: "delete it", Risk: orchestrator.RiskDestructive},
		{ID: "rightsize:vps:3", Category: "rightsize", Title: "Consistently idle resource",
			ResourceType: orchestrator.TypeVPS, ResourceID: "3", ResourceName: "web-03",
			Proposed: "1 vCPU, 2048 MB", Risk: orchestrator.RiskHigh},
	})
	if len(events) != 3 {
		t.Fatalf("events = %d, want 3", len(events))
	}
	for i, want := range []string{"critical", "warning", "info"} {
		if events[i].Severity != want {
			t.Errorf("events[%d].Severity = %q, want %q", i, events[i].Severity, want)
		}
		if events[i].Trigger != models.PolicyTriggerAdvisor {
			t.Errorf("events[%d].Trigger = %q", i, events[i].Trigger)
		}
		if events[i].RecommendationID == "" {
			t.Errorf("events[%d] has no recommendation id", i)
		}
	}
	if events[1].RecommendationID != "waste:vps:2" || events[1].Risk != orchestrator.RiskDestructive {
		t.Errorf("waste event = %+v", events[1])
	}
	if !strings.Contains(events[0].Message, "web-01") {
		t.Errorf("message does not name the resource: %q", events[0].Message)
	}
	if events[0].ResourceID != "1" || events[0].Category != "reliability" {
		t.Errorf("event does not carry the recommendation target: %+v", events[0])
	}
}

func TestAdvisorPolicyCannotCreatePlan(t *testing.T) {
	in := CreatePolicyInput{Name: "advisor rule", Trigger: models.PolicyTriggerAdvisor,
		Action: models.PolicyActionCreatePlan, ActionConfig: map[string]any{"template_id": uuid.New().String()}}
	if e := normalizePolicyInput(&in); e == nil {
		t.Fatal("an advisor policy with action create_plan was accepted")
	}
	in.Action = models.PolicyActionNotify
	in.ActionConfig = map[string]any{}
	if e := normalizePolicyInput(&in); e != nil {
		t.Fatalf("an advisor notify policy was rejected: %v", e)
	}
}

func TestPolicyConditionsMatch(t *testing.T) {
	values := policyConditionValues(PolicyEvent{Severity: "critical", ResourceType: "vps"})
	if !conditionsMatch(map[string]string{"severity": "critical,warning"}, values) {
		t.Fatal("comma-separated condition did not match")
	}
	if conditionsMatch(map[string]string{"resource_type": "app"}, values) {
		t.Fatal("mismatching condition matched")
	}
	if !conditionsMatch(map[string]string{"severity": "*"}, values) {
		t.Fatal("wildcard did not match")
	}
}

func TestValidateCheckCompatibility(t *testing.T) {
	in := CreateCheckInput{CheckType: models.CheckTypeInstanceHealth, ResourceType: "app", IntervalSeconds: 300}
	if e := validateCheck(&in, uuid.New()); e == nil {
		t.Fatal("incompatible check and resource type were accepted")
	}
	in = CreateCheckInput{CheckType: models.CheckTypeQuotaUsage, ResourceType: "account", IntervalSeconds: 0}
	if e := validateCheck(&in, uuid.New()); e != nil {
		t.Fatalf("account check rejected: %v", e)
	}
	if in.IntervalSeconds != 300 || in.ResourceID == nil {
		t.Fatalf("check defaults not applied: %+v", in)
	}
}

func intPtr(v int) *int { return &v }
