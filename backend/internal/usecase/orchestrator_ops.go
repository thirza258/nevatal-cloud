package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/repository"
)

// Phase 2-4 operations: reusable templates, diagnostics, remediation,
// optimization/cost advice, topology and logical agent roles. Everything
// here is deterministic; model output never bypasses the policy engine.

// TemplateParameter describes one deploy-time template parameter.
type TemplateParameter struct {
	Key         string   `json:"key"`
	Label       string   `json:"label"`
	Description string   `json:"description,omitempty"`
	Type        string   `json:"type"`
	Required    bool     `json:"required"`
	Default     any      `json:"default,omitempty"`
	Enum        []string `json:"enum,omitempty"`
	Min         *int     `json:"min,omitempty"`
	Max         *int     `json:"max,omitempty"`
}

// TemplateView is the API view of an infrastructure template.
type TemplateView struct {
	ID               uuid.UUID                  `json:"id"`
	OwnerID          uuid.UUID                  `json:"owner_id"`
	Name             string                     `json:"name"`
	Description      string                     `json:"description"`
	Intent           string                     `json:"intent"`
	Version          int                        `json:"version"`
	IsPublic         bool                       `json:"is_public"`
	UsageCount       int64                      `json:"usage_count"`
	ValidationStatus string                     `json:"validation_status"`
	Specification    orchestrator.Specification `json:"specification"`
	Parameters       []TemplateParameter        `json:"parameters"`
	CreatedAt        time.Time                  `json:"created_at"`
	UpdatedAt        time.Time                  `json:"updated_at"`
}

// SaveTemplateInput is the body of POST /ai/templates.
type SaveTemplateInput struct {
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	Specification orchestrator.Specification `json:"specification"`
	Parameters    []TemplateParameter        `json:"parameters"`
	IsPublic      bool                       `json:"is_public"`
}

var (
	templateKeyRegex = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,63}$`)
	secretKeyWords   = []string{"password", "passwd", "secret", "token", "api_key", "apikey", "private_key", "access_key", "credential", "connection_string"}
	secretValueRegex = regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`)
)

func secretKey(key string) bool {
	lower := strings.ToLower(key)
	for _, w := range secretKeyWords {
		if strings.Contains(lower, w) {
			return true
		}
	}
	return false
}

// rejectSecrets walks a config and rejects secret-looking keys and values
// that embed credentials (templates must never contain plaintext secrets).
func rejectSecrets(value any, path string) error {
	switch v := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(v))
		for k := range v {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if secretKey(k) {
				return &orchestrator.SpecError{Field: path + "." + k, Message: "secrets are not allowed in templates; use a parameter or configure the secret afterwards"}
			}
			if err := rejectSecrets(v[k], path+"."+k); err != nil {
				return err
			}
		}
	case []any:
		for i, item := range v {
			if err := rejectSecrets(item, fmt.Sprintf("%s[%d]", path, i)); err != nil {
				return err
			}
		}
	case string:
		if secretValueRegex.MatchString(v) {
			return &orchestrator.SpecError{Field: path, Message: "values with embedded credentials are not allowed in templates"}
		}
	}
	return nil
}

func validateTemplateParameters(params []TemplateParameter) error {
	seen := map[string]bool{}
	for i := range params {
		p := &params[i]
		p.Key = strings.TrimSpace(p.Key)
		if !templateKeyRegex.MatchString(p.Key) {
			return ValidationError(fmt.Sprintf("parameters[%d].key", i), "must start with a letter or underscore and contain letters, digits or underscores")
		}
		if seen[p.Key] {
			return ValidationError(fmt.Sprintf("parameters[%d].key", i), "duplicate parameter key")
		}
		seen[p.Key] = true
		switch p.Type {
		case "", models.TemplateParamString:
			p.Type = models.TemplateParamString
		case models.TemplateParamInt, models.TemplateParamBool, models.TemplateParamEnum:
		default:
			return ValidationError(fmt.Sprintf("parameters[%d].type", i), "must be string, integer, boolean or enum")
		}
		if p.Type == models.TemplateParamEnum && len(p.Enum) == 0 {
			return ValidationError(fmt.Sprintf("parameters[%d].enum", i), "an enum parameter needs at least one value")
		}
		if p.Default != nil {
			if err := rejectSecrets(p.Default, fmt.Sprintf("parameters[%d].default", i)); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateTemplateSpec validates the parts of a specification that do not
// depend on parameter substitution.
func (s *OrchestratorUsecase) validateTemplateSpec(spec orchestrator.Specification) *orchestrator.SpecError {
	if len(spec.Resources) == 0 {
		return &orchestrator.SpecError{Field: "specification.resources", Message: "at least one resource is required"}
	}
	if len(spec.Resources) > orchestrator.MaxResources {
		return &orchestrator.SpecError{Field: "specification.resources", Message: fmt.Sprintf("at most %d resources are allowed", orchestrator.MaxResources)}
	}
	seen := map[string]bool{}
	for i, res := range spec.Resources {
		field := fmt.Sprintf("specification.resources[%d]", i)
		if res.Key == "" {
			return &orchestrator.SpecError{Field: field + ".key", Message: "is required"}
		}
		if seen[res.Key] {
			return &orchestrator.SpecError{Field: field + ".key", Message: "duplicate resource key"}
		}
		seen[res.Key] = true
		if _, ok := s.reg.ToolFor(res.Type, res.Action); !ok {
			return &orchestrator.SpecError{Field: field, Message: fmt.Sprintf("unsupported resource type/action %q/%q", res.Type, res.Action)}
		}
		if res.Config == nil {
			return &orchestrator.SpecError{Field: field + ".config", Message: "is required"}
		}
		if err := rejectSecrets(res.Config, field+".config"); err != nil {
			se, _ := err.(*orchestrator.SpecError)
			return se
		}
	}
	return nil
}

func decodeTemplate(t *models.InfraTemplate) (*TemplateView, error) {
	v := &TemplateView{
		ID: t.ID, OwnerID: t.OwnerID, Name: t.Name, Description: t.Description, Intent: t.Intent,
		Version: t.Version, IsPublic: t.IsPublic, UsageCount: t.UsageCount, ValidationStatus: t.ValidationStatus,
		CreatedAt: t.CreatedAt.UTC(), UpdatedAt: t.UpdatedAt.UTC(), Parameters: []TemplateParameter{},
	}
	if err := orchestrator.DecodeJSON(t.Specification, &v.Specification); err != nil {
		return nil, err
	}
	if v.Specification.Resources == nil {
		v.Specification.Resources = []orchestrator.ResourceSpec{}
	}
	_ = orchestrator.DecodeJSON(t.Parameters, &v.Parameters)
	if v.Parameters == nil {
		v.Parameters = []TemplateParameter{}
	}
	return v, nil
}

// ListTemplates returns the user's templates and public templates.
func (s *OrchestratorUsecase) ListTemplates(ctx context.Context, userID uuid.UUID) ([]TemplateView, error) {
	items, err := s.repos.InfraTemplates.List(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	out := make([]TemplateView, 0, len(items))
	for i := range items {
		v, err := decodeTemplate(&items[i])
		if err != nil {
			continue
		}
		out = append(out, *v)
	}
	return out, nil
}

// GetTemplate returns one visible template.
func (s *OrchestratorUsecase) GetTemplate(ctx context.Context, id, userID uuid.UUID) (*TemplateView, error) {
	t, err := s.repos.InfraTemplates.FindVisible(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, newError(KindNotFound, "TEMPLATE_NOT_FOUND", "template not found")
		}
		return nil, err
	}
	return decodeTemplate(t)
}

// SaveTemplate validates and stores a template. Secrets are rejected.
func (s *OrchestratorUsecase) SaveTemplate(ctx context.Context, userID uuid.UUID, in SaveTemplateInput, clientIP string) (*TemplateView, error) {
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 || len(in.Name) > 120 {
		return nil, ValidationError("name", "must be 2-120 characters")
	}
	if len(in.Description) > 2000 {
		return nil, ValidationError("description", "must be at most 2000 characters")
	}
	if err := validateTemplateParameters(in.Parameters); err != nil {
		return nil, err
	}
	if se := s.validateTemplateSpec(in.Specification); se != nil {
		return nil, ValidationError(se.Field, se.Error())
	}
	specJSON, err := orchestrator.Canonical(in.Specification)
	if err != nil {
		return nil, ValidationError("specification", err.Error())
	}
	paramsJSON, err := orchestrator.Canonical(in.Parameters)
	if err != nil {
		return nil, ValidationError("parameters", err.Error())
	}
	intent := in.Specification.Intent
	if intent == "" {
		intent = "template"
	}
	now := time.Now()
	t := &models.InfraTemplate{
		OwnerID: userID, Name: in.Name, Description: in.Description, Intent: intent,
		Specification: string(specJSON), Parameters: string(paramsJSON), Version: 1,
		IsPublic: in.IsPublic, ValidationStatus: "valid", CreatedAt: now, UpdatedAt: now,
	}
	if err := s.repos.InfraTemplates.Create(ctx, t); err != nil {
		if errors.Is(err, repository.ErrDuplicate) {
			return nil, newError(KindConflict, "TEMPLATE_NAME_TAKEN", "a template with this name already exists")
		}
		return nil, err
	}
	s.svc.Audit.Log(ctx, &userID, "ai.template.create", "infra_template", &t.ID, "success", clientIP, map[string]any{"name": t.Name})
	return decodeTemplate(t)
}

// UpdateTemplate replaces the editable fields of a template.
func (s *OrchestratorUsecase) UpdateTemplate(ctx context.Context, id, userID uuid.UUID, in SaveTemplateInput, clientIP string) (*TemplateView, error) {
	t, err := s.repos.InfraTemplates.FindVisible(ctx, id, userID)
	if err != nil || t.OwnerID != userID {
		return nil, newError(KindNotFound, "TEMPLATE_NOT_FOUND", "template not found")
	}
	in.Name = strings.TrimSpace(in.Name)
	if len(in.Name) < 2 || len(in.Name) > 120 {
		return nil, ValidationError("name", "must be 2-120 characters")
	}
	if err := validateTemplateParameters(in.Parameters); err != nil {
		return nil, err
	}
	if se := s.validateTemplateSpec(in.Specification); se != nil {
		return nil, ValidationError(se.Field, se.Error())
	}
	specJSON, _ := orchestrator.Canonical(in.Specification)
	paramsJSON, _ := orchestrator.Canonical(in.Parameters)
	intent := in.Specification.Intent
	if intent == "" {
		intent = t.Intent
	}
	t.Name, t.Description, t.Intent = in.Name, in.Description, intent
	t.Specification, t.Parameters, t.IsPublic = string(specJSON), string(paramsJSON), in.IsPublic
	t.Version++
	t.UpdatedAt = time.Now()
	if err := s.repos.InfraTemplates.Save(ctx, t); err != nil {
		return nil, err
	}
	s.svc.Audit.Log(ctx, &userID, "ai.template.update", "infra_template", &t.ID, "success", clientIP, nil)
	return decodeTemplate(t)
}

// DeleteTemplate removes one of the user's templates.
func (s *OrchestratorUsecase) DeleteTemplate(ctx context.Context, id, userID uuid.UUID, clientIP string) error {
	if err := s.repos.InfraTemplates.SoftDelete(ctx, id, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return newError(KindNotFound, "TEMPLATE_NOT_FOUND", "template not found")
		}
		return err
	}
	s.svc.Audit.Log(ctx, &userID, "ai.template.delete", "infra_template", &id, "success", clientIP, nil)
	return nil
}

// DeployTemplate creates a plan from a template and parameter values in the
// template conversation.
func (s *OrchestratorUsecase) DeployTemplate(ctx context.Context, userID uuid.UUID, isAdmin bool, templateID uuid.UUID, params map[string]any) (*models.AIResourcePlan, error) {
	convID, err := s.requestConversation(ctx, userID, TemplateConversationTitle)
	if err != nil {
		return nil, err
	}
	plan, _, err := s.DeployTemplateToConversation(ctx, userID, isAdmin, templateID, params, convID)
	return plan, err
}

// DeployTemplateToConversation creates a plan from a template in an existing
// conversation (used by chat so the plan stays in the conversation the user is
// already looking at). Nothing executes: the plan still needs approval.
func (s *OrchestratorUsecase) DeployTemplateToConversation(ctx context.Context, userID uuid.UUID, isAdmin bool,
	templateID uuid.UUID, params map[string]any, conversationID uuid.UUID) (*models.AIResourcePlan, *orchestrator.Revision, error) {
	ctx = context.WithoutCancel(ctx)
	if conversationID == uuid.Nil {
		return nil, nil, ValidationError("conversation_id", "is required")
	}
	t, err := s.repos.InfraTemplates.FindVisible(ctx, templateID, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, newError(KindNotFound, "TEMPLATE_NOT_FOUND", "template not found")
		}
		return nil, nil, err
	}
	if t.ValidationStatus != "valid" {
		return nil, nil, newError(KindUnprocessable, "TEMPLATE_INVALID", "the template failed validation and cannot be deployed")
	}
	var spec orchestrator.Specification
	if err := orchestrator.DecodeJSON(t.Specification, &spec); err != nil {
		return nil, nil, err
	}
	var declared []TemplateParameter
	_ = orchestrator.DecodeJSON(t.Parameters, &declared)
	spec, err = substituteTemplate(spec, declared, params)
	if err != nil {
		return nil, nil, err
	}
	if err := s.reg.CheckStructure(spec); err != nil {
		return nil, nil, specError(err)
	}
	plan, rev, err := s.CreatePlan(ctx, PlanDraft{
		ConversationID: conversationID, UserID: userID, IsAdmin: isAdmin,
		Intent:      firstNonEmpty(spec.Intent, t.Intent, "template_deploy"),
		Title:       "Deploy template: " + t.Name,
		Description: fmt.Sprintf("Template %q version %d", t.Name, t.Version),
		Spec:        spec,
	})
	if err != nil {
		return nil, nil, err
	}
	_ = s.repos.InfraTemplates.IncrementUsage(ctx, t.ID)
	s.svc.Audit.Log(ctx, &userID, "ai.template.deploy", "infra_template", &t.ID, "success", "", map[string]any{"plan_id": plan.ID})
	return plan, rev, nil
}

// TemplateConversationTitle is where plans proposed by templates are filed.
const TemplateConversationTitle = "Infrastructure templates"

// substituteTemplate replaces {{key}} placeholders in a specification with
// typed parameter values (declared defaults fill missing values).
func substituteTemplate(spec orchestrator.Specification, declared []TemplateParameter, provided map[string]any) (orchestrator.Specification, error) {
	values := map[string]any{}
	types := map[string]TemplateParameter{}
	for _, p := range declared {
		types[p.Key] = p
		if p.Default != nil {
			values[p.Key] = p.Default
		}
	}
	for k, v := range provided {
		if _, ok := types[k]; !ok {
			return orchestrator.Specification{}, ValidationError("parameters."+k, "is not a parameter of this template")
		}
		values[k] = v
	}
	for _, p := range declared {
		if _, ok := values[p.Key]; !ok && p.Required {
			return orchestrator.Specification{}, ValidationError("parameters."+p.Key, "is required")
		}
	}
	raw, err := orchestrator.Canonical(spec)
	if err != nil {
		return orchestrator.Specification{}, err
	}
	var generic map[string]any
	if err := orchestrator.DecodeJSON(string(raw), &generic); err != nil {
		return orchestrator.Specification{}, err
	}
	replaced, err := replacePlaceholders(generic, values, types)
	if err != nil {
		return orchestrator.Specification{}, err
	}
	out, err := orchestrator.Canonical(replaced)
	if err != nil {
		return orchestrator.Specification{}, err
	}
	if strings.Contains(string(out), "{{") {
		return orchestrator.Specification{}, ValidationError("specification", "has an unresolved or unknown template parameter")
	}
	var resolved orchestrator.Specification
	if err := orchestrator.DecodeJSON(string(out), &resolved); err != nil {
		return orchestrator.Specification{}, err
	}
	return resolved, nil
}

var placeholderRegex = regexp.MustCompile(`^\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}$`)
var placeholderAnywhere = regexp.MustCompile(`\{\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*\}\}`)

func replacePlaceholders(value any, values map[string]any, types map[string]TemplateParameter) (any, error) {
	switch v := value.(type) {
	case map[string]any:
		for k, item := range v {
			replaced, err := replacePlaceholders(item, values, types)
			if err != nil {
				return nil, err
			}
			v[k] = replaced
		}
		return v, nil
	case []any:
		for i, item := range v {
			replaced, err := replacePlaceholders(item, values, types)
			if err != nil {
				return nil, err
			}
			v[i] = replaced
		}
		return v, nil
	case string:
		if m := placeholderRegex.FindStringSubmatch(v); m != nil {
			val, ok := values[m[1]]
			if !ok {
				return nil, ValidationError("parameters."+m[1], "is required")
			}
			return coerceParam(m[1], val, types[m[1]])
		}
		if !placeholderAnywhere.MatchString(v) {
			return v, nil
		}
		var outErr error
		out := placeholderAnywhere.ReplaceAllStringFunc(v, func(token string) string {
			m := placeholderAnywhere.FindStringSubmatch(token)
			val, ok := values[m[1]]
			if !ok {
				outErr = ValidationError("parameters."+m[1], "is required")
				return token
			}
			coerced, err := coerceParam(m[1], val, types[m[1]])
			if err != nil {
				outErr = err
				return token
			}
			return fmt.Sprint(coerced)
		})
		if outErr != nil {
			return nil, outErr
		}
		return out, nil
	}
	return value, nil
}

func coerceParam(key string, value any, p TemplateParameter) (any, error) {
	switch p.Type {
	case models.TemplateParamInt:
		var n int
		switch v := value.(type) {
		case float64:
			n = int(v)
		case int:
			n = v
		case json.Number:
			i, err := v.Int64()
			if err != nil {
				return nil, ValidationError("parameters."+key, "must be an integer")
			}
			n = int(i)
		default:
			return nil, ValidationError("parameters."+key, "must be an integer")
		}
		if p.Min != nil && n < *p.Min {
			return nil, ValidationError("parameters."+key, fmt.Sprintf("must be at least %d", *p.Min))
		}
		if p.Max != nil && n > *p.Max {
			return nil, ValidationError("parameters."+key, fmt.Sprintf("must be at most %d", *p.Max))
		}
		return n, nil
	case models.TemplateParamBool:
		b, ok := value.(bool)
		if !ok {
			return nil, ValidationError("parameters."+key, "must be a boolean")
		}
		return b, nil
	case models.TemplateParamEnum:
		s, ok := value.(string)
		if !ok {
			return nil, ValidationError("parameters."+key, "must be a string")
		}
		for _, allowed := range p.Enum {
			if s == allowed {
				return s, nil
			}
		}
		return nil, ValidationError("parameters."+key, "must be one of "+strings.Join(p.Enum, ", "))
	}
	if value == nil {
		return "", nil
	}
	if s, ok := value.(string); ok {
		return s, nil
	}
	return fmt.Sprint(value), nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}

// --- Diagnostics -----------------------------------------------------------

// Diagnose collects authorized evidence for one resource and ranks causes.
func (s *OrchestratorUsecase) Diagnose(ctx context.Context, userID uuid.UUID, resourceType string, resourceID uuid.UUID) (*orchestrator.DiagnosticResult, error) {
	now := time.Now()
	in := orchestrator.DiagnosticInput{ResourceType: resourceType, ResourceID: resourceID}
	switch resourceType {
	case orchestrator.TypeVPS:
		inst, err := s.svc.Instance.Get(ctx, resourceID, userID, false)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "instance not found or access denied")
		}
		in.Name, in.Status, in.Running = inst.Name, inst.Status, inst.Status == models.StateRunning
		in.AgeDays = int(now.Sub(inst.CreatedAt).Hours() / 24)
		if inst.PrivateIP != nil {
			in.PrivateIP = *inst.PrivateIP
		}
		if in.Running {
			if m, err := s.svc.Instance.GetMetrics(ctx, resourceID, userID, false); err == nil && m != nil && m.Available && m.Status != nil {
				in.HasMetrics = true
				in.CPUPercent = m.Status.CPUUsage * 100
				in.MemoryPercent = pct(m.Status.MemoryUsed, m.Status.MemoryMax)
				in.DiskPercent = pct(m.Status.DiskUsed, m.Status.DiskMax)
			}
		}
	case orchestrator.TypeApp:
		app, err := s.svc.App.Get(ctx, resourceID, userID, false)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "application not found or access denied")
		}
		in.Name, in.Status = app.Name, app.Status
		in.Running = app.Status == "running" || app.Status == "active"
		in.AgeDays = int(now.Sub(app.CreatedAt).Hours() / 24)
		if len(app.Deployments) > 0 {
			dep := app.Deployments[0]
			if dep.Status == "failed" {
				in.ErrorMessage = firstNonEmpty(dep.ErrorMessage, "the last deployment failed")
			}
		}
	case orchestrator.TypeN8n:
		a, err := s.svc.Automation.Get(ctx, resourceID, userID, false)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "n8n instance not found or access denied")
		}
		in.Name, in.Status, in.Running = a.Name, a.Status, a.Status == "running"
		in.AgeDays = int(now.Sub(a.CreatedAt).Hours() / 24)
	case orchestrator.TypeOpenClaw:
		a, err := s.svc.OpenClaw.Get(ctx, resourceID, userID, false)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "OpenClaw agent not found or access denied")
		}
		in.Name, in.Status, in.Running = a.Name, a.Status, a.Status == "running"
		in.AgeDays = int(now.Sub(a.CreatedAt).Hours() / 24)
	case orchestrator.TypeGameServer:
		g, err := s.svc.GameServer.Get(ctx, resourceID, userID, false)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "game server not found or access denied")
		}
		in.Name, in.Status = g.Name, g.Status
		in.Running = g.Status == models.GameServerStatusRunning
		in.LastBackupAt = g.LastBackupAt
		in.AgeDays = int(now.Sub(g.CreatedAt).Hours() / 24)
		if in.Running {
			if m, err := s.svc.GameServer.GetMetrics(ctx, resourceID, userID, false); err == nil {
				in.HasMetrics = true
				in.CPUPercent, in.MemoryPercent = m.CPUUsagePercent, m.MemoryUsagePercent
				in.DiskPercent = pctFloat(m.DiskUsedGB, float64(m.DiskTotalGB))
			}
		}
	case orchestrator.TypeAndroid:
		a, err := s.svc.Android.Get(ctx, resourceID, userID, false)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "Android instance not found or access denied")
		}
		in.Name, in.Status = a.Name, a.Status
		in.Running = a.Status == models.AndroidStatusRunning
		in.LastBackupAt = a.LastBackupAt
		in.AgeDays = int(now.Sub(a.CreatedAt).Hours() / 24)
		if in.Running {
			if m, err := s.svc.Android.GetMetrics(ctx, resourceID, userID, false); err == nil {
				in.HasMetrics = true
				in.CPUPercent, in.MemoryPercent = m.CPUUsagePercent, m.MemoryUsagePercent
				in.DiskPercent = pctFloat(m.DiskUsedGB, float64(m.DiskTotalGB))
			}
		}
	case orchestrator.TypeDatabase:
		db, err := s.svc.Database.Get(ctx, resourceID, userID)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "database not found or access denied")
		}
		in.Name, in.Status = db.Name, db.Status
		in.Running = db.Status == models.DatabaseStatusRunning
		in.LastBackupAt = db.LastBackupAt
		in.AgeDays = int(now.Sub(db.CreatedAt).Hours() / 24)
		if m, err := s.svc.Database.Metrics(ctx, resourceID, userID); err == nil {
			in.HasMetrics = true
			in.CPUPercent, in.MemoryPercent, in.DiskPercent = m.CPUPercent, m.MemoryUsagePercent, m.StorageUsagePercent
		}
	case orchestrator.TypeBucket:
		b, err := s.svc.Storage.GetOwnedBucket(ctx, resourceID, userID)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "bucket not found or access denied")
		}
		in.Name, in.Status, in.Running = b.Name, "active", true
		in.DiskPercent = pctFloat(float64(b.TotalSizeBytes), 0)
	case orchestrator.TypeDomain:
		domains, err := s.svc.Domain.List(ctx, userID)
		if err != nil {
			return nil, err
		}
		found := false
		for _, d := range domains {
			if d.Domain.ID == resourceID {
				in.Name = d.Domain.Hostname
				in.Status = string(d.LiveStatus)
				in.Running = d.LiveStatus == "live"
				in.AgeDays = int(now.Sub(d.Domain.CreatedAt).Hours() / 24)
				found = true
				break
			}
		}
		if !found {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "domain not found or access denied")
		}
	default:
		return nil, ValidationError("resource_type", "unsupported resource type")
	}
	result := orchestrator.Diagnose(in, now)
	return &result, nil
}

func pct(used, total int64) float64 {
	if total <= 0 {
		return 0
	}
	return float64(used) / float64(total) * 100
}

func pctFloat(used, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return used / total * 100
}

// RemediationInput is the body of POST /ai/diagnostics/:type/:id/remediate.
type RemediationInput struct {
	Action   string `json:"action"`
	VCPUs    int    `json:"vcpus"`
	MemoryMB int    `json:"memory_mb"`
	DiskGB   int    `json:"disk_gb"`
}

// Remediate turns a diagnostic recommendation into an approvable plan.
func (s *OrchestratorUsecase) Remediate(ctx context.Context, userID uuid.UUID, isAdmin bool, resourceType string, resourceID uuid.UUID, in RemediationInput) (*PlanDetail, error) {
	ref, err := s.policyEnv().ServiceResource(ctx, userID, resourceType, resourceID)
	if err != nil || ref == nil {
		return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "resource not found or access denied")
	}
	res := orchestrator.ResourceSpec{Key: ref.Name, Type: resourceType, Config: map[string]any{"name": ref.Name}}
	if resourceType == orchestrator.TypeVPS {
		res.Config["instance_id"] = resourceID.String()
	} else {
		res.Config["resource_id"] = resourceID.String()
	}
	switch in.Action {
	case orchestrator.ActionStart, orchestrator.ActionStop, orchestrator.ActionRestart:
		res.Action = in.Action
	case orchestrator.ActionBackupCreate:
		if resourceType == orchestrator.TypeVPS || resourceType == orchestrator.TypeApp {
			return nil, ValidationError("action", "this resource type does not support backups")
		}
		res.Action = in.Action
	case orchestrator.ActionDelete:
		res.Action = in.Action
	case "deploy":
		if resourceType != orchestrator.TypeApp {
			return nil, ValidationError("action", "deploy is only available for applications")
		}
		res.Action = orchestrator.ActionDeploy
		delete(res.Config, "resource_id")
		res.Config["app_id"] = resourceID.String()
	case "resize":
		if resourceType != orchestrator.TypeVPS {
			return nil, ValidationError("action", "resize is only available for virtual machines")
		}
		inst, err := s.svc.Instance.Get(ctx, resourceID, userID, false)
		if err != nil {
			return nil, newError(KindNotFound, "RESOURCE_NOT_FOUND", "instance not found or access denied")
		}
		res.Action = orchestrator.ActionResize
		res.Config["vcpus"] = firstPositive(in.VCPUs, inst.VCPUs)
		res.Config["memory_mb"] = firstPositive(in.MemoryMB, inst.MemoryMB)
		res.Config["disk_gb"] = firstPositive(in.DiskGB, inst.DiskGB)
	default:
		return nil, ValidationError("action", "unsupported remediation action")
	}
	convID, err := s.requestConversation(ctx, userID, DiagnosticsConversationTitle)
	if err != nil {
		return nil, err
	}
	plan, _, err := s.CreatePlan(ctx, PlanDraft{
		ConversationID: convID, UserID: userID, IsAdmin: isAdmin,
		Intent: "remediate", Title: fmt.Sprintf("Remediate %s %s", resourceType, ref.Name),
		Description: fmt.Sprintf("Proposed remediation from diagnostics: %s", in.Action),
		Spec:        orchestrator.Specification{Intent: "remediate", Resources: []orchestrator.ResourceSpec{res}},
	})
	if err != nil {
		return nil, err
	}
	s.svc.Audit.Log(ctx, &userID, "ai.diagnostics.remediate", "ai_plan", &plan.ID, "success", "", map[string]any{"resource_type": resourceType, "resource_id": resourceID.String(), "action": in.Action})
	return s.GetPlan(ctx, plan.ID, userID)
}

// DiagnosticsConversationTitle is where remediation plans are filed.
const DiagnosticsConversationTitle = "AI diagnostics"

func firstPositive(values ...int) int {
	for _, v := range values {
		if v > 0 {
			return v
		}
	}
	return 0
}

func (s *OrchestratorUsecase) policyEnv() *policyEnv { return &policyEnv{s: s} }

// --- Advisor and costs -----------------------------------------------------

// AdvisorReport is the optimization and cost analysis of one account.
type AdvisorReport struct {
	GeneratedAt   time.Time                   `json:"generated_at"`
	Currency      string                      `json:"currency"`
	MonthlyTotal  float64                     `json:"monthly_total"`
	CostResources []orchestrator.CostItem     `json:"cost_resources"`
	Optimizations []orchestrator.Optimization `json:"optimizations"`
	Notes         []string                    `json:"notes"`
	Quota         *models.QuotaUsage          `json:"quota"`
}

func (s *OrchestratorUsecase) rates() orchestrator.CostRate {
	return orchestrator.CostRate{
		Currency:         s.cfg.OrchCostCurrency,
		VCPUPerMonth:     s.cfg.OrchCostVCPU,
		MemoryGBPerMonth: s.cfg.OrchCostMemoryGB,
		DiskGBPerMonth:   s.cfg.OrchCostDiskGB,
		BucketGBPerMonth: s.cfg.OrchCostBucketGB,
	}
}

// Optimize analyzes the account and returns recommendations with costs. It
// also evaluates the user's advisor.recommendation policies, so a saved rule
// can point the user at a new finding. Nothing is executed: such policies can
// only notify.
func (s *OrchestratorUsecase) Optimize(ctx context.Context, userID uuid.UUID) (*AdvisorReport, error) {
	costs, resources, err := s.gatherAdvisor(ctx, userID)
	if err != nil {
		return nil, err
	}
	report := &AdvisorReport{GeneratedAt: time.Now().UTC(), Currency: costs.Currency, MonthlyTotal: costs.MonthlyTotal,
		CostResources: costs.Resources, Optimizations: orchestrator.GenerateOptimizations(resources), Notes: costs.Notes}
	if usage, err := s.svc.Quota.GetUsage(ctx, userID); err == nil {
		report.Quota = usage
	}
	if s.svc.Policies != nil {
		s.svc.Policies.EvaluateFor(ctx, userID, advisorPolicyEvents(report.Optimizations))
	}
	return report, nil
}

// advisorPolicyEvents turns each recommendation into a policy event. Severity
// follows the category: a failed resource is critical, waste and capacity
// pressure are warnings, and a rightsize suggestion is informational.
func advisorPolicyEvents(optimizations []orchestrator.Optimization) []PolicyEvent {
	events := make([]PolicyEvent, 0, len(optimizations))
	for _, o := range optimizations {
		severity := models.NotificationSeverityWarning
		switch o.Category {
		case "reliability":
			severity = models.NotificationSeverityCritical
		case "rightsize":
			severity = models.NotificationSeverityInfo
		}
		message := o.Title
		if o.ResourceName != "" {
			message += " (" + o.ResourceName + ")"
		}
		if o.Proposed != "" {
			message += ": " + o.Proposed
		}
		events = append(events, PolicyEvent{
			Trigger: models.PolicyTriggerAdvisor, Severity: severity, Action: "recommend",
			ResourceType: o.ResourceType, ResourceID: o.ResourceID, ResourceName: o.ResourceName,
			Category: o.Category, Risk: o.Risk, Message: message, RecommendationID: o.ID,
		})
	}
	return events
}

// Costs returns the cost estimate only.
func (s *OrchestratorUsecase) Costs(ctx context.Context, userID uuid.UUID) (*orchestrator.CostReport, error) {
	costs, _, err := s.gatherAdvisor(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &costs, nil
}

func (s *OrchestratorUsecase) gatherAdvisor(ctx context.Context, userID uuid.UUID) (orchestrator.CostReport, []orchestrator.AdvisorResource, error) {
	var items []orchestrator.CostItem
	var resources []orchestrator.AdvisorResource
	now := time.Now()
	add := func(item orchestrator.CostItem, res orchestrator.AdvisorResource) {
		items = append(items, item)
		resources = append(resources, res)
	}

	if instances, err := s.svc.Instance.List(ctx, userID, false, ""); err == nil {
		for _, inst := range instances {
			res := orchestrator.AdvisorResource{
				ResourceType: orchestrator.TypeVPS, ID: inst.ID.String(), Name: inst.Name, Status: inst.Status,
				VCPUs: inst.VCPUs, MemoryMB: inst.MemoryMB, DiskGB: inst.DiskGB,
				AgeDays: int(now.Sub(inst.CreatedAt).Hours() / 24),
			}
			if inst.Status == models.StateRunning {
				if m, err := s.svc.Instance.GetMetrics(ctx, inst.ID, userID, false); err == nil && m != nil && m.Available && m.Status != nil {
					res.HasMetrics = true
					res.CPUPercent = m.Status.CPUUsage * 100
					res.MemoryPercent = pct(m.Status.MemoryUsed, m.Status.MemoryMax)
					res.DiskPercent = pct(m.Status.DiskUsed, m.Status.DiskMax)
				}
			}
			add(orchestrator.CostItem{ResourceType: orchestrator.TypeVPS, ID: inst.ID.String(), Name: inst.Name,
				VCPUs: inst.VCPUs, MemoryMB: inst.MemoryMB, DiskGB: inst.DiskGB, Status: inst.Status}, res)
		}
	}
	if apps, err := s.svc.App.List(ctx, userID, false); err == nil {
		for _, a := range apps {
			res := orchestrator.AdvisorResource{ResourceType: orchestrator.TypeApp, ID: a.ID.String(), Name: a.Name, Status: a.Status,
				VCPUs: a.VCPUs, MemoryMB: a.MemoryMB, DiskGB: a.DiskGB, AgeDays: int(now.Sub(a.CreatedAt).Hours() / 24)}
			add(orchestrator.CostItem{ResourceType: orchestrator.TypeApp, ID: a.ID.String(), Name: a.Name,
				VCPUs: a.VCPUs, MemoryMB: a.MemoryMB, DiskGB: a.DiskGB, Status: a.Status}, res)
		}
	}
	if itemsN8n, err := s.svc.Automation.List(ctx, userID, false); err == nil {
		for _, a := range itemsN8n {
			add(orchestrator.CostItem{ResourceType: orchestrator.TypeN8n, ID: a.ID.String(), Name: a.Name,
				VCPUs: a.CPULimit, MemoryMB: a.MemoryLimitMB, DiskGB: a.StorageLimitGB, Status: a.Status},
				orchestrator.AdvisorResource{ResourceType: orchestrator.TypeN8n, ID: a.ID.String(), Name: a.Name, Status: a.Status,
					VCPUs: a.CPULimit, MemoryMB: a.MemoryLimitMB, DiskGB: a.StorageLimitGB, AgeDays: int(now.Sub(a.CreatedAt).Hours() / 24)})
		}
	}
	if claws, err := s.svc.OpenClaw.List(ctx, userID, false); err == nil {
		for _, a := range claws {
			add(orchestrator.CostItem{ResourceType: orchestrator.TypeOpenClaw, ID: a.ID.String(), Name: a.Name,
				VCPUs: a.CPULimit, MemoryMB: a.MemoryLimitMB, DiskGB: a.StorageLimitGB, Status: a.Status},
				orchestrator.AdvisorResource{ResourceType: orchestrator.TypeOpenClaw, ID: a.ID.String(), Name: a.Name, Status: a.Status,
					VCPUs: a.CPULimit, MemoryMB: a.MemoryLimitMB, DiskGB: a.StorageLimitGB, AgeDays: int(now.Sub(a.CreatedAt).Hours() / 24)})
		}
	}
	if servers, err := s.svc.GameServer.List(ctx, userID, false, ""); err == nil {
		for _, g := range servers {
			res := orchestrator.AdvisorResource{ResourceType: orchestrator.TypeGameServer, ID: g.ID.String(), Name: g.Name, Status: g.Status,
				VCPUs: g.CPUCores, MemoryMB: g.MemoryMB, DiskGB: g.StorageGB, AgeDays: int(now.Sub(g.CreatedAt).Hours() / 24)}
			if g.Status == models.GameServerStatusRunning {
				if m, err := s.svc.GameServer.GetMetrics(ctx, g.ID, userID, false); err == nil {
					res.HasMetrics = true
					res.CPUPercent, res.MemoryPercent = m.CPUUsagePercent, m.MemoryUsagePercent
					res.DiskPercent = pctFloat(m.DiskUsedGB, float64(m.DiskTotalGB))
				}
			}
			add(orchestrator.CostItem{ResourceType: orchestrator.TypeGameServer, ID: g.ID.String(), Name: g.Name,
				VCPUs: g.CPUCores, MemoryMB: g.MemoryMB, DiskGB: g.StorageGB, Status: g.Status}, res)
		}
	}
	if androids, err := s.svc.Android.List(ctx, userID, false, ""); err == nil {
		for _, a := range androids {
			res := orchestrator.AdvisorResource{ResourceType: orchestrator.TypeAndroid, ID: a.ID.String(), Name: a.Name, Status: a.Status,
				VCPUs: a.VCPUs, MemoryMB: a.MemoryMB, DiskGB: a.StorageGB, AgeDays: int(now.Sub(a.CreatedAt).Hours() / 24)}
			if a.Status == models.AndroidStatusRunning {
				if m, err := s.svc.Android.GetMetrics(ctx, a.ID, userID, false); err == nil {
					res.HasMetrics = true
					res.CPUPercent, res.MemoryPercent = m.CPUUsagePercent, m.MemoryUsagePercent
					res.DiskPercent = pctFloat(m.DiskUsedGB, float64(m.DiskTotalGB))
				}
			}
			add(orchestrator.CostItem{ResourceType: orchestrator.TypeAndroid, ID: a.ID.String(), Name: a.Name,
				VCPUs: a.VCPUs, MemoryMB: a.MemoryMB, DiskGB: a.StorageGB, Status: a.Status}, res)
		}
	}
	if dbs, err := s.svc.Database.List(ctx, userID); err == nil {
		for _, db := range dbs {
			res := orchestrator.AdvisorResource{ResourceType: orchestrator.TypeDatabase, ID: db.ID.String(), Name: db.Name, Status: db.Status,
				VCPUs: db.VCPUs, MemoryMB: db.MemoryMB, DiskGB: db.StorageGB, AgeDays: int(now.Sub(db.CreatedAt).Hours() / 24)}
			if m, err := s.svc.Database.Metrics(ctx, db.ID, userID); err == nil {
				res.HasMetrics = true
				res.CPUPercent, res.MemoryPercent, res.DiskPercent = m.CPUPercent, m.MemoryUsagePercent, m.StorageUsagePercent
			}
			add(orchestrator.CostItem{ResourceType: orchestrator.TypeDatabase, ID: db.ID.String(), Name: db.Name,
				VCPUs: db.VCPUs, MemoryMB: db.MemoryMB, DiskGB: db.StorageGB, Status: db.Status}, res)
		}
	}
	if buckets, err := s.svc.Storage.ListBuckets(ctx, userID); err == nil {
		for _, b := range buckets {
			add(orchestrator.CostItem{ResourceType: orchestrator.TypeBucket, ID: b.ID.String(), Name: b.Name,
				SizeBytes: b.TotalSizeBytes, Status: "active"},
				orchestrator.AdvisorResource{ResourceType: orchestrator.TypeBucket, ID: b.ID.String(), Name: b.Name, Status: "active",
					Objects: b.TotalObjects, SizeBytes: b.TotalSizeBytes, AgeDays: int(now.Sub(b.CreatedAt).Hours() / 24)})
		}
	}
	return orchestrator.EstimateCosts(items, s.rates()), resources, nil
}

// --- Topology --------------------------------------------------------------

// TopologyNode is one resource in the topology graph.
type TopologyNode struct {
	ID       uuid.UUID      `json:"id"`
	Type     string         `json:"type"`
	Name     string         `json:"name"`
	Status   string         `json:"status"`
	Metadata map[string]any `json:"metadata,omitempty"`
}

// TopologyEdge is one directed relationship.
type TopologyEdge struct {
	From     uuid.UUID `json:"from"`
	FromType string    `json:"from_type"`
	To       uuid.UUID `json:"to"`
	ToType   string    `json:"to_type"`
	Relation string    `json:"relation"`
	ToName   string    `json:"to_name,omitempty"`
}

// TopologyView is the user's resources and their relationships.
type TopologyView struct {
	GeneratedAt time.Time      `json:"generated_at"`
	Nodes       []TopologyNode `json:"nodes"`
	Edges       []TopologyEdge `json:"edges"`
}

// Topology builds the resource graph from live records and recorded
// relationships.
func (s *OrchestratorUsecase) Topology(ctx context.Context, userID uuid.UUID) (*TopologyView, error) {
	view := &TopologyView{GeneratedAt: time.Now().UTC(), Nodes: []TopologyNode{}, Edges: []TopologyEdge{}}
	nodes := map[uuid.UUID]bool{}
	add := func(id uuid.UUID, typ, name, status string, meta map[string]any) {
		if nodes[id] {
			return
		}
		nodes[id] = true
		view.Nodes = append(view.Nodes, TopologyNode{ID: id, Type: typ, Name: name, Status: status, Metadata: meta})
	}
	if list, err := s.svc.Instance.List(ctx, userID, false, ""); err == nil {
		for _, x := range list {
			add(x.ID, orchestrator.TypeVPS, x.Name, x.Status, map[string]any{"vcpus": x.VCPUs, "memory_mb": x.MemoryMB, "disk_gb": x.DiskGB})
		}
	}
	if list, err := s.svc.App.List(ctx, userID, false); err == nil {
		for _, x := range list {
			add(x.ID, orchestrator.TypeApp, x.Name, x.Status, map[string]any{"url": "https://" + x.AssignedSubdomain})
		}
	}
	if list, err := s.svc.Automation.List(ctx, userID, false); err == nil {
		for _, x := range list {
			add(x.ID, orchestrator.TypeN8n, x.Name, x.Status, map[string]any{"url": x.DashboardURL})
		}
	}
	if list, err := s.svc.OpenClaw.List(ctx, userID, false); err == nil {
		for _, x := range list {
			add(x.ID, orchestrator.TypeOpenClaw, x.Name, x.Status, map[string]any{"gateway": x.GatewayEndpoint})
		}
	}
	if list, err := s.svc.GameServer.List(ctx, userID, false, ""); err == nil {
		for _, x := range list {
			add(x.ID, orchestrator.TypeGameServer, x.Name, x.Status, map[string]any{"game_type": x.GameType, "address": x.PublicAddress})
		}
	}
	if list, err := s.svc.Android.List(ctx, userID, false, ""); err == nil {
		for _, x := range list {
			add(x.ID, orchestrator.TypeAndroid, x.Name, x.Status, nil)
		}
	}
	if list, err := s.svc.Database.List(ctx, userID); err == nil {
		for _, x := range list {
			add(x.ID, orchestrator.TypeDatabase, x.Name, x.Status, map[string]any{"engine": x.Engine, "host": x.Host, "port": x.Port})
		}
	}
	if list, err := s.svc.Storage.ListBuckets(ctx, userID); err == nil {
		for _, x := range list {
			add(x.ID, orchestrator.TypeBucket, x.Name, "active", map[string]any{"objects": x.TotalObjects, "size_bytes": x.TotalSizeBytes})
		}
	}
	if s.svc.Domain != nil && s.svc.Domain.Enabled() {
		if list, err := s.svc.Domain.List(ctx, userID); err == nil {
			for _, x := range list {
				add(x.Domain.ID, orchestrator.TypeDomain, x.Domain.Hostname, string(x.LiveStatus), map[string]any{"target_port": x.Domain.TargetPort})
			}
		}
	}
	rels, _ := s.repos.Orch.ListUserRelationships(ctx, userID, 500)
	seen := map[string]bool{}
	for _, rel := range rels {
		if rel.RelatedResourceID == nil {
			continue
		}
		key := rel.ResourceID.String() + "|" + rel.Relation + "|" + rel.RelatedResourceID.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		if !nodes[rel.ResourceID] {
			add(rel.ResourceID, rel.ResourceType, rel.ResourceName, "unknown", nil)
		}
		if !nodes[*rel.RelatedResourceID] {
			add(*rel.RelatedResourceID, rel.RelatedResourceType, rel.RelatedResourceName, "unknown", nil)
		}
		view.Edges = append(view.Edges, TopologyEdge{
			From: rel.ResourceID, FromType: rel.ResourceType, To: *rel.RelatedResourceID,
			ToType: rel.RelatedResourceType, Relation: rel.Relation, ToName: rel.RelatedResourceName,
		})
	}
	return view, nil
}

// AgentRoles returns the logical agent roles and their tools.
func (s *OrchestratorUsecase) AgentRoles() []orchestrator.AgentRole {
	return orchestrator.AgentRoles(s.reg)
}

// link records a topology relationship between two resources of the user.
func (s *OrchestratorUsecase) link(ctx context.Context, tc *orchestrator.TaskContext, relation,
	fromType string, fromID uuid.UUID, fromName, toType string, toID uuid.UUID, toName string) {
	if fromID == uuid.Nil || toID == uuid.Nil {
		return
	}
	execID, taskID, planID := tc.ExecutionID, tc.TaskID, tc.PlanID
	rel := &models.ResourceRelationship{
		OwnerID: tc.UserID, ExecutionID: &execID, TaskID: &taskID, PlanID: &planID,
		Relation: relation, ResourceType: fromType, ResourceID: fromID, ResourceName: fromName,
		RelatedResourceType: toType, RelatedResourceID: &toID, RelatedResourceName: toName,
	}
	if err := s.repos.Orch.RecordRelationship(ctx, rel); err != nil {
		return
	}
}
