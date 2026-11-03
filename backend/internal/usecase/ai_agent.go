package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
)

// AIAgentRepos are the repositories AIAgentUsecase reads and writes.
type AIAgentRepos struct {
	Agent     repository.AIAgentRepository
	Templates repository.TemplateRepository
	SSHKeys   repository.SSHKeyRepository
	Android   repository.AndroidRepository
}

type AIAgentUsecase struct {
	repos             AIAgentRepos
	cfg               *config.Config
	instanceService   *InstanceUsecase
	appService        *AppUsecase
	automationService *AutomationUsecase
	openClawService   *OpenClawUsecase
	storageService    *StorageUsecase
	gameServerService *GameServerUsecase
	androidService    *AndroidUsecase
	databaseService   *DatabaseUsecase
	quotaService      *QuotaUsecase
	mcpService        *MCPUsecase
	orch              *OrchestratorUsecase
	pve               proxmox.ProxmoxClient
	httpClient        *http.Client
}

func NewAIAgentUsecase(
	repos AIAgentRepos,
	cfg *config.Config,
	instanceService *InstanceUsecase,
	appService *AppUsecase,
	automationService *AutomationUsecase,
	openClawService *OpenClawUsecase,
	storageService *StorageUsecase,
	gameServerService *GameServerUsecase,
	androidService *AndroidUsecase,
	quotaService *QuotaUsecase,
	mcpService *MCPUsecase,
	orch *OrchestratorUsecase,
	pve proxmox.ProxmoxClient,
) *AIAgentUsecase {
	return &AIAgentUsecase{
		repos:             repos,
		cfg:               cfg,
		instanceService:   instanceService,
		appService:        appService,
		automationService: automationService,
		openClawService:   openClawService,
		storageService:    storageService,
		gameServerService: gameServerService,
		androidService:    androidService,
		quotaService:      quotaService,
		mcpService:        mcpService,
		orch:              orch,
		pve:               pve,
		httpClient: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

// Conversation Management Methods

func (s *AIAgentUsecase) CreateConversation(ctx context.Context, userID uuid.UUID, title string) (*models.AIConversation, error) {
	ctx = context.WithoutCancel(ctx)

	if strings.TrimSpace(title) == "" {
		title = "New Infrastructure Plan"
	}

	conv := models.AIConversation{
		UserID:        userID,
		Title:         title,
		State:         models.StateUnderstandingRequest,
		CurrentIntent: "",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	if err := s.repos.Agent.CreateConversation(ctx, &conv); err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}

	// Create initial conversation state
	initialState := models.AIConversationState{
		ConversationID:       conv.ID,
		State:                models.StateUnderstandingRequest,
		CurrentIntent:        "",
		PendingQuestions:     "[]",
		GatheredRequirements: "{}",
		UpdatedAt:            time.Now(),
	}
	_ = s.repos.Agent.CreateState(ctx, &initialState)

	// Send initial greeting assistant message
	welcomeMsg := models.AIMessage{
		ConversationID: conv.ID,
		UserID:         userID,
		Role:           "assistant",
		Content:        "👋 **Hello! I am your Nevatal AI Cloud Agent.**\n\nI can help you design, configure, and provision production cloud infrastructure through simple natural language.\n\n**What would you like to build today?**\n- 🖥️ **Virtual Private Server (VPS)** with Ubuntu, Debian, Alpine, or WordPress\n- 🌐 **Deploy a Web Application** from GitHub or GitLab\n- ⚡ **n8n Workflow Automation** instance\n- 🤖 **OpenClaw Autonomous AI Agent**\n- 🗄️ **S3-compatible Object Storage Bucket**\n- 🎮 **Game Server Hosting** (Minecraft Java/Bedrock, Terraria)\n- 💾 **Managed Database** (PostgreSQL, MySQL, Redis, MongoDB)\n- 🧩 **Deploy a saved infrastructure template**\n- 🩺 **Diagnose resources and check infrastructure health**\n- 💡 **Optimize costs and right-size idle resources**",
		CreatedAt:      time.Now(),
	}
	_ = s.repos.Agent.CreateMessage(ctx, &welcomeMsg)

	_ = s.repos.Agent.ReloadConversation(ctx, &conv)
	return &conv, nil
}

func (s *AIAgentUsecase) ListConversations(ctx context.Context, userID uuid.UUID) ([]models.AIConversation, error) {
	return s.repos.Agent.ListConversations(ctx, userID)
}

func (s *AIAgentUsecase) GetConversation(ctx context.Context, convID uuid.UUID, userID uuid.UUID) (*models.AIConversation, error) {
	conv, err := s.repos.Agent.FindConversationDetails(ctx, convID, userID)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	return conv, nil
}

func (s *AIAgentUsecase) DeleteConversation(ctx context.Context, convID uuid.UUID, userID uuid.UUID) error {
	return s.repos.Agent.DeleteConversation(context.WithoutCancel(ctx), convID, userID)
}

// GetConversationState returns the workflow state of one of the user's
// conversations, creating it when missing.
func (s *AIAgentUsecase) GetConversationState(ctx context.Context, convID uuid.UUID, userID uuid.UUID) (*models.AIConversationState, error) {
	conv, err := s.repos.Agent.FindConversation(ctx, convID, userID)
	if err != nil {
		return nil, errors.New("conversation not found")
	}

	state, err := s.repos.Agent.FindState(ctx, convID)
	if err != nil {
		// Initialize if missing
		state = &models.AIConversationState{
			ConversationID:       convID,
			State:                conv.State,
			CurrentIntent:        conv.CurrentIntent,
			PendingQuestions:     "[]",
			GatheredRequirements: "{}",
			UpdatedAt:            time.Now(),
		}
		_ = s.repos.Agent.CreateState(context.WithoutCancel(ctx), state)
	}
	return state, nil
}

func (s *AIAgentUsecase) ListMessages(ctx context.Context, convID uuid.UUID, userID uuid.UUID) ([]models.AIMessage, error) {
	if _, err := s.repos.Agent.FindConversation(ctx, convID, userID); err != nil {
		return nil, errors.New("conversation not found")
	}
	return s.repos.Agent.ListMessages(ctx, convID)
}

func (s *AIAgentUsecase) ListPlans(ctx context.Context, convID uuid.UUID, userID uuid.UUID) ([]models.AIResourcePlan, error) {
	return s.repos.Agent.ListPlans(ctx, convID, userID)
}

// User Message Processing & Conversation State Machine

type SendMessageInput struct {
	Message   string                 `json:"message"`
	AnswerKey string                 `json:"answer_key,omitempty"` // If user clicked an interactive answer chip
	AnswerVal string                 `json:"answer_val,omitempty"` // Value for the answered key
	Answers   map[string]interface{} `json:"answers,omitempty"`    // Map of key-value answers
	APIKey    string                 `json:"api_key,omitempty"`    // Optional user OpenRouter API Key
	Model     string                 `json:"model,omitempty"`      // Model preference
	// IsAdmin is set by the handler from the authenticated request; it is
	// never accepted from the body. Plans created from chat use it for
	// administrator-only tools (for example an admin template).
	IsAdmin bool `json:"-"`
}

type SendMessageResponse struct {
	Message      models.AIMessage           `json:"message"`
	State        models.WorkflowState       `json:"state"`
	PlanProposal *models.StructuredPlanView `json:"plan_proposal,omitempty"`
	PlanID       string                     `json:"plan_id,omitempty"`
}

func (s *AIAgentUsecase) ProcessUserMessage(
	ctx context.Context,
	convID uuid.UUID,
	userID uuid.UUID,
	input SendMessageInput,
	clientIP string,
) (*SendMessageResponse, error) {
	ctx = context.WithoutCancel(ctx)

	conv, err := s.repos.Agent.FindConversation(ctx, convID, userID)
	if err != nil {
		return nil, errors.New("conversation not found")
	}

	convState, err := s.GetConversationState(ctx, convID, userID)
	if err != nil {
		return nil, err
	}

	// 1. Record User Message
	userMsg := models.AIMessage{
		ConversationID: convID,
		UserID:         userID,
		Role:           "user",
		Content:        input.Message,
		RawInput:       input.Message,
		CreatedAt:      time.Now(),
	}
	if err := s.repos.Agent.CreateMessage(ctx, &userMsg); err != nil {
		return nil, fmt.Errorf("failed to save message: %w", err)
	}

	// Update conversation title if default
	if conv.Title == "New Infrastructure Plan" && len(input.Message) > 0 {
		cleanTitle := input.Message
		if len(cleanTitle) > 40 {
			cleanTitle = cleanTitle[:37] + "..."
		}
		conv.Title = cleanTitle
		_ = s.repos.Agent.SetConversationTitle(ctx, conv, conv.Title)
	}

	// 2. Parse Requirements & Context
	reqs := make(map[string]string)
	_ = json.Unmarshal([]byte(convState.GatheredRequirements), &reqs)

	if input.AnswerKey != "" && input.AnswerVal != "" {
		reqs[input.AnswerKey] = input.AnswerVal
	}
	for k, v := range input.Answers {
		reqs[k] = fmt.Sprintf("%v", v)
	}

	// Load platform context for validation & capacity checks
	templates, _ := s.repos.Templates.ListEnabledByName(ctx)
	sshKeys, _ := s.repos.SSHKeys.ListByOwnerByName(ctx, userID)

	quotaUsage, _ := s.quotaService.GetUsage(ctx, userID)

	// 3. Determine Intent & Workflow Transition
	msgLower := strings.ToLower(input.Message)

	var assistantReply string
	var uiPayload models.UIComponentsPayload
	var planProposal *models.StructuredPlanView
	var newPlanID string

	// Questions about existing resources ("list my servers", "what is the
	// status of web-01") are answered with read tools. They never start or
	// change a plan, and the conversation's workflow is left as it was.
	// Structured answers to the planner's questions are never read requests.
	if input.AnswerKey == "" && len(input.Answers) == 0 && isReadRequest(msgLower, func() []string { return s.instanceNames(ctx, userID) }) {
		assistantReply = s.answerReadRequest(ctx, userID, msgLower, quotaUsage, clientIP)
		return s.finishTurn(ctx, conv, convState, reqs, convID, userID, assistantReply, uiPayload, planProposal, newPlanID)
	}

	intent := convState.CurrentIntent
	if intent == "" {
		intent = detectIntent(msgLower)
		convState.CurrentIntent = intent
	}

	// Explicit operation requests switch the turn even when the conversation
	// was about something else. Destructive and plan-creating requests also
	// become the conversation's intent (they start a new plan); read-only
	// diagnostics and advice are handled for this turn only, so a pending
	// plan stays intact.
	turnIntent := intent
	if input.AnswerKey == "" && len(input.Answers) == 0 {
		switch detected := detectIntent(msgLower); detected {
		case "delete_vps", "deploy_template", "create_database":
			if intent != detected {
				turnIntent, intent = detected, detected
				convState.CurrentIntent = detected
				convState.ActivePlanID = nil
				convState.PendingQuestions = "[]"
				delete(reqs, "target_instance_id")
				delete(reqs, "infra_template_id")
				delete(reqs, "template_name")
				delete(reqs, "template_awaiting_param")
				for k := range reqs {
					if strings.HasPrefix(k, templateParamPrefix) {
						delete(reqs, k)
					}
				}
			}
		case "troubleshoot", "advisor":
			turnIntent = detected
		}
	}

	// Extract inline specifications from user message
	extractRequirementsFromText(msgLower, reqs, templates, sshKeys)

	// 4. State Machine Branching
	switch turnIntent {
	case "create_vps":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleVPSWorkflow(
			ctx, convID, userID, convState, reqs, templates, sshKeys, quotaUsage,
		)
	case "deploy_website":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleWebsiteWorkflow(
			ctx, convID, userID, convState, reqs, quotaUsage,
		)
	case "n8n_instance":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleN8nWorkflow(
			ctx, convID, userID, convState, reqs, quotaUsage,
		)
	case "openclaw_instance":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleOpenClawWorkflow(
			ctx, convID, userID, convState, reqs, quotaUsage,
		)
	case "create_storage":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleStorageWorkflow(
			ctx, convID, userID, convState, reqs,
		)
	case "game_server":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleGameServerWorkflow(
			ctx, convID, userID, convState, reqs, quotaUsage,
		)
	case "android_instance":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleAndroidWorkflow(
			ctx, convID, userID, convState, reqs, quotaUsage,
		)
	case "delete_vps":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleDeleteVPSWorkflow(
			ctx, convID, userID, convState, reqs, msgLower,
		)
	case "create_database":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleDatabaseWorkflow(
			ctx, convID, userID, convState, reqs, msgLower,
		)
	case "deploy_template":
		assistantReply, uiPayload, planProposal, newPlanID = s.handleTemplateWorkflow(
			ctx, convID, userID, input.IsAdmin, convState, reqs, msgLower,
		)
	case "advisor":
		assistantReply = s.handleAdvisor(ctx, userID)
		if turnIntent == convState.CurrentIntent {
			convState.State = models.StateCompleted
		}
	case "troubleshoot":
		assistantReply = s.handleDiagnostics(ctx, userID, msgLower)
		if turnIntent == convState.CurrentIntent {
			convState.State = models.StateCompleted
		}
	default:
		// General cloud advice & discovery
		assistantReply = s.handleGeneralQuery(msgLower, quotaUsage, templates)
		convState.State = models.StateUnderstandingRequest
	}

	return s.finishTurn(ctx, conv, convState, reqs, convID, userID, assistantReply, uiPayload, planProposal, newPlanID)
}

// finishTurn saves the conversation state and the assistant message of one
// chat turn.
func (s *AIAgentUsecase) finishTurn(
	ctx context.Context,
	conv *models.AIConversation,
	convState *models.AIConversationState,
	reqs map[string]string,
	convID, userID uuid.UUID,
	assistantReply string,
	uiPayload models.UIComponentsPayload,
	planProposal *models.StructuredPlanView,
	newPlanID string,
) (*SendMessageResponse, error) {
	// 5. Save State and Assistant Message
	reqsBytes, _ := json.Marshal(reqs)
	convState.GatheredRequirements = string(reqsBytes)
	convState.UpdatedAt = time.Now()
	_ = s.repos.Agent.SaveState(ctx, convState)

	conv.State = convState.State
	conv.CurrentIntent = convState.CurrentIntent
	conv.UpdatedAt = time.Now()
	_ = s.repos.Agent.SaveConversation(ctx, conv)

	uiBytes, _ := json.Marshal(uiPayload)
	assistantMsg := models.AIMessage{
		ConversationID: convID,
		UserID:         userID,
		Role:           "assistant",
		Content:        assistantReply,
		UIComponents:   string(uiBytes),
		CreatedAt:      time.Now(),
	}
	_ = s.repos.Agent.CreateMessage(ctx, &assistantMsg)

	return &SendMessageResponse{
		Message:      assistantMsg,
		State:        convState.State,
		PlanProposal: planProposal,
		PlanID:       newPlanID,
	}, nil
}

// Workflow Handlers for Supported Use Cases

func (s *AIAgentUsecase) handleVPSWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
	templates []models.OSTemplate,
	sshKeys []models.SSHKey,
	quota *models.QuotaUsage,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	// Check what is missing according to Section 35.1 (OS, Sizing, Public IP, SSH Key)
	var missingQuestions []models.StructuredQuestion

	if _, ok := reqs["os_template_id"]; !ok {
		var osOptions []models.StructuredQuestionOption
		for i, t := range templates {
			desc := fmt.Sprintf("%s (%s)", t.Distribution, t.Version)
			osOptions = append(osOptions, models.StructuredQuestionOption{
				ID:          t.ID.String(),
				Label:       t.Name,
				Description: desc,
				Value:       t.ID.String(),
				IsDefault:   i == 0,
			})
		}
		missingQuestions = append(missingQuestions, models.StructuredQuestion{
			ID:          "q_os",
			Key:         "os_template_id",
			Title:       "Which operating system would you like to install?",
			Description: "All images are pre-configured with cloud-init for automated boot.",
			Type:        "single_choice",
			Options:     osOptions,
			Required:    true,
		})
	}

	if _, ok := reqs["specs"]; !ok {
		missingQuestions = append(missingQuestions, models.StructuredQuestion{
			ID:          "q_specs",
			Key:         "specs",
			Title:       "What compute size do you need?",
			Description: "Allocated directly on Dell PowerEdge R640 local-zfs NVMe storage.",
			Type:        "single_choice",
			Options: []models.StructuredQuestionOption{
				{ID: "nano", Label: "Nano (1 vCPU, 1 GB RAM, 20 GB SSD)", Value: "1:1024:20", Description: "Microservices & sandbox"},
				{ID: "standard", Label: "Standard (2 vCPU, 4 GB RAM, 40 GB SSD)", Value: "2:4096:40", IsDefault: true, Description: "APIs, web servers & databases"},
				{ID: "power", Label: "Power (4 vCPU, 8 GB RAM, 80 GB SSD)", Value: "4:8192:80", Description: "Heavy production workloads"},
			},
			Required: true,
		})
	}

	if _, ok := reqs["public_networking"]; !ok {
		missingQuestions = append(missingQuestions, models.StructuredQuestion{
			ID:          "q_net",
			Key:         "public_networking",
			Title:       "Do you require a dedicated public IPv4 address?",
			Description: "Allows direct inbound SSH and HTTPS traffic from the Internet.",
			Type:        "single_choice",
			Options: []models.StructuredQuestionOption{
				{ID: "yes", Label: "Yes, allocate Public IPv4", Value: "true", IsDefault: true},
				{ID: "no", Label: "No, private VPC only", Value: "false"},
			},
			Required: false,
		})
	}

	// If questions remain unanswered, ask them
	if len(missingQuestions) > 0 {
		state.State = models.StateClarifyingRequirements
		qBytes, _ := json.Marshal(missingQuestions)
		state.PendingQuestions = string(qBytes)

		reply := "I can help you provision this VPS on our Dell PowerEdge R640 cluster. I need a couple of details to prepare the deployment plan:"
		return reply, models.UIComponentsPayload{Questions: missingQuestions}, nil, ""
	}

	// All requirements collected -> Generate Structured Plan
	state.State = models.StateAwaitingConfirmation
	state.PendingQuestions = "[]"

	// Parse Specs
	vcpus, memoryMB, diskGB := 2, 4096, 40
	if specStr, ok := reqs["specs"]; ok {
		parts := strings.Split(specStr, ":")
		if len(parts) == 3 {
			fmt.Sscanf(parts[0], "%d", &vcpus)
			fmt.Sscanf(parts[1], "%d", &memoryMB)
			fmt.Sscanf(parts[2], "%d", &diskGB)
		}
	}

	// Identify OS Template
	selectedTpl := templates[0]
	if tplIDStr, ok := reqs["os_template_id"]; ok {
		for _, t := range templates {
			if t.ID.String() == tplIDStr {
				selectedTpl = t
				break
			}
		}
	}

	// Instance Name
	instanceName := reqs["name"]
	if instanceName == "" {
		instanceName = fmt.Sprintf("vps-%s-%d", strings.ToLower(selectedTpl.Distribution), time.Now().Unix()%1000)
		reqs["name"] = instanceName // follow-up messages revise this plan, same name
	}

	planTitle := fmt.Sprintf("Provision %s Virtual Machine", selectedTpl.Name)
	planSummary := fmt.Sprintf("%d vCPU • %d MB RAM • %d GB NVMe SSD (Dell PowerEdge R640)", vcpus, memoryMB, diskGB)

	vmConfig := map[string]any{
		"name":        instanceName,
		"template_id": selectedTpl.ID.String(),
		"vcpus":       vcpus,
		"memory_mb":   memoryMB,
		"disk_gb":     diskGB,
	}
	if keyID := reqs["ssh_key_id"]; keyID != "" {
		vmConfig["ssh_key_ids"] = []string{keyID}
	}
	spec := orchestrator.Specification{Intent: "create_vps", Resources: []orchestrator.ResourceSpec{{
		Key: instanceName, Type: orchestrator.TypeVPS, Action: orchestrator.ActionCreate, Config: vmConfig,
	}}}

	// Prepare UI Structured Plan View
	planView := &models.StructuredPlanView{
		Title:                planTitle,
		Intent:               "create_vps",
		Summary:              planSummary,
		Reversible:           true,
		RequiresConfirmation: true,
		Status:               "awaiting_confirmation",
		EstimatedAllocations: map[string]interface{}{
			"vcpus":     vcpus,
			"memory_mb": memoryMB,
			"disk_gb":   diskGB,
			"storage":   "local-zfs (NVMe RAID-10)",
			"node":      "Dell PowerEdge R640",
		},
		Resources: []models.StructuredResourceView{
			{
				Type:    "Virtual Machine",
				Name:    instanceName,
				Summary: fmt.Sprintf("%s with %d vCPU / %d MB RAM / %d GB Disk", selectedTpl.Name, vcpus, memoryMB, diskGB),
				Details: map[string]interface{}{
					"Operating System": selectedTpl.Name,
					"CPU Allocation":   fmt.Sprintf("%d vCPU Cores", vcpus),
					"RAM Allocation":   fmt.Sprintf("%d MB", memoryMB),
					"NVMe SSD":         fmt.Sprintf("%d GB", diskGB),
					"Networking":       "Public IPv4 DHCP (vmbr0)",
					"Access":           "SSH Key / Proxmox Web VNC Console",
				},
				Order:  1,
				Status: "pending",
			},
		},
	}

	card, planID, err := s.proposePlan(ctx, convID, userID, state, "create_vps", planTitle, planSummary, "", spec, planView)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}

	reply := fmt.Sprintf("I have validated your requirements against the cluster hardware capacity and generated an infrastructure plan for your approval.\n\n### Proposed VPS Configuration\n- **Operating System**: %s\n- **Instance Name**: `%s`\n- **Specs**: %d vCPU · %s RAM · %d GB NVMe SSD\n- **Network Exposure**: Public IPv4\n\nPlease review the configuration below and confirm when you are ready to begin provisioning.",
		selectedTpl.Name, instanceName, vcpus, formatMB(memoryMB), diskGB)

	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, planID
}

func (s *AIAgentUsecase) handleWebsiteWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
	quota *models.QuotaUsage,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	var missing []models.StructuredQuestion

	if _, ok := reqs["git_repo"]; !ok {
		missing = append(missing, models.StructuredQuestion{
			ID:          "q_git_repo",
			Key:         "git_repo",
			Title:       "What is your Git repository URL?",
			Description: "GitHub or GitLab repository (HTTPS URL).",
			Type:        "single_choice",
			Options: []models.StructuredQuestionOption{
				{ID: "sample_go", Label: "Sample Go API (https://github.com/nevatal/go-api-service)", Value: "https://github.com/nevatal/go-api-service", IsDefault: true},
				{ID: "sample_react", Label: "Sample React App (https://github.com/nevatal/react-portal)", Value: "https://github.com/nevatal/react-portal"},
			},
			Required: true,
		})
	}

	if _, ok := reqs["framework"]; !ok {
		missing = append(missing, models.StructuredQuestion{
			ID:    "q_framework",
			Key:   "framework",
			Title: "What framework or language does the application use?",
			Type:  "single_choice",
			Options: []models.StructuredQuestionOption{
				{ID: "go", Label: "Go (Golang API / Binary)", Value: "go", IsDefault: true},
				{ID: "react", Label: "React / Vite (Static SPA)", Value: "react"},
				{ID: "nextjs", Label: "Next.js (Node.js SSR)", Value: "nextjs"},
				{ID: "python", Label: "Python (FastAPI / Flask)", Value: "python"},
				{ID: "docker", Label: "Dockerfile (Custom Container)", Value: "docker"},
			},
			Required: true,
		})
	}

	if len(missing) > 0 {
		state.State = models.StateClarifyingRequirements
		qBytes, _ := json.Marshal(missing)
		state.PendingQuestions = string(qBytes)
		return "I can help you build and deploy your web application. Please specify the repository and framework details:",
			models.UIComponentsPayload{Questions: missing}, nil, ""
	}

	state.State = models.StateAwaitingConfirmation
	appName := reqs["name"]
	if appName == "" {
		appName = "web-app-" + fmt.Sprintf("%d", time.Now().Unix()%1000)
		reqs["name"] = appName
	}
	repo := reqs["git_repo"]
	framework := reqs["framework"]

	planTitle := fmt.Sprintf("Deploy %s Application (%s)", appName, framework)
	planSummary := fmt.Sprintf("Automated build from %s with Caddy reverse proxy and SSL", repo)

	planView := &models.StructuredPlanView{
		Title:                planTitle,
		Intent:               "deploy_website",
		Summary:              planSummary,
		Reversible:           true,
		RequiresConfirmation: true,
		Status:               "awaiting_confirmation",
		EstimatedAllocations: map[string]interface{}{
			"vcpus":     1,
			"memory_mb": 1024,
			"disk_gb":   10,
			"proxy":     "Caddy (Auto-HTTPS)",
		},
		Resources: []models.StructuredResourceView{
			{
				Type:    "Application Deployment",
				Name:    appName,
				Summary: fmt.Sprintf("Build %s from %s", framework, repo),
				Details: map[string]interface{}{
					"Repository": repo,
					"Framework":  framework,
					"Branch":     "main",
					"Domain":     appName + ".nevatal.app",
					"HTTPS":      "Automatic Let's Encrypt TLS",
				},
				Order:  1,
				Status: "pending",
			},
		},
	}

	// Create Plan Record
	spec := orchestrator.Specification{Intent: "deploy_website", Resources: []orchestrator.ResourceSpec{{
		Key: appName, Type: orchestrator.TypeApp, Action: orchestrator.ActionCreate,
		Config: map[string]any{"name": appName, "repository_url": repo, "framework": framework, "branch": "main"},
	}}}
	card, planID, err := s.proposePlan(ctx, convID, userID, state, "deploy_website", planTitle, planSummary, "", spec, planView)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	reply := fmt.Sprintf("I have prepared the deployment plan for **%s**. Once confirmed, I will clone the repository, build the runtime, supervise the process, and configure automatic HTTPS routing.", appName)
	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, planID
}

func (s *AIAgentUsecase) handleN8nWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
	quota *models.QuotaUsage,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	name := reqs["name"]
	if name == "" {
		name = fmt.Sprintf("n8n-workflow-%d", time.Now().Unix()%1000)
		reqs["name"] = name
	}

	planTitle := fmt.Sprintf("Provision Dedicated n8n Automation Instance (%s)", name)
	planSummary := "Dedicated n8n engine with SQLite / PostgreSQL persistence and webhook triggers."

	planView := &models.StructuredPlanView{
		Title:                planTitle,
		Intent:               "n8n_instance",
		Summary:              planSummary,
		Reversible:           true,
		RequiresConfirmation: true,
		Status:               "awaiting_confirmation",
		EstimatedAllocations: map[string]interface{}{
			"vcpus":     1,
			"memory_mb": 1024,
			"disk_gb":   15,
		},
		Resources: []models.StructuredResourceView{
			{
				Type:    "Automation Instance",
				Name:    name,
				Summary: "n8n Workflow Automation Server",
				Details: map[string]interface{}{
					"Instance Name": name,
					"Engine":        "n8n latest",
					"RAM":           "1024 MB",
					"Storage":       "15 GB persistent volume",
					"Webhooks":      "Active public endpoint with TLS",
				},
				Order:  1,
				Status: "pending",
			},
		},
	}

	spec := orchestrator.Specification{Intent: "n8n_instance", Resources: []orchestrator.ResourceSpec{{
		Key: name, Type: orchestrator.TypeN8n, Action: orchestrator.ActionCreate,
		Config: map[string]any{"name": name},
	}}}
	card, planID, err := s.proposePlan(ctx, convID, userID, state, "n8n_instance", planTitle, planSummary, "", spec, planView)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	reply := fmt.Sprintf("I have drafted the plan to launch your **n8n Workflow Automation** server (`%s`). Please review and confirm to deploy.", name)
	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, planID
}

func (s *AIAgentUsecase) handleOpenClawWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
	quota *models.QuotaUsage,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	name := reqs["name"]
	if name == "" {
		name = fmt.Sprintf("openclaw-agent-%d", time.Now().Unix()%1000)
		reqs["name"] = name
	}
	provider := reqs["provider"]
	if provider == "" {
		provider = "openai"
	}
	model := reqs["model"]
	if model == "" {
		model = "gpt-4o-mini"
	}

	planTitle := fmt.Sprintf("Deploy OpenClaw AI Agent (%s)", name)
	planSummary := fmt.Sprintf("Personal autonomous agent container connected to %s (%s)", provider, model)

	planView := &models.StructuredPlanView{
		Title:                planTitle,
		Intent:               "openclaw_instance",
		Summary:              planSummary,
		Reversible:           true,
		RequiresConfirmation: true,
		Status:               "awaiting_confirmation",
		EstimatedAllocations: map[string]interface{}{
			"vcpus":     1,
			"memory_mb": 1024,
			"disk_gb":   10,
		},
		Resources: []models.StructuredResourceView{
			{
				Type:    "AI Agent Instance",
				Name:    name,
				Summary: "OpenClaw Autonomous Agent Container",
				Details: map[string]interface{}{
					"Agent Name": name,
					"Provider":   provider,
					"Model":      model,
					"Workspace":  "/home/openclaw/workspace (Persistent)",
				},
				Order:  1,
				Status: "pending",
			},
		},
	}

	spec := orchestrator.Specification{Intent: "openclaw_instance", Resources: []orchestrator.ResourceSpec{{
		Key: name, Type: orchestrator.TypeOpenClaw, Action: orchestrator.ActionCreate,
		Config: map[string]any{"name": name, "provider": provider, "model": model},
	}}}
	card, planID, err := s.proposePlan(ctx, convID, userID, state, "openclaw_instance", planTitle, planSummary, "", spec, planView)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	reply := fmt.Sprintf("I have prepared the deployment proposal for **OpenClaw AI Agent** (`%s`). Confirm below to start provisioning.", name)
	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, planID
}

func (s *AIAgentUsecase) handleStorageWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	bucketName := reqs["bucket_name"]
	if bucketName == "" {
		bucketName = reqs["name"]
	}
	if bucketName == "" {
		bucketName = fmt.Sprintf("storage-bucket-%d", time.Now().Unix()%1000)
		reqs["bucket_name"] = bucketName
	}
	isPublic := reqs["is_public"] == "true"
	vis := "private"
	if isPublic {
		vis = "public"
	}

	planTitle := fmt.Sprintf("Create MinIO S3 Object Storage Bucket (%s)", bucketName)
	planSummary := fmt.Sprintf("S3-compatible bucket on distributed local-zfs NVMe storage (%s)", vis)

	planView := &models.StructuredPlanView{
		Title:                planTitle,
		Intent:               "create_storage",
		Summary:              planSummary,
		Reversible:           true,
		RequiresConfirmation: true,
		Status:               "awaiting_confirmation",
		EstimatedAllocations: map[string]interface{}{
			"storage_backend": "MinIO Distributed ZFS",
			"visibility":      vis,
			"endpoints":       "S3 API + Web File Browser",
		},
		Resources: []models.StructuredResourceView{
			{
				Type:    "Storage Bucket",
				Name:    bucketName,
				Summary: fmt.Sprintf("MinIO S3 Bucket (%s)", vis),
				Details: map[string]interface{}{
					"Bucket Name": bucketName,
					"Visibility":  vis,
					"S3 Endpoint": "https://s3.nevatal.id",
					"Features":    "Presigned URLs, Multipart Uploads, Public Links",
				},
				Order:  1,
				Status: "pending",
			},
		},
	}

	spec := orchestrator.Specification{Intent: "create_storage", Resources: []orchestrator.ResourceSpec{{
		Key: bucketName, Type: orchestrator.TypeBucket, Action: orchestrator.ActionCreate,
		Config: map[string]any{"name": bucketName, "is_public": isPublic},
	}}}
	card, planID, err := s.proposePlan(ctx, convID, userID, state, "create_storage", planTitle, planSummary, "", spec, planView)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	reply := fmt.Sprintf("I have prepared the proposal to create the S3 object storage bucket **`%s`**. Review the plan below to approve creation.", bucketName)
	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, planID
}

func (s *AIAgentUsecase) handleGameServerWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
	quota *models.QuotaUsage,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	gameType := reqs["game_type"]

	// Step 1: Detect Game Type (Minecraft Java, Minecraft Bedrock, Terraria)
	if gameType == "" {
		state.State = models.StateClarifyingRequirements
		question := models.StructuredQuestion{
			ID:          "q_game_type",
			Key:         "game_type",
			Title:       "Which game would you like to host?",
			Description: "Nevatal Cloud deploys managed game servers with dedicated networking and persistent world storage.",
			Type:        "single_choice",
			Required:    true,
			Options: []models.StructuredQuestionOption{
				{ID: "mc_java", Label: "Minecraft Java Edition", Value: models.GameTypeMinecraftJava, Description: "PC Minecraft with support for Paper, Fabric, Forge, plugins & mods", IsDefault: true},
				{ID: "mc_bedrock", Label: "Minecraft Bedrock Edition", Value: models.GameTypeMinecraftBedrock, Description: "Cross-platform Minecraft for Windows, iOS, Android, Xbox, and Switch"},
				{ID: "terraria", Label: "Terraria Dedicated Server", Value: models.GameTypeTerraria, Description: "Official multiplayer server with classic, expert, or master worlds"},
			},
		}
		return question.Title, models.UIComponentsPayload{
			Questions: []models.StructuredQuestion{question},
		}, nil, ""
	}

	// Step 2: Minecraft Java specific questions
	if gameType == models.GameTypeMinecraftJava {
		software := reqs["server_software"]
		if software == "" {
			state.State = models.StateClarifyingRequirements
			question := models.StructuredQuestion{
				ID:          "q_server_software",
				Key:         "server_software",
				Title:       "Which Minecraft Java server software would you prefer?",
				Description: "Select your preferred server engine.",
				Type:        "single_choice",
				Required:    true,
				Options: []models.StructuredQuestionOption{
					{ID: "paper", Label: "Paper (Recommended)", Value: models.SoftwarePaper, Description: "High-performance server with Bukkit/Spigot plugin support", IsDefault: true},
					{ID: "fabric", Label: "Fabric (Modded)", Value: models.SoftwareFabric, Description: "Lightweight, modern modding ecosystem"},
					{ID: "forge", Label: "Forge (Modded)", Value: models.SoftwareForge, Description: "Traditional modding engine for large modpacks"},
					{ID: "vanilla", Label: "Vanilla", Value: models.SoftwareVanilla, Description: "Official pure Mojang Minecraft server experience"},
					{ID: "purpur", Label: "Purpur", Value: models.SoftwarePurpur, Description: "Extended Paper fork with extra custom gameplay knobs"},
				},
			}
			return question.Title, models.UIComponentsPayload{
				Questions: []models.StructuredQuestion{question},
			}, nil, ""
		}

		difficulty := reqs["difficulty"]
		gamemode := reqs["gamemode"]
		if difficulty == "" || gamemode == "" {
			state.State = models.StateClarifyingRequirements
			question := models.StructuredQuestion{
				ID:          "q_mc_mode",
				Key:         "difficulty",
				Title:       "Do you want a survival or creative world, and what difficulty level?",
				Description: "Configures default world rules for new players.",
				Type:        "single_choice",
				Required:    true,
				Options: []models.StructuredQuestionOption{
					{ID: "survival_normal", Label: "Survival — Normal Difficulty", Value: "normal", Description: "Standard Minecraft multiplayer survival experience", IsDefault: true},
					{ID: "survival_hard", Label: "Survival — Hard Difficulty", Value: "hard", Description: "Challenging gameplay with full mob damage"},
					{ID: "survival_peaceful", Label: "Survival — Peaceful", Value: "peaceful", Description: "No hostile mobs, safe exploration and building"},
					{ID: "creative", Label: "Creative Mode", Value: "creative", Description: "Infinite resources and flight enabled for all builders"},
				},
			}
			return question.Title, models.UIComponentsPayload{
				Questions: []models.StructuredQuestion{question},
			}, nil, ""
		}
	} else if gameType == models.GameTypeTerraria {
		difficulty := reqs["difficulty"]
		if difficulty == "" {
			state.State = models.StateClarifyingRequirements
			question := models.StructuredQuestion{
				ID:          "q_terraria_difficulty",
				Key:         "difficulty",
				Title:       "Which Terraria difficulty would you like for your world?",
				Description: "Affects enemy damage, health, and exclusive boss loot drops.",
				Type:        "single_choice",
				Required:    true,
				Options: []models.StructuredQuestionOption{
					{ID: "classic", Label: "Classic", Value: "classic", Description: "Standard Terraria combat and monster stats", IsDefault: true},
					{ID: "expert", Label: "Expert", Value: "expert", Description: "Tougher enemies with unique boss accessories and AI"},
					{ID: "master", Label: "Master", Value: "master", Description: "Ultimate challenge with exclusive relic and pet trophies"},
				},
			}
			return question.Title, models.UIComponentsPayload{
				Questions: []models.StructuredQuestion{question},
			}, nil, ""
		}

		worldSize := reqs["world_size"]
		if worldSize == "" {
			state.State = models.StateClarifyingRequirements
			question := models.StructuredQuestion{
				ID:          "q_terraria_world_size",
				Key:         "world_size",
				Title:       "What world size would you like to generate?",
				Description: "Select the physical boundaries and exploration size of your Terraria map.",
				Type:        "single_choice",
				Required:    true,
				Options: []models.StructuredQuestionOption{
					{ID: "medium", Label: "Medium World (Recommended)", Value: "medium", Description: "Balanced world size with plenty of biomes", IsDefault: true},
					{ID: "large", Label: "Large World", Value: "large", Description: "Vast expanses, abundant sky islands and subterranean caverns"},
					{ID: "small", Label: "Small World", Value: "small", Description: "Quick to explore, ideal for 2 to 4 players"},
				},
			}
			return question.Title, models.UIComponentsPayload{
				Questions: []models.StructuredQuestion{question},
			}, nil, ""
		}
	}

	// Step 3: Determine Sizing & Plan
	playerSlots := 10
	if reqs["player_slots"] != "" {
		fmt.Sscanf(reqs["player_slots"], "%d", &playerSlots)
		if playerSlots <= 0 {
			playerSlots = 10
		}
	}

	planName := "basic"
	vcpus := 2
	memoryMB := 4096
	diskGB := 20

	if playerSlots <= 5 {
		planName = "starter"
		vcpus = 1
		memoryMB = 2048
		diskGB = 10
	} else if playerSlots > 20 || reqs["server_software"] == models.SoftwareForge {
		planName = "advanced"
		vcpus = 4
		memoryMB = 8192
		diskGB = 80
	} else if playerSlots > 10 || reqs["server_software"] == models.SoftwareFabric || reqs["server_software"] == models.SoftwareTModLoader {
		planName = "standard"
		vcpus = 3
		memoryMB = 6144
		diskGB = 40
	}

	serverName := reqs["name"]
	if serverName == "" {
		if gameType == models.GameTypeMinecraftJava {
			serverName = fmt.Sprintf("minecraft-%s-%d", reqs["server_software"], time.Now().Unix()%1000)
		} else if gameType == models.GameTypeMinecraftBedrock {
			serverName = fmt.Sprintf("bedrock-%d", time.Now().Unix()%1000)
		} else {
			serverName = fmt.Sprintf("terraria-%s-%d", reqs["difficulty"], time.Now().Unix()%1000)
		}
		reqs["name"] = serverName
	}

	gameVersion := "1.21.4"
	if gameType == models.GameTypeMinecraftBedrock {
		gameVersion = "1.21.51"
	} else if gameType == models.GameTypeTerraria {
		gameVersion = "1.4.4.9"
	}

	software := reqs["server_software"]
	if software == "" {
		if gameType == models.GameTypeTerraria {
			software = models.SoftwareTerraria
		} else if gameType == models.GameTypeMinecraftBedrock {
			software = models.SoftwareBedrock
		} else {
			software = models.SoftwarePaper
		}
	}

	difficulty := reqs["difficulty"]
	if difficulty == "" {
		difficulty = "normal"
	}

	planTitle := fmt.Sprintf("Provision Dedicated %s Game Server (%s)", strings.ReplaceAll(gameType, "_", " "), serverName)
	planSummary := fmt.Sprintf("Fully managed multiplayer %s server (%s v%s) with persistent ZFS storage, dynamic port allocation, and daily backups.", strings.ReplaceAll(gameType, "_", " "), software, gameVersion)

	planView := &models.StructuredPlanView{
		Title:                planTitle,
		Intent:               "game_server",
		Summary:              planSummary,
		Reversible:           true,
		RequiresConfirmation: true,
		Status:               "awaiting_confirmation",
		EstimatedAllocations: map[string]interface{}{
			"vcpus":     vcpus,
			"memory_mb": memoryMB,
			"disk_gb":   diskGB,
		},
		Resources: []models.StructuredResourceView{
			{
				Type:    "Game Server",
				Name:    serverName,
				Summary: fmt.Sprintf("%s (%s v%s)", strings.ReplaceAll(gameType, "_", " "), software, gameVersion),
				Details: map[string]interface{}{
					"Game Type":       strings.ReplaceAll(gameType, "_", " "),
					"Server Software": software,
					"Version":         gameVersion,
					"Plan Tier":       planName,
					"Player Slots":    playerSlots,
					"Difficulty":      difficulty,
					"Port":            "Assigned dynamically upon provisioning",
					"Backups":         "Automated Daily Snapshot",
				},
			},
		},
	}

	spec := orchestrator.Specification{Intent: "game_server", Resources: []orchestrator.ResourceSpec{{
		Key: serverName, Type: orchestrator.TypeGameServer, Action: orchestrator.ActionCreate,
		Config: map[string]any{
			"name": serverName, "game_type": gameType, "game_version": gameVersion, "server_software": software,
			"plan_name": planName, "cpu_cores": vcpus, "memory_mb": memoryMB, "storage_gb": diskGB,
			"player_slots": playerSlots, "difficulty": difficulty, "world_size": reqs["world_size"],
			"motd": fmt.Sprintf("Nevatal Cloud | %s Multiplayer", serverName),
		},
	}}}
	card, planID, err := s.proposePlan(ctx, convID, userID, state, "game_server", planTitle, planSummary, `{"monthly_estimate": "$10.00 / month", "included_bandwidth": "Unmetered 1Gbps"}`, spec, planView)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	reply := fmt.Sprintf("I have prepared the deployment proposal for your **%s** multiplayer server (**`%s`**). Review the plan below and click **Approve and Provision** to deploy your server!", strings.ReplaceAll(gameType, "_", " "), serverName)
	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, planID
}

func (s *AIAgentUsecase) handleAndroidWorkflow(
	ctx context.Context,
	convID uuid.UUID,
	userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
	quota *models.QuotaUsage,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	// Step 1: Select Image / Android Version if not specified
	imageID := reqs["image_id"]
	if imageID == "" && reqs["image_slug"] != "" {
		if img, err := s.repos.Android.FindImageBySlug(ctx, reqs["image_slug"]); err == nil {
			imageID = img.ID.String()
			reqs["image_id"] = imageID
		}
	}

	if imageID == "" {
		images, _ := s.androidService.ListImages(ctx, true)
		var options []models.StructuredQuestionOption
		for i, img := range images {
			isDefault := i == 0
			options = append(options, models.StructuredQuestionOption{
				ID:          img.ID.String(),
				Label:       fmt.Sprintf("%s (Android %s)", img.Name, img.AndroidVersion),
				Value:       img.ID.String(),
				Description: img.Description,
				IsDefault:   isDefault,
			})
		}
		if len(options) == 0 {
			options = []models.StructuredQuestionOption{
				{ID: "lineageos-20", Label: "LineageOS 20 (Android 13) - Recommended", Value: "lineageos-20", Description: "Modern Android 13 runtime for apps & tests", IsDefault: true},
				{ID: "lineageos-18", Label: "LineageOS 18.1 (Android 11)", Value: "lineageos-18", Description: "Legacy Android 11 for QA testing"},
				{ID: "android-12-dev", Label: "Android 12 Developer Preview", Value: "android-12-dev", Description: "Root-enabled developer build with pre-configured ADB"},
			}
		}

		state.State = models.StateClarifyingRequirements
		question := models.StructuredQuestion{
			ID:          "q_android_image",
			Key:         "image_id",
			Title:       "Which Android OS version would you like to run in Waydroid?",
			Description: "Select a containerized Android system image optimized for cloud desktop and ADB testing.",
			Type:        "single_choice",
			Required:    true,
			Options:     options,
		}
		return question.Title, models.UIComponentsPayload{
			Questions: []models.StructuredQuestion{question},
		}, nil, ""
	}

	// Step 2: Sizing Plan
	planName := reqs["plan"]
	if planName == "" {
		state.State = models.StateClarifyingRequirements
		question := models.StructuredQuestion{
			ID:          "q_android_plan",
			Key:         "plan",
			Title:       "What hardware sizing tier fits your Android workload?",
			Description: "Allocate virtual CPU, RAM, and dedicated user storage.",
			Type:        "single_choice",
			Required:    true,
			Options: []models.StructuredQuestionOption{
				{ID: "developer", Label: "Developer (Recommended) — 2 vCPU, 4 GB RAM, 32 GB Storage", Value: "developer", Description: "Optimal for app debugging, continuous testing, and daily emulation", IsDefault: true},
				{ID: "standard", Label: "Standard — 2 vCPU, 3 GB RAM, 24 GB Storage", Value: "standard", Description: "Lightweight app usage and web browsing"},
				{ID: "power", Label: "Power — 4 vCPU, 8 GB RAM, 64 GB Storage", Value: "power", Description: "High performance for complex Android apps and games"},
				{ID: "testing", Label: "Testing / QA — 1 vCPU, 2 GB RAM, 16 GB Storage", Value: "testing", Description: "Minimal container for CI/CD test automation pipelines"},
			},
		}
		return question.Title, models.UIComponentsPayload{
			Questions: []models.StructuredQuestion{question},
		}, nil, ""
	}

	// Step 3: Display & Remote Access Resolution
	resolution := reqs["display_resolution"]
	if resolution == "" {
		state.State = models.StateClarifyingRequirements
		question := models.StructuredQuestion{
			ID:          "q_android_resolution",
			Key:         "display_resolution",
			Title:       "Select the virtual screen resolution and orientation:",
			Description: "Defines the virtual framebuffer streamed to your browser via WebRTC/canvas.",
			Type:        "single_choice",
			Required:    true,
			Options: []models.StructuredQuestionOption{
				{ID: "phone_portrait", Label: "Phone Portrait (720 × 1280, 240 DPI)", Value: "720x1280", Description: "Standard smartphone aspect ratio", IsDefault: true},
				{ID: "tablet_landscape", Label: "Tablet Landscape (1280 × 720, 240 DPI)", Value: "1280x720", Description: "Widescreen dashboard or tablet orientation"},
				{ID: "fhd_portrait", Label: "Full HD Portrait (1080 × 1920, 320 DPI)", Value: "1080x1920", Description: "High-DPI smartphone display"},
				{ID: "fhd_landscape", Label: "Full HD Landscape (1920 × 1080, 320 DPI)", Value: "1920x1080", Description: "Full HD desktop tablet viewport"},
			},
		}
		return question.Title, models.UIComponentsPayload{
			Questions: []models.StructuredQuestion{question},
		}, nil, ""
	}

	// Build Proposal
	instanceName := reqs["name"]
	if instanceName == "" {
		instanceName = fmt.Sprintf("android-box-%d", time.Now().Unix()%1000)
		reqs["name"] = instanceName
	}

	vcpus := 2
	memoryMB := 4096
	storageGB := 32

	switch planName {
	case "testing":
		vcpus, memoryMB, storageGB = 1, 2048, 16
	case "standard":
		vcpus, memoryMB, storageGB = 2, 3072, 24
	case "developer":
		vcpus, memoryMB, storageGB = 2, 4096, 32
	case "power":
		vcpus, memoryMB, storageGB = 4, 8192, 64
	}

	density := 240
	if strings.Contains(resolution, "1920") {
		density = 320
	}
	orientation := "portrait"
	if strings.HasPrefix(resolution, "1280") || strings.HasPrefix(resolution, "1920") {
		orientation = "landscape"
	}

	adbEnabled := true
	if reqs["adb_enabled"] == "false" {
		adbEnabled = false
	}

	planTitle := fmt.Sprintf("Provision Waydroid Android Instance (%s)", instanceName)
	planSummary := fmt.Sprintf("Containerized Waydroid Android environment (%s tier: %d vCPU, %d MB RAM, %d GB Storage) with browser-based display stream and ADB network bridging.", strings.Title(planName), vcpus, memoryMB, storageGB)

	planView := &models.StructuredPlanView{
		Title:                planTitle,
		Intent:               "android_instance",
		Summary:              planSummary,
		Reversible:           true,
		RequiresConfirmation: true,
		Status:               "awaiting_confirmation",
		EstimatedAllocations: map[string]interface{}{
			"vcpus":               vcpus,
			"memory_mb":           memoryMB,
			"storage_gb":          storageGB,
			"display_resolution":  resolution,
			"display_density_dpi": density,
			"orientation":         orientation,
			"adb_enabled":         adbEnabled,
			"remote_access":       "Browser WebRTC / Canvas Display",
		},
	}

	spec := orchestrator.Specification{Intent: "android_instance", Resources: []orchestrator.ResourceSpec{{
		Key: instanceName, Type: orchestrator.TypeAndroid, Action: orchestrator.ActionCreate,
		Config: map[string]any{
			"name": instanceName, "image_id": imageID, "plan_name": planName, "vcpus": vcpus, "memory_mb": memoryMB,
			"storage_gb": storageGB, "display_resolution": resolution, "display_density": density,
			"orientation": orientation, "adb_enabled": adbEnabled,
		},
	}}}
	card, planID, err := s.proposePlan(ctx, convID, userID, state, "android_instance", planTitle, planSummary, `{"monthly_estimate": "$12.00 / month", "included_bandwidth": "Unmetered"}`, spec, planView)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	reply := fmt.Sprintf("I have prepared the deployment proposal for your **Waydroid Android Instance** (**`%s`**). Review the hardware and display configuration below, then click **Approve and Provision** to launch your cloud Android device!", instanceName)
	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, planID
}

// proposePlan stores a planner proposal through the orchestrator: a new
// revision of the conversation's open plan when it has the same intent and
// can still be edited, otherwise a new plan. It returns the chat card (with
// the plan id and version the approve call binds to).
func (s *AIAgentUsecase) proposePlan(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	intent, title, summary, costs string,
	spec orchestrator.Specification,
	view *models.StructuredPlanView,
) (*models.StructuredPlanView, string, error) {
	if s.orch == nil {
		return nil, "", errors.New("the orchestrator is not available")
	}
	if state.ActivePlanID != nil {
		if plan, rev, revised, err := s.orch.RevisePlan(ctx, *state.ActivePlanID, userID, intent, spec); err != nil {
			return nil, "", err
		} else if revised {
			state.State = models.StateAwaitingConfirmation
			return planCard(plan, rev, view), plan.ID.String(), nil
		}
	}
	plan, rev, err := s.orch.CreatePlan(ctx, PlanDraft{
		ConversationID: convID, UserID: userID, Intent: intent, Title: title, Description: summary,
		EstimatedCosts: costs, Spec: spec,
	})
	if err != nil {
		return nil, "", err
	}
	state.State = models.StateAwaitingConfirmation
	state.PendingQuestions = "[]"
	state.ActivePlanID = &plan.ID
	return planCard(plan, rev, view), plan.ID.String(), nil
}

func planError(err error) string {
	return "I could not prepare a plan for this request: " + err.Error()
}

// validationNote tells the user when the policy engine rejected the plan.
func validationNote(card *models.StructuredPlanView) string {
	if card == nil || card.Status != models.PlanStatusDraft {
		return ""
	}
	return "\n\n⚠️ **This plan does not pass policy validation yet** (see the plan's validation errors). Adjust the request or edit the plan before approving it."
}

// handleDeleteVPSWorkflow proposes a destructive plan that deletes one of the
// user's VPS. Approval requires acknowledging the risk and typing the VPS
// name; nothing is deleted from chat.
func (s *AIAgentUsecase) handleDeleteVPSWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
	msg string,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	instances, _ := s.instanceService.List(ctx, userID, false, "")
	var target *models.Instance
	if id := reqs["target_instance_id"]; id != "" {
		for i := range instances {
			if instances[i].ID.String() == id {
				target = &instances[i]
			}
		}
	}
	if target == nil {
		// Longest name mentioned in the message wins ("web-01" vs "web-010").
		for i := range instances {
			name := strings.ToLower(instances[i].Name)
			if name != "" && strings.Contains(msg, name) && (target == nil || len(name) > len(target.Name)) {
				target = &instances[i]
			}
		}
	}
	if target == nil {
		if len(instances) == 0 {
			state.State = models.StateCompleted
			state.CurrentIntent = ""
			return "You have no virtual machines to delete.", models.UIComponentsPayload{}, nil, ""
		}
		options := make([]models.StructuredQuestionOption, 0, len(instances))
		for _, inst := range instances {
			options = append(options, models.StructuredQuestionOption{
				ID: inst.ID.String(), Label: inst.Name, Value: inst.ID.String(),
				Description: fmt.Sprintf("%s · %d vCPU · %d MB", inst.Status, inst.VCPUs, inst.MemoryMB),
			})
		}
		q := models.StructuredQuestion{
			ID: "q_delete_target", Key: "target_instance_id", Title: "Which virtual machine should be deleted?",
			Description: "Deleting a VM destroys its disks. You will confirm by typing its name.",
			Type:        "single_choice", Options: options, Required: true,
		}
		state.State = models.StateClarifyingRequirements
		qBytes, _ := json.Marshal([]models.StructuredQuestion{q})
		state.PendingQuestions = string(qBytes)
		return q.Title, models.UIComponentsPayload{Questions: []models.StructuredQuestion{q}}, nil, ""
	}
	reqs["target_instance_id"] = target.ID.String()
	spec := orchestrator.Specification{Intent: "delete_vps", Resources: []orchestrator.ResourceSpec{{
		Key: target.Name, Type: orchestrator.TypeVPS, Action: orchestrator.ActionDelete,
		Config: map[string]any{"instance_id": target.ID.String(), "name": target.Name},
	}}}
	title := fmt.Sprintf("Delete VPS %s", target.Name)
	view := &models.StructuredPlanView{
		Title: title, Intent: "delete_vps", Summary: "Permanently delete the virtual machine and its disks.",
		Reversible: false,
		Resources: []models.StructuredResourceView{{
			Type: "Virtual Machine", Name: target.Name, Summary: "Delete (destructive)", Order: 1, Status: "pending",
			Details: map[string]any{"Status": target.Status, "vCPU": target.VCPUs, "Memory (MB)": target.MemoryMB, "Disk (GB)": target.DiskGB},
		}},
	}
	card, planID, err := s.proposePlan(ctx, convID, userID, state, "delete_vps", title, view.Summary, "", spec, view)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	reply := fmt.Sprintf("⚠️ This is a **destructive** plan: it permanently deletes `%s` and its disks. To approve it you must acknowledge the risk and type the VM name `%s` exactly. Nothing is deleted until you do.", target.Name, target.Name)
	return reply + validationNote(card), models.UIComponentsPayload{PlanProposal: card}, card, planID
}

// Plans are approved and executed only through OrchestratorUsecase
// (POST /ai/plans/:id/approve): no chat message, answer chip or MCP call can
// approve a plan.

// Helpers for Intelligent Dynamic Parsing

func detectIntent(msg string) string {
	if containsAny(msg, "delete ", "destroy ", "remove ", "terminate ") &&
		containsAny(msg, "vps", " vm ", " vm", "virtual machine") &&
		!containsAny(msg, "game", "minecraft", "terraria", "android", "bucket", "n8n", "openclaw") {
		return "delete_vps"
	}
	if isTemplateRequest(msg) {
		return "deploy_template"
	}
	if isDatabaseRequest(msg) {
		return "create_database"
	}
	if isAdvisorRequest(msg) {
		return "advisor"
	}
	if strings.Contains(msg, "android") || strings.Contains(msg, "waydroid") ||
		strings.Contains(msg, "apk") || strings.Contains(msg, "lineageos") ||
		strings.Contains(msg, "mobile app") || strings.Contains(msg, "android cloud") {
		return "android_instance"
	}
	if strings.Contains(msg, "minecraft") || strings.Contains(msg, "terraria") ||
		strings.Contains(msg, "game server") || strings.Contains(msg, "game") ||
		strings.Contains(msg, "paper") || strings.Contains(msg, "bedrock") ||
		strings.Contains(msg, "fabric") || strings.Contains(msg, "forge") ||
		strings.Contains(msg, "purpur") || strings.Contains(msg, "tmodloader") {
		return "game_server"
	}
	if strings.Contains(msg, "openclaw") || strings.Contains(msg, "agent") || strings.Contains(msg, "ai agent") {
		return "openclaw_instance"
	}
	if strings.Contains(msg, "n8n") || strings.Contains(msg, "workflow") || strings.Contains(msg, "automation") {
		return "n8n_instance"
	}
	if strings.Contains(msg, "s3") || strings.Contains(msg, "bucket") || strings.Contains(msg, "minio") {
		return "create_storage"
	}
	if strings.Contains(msg, "troubleshoot") || strings.Contains(msg, "diagnose") || strings.Contains(msg, "error") ||
		strings.Contains(msg, "stopped") || strings.Contains(msg, "failed") || strings.Contains(msg, "failing") ||
		strings.Contains(msg, "debug") || strings.Contains(msg, "unhealthy") || strings.Contains(msg, "not working") ||
		strings.Contains(msg, "broken") || strings.Contains(msg, "is down") || strings.Contains(msg, " down") ||
		strings.Contains(msg, "health check") {
		return "troubleshoot"
	}
	if strings.Contains(msg, "website") || strings.Contains(msg, "github") ||
		strings.Contains(msg, "git") || strings.Contains(msg, "react") || strings.Contains(msg, "nextjs") {
		return "deploy_website"
	}
	if strings.Contains(msg, "vps") || strings.Contains(msg, "vm") || strings.Contains(msg, "virtual machine") ||
		strings.Contains(msg, "ubuntu") || strings.Contains(msg, "debian") || strings.Contains(msg, "alpine") ||
		strings.Contains(msg, "linux") || strings.Contains(msg, "server") {
		return "create_vps"
	}
	if strings.Contains(msg, "storage") {
		return "create_storage"
	}
	if strings.Contains(msg, "deploy") || strings.Contains(msg, "app") {
		return "deploy_website"
	}
	return "create_vps"
}

func extractRequirementsFromText(
	msg string,
	reqs map[string]string,
	templates []models.OSTemplate,
	sshKeys []models.SSHKey,
) {
	// Detect OS
	for _, t := range templates {
		distLower := strings.ToLower(t.Distribution)
		nameLower := strings.ToLower(t.Name)
		if strings.Contains(msg, distLower) || strings.Contains(msg, nameLower) {
			reqs["os_template_id"] = t.ID.String()
			break
		}
	}

	// Detect Sizing
	if strings.Contains(msg, "nano") || strings.Contains(msg, "1gb") || strings.Contains(msg, "micro") {
		reqs["specs"] = "1:1024:20"
	} else if strings.Contains(msg, "large") || strings.Contains(msg, "8gb") || strings.Contains(msg, "power") {
		reqs["specs"] = "4:8192:80"
	} else if strings.Contains(msg, "4gb") || strings.Contains(msg, "standard") || strings.Contains(msg, "medium") {
		reqs["specs"] = "2:4096:40"
	}

	// Detect IP preference
	if strings.Contains(msg, "public ip") || strings.Contains(msg, "ipv4") {
		reqs["public_networking"] = "true"
	} else if strings.Contains(msg, "private only") || strings.Contains(msg, "private vpc") {
		reqs["public_networking"] = "false"
	}

	// Explicit resource name: "... named web-01" / "... called web-01"
	namedRegex := regexp.MustCompile(`\b(?:named|called)\s+["'` + "`" + `]?([a-z0-9][a-z0-9\-\.]{1,62}[a-z0-9])`)
	if match := namedRegex.FindStringSubmatch(msg); len(match) > 1 {
		reqs["name"] = match[1]
	}

	// Detect SSH Key
	if len(sshKeys) > 0 {
		reqs["ssh_key_id"] = sshKeys[0].ID.String()
	}

	// Extract Git URL if present
	gitURLRegex := regexp.MustCompile(`https?://[^\s]+\.git|https?://github\.com/[^\s]+|https?://gitlab\.com/[^\s]+`)
	if match := gitURLRegex.FindString(msg); match != "" {
		reqs["git_repo"] = match
	}

	// Detect Game Server preferences per PRD Section 57.1 & 57.2
	if strings.Contains(msg, "minecraft") {
		if strings.Contains(msg, "bedrock") {
			reqs["game_type"] = models.GameTypeMinecraftBedrock
		} else {
			reqs["game_type"] = models.GameTypeMinecraftJava
		}
	} else if strings.Contains(msg, "terraria") {
		reqs["game_type"] = models.GameTypeTerraria
	}

	if strings.Contains(msg, "paper") {
		reqs["server_software"] = models.SoftwarePaper
	} else if strings.Contains(msg, "fabric") {
		reqs["server_software"] = models.SoftwareFabric
	} else if strings.Contains(msg, "forge") {
		reqs["server_software"] = models.SoftwareForge
	} else if strings.Contains(msg, "purpur") {
		reqs["server_software"] = models.SoftwarePurpur
	} else if strings.Contains(msg, "vanilla") {
		reqs["server_software"] = models.SoftwareVanilla
	} else if strings.Contains(msg, "tmodloader") {
		reqs["server_software"] = models.SoftwareTModLoader
	}

	if strings.Contains(msg, "survival") {
		reqs["gamemode"] = "survival"
	} else if strings.Contains(msg, "creative") {
		reqs["gamemode"] = "creative"
	}

	if strings.Contains(msg, "expert") {
		reqs["difficulty"] = "expert"
	} else if strings.Contains(msg, "master") {
		reqs["difficulty"] = "master"
	} else if strings.Contains(msg, "classic") {
		reqs["difficulty"] = "classic"
	} else if strings.Contains(msg, "hard") {
		reqs["difficulty"] = "hard"
	} else if strings.Contains(msg, "normal") {
		reqs["difficulty"] = "normal"
	} else if strings.Contains(msg, "peaceful") {
		reqs["difficulty"] = "peaceful"
	}

	if strings.Contains(msg, "large") {
		reqs["world_size"] = "large"
	} else if strings.Contains(msg, "medium") {
		reqs["world_size"] = "medium"
	} else if strings.Contains(msg, "small") {
		reqs["world_size"] = "small"
	}

	// Player numbers e.g. "10 friends", "10 players", "6 friends"
	playerRegex := regexp.MustCompile(`(\d+)\s*(?:players?|friends?|slots?)`)
	if match := playerRegex.FindStringSubmatch(msg); len(match) > 1 {
		reqs["player_slots"] = match[1]
	}

	// Detect Android Waydroid preferences
	if strings.Contains(msg, "lineageos 20") || strings.Contains(msg, "android 13") {
		reqs["image_slug"] = "lineageos-20-android-13"
	} else if strings.Contains(msg, "lineageos 18") || strings.Contains(msg, "android 11") {
		reqs["image_slug"] = "lineageos-18-android-11"
	} else if strings.Contains(msg, "developer") && (strings.Contains(msg, "android") || strings.Contains(msg, "waydroid")) {
		reqs["image_slug"] = "android-12-developer"
	}

	if strings.Contains(msg, "power") {
		reqs["plan"] = "power"
	} else if strings.Contains(msg, "testing") {
		reqs["plan"] = "testing"
	}
}

func (s *AIAgentUsecase) handleGeneralQuery(msg string, quota *models.QuotaUsage, templates []models.OSTemplate) string {
	usedVCPUs, maxVCPUs := 0, 4
	if quota != nil {
		usedVCPUs = quota.UsedVCPUs
		maxVCPUs = quota.MaxVCPUs
	}
	return fmt.Sprintf(`### ☁️ Nevatal AI Cloud Agent
I am ready to help you manage and deploy services on our Dell PowerEdge R640 enterprise virtualization cluster.

**Your Quota Status:**
- %d / %d vCPU Cores currently allocated
- %d available OS images ready for deployment

**Tell me what you'd like to build or check:**
- *"Create an Ubuntu 24.04 VPS with 4GB RAM for my Go API"*
- *"Deploy my GitHub repository to a live web server"*
- *"Create a PostgreSQL database named appdb"*
- *"Deploy the web-stack template"*
- *"Diagnose web-01"* or *"health check all resources"*
- *"Optimize my costs"*
- *"Set up an S3 storage bucket for backups"*`, usedVCPUs, maxVCPUs, len(templates))
}

// Read requests

// readActionWords mark a message that asks for a change: it is never a
// read request, whatever else it says.
var readActionWords = []string{
	"create", "deploy", "launch", "provision", "set up", "setup", "spin up", "build", "install",
	"start", "stop", "reboot", "delete", "destroy", "remove", "terminate", "resize", "scale",
	"upgrade", "rename", "snapshot", "restore", "new ", "i want", "i need", "i'd like", "make a", "make me", "host my",
}

// readQuestionWords mark a question about existing resources.
var readQuestionWords = []string{
	"list", "show", "status", "what is", "what's", "whats", "what are", "which", "how many",
	"do i have", "display", "view ", "see my", "check my", "is my", "are my", "describe",
	"details", "info about", "information about", "get my", "my servers", "my vps", "my instances",
	"my vms", "my machines", "running?",
}

// readTroubleWords mark a troubleshooting question (handled by the
// troubleshooting answer, not as a listing).
var readTroubleWords = []string{"why", "error", "fail", "troubleshoot", "debug", "broken", "not working", "can't", "cannot"}

// isReadRequest reports whether a message only asks to see existing
// resources or their status: it has no create/change verb and no
// troubleshooting word, and it either asks a list/show/status question or
// names one of the caller's instances ("is web-01 up"). ownedNames is only
// called in that last case.
func isReadRequest(msg string, ownedNames func() []string) bool {
	if containsAny(msg, readActionWords...) || containsAny(msg, readTroubleWords...) {
		return false
	}
	if containsAny(msg, readQuestionWords...) {
		return true
	}
	if ownedNames != nil {
		for _, name := range ownedNames() {
			if mentionsName(msg, name) {
				return true
			}
		}
	}
	return false
}

// instanceNames returns the names of the caller's instances.
func (s *AIAgentUsecase) instanceNames(ctx context.Context, userID uuid.UUID) []string {
	if s.instanceService == nil {
		return nil
	}
	list, err := s.instanceService.List(ctx, userID, false, "")
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(list))
	for _, inst := range list {
		names = append(names, inst.Name)
	}
	return names
}

// mentionsName reports whether msg (lower case) contains name as a whole
// word: "web-01" matches "is web-01 up?" but not "web-011".
func mentionsName(msg, name string) bool {
	name = strings.ToLower(name)
	if name == "" {
		return false
	}
	isNameChar := func(c byte) bool {
		return c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_'
	}
	for i := 0; i < len(msg); {
		j := strings.Index(msg[i:], name)
		if j < 0 {
			return false
		}
		j += i
		end := j + len(name)
		if (j == 0 || !isNameChar(msg[j-1])) && (end == len(msg) || !isNameChar(msg[end])) {
			return true
		}
		i = j + 1
	}
	return false
}

// answerReadRequest answers a read request with the registry's read tools
// (vps.list, vps.status) or, for other resource types, a read-only pointer
// to their page. It never proposes or creates anything.
func (s *AIAgentUsecase) answerReadRequest(ctx context.Context, userID uuid.UUID, msg string, quota *models.QuotaUsage, clientIP string) string {
	const footer = "\n\n_This is a read-only answer: nothing was planned or changed. Ask me to create or change something to start a plan._"
	switch {
	case containsAny(msg, "quota", "capacity", "limit", "allocation"):
		if quota == nil {
			return "I could not read your quota right now. See [Usage & Quotas](/dashboard/usage)." + footer
		}
		return fmt.Sprintf("### Your quota\n\n- vCPU: %d / %d\n- Memory: %s / %s\n- Disk: %d / %d GB\n- Instances: %d / %d",
			quota.UsedVCPUs, quota.MaxVCPUs, formatMB(quota.UsedMemoryMB), formatMB(quota.MaxMemoryMB),
			quota.UsedStorageGB, quota.MaxStorageGB, quota.UsedInstances, quota.MaxInstances) + footer
	case containsAny(msg, "cost", "spend", "bill", "savings", "optimize", "advisor"):
		return "Your estimated costs and optimization advice are on [Operations · Advisor](/dashboard/operations?tab=advisor)." + footer
	case containsAny(msg, "database", "postgres", "mysql", "redis", "mongo"):
		return "Your managed databases are listed under [Operations · Databases](/dashboard/operations?tab=databases)." + footer
	case containsAny(msg, "template"):
		return "Your infrastructure templates are listed under [Operations · Templates](/dashboard/operations?tab=templates)." + footer
	case containsAny(msg, "bucket", "storage", "s3", "minio"):
		return "Your buckets are listed under [Object Storage](/dashboard/storage)." + footer
	case containsAny(msg, "game", "minecraft", "terraria"):
		return "Your game servers are listed under [Game Servers](/dashboard/game-servers)." + footer
	case containsAny(msg, "android", "waydroid"):
		return "Your Android instances are listed under [Android Cloud](/dashboard/android)." + footer
	case containsAny(msg, "n8n", "automation", "workflow"):
		return "Your n8n instances are listed under [Automation](/dashboard/automation)." + footer
	case containsAny(msg, "openclaw", "agent"):
		return "Your OpenClaw agents are listed under [AI Agents](/dashboard/ai-agents)." + footer
	case containsAny(msg, "website", "app ", "apps", "application"):
		return "Your applications are listed under [Apps](/dashboard/apps)." + footer
	}

	if s.orch == nil {
		return "I can't read your instances right now. See [Instances](/dashboard/instances)." + footer
	}
	res, err := s.orch.CallTool(ctx, userID, false, orchestrator.ToolVPSList, map[string]any{}, clientIP)
	if err != nil {
		return "I could not list your instances right now (" + err.Error() + "). See [Instances](/dashboard/instances)." + footer
	}
	data, _ := res.Data.(map[string]any)
	items, _ := data["instances"].([]map[string]any)

	// A named instance: report its status (vps.status).
	for _, it := range items {
		name, _ := it["name"].(string)
		id, _ := it["instance_id"].(string)
		if !mentionsName(msg, name) {
			continue
		}
		st, err := s.orch.CallTool(ctx, userID, false, orchestrator.ToolVPSStatus, map[string]any{"instance_id": id}, clientIP)
		if err != nil {
			return fmt.Sprintf("I could not read the status of **%s** (%s)."+footer, name, err.Error())
		}
		out, _ := st.Data.(map[string]any)
		return fmt.Sprintf("### %s\n\n- Status: **%v** (hypervisor: %v)\n- Size: %v vCPU, %v MB RAM, %v GB disk\n- Private IP: %v",
			name, out["status"], out["hypervisor_status"], out["vcpus"], out["memory_mb"], out["disk_gb"], valueOr(out["private_ip"], "none yet")) + footer
	}

	if len(items) == 0 {
		return "You have no VPS instances yet." + footer
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### Your VPS instances (%d)\n\n| Name | Status | Size | Private IP |\n|---|---|---|---|\n", len(items))
	for _, it := range items {
		fmt.Fprintf(&b, "| %v | %v | %v vCPU / %v MB / %v GB | %v |\n",
			it["name"], it["status"], it["vcpus"], it["memory_mb"], it["disk_gb"], valueOr(it["private_ip"], "-"))
	}
	return strings.TrimRight(b.String(), "\n") + footer
}

func valueOr(v any, fallback string) any {
	if v == nil {
		return fallback
	}
	if s, ok := v.(string); ok && s == "" {
		return fallback
	}
	return v
}

func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
