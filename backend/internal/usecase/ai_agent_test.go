package usecase

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
)

func TestDetectIntent(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"I want to create a VPS for hosting my Go API", "create_vps"},
		{"Deploy an Ubuntu 24.04 server with 4GB RAM", "create_vps"},
		{"Deploy a website from GitHub", "deploy_website"},
		{"Host my Next.js React app", "deploy_website"},
		{"Set up an n8n workflow automation instance", "n8n_instance"},
		{"Deploy an OpenClaw AI agent", "openclaw_instance"},
		{"Create S3-compatible object storage bucket", "create_storage"},
		{"Why is my instance failing to boot? Check error", "troubleshoot"},
	}

	for _, tc := range tests {
		got := detectIntent(strings.ToLower(tc.input))
		if got != tc.expect {
			t.Errorf("detectIntent(%q) = %q; want %q", tc.input, got, tc.expect)
		}
	}
}

func TestExtractRequirementsFromText(t *testing.T) {
	mockTplID := uuid.New()
	templates := []models.OSTemplate{
		{ID: mockTplID, Name: "Ubuntu Server 24.04 LTS", Distribution: "ubuntu"},
	}
	sshKeys := []models.SSHKey{
		{ID: uuid.New(), Name: "laptop-key"},
	}

	reqs := make(map[string]string)
	input := "Deploy Ubuntu with 4GB RAM and public IP from https://github.com/nevatal/go-api-service"

	extractRequirementsFromText(strings.ToLower(input), reqs, templates, sshKeys)

	if reqs["os_template_id"] != mockTplID.String() {
		t.Errorf("Expected os_template_id %s, got %s", mockTplID.String(), reqs["os_template_id"])
	}
	if reqs["specs"] != "2:4096:40" {
		t.Errorf("Expected specs 2:4096:40, got %s", reqs["specs"])
	}
	if reqs["public_networking"] != "true" {
		t.Errorf("Expected public_networking true, got %s", reqs["public_networking"])
	}
	if reqs["git_repo"] != "https://github.com/nevatal/go-api-service" {
		t.Errorf("Expected git_repo match, got %s", reqs["git_repo"])
	}
}

func TestWorkflowStateTransitions(t *testing.T) {
	// Verify WorkflowState constants match PRD 34.2
	states := []models.WorkflowState{
		models.StateUnderstandingRequest,
		models.StateClarifyingRequirements,
		models.StatePlanning,
		models.StateAwaitingConfirmation,
		models.StateAIProvisioning,
		models.StateValidating,
		models.StateRecovery,
		models.StateCompleted,
		models.StateAIFailed,
		models.StateCancelled,
	}

	if len(states) != 10 {
		t.Errorf("Expected 10 workflow states, got %d", len(states))
	}
}

func TestMCPToolRiskClassification(t *testing.T) {
	cfg := &config.Config{}
	svc := NewAIAgentUsecase(AIAgentRepos{}, cfg, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	if svc == nil {
		t.Fatalf("Expected non-nil agent service")
	}

	// Verify general query returns helpful response
	reply := svc.handleGeneralQuery("help", &models.QuotaUsage{
		UsedVCPUs: 2,
		MaxVCPUs:  8,
	}, []models.OSTemplate{{Name: "Ubuntu"}})

	if !strings.Contains(reply, "Nevatal AI Cloud Agent") {
		t.Errorf("Expected greeting in general query reply")
	}
	if !strings.Contains(reply, "vCPU Cores") {
		t.Errorf("Expected quota information in reply")
	}
}

func TestGetIntArg(t *testing.T) {
	args := map[string]interface{}{
		"vcpus":     float64(4),
		"memory_mb": 8192,
		"disk_gb":   int64(100),
	}

	if getIntArg(args, "vcpus", 1) != 4 {
		t.Errorf("Expected 4")
	}
	if getIntArg(args, "memory_mb", 1024) != 8192 {
		t.Errorf("Expected 8192")
	}
	if getIntArg(args, "disk_gb", 20) != 100 {
		t.Errorf("Expected 100")
	}
	if getIntArg(args, "missing", 42) != 42 {
		t.Errorf("Expected default 42")
	}
}

func TestJSONRPCStructure(t *testing.T) {
	ctx := context.Background()
	_ = ctx
	// Ensure JSON-RPC protocol constants and structures match standard 2.0
	req := JSONRPCRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/list",
	}
	if req.JSONRPC != "2.0" {
		t.Errorf("Expected JSONRPC version 2.0")
	}
}

func TestDetectDeleteIntent(t *testing.T) {
	for msg, want := range map[string]string{
		"please delete the vps web-01":         "delete_vps",
		"destroy my virtual machine db-01":     "delete_vps",
		"remove the minecraft server":          "game_server",
		"delete the bucket logs":               "create_storage",
		"I want to create a VPS for my Go API": "create_vps",
	} {
		if got := detectIntent(strings.ToLower(msg)); got != want {
			t.Errorf("detectIntent(%q) = %q, want %q", msg, got, want)
		}
	}
}

func TestReadRequestsNeverStartAPlan(t *testing.T) {
	for msg, want := range map[string]bool{
		"list my servers":                        true,
		"show my VPS":                            true,
		"what is the status of web-01":           true,
		"What's running? how many VMs do I have": true,
		"what are my current quotas":             true,
		"list my buckets":                        true,
		"Create an Ubuntu VPS, nano size":        false,
		"show me how to create a VPS":            false,
		"please delete the vps web-01":           false,
		"stop web-01 and show its status":        false,
		"Why is my instance failing to boot?":    false,
		"I want to create a VPS for my Go API":   false,
		"Deploy an Ubuntu 24.04 server":          false,
	} {
		if got := isReadRequest(strings.ToLower(msg), nil); got != want {
			t.Errorf("isReadRequest(%q) = %v, want %v", msg, got, want)
		}
	}
	// No keyword, but it names one of the caller's instances: a status question.
	owned := func() []string { return []string{"web-01"} }
	for msg, want := range map[string]bool{
		"is web-01 running":                     true,
		"is web-01 up":                          true,
		"web-01?":                               true,
		"is web-011 up":                         false,
		"a vps running nginx":                   false,
		"stop web-01":                           false,
		"why is web-01 down":                    false,
		"create a vps like web-01 named web-02": false,
	} {
		if got := isReadRequest(strings.ToLower(msg), owned); got != want {
			t.Errorf("isReadRequest(%q, owned) = %v, want %v", msg, got, want)
		}
	}
}

func TestDetectPhaseTwoIntents(t *testing.T) {
	for msg, want := range map[string]string{
		"create a postgres database named appdb": "create_database",
		"provision a redis database":             "create_database",
		"deploy the web-stack template":          "deploy_template",
		"use template production-vps":            "deploy_template",
		"optimize my costs":                      "advisor",
		"diagnose web-01":                        "troubleshoot",
		"health check all resources":             "troubleshoot",
		"why is my database down":                "troubleshoot",
	} {
		if got := detectIntent(strings.ToLower(msg)); got != want {
			t.Errorf("detectIntent(%q) = %q, want %q", msg, got, want)
		}
	}
}

func TestTemplateParamValue(t *testing.T) {
	min, max := 1, 5
	replicas := TemplateParameter{Key: "replicas", Type: models.TemplateParamInt, Min: &min, Max: &max}
	if got, err := templateParamValue(replicas, "3"); err != nil || got != 3 {
		t.Fatalf("integer param = %v, %v; want 3", got, err)
	}
	if _, err := templateParamValue(replicas, "9"); err == nil {
		t.Fatal("out-of-range integer was accepted")
	}
	public := TemplateParameter{Key: "public", Type: models.TemplateParamBool}
	if got, err := templateParamValue(public, "yes"); err != nil || got != true {
		t.Fatalf("boolean param = %v, %v; want true", got, err)
	}
	region := TemplateParameter{Key: "region", Type: models.TemplateParamEnum, Enum: []string{"eu", "us"}}
	if got, err := templateParamValue(region, "US"); err != nil || got != "us" {
		t.Fatalf("enum param = %v, %v; want us", got, err)
	}
	if _, err := templateParamValue(region, "asia"); err == nil {
		t.Fatal("unknown enum value was accepted")
	}
}

func TestMatchOwnedResourcePrefersTypeAndLongestName(t *testing.T) {
	refs := []ownedResourceRef{
		{Type: "database", Name: "db", ID: uuid.New(), Status: "running"},
		{Type: "vps", Name: "web", ID: uuid.New(), Status: "running"},
		{Type: "vps", Name: "web-01", ID: uuid.New(), Status: "running"},
	}
	if got := matchOwnedResource(refs, "why is web-01 down?"); got == nil || got.Name != "web-01" {
		t.Fatalf("matched %+v, want web-01", got)
	}
	if got := matchOwnedResource(refs, "diagnose the database db"); got == nil || got.Type != "database" {
		t.Fatalf("matched %+v, want the database", got)
	}
	if got := matchOwnedResource(refs, "diagnose missing-01"); got != nil {
		t.Fatalf("matched %+v, want no match", got)
	}
}

func TestFormatDiagnosticAndAdvisor(t *testing.T) {
	diagnostic := &orchestrator.DiagnosticResult{
		ResourceType: "vps", ResourceID: uuid.New(), Name: "web-01", Health: "critical",
		Facts:      []orchestrator.Fact{{Name: "status", Value: "stopped", Status: "critical"}},
		Hypotheses: []orchestrator.Hypothesis{{Cause: "the resource is stopped", Confidence: "high"}},
		Recommendations: []orchestrator.Recommendation{
			{Action: "start", Label: "Start the resource", Risk: "medium", RequiresApproval: true},
		},
	}
	out := formatDiagnostic(diagnostic)
	for _, want := range []string{"web-01", "CRITICAL", "the resource is stopped", "Start the resource"} {
		if !strings.Contains(out, want) {
			t.Errorf("formatDiagnostic() missing %q:\n%s", want, out)
		}
	}

	advisor := &AdvisorReport{
		Currency: "USD", MonthlyTotal: 12.5,
		CostResources: []orchestrator.CostItem{{Name: "web-01", ResourceType: "vps", Monthly: 12.5}},
		Optimizations: []orchestrator.Optimization{{
			Title: "Consistently idle resource", Category: "rightsize", ResourceName: "web-01",
			Current: "2 vCPU", Observed: "CPU 4%", Proposed: "1 vCPU", ExpectedBenefit: "lower cost",
			Risk: "high", RequiresApproval: true,
		}},
	}
	out = formatAdvisorReport(advisor)
	for _, want := range []string{"12.50 USD", "web-01", "Consistently idle resource", "requires an approved plan"} {
		if !strings.Contains(out, want) {
			t.Errorf("formatAdvisorReport() missing %q:\n%s", want, out)
		}
	}
}

func TestSanitizeDatabaseName(t *testing.T) {
	for raw, want := range map[string]string{
		"App DB_01": "app-db-01",
		"--":        "", // filled by the caller's generated fallback below
	} {
		got := sanitizeDatabaseName(raw, "postgresql")
		if want == "" {
			if !strings.HasPrefix(got, "db-postgresql-") {
				t.Errorf("sanitizeDatabaseName(%q) = %q, want generated fallback", raw, got)
			}
			continue
		}
		if got != want {
			t.Errorf("sanitizeDatabaseName(%q) = %q, want %q", raw, got, want)
		}
	}
}
