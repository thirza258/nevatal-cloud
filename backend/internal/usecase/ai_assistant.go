package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// AIAssistantRepos are the repositories AIAssistantUsecase reads and writes.
type AIAssistantRepos struct {
	Messages  repository.AIAssistantRepository
	Users     repository.UserRepository
	Templates repository.TemplateRepository
	Plans     repository.PlanRepository
	SSHKeys   repository.SSHKeyRepository
}

type AIAssistantUsecase struct {
	repos           AIAssistantRepos
	cfg             *config.Config
	instanceService *InstanceUsecase
	quotaService    *QuotaUsecase
	auditService    *AuditUsecase
	httpClient      *http.Client
}

func NewAIAssistantUsecase(
	repos AIAssistantRepos,
	cfg *config.Config,
	instanceService *InstanceUsecase,
	quotaService *QuotaUsecase,
	auditService *AuditUsecase,
) *AIAssistantUsecase {
	return &AIAssistantUsecase{
		repos:           repos,
		cfg:             cfg,
		instanceService: instanceService,
		quotaService:    quotaService,
		auditService:    auditService,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

type AIPlatformContext struct {
	User            models.User                 `json:"user"`
	QuotaUsage      *models.QuotaUsage          `json:"quota_usage"`
	Instances       []models.Instance           `json:"instances"`
	Templates       []models.OSTemplate         `json:"templates"`
	Plans           []models.ResourcePlan       `json:"plans"`
	SSHKeys         []models.SSHKey             `json:"ssh_keys"`
	HasServerAPIKey bool                        `json:"has_server_api_key"`
	DefaultModel    string                      `json:"default_model"`
	AvailableModels []OpenRouterModelDefinition `json:"available_models"`
}

type OpenRouterModelDefinition struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Provider    string `json:"provider"`
	Description string `json:"description"`
	ContextLen  int    `json:"context_len"`
}

var RecommendedOpenRouterModels = []OpenRouterModelDefinition{
	{
		ID:          "deepseek/deepseek-chat",
		Name:        "DeepSeek V3 (Reasoning & Code)",
		Provider:    "DeepSeek",
		Description: "Fast, intelligent, highly capable for cloud DevOps automation and troubleshooting.",
		ContextLen:  64000,
	},
	{
		ID:          "anthropic/claude-3.5-sonnet",
		Name:        "Claude 3.5 Sonnet",
		Provider:    "Anthropic",
		Description: "Industry benchmark for infrastructure reasoning, coding, and technical support.",
		ContextLen:  200000,
	},
	{
		ID:          "openai/gpt-4o-mini",
		Name:        "GPT-4o Mini",
		Provider:    "OpenAI",
		Description: "Affordable, snappy, excellent for quick cloud assistance and container management.",
		ContextLen:  128000,
	},
	{
		ID:          "openai/gpt-4o",
		Name:        "GPT-4o",
		Provider:    "OpenAI",
		Description: "Flagship multimodal intelligence for mission-critical troubleshooting and architecture.",
		ContextLen:  128000,
	},
	{
		ID:          "meta-llama/llama-3.3-70b-instruct",
		Name:        "Llama 3.3 70B Instruct",
		Provider:    "Meta",
		Description: "Top-tier open-weights model with excellent system administration skills.",
		ContextLen:  128000,
	},
	{
		ID:          "google/gemini-2.0-flash-exp:free",
		Name:        "Gemini 2.0 Flash (Free)",
		Provider:    "Google",
		Description: "Ultra-fast response latency with expansive context understanding.",
		ContextLen:  1048576,
	},
}

type ChatRequestInput struct {
	Message    string `json:"message"`
	Model      string `json:"model"`
	APIKey     string `json:"api_key"`
	ActionOnly bool   `json:"action_only"`
}

type ChatResponse struct {
	Message        models.AIAssistantMessage         `json:"message"`
	ActionProposal *models.AIAssistantActionProposal `json:"action_proposal,omitempty"`
	UsedFallback   bool                              `json:"used_fallback"`
	ErrorNotice    string                            `json:"error_notice,omitempty"`
}

type ExecuteActionInput struct {
	MessageID uuid.UUID       `json:"message_id"`
	Action    string          `json:"action"`
	Params    json.RawMessage `json:"params"`
}

// ExecuteActionResult is the outcome of an executed create_instance proposal.
type ExecuteActionResult struct {
	InstanceID *uuid.UUID `json:"instance_id"`
	JobID      uuid.UUID  `json:"job_id"`
	Message    string     `json:"message"`
	Name       string     `json:"name"`
	Status     string     `json:"status"`
}

func (s *AIAssistantUsecase) GetPlatformContext(ctx context.Context, userID uuid.UUID) (*AIPlatformContext, error) {
	user, err := s.repos.Users.FindByID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}

	quotaUsage, err := s.quotaService.GetUsage(ctx, userID)
	if err != nil {
		log.Printf("Warning: failed to load quota usage: %v", err)
	}

	instances, err := s.instanceService.List(ctx, userID, user.Role == "admin", "")
	if err != nil {
		log.Printf("Warning: failed to load instances: %v", err)
	}

	templates, _ := s.repos.Templates.ListEnabledByName(ctx)
	plans, _ := s.repos.Plans.ListEnabledBySortOrder(ctx)
	sshKeys, _ := s.repos.SSHKeys.ListByOwnerByName(ctx, userID)

	defaultModel := s.cfg.OpenRouterModel
	if defaultModel == "" {
		defaultModel = "openai/gpt-4o-mini"
	}

	return &AIPlatformContext{
		User:            *user,
		QuotaUsage:      quotaUsage,
		Instances:       instances,
		Templates:       templates,
		Plans:           plans,
		SSHKeys:         sshKeys,
		HasServerAPIKey: s.cfg.OpenRouterAPIKey != "",
		DefaultModel:    defaultModel,
		AvailableModels: RecommendedOpenRouterModels,
	}, nil
}

func (s *AIAssistantUsecase) ListHistory(ctx context.Context, userID uuid.UUID, limit int) ([]models.AIAssistantMessage, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.repos.Messages.ListHistory(ctx, userID, limit)
}

func (s *AIAssistantUsecase) ClearHistory(ctx context.Context, userID uuid.UUID) error {
	return s.repos.Messages.DeleteHistory(context.WithoutCancel(ctx), userID)
}

type openRouterMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openRouterChatRequest struct {
	Model       string              `json:"model"`
	Messages    []openRouterMessage `json:"messages"`
	Temperature float32             `json:"temperature"`
	MaxTokens   int                 `json:"max_tokens,omitempty"`
}

type openRouterChatResponse struct {
	ID      string `json:"id"`
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (s *AIAssistantUsecase) Chat(
	ctx context.Context,
	userID uuid.UUID,
	input ChatRequestInput,
	clientIP string,
) (*ChatResponse, error) {
	pCtx, err := s.GetPlatformContext(ctx, userID)
	if err != nil {
		return nil, err
	}

	// Messages are persisted even if the client disconnects mid-request.
	dbCtx := context.WithoutCancel(ctx)

	// 1. Save user message to database
	userMsg := models.AIAssistantMessage{
		UserID:    userID,
		Role:      "user",
		Content:   input.Message,
		Model:     input.Model,
		CreatedAt: time.Now(),
	}
	if err := s.repos.Messages.Create(dbCtx, &userMsg); err != nil {
		log.Printf("Warning: failed to save user chat message: %v", err)
	}

	// 2. Select model and API key
	activeModel := input.Model
	if strings.TrimSpace(activeModel) == "" {
		activeModel = pCtx.DefaultModel
	}

	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" {
		apiKey = strings.TrimSpace(s.cfg.OpenRouterAPIKey)
	}

	// 3. Retrieve recent history for context
	history, _ := s.repos.Messages.ListRecent(dbCtx, userID, 8)

	// Reverse history so oldest is first
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	// 4. Try calling OpenRouter if API key is present
	var assistantReply string
	var actionProposal *models.AIAssistantActionProposal
	var usedFallback bool
	var errorNotice string

	if apiKey != "" {
		reply, action, callErr := s.callOpenRouter(ctx, apiKey, activeModel, pCtx, history, input.Message)
		if callErr == nil && reply != "" {
			assistantReply = reply
			actionProposal = action
		} else {
			log.Printf("OpenRouter API call failed or returned error: %v. Falling back to local assistant engine.", callErr)
			errorNotice = fmt.Sprintf("OpenRouter Notice: %v. Activated local Nevatal Cloud AI Copilot fallback.", callErr)
			assistantReply, actionProposal = s.generateLocalFallback(pCtx, input.Message)
			usedFallback = true
		}
	} else {
		// No OpenRouter API key provided - use local fallback engine
		assistantReply, actionProposal = s.generateLocalFallback(pCtx, input.Message)
		usedFallback = true
		errorNotice = "No OpenRouter API key provided. Operating in high-speed local AI DevOps mode. You can supply an OpenRouter API key anytime to unlock Claude, GPT-4o, or DeepSeek models."
	}

	// 5. Save assistant message to database
	var actionPayloadStr string
	var actionTypeStr string
	if actionProposal != nil {
		actionTypeStr = actionProposal.Action
		if bytes, mErr := json.Marshal(actionProposal.Params); mErr == nil {
			actionPayloadStr = string(bytes)
		}
	}

	assistantMsg := models.AIAssistantMessage{
		UserID:         userID,
		Role:           "assistant",
		Content:        assistantReply,
		Model:          activeModel,
		ActionType:     actionTypeStr,
		ActionPayload:  actionPayloadStr,
		ActionExecuted: false,
		CreatedAt:      time.Now(),
	}
	if err := s.repos.Messages.Create(dbCtx, &assistantMsg); err != nil {
		log.Printf("Warning: failed to save assistant message: %v", err)
	}

	return &ChatResponse{
		Message:        assistantMsg,
		ActionProposal: actionProposal,
		UsedFallback:   usedFallback,
		ErrorNotice:    errorNotice,
	}, nil
}

func (s *AIAssistantUsecase) callOpenRouter(
	ctx context.Context,
	apiKey string,
	model string,
	pCtx *AIPlatformContext,
	history []models.AIAssistantMessage,
	latestMessage string,
) (string, *models.AIAssistantActionProposal, error) {
	// Build system prompt
	systemPrompt := s.buildSystemPrompt(pCtx)

	var messages []openRouterMessage
	messages = append(messages, openRouterMessage{
		Role:    "system",
		Content: systemPrompt,
	})

	for _, h := range history {
		if h.Content == latestMessage && h.Role == "user" {
			continue // skip duplicate latest message
		}
		messages = append(messages, openRouterMessage{
			Role:    h.Role,
			Content: h.Content,
		})
	}

	messages = append(messages, openRouterMessage{
		Role:    "user",
		Content: latestMessage,
	})

	reqBody := openRouterChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: 0.3,
		MaxTokens:   2000,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", nil, err
	}

	baseURL := s.cfg.OpenRouterBaseURL
	if baseURL == "" {
		baseURL = "https://openrouter.ai/api/v1"
	}
	endpoint := fmt.Sprintf("%s/chat/completions", strings.TrimRight(baseURL, "/"))

	req, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", s.cfg.AppBaseURL)
	req.Header.Set("X-Title", "Nevatal Cloud AI DevOps Assistant")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return "", nil, fmt.Errorf("network error connecting to OpenRouter: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("failed to read OpenRouter response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp openRouterChatResponse
		_ = json.Unmarshal(bodyBytes, &errResp)
		if errResp.Error != nil && errResp.Error.Message != "" {
			return "", nil, fmt.Errorf("OpenRouter API error (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return "", nil, fmt.Errorf("OpenRouter returned HTTP %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var chatResp openRouterChatResponse
	if err := json.Unmarshal(bodyBytes, &chatResp); err != nil {
		return "", nil, fmt.Errorf("invalid JSON response from OpenRouter: %w", err)
	}

	if len(chatResp.Choices) == 0 {
		return "", nil, errors.New("OpenRouter returned empty choices list")
	}

	rawContent := chatResp.Choices[0].Message.Content

	// Parse out any action block
	cleanedText, action := s.extractActionProposal(rawContent, pCtx)
	return cleanedText, action, nil
}

func (s *AIAssistantUsecase) buildSystemPrompt(pCtx *AIPlatformContext) string {
	var tplSummary []string
	for _, t := range pCtx.Templates {
		tplSummary = append(tplSummary, fmt.Sprintf("- ID: %s | Name: %s (%s, user: %s)", t.ID, t.Name, t.Version, t.DefaultUsername))
	}

	var planSummary []string
	for _, p := range pCtx.Plans {
		planSummary = append(planSummary, fmt.Sprintf("- Plan '%s': %d vCPU, %d MB RAM, %d GB SSD", p.Name, p.VCPUs, p.MemoryMB, p.DiskGB))
	}

	var instSummary []string
	for _, inst := range pCtx.Instances {
		ip := "allocating"
		if inst.PublicIP != nil && *inst.PublicIP != "" {
			ip = *inst.PublicIP
		} else if inst.PrivateIP != nil && *inst.PrivateIP != "" {
			ip = *inst.PrivateIP
		}
		instSummary = append(instSummary, fmt.Sprintf("- Name: %s | Status: %s | IP: %s | Specs: %d vCPU / %d MB RAM / %d GB SSD",
			inst.Name, inst.Status, ip, inst.VCPUs, inst.MemoryMB, inst.DiskGB))
	}

	var keySummary []string
	for _, k := range pCtx.SSHKeys {
		keySummary = append(keySummary, fmt.Sprintf("- Key: %s (ID: %s, %s)", k.Name, k.ID, k.Fingerprint))
	}

	remainingVCPUs := 0
	remainingMemoryMB := 0
	remainingStorageGB := 0
	remainingInstances := 0
	if pCtx.QuotaUsage != nil {
		remainingVCPUs = pCtx.QuotaUsage.MaxVCPUs - pCtx.QuotaUsage.UsedVCPUs
		remainingMemoryMB = pCtx.QuotaUsage.MaxMemoryMB - pCtx.QuotaUsage.UsedMemoryMB
		remainingStorageGB = pCtx.QuotaUsage.MaxStorageGB - pCtx.QuotaUsage.UsedStorageGB
		remainingInstances = pCtx.QuotaUsage.MaxInstances - pCtx.QuotaUsage.UsedInstances
	}

	prompt := fmt.Sprintf(`You are the Nevatal Cloud AI DevOps Assistant & Technical Support Copilot.
Nevatal Cloud is an enterprise self-hosted cloud platform orchestrating virtual servers and Linux containers on a Dell PowerEdge R640 running Proxmox VE with local-zfs storage and cloud-init automation.

Current User Profile:
- Name: %s (%s)
- Role: %s

User Quota Status:
- vCPUs: %d used / %d max (Remaining: %d vCPUs)
- Memory: %d MB used / %d MB max (Remaining: %d MB RAM)
- Storage: %d GB used / %d GB max (Remaining: %d GB SSD)
- Containers/VMs: %d active / %d max (Remaining: %d slots)

Available Operating System Templates:
%s

Available Standard Plans:
%s

User SSH Keys:
%s

User Active Instances:
%s

CORE INSTRUCTIONS:
1. Container & VM Creation Assistance ("Create the container etc"):
   When the user asks to create, launch, deploy, or configure a container, Linux VM, or web application:
   - Help them select the best OS template (Ubuntu 24.04, Debian 12, Alpine 3.20, WordPress 6.7).
   - Check their remaining quotas to ensure the requested specs fit.
   - Propose optimal specs (vCPU, RAM, Disk, Hostname).
   - Whenever proposing or agreeing to create an instance, YOU MUST append a machine-readable JSON action proposal at the very end of your response inside triple backticks with tag 'json:action'.
   Format:
   `+"```json:action"+`
   {
     "action": "create_instance",
     "title": "Deploy Ubuntu 24.04 LTS Container",
     "description": "2 vCPUs, 4096 MB RAM, 40 GB SSD",
     "params": {
       "name": "web-server-01",
       "hostname": "web-server-01",
       "template_id": "<EXACT_TEMPLATE_UUID>",
       "template_name": "Ubuntu Server 24.04 LTS (Noble Numbat)",
       "vcpus": 2,
       "memory_mb": 4096,
       "disk_gb": 40,
       "ssh_key_id": "<EXACT_SSH_KEY_UUID_OR_EMPTY>"
     }
   }
   `+"```"+`
   Rules for action params:
   - 'name': 3-64 characters, lowercase alphanumeric and hyphens only, starting and ending with alphanumeric.
   - 'template_id': Must be an EXACT UUID from the Available Operating System Templates list above.
   - 'vcpus': integer >= 1.
   - 'memory_mb': integer >= 512.
   - 'disk_gb': integer >= 10.
   - 'ssh_key_id': UUID of the user's SSH key if one exists, or empty string.

2. Cloud Support & Diagnostics ("or as support"):
   When the user asks questions or encounters problems:
   - Provide direct, concise, high-quality technical answers.
   - For SSH connectivity: Explain using 'ssh <username>@<ip>' with the registered SSH key, checking instance status and security groups.
   - For Proxmox/Hypervisor questions: Explain that instances run as QEMU/KVM virtual machines and containers on Dell PowerEdge R640 with ZFS RAID storage and automated cloud-init provisioning.
   - For quotas: Explain limits and suggest right-sizing if they are near limits.
   - For Object Storage / S3: Explain MinIO-compatible S3 buckets and pre-signed shared links.
   - For Automations / n8n / OpenClaw: Explain workflow triggers and personal AI agents.
   - Keep answers professional, friendly, and formatted in clean GitHub Markdown with code snippets.`,
		pCtx.User.DisplayName, pCtx.User.Email, pCtx.User.Role,
		pCtx.QuotaUsage.UsedVCPUs, pCtx.QuotaUsage.MaxVCPUs, remainingVCPUs,
		pCtx.QuotaUsage.UsedMemoryMB, pCtx.QuotaUsage.MaxMemoryMB, remainingMemoryMB,
		pCtx.QuotaUsage.UsedStorageGB, pCtx.QuotaUsage.MaxStorageGB, remainingStorageGB,
		pCtx.QuotaUsage.UsedInstances, pCtx.QuotaUsage.MaxInstances, remainingInstances,
		strings.Join(tplSummary, "\n"),
		strings.Join(planSummary, "\n"),
		strings.Join(keySummary, "\n"),
		strings.Join(instSummary, "\n"),
	)

	return prompt
}

var actionBlockRegex = regexp.MustCompile("(?s)```json:action\\s*(\\{.*?\\})\\s*```")

func (s *AIAssistantUsecase) extractActionProposal(
	content string,
	pCtx *AIPlatformContext,
) (string, *models.AIAssistantActionProposal) {
	match := actionBlockRegex.FindStringSubmatch(content)
	if len(match) < 2 {
		return content, nil
	}

	jsonStr := match[1]
	cleanedContent := strings.TrimSpace(actionBlockRegex.ReplaceAllString(content, ""))

	var proposal models.AIAssistantActionProposal
	if err := json.Unmarshal([]byte(jsonStr), &proposal); err != nil {
		log.Printf("Warning: failed to unmarshal action proposal: %v, json=%s", err, jsonStr)
		return content, nil
	}

	// Validate action proposal template_id
	if proposal.Action == "create_instance" {
		paramsMap, ok := proposal.Params.(map[string]interface{})
		if ok {
			var params models.CreateInstanceActionParams
			pBytes, _ := json.Marshal(paramsMap)
			_ = json.Unmarshal(pBytes, &params)

			// Fallback template_id if empty or invalid
			if params.TemplateID == "" || params.TemplateID == "<EXACT_TEMPLATE_UUID>" {
				if len(pCtx.Templates) > 0 {
					params.TemplateID = pCtx.Templates[0].ID.String()
					params.TemplateName = pCtx.Templates[0].Name
				}
			}

			// Sanitize instance name
			if params.Name == "" {
				params.Name = "cloud-node-01"
			}
			params.Name = strings.ToLower(regexp.MustCompile(`[^a-z0-9\-]`).ReplaceAllString(params.Name, "-"))
			params.Name = strings.Trim(params.Name, "-")
			if len(params.Name) < 3 {
				params.Name = fmt.Sprintf("node-%s", params.Name)
			}
			if params.Hostname == "" {
				params.Hostname = params.Name
			}

			// Validate SSH key
			if (params.SSHKeyID == "" || params.SSHKeyID == "<EXACT_SSH_KEY_UUID_OR_EMPTY>") && len(pCtx.SSHKeys) > 0 {
				params.SSHKeyID = pCtx.SSHKeys[0].ID.String()
				params.SSHKeyName = pCtx.SSHKeys[0].Name
			}

			if params.VCPUs <= 0 {
				params.VCPUs = 2
			}
			if params.MemoryMB <= 0 {
				params.MemoryMB = 4096
			}
			if params.DiskGB <= 0 {
				params.DiskGB = 40
			}

			proposal.Params = params
		}
	}

	return cleanedContent, &proposal
}

func (s *AIAssistantUsecase) generateLocalFallback(
	pCtx *AIPlatformContext,
	msg string,
) (string, *models.AIAssistantActionProposal) {
	lower := strings.ToLower(msg)

	// 1. Support & Troubleshooting Queries (check first before creation intent)
	if strings.Contains(lower, "ssh") || strings.Contains(lower, "connect") || strings.Contains(lower, "login") {
		return `### 🔑 How to Connect to Your Nevatal Cloud Instances

To connect securely to any of your deployed Linux virtual machines or containers:

1. **Verify Instance Status**: Ensure your instance status is **Running** and has an allocated IP address in [Compute Instances](/dashboard/instances).
2. **Use Your SSH Key**: Connect from your local terminal using the SSH key attached during provisioning:
` + "```bash\n# Ubuntu instances\nssh ubuntu@<INSTANCE_IP> -i ~/.ssh/id_rsa\n\n# Debian instances\nssh debian@<INSTANCE_IP> -i ~/.ssh/id_rsa\n\n# Alpine instances\nssh alpine@<INSTANCE_IP> -i ~/.ssh/id_rsa\n```" + `
3. **Web Console Access**: If SSH is unreachable, open the instance details page and click **"Open Proxmox Console"** for zero-dependency VNC/xterm.js browser terminal access.`, nil
	}

	if strings.Contains(lower, "quota") || strings.Contains(lower, "usage") || strings.Contains(lower, "capacity") {
		usedVCPU := 0
		maxVCPU := 0
		usedRAM := 0
		maxRAM := 0
		usedDisk := 0
		maxDisk := 0
		if pCtx.QuotaUsage != nil {
			usedVCPU = pCtx.QuotaUsage.UsedVCPUs
			maxVCPU = pCtx.QuotaUsage.MaxVCPUs
			usedRAM = pCtx.QuotaUsage.UsedMemoryMB
			maxRAM = pCtx.QuotaUsage.MaxMemoryMB
			usedDisk = pCtx.QuotaUsage.UsedStorageGB
			maxDisk = pCtx.QuotaUsage.MaxStorageGB
		}
		return fmt.Sprintf(`### 📊 Your Current Resource Quotas & Usage

Here is your real-time resource utilization on the Dell PowerEdge R640 cluster:

| Resource | Used | Limit | Remaining | Utilization |
|---|---|---|---|---|
| **vCPU Cores** | %d | %d | %d vCPU | %s |
| **RAM** | %s | %s | %s | %s |
| **NVMe SSD Storage** | %d GB | %d GB | %d GB | %s |
| **Active Instances** | %d | %d | %d slots | %d%% |

💡 *Tip: Need higher quotas for production workloads? Contact your infrastructure administrator or upgrade your resource tier in [Usage & Quotas](/dashboard/usage).*`,
			usedVCPU, maxVCPU, maxVCPU-usedVCPU, calcPct(usedVCPU, maxVCPU),
			formatMB(usedRAM), formatMB(maxRAM), formatMB(maxRAM-usedRAM), calcPct(usedRAM, maxRAM),
			usedDisk, maxDisk, maxDisk-usedDisk, calcPct(usedDisk, maxDisk),
			len(pCtx.Instances), pCtx.QuotaUsage.MaxInstances, pCtx.QuotaUsage.MaxInstances-len(pCtx.Instances),
			(len(pCtx.Instances)*100)/max(1, pCtx.QuotaUsage.MaxInstances),
		), nil
	}

	if strings.Contains(lower, "proxmox") || strings.Contains(lower, "hardware") || strings.Contains(lower, "dell") || strings.Contains(lower, "r640") || strings.Contains(lower, "node") {
		return `### 🖥️ Hypervisor Architecture: Dell PowerEdge R640

Nevatal Cloud runs on enterprise Dell PowerEdge R640 rackmount servers managed by **Proxmox VE 8.x**:

- **Compute**: Dual Intel Xeon Gold Scalable processors (64 logical threads)
- **Memory**: 256 GB DDR4-2666 ECC Registered RAM
- **Storage**: Enterprise NVMe / SAS SSD array formatted in ZFS RAID-10 (` + "`local-zfs`" + `)
- **Networking**: Dual 10GbE SFP+ uplinks with Proxmox software-defined bridges (` + "`vmbr0`" + `)
- **Provisioning Engine**: REST API v2 token-authenticated worker pipeline with automated Cloud-Init metadata injection.`, nil
	}

	if strings.Contains(lower, "error") || strings.Contains(lower, "troubleshoot") || strings.Contains(lower, "stopped") || strings.Contains(lower, "failed") {
		return `### 🛠️ Instance Troubleshooting Checklist

If an instance is not behaving as expected:

1. **Check Provisioning Job**: Navigate to [Activity Logs](/dashboard/activity) to inspect the hypervisor task logs.
2. **Review Cloud-Init Logs**:
` + "```bash\nsudo cat /var/log/cloud-init-output.log\nsudo cat /var/log/syslog | grep cloud-init\n```" + `
3. **Verify Network & DHCP**: Ensure the guest received an IPv4 address. Run:
` + "```bash\nip a\nping -c 3 1.1.1.1\n```" + `
4. **Power Cycle / Restart**: Use the **"Restart"** button on the instance details page to execute an ACPI reboot or hard reset.
5. **Console Access**: Launch the Proxmox VNC console from your instance dashboard to observe boot messages and kernel panics directly.`, nil
	}

	// 2. Check if user is asking to create/spin up a container or instance
	isCreateIntent := strings.Contains(lower, "create") ||
		strings.Contains(lower, "launch") ||
		strings.Contains(lower, "spin up") ||
		strings.Contains(lower, "deploy") ||
		strings.Contains(lower, "provision") ||
		strings.Contains(lower, "new container") ||
		strings.Contains(lower, "new instance") ||
		strings.Contains(lower, "new vm") ||
		strings.Contains(lower, "container")

	if isCreateIntent {
		// Identify OS
		var chosenTemplate models.OSTemplate
		if len(pCtx.Templates) > 0 {
			chosenTemplate = pCtx.Templates[0] // default Ubuntu
		}

		for _, t := range pCtx.Templates {
			tLower := strings.ToLower(t.Name)
			if strings.Contains(lower, "debian") && strings.Contains(tLower, "debian") {
				chosenTemplate = t
				break
			}
			if strings.Contains(lower, "alpine") && strings.Contains(tLower, "alpine") {
				chosenTemplate = t
				break
			}
			if strings.Contains(lower, "wordpress") && strings.Contains(tLower, "wordpress") {
				chosenTemplate = t
				break
			}
			if strings.Contains(lower, "ubuntu") && strings.Contains(tLower, "ubuntu") {
				chosenTemplate = t
				break
			}
		}

		// Spec selection
		vcpus := 2
		memoryMB := 4096
		diskGB := 40
		name := "server-01"

		if strings.Contains(lower, "micro") || strings.Contains(lower, "nano") || strings.Contains(lower, "light") || strings.Contains(lower, "alpine") {
			vcpus = 1
			memoryMB = 1024
			diskGB = 15
			name = "micro-node"
		} else if strings.Contains(lower, "pro") || strings.Contains(lower, "large") || strings.Contains(lower, "database") {
			vcpus = 4
			memoryMB = 8192
			diskGB = 80
			name = "db-server"
		} else if strings.Contains(lower, "wordpress") {
			vcpus = 2
			memoryMB = 4096
			diskGB = 50
			name = "wp-portal"
		}

		name = fmt.Sprintf("%s-%d", name, time.Now().Unix()%1000)

		var sshKeyID string
		var sshKeyName string
		if len(pCtx.SSHKeys) > 0 {
			sshKeyID = pCtx.SSHKeys[0].ID.String()
			sshKeyName = pCtx.SSHKeys[0].Name
		}

		params := models.CreateInstanceActionParams{
			Name:         name,
			Hostname:     name,
			TemplateID:   chosenTemplate.ID.String(),
			TemplateName: chosenTemplate.Name,
			VCPUs:        vcpus,
			MemoryMB:     memoryMB,
			DiskGB:       diskGB,
			SSHKeyID:     sshKeyID,
			SSHKeyName:   sshKeyName,
			Reason:       fmt.Sprintf("Configured with %s on Dell PowerEdge R640 cluster.", chosenTemplate.Name),
		}

		proposal := &models.AIAssistantActionProposal{
			Action:      "create_instance",
			Title:       fmt.Sprintf("Provision %s Container", chosenTemplate.Name),
			Description: fmt.Sprintf("%d vCPUs • %d MB RAM • %d GB NVMe SSD", vcpus, memoryMB, diskGB),
			Params:      params,
			Executed:    false,
		}

		reply := fmt.Sprintf(`### 🚀 Container Provisioning Recommendation

I have analyzed your request and prepared a high-performance **%s** container configuration on our Dell PowerEdge R640 Proxmox cluster.

**Configuration Details:**
- **OS Template**: %s
- **Instance Name**: %s
- **Compute Sizing**: %d vCPUs | %s RAM | %d GB NVMe SSD
- **SSH Key**: %s

You can deploy this container immediately using the 1-click **Deploy Container Now** action below, or customize it further in the deployment wizard!`,
			chosenTemplate.Name,
			chosenTemplate.Name,
			name,
			vcpus,
			formatMB(memoryMB),
			diskGB,
			formatSSHKey(sshKeyName),
		)

		return reply, proposal
	}

	// Default overview response
	return fmt.Sprintf(`### 👋 Welcome to Nevatal Cloud AI Copilot!

I am your infrastructure copilot powered by OpenRouter. I can help you manage containers, deploy apps, and solve technical support issues on our Dell PowerEdge R640 Proxmox VE platform.

**What would you like to do?**
- 🚀 **Create a Container**: Try *"Create an Ubuntu 24.04 container with 2 vCPUs and 4GB RAM"* or *"Deploy Alpine Linux micro-instance"*.
- 🌐 **Deploy Applications**: Ask *"Deploy WordPress 6.7 LEMP stack"*.
- 📊 **Check Resources**: Ask *"What are my current resource quotas and usage?"*.
- 🔑 **Connectivity Support**: Ask *"How do I connect to my instance via SSH?"*.
- 🖥️ **Infrastructure**: Ask *"Explain the Dell PowerEdge R640 Proxmox architecture"*.

*(Tip: You can use your own OpenRouter API key with models like Claude 3.5 Sonnet, GPT-4o, or DeepSeek V3 in the model selector above!)*`), nil
}

func (s *AIAssistantUsecase) ExecuteAction(
	ctx context.Context,
	userID uuid.UUID,
	input ExecuteActionInput,
	clientIP string,
) (*ExecuteActionResult, error) {
	ctx = context.WithoutCancel(ctx)

	switch input.Action {
	case "create_instance":
		var params models.CreateInstanceActionParams
		if err := json.Unmarshal(input.Params, &params); err != nil {
			return nil, fmt.Errorf("invalid create_instance parameters: %w", err)
		}

		tplID, err := uuid.Parse(params.TemplateID)
		if err != nil {
			return nil, errors.New("invalid template ID in action parameters")
		}

		var sshKeyUUID *uuid.UUID
		if params.SSHKeyID != "" {
			if parsed, pErr := uuid.Parse(params.SSHKeyID); pErr == nil {
				sshKeyUUID = &parsed
			}
		}

		createInput := CreateInstanceInput{
			Name:       params.Name,
			Hostname:   params.Hostname,
			TemplateID: tplID,
			VCPUs:      params.VCPUs,
			MemoryMB:   params.MemoryMB,
			DiskGB:     params.DiskGB,
			SSHKeyID:   sshKeyUUID,
		}

		job, err := s.instanceService.Create(ctx, userID, createInput, clientIP)
		if err != nil {
			return nil, fmt.Errorf("failed to deploy container: %w", err)
		}

		// Mark action as executed in message if message ID is provided
		if input.MessageID != uuid.Nil {
			_ = s.repos.Messages.MarkActionExecuted(ctx, input.MessageID, userID)
		}

		return &ExecuteActionResult{
			InstanceID: job.InstanceID,
			JobID:      job.ID,
			Message:    "Container provisioning job queued successfully!",
			Name:       params.Name,
			Status:     job.Status,
		}, nil

	default:
		return nil, fmt.Errorf("unsupported action type: %s", input.Action)
	}
}

func formatMB(mb int) string {
	if mb >= 1024 {
		return fmt.Sprintf("%.1f GB", float64(mb)/1024.0)
	}
	return fmt.Sprintf("%d MB", mb)
}

func formatSSHKey(name string) string {
	if name == "" {
		return "Default SSH Key / Password via Cloud-Init"
	}
	return name
}

func calcPct(used, maxVal int) string {
	if maxVal <= 0 {
		return "0%"
	}
	pct := (used * 100) / maxVal
	return fmt.Sprintf("%d%%", pct)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// OpenRouterKeyCheck is the outcome of verifying an OpenRouter API key. When
// Valid is false, RawResponse holds OpenRouter's rejection body.
type OpenRouterKeyCheck struct {
	Valid       bool
	Info        map[string]interface{}
	RawResponse string
}

// VerifyOpenRouterKey asks OpenRouter whether apiKey is valid. An unreachable
// OpenRouter is a KindBadGateway error.
func (s *AIAssistantUsecase) VerifyOpenRouterKey(ctx context.Context, apiKey string) (*OpenRouterKeyCheck, error) {
	// Ping OpenRouter key check endpoint
	req, err := http.NewRequestWithContext(ctx, "GET", "https://openrouter.ai/api/v1/auth/key", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiKey))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, newError(KindBadGateway, "OPENROUTER_UNREACHABLE", "Failed to connect to OpenRouter: "+err.Error())
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		return &OpenRouterKeyCheck{Valid: false, RawResponse: string(bodyBytes)}, nil
	}

	var keyInfo map[string]interface{}
	_ = json.Unmarshal(bodyBytes, &keyInfo)
	return &OpenRouterKeyCheck{Valid: true, Info: keyInfo}, nil
}
