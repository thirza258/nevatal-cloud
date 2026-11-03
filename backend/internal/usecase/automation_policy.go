package usecase

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/repository"
)

// PlanProposer creates a plan from a template. It is implemented by
// OrchestratorUsecase; policies never execute infrastructure changes.
type PlanProposer interface {
	DeployTemplate(ctx context.Context, userID uuid.UUID, isAdmin bool, templateID uuid.UUID, params map[string]any) (*models.AIResourcePlan, error)
}

// PolicyEvent is the sanitized event a policy is evaluated against.
type PolicyEvent struct {
	Trigger      string
	Severity     string
	Action       string
	ResourceType string
	ResourceID   string
	ResourceName string
	CheckType    string
	Category     string
	Risk         string
	Message      string
	ExecutionID  string
	PlanID       string
	TemplateID   string
	CheckID      string
	// RecommendationID identifies one advisor recommendation. It is empty
	// for every other trigger and makes the notification dedupe key stable
	// across advisor runs, so a finding notifies at most once.
	RecommendationID string
}

// AutomationPolicyUsecase manages and evaluates deterministic automation
// policies. Supported actions are in-app notifications and plan proposals.
type AutomationPolicyUsecase struct {
	repo          repository.AutomationPolicyRepository
	notifications *NotificationUsecase
	plans         PlanProposer
	audit         *AuditUsecase
}

func NewAutomationPolicyUsecase(repo repository.AutomationPolicyRepository, notifications *NotificationUsecase, audit *AuditUsecase) *AutomationPolicyUsecase {
	return &AutomationPolicyUsecase{repo: repo, notifications: notifications, audit: audit}
}

// SetPlans sets the plan proposer (built after the policy use case to avoid
// a construction cycle).
func (s *AutomationPolicyUsecase) SetPlans(p PlanProposer) { s.plans = p }

// PolicyView is the API view of a policy.
type PolicyView struct {
	ID              uuid.UUID         `json:"id"`
	Name            string            `json:"name"`
	Trigger         string            `json:"trigger"`
	Conditions      map[string]string `json:"conditions"`
	Action          string            `json:"action"`
	ActionConfig    map[string]any    `json:"action_config"`
	Enabled         bool              `json:"enabled"`
	LastTriggeredAt *time.Time        `json:"last_triggered_at"`
	TriggerCount    int64             `json:"trigger_count"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// CreatePolicyInput is the body of POST /ai/policies.
type CreatePolicyInput struct {
	Name         string            `json:"name"`
	Trigger      string            `json:"trigger"`
	Conditions   map[string]string `json:"conditions"`
	Action       string            `json:"action"`
	ActionConfig map[string]any    `json:"action_config"`
	Enabled      *bool             `json:"enabled"`
}

func decodePolicyView(p *models.AutomationPolicy) PolicyView {
	v := PolicyView{
		ID: p.ID, Name: p.Name, Trigger: p.Trigger, Action: p.Action, Enabled: p.Enabled,
		LastTriggeredAt: p.LastTriggeredAt, TriggerCount: p.TriggerCount, CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
		Conditions: map[string]string{}, ActionConfig: map[string]any{},
	}
	_ = json.Unmarshal([]byte(p.Conditions), &v.Conditions)
	_ = json.Unmarshal([]byte(p.ActionConfig), &v.ActionConfig)
	if v.Conditions == nil {
		v.Conditions = map[string]string{}
	}
	if v.ActionConfig == nil {
		v.ActionConfig = map[string]any{}
	}
	return v
}

func validPolicyTrigger(trigger string) bool {
	switch trigger {
	case models.PolicyTriggerExecutionFailed, models.PolicyTriggerExecutionCompleted,
		models.PolicyTriggerCheckFailed, models.PolicyTriggerAdvisor:
		return true
	}
	return false
}

func validPolicyAction(action string) bool {
	return action == models.PolicyActionNotify || action == models.PolicyActionCreatePlan
}

func normalizePolicyInput(in *CreatePolicyInput) *Error {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 || len(in.Name) > 120 {
		return ValidationError("name", "must be 2-120 characters")
	}
	if !validPolicyTrigger(in.Trigger) {
		return ValidationError("trigger", "must be execution.failed, execution.completed, check.failed or advisor.recommendation")
	}
	if !validPolicyAction(in.Action) {
		return ValidationError("action", "must be notify or create_plan")
	}
	if in.Conditions == nil {
		in.Conditions = map[string]string{}
	}
	if in.ActionConfig == nil {
		in.ActionConfig = map[string]any{}
	}
	if in.Action == models.PolicyActionCreatePlan {
		if in.Trigger == models.PolicyTriggerAdvisor {
			return ValidationError("action", "the advisor trigger can only notify: an optimization is a tool suggestion, not a template deployment")
		}
		if _, err := uuid.Parse(stringField(in.ActionConfig, "template_id")); err != nil {
			return ValidationError("action_config.template_id", "a template_id is required to create a plan")
		}
	}
	return nil
}

func stringField(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

// List returns the user's policies.
func (s *AutomationPolicyUsecase) List(ctx context.Context, userID uuid.UUID) ([]PolicyView, error) {
	items, err := s.repo.ListByOwner(ctx, userID)
	if err != nil {
		return nil, err
	}
	out := make([]PolicyView, 0, len(items))
	for i := range items {
		out = append(out, decodePolicyView(&items[i]))
	}
	return out, nil
}

// Get returns one policy.
func (s *AutomationPolicyUsecase) Get(ctx context.Context, id, userID uuid.UUID) (*PolicyView, error) {
	p, err := s.repo.FindByID(ctx, id, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, newError(KindNotFound, "POLICY_NOT_FOUND", "policy not found")
		}
		return nil, err
	}
	v := decodePolicyView(p)
	return &v, nil
}

// Create stores a policy.
func (s *AutomationPolicyUsecase) Create(ctx context.Context, userID uuid.UUID, in CreatePolicyInput, clientIP string) (*PolicyView, error) {
	if e := normalizePolicyInput(&in); e != nil {
		return nil, e
	}
	enabled := true
	if in.Enabled != nil {
		enabled = *in.Enabled
	}
	conditions, _ := json.Marshal(in.Conditions)
	actionConfig, _ := json.Marshal(in.ActionConfig)
	now := time.Now()
	p := &models.AutomationPolicy{
		OwnerID: userID, Name: in.Name, Trigger: in.Trigger, Conditions: string(conditions),
		Action: in.Action, ActionConfig: string(actionConfig), Enabled: enabled, CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, &userID, "ai.policy.create", "automation_policy", &p.ID, "success", clientIP, map[string]any{"trigger": p.Trigger, "action": p.Action})
	v := decodePolicyView(p)
	return &v, nil
}

// Update changes a policy.
func (s *AutomationPolicyUsecase) Update(ctx context.Context, id, userID uuid.UUID, in CreatePolicyInput, clientIP string) (*PolicyView, error) {
	p, err := s.repo.FindByID(ctx, id, userID)
	if err != nil {
		if err == repository.ErrNotFound {
			return nil, newError(KindNotFound, "POLICY_NOT_FOUND", "policy not found")
		}
		return nil, err
	}
	if e := normalizePolicyInput(&in); e != nil {
		return nil, e
	}
	p.Name, p.Trigger, p.Action = in.Name, in.Trigger, in.Action
	if in.Enabled != nil {
		p.Enabled = *in.Enabled
	}
	conditions, _ := json.Marshal(in.Conditions)
	actionConfig, _ := json.Marshal(in.ActionConfig)
	p.Conditions, p.ActionConfig, p.UpdatedAt = string(conditions), string(actionConfig), time.Now()
	if err := s.repo.Save(ctx, p); err != nil {
		return nil, err
	}
	s.audit.Log(ctx, &userID, "ai.policy.update", "automation_policy", &p.ID, "success", clientIP, nil)
	v := decodePolicyView(p)
	return &v, nil
}

// Delete removes a policy.
func (s *AutomationPolicyUsecase) Delete(ctx context.Context, id, userID uuid.UUID, clientIP string) error {
	if err := s.repo.Delete(ctx, id, userID); err != nil {
		if err == repository.ErrNotFound {
			return newError(KindNotFound, "POLICY_NOT_FOUND", "policy not found")
		}
		return err
	}
	s.audit.Log(ctx, &userID, "ai.policy.delete", "automation_policy", &id, "success", clientIP, nil)
	return nil
}

// Evaluate runs the user's enabled policies for trigger. It always runs
// outside the transaction that produced the event; failures are ignored
// because a policy must never break the operation that triggered it.
func (s *AutomationPolicyUsecase) Evaluate(ctx context.Context, ownerID uuid.UUID, event PolicyEvent) {
	s.EvaluateFor(ctx, ownerID, []PolicyEvent{event})
}

// EvaluateFor evaluates several events of the same trigger, listing the
// user's enabled policies once. The advisor uses it because one report can
// hold several recommendations.
func (s *AutomationPolicyUsecase) EvaluateFor(ctx context.Context, ownerID uuid.UUID, events []PolicyEvent) {
	if len(events) == 0 {
		return
	}
	policies, err := s.repo.ListEnabledForTrigger(ctx, ownerID, events[0].Trigger)
	if err != nil {
		return
	}
	for _, event := range events {
		for i := range policies {
			s.apply(ctx, ownerID, &policies[i], event)
		}
	}
}

// apply evaluates one event against one policy.
func (s *AutomationPolicyUsecase) apply(ctx context.Context, ownerID uuid.UUID, p *models.AutomationPolicy, event PolicyEvent) {
	if event.Trigger != p.Trigger {
		return
	}
	var conditions map[string]string
	_ = json.Unmarshal([]byte(p.Conditions), &conditions)
	if !conditionsMatch(conditions, policyConditionValues(event)) {
		return
	}
	switch p.Action {
	case models.PolicyActionNotify:
		var cfg map[string]any
		_ = json.Unmarshal([]byte(p.ActionConfig), &cfg)
		title := stringField(cfg, "title")
		if title == "" {
			title = p.Name
		}
		message := stringField(cfg, "message")
		if message == "" {
			message = event.Message
		}
		notifType := models.NotificationTypeSystem
		actionURL := ""
		if event.Trigger == models.PolicyTriggerAdvisor {
			notifType, actionURL = models.NotificationTypeAdvisor, "/dashboard/operations?tab=advisor"
		}
		s.notifications.Notify(ctx, NotificationInput{
			UserID: ownerID, Type: notifType, Severity: event.Severity,
			Title: title, Message: message, ResourceType: event.ResourceType,
			ActionURL: actionURL, DedupeKey: policyDedupeKey(p.ID, event),
		})
	case models.PolicyActionCreatePlan:
		// Recommendations carry a tool suggestion, not a template: an advisor
		// policy may only notify, even if one predates that rule.
		if event.Trigger == models.PolicyTriggerAdvisor || s.plans == nil {
			return
		}
		var cfg map[string]any
		_ = json.Unmarshal([]byte(p.ActionConfig), &cfg)
		templateID, err := uuid.Parse(stringField(cfg, "template_id"))
		if err != nil {
			return
		}
		params, _ := cfg["parameters"].(map[string]any)
		plan, err := s.plans.DeployTemplate(ctx, ownerID, false, templateID, params)
		if err != nil {
			s.notifications.Notify(ctx, NotificationInput{
				UserID: ownerID, Type: models.NotificationTypeSystem, Severity: models.NotificationSeverityWarning,
				Title:     "Automation policy could not create a plan",
				Message:   "Policy " + p.Name + ": " + err.Error(),
				DedupeKey: policyDedupeKey(p.ID, event) + ":error",
			})
			return
		}
		s.notifications.Notify(ctx, NotificationInput{
			UserID: ownerID, Type: models.NotificationTypeSystem, Severity: models.NotificationSeverityInfo,
			Title: "Automation policy proposed a plan", Message: "Review and approve it in the AI workspace.",
			ResourceType: "plan", ResourceID: &plan.ID, ActionURL: PlanURL(plan.ConversationID, plan.ID),
			DedupeKey: "policy:" + p.ID.String() + ":" + plan.ID.String(),
		})
	}
	_ = s.repo.MarkTriggered(ctx, p.ID)
}

// policyConditionValues projects an event onto the condition keys a policy may
// match.
func policyConditionValues(event PolicyEvent) map[string]string {
	return map[string]string{
		"severity": event.Severity, "action": event.Action, "resource_type": event.ResourceType,
		"category": event.Category, "check_type": event.CheckType, "risk": event.Risk,
	}
}

// policyDedupeKey is stable per policy and finding. Execution, plan and check
// events are identified by their ids; advisor findings by the recommendation
// id, which is derived from the category and the resource.
func policyDedupeKey(policyID uuid.UUID, event PolicyEvent) string {
	key := "policy:" + policyID.String() + ":" + event.ExecutionID + ":" + event.PlanID + ":" + event.CheckID
	if event.RecommendationID != "" {
		key += ":" + event.RecommendationID
	}
	return key
}

func conditionsMatch(conditions, values map[string]string) bool {
	for key, want := range conditions {
		if want == "" || want == "*" {
			continue
		}
		got, known := values[key]
		if !known {
			return false
		}
		if !valueMatches(got, want) {
			return false
		}
	}
	return true
}

func valueMatches(got, want string) bool {
	for _, part := range strings.Split(want, ",") {
		if strings.TrimSpace(part) == got {
			return true
		}
	}
	return false
}
