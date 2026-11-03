package usecase

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
)

// UnifiedResourceItem is one row of the unified resource inventory.
type UnifiedResourceItem struct {
	ID        string                 `json:"id"`
	Type      string                 `json:"type"` // "vps", "app", "automation", "ai_agent", "bucket"
	Name      string                 `json:"name"`
	Status    string                 `json:"status"`
	Endpoint  string                 `json:"endpoint,omitempty"`
	Specs     string                 `json:"specs,omitempty"`
	CreatedAt string                 `json:"created_at"`
	Details   map[string]interface{} `json:"details,omitempty"`
}

// ResourceCounts are the per-product totals of a ResourceSummary.
type ResourceCounts struct {
	VPS              int
	Apps             int
	Automations      int
	AIAgents         int
	Buckets          int
	GameServers      int
	AndroidInstances int
	Databases        int
}

// ResourceSummary is everything a customer runs on the platform plus quota
// usage. A product whose listing fails contributes no items.
type ResourceSummary struct {
	Items  []UnifiedResourceItem
	Counts ResourceCounts
	Usage  *models.QuotaUsage
}

// ResourceUsecase builds the unified resource inventory (PRD 34.1).
type ResourceUsecase struct {
	instanceService   *InstanceUsecase
	appService        *AppUsecase
	automationService *AutomationUsecase
	openClawService   *OpenClawUsecase
	storageService    *StorageUsecase
	gameServerService *GameServerUsecase
	androidService    *AndroidUsecase
	databaseService   *DatabaseUsecase
	quotaService      *QuotaUsecase
}

func NewResourceUsecase(
	instanceService *InstanceUsecase,
	appService *AppUsecase,
	automationService *AutomationUsecase,
	openClawService *OpenClawUsecase,
	storageService *StorageUsecase,
	gameServerService *GameServerUsecase,
	androidService *AndroidUsecase,
	databaseService *DatabaseUsecase,
	quotaService *QuotaUsecase,
) *ResourceUsecase {
	return &ResourceUsecase{
		instanceService:   instanceService,
		appService:        appService,
		automationService: automationService,
		openClawService:   openClawService,
		storageService:    storageService,
		gameServerService: gameServerService,
		androidService:    androidService,
		databaseService:   databaseService,
		quotaService:      quotaService,
	}
}

func (s *ResourceUsecase) ListAll(ctx context.Context, userID uuid.UUID) *ResourceSummary {
	var items []UnifiedResourceItem

	// 1. Instances
	instances, err := s.instanceService.List(ctx, userID, false, "")
	if err == nil {
		for _, inst := range instances {
			ip := "allocating"
			if inst.PublicIP != nil && *inst.PublicIP != "" {
				ip = *inst.PublicIP
			} else if inst.PrivateIP != nil && *inst.PrivateIP != "" {
				ip = *inst.PrivateIP
			}
			items = append(items, UnifiedResourceItem{
				ID:        inst.ID.String(),
				Type:      "vps",
				Name:      inst.Name,
				Status:    inst.Status,
				Endpoint:  ip,
				Specs:     inst.OSTemplate.Name,
				CreatedAt: inst.CreatedAt.Format("2006-01-02 15:04"),
				Details: map[string]interface{}{
					"vcpus":     inst.VCPUs,
					"memory_mb": inst.MemoryMB,
					"disk_gb":   inst.DiskGB,
					"vmid":      inst.ProxmoxVMID,
				},
			})
		}
	}

	// 2. Apps
	apps, err := s.appService.List(ctx, userID, false)
	if err == nil {
		for _, app := range apps {
			items = append(items, UnifiedResourceItem{
				ID:        app.ID.String(),
				Type:      "app",
				Name:      app.Name,
				Status:    app.Status,
				Endpoint:  "https://" + app.AssignedSubdomain,
				Specs:     app.ApplicationType,
				CreatedAt: app.CreatedAt.Format("2006-01-02 15:04"),
				Details: map[string]interface{}{
					"repository": app.RepositoryURL,
					"branch":     app.ProductionBranch,
				},
			})
		}
	}

	// 3. Automation (n8n)
	autos, err := s.automationService.List(ctx, userID, false)
	if err == nil {
		for _, auto := range autos {
			items = append(items, UnifiedResourceItem{
				ID:        auto.ID.String(),
				Type:      "automation",
				Name:      auto.Name,
				Status:    auto.Status,
				Endpoint:  auto.DashboardURL,
				Specs:     "n8n Workflow Automation",
				CreatedAt: auto.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
	}

	// 4. OpenClaw AI Agents
	agents, err := s.openClawService.List(ctx, userID, false)
	if err == nil {
		for _, agent := range agents {
			items = append(items, UnifiedResourceItem{
				ID:        agent.ID.String(),
				Type:      "ai_agent",
				Name:      agent.Name,
				Status:    agent.Status,
				Specs:     agent.AIProvider + " (" + agent.DefaultModel + ")",
				CreatedAt: agent.CreatedAt.Format("2006-01-02 15:04"),
			})
		}
	}

	// 5. Storage Buckets
	buckets, err := s.storageService.ListBuckets(ctx, userID)
	if err == nil {
		for _, b := range buckets {
			items = append(items, UnifiedResourceItem{
				ID:        b.ID.String(),
				Type:      "bucket",
				Name:      b.Name,
				Status:    "active",
				Endpoint:  "s3://" + b.Name,
				Specs:     b.Visibility,
				CreatedAt: b.CreatedAt.Format("2006-01-02 15:04"),
				Details: map[string]interface{}{
					"objects": b.TotalObjects,
					"bytes":   b.TotalSizeBytes,
				},
			})
		}
	}

	// 6. Game Servers per PRD Section 49 & 65
	gameServers, err := s.gameServerService.List(ctx, userID, false, "")
	if err == nil {
		for _, gs := range gameServers {
			items = append(items, UnifiedResourceItem{
				ID:        gs.ID.String(),
				Type:      "game_server",
				Name:      gs.Name,
				Status:    gs.Status,
				Endpoint:  gs.PublicAddress,
				Specs:     fmt.Sprintf("%s (%s v%s)", strings.ReplaceAll(gs.GameType, "_", " "), gs.ServerSoftware, gs.GameVersion),
				CreatedAt: gs.CreatedAt.Format("2006-01-02 15:04"),
				Details: map[string]interface{}{
					"game_type":       gs.GameType,
					"server_software": gs.ServerSoftware,
					"port":            gs.AssignedPort,
					"players":         fmt.Sprintf("%d/%d", gs.OnlinePlayerCount, gs.PlayerSlots),
					"tps":             gs.TPS,
				},
			})
		}
	}

	// 7. Android Cloud (Waydroid) per PRD Section 66 & 83
	androidList, err := s.androidService.List(ctx, userID, false, "")
	if err == nil {
		for _, and := range androidList {
			items = append(items, UnifiedResourceItem{
				ID:        and.ID.String(),
				Type:      "android_instance",
				Name:      and.Name,
				Status:    and.Status,
				Endpoint:  and.AccessURL,
				Specs:     fmt.Sprintf("Android %s (%s, %s)", and.PlanName, and.DisplayResolution, and.Orientation),
				CreatedAt: and.CreatedAt.Format("2006-01-02 15:04"),
				Details: map[string]interface{}{
					"plan":        and.PlanName,
					"vcpus":       and.VCPUs,
					"memory_mb":   and.MemoryMB,
					"resolution":  and.DisplayResolution,
					"adb_enabled": and.ADBEnabled,
				},
			})
		}
	}

	// 8. Managed databases (PostgreSQL, MySQL, Redis, MongoDB)
	var databases []models.ManagedDatabase
	if s.databaseService != nil {
		databases, _ = s.databaseService.List(ctx, userID)
	}
	for _, db := range databases {
		items = append(items, UnifiedResourceItem{
			ID:        db.ID.String(),
			Type:      "database",
			Name:      db.Name,
			Status:    db.Status,
			Endpoint:  fmt.Sprintf("%s:%d", db.Host, db.Port),
			Specs:     fmt.Sprintf("%s %s (%s)", db.Engine, db.EngineVersion, db.PlanName),
			CreatedAt: db.CreatedAt.Format("2006-01-02 15:04"),
			Details: map[string]interface{}{
				"engine":       db.Engine,
				"vcpus":        db.VCPUs,
				"memory_mb":    db.MemoryMB,
				"storage_gb":   db.StorageGB,
				"database":     db.DatabaseName,
				"auto_backup":  db.AutoBackup,
				"last_backup":  db.LastBackupAt,
				"health_check": db.LastHealthCheckAt,
			},
		})
	}

	usage, _ := s.quotaService.GetUsage(ctx, userID)

	return &ResourceSummary{
		Items: items,
		Counts: ResourceCounts{
			VPS:              len(instances),
			Apps:             len(apps),
			Automations:      len(autos),
			AIAgents:         len(agents),
			Buckets:          len(buckets),
			GameServers:      len(gameServers),
			AndroidInstances: len(androidList),
			Databases:        len(databases),
		},
		Usage: usage,
	}
}
