package usecase

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
)

// Phase 2-4 chat operations: conversational diagnostics, the advisor,
// template deployment and managed database creation. Everything here is
// deterministic and read-only except for the two plan-creating workflows;
// neither executes anything: they only file an approvable plan.

const templateParamPrefix = "tpl_param_"

// SetDatabaseService wires managed databases into the chat agent. It is a
// setter because the AI agent is built before the database use case.
func (s *AIAgentUsecase) SetDatabaseService(db *DatabaseUsecase) {
	s.databaseService = db
}

// ownedResourceRef is one resource the caller owns, across every service.
type ownedResourceRef struct {
	Type   string
	ID     uuid.UUID
	Name   string
	Status string
}

// ownedResources lists every resource of the caller. Errors are ignored per
// service: chat degrades to the resources it can read.
func (s *AIAgentUsecase) ownedResources(ctx context.Context, userID uuid.UUID) []ownedResourceRef {
	out := []ownedResourceRef{}
	if s.instanceService != nil {
		if list, err := s.instanceService.List(ctx, userID, false, ""); err == nil {
			for _, x := range list {
				out = append(out, ownedResourceRef{Type: orchestrator.TypeVPS, ID: x.ID, Name: x.Name, Status: x.Status})
			}
		}
	}
	if s.appService != nil {
		if list, err := s.appService.List(ctx, userID, false); err == nil {
			for _, x := range list {
				out = append(out, ownedResourceRef{Type: orchestrator.TypeApp, ID: x.ID, Name: x.Name, Status: x.Status})
			}
		}
	}
	if s.automationService != nil {
		if list, err := s.automationService.List(ctx, userID, false); err == nil {
			for _, x := range list {
				out = append(out, ownedResourceRef{Type: orchestrator.TypeN8n, ID: x.ID, Name: x.Name, Status: x.Status})
			}
		}
	}
	if s.openClawService != nil {
		if list, err := s.openClawService.List(ctx, userID, false); err == nil {
			for _, x := range list {
				out = append(out, ownedResourceRef{Type: orchestrator.TypeOpenClaw, ID: x.ID, Name: x.Name, Status: x.Status})
			}
		}
	}
	if s.gameServerService != nil {
		if list, err := s.gameServerService.List(ctx, userID, false, ""); err == nil {
			for _, x := range list {
				out = append(out, ownedResourceRef{Type: orchestrator.TypeGameServer, ID: x.ID, Name: x.Name, Status: x.Status})
			}
		}
	}
	if s.androidService != nil {
		if list, err := s.androidService.List(ctx, userID, false, ""); err == nil {
			for _, x := range list {
				out = append(out, ownedResourceRef{Type: orchestrator.TypeAndroid, ID: x.ID, Name: x.Name, Status: x.Status})
			}
		}
	}
	if s.databaseService != nil {
		if list, err := s.databaseService.List(ctx, userID); err == nil {
			for _, x := range list {
				out = append(out, ownedResourceRef{Type: orchestrator.TypeDatabase, ID: x.ID, Name: x.Name, Status: x.Status})
			}
		}
	}
	if s.storageService != nil {
		if list, err := s.storageService.ListBuckets(ctx, userID); err == nil {
			for _, x := range list {
				out = append(out, ownedResourceRef{Type: orchestrator.TypeBucket, ID: x.ID, Name: x.Name, Status: "active"})
			}
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Type != out[j].Type {
			return out[i].Type < out[j].Type
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// resourceTypeLabel is the human label of a resource type in chat replies.
func resourceTypeLabel(typ string) string {
	switch typ {
	case orchestrator.TypeVPS:
		return "VPS"
	case orchestrator.TypeApp:
		return "application"
	case orchestrator.TypeN8n:
		return "n8n automation"
	case orchestrator.TypeOpenClaw:
		return "OpenClaw agent"
	case orchestrator.TypeGameServer:
		return "game server"
	case orchestrator.TypeAndroid:
		return "Android instance"
	case orchestrator.TypeDatabase:
		return "database"
	case orchestrator.TypeBucket:
		return "storage bucket"
	case orchestrator.TypeDomain:
		return "domain"
	}
	return typ
}

// resourceTypeHint returns the resource type a message explicitly mentions,
// or "" when it does not say.
func resourceTypeHint(msg string) string {
	switch {
	case containsAny(msg, "database", "postgres", "mysql", "redis", "mongo"):
		return orchestrator.TypeDatabase
	case containsAny(msg, "bucket", "object storage", "s3 ", "minio"):
		return orchestrator.TypeBucket
	case containsAny(msg, "application", "web app", "web site", "website", "app "):
		return orchestrator.TypeApp
	case containsAny(msg, "n8n", "automation", "workflow"):
		return orchestrator.TypeN8n
	case containsAny(msg, "openclaw", "ai agent"):
		return orchestrator.TypeOpenClaw
	case containsAny(msg, "game server", "minecraft", "terraria"):
		return orchestrator.TypeGameServer
	case containsAny(msg, "android", "waydroid"):
		return orchestrator.TypeAndroid
	case containsAny(msg, "vps", " vm", "virtual machine", "compute instance", "instance"):
		return orchestrator.TypeVPS
	}
	return ""
}

// matchOwnedResource picks the resource whose name is mentioned in msg. The
// longest name wins ("web-01" beats "web"); when two types share a name, the
// type mentioned in the message wins.
func matchOwnedResource(refs []ownedResourceRef, msg string) *ownedResourceRef {
	hint := resourceTypeHint(msg)
	var best *ownedResourceRef
	bestScore := -1
	for i := range refs {
		ref := &refs[i]
		if !mentionsName(msg, ref.Name) {
			continue
		}
		score := len(ref.Name)
		if hint != "" && ref.Type == hint {
			score += 1000
		}
		if score > bestScore {
			best, bestScore = ref, score
		}
	}
	return best
}

func isAccountHealthRequest(msg string) bool {
	return containsAny(msg, "all resources", "everything", "whole account", "my account", "entire account",
		"infrastructure health", "health check all", "diagnose all", "all my resources")
}

// handleDiagnostics answers a troubleshooting message with the deterministic
// diagnostics engine: facts, ranked causes and remediation recommendations.
// It never changes anything.
func (s *AIAgentUsecase) handleDiagnostics(ctx context.Context, userID uuid.UUID, msg string) string {
	const footer = "\n\n_This is a read-only diagnosis: nothing was planned or changed. Remediations are proposed for approval on the resource's page in [Operations](/dashboard/operations)._"
	if s.orch == nil {
		return "Diagnostics are not available on this deployment. Open [Cloud Resources](/dashboard/resources) to inspect a resource." + footer
	}
	refs := s.ownedResources(ctx, userID)
	if len(refs) == 0 {
		return "You have no resources to diagnose yet. Ask me to create a VPS, application, database or another service first." + footer
	}
	if isAccountHealthRequest(msg) {
		return s.diagnoseAllResources(ctx, userID, refs) + footer
	}
	ref := matchOwnedResource(refs, msg)
	if ref == nil {
		return "I could not match a resource name in your message.\n\n" + formatResourceChoices(refs) +
			"\n\nAsk me e.g. `diagnose web-01` or `health check all resources`." + footer
	}
	result, err := s.orch.Diagnose(ctx, userID, ref.Type, ref.ID)
	if err != nil {
		return fmt.Sprintf("I could not diagnose **%s**: %s", ref.Name, err.Error()) + footer
	}
	return formatDiagnostic(result) + footer
}

func formatResourceChoices(refs []ownedResourceRef) string {
	const max = 30
	var b strings.Builder
	fmt.Fprintf(&b, "**Your resources (%d)**\n\n| Name | Type | Status |\n|---|---|---|\n", len(refs))
	for i, ref := range refs {
		if i >= max {
			fmt.Fprintf(&b, "| _and %d more_ | | |\n", len(refs)-max)
			break
		}
		fmt.Fprintf(&b, "| %s | %s | %s |\n", ref.Name, resourceTypeLabel(ref.Type), ref.Status)
	}
	return strings.TrimRight(b.String(), "\n")
}

func healthIcon(health string) string {
	switch health {
	case "healthy":
		return "🟢"
	case "degraded":
		return "🟡"
	case "critical":
		return "🔴"
	}
	return "⚪"
}

// formatDiagnostic renders one diagnostic result as Markdown.
func formatDiagnostic(res *orchestrator.DiagnosticResult) string {
	if res == nil {
		return "The diagnostic returned no result."
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### %s %s — %s\n\n", healthIcon(res.Health), res.Name, strings.ToUpper(res.Health))
	fmt.Fprintf(&b, "**Resource:** %s · `%s`\n\n", resourceTypeLabel(res.ResourceType), res.ResourceID)

	b.WriteString("**Observed facts**\n")
	if len(res.Facts) == 0 {
		b.WriteString("- no facts were collected\n")
	}
	for _, fact := range res.Facts {
		fmt.Fprintf(&b, "- **%s:** %s (%s)\n", fact.Name, fact.Value, fact.Status)
	}

	b.WriteString("\n**Likely causes**\n")
	if len(res.Hypotheses) == 0 {
		b.WriteString("- no failure hypothesis matched the observed state\n")
	}
	for i, h := range res.Hypotheses {
		fmt.Fprintf(&b, "%d. %s _(%s confidence)_", i+1, h.Cause, h.Confidence)
		if len(h.Evidence) > 0 {
			fmt.Fprintf(&b, " — %s", strings.Join(h.Evidence, "; "))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n**Recommended next steps**\n")
	if len(res.Recommendations) == 0 {
		b.WriteString("- keep monitoring; no remediation is suggested\n")
	}
	for _, rec := range res.Recommendations {
		approval := ""
		if rec.RequiresApproval {
			approval = " · requires an approved plan"
		} else if rec.Risk != "" {
			approval = " · read-only"
		}
		fmt.Fprintf(&b, "- **%s** (%s risk%s)\n", rec.Label, rec.Risk, approval)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (s *AIAgentUsecase) diagnoseAllResources(ctx context.Context, userID uuid.UUID, refs []ownedResourceRef) string {
	const max = 25
	if len(refs) > max {
		refs = refs[:max]
	}
	results := make([]orchestrator.DiagnosticResult, 0, len(refs))
	for _, ref := range refs {
		if res, err := s.orch.Diagnose(ctx, userID, ref.Type, ref.ID); err == nil {
			results = append(results, *res)
		}
	}
	if len(results) == 0 {
		return "I could not collect diagnostics for your resources right now."
	}
	rank := map[string]int{"critical": 0, "degraded": 1, "unknown": 2, "healthy": 3}
	sort.SliceStable(results, func(i, j int) bool {
		if rank[results[i].Health] != rank[results[j].Health] {
			return rank[results[i].Health] < rank[results[j].Health]
		}
		return results[i].Name < results[j].Name
	})
	counts := map[string]int{}
	for _, res := range results {
		counts[res.Health]++
	}
	var b strings.Builder
	fmt.Fprintf(&b, "### 🩺 Infrastructure health\n\n**%d resources checked:** %d healthy · %d degraded · %d critical\n\n",
		len(results), counts["healthy"], counts["degraded"], counts["critical"])
	b.WriteString("| Resource | Type | Health | Top cause |\n|---|---|---|---|\n")
	for _, res := range results {
		cause := "-"
		if len(res.Hypotheses) > 0 {
			cause = res.Hypotheses[0].Cause
		}
		fmt.Fprintf(&b, "| %s | %s | %s %s | %s |\n", res.Name, resourceTypeLabel(res.ResourceType), healthIcon(res.Health), res.Health, cause)
	}
	return strings.TrimRight(b.String(), "\n")
}

// handleAdvisor answers with the deterministic optimization and cost report.
func (s *AIAgentUsecase) handleAdvisor(ctx context.Context, userID uuid.UUID) string {
	if s.orch == nil {
		return "The cost advisor is not available on this deployment. See [Usage & Quotas](/dashboard/usage)."
	}
	report, err := s.orch.Optimize(ctx, userID)
	if err != nil {
		return "I could not build the advisor report right now: " + err.Error()
	}
	return formatAdvisorReport(report)
}

// formatAdvisorReport renders an advisor report as Markdown.
func formatAdvisorReport(report *AdvisorReport) string {
	if report == nil {
		return "The advisor returned no report."
	}
	currency := report.Currency
	if currency == "" {
		currency = "USD"
	}
	var b strings.Builder
	b.WriteString("### 💡 Optimization & cost advisor\n\n")
	fmt.Fprintf(&b, "**Estimated monthly total:** %.2f %s\n\n", report.MonthlyTotal, currency)

	if len(report.CostResources) > 0 {
		b.WriteString("**Estimated cost by resource**\n\n| Resource | Type | Monthly |\n|---|---|---|\n")
		items := append([]orchestrator.CostItem(nil), report.CostResources...)
		sort.SliceStable(items, func(i, j int) bool { return items[i].Monthly > items[j].Monthly })
		limit := len(items)
		if limit > 10 {
			limit = 10
		}
		for _, item := range items[:limit] {
			fmt.Fprintf(&b, "| %s | %s | %.2f %s |\n", item.Name, resourceTypeLabel(item.ResourceType), item.Monthly, currency)
		}
		if len(items) > limit {
			fmt.Fprintf(&b, "\n_%d more resources are included in the total._\n", len(items)-limit)
		}
		b.WriteString("\n")
	}

	fmt.Fprintf(&b, "**Recommendations (%d)**\n\n", len(report.Optimizations))
	if len(report.Optimizations) == 0 {
		b.WriteString("- No waste, pressure or idle capacity rule matched your resources.\n")
	}
	for _, o := range report.Optimizations {
		fmt.Fprintf(&b, "- **%s** — %s (`%s`)\n", o.Title, o.ResourceName, o.Category)
		if o.Current != "" || o.Observed != "" {
			fmt.Fprintf(&b, "  - Current: %s · observed: %s\n", o.Current, o.Observed)
		}
		if o.Proposed != "" {
			fmt.Fprintf(&b, "  - Proposed: %s\n", o.Proposed)
		}
		fmt.Fprintf(&b, "  - Benefit: %s · risk: %s%s\n", o.ExpectedBenefit, o.Risk, map[bool]string{true: " · requires an approved plan"}[o.RequiresApproval])
	}

	if report.Quota != nil {
		fmt.Fprintf(&b, "\n**Quota:** %d/%d vCPU · %s/%s memory · %d/%d GB disk · %d/%d instances\n",
			report.Quota.UsedVCPUs, report.Quota.MaxVCPUs,
			formatMB(report.Quota.UsedMemoryMB), formatMB(report.Quota.MaxMemoryMB),
			report.Quota.UsedStorageGB, report.Quota.MaxStorageGB,
			report.Quota.UsedInstances, report.Quota.MaxInstances)
	}
	b.WriteString("\n_This advice is read-only: every infrastructure change becomes a plan that you approve. Estimates use the platform rate card and are not an invoice._")
	return b.String()
}

// --- Managed databases -----------------------------------------------------

func databaseEngineFromText(msg string) string {
	switch {
	case containsAny(msg, "postgres", "postgresql", "pgsql"):
		return models.DatabaseEnginePostgres
	case containsAny(msg, "mysql", "mariadb"):
		return models.DatabaseEngineMySQL
	case containsAny(msg, "redis"):
		return models.DatabaseEngineRedis
	case containsAny(msg, "mongo"):
		return models.DatabaseEngineMongo
	}
	return ""
}

func databasePlanFromText(msg string) string {
	switch {
	case containsAny(msg, "starter", "nano", "small"):
		return "starter"
	case containsAny(msg, "standard", "medium"):
		return "standard"
	case containsAny(msg, "power", "large"):
		return "power"
	}
	return ""
}

func databasePlanByName(name string) DatabasePlan {
	for _, p := range AvailableDatabasePlans {
		if p.ID == name {
			return p
		}
	}
	return AvailableDatabasePlans[0]
}

func engineLabel(engine string) string {
	switch engine {
	case models.DatabaseEnginePostgres:
		return "PostgreSQL"
	case models.DatabaseEngineMySQL:
		return "MySQL"
	case models.DatabaseEngineRedis:
		return "Redis"
	case models.DatabaseEngineMongo:
		return "MongoDB"
	}
	return engine
}

func sanitizeDatabaseName(raw, engine string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	name = strings.NewReplacer(" ", "-", "_", "-", ".", "-").Replace(name)
	var b strings.Builder
	for _, r := range name {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '-' {
			b.WriteRune(r)
		}
	}
	name = strings.Trim(b.String(), "-")
	if len(name) > 63 {
		name = strings.TrimRight(name[:63], "-")
	}
	if !databaseNameRegex.MatchString(name) {
		name = fmt.Sprintf("db-%s-%d", engine, time.Now().Unix()%10000)
	}
	return name
}

// handleDatabaseWorkflow proposes a managed database plan. Credentials are
// generated during execution and shown once; the plan itself never contains
// a password.
func (s *AIAgentUsecase) handleDatabaseWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	state *models.AIConversationState,
	reqs map[string]string,
	msg string,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	if s.databaseService == nil {
		state.State = models.StateAIFailed
		return "Managed databases are not enabled on this deployment.", models.UIComponentsPayload{}, nil, ""
	}

	engine := reqs["db_engine"]
	if engine == "" {
		engine = databaseEngineFromText(msg)
	}
	if engine == "" {
		state.State = models.StateClarifyingRequirements
		q := models.StructuredQuestion{
			ID:          "q_db_engine",
			Key:         "db_engine",
			Title:       "Which database engine would you like?",
			Description: "The managed service provisions the engine with encrypted, generated credentials and automatic backups.",
			Type:        "single_choice",
			Required:    true,
			Options: []models.StructuredQuestionOption{
				{ID: "postgres", Label: "PostgreSQL 16", Value: models.DatabaseEnginePostgres, Description: "General-purpose relational database", IsDefault: true},
				{ID: "mysql", Label: "MySQL 8.4", Value: models.DatabaseEngineMySQL, Description: "Widely compatible relational database"},
				{ID: "redis", Label: "Redis 7.4", Value: models.DatabaseEngineRedis, Description: "In-memory cache and key-value store"},
				{ID: "mongo", Label: "MongoDB 7.0", Value: models.DatabaseEngineMongo, Description: "Document database for flexible schemas"},
			},
		}
		return q.Title, models.UIComponentsPayload{Questions: []models.StructuredQuestion{q}}, nil, ""
	}
	reqs["db_engine"] = engine

	planName := reqs["db_plan"]
	if planName == "" {
		planName = databasePlanFromText(msg)
	}
	if planName == "" {
		state.State = models.StateClarifyingRequirements
		q := models.StructuredQuestion{
			ID:          "q_db_plan",
			Key:         "db_plan",
			Title:       "Which sizing plan should the database use?",
			Description: "You can resize later through a new approved plan.",
			Type:        "single_choice",
			Required:    true,
			Options: []models.StructuredQuestionOption{
				{ID: "starter", Label: "Starter — 1 vCPU, 1 GB RAM, 10 GB", Value: "starter", Description: "Small apps, staging and development", IsDefault: true},
				{ID: "standard", Label: "Standard — 2 vCPU, 4 GB RAM, 50 GB", Value: "standard", Description: "Production APIs and small workloads"},
				{ID: "power", Label: "Power — 4 vCPU, 8 GB RAM, 200 GB", Value: "power", Description: "High-traffic production databases"},
			},
		}
		return q.Title, models.UIComponentsPayload{Questions: []models.StructuredQuestion{q}}, nil, ""
	}
	reqs["db_plan"] = planName

	size := databasePlanByName(planName)
	name := sanitizeDatabaseName(reqs["name"], engine)
	reqs["name"] = name
	databaseName := strings.ReplaceAll(name, "-", "_")

	config := map[string]any{
		"name": name, "engine": engine, "plan_name": size.ID,
		"vcpus": size.VCPUs, "memory_mb": size.MemoryMB, "storage_gb": size.StorageGB,
		"database_name": databaseName, "auto_backup": true,
	}
	spec := orchestrator.Specification{Intent: "create_database", Resources: []orchestrator.ResourceSpec{{
		Key: name, Type: orchestrator.TypeDatabase, Action: orchestrator.ActionCreate, Config: config,
	}}}

	title := fmt.Sprintf("Provision %s database %s", engineLabel(engine), name)
	summary := fmt.Sprintf("Managed %s %s on %d vCPU / %d MB RAM / %d GB storage with automatic backups",
		engineLabel(engine), size.Name, size.VCPUs, size.MemoryMB, size.StorageGB)
	view := &models.StructuredPlanView{
		Title: title, Intent: "create_database", Summary: summary,
		Reversible: true, RequiresConfirmation: true, Status: "awaiting_confirmation",
		EstimatedAllocations: map[string]interface{}{
			"vcpus": size.VCPUs, "memory_mb": size.MemoryMB, "storage_gb": size.StorageGB,
			"plan": size.Name, "engine": engine,
		},
		Resources: []models.StructuredResourceView{{
			Type: "Managed Database", Name: name, Summary: summary, Order: 1, Status: "pending",
			Details: map[string]interface{}{
				"Engine":            engineLabel(engine),
				"Sizing Plan":       size.Name,
				"vCPU":              size.VCPUs,
				"Memory":            fmt.Sprintf("%d MB", size.MemoryMB),
				"Storage":           fmt.Sprintf("%d GB", size.StorageGB),
				"Database Name":     databaseName,
				"Automatic Backups": "Enabled",
				"Credentials":       "Generated during provisioning and shown once",
			},
		}},
	}

	card, planID, err := s.proposePlan(ctx, convID, userID, state, "create_database", title, summary, "", spec, view)
	if err != nil {
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	reply := fmt.Sprintf("I have prepared the plan for **%s** (`%s`, %s). "+
		"After approval the database is provisioned, a strong password is generated and encrypted, and the credentials are shown to you once. "+
		"You can retrieve them later from [Operations · Databases](/dashboard/operations?tab=databases).",
		title, name, size.Name)
	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, planID
}

// --- Infrastructure templates ---------------------------------------------

var templatePairRegex = regexp.MustCompile(`([A-Za-z_][A-Za-z0-9_]*)\s*[=:]\s*["']?([^"'\s,;]+)`)

func parseTemplateParamPairs(msg string, reqs map[string]string) {
	for _, match := range templatePairRegex.FindAllStringSubmatch(msg, -1) {
		reqs[templateParamPrefix+match[1]] = strings.TrimSpace(match[2])
	}
}

// matchInfraTemplate picks the visible template whose name is mentioned in
// msg. The longest name wins.
func matchInfraTemplate(templates []TemplateView, msg string) *TemplateView {
	msg = strings.ToLower(strings.TrimSpace(msg))
	if msg == "" {
		return nil
	}
	var best *TemplateView
	for i := range templates {
		name := strings.ToLower(strings.TrimSpace(templates[i].Name))
		if name == "" || !strings.Contains(msg, name) {
			continue
		}
		if best == nil || len(name) > len(best.Name) {
			best = &templates[i]
		}
	}
	return best
}

func templateParameterQuestion(p TemplateParameter) models.StructuredQuestion {
	q := models.StructuredQuestion{
		ID: "q_tpl_" + p.Key, Key: templateParamPrefix + p.Key, Title: p.Label,
		Description: p.Description, Type: "text", Required: p.Required,
	}
	switch p.Type {
	case models.TemplateParamEnum:
		q.Type = "single_choice"
		for i, value := range p.Enum {
			q.Options = append(q.Options, models.StructuredQuestionOption{
				ID: value, Label: value, Value: value, IsDefault: i == 0,
			})
		}
	case models.TemplateParamBool:
		q.Type = "single_choice"
		q.Options = []models.StructuredQuestionOption{
			{ID: "true", Label: "Yes", Value: "true", IsDefault: true},
			{ID: "false", Label: "No", Value: "false"},
		}
	}
	return q
}

// templateParamValue coerces a chat value to the type the template declares.
func templateParamValue(p TemplateParameter, raw string) (any, error) {
	raw = strings.TrimSpace(raw)
	switch p.Type {
	case models.TemplateParamInt:
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, ValidationError("parameters."+p.Key, "must be an integer")
		}
		if p.Min != nil && n < *p.Min {
			return nil, ValidationError("parameters."+p.Key, fmt.Sprintf("must be at least %d", *p.Min))
		}
		if p.Max != nil && n > *p.Max {
			return nil, ValidationError("parameters."+p.Key, fmt.Sprintf("must be at most %d", *p.Max))
		}
		return n, nil
	case models.TemplateParamBool:
		switch strings.ToLower(raw) {
		case "true", "yes", "1", "on":
			return true, nil
		case "false", "no", "0", "off":
			return false, nil
		}
		return nil, ValidationError("parameters."+p.Key, "must be true or false")
	case models.TemplateParamEnum:
		for _, allowed := range p.Enum {
			if strings.EqualFold(allowed, raw) {
				return allowed, nil
			}
		}
		return nil, ValidationError("parameters."+p.Key, "must be one of "+strings.Join(p.Enum, ", "))
	}
	return raw, nil
}

// handleTemplateWorkflow asks for the template's required parameters and
// files an approvable plan from it. It never writes a plan outside the
// current conversation.
func (s *AIAgentUsecase) handleTemplateWorkflow(
	ctx context.Context,
	convID, userID uuid.UUID,
	isAdmin bool,
	state *models.AIConversationState,
	reqs map[string]string,
	msg string,
) (string, models.UIComponentsPayload, *models.StructuredPlanView, string) {
	if s.orch == nil {
		state.State = models.StateAIFailed
		return "Infrastructure templates are not available on this deployment.", models.UIComponentsPayload{}, nil, ""
	}
	if containsAny(msg, "save template", "create template", "new template") && !containsAny(msg, "from template") {
		return "Templates are saved from the [Operations · Templates](/dashboard/operations?tab=templates) page, where you can validate and version a parameterized specification. Once saved, ask me to *deploy the <template name> template*.",
			models.UIComponentsPayload{}, nil, ""
	}

	templates, err := s.orch.ListTemplates(ctx, userID)
	if err != nil {
		return "I could not read your templates right now: " + err.Error(), models.UIComponentsPayload{}, nil, ""
	}
	if len(templates) == 0 {
		state.State = models.StateCompleted
		return "You do not have any infrastructure templates yet. Create one from [Operations · Templates](/dashboard/operations?tab=templates), then ask me to deploy it.",
			models.UIComponentsPayload{}, nil, ""
	}

	var tpl *TemplateView
	if raw := reqs["infra_template_id"]; raw != "" {
		if id, err := uuid.Parse(raw); err == nil {
			for i := range templates {
				if templates[i].ID == id {
					tpl = &templates[i]
					break
				}
			}
		}
	}
	if tpl == nil {
		tpl = matchInfraTemplate(templates, msg)
	}
	if tpl == nil {
		tpl = matchInfraTemplate(templates, reqs["template_name"])
	}
	if tpl == nil {
		state.State = models.StateClarifyingRequirements
		options := make([]models.StructuredQuestionOption, 0, len(templates))
		for i, t := range templates {
			options = append(options, models.StructuredQuestionOption{
				ID: t.ID.String(), Label: t.Name, Value: t.ID.String(),
				Description: fmt.Sprintf("v%d · %s", t.Version, t.Description),
				IsDefault:   i == 0,
			})
		}
		q := models.StructuredQuestion{
			ID: "q_template", Key: "infra_template_id", Title: "Which template would you like to deploy?",
			Description: "The plan stays in this conversation and still requires your approval.",
			Type:        "single_choice", Required: true, Options: options,
		}
		return q.Title, models.UIComponentsPayload{Questions: []models.StructuredQuestion{q}}, nil, ""
	}

	// A different template invalidates values collected for the previous one.
	if reqs["infra_template_id"] != tpl.ID.String() {
		for k := range reqs {
			if strings.HasPrefix(k, templateParamPrefix) {
				delete(reqs, k)
			}
		}
		reqs["template_awaiting_param"] = ""
	}
	reqs["infra_template_id"] = tpl.ID.String()
	reqs["template_name"] = tpl.Name

	parseTemplateParamPairs(msg, reqs)

	missing := make([]TemplateParameter, 0)
	for _, p := range tpl.Parameters {
		raw := strings.TrimSpace(reqs[templateParamPrefix+p.Key])
		if raw == "" && p.Required {
			missing = append(missing, p)
		}
	}
	// A plain text reply to a one-question prompt is that question's value.
	if len(missing) == 1 && reqs["template_awaiting_param"] == "1" &&
		!strings.ContainsAny(msg, "=:") && !strings.Contains(msg, "template") {
		if raw := strings.TrimSpace(msg); raw != "" {
			reqs[templateParamPrefix+missing[0].Key] = raw
			missing = missing[:0]
		}
	}
	if len(missing) > 0 {
		state.State = models.StateClarifyingRequirements
		reqs["template_awaiting_param"] = "1"
		questions := make([]models.StructuredQuestion, 0, len(missing))
		for _, p := range missing {
			questions = append(questions, templateParameterQuestion(p))
		}
		state.PendingQuestions = "[]"
		return fmt.Sprintf("The **%s** template needs a few parameter values before I can prepare its plan:", tpl.Name),
			models.UIComponentsPayload{Questions: questions}, nil, ""
	}

	delete(reqs, "template_awaiting_param")
	params := map[string]any{}
	for _, p := range tpl.Parameters {
		raw := strings.TrimSpace(reqs[templateParamPrefix+p.Key])
		if raw == "" {
			continue // optional: the declared default fills it
		}
		value, err := templateParamValue(p, raw)
		if err != nil {
			state.State = models.StateClarifyingRequirements
			return fmt.Sprintf("**%s**: %s", p.Label, err.Error()), models.UIComponentsPayload{
				Questions: []models.StructuredQuestion{templateParameterQuestion(p)},
			}, nil, ""
		}
		params[p.Key] = value
	}

	plan, rev, err := s.orch.DeployTemplateToConversation(ctx, userID, isAdmin, tpl.ID, params, convID)
	if err != nil {
		state.State = models.StateClarifyingRequirements
		return planError(err), models.UIComponentsPayload{}, nil, ""
	}
	state.State = models.StateAwaitingConfirmation
	state.PendingQuestions = "[]"
	state.ActivePlanID = &plan.ID

	card := planCard(plan, rev, nil)
	reply := fmt.Sprintf("I have prepared the plan to deploy template **%s** (v%d) in this conversation. Nothing runs until you approve it.", tpl.Name, tpl.Version)
	reply += validationNote(card)
	return reply, models.UIComponentsPayload{PlanProposal: card}, card, plan.ID.String()
}

// Explicit request predicates used by detectIntent.

func isDatabaseRequest(msg string) bool {
	if !containsAny(msg, "database", "postgres", "postgresql", "mysql", "redis", "mongo") {
		return false
	}
	// "deploy a website backed by postgres" is an app request; only an
	// explicit database noun or a create verb makes it a database request.
	if !strings.Contains(msg, "database") &&
		containsAny(msg, "website", "web app", "application", "n8n", "openclaw", "game server", "android", "vps", " vm", "bucket") {
		return false
	}
	return containsAny(msg, "create", "provision", "launch", "set up", "setup", "spin up", "new ", "deploy")
}

func isAdvisorRequest(msg string) bool {
	return containsAny(msg, "optimize", "optimization", "advisor", "rightsize", "right-size", "savings",
		"reduce cost", "reduce my cost", "monthly cost", "monthly spend", "my cost", "my spend",
		"billing", "bill ", "spending", "expensive")
}

func isTemplateRequest(msg string) bool {
	return strings.Contains(msg, "template") &&
		containsAny(msg, "deploy", "use", "launch", "apply", "provision", "spin up", "from ")
}
