package orchestrator

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Diagnostics: deterministic evidence collection and ranking. The model may
// explain a result, but the facts, the hypotheses and their ordering come
// from observed platform data.

// Fact is one observed value.
type Fact struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Status string `json:"status"` // ok | warning | critical | info
}

// Hypothesis is a possible cause with its supporting evidence.
type Hypothesis struct {
	Cause      string   `json:"cause"`
	Confidence string   `json:"confidence"` // high | medium | low
	Evidence   []string `json:"evidence"`
}

// Recommendation is a proposed remediation. It never runs on its own: an
// infrastructure-changing action requires a plan and approval.
type Recommendation struct {
	Action           string         `json:"action"`
	Label            string         `json:"label"`
	Risk             string         `json:"risk"`
	RequiresApproval bool           `json:"requires_approval"`
	Tool             string         `json:"tool"`
	Input            map[string]any `json:"input"`
}

// DiagnosticResult is the verdict for one resource.
type DiagnosticResult struct {
	ResourceType    string           `json:"resource_type"`
	ResourceID      uuid.UUID        `json:"resource_id"`
	Name            string           `json:"name"`
	Health          string           `json:"health"` // healthy | degraded | critical | unknown
	ObservedAt      time.Time        `json:"observed_at"`
	Facts           []Fact           `json:"facts"`
	Hypotheses      []Hypothesis     `json:"hypotheses"`
	Recommendations []Recommendation `json:"recommendations"`
}

// DiagnosticInput is the evidence the use case collected for a resource.
type DiagnosticInput struct {
	ResourceType  string
	ResourceID    uuid.UUID
	Name          string
	Status        string
	Running       bool
	PrivateIP     string
	CPUPercent    float64
	MemoryPercent float64
	DiskPercent   float64
	HasMetrics    bool
	LastBackupAt  *time.Time
	AgeDays       int
	ErrorMessage  string
}

// Diagnose applies deterministic rules to the evidence. Metrics at or above
// 90% are critical, at or above 75% are warnings, and a resource that is not
// running is critical (its services are unavailable).
func Diagnose(in DiagnosticInput, now time.Time) DiagnosticResult {
	res := DiagnosticResult{
		ResourceType: in.ResourceType, ResourceID: in.ResourceID, Name: in.Name,
		ObservedAt: now.UTC(), Facts: []Fact{}, Hypotheses: []Hypothesis{}, Recommendations: []Recommendation{},
	}
	critical, warning := false, false
	addFact := func(name, value, status string) {
		res.Facts = append(res.Facts, Fact{Name: name, Value: value, Status: status})
		switch status {
		case "critical":
			critical = true
		case "warning":
			warning = true
		}
	}
	addHypothesis := func(cause, confidence string, evidence ...string) {
		res.Hypotheses = append(res.Hypotheses, Hypothesis{Cause: cause, Confidence: confidence, Evidence: evidence})
	}
	recommend := func(action, label, risk, tool string, input map[string]any) {
		res.Recommendations = append(res.Recommendations, Recommendation{
			Action: action, Label: label, Risk: risk, RequiresApproval: IsWrite(risk), Tool: tool, Input: input,
		})
	}

	status := in.Status
	if status == "" {
		status = "unknown"
	}
	addFact("status", status, statusFact(status))
	switch status {
	case "failed", "error":
		addHypothesis("the resource failed to provision or the runtime crashed", "high", "recorded status is "+status)
		recommend("restart", "Restart the resource", RiskMedium, ToolServicePower, map[string]any{
			"resource_type": in.ResourceType, "resource_id": in.ResourceID.String(), "action": ActionRestart,
		})
		recommend("logs", "Inspect logs before restarting", RiskRead, ToolServiceLogs, map[string]any{
			"resource_type": in.ResourceType, "resource_id": in.ResourceID.String(),
		})
	case "stopped":
		addHypothesis("the resource is stopped", "high", "recorded status is stopped")
		recommend("start", "Start the resource", RiskMedium, ToolServicePower, map[string]any{
			"resource_type": in.ResourceType, "resource_id": in.ResourceID.String(), "action": ActionStart,
		})
	case "provisioning", "building", "deploying", "installing", "starting":
		addFact("transition", "in progress", "info")
		addHypothesis("the resource is still being provisioned", "high", "recorded status is "+status)
	}

	if in.HasMetrics {
		addFact("cpu", fmt.Sprintf("%.1f%%", in.CPUPercent), threshold(in.CPUPercent))
		addFact("memory", fmt.Sprintf("%.1f%%", in.MemoryPercent), threshold(in.MemoryPercent))
		addFact("disk", fmt.Sprintf("%.1f%%", in.DiskPercent), threshold(in.DiskPercent))
		if in.CPUPercent >= 75 {
			confidence := "medium"
			if in.CPUPercent >= 90 {
				confidence = "high"
			}
			addHypothesis("CPU saturation (a busy process or too-small a plan)", confidence,
				fmt.Sprintf("CPU utilization is %.1f%%", in.CPUPercent))
			recommend("restart", "Restart to clear a runaway process", RiskHigh, ToolServicePower, map[string]any{
				"resource_type": in.ResourceType, "resource_id": in.ResourceID.String(), "action": ActionRestart,
			})
			if in.ResourceType == TypeVPS {
				recommend("resize", "Review the VPS size (resize is a separate approved plan)", RiskHigh, ToolVPSResize, map[string]any{
					"instance_id": in.ResourceID.String(),
				})
			}
		}
		if in.MemoryPercent >= 80 {
			confidence := "medium"
			if in.MemoryPercent >= 92 {
				confidence = "high"
			}
			addHypothesis("memory pressure (leak, cache growth or too-small a plan)", confidence,
				fmt.Sprintf("memory utilization is %.1f%%", in.MemoryPercent))
		}
		if in.DiskPercent >= 80 {
			addHypothesis("disk pressure (logs, world data or database growth)", "medium",
				fmt.Sprintf("disk utilization is %.1f%%", in.DiskPercent))
			recommend("free_space", "Clean up data or grow the volume", RiskHigh, "", nil)
		}
	} else {
		addFact("metrics", "not observable", "info")
	}

	if in.ResourceType == TypeVPS && in.Running && in.PrivateIP == "" {
		addFact("private_ip", "missing", "warning")
		addHypothesis("the guest agent has not reported a network address", "medium",
			"the instance is running but no private IP is recorded")
	}
	if in.LastBackupAt != nil {
		age := now.Sub(*in.LastBackupAt)
		addFact("last_backup", age.Round(time.Hour).String()+" ago", "info")
		if age > 7*24*time.Hour {
			addFact("backup_freshness", "older than 7 days", "warning")
			addHypothesis("no recent backup", "high", "the last backup is older than 7 days")
		}
	} else if in.ResourceType == TypeGameServer || in.ResourceType == TypeDatabase || in.ResourceType == TypeAndroid {
		addFact("last_backup", "never", "warning")
	}
	if in.ErrorMessage != "" {
		addFact("error", in.ErrorMessage, "critical")
	}

	switch {
	case critical:
		res.Health = "critical"
	case warning:
		res.Health = "degraded"
	case in.HasMetrics || in.Running:
		res.Health = "healthy"
	default:
		res.Health = "unknown"
	}
	return res
}

func statusFact(status string) string {
	switch status {
	case "running", "active":
		return "ok"
	case "failed", "error", "deleting", "stopped":
		return "critical"
	case "suspended":
		return "warning"
	}
	return "info"
}

func threshold(pct float64) string {
	switch {
	case pct >= 90:
		return "critical"
	case pct >= 75:
		return "warning"
	}
	return "ok"
}
