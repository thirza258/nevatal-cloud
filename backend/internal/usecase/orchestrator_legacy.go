package usecase

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
)

// Legacy plan representation: ai_plan_resources rows with a resource type
// and a config_json, and plan_json. The orchestrator keeps them in sync with
// the current revision so pre-orchestrator clients keep working.

var legacyTypeOf = map[string]string{
	orchestrator.TypeVPS:        "virtual_machine",
	orchestrator.TypeBucket:     "storage_bucket",
	orchestrator.TypeApp:        "application",
	orchestrator.TypeN8n:        "n8n",
	orchestrator.TypeOpenClaw:   "openclaw",
	orchestrator.TypeGameServer: "game_server",
	orchestrator.TypeAndroid:    "android_instance",
}

// legacyResources renders a specification as ai_plan_resources rows.
func legacyResources(planID uuid.UUID, spec orchestrator.Specification) []models.AIPlanResource {
	out := make([]models.AIPlanResource, 0, len(spec.Resources))
	for i, res := range spec.Resources {
		typ := legacyTypeOf[res.Type]
		if typ == "" {
			typ = res.Type
		}
		if res.Action != orchestrator.ActionCreate {
			typ = typ + "." + res.Action
		}
		name := orchestrator.String(res.Config, "name")
		if name == "" {
			name = res.Key
		}
		if len(name) > 128 {
			name = name[:128]
		}
		out = append(out, models.AIPlanResource{
			ID:             uuid.New(),
			PlanID:         planID,
			ResourceType:   typ,
			ResourceName:   name,
			ConfigJSON:     orchestrator.MustJSON(res.Config),
			Status:         "pending",
			ExecutionOrder: i + 1,
			CreatedAt:      time.Now(),
		})
	}
	return out
}

// legacyPlanJSON renders the plan_json of a specification.
func legacyPlanJSON(spec orchestrator.Specification) string {
	return orchestrator.MustJSON(map[string]any{
		"intent":                spec.Intent,
		"specification":         spec,
		"requires_confirmation": true,
	})
}

// specFromLegacy rebuilds the specification of a plan created before the
// orchestrator from its resource rows (revision 1 backfill).
func specFromLegacy(plan *models.AIResourcePlan, resources []models.AIPlanResource) orchestrator.Specification {
	spec := orchestrator.Specification{Intent: plan.Intent, Resources: []orchestrator.ResourceSpec{}}
	seen := map[string]bool{}
	for _, r := range resources {
		var cfg map[string]any
		if err := orchestrator.DecodeJSON(r.ConfigJSON, &cfg); err != nil || cfg == nil {
			cfg = map[string]any{}
		}
		typ, action, config := legacyToSpec(r.ResourceType, cfg)
		if typ == "" {
			continue
		}
		key := strings.TrimSpace(r.ResourceName)
		if key == "" {
			key = typ
		}
		for seen[key] {
			key += "-x"
		}
		seen[key] = true
		spec.Resources = append(spec.Resources, orchestrator.ResourceSpec{Key: key, Type: typ, Action: action, Config: config})
	}
	return spec
}

func pick(cfg map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for _, k := range keys {
		if v, ok := cfg[k]; ok && v != nil {
			out[k] = v
		}
	}
	return out
}

// legacyToSpec maps a legacy resource row to a specification resource,
// keeping only the fields the tool accepts.
func legacyToSpec(resourceType string, cfg map[string]any) (string, string, map[string]any) {
	base, action, hasAction := strings.Cut(resourceType, ".")
	if !hasAction {
		action = orchestrator.ActionCreate
	}
	switch base {
	case "virtual_machine":
		if action == orchestrator.ActionCreate {
			return orchestrator.TypeVPS, action, pick(cfg, "name", "template_id", "plan_id", "vcpus", "memory_mb", "disk_gb", "ssh_key_ids")
		}
		return orchestrator.TypeVPS, action, cfg
	case "application":
		out := pick(cfg, "name", "framework", "branch")
		if repo := orchestrator.String(cfg, "repo"); repo != "" {
			out["repository_url"] = repo
		} else if repo := orchestrator.String(cfg, "repository_url"); repo != "" {
			out["repository_url"] = repo
		}
		return orchestrator.TypeApp, action, out
	case "n8n":
		return orchestrator.TypeN8n, action, pick(cfg, "name")
	case "openclaw":
		return orchestrator.TypeOpenClaw, action, pick(cfg, "name", "provider", "model")
	case "storage_bucket":
		if action == orchestrator.ActionCreate {
			return orchestrator.TypeBucket, action, pick(cfg, "name", "is_public", "description")
		}
		return orchestrator.TypeBucket, action, cfg
	case "game_server":
		return orchestrator.TypeGameServer, action, pick(cfg, "name", "game_type", "game_version", "server_software", "plan_name",
			"cpu_cores", "memory_mb", "storage_gb", "player_slots", "difficulty", "world_size", "motd")
	case "android_instance":
		return orchestrator.TypeAndroid, action, pick(cfg, "name", "image_id", "plan_name", "vcpus", "memory_mb", "storage_gb",
			"display_resolution", "display_density", "orientation", "adb_enabled")
	}
	return "", "", nil
}

// planCard is the chat card of a plan revision (plan_id and version are what
// the approve call binds to).
func planCard(plan *models.AIResourcePlan, rev *orchestrator.Revision, view *models.StructuredPlanView) *models.StructuredPlanView {
	if view == nil {
		view = &models.StructuredPlanView{}
	}
	view.PlanID = plan.ID.String()
	view.Version = rev.Version
	view.Status = plan.Status
	view.RequiresConfirmation = rev.RequiresApproval
	if view.Title == "" {
		view.Title = plan.Title
	}
	if view.Intent == "" {
		view.Intent = plan.Intent
	}
	if view.Summary == "" {
		view.Summary = rev.Summary
	}
	if view.EstimatedAllocations == nil {
		var est map[string]any
		b, _ := json.Marshal(rev.EstimatedResources)
		_ = json.Unmarshal(b, &est)
		view.EstimatedAllocations = est
	}
	if len(view.Resources) == 0 {
		for i, res := range rev.Specification.Resources {
			view.Resources = append(view.Resources, models.StructuredResourceView{
				Type: res.Type, Name: res.Key, Summary: res.Action + " " + res.Type, Details: res.Config, Order: i + 1, Status: "pending",
			})
		}
	}
	return view
}
