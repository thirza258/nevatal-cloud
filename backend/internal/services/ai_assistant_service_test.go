package services

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
)

func TestExtractActionProposal(t *testing.T) {
	cfg := &config.Config{
		OpenRouterModel: "openai/gpt-4o-mini",
	}
	svc := NewAIAssistantService(nil, cfg, nil, nil, nil)

	mockTplID := uuid.New()
	pCtx := &AIPlatformContext{
		User: models.User{
			ID:          uuid.New(),
			DisplayName: "Test User",
			Role:        "customer",
		},
		Templates: []models.OSTemplate{
			{
				ID:   mockTplID,
				Name: "Ubuntu Server 24.04 LTS (Noble Numbat)",
			},
		},
	}

	rawText := `I have designed an optimal Ubuntu container for your needs.

` + "```json:action\n" +
		`{
  "action": "create_instance",
  "title": "Deploy Ubuntu Server",
  "description": "2 vCPUs, 4GB RAM",
  "params": {
    "name": "web-prod-01",
    "hostname": "web-prod-01",
    "template_id": "` + mockTplID.String() + `",
    "vcpus": 2,
    "memory_mb": 4096,
    "disk_gb": 40
  }
}
` + "```\n" +
		`Click the button below to provision your container immediately!`

	cleaned, proposal := svc.extractActionProposal(rawText, pCtx)

	if proposal == nil {
		t.Fatalf("Expected non-nil action proposal")
	}

	if proposal.Action != "create_instance" {
		t.Errorf("Expected action create_instance, got %s", proposal.Action)
	}

	params, ok := proposal.Params.(models.CreateInstanceActionParams)
	if !ok {
		t.Fatalf("Expected Params to be CreateInstanceActionParams")
	}

	if params.Name != "web-prod-01" {
		t.Errorf("Expected name web-prod-01, got %s", params.Name)
	}

	if params.VCPUs != 2 || params.MemoryMB != 4096 || params.DiskGB != 40 {
		t.Errorf("Unexpected specs: %+v", params)
	}

	if strings.Contains(cleaned, "```json:action") {
		t.Errorf("Expected action block to be stripped from cleaned text")
	}
}

func TestGenerateLocalFallback(t *testing.T) {
	cfg := &config.Config{}
	svc := NewAIAssistantService(nil, cfg, nil, nil, nil)

	mockTplID := uuid.New()
	pCtx := &AIPlatformContext{
		User: models.User{
			ID:          uuid.New(),
			DisplayName: "Dev User",
			Role:        "customer",
		},
		QuotaUsage: &models.QuotaUsage{
			MaxVCPUs:      8,
			UsedVCPUs:     2,
			MaxMemoryMB:   16384,
			UsedMemoryMB:  4096,
			MaxStorageGB:  200,
			UsedStorageGB: 50,
			MaxInstances:  5,
			UsedInstances: 1,
		},
		Templates: []models.OSTemplate{
			{
				ID:   mockTplID,
				Name: "Ubuntu Server 24.04 LTS",
			},
		},
	}

	// 1. Test Container Creation Intent
	reply, proposal := svc.generateLocalFallback(pCtx, "Please create an Ubuntu container with 2 vCPUs")
	if proposal == nil {
		t.Fatalf("Expected create container proposal")
	}
	if proposal.Action != "create_instance" {
		t.Errorf("Expected action create_instance, got %s", proposal.Action)
	}
	if !strings.Contains(reply, "Container Provisioning Recommendation") {
		t.Errorf("Expected container recommendation in reply")
	}

	// 2. Test SSH Support Intent
	replySSH, proposalSSH := svc.generateLocalFallback(pCtx, "How do I connect to my VM via ssh?")
	if proposalSSH != nil {
		t.Errorf("Expected nil proposal for support query")
	}
	if !strings.Contains(replySSH, "How to Connect to Your Nevatal Cloud Instances") {
		t.Errorf("Expected SSH connect instructions in reply, got: %s", replySSH)
	}

	// 3. Test Quota Query Intent
	replyQuota, _ := svc.generateLocalFallback(pCtx, "Show my resource quotas and usage")
	if !strings.Contains(replyQuota, "Your Current Resource Quotas & Usage") {
		t.Errorf("Expected quota table in reply")
	}

	// 4. Test Proxmox Hardware Query
	replyHardware, _ := svc.generateLocalFallback(pCtx, "What are the Dell R640 Proxmox specs?")
	if !strings.Contains(replyHardware, "Dell PowerEdge R640") {
		t.Errorf("Expected Dell PowerEdge R640 specs in reply")
	}
}

func TestRecommendedOpenRouterModels(t *testing.T) {
	if len(RecommendedOpenRouterModels) < 4 {
		t.Errorf("Expected at least 4 recommended OpenRouter models, got %d", len(RecommendedOpenRouterModels))
	}

	foundDeepSeek := false
	foundClaude := false
	foundGPT4o := false

	for _, m := range RecommendedOpenRouterModels {
		if strings.Contains(m.ID, "deepseek") {
			foundDeepSeek = true
		}
		if strings.Contains(m.ID, "claude") {
			foundClaude = true
		}
		if strings.Contains(m.ID, "gpt-4o") {
			foundGPT4o = true
		}
	}

	if !foundDeepSeek || !foundClaude || !foundGPT4o {
		t.Errorf("Expected DeepSeek, Claude, and GPT-4o in recommended models")
	}
}
