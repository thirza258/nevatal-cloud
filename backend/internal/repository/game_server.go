package repository

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/models"
	"gorm.io/gorm"
)

// GameServerRepository persists game servers and their children: catalog
// templates and versions, configuration entries, players, plugins, mods,
// worlds and backups. Deleted servers are excluded.
type GameServerRepository interface {
	// Catalog
	// ListTemplates returns enabled templates, optionally for one game.
	ListTemplates(ctx context.Context, gameType string) ([]models.GameServerTemplate, error)
	// ListVersions returns enabled versions (latest stable first), optionally
	// filtered by game and server software.
	ListVersions(ctx context.Context, gameType, software string) ([]models.GameServerVersion, error)

	// Servers
	// List returns live servers (newest first) with ports, players and the
	// active world preloaded, optionally filtered by owner and game.
	List(ctx context.Context, ownerID *uuid.UUID, gameType string) ([]models.GameServer, error)
	// FindByID returns a live server with every association preloaded (the
	// ten newest backups only). With ownerID set, other owners' servers are
	// not found.
	FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.GameServer, error)
	// FindShallow is FindByID without associations.
	FindShallow(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.GameServer, error)
	Create(ctx context.Context, server *models.GameServer) error
	// Updates writes the given columns of server.
	Updates(ctx context.Context, server *models.GameServer, values map[string]interface{}) error
	// SetLastBackup sets last_backup_at on server.
	SetLastBackup(ctx context.Context, server *models.GameServer, at *time.Time) error

	// Configuration
	FindConfiguration(ctx context.Context, serverID uuid.UUID, key string) (*models.GameServerConfiguration, error)
	CreateConfiguration(ctx context.Context, cfg *models.GameServerConfiguration) error
	// UpdateConfiguration writes the given columns of cfg.
	UpdateConfiguration(ctx context.Context, cfg *models.GameServerConfiguration, values map[string]interface{}) error

	// Players
	// ListPlayers returns a server's players, online first, then by name.
	ListPlayers(ctx context.Context, serverID uuid.UUID) ([]models.GameServerPlayer, error)
	FindPlayer(ctx context.Context, serverID uuid.UUID, name string) (*models.GameServerPlayer, error)
	CreatePlayer(ctx context.Context, player *models.GameServerPlayer) error
	// UpdatePlayer writes the given columns of player.
	UpdatePlayer(ctx context.Context, player *models.GameServerPlayer, values map[string]interface{}) error
	// SetPlayerFlag sets one column of player.
	SetPlayerFlag(ctx context.Context, player *models.GameServerPlayer, column string, value bool) error
	// UpdatePlayersByName writes the given columns of the named player rows.
	UpdatePlayersByName(ctx context.Context, serverID uuid.UUID, name string, values map[string]interface{}) error

	// Plugins and mods
	ListPlugins(ctx context.Context, serverID uuid.UUID) ([]models.GameServerPlugin, error)
	CreatePlugin(ctx context.Context, plugin *models.GameServerPlugin) error
	SetPluginEnabled(ctx context.Context, serverID, pluginID uuid.UUID, enabled bool) error
	DeletePlugin(ctx context.Context, serverID, pluginID uuid.UUID) error
	ListMods(ctx context.Context, serverID uuid.UUID) ([]models.GameServerMod, error)
	CreateMod(ctx context.Context, mod *models.GameServerMod) error
	SetModEnabled(ctx context.Context, serverID, modID uuid.UUID, enabled bool) error
	DeleteMod(ctx context.Context, serverID, modID uuid.UUID) error

	// Worlds
	// ListWorlds returns a server's worlds, active first, then newest first.
	ListWorlds(ctx context.Context, serverID uuid.UUID) ([]models.GameServerWorld, error)
	CreateWorld(ctx context.Context, world *models.GameServerWorld) error
	// DeactivateWorlds marks every world of the server inactive.
	DeactivateWorlds(ctx context.Context, serverID uuid.UUID) error
	// UpdateWorld writes the given columns of the server's world.
	UpdateWorld(ctx context.Context, serverID, worldID uuid.UUID, values map[string]interface{}) error
	DeleteWorld(ctx context.Context, serverID, worldID uuid.UUID) error

	// Backups
	// ListBackups returns a server's backups, newest first.
	ListBackups(ctx context.Context, serverID uuid.UUID) ([]models.GameServerBackup, error)
	FindBackup(ctx context.Context, serverID, backupID uuid.UUID) (*models.GameServerBackup, error)
	CreateBackup(ctx context.Context, backup *models.GameServerBackup) error
	DeleteBackup(ctx context.Context, serverID, backupID uuid.UUID) error
}

type gameServerRepository struct{ base }

var _ GameServerRepository = (*gameServerRepository)(nil)

func NewGameServerRepository(db *gorm.DB) GameServerRepository {
	return &gameServerRepository{base{db}}
}

func (r *gameServerRepository) ListTemplates(ctx context.Context, gameType string) ([]models.GameServerTemplate, error) {
	var list []models.GameServerTemplate
	query := r.conn(ctx).Where("enabled = ?", true)
	if gameType != "" {
		query = query.Where("game_type = ?", gameType)
	}
	err := query.Order("game_type asc, name asc").Find(&list).Error
	return list, err
}

func (r *gameServerRepository) ListVersions(ctx context.Context, gameType, software string) ([]models.GameServerVersion, error) {
	var list []models.GameServerVersion
	query := r.conn(ctx).Where("enabled = ?", true)
	if gameType != "" {
		query = query.Where("game_type = ?", gameType)
	}
	if software != "" {
		query = query.Where("server_software = ?", software)
	}
	err := query.Order("is_latest_stable desc, version_string desc").Find(&list).Error
	return list, err
}

func (r *gameServerRepository) List(ctx context.Context, ownerID *uuid.UUID, gameType string) ([]models.GameServer, error) {
	var list []models.GameServer
	query := r.conn(ctx).Preload("Ports").
		Preload("Players").
		Preload("Worlds", "is_active = ?", true).
		Where("deleted_at IS NULL")

	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}
	if gameType != "" {
		query = query.Where("game_type = ?", gameType)
	}

	err := query.Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *gameServerRepository) FindByID(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.GameServer, error) {
	var item models.GameServer
	query := r.conn(ctx).Preload("Ports").
		Preload("Configurations").
		Preload("Players").
		Preload("Plugins").
		Preload("Mods").
		Preload("Worlds").
		Preload("Backups", func(db *gorm.DB) *gorm.DB {
			return db.Order("created_at desc").Limit(10)
		}).
		Where("id = ? AND deleted_at IS NULL", id)

	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}

	if err := query.First(&item).Error; err != nil {
		return nil, mapErr(err)
	}
	return &item, nil
}

func (r *gameServerRepository) FindShallow(ctx context.Context, id uuid.UUID, ownerID *uuid.UUID) (*models.GameServer, error) {
	var item models.GameServer
	query := r.conn(ctx).Where("id = ? AND deleted_at IS NULL", id)
	if ownerID != nil {
		query = query.Where("owner_id = ?", *ownerID)
	}
	if err := query.First(&item).Error; err != nil {
		return nil, mapErr(err)
	}
	return &item, nil
}

func (r *gameServerRepository) Create(ctx context.Context, server *models.GameServer) error {
	return r.conn(ctx).Create(server).Error
}

func (r *gameServerRepository) Updates(ctx context.Context, server *models.GameServer, values map[string]interface{}) error {
	return r.conn(ctx).Model(server).Updates(values).Error
}

func (r *gameServerRepository) SetLastBackup(ctx context.Context, server *models.GameServer, at *time.Time) error {
	return r.conn(ctx).Model(server).Update("last_backup_at", at).Error
}

func (r *gameServerRepository) FindConfiguration(ctx context.Context, serverID uuid.UUID, key string) (*models.GameServerConfiguration, error) {
	var cfg models.GameServerConfiguration
	if err := r.conn(ctx).Where("server_id = ? AND key = ?", serverID, key).First(&cfg).Error; err != nil {
		return nil, mapErr(err)
	}
	return &cfg, nil
}

func (r *gameServerRepository) CreateConfiguration(ctx context.Context, cfg *models.GameServerConfiguration) error {
	return r.conn(ctx).Create(cfg).Error
}

func (r *gameServerRepository) UpdateConfiguration(ctx context.Context, cfg *models.GameServerConfiguration, values map[string]interface{}) error {
	return r.conn(ctx).Model(cfg).Updates(values).Error
}

func (r *gameServerRepository) ListPlayers(ctx context.Context, serverID uuid.UUID) ([]models.GameServerPlayer, error) {
	var list []models.GameServerPlayer
	err := r.conn(ctx).Where("server_id = ?", serverID).Order("is_online desc, player_name asc").Find(&list).Error
	return list, err
}

func (r *gameServerRepository) FindPlayer(ctx context.Context, serverID uuid.UUID, name string) (*models.GameServerPlayer, error) {
	var player models.GameServerPlayer
	if err := r.conn(ctx).Where("server_id = ? AND player_name = ?", serverID, name).First(&player).Error; err != nil {
		return nil, mapErr(err)
	}
	return &player, nil
}

func (r *gameServerRepository) CreatePlayer(ctx context.Context, player *models.GameServerPlayer) error {
	return r.conn(ctx).Create(player).Error
}

func (r *gameServerRepository) UpdatePlayer(ctx context.Context, player *models.GameServerPlayer, values map[string]interface{}) error {
	return r.conn(ctx).Model(player).Updates(values).Error
}

func (r *gameServerRepository) SetPlayerFlag(ctx context.Context, player *models.GameServerPlayer, column string, value bool) error {
	return r.conn(ctx).Model(player).Update(column, value).Error
}

func (r *gameServerRepository) UpdatePlayersByName(ctx context.Context, serverID uuid.UUID, name string, values map[string]interface{}) error {
	return r.conn(ctx).Model(&models.GameServerPlayer{}).
		Where("server_id = ? AND player_name = ?", serverID, name).
		Updates(values).Error
}

func (r *gameServerRepository) ListPlugins(ctx context.Context, serverID uuid.UUID) ([]models.GameServerPlugin, error) {
	var list []models.GameServerPlugin
	err := r.conn(ctx).Where("server_id = ?", serverID).Order("name asc").Find(&list).Error
	return list, err
}

func (r *gameServerRepository) CreatePlugin(ctx context.Context, plugin *models.GameServerPlugin) error {
	return r.conn(ctx).Create(plugin).Error
}

func (r *gameServerRepository) SetPluginEnabled(ctx context.Context, serverID, pluginID uuid.UUID, enabled bool) error {
	return r.conn(ctx).Model(&models.GameServerPlugin{}).
		Where("id = ? AND server_id = ?", pluginID, serverID).
		Update("is_enabled", enabled).Error
}

func (r *gameServerRepository) DeletePlugin(ctx context.Context, serverID, pluginID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND server_id = ?", pluginID, serverID).Delete(&models.GameServerPlugin{}).Error
}

func (r *gameServerRepository) ListMods(ctx context.Context, serverID uuid.UUID) ([]models.GameServerMod, error) {
	var list []models.GameServerMod
	err := r.conn(ctx).Where("server_id = ?", serverID).Order("name asc").Find(&list).Error
	return list, err
}

func (r *gameServerRepository) CreateMod(ctx context.Context, mod *models.GameServerMod) error {
	return r.conn(ctx).Create(mod).Error
}

func (r *gameServerRepository) SetModEnabled(ctx context.Context, serverID, modID uuid.UUID, enabled bool) error {
	return r.conn(ctx).Model(&models.GameServerMod{}).
		Where("id = ? AND server_id = ?", modID, serverID).
		Update("is_enabled", enabled).Error
}

func (r *gameServerRepository) DeleteMod(ctx context.Context, serverID, modID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND server_id = ?", modID, serverID).Delete(&models.GameServerMod{}).Error
}

func (r *gameServerRepository) ListWorlds(ctx context.Context, serverID uuid.UUID) ([]models.GameServerWorld, error) {
	var list []models.GameServerWorld
	err := r.conn(ctx).Where("server_id = ?", serverID).Order("is_active desc, created_at desc").Find(&list).Error
	return list, err
}

func (r *gameServerRepository) CreateWorld(ctx context.Context, world *models.GameServerWorld) error {
	return r.conn(ctx).Create(world).Error
}

func (r *gameServerRepository) DeactivateWorlds(ctx context.Context, serverID uuid.UUID) error {
	return r.conn(ctx).Model(&models.GameServerWorld{}).Where("server_id = ?", serverID).Update("is_active", false).Error
}

func (r *gameServerRepository) UpdateWorld(ctx context.Context, serverID, worldID uuid.UUID, values map[string]interface{}) error {
	return r.conn(ctx).Model(&models.GameServerWorld{}).
		Where("id = ? AND server_id = ?", worldID, serverID).
		Updates(values).Error
}

func (r *gameServerRepository) DeleteWorld(ctx context.Context, serverID, worldID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND server_id = ?", worldID, serverID).Delete(&models.GameServerWorld{}).Error
}

func (r *gameServerRepository) ListBackups(ctx context.Context, serverID uuid.UUID) ([]models.GameServerBackup, error) {
	var list []models.GameServerBackup
	err := r.conn(ctx).Where("server_id = ?", serverID).Order("created_at desc").Find(&list).Error
	return list, err
}

func (r *gameServerRepository) FindBackup(ctx context.Context, serverID, backupID uuid.UUID) (*models.GameServerBackup, error) {
	var backup models.GameServerBackup
	if err := r.conn(ctx).Where("id = ? AND server_id = ?", backupID, serverID).First(&backup).Error; err != nil {
		return nil, mapErr(err)
	}
	return &backup, nil
}

func (r *gameServerRepository) CreateBackup(ctx context.Context, backup *models.GameServerBackup) error {
	return r.conn(ctx).Create(backup).Error
}

func (r *gameServerRepository) DeleteBackup(ctx context.Context, serverID, backupID uuid.UUID) error {
	return r.conn(ctx).Where("id = ? AND server_id = ?", backupID, serverID).Delete(&models.GameServerBackup{}).Error
}
