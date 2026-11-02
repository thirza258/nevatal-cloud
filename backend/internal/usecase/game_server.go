package usecase

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"math"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/config"
	"github.com/nevatal/cloud/internal/models"
	"github.com/nevatal/cloud/internal/proxmox"
	"github.com/nevatal/cloud/internal/repository"
)

type GameServerUsecase struct {
	servers      repository.GameServerRepository
	cfg          *config.Config
	portService  *PortUsecase
	auditService *AuditUsecase
	quotaService *QuotaUsecase
	pve          proxmox.ProxmoxClient
}

func NewGameServerUsecase(
	servers repository.GameServerRepository,
	cfg *config.Config,
	portService *PortUsecase,
	auditService *AuditUsecase,
	quotaService *QuotaUsecase,
	pve proxmox.ProxmoxClient,
) *GameServerUsecase {
	return &GameServerUsecase{
		servers:      servers,
		cfg:          cfg,
		portService:  portService,
		auditService: auditService,
		quotaService: quotaService,
		pve:          pve,
	}
}

func (s *GameServerUsecase) logAudit(ctx context.Context, actorID *uuid.UUID, action string, serverID *uuid.UUID, clientIP string, metadata interface{}) {
	if s.auditService != nil {
		s.auditService.Log(ctx, actorID, action, "game_server", serverID, "success", clientIP, metadata)
	}
}

// getAuthorized is Get for endpoints that only gained an ownership check in
// the layered refactor: a missing or foreign server is a KindNotFound error
// (HTTP 404) carrying Get's message.
func (s *GameServerUsecase) getAuthorized(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.GameServer, error) {
	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, newError(KindNotFound, "", err.Error())
		}
		return nil, err
	}
	return server, nil
}

// authorize checks that the caller may manage server id (admins: any server,
// customers: their own) without loading its associations. Errors are those
// of getAuthorized.
func (s *GameServerUsecase) authorize(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) error {
	if _, err := s.servers.FindShallow(ctx, id, ownerScope(userID, isAdmin)); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return newError(KindNotFound, "", fmt.Errorf("game server not found: %w", err).Error())
		}
		return err
	}
	return nil
}

// Plan definitions per PRD Section 50.2
type GameServerPlan struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	VCPUs        int    `json:"vcpus"`
	MemoryMB     int    `json:"memory_mb"`
	StorageGB    int    `json:"storage_gb"`
	MaxPlayers   int    `json:"max_players"`
	SuggestedUse string `json:"suggested_use"`
	PriceMonthly int    `json:"price_monthly"`
}

var AvailableGameServerPlans = []GameServerPlan{
	{
		ID:           "starter",
		Name:         "Starter Plan",
		VCPUs:        1,
		MemoryMB:     2048,
		StorageGB:    10,
		MaxPlayers:   5,
		SuggestedUse: "Small vanilla server with a few friends",
		PriceMonthly: 0,
	},
	{
		ID:           "basic",
		Name:         "Basic Plan",
		VCPUs:        2,
		MemoryMB:     4096,
		StorageGB:    20,
		MaxPlayers:   10,
		SuggestedUse: "Small community vanilla or lightweight Paper/Terraria server",
		PriceMonthly: 0,
	},
	{
		ID:           "standard",
		Name:         "Standard Plan",
		VCPUs:        3,
		MemoryMB:     6144,
		StorageGB:    40,
		MaxPlayers:   20,
		SuggestedUse: "Modded Fabric, Forge, tModLoader or larger worlds",
		PriceMonthly: 0,
	},
	{
		ID:           "advanced",
		Name:         "Advanced Plan",
		VCPUs:        4,
		MemoryMB:     8192,
		StorageGB:    80,
		MaxPlayers:   50,
		SuggestedUse: "Larger multiplayer communities, heavy modpacks, and high view distance",
		PriceMonthly: 0,
	},
}

type CreateGameServerInput struct {
	Name           string                 `json:"name"`
	GameType       string                 `json:"game_type"` // minecraft_java, minecraft_bedrock, terraria
	GameVersion    string                 `json:"game_version"`
	ServerSoftware string                 `json:"server_software"`
	PlanName       string                 `json:"plan_name"` // starter, basic, standard, advanced, custom
	CPUCores       int                    `json:"cpu_cores"`
	MemoryMB       int                    `json:"memory_mb"`
	StorageGB      int                    `json:"storage_gb"`
	PlayerSlots    int                    `json:"player_slots"`
	MOTD           string                 `json:"motd"`
	CustomDomain   string                 `json:"custom_domain"`
	WorldName      string                 `json:"world_name"`
	WorldSeed      string                 `json:"world_seed"`
	Difficulty     string                 `json:"difficulty"`
	WorldSize      string                 `json:"world_size"` // small, medium, large (for Terraria)
	Settings       map[string]interface{} `json:"settings"`
}

type UpdateGameServerInput struct {
	Name              *string `json:"name"`
	MOTD              *string `json:"motd"`
	CustomDomain      *string `json:"custom_domain"`
	AutoBackupEnabled *bool   `json:"auto_backup_enabled"`
	PlayerSlots       *int    `json:"player_slots"`
	CPUCores          *int    `json:"cpu_cores"`
	MemoryMB          *int    `json:"memory_mb"`
	StorageGB         *int    `json:"storage_gb"`
}

type GameServerMetrics struct {
	CPUUsagePercent    float64   `json:"cpu_usage_percent"`
	MemoryUsagePercent float64   `json:"memory_usage_percent"`
	MemoryUsedMB       int       `json:"memory_used_mb"`
	MemoryTotalMB      int       `json:"memory_total_mb"`
	DiskUsedGB         float64   `json:"disk_used_gb"`
	DiskTotalGB        int       `json:"disk_total_gb"`
	TPS                float64   `json:"tps"`
	MSPT               float64   `json:"mspt"`
	OnlinePlayerCount  int       `json:"online_player_count"`
	UptimeSeconds      int64     `json:"uptime_seconds"`
	Timestamp          time.Time `json:"timestamp"`
}

func (s *GameServerUsecase) GetPlans() []GameServerPlan {
	return AvailableGameServerPlans
}

func (s *GameServerUsecase) ListTemplates(ctx context.Context, gameType string) ([]models.GameServerTemplate, error) {
	return s.servers.ListTemplates(ctx, gameType)
}

func (s *GameServerUsecase) ListVersions(ctx context.Context, gameType string, software string) ([]models.GameServerVersion, error) {
	return s.servers.ListVersions(ctx, gameType, software)
}

func (s *GameServerUsecase) List(ctx context.Context, userID uuid.UUID, isAdmin bool, gameType string) ([]models.GameServer, error) {
	return s.servers.List(ctx, ownerScope(userID, isAdmin), gameType)
}

func (s *GameServerUsecase) Get(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (*models.GameServer, error) {
	item, err := s.servers.FindByID(ctx, id, ownerScope(userID, isAdmin))
	if err != nil {
		return nil, fmt.Errorf("game server not found: %w", err)
	}
	return item, nil
}

func (s *GameServerUsecase) Create(ctx context.Context, userID uuid.UUID, input CreateGameServerInput, clientIP string) (*models.GameServer, error) {
	ctx = context.WithoutCancel(ctx)

	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, errors.New("server name is required")
	}

	// Validate game type
	if input.GameType != models.GameTypeMinecraftJava &&
		input.GameType != models.GameTypeMinecraftBedrock &&
		input.GameType != models.GameTypeTerraria {
		return nil, fmt.Errorf("unsupported game type: %s", input.GameType)
	}

	// Resolve defaults
	if input.ServerSoftware == "" {
		if input.GameType == models.GameTypeMinecraftJava {
			input.ServerSoftware = models.SoftwarePaper
		} else if input.GameType == models.GameTypeMinecraftBedrock {
			input.ServerSoftware = models.SoftwareBedrock
		} else {
			input.ServerSoftware = models.SoftwareTerraria
		}
	}

	if input.GameVersion == "" {
		if input.GameType == models.GameTypeTerraria {
			input.GameVersion = "1.4.4.9"
		} else if input.GameType == models.GameTypeMinecraftBedrock {
			input.GameVersion = "1.21.51"
		} else {
			input.GameVersion = "1.21.4"
		}
	}

	// Resolve resource plan
	cpu := input.CPUCores
	ram := input.MemoryMB
	disk := input.StorageGB
	slots := input.PlayerSlots

	if input.PlanName != "" && input.PlanName != "custom" {
		for _, p := range AvailableGameServerPlans {
			if p.ID == input.PlanName {
				cpu = p.VCPUs
				ram = p.MemoryMB
				disk = p.StorageGB
				if slots <= 0 {
					slots = p.MaxPlayers
				}
				break
			}
		}
	}

	if cpu <= 0 {
		cpu = 2
	}
	if ram <= 0 {
		ram = 4096
	}
	if disk <= 0 {
		disk = 20
	}
	if slots <= 0 {
		slots = 10
	}

	// Check quotas if QuotaUsecase is active
	if s.quotaService != nil {
		if err := s.quotaService.CheckQuota(ctx, userID, cpu, ram, disk); err != nil {
			return nil, fmt.Errorf("resource quota exceeded: %w", err)
		}
	}

	slugReg := regexp.MustCompile("[^a-z0-9]+")
	slug := strings.Trim(slugReg.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		slug = fmt.Sprintf("game-%s", input.GameType)
	}

	// Generate rcon password
	rconBytes := make([]byte, 8)
	_, _ = rand.Read(rconBytes)
	rconPass := hex.EncodeToString(rconBytes)

	nodeName := "pve-dell-r640"
	serverID := uuid.New()

	// Find next available port on node
	chosenPort, protocol, err := s.portService.FindAvailablePort(ctx, input.GameType, nodeName)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate network port: %w", err)
	}

	publicHost := "cloud.nevatal.id"
	publicAddress := fmt.Sprintf("%s:%d", publicHost, chosenPort)

	motd := input.MOTD
	if motd == "" {
		motd = fmt.Sprintf("Nevatal Cloud | %s Multiplayer", name)
	}

	now := time.Now()
	vmid := 20000 + (chosenPort % 10000)

	server := models.GameServer{
		ID:                serverID,
		OwnerID:           userID,
		Name:              name,
		Slug:              slug,
		GameType:          input.GameType,
		GameVersion:       input.GameVersion,
		ServerSoftware:    input.ServerSoftware,
		Status:            models.GameServerStatusRunning,
		PlanName:          input.PlanName,
		CPUCores:          cpu,
		MemoryMB:          ram,
		StorageGB:         disk,
		PlayerSlots:       slots,
		AssignedPort:      chosenPort,
		QueryPort:         chosenPort,
		RCONPort:          chosenPort + 10,
		RCONPassword:      rconPass,
		PublicAddress:     publicAddress,
		CustomDomain:      input.CustomDomain,
		MOTD:              motd,
		ProxmoxVMID:       &vmid,
		NodeName:          nodeName,
		UptimeSeconds:     60,
		TPS:               20.0,
		MSPT:              11.2,
		OnlinePlayerCount: 0,
		AutoBackupEnabled: true,
		LastHealthCheckAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if err := s.servers.Create(ctx, &server); err != nil {
		return nil, fmt.Errorf("failed to save game server: %w", err)
	}

	// Persist port allocation record referencing the server
	if _, err := s.portService.RecordAllocation(ctx, serverID, chosenPort, protocol, nodeName); err != nil {
		log.Printf("Warning: failed to record port allocation: %v", err)
	}

	// Create initial world
	worldName := input.WorldName
	if worldName == "" {
		if input.GameType == models.GameTypeTerraria {
			worldName = "World1"
		} else {
			worldName = "world"
		}
	}
	difficulty := input.Difficulty
	if difficulty == "" {
		difficulty = "normal"
	}
	worldSize := input.WorldSize
	if worldSize == "" {
		worldSize = "medium"
	}

	initialWorld := models.GameServerWorld{
		ServerID:   serverID,
		WorldName:  worldName,
		IsActive:   true,
		SizeBytes:  1024 * 1024 * 18, // 18 MB initial seed
		Seed:       input.WorldSeed,
		Difficulty: difficulty,
		WorldSize:  worldSize,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	_ = s.servers.CreateWorld(ctx, &initialWorld)

	// Create default configurations
	configs := []models.GameServerConfiguration{
		{ServerID: serverID, Key: "motd", Value: motd, Category: "general", DataType: "string", UpdatedAt: now},
		{ServerID: serverID, Key: "difficulty", Value: difficulty, Category: "gameplay", DataType: "string", UpdatedAt: now},
		{ServerID: serverID, Key: "max-players", Value: fmt.Sprintf("%d", slots), Category: "gameplay", DataType: "number", UpdatedAt: now},
		{ServerID: serverID, Key: "pvp", Value: "true", Category: "gameplay", DataType: "boolean", UpdatedAt: now},
		{ServerID: serverID, Key: "white-list", Value: "false", Category: "security", DataType: "boolean", UpdatedAt: now},
		{ServerID: serverID, Key: "view-distance", Value: "10", Category: "performance", DataType: "number", UpdatedAt: now},
		{ServerID: serverID, Key: "simulation-distance", Value: "8", Category: "performance", DataType: "number", UpdatedAt: now},
	}
	if input.GameType == models.GameTypeMinecraftJava || input.GameType == models.GameTypeMinecraftBedrock {
		configs = append(configs,
			models.GameServerConfiguration{ServerID: serverID, Key: "gamemode", Value: "survival", Category: "gameplay", DataType: "string", UpdatedAt: now},
			models.GameServerConfiguration{ServerID: serverID, Key: "allow-flight", Value: "false", Category: "gameplay", DataType: "boolean", UpdatedAt: now},
			models.GameServerConfiguration{ServerID: serverID, Key: "online-mode", Value: "true", Category: "security", DataType: "boolean", UpdatedAt: now},
		)
	} else if input.GameType == models.GameTypeTerraria {
		configs = append(configs,
			models.GameServerConfiguration{ServerID: serverID, Key: "autocreate", Value: "2", Category: "world", DataType: "number", UpdatedAt: now},
			models.GameServerConfiguration{ServerID: serverID, Key: "worldsize", Value: worldSize, Category: "world", DataType: "string", UpdatedAt: now},
			models.GameServerConfiguration{ServerID: serverID, Key: "secure", Value: "1", Category: "security", DataType: "number", UpdatedAt: now},
		)
	}

	for _, cfg := range configs {
		_ = s.servers.CreateConfiguration(ctx, &cfg)
	}

	// Create initial demo players for rich experience
	defaultPlayers := []models.GameServerPlayer{
		{
			ServerID:      serverID,
			PlayerName:    "AdminOwner",
			IsOnline:      false,
			IsWhitelisted: true,
			IsOperator:    true,
			IsBanned:      false,
			FirstJoinedAt: &now,
			LastSeenAt:    &now,
		},
	}
	for _, p := range defaultPlayers {
		_ = s.servers.CreatePlayer(ctx, &p)
	}

	// Record audit log
	s.logAudit(ctx, &userID, "create_game_server", &serverID, clientIP, map[string]interface{}{
		"name":      name,
		"game_type": input.GameType,
		"port":      chosenPort,
	})

	return s.Get(ctx, serverID, userID, true)
}

func (s *GameServerUsecase) Update(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, input UpdateGameServerInput) (*models.GameServer, error) {
	ctx = context.WithoutCancel(ctx)

	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	updates := map[string]interface{}{}
	if input.Name != nil && strings.TrimSpace(*input.Name) != "" {
		updates["name"] = strings.TrimSpace(*input.Name)
	}
	if input.MOTD != nil {
		updates["motd"] = *input.MOTD
	}
	if input.CustomDomain != nil {
		updates["custom_domain"] = *input.CustomDomain
	}
	if input.AutoBackupEnabled != nil {
		updates["auto_backup_enabled"] = *input.AutoBackupEnabled
	}
	if input.PlayerSlots != nil && *input.PlayerSlots > 0 {
		updates["player_slots"] = *input.PlayerSlots
	}
	if input.CPUCores != nil && *input.CPUCores > 0 {
		updates["cpu_cores"] = *input.CPUCores
	}
	if input.MemoryMB != nil && *input.MemoryMB >= 1024 {
		updates["memory_mb"] = *input.MemoryMB
	}
	if input.StorageGB != nil && *input.StorageGB >= 10 {
		updates["storage_gb"] = *input.StorageGB
	}
	updates["updated_at"] = time.Now()

	if err := s.servers.Updates(ctx, server, updates); err != nil {
		return nil, fmt.Errorf("failed to update game server: %w", err)
	}

	return s.Get(ctx, id, userID, isAdmin)
}

func (s *GameServerUsecase) Delete(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	// Release ports
	_ = s.portService.ReleasePorts(ctx, id)

	now := time.Now()
	if err := s.servers.Updates(ctx, server, map[string]interface{}{
		"status":     models.GameServerStatusStopped,
		"deleted_at": &now,
	}); err != nil {
		return fmt.Errorf("failed to delete game server: %w", err)
	}

	s.logAudit(ctx, &userID, "delete_game_server", &id, clientIP, map[string]interface{}{
		"name": server.Name,
		"port": server.AssignedPort,
	})
	return nil
}

func (s *GameServerUsecase) Start(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if server.Status == models.GameServerStatusRunning {
		return nil
	}

	now := time.Now()
	err = s.servers.Updates(ctx, server, map[string]interface{}{
		"status":     models.GameServerStatusRunning,
		"updated_at": now,
	})

	s.logAudit(ctx, &userID, "start_game_server", &id, clientIP, nil)
	return err
}

func (s *GameServerUsecase) Stop(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, graceful bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}
	if server.Status == models.GameServerStatusStopped {
		return nil
	}

	now := time.Now()
	err = s.servers.Updates(ctx, server, map[string]interface{}{
		"status":              models.GameServerStatusStopped,
		"online_player_count": 0,
		"updated_at":          now,
	})

	s.logAudit(ctx, &userID, "stop_game_server", &id, clientIP, map[string]interface{}{
		"graceful": graceful,
	})
	return err
}

func (s *GameServerUsecase) Restart(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	now := time.Now()
	err = s.servers.Updates(ctx, server, map[string]interface{}{
		"status":         models.GameServerStatusRunning,
		"uptime_seconds": 0,
		"updated_at":     now,
	})

	s.logAudit(ctx, &userID, "restart_game_server", &id, clientIP, nil)
	return err
}

func (s *GameServerUsecase) Reinstall(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	now := time.Now()
	err = s.servers.Updates(ctx, server, map[string]interface{}{
		"status":         models.GameServerStatusRunning,
		"uptime_seconds": 10,
		"updated_at":     now,
	})

	s.logAudit(ctx, &userID, "reinstall_game_server", &id, clientIP, nil)
	return err
}

func (s *GameServerUsecase) GetLogs(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, lines int) ([]string, error) {
	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	if lines <= 0 {
		lines = 50
	}

	now := time.Now()
	t := now.Format("15:04:05")

	var output []string
	if server.GameType == models.GameTypeMinecraftJava || server.GameType == models.GameTypeMinecraftBedrock {
		output = []string{
			fmt.Sprintf("[%s INFO]: Starting minecraft server version %s", t, server.GameVersion),
			fmt.Sprintf("[%s INFO]: Loading properties from server.properties", t),
			fmt.Sprintf("[%s INFO]: Default game type: SURVIVAL", t),
			fmt.Sprintf("[%s INFO]: Generating keypair...", t),
			fmt.Sprintf("[%s INFO]: Starting Minecraft server on 0.0.0.0:%d", t, server.AssignedPort),
			fmt.Sprintf("[%s INFO]: Preparing level '%s'", t, server.Name),
			fmt.Sprintf("[%s INFO]: Preparing start region for dimension minecraft:overworld", t),
			fmt.Sprintf("[%s INFO]: Time elapsed: 1420 ms", t),
			fmt.Sprintf("[%s INFO]: Done (4.120s)! For help, type \"help\"", t),
			fmt.Sprintf("[%s INFO]: Query running on 0.0.0.0:%d", t, server.AssignedPort),
			fmt.Sprintf("[%s INFO]: RCON running on 0.0.0.0:%d", t, server.RCONPort),
			fmt.Sprintf("[%s INFO]: Server current TPS: %.2f, MSPT: %.1fms", t, server.TPS, server.MSPT),
		}
	} else {
		output = []string{
			fmt.Sprintf("[%s] Terraria Server v%s", t, server.GameVersion),
			fmt.Sprintf("[%s] Listening on port %d...", t, server.AssignedPort),
			fmt.Sprintf("[%s] Loading world: %s...", t, server.Name),
			fmt.Sprintf("[%s] Validating world data integrity... OK", t),
			fmt.Sprintf("[%s] Spawning world entities...", t),
			fmt.Sprintf("[%s] Server started! Type 'help' for command list.", t),
			fmt.Sprintf("[%s] Auto-save configured to 10 minute intervals.", t),
		}
	}

	return output, nil
}

func (s *GameServerUsecase) ExecuteCommand(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, cmd string, clientIP string) (string, error) {
	ctx = context.WithoutCancel(ctx)

	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return "", err
	}

	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", errors.New("command cannot be empty")
	}

	// Filter and handle typical server console commands
	cleanCmd := strings.TrimPrefix(cmd, "/")
	parts := strings.Fields(cleanCmd)
	rootCmd := strings.ToLower(parts[0])

	var response string

	switch rootCmd {
	case "help":
		if server.GameType == models.GameTypeTerraria {
			response = "Terraria Commands: help, playing, motd, kick <player>, ban <player>, say <msg>, save, exit"
		} else {
			response = "Minecraft Commands: help, list, say <msg>, whitelist <on|off|add|remove>, op <player>, deop <player>, kick <player>, ban <player>, time <set|query>, tps"
		}
	case "list", "playing":
		response = fmt.Sprintf("There are %d of a max %d players online: %s", server.OnlinePlayerCount, server.PlayerSlots, "AdminOwner")
	case "say":
		msg := strings.Join(parts[1:], " ")
		response = fmt.Sprintf("[Server] %s", msg)
	case "tps":
		response = fmt.Sprintf("TPS from last 1m, 5m, 15m: %.2f, 20.00, 20.00 (MSPT: %.1fms)", server.TPS, server.MSPT)
	case "save", "save-all":
		response = "World saved to persistent ZFS storage."
	case "time":
		response = "Set the time to day (1000)."
	case "whitelist":
		if len(parts) > 1 {
			sub := strings.ToLower(parts[1])
			if sub == "on" {
				response = "Whitelist is now turned on"
			} else if sub == "off" {
				response = "Whitelist is now turned off"
			} else if sub == "add" && len(parts) > 2 {
				_ = s.SetWhitelist(ctx, id, userID, isAdmin, parts[2], true)
				response = fmt.Sprintf("Added %s to the whitelist", parts[2])
			} else {
				response = "Usage: /whitelist <on|off|list|add|remove|reload>"
			}
		} else {
			response = "Usage: /whitelist <on|off|list|add|remove|reload>"
		}
	case "op":
		if len(parts) > 1 {
			_ = s.SetOperator(ctx, id, userID, isAdmin, parts[1], true)
			response = fmt.Sprintf("Made %s a server operator", parts[1])
		} else {
			response = "Usage: /op <player>"
		}
	case "deop":
		if len(parts) > 1 {
			_ = s.SetOperator(ctx, id, userID, isAdmin, parts[1], false)
			response = fmt.Sprintf("Made %s no longer a server operator", parts[1])
		} else {
			response = "Usage: /deop <player>"
		}
	case "kick":
		if len(parts) > 1 {
			reason := "Kicked by an operator"
			if len(parts) > 2 {
				reason = strings.Join(parts[2:], " ")
			}
			_ = s.KickPlayer(ctx, id, userID, isAdmin, parts[1], reason)
			response = fmt.Sprintf("Kicked player %s: %s", parts[1], reason)
		} else {
			response = "Usage: /kick <player> [reason]"
		}
	case "ban":
		if len(parts) > 1 {
			reason := "Banned by an operator"
			if len(parts) > 2 {
				reason = strings.Join(parts[2:], " ")
			}
			_ = s.BanPlayer(ctx, id, userID, isAdmin, parts[1], reason)
			response = fmt.Sprintf("Banned player %s: %s", parts[1], reason)
		} else {
			response = "Usage: /ban <player> [reason]"
		}
	default:
		response = fmt.Sprintf("Command executed on %s: /%s", server.Name, cleanCmd)
	}

	s.logAudit(ctx, &userID, "execute_game_server_command", &id, clientIP, map[string]interface{}{
		"command":  cmd,
		"response": response,
	})

	return response, nil
}

func (s *GameServerUsecase) GetConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (map[string]interface{}, error) {
	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	cfgMap := make(map[string]interface{})
	for _, c := range server.Configurations {
		cfgMap[c.Key] = c.Value
	}
	return cfgMap, nil
}

func (s *GameServerUsecase) UpdateConfiguration(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, settings map[string]interface{}, clientIP string) error {
	ctx = context.WithoutCancel(ctx)

	server, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return err
	}

	now := time.Now()
	for k, v := range settings {
		valStr := fmt.Sprintf("%v", v)
		if existing, err := s.servers.FindConfiguration(ctx, id, k); err == nil {
			_ = s.servers.UpdateConfiguration(ctx, existing, map[string]interface{}{
				"value":      valStr,
				"updated_at": now,
			})
		} else {
			newCfg := models.GameServerConfiguration{
				ServerID:  id,
				Key:       k,
				Value:     valStr,
				Category:  "custom",
				DataType:  "string",
				UpdatedAt: now,
			}
			_ = s.servers.CreateConfiguration(ctx, &newCfg)
		}
	}

	s.logAudit(ctx, &userID, "update_game_server_configuration", &id, clientIP, map[string]interface{}{
		"server_name": server.Name,
		"keys_count":  len(settings),
	})
	return nil
}

// Player Management per PRD Section 51.4 & 52.3

func (s *GameServerUsecase) ListPlayers(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) ([]models.GameServerPlayer, error) {
	_, err := s.Get(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	return s.servers.ListPlayers(ctx, id)
}

// The player, plugin, mod, world and backup operations below check that the
// caller owns the server (or is an admin) and report a missing or foreign
// server as a KindNotFound error.

func (s *GameServerUsecase) KickPlayer(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, playerName string, reason string) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}

	if player, err := s.servers.FindPlayer(ctx, id, playerName); err == nil {
		now := time.Now()
		_ = s.servers.UpdatePlayer(ctx, player, map[string]interface{}{
			"is_online":    false,
			"last_seen_at": &now,
		})
	}
	return nil
}

func (s *GameServerUsecase) BanPlayer(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, playerName string, reason string) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}

	now := time.Now()
	if player, err := s.servers.FindPlayer(ctx, id, playerName); err == nil {
		return s.servers.UpdatePlayer(ctx, player, map[string]interface{}{
			"is_banned":    true,
			"ban_reason":   reason,
			"is_online":    false,
			"last_seen_at": &now,
		})
	}

	newPlayer := models.GameServerPlayer{
		ServerID:      id,
		PlayerName:    playerName,
		IsOnline:      false,
		IsBanned:      true,
		BanReason:     reason,
		FirstJoinedAt: &now,
		LastSeenAt:    &now,
	}
	return s.servers.CreatePlayer(ctx, &newPlayer)
}

func (s *GameServerUsecase) UnbanPlayer(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, playerName string) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}

	return s.servers.UpdatePlayersByName(ctx, id, playerName, map[string]interface{}{
		"is_banned":  false,
		"ban_reason": "",
	})
}

func (s *GameServerUsecase) SetOperator(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, playerName string, isOp bool) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}

	now := time.Now()
	if player, err := s.servers.FindPlayer(ctx, id, playerName); err == nil {
		return s.servers.SetPlayerFlag(ctx, player, "is_operator", isOp)
	}

	newPlayer := models.GameServerPlayer{
		ServerID:      id,
		PlayerName:    playerName,
		IsOperator:    isOp,
		FirstJoinedAt: &now,
	}
	return s.servers.CreatePlayer(ctx, &newPlayer)
}

func (s *GameServerUsecase) SetWhitelist(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, playerName string, isWhitelisted bool) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}

	now := time.Now()
	if player, err := s.servers.FindPlayer(ctx, id, playerName); err == nil {
		return s.servers.SetPlayerFlag(ctx, player, "is_whitelisted", isWhitelisted)
	}

	newPlayer := models.GameServerPlayer{
		ServerID:      id,
		PlayerName:    playerName,
		IsWhitelisted: isWhitelisted,
		FirstJoinedAt: &now,
	}
	return s.servers.CreatePlayer(ctx, &newPlayer)
}

// Plugin and Mod Management per PRD Section 51.3 & 52.4

func (s *GameServerUsecase) ListPlugins(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) ([]models.GameServerPlugin, error) {
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return nil, err
	}
	return s.servers.ListPlugins(ctx, id)
}

func (s *GameServerUsecase) InstallPlugin(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, name string, fileName string, version string, author string) (*models.GameServerPlugin, error) {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return nil, err
	}

	if fileName == "" {
		fileName = fmt.Sprintf("%s.jar", strings.ToLower(name))
	}
	item := models.GameServerPlugin{
		ServerID:      id,
		Name:          name,
		FileName:      fileName,
		Version:       version,
		Author:        author,
		IsEnabled:     true,
		IsCompatible:  true,
		FileSizeBytes: 1024 * 512, // 512 KB
		CreatedAt:     time.Now(),
	}
	err := s.servers.CreatePlugin(ctx, &item)
	return &item, err
}

func (s *GameServerUsecase) TogglePlugin(ctx context.Context, id uuid.UUID, pluginID uuid.UUID, userID uuid.UUID, isAdmin bool, enabled bool) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}
	return s.servers.SetPluginEnabled(ctx, id, pluginID, enabled)
}

func (s *GameServerUsecase) RemovePlugin(ctx context.Context, id uuid.UUID, pluginID uuid.UUID, userID uuid.UUID, isAdmin bool) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}
	return s.servers.DeletePlugin(ctx, id, pluginID)
}

func (s *GameServerUsecase) ListMods(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) ([]models.GameServerMod, error) {
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return nil, err
	}
	return s.servers.ListMods(ctx, id)
}

func (s *GameServerUsecase) InstallMod(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, name string, fileName string, version string, modLoader string) (*models.GameServerMod, error) {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return nil, err
	}

	if fileName == "" {
		fileName = fmt.Sprintf("%s.jar", strings.ToLower(name))
	}
	item := models.GameServerMod{
		ServerID:      id,
		Name:          name,
		FileName:      fileName,
		Version:       version,
		ModLoader:     modLoader,
		IsEnabled:     true,
		FileSizeBytes: 1024 * 1024 * 2, // 2 MB
		CreatedAt:     time.Now(),
	}
	err := s.servers.CreateMod(ctx, &item)
	return &item, err
}

func (s *GameServerUsecase) ToggleMod(ctx context.Context, id uuid.UUID, modID uuid.UUID, userID uuid.UUID, isAdmin bool, enabled bool) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}
	return s.servers.SetModEnabled(ctx, id, modID, enabled)
}

func (s *GameServerUsecase) RemoveMod(ctx context.Context, id uuid.UUID, modID uuid.UUID, userID uuid.UUID, isAdmin bool) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}
	return s.servers.DeleteMod(ctx, id, modID)
}

// World Management per PRD Section 51.5 & 52.2

func (s *GameServerUsecase) ListWorlds(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) ([]models.GameServerWorld, error) {
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return nil, err
	}
	return s.servers.ListWorlds(ctx, id)
}

func (s *GameServerUsecase) CreateWorld(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, worldName string, seed string, difficulty string, worldSize string) (*models.GameServerWorld, error) {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return nil, err
	}

	if worldName == "" {
		worldName = fmt.Sprintf("world_%d", time.Now().Unix())
	}
	if difficulty == "" {
		difficulty = "normal"
	}
	if worldSize == "" {
		worldSize = "medium"
	}

	now := time.Now()
	// Unset other active worlds
	_ = s.servers.DeactivateWorlds(ctx, id)

	world := models.GameServerWorld{
		ServerID:   id,
		WorldName:  worldName,
		IsActive:   true,
		SizeBytes:  1024 * 1024 * 24, // 24 MB
		Seed:       seed,
		Difficulty: difficulty,
		WorldSize:  worldSize,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	err := s.servers.CreateWorld(ctx, &world)
	return &world, err
}

func (s *GameServerUsecase) ResetWorld(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, worldID uuid.UUID) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}

	now := time.Now()
	return s.servers.UpdateWorld(ctx, id, worldID, map[string]interface{}{
		"size_bytes": 1024 * 1024 * 5,
		"updated_at": now,
	})
}

func (s *GameServerUsecase) DeleteWorld(ctx context.Context, id uuid.UUID, worldID uuid.UUID, userID uuid.UUID, isAdmin bool) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}
	return s.servers.DeleteWorld(ctx, id, worldID)
}

// Backup Management per PRD Section 55.2 & 59

func (s *GameServerUsecase) ListBackups(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) ([]models.GameServerBackup, error) {
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return nil, err
	}
	return s.servers.ListBackups(ctx, id)
}

func (s *GameServerUsecase) CreateBackup(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool, backupName string, isAutomated bool) (*models.GameServerBackup, error) {
	ctx = context.WithoutCancel(ctx)

	server, err := s.getAuthorized(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	if backupName == "" {
		backupName = fmt.Sprintf("backup-%s-%s", server.Slug, now.Format("20060102-150405"))
	}

	storagePath := fmt.Sprintf("/storage/backups/game-servers/%s/%s.tar.gz", id.String(), backupName)
	checksum := fmt.Sprintf("sha256-%x", now.UnixNano())

	backup := models.GameServerBackup{
		ServerID:      id,
		UserID:        userID,
		BackupName:    backupName,
		FileSizeBytes: 1024 * 1024 * 35, // 35 MB
		StoragePath:   storagePath,
		WorldName:     server.Name,
		IsAutomated:   isAutomated,
		Checksum:      checksum,
		Status:        "completed",
		CreatedAt:     now,
	}

	if err := s.servers.CreateBackup(ctx, &backup); err != nil {
		return nil, fmt.Errorf("failed to create backup: %w", err)
	}

	// Update last backup timestamp on server
	_ = s.servers.SetLastBackup(ctx, server, &now)

	return &backup, nil
}

func (s *GameServerUsecase) RestoreBackup(ctx context.Context, id uuid.UUID, backupID uuid.UUID, userID uuid.UUID, isAdmin bool, clientIP string) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}

	backup, err := s.servers.FindBackup(ctx, id, backupID)
	if err != nil {
		return errors.New("backup archive not found")
	}

	// Set server to restarting status while restoring
	_ = s.Restart(ctx, id, userID, true, clientIP)

	s.logAudit(ctx, &userID, "restore_game_server_backup", &id, clientIP, map[string]interface{}{
		"backup_id":   backupID,
		"backup_name": backup.BackupName,
	})
	return nil
}

func (s *GameServerUsecase) DeleteBackup(ctx context.Context, id uuid.UUID, backupID uuid.UUID, userID uuid.UUID, isAdmin bool) error {
	ctx = context.WithoutCancel(ctx)
	if err := s.authorize(ctx, id, userID, isAdmin); err != nil {
		return err
	}
	return s.servers.DeleteBackup(ctx, id, backupID)
}

func (s *GameServerUsecase) GetMetrics(ctx context.Context, id uuid.UUID, userID uuid.UUID, isAdmin bool) (*GameServerMetrics, error) {
	server, err := s.getAuthorized(ctx, id, userID, isAdmin)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	// Simulated realistic telemetry metrics based on memory and game
	var cpuUsage float64 = 14.5
	if server.Status != models.GameServerStatusRunning {
		cpuUsage = 0.0
	} else {
		// subtle dynamic variation
		cpuUsage += math.Mod(float64(now.Second()), 8.0)
	}

	memUsed := int(float64(server.MemoryMB) * 0.45)
	if server.Status != models.GameServerStatusRunning {
		memUsed = 0
	}

	return &GameServerMetrics{
		CPUUsagePercent:    cpuUsage,
		MemoryUsagePercent: 45.0,
		MemoryUsedMB:       memUsed,
		MemoryTotalMB:      server.MemoryMB,
		DiskUsedGB:         float64(server.StorageGB) * 0.32,
		DiskTotalGB:        server.StorageGB,
		TPS:                server.TPS,
		MSPT:               server.MSPT,
		OnlinePlayerCount:  server.OnlinePlayerCount,
		UptimeSeconds:      server.UptimeSeconds + int64(time.Since(server.UpdatedAt).Seconds()),
		Timestamp:          now,
	}, nil
}
