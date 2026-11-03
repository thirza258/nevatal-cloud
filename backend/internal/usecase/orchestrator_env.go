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
	case orchestrator.TypeDatabase:
		if engine, ok := cfg["engine"].(string); ok {
			cfg["engine"] = strings.ToLower(engine)
		}
		if orchestrator.String(cfg, "engine") == "" {
			cfg["engine"] = orchestrator.DatabaseEnginePostgres
		}
		plan := orchestrator.String(cfg, "plan_name")
		if plan == "" {
			plan = "starter"
			cfg["plan_name"] = plan
		}
		if plan != "custom" {
			for _, p := range AvailableDatabasePlans {
				if p.ID == plan {
					cfg["vcpus"], cfg["memory_mb"], cfg["storage_gb"] = p.VCPUs, p.MemoryMB, p.StorageGB
				}
			}
		}
		if orchestrator.Int(cfg, "vcpus", 0) <= 0 {
			cfg["vcpus"] = 1
		}
		if orchestrator.Int(cfg, "memory_mb", 0) <= 0 {
			cfg["memory_mb"] = 1024
		}
		if orchestrator.Int(cfg, "storage_gb", 0) <= 0 {
			cfg["storage_gb"] = 10
		}
		if _, set := cfg["auto_backup"]; !set {
			cfg["auto_backup"] = true
		}
	case orchestrator.TypeDomain:
		if sub, ok := cfg["subdomain"].(string); ok {
			cfg["subdomain"] = strings.ToLower(sub)
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
	snap := &orchestrator.QuotaSnapshot{
		MaxVCPUs: u.MaxVCPUs, UsedVCPUs: u.UsedVCPUs,
		MaxMemoryMB: u.MaxMemoryMB, UsedMemoryMB: u.UsedMemoryMB,
		MaxStorageGB: u.MaxStorageGB, UsedStorageGB: u.UsedStorageGB,
		MaxInstances: u.MaxInstances, UsedInstances: u.UsedInstances,
	}
	// Managed databases consume the same compute quota as instances.
	if e.s.repos.Databases != nil {
		vcpus, mem, disk, count, err := e.s.repos.Databases.SumResources(ctx, userID)
		if err == nil {
			snap.UsedVCPUs += vcpus
			snap.UsedMemoryMB += mem
			snap.UsedStorageGB += disk
			snap.UsedInstances += int(count)
		}
	}
	return snap, nil
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

func notFoundOrDenied(err error) bool {
	var ue *Error
	if errors.As(err, &ue) {
		return ue.Kind == KindNotFound || ue.Kind == KindForbidden
	}
	return errors.Is(err, repository.ErrNotFound)
}

// ServiceResource looks up an app, n8n instance, OpenClaw agent, game server,
// Android instance or managed database (plus VPS, bucket and domain) owned by
// the user.
func (e *policyEnv) ServiceResource(ctx context.Context, userID uuid.UUID, resourceType string, id uuid.UUID) (*orchestrator.ServiceResourceRef, error) {
	ref := func(t, name, status string) *orchestrator.ServiceResourceRef {
		return &orchestrator.ServiceResourceRef{ID: id, Type: t, Name: name, Status: status}
	}
	switch resourceType {
	case orchestrator.TypeVPS:
		inst, err := e.s.repos.Instances.FindByID(ctx, id, &userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return ref(resourceType, inst.Name, inst.Status), nil
	case orchestrator.TypeApp:
		if e.s.svc.App == nil {
			return nil, nil
		}
		a, err := e.s.svc.App.Get(ctx, id, userID, false)
		if err != nil {
			if notFoundOrDenied(err) {
				return nil, nil
			}
			return nil, err
		}
		return ref(resourceType, a.Name, a.Status), nil
	case orchestrator.TypeN8n:
		a, err := e.s.svc.Automation.Get(ctx, id, userID, false)
		if err != nil {
			if notFoundOrDenied(err) {
				return nil, nil
			}
			return nil, err
		}
		return ref(resourceType, a.Name, a.Status), nil
	case orchestrator.TypeOpenClaw:
		a, err := e.s.svc.OpenClaw.Get(ctx, id, userID, false)
		if err != nil {
			if notFoundOrDenied(err) {
				return nil, nil
			}
			return nil, err
		}
		return ref(resourceType, a.Name, a.Status), nil
	case orchestrator.TypeGameServer:
		a, err := e.s.svc.GameServer.Get(ctx, id, userID, false)
		if err != nil {
			if notFoundOrDenied(err) {
				return nil, nil
			}
			return nil, err
		}
		return ref(resourceType, a.Name, a.Status), nil
	case orchestrator.TypeAndroid:
		a, err := e.s.svc.Android.Get(ctx, id, userID, false)
		if err != nil {
			if notFoundOrDenied(err) {
				return nil, nil
			}
			return nil, err
		}
		return ref(resourceType, a.Name, a.Status), nil
	case orchestrator.TypeDatabase:
		if e.s.svc.Database == nil {
			return nil, nil
		}
		a, err := e.s.svc.Database.Get(ctx, id, userID)
		if err != nil {
			if notFoundOrDenied(err) {
				return nil, nil
			}
			return nil, err
		}
		return ref(resourceType, a.Name, a.Status), nil
	case orchestrator.TypeBucket:
		b, err := e.s.repos.Storage.FindBucket(ctx, id, userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}
		return ref(resourceType, b.Name, "active"), nil
	case orchestrator.TypeDomain:
		d, err := e.s.repos.Domains.FindActiveByID(ctx, id, &userID)
		if err != nil {
			if errors.Is(err, repository.ErrNotFound) {
				return nil, nil
			}
			return nil, err
		}
		status := "live"
		if d.AdminDisabled || !d.Enabled {
			status = "disabled"
		}
		return ref(resourceType, d.Hostname, status), nil
	}
	return nil, nil
}

func (e *policyEnv) DatabaseNameTaken(ctx context.Context, userID uuid.UUID, name string) (bool, error) {
	if e.s.svc.Database == nil {
		return false, nil
	}
	items, err := e.s.svc.Database.List(ctx, userID)
	if err != nil {
		return false, err
	}
	for _, db := range items {
		if strings.EqualFold(db.Name, name) {
			return true, nil
		}
	}
	return false, nil
}

func (e *policyEnv) Template(ctx context.Context, userID, id uuid.UUID) (*orchestrator.TemplateRef, error) {
	if e.s.repos.InfraTemplates == nil {
		return nil, nil
	}
	t, err := e.s.repos.InfraTemplates.FindVisible(ctx, id, userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &orchestrator.TemplateRef{
		ID: t.ID, OwnerID: t.OwnerID, Name: t.Name, IsPublic: t.IsPublic,
		Valid: t.ValidationStatus == "valid", Specification: t.Specification,
	}, nil
}

func (e *policyEnv) Domain(ctx context.Context, userID, id uuid.UUID) (*orchestrator.DomainRef, error) {
	d, err := e.s.repos.Domains.FindActiveByID(ctx, id, &userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	ref := &orchestrator.DomainRef{ID: d.ID, Hostname: d.Hostname}
	if d.InstanceID != nil {
		ref.Instance = *d.InstanceID
	}
	return ref, nil
}

func (e *policyEnv) DomainsEnabled() bool {
	return e.s.svc.Domain != nil && e.s.svc.Domain.Enabled()
}

func (e *policyEnv) SubdomainAvailable(ctx context.Context, userID uuid.UUID, subdomain string) (bool, error) {
	if e.s.svc.Domain == nil {
		return false, nil
	}
	res, err := e.s.svc.Domain.Check(ctx, userID, subdomain)
	if err != nil {
		return false, err
	}
	return res.Available, nil
}

func (e *policyEnv) BucketObject(ctx context.Context, userID, bucketID, objectID uuid.UUID) (*orchestrator.BucketObjectRef, error) {
	obj, err := e.s.repos.Storage.FindObject(ctx, objectID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if obj.BucketID != bucketID {
		return nil, nil
	}
	if _, err := e.s.repos.Storage.FindBucket(ctx, bucketID, userID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &orchestrator.BucketObjectRef{ID: obj.ID, Key: obj.Key, IsDir: obj.IsFolder}, nil
}
