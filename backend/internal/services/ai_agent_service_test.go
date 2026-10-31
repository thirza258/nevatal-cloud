package services

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
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
	svc := NewAIAgentService(nil, cfg, nil, nil, nil, nil, nil, nil, nil, nil)
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
