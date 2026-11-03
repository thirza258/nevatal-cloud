package orchestrator

import (
	"fmt"
	"math"
	"sort"
)

// Advisor: deterministic cost estimation and optimization recommendations
// over observed usage. Recommendations never change infrastructure; they
// carry the tool and input a user may turn into an approvable plan.

// CostRate is the monthly price list (in Currency) used for estimates.
type CostRate struct {
	Currency         string  `json:"currency"`
	VCPUPerMonth     float64 `json:"vcpu_per_month"`
	MemoryGBPerMonth float64 `json:"memory_gb_per_month"`
	DiskGBPerMonth   float64 `json:"disk_gb_per_month"`
	BucketGBPerMonth float64 `json:"bucket_gb_per_month"`
}

// CostItem is one billable resource.
type CostItem struct {
	ResourceType string  `json:"resource_type"`
	ID           string  `json:"id"`
	Name         string  `json:"name"`
	VCPUs        int     `json:"vcpus"`
	MemoryMB     int     `json:"memory_mb"`
	DiskGB       int     `json:"disk_gb"`
	SizeBytes    int64   `json:"size_bytes"`
	Status       string  `json:"status"`
	Monthly      float64 `json:"monthly"`
}

// CostReport is the estimated monthly cost of a set of resources.
type CostReport struct {
	Currency     string     `json:"currency"`
	MonthlyTotal float64    `json:"monthly_total"`
	Resources    []CostItem `json:"resources"`
	Notes        []string   `json:"notes"`
}

// EstimateCosts computes each item's monthly cost. Stopped resources are
// billed at 25% (storage only); running resources pay for vCPU, memory and
// disk.
func EstimateCosts(items []CostItem, rates CostRate) CostReport {
	report := CostReport{Currency: rates.Currency, Resources: make([]CostItem, 0, len(items)), Notes: []string{}}
	if report.Currency == "" {
		report.Currency = "USD"
	}
	for _, item := range items {
		cost := 0.0
		if item.ResourceType == TypeBucket {
			cost = float64(item.SizeBytes) / (1024 * 1024 * 1024) * rates.BucketGBPerMonth
		} else {
			cost = float64(item.VCPUs)*rates.VCPUPerMonth +
				float64(item.MemoryMB)/1024*rates.MemoryGBPerMonth +
				float64(item.DiskGB)*rates.DiskGBPerMonth
			if item.Status == "stopped" || item.Status == "suspended" {
				cost = float64(item.DiskGB) * rates.DiskGBPerMonth
			}
		}
		item.Monthly = round2(cost)
		report.MonthlyTotal += item.Monthly
		report.Resources = append(report.Resources, item)
	}
	report.MonthlyTotal = round2(report.MonthlyTotal)
	if len(rates.Currency) > 0 {
		report.Notes = append(report.Notes, "Estimates use the platform rate card; they are not an invoice.")
	}
	return report
}

// AdvisorResource is one observed resource for optimization.
type AdvisorResource struct {
	ResourceType  string  `json:"resource_type"`
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	Status        string  `json:"status"`
	VCPUs         int     `json:"vcpus"`
	MemoryMB      int     `json:"memory_mb"`
	DiskGB        int     `json:"disk_gb"`
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskPercent   float64 `json:"disk_percent"`
	HasMetrics    bool    `json:"has_metrics"`
	Objects       int64   `json:"objects"`
	SizeBytes     int64   `json:"size_bytes"`
	AgeDays       int     `json:"age_days"`
}

// Optimization is one recommendation.
type Optimization struct {
	ID               string         `json:"id"`
	Category         string         `json:"category"`
	Title            string         `json:"title"`
	ResourceType     string         `json:"resource_type"`
	ResourceID       string         `json:"resource_id"`
	ResourceName     string         `json:"resource_name"`
	Current          string         `json:"current"`
	Observed         string         `json:"observed"`
	Proposed         string         `json:"proposed"`
	ExpectedBenefit  string         `json:"expected_benefit"`
	Risk             string         `json:"risk"`
	RequiresApproval bool           `json:"requires_approval"`
	Tool             string         `json:"tool"`
	Input            map[string]any `json:"input"`
}

// GenerateOptimizations ranks deterministic rules:
//   - a resource stopped for a week is waste;
//   - consistently idle compute is oversized;
//   - memory or disk pressure is undersized;
//   - an empty bucket is unused.
//
// It never proposes an automatic change: every recommendation is a plan the
// user must approve.
func GenerateOptimizations(items []AdvisorResource) []Optimization {
	var out []Optimization
	add := func(o Optimization) {
		o.RequiresApproval = IsWrite(o.Risk)
		if o.ID == "" {
			o.ID = o.Category + ":" + o.ResourceType + ":" + o.ResourceID
		}
		out = append(out, o)
	}
	for _, r := range items {
		switch {
		case (r.Status == "stopped" || r.Status == "suspended") && r.AgeDays >= 7:
			add(Optimization{
				Category: "waste", Title: "Stopped resource still allocated",
				ResourceType: r.ResourceType, ResourceID: r.ID, ResourceName: r.Name,
				Current: r.Status, Observed: fmt.Sprintf("stopped for at least %d days", r.AgeDays),
				Proposed:        "Delete it or take a final backup and delete it",
				ExpectedBenefit: fmt.Sprintf("reclaim %d vCPU, %d MB RAM, %d GB disk", r.VCPUs, r.MemoryMB, r.DiskGB),
				Risk:            RiskDestructive,
				Tool:            ToolServiceDelete,
				Input:           map[string]any{"resource_type": r.ResourceType, "resource_id": r.ID},
			})
		case r.Status == "failed" || r.Status == "error":
			add(Optimization{
				Category: "reliability", Title: "Failed resource needs attention",
				ResourceType: r.ResourceType, ResourceID: r.ID, ResourceName: r.Name,
				Current: r.Status, Observed: "recorded status is " + r.Status,
				Proposed:        "Inspect logs, then redeploy or restart",
				ExpectedBenefit: "restore service availability",
				Risk:            RiskMedium,
				Tool:            ToolDiagnosticsResource,
				Input:           map[string]any{"resource_type": r.ResourceType, "resource_id": r.ID},
			})
		case r.ResourceType == TypeBucket && r.Objects == 0 && r.SizeBytes == 0:
			add(Optimization{
				Category: "waste", Title: "Unused bucket",
				ResourceType: r.ResourceType, ResourceID: r.ID, ResourceName: r.Name,
				Current: "empty", Observed: "0 objects",
				Proposed:        "Delete the bucket",
				ExpectedBenefit: "reduce clutter; no data would be lost",
				Risk:            RiskDestructive,
				Tool:            ToolBucketDelete,
				Input:           map[string]any{"bucket_id": r.ID},
			})
		default:
			if r.HasMetrics && r.Status == "running" {
				if r.CPUPercent > 0 && r.CPUPercent < 10 && r.MemoryPercent < 30 && r.MemoryMB > 1024 {
					proposedVCPU := maxInt(1, r.VCPUs/2)
					proposedMem := maxInt(512, r.MemoryMB/2)
					add(Optimization{
						Category: "rightsize", Title: "Consistently idle resource",
						ResourceType: r.ResourceType, ResourceID: r.ID, ResourceName: r.Name,
						Current:         fmt.Sprintf("%d vCPU, %d MB", r.VCPUs, r.MemoryMB),
						Observed:        fmt.Sprintf("CPU %.1f%%, memory %.1f%%", r.CPUPercent, r.MemoryPercent),
						Proposed:        fmt.Sprintf("%d vCPU, %d MB", proposedVCPU, proposedMem),
						ExpectedBenefit: "lower resource consumption",
						Risk:            RiskHigh, Tool: ToolVPSResize,
						Input: map[string]any{"instance_id": r.ID, "vcpus": proposedVCPU, "memory_mb": proposedMem, "disk_gb": r.DiskGB},
					})
				}
				if r.MemoryPercent >= 85 {
					add(Optimization{
						Category: "capacity", Title: "Memory pressure",
						ResourceType: r.ResourceType, ResourceID: r.ID, ResourceName: r.Name,
						Current:         fmt.Sprintf("%d MB", r.MemoryMB),
						Observed:        fmt.Sprintf("memory %.1f%%", r.MemoryPercent),
						Proposed:        fmt.Sprintf("%d MB", r.MemoryMB*2),
						ExpectedBenefit: "avoid OOM kills and restarts",
						Risk:            RiskHigh, Tool: ToolVPSResize,
						Input: map[string]any{"instance_id": r.ID, "vcpus": r.VCPUs, "memory_mb": r.MemoryMB * 2, "disk_gb": r.DiskGB},
					})
				}
				if r.DiskPercent >= 85 {
					add(Optimization{
						Category: "capacity", Title: "Disk pressure",
						ResourceType: r.ResourceType, ResourceID: r.ID, ResourceName: r.Name,
						Current:         fmt.Sprintf("%d GB", r.DiskGB),
						Observed:        fmt.Sprintf("disk %.1f%%", r.DiskPercent),
						Proposed:        fmt.Sprintf("%d GB", r.DiskGB*2),
						ExpectedBenefit: "avoid write failures",
						Risk:            RiskHigh, Tool: ToolVPSResize,
						Input: map[string]any{"instance_id": r.ID, "vcpus": r.VCPUs, "memory_mb": r.MemoryMB, "disk_gb": r.DiskGB * 2},
					})
				}
			}
		}
	}
	// Highest-impact first: waste, then capacity, then rightsize, then reliability.
	rank := map[string]int{"waste": 0, "capacity": 1, "rightsize": 2, "reliability": 3}
	sort.SliceStable(out, func(i, j int) bool { return rank[out[i].Category] < rank[out[j].Category] })
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
