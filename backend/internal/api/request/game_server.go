package request

import "github.com/nevatal/cloud/internal/usecase"

// CreateGameServerInput is the body of POST /game-servers.
type CreateGameServerInput struct {
	Name           string                 `json:"name" binding:"required"`
	GameType       string                 `json:"game_type" binding:"required"` // minecraft_java, minecraft_bedrock, terraria
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

func (r CreateGameServerInput) ToUsecase() usecase.CreateGameServerInput {
	return usecase.CreateGameServerInput(r)
}

// UpdateGameServerInput is the body of PATCH /game-servers/:id.
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

func (r UpdateGameServerInput) ToUsecase() usecase.UpdateGameServerInput {
	return usecase.UpdateGameServerInput(r)
}

// GameServerCommandRequest is the body of POST /game-servers/:id/command.
type GameServerCommandRequest = struct {
	Command string `json:"command" binding:"required"`
}

// PlayerModerationRequest is the body of the kick and ban endpoints.
type PlayerModerationRequest = struct {
	PlayerName string `json:"player_name" binding:"required"`
	Reason     string `json:"reason"`
}

// PlayerNameRequest is the body of POST /game-servers/:id/players/unban.
type PlayerNameRequest = struct {
	PlayerName string `json:"player_name" binding:"required"`
}

// SetOperatorRequest is the body of POST /game-servers/:id/players/op.
type SetOperatorRequest = struct {
	PlayerName string `json:"player_name" binding:"required"`
	IsOperator bool   `json:"is_operator"`
}

// SetWhitelistRequest is the body of POST /game-servers/:id/players/whitelist.
type SetWhitelistRequest = struct {
	PlayerName    string `json:"player_name" binding:"required"`
	IsWhitelisted bool   `json:"is_whitelisted"`
}

// InstallPluginRequest is the body of POST /game-servers/:id/plugins.
type InstallPluginRequest = struct {
	Name     string `json:"name" binding:"required"`
	FileName string `json:"file_name"`
	Version  string `json:"version"`
	Author   string `json:"author"`
}

// InstallModRequest is the body of POST /game-servers/:id/mods.
type InstallModRequest = struct {
	Name      string `json:"name" binding:"required"`
	FileName  string `json:"file_name"`
	Version   string `json:"version"`
	ModLoader string `json:"mod_loader"`
}

// ToggleRequest is the body of the plugin and mod PATCH endpoints.
type ToggleRequest = struct {
	Enabled bool `json:"enabled"`
}

// CreateWorldRequest is the body of POST /game-servers/:id/worlds.
type CreateWorldRequest = struct {
	WorldName  string `json:"world_name"`
	Seed       string `json:"seed"`
	Difficulty string `json:"difficulty"`
	WorldSize  string `json:"world_size"`
}

// ResetWorldRequest is the body of POST /game-servers/:id/worlds/reset.
type ResetWorldRequest = struct {
	WorldID string `json:"world_id" binding:"required"`
}

// CreateGameServerBackupRequest is the optional body of
// POST /game-servers/:id/backups.
type CreateGameServerBackupRequest = struct {
	BackupName string `json:"backup_name"`
}
