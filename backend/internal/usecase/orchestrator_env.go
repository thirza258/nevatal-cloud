package usecase

import (
	"context"
	"errors"
	"strings"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/orchestrator"
	"github.com/nevatal/cloud/internal/repository"
)

// policyEnv answers the policy engine's questions from the repositories.
type policyEnv struct {
	s *OrchestratorUsecase
}

var _ orchestrator.Env = (*policyEnv)(nil)

func (e *policyEnv) IsAdmin(ctx context.Context, userID uuid.UUID) (bool, error) {
	u, err := e.s.repos.Users.FindByID(ctx, userID)
	if err != nil {
		return false, err
	}
	return u.Role == models.RoleAdmin, nil
}

// Normalize fills sizes and defaults the way the services would, so the
// estimate, the quota check and the spec hash see the values that will be
// provisioned.
func (e *policyEnv) Normalize(ctx context.Context, userID uuid.UUID, res *orchestrator.ResourceSpec) error {
	cfg := res.Config
	trimString := func(key string) {
		if v, ok := cfg[key].(string); ok {
			cfg[key] = strings.TrimSpace(v)
		}
	}
	trimString("name")
	if res.Action != orchestrator.ActionCreate {
		return nil
	}
	switch res.Type {
	case orchestrator.TypeVPS:
		if name, ok := cfg["name"].(string); ok {
			cfg["name"] = strings.ToLower(name)
		}
		if id, ok := orchestrator.UUID(cfg, "plan_id"); ok {
			if plan, err := e.s.repos.Plans.FindEnabledByID(ctx, id); err == nil {
				cfg["vcpus"], cfg["memory_mb"], cfg["disk_gb"] = plan.VCPUs, plan.MemoryMB, plan.DiskGB
			}
		}
	case orchestrator.TypeBucket:
		if name, ok := cfg["name"].(string); ok {
			cfg["name"] = strings.ToLower(name)
		}
	case orchestrator.TypeApp:
		if orchestrator.String(cfg, "framework") == "" {
			cfg["framework"] = "go"
		}
		if orchestrator.String(cfg, "branch") == "" {
			cfg["branch"] = "main"
		}
	case orchestrator.TypeOpenClaw:
		if orchestrator.String(cfg, "provider") == "" {
			cfg["provider"] = "openai"
		}
		if orchestrator.String(cfg, "model") == "" {
			cfg["model"] = "gpt-4o-mini"
		}
	case orchestrator.TypeGameServer:
		plan := orchestrator.String(cfg, "plan_name")
		if plan != "" && plan != "custom" {
			for _, p := range AvailableGameServerPlans {
				if p.ID == plan {
					cfg["cpu_cores"], cfg["memory_mb"], cfg["storage_gb"] = p.VCPUs, p.MemoryMB, p.StorageGB
					if orchestrator.Int(cfg, "player_slots", 0) <= 0 {
						cfg["player_slots"] = p.MaxPlayers
					}
				}
			}
		}
		if orchestrator.Int(cfg, "cpu_cores", 0) <= 0 {
			cfg["cpu_cores"] = 2
		}
		if orchestrator.Int(cfg, "memory_mb", 0) <= 0 {
			cfg["memory_mb"] = 4096
		}
		if orchestrator.Int(cfg, "storage_gb", 0) <= 0 {
			cfg["storage_gb"] = 20
		}
	case orchestrator.TypeAndroid:
		plan := orchestrator.String(cfg, "plan_name")
		if plan != "" && plan != "custom" && e.s.svc.Android != nil {
			for _, p := range e.s.svc.Android.ListPlans() {
				if p.ID == plan {
					cfg["vcpus"], cfg["memory_mb"], cfg["storage_gb"] = p.VCPUs, p.MemoryMB, p.StorageGB
				}
			}
		}
		if id, ok := orchestrator.UUID(cfg, "image_id"); ok && e.s.repos.Android != nil {
			if img, err := e.s.repos.Android.FindPublishedImage(ctx, id); err == nil {
				if orchestrator.Int(cfg, "vcpus", 0) < img.MinCPU {
					cfg["vcpus"] = img.MinCPU
				}
				if orchestrator.Int(cfg, "memory_mb", 0) < img.MinRAMMB {
					cfg["memory_mb"] = img.MinRAMMB
				}
				if orchestrator.Int(cfg, "storage_gb", 0) < img.MinStorageGB {
					cfg["storage_gb"] = img.MinStorageGB
				}
			}
		}
	}
	return nil
}

func (e *policyEnv) TemplateEnabled(ctx context.Context, id uuid.UUID) (bool, error) {
	_, err := e.s.repos.Templates.FindEnabledByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (e *policyEnv) PlanEnabled(ctx context.Context, id uuid.UUID) (bool, error) {
	_, err := e.s.repos.Plans.FindEnabledByID(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (e *policyEnv) SSHKeyOwned(ctx context.Context, userID, id uuid.UUID) (bool, error) {
	_, err := e.s.repos.SSHKeys.FindByIDAndOwner(ctx, id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (e *policyEnv) Instance(ctx context.Context, userID, id uuid.UUID) (*orchestrator.InstanceRef, error) {
	inst, err := e.s.repos.Instances.FindByID(ctx, id, &userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &orchestrator.InstanceRef{ID: inst.ID, Name: inst.Name, Status: inst.Status, VCPUs: inst.VCPUs, MemoryMB: inst.MemoryMB, DiskGB: inst.DiskGB}, nil
}

func (e *policyEnv) SnapshotExists(ctx context.Context, instanceID uuid.UUID, name string) (bool, error) {
	n, err := e.s.repos.Snapshots.CountByName(ctx, instanceID, name)
	return n > 0, err
}

func (e *policyEnv) Bucket(ctx context.Context, userID, id uuid.UUID) (*orchestrator.BucketRef, error) {
	b, err := e.s.repos.Storage.FindBucket(ctx, id, userID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	n, _ := e.s.repos.Storage.CountObjects(ctx, b.ID)
	return &orchestrator.BucketRef{ID: b.ID, Name: b.Name, Objects: n}, nil
}

func (e *policyEnv) BucketNameTaken(ctx context.Context, name string) (bool, error) {
	_, err := e.s.repos.Storage.FindBucketByName(ctx, name)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (e *policyEnv) AndroidImagePublished(ctx context.Context, id uuid.UUID) (bool, error) {
	_, err := e.s.repos.Android.FindPublishedImage(ctx, id)
	if errors.Is(err, repository.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (e *policyEnv) Quota(ctx context.Context, userID uuid.UUID) (*orchestrator.QuotaSnapshot, error) {
	u, err := e.s.svc.Quota.GetUsage(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &orchestrator.QuotaSnapshot{
		MaxVCPUs: u.MaxVCPUs, UsedVCPUs: u.UsedVCPUs,
		MaxMemoryMB: u.MaxMemoryMB, UsedMemoryMB: u.UsedMemoryMB,
		MaxStorageGB: u.MaxStorageGB, UsedStorageGB: u.UsedStorageGB,
		MaxInstances: u.MaxInstances, UsedInstances: u.UsedInstances,
	}, nil
}

// Capacity is the free capacity of the scheduling node with the platform's
// 4:1 vCPU overcommit (the same rule cloud.get_available_capacity reports).
func (e *policyEnv) Capacity(ctx context.Context) (*orchestrator.CapacitySnapshot, error) {
	node, err := e.s.repos.Nodes.FirstOnline(ctx)
	if errors.Is(err, repository.ErrNotFound) {
		return &orchestrator.CapacitySnapshot{}, nil
	}
	if err != nil {
		return nil, err
	}
	usedVCPUs, _ := e.s.repos.Instances.SumAllocatedVCPUs(ctx)
	usedMem, _ := e.s.repos.Instances.SumAllocatedMemory(ctx)
	return &orchestrator.CapacitySnapshot{
		NodeOnline:        true,
		NodeName:          node.Name,
		AvailableVCPUs:    int64(node.TotalCPUThreads*4) - usedVCPUs,
		AvailableMemoryMB: int64(node.TotalMemoryMB) - usedMem,
	}, nil
}
