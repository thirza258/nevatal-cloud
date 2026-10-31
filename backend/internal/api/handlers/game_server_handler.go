package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/services"
)

type GameServerHandler struct {
	svc *services.GameServerService
}

func NewGameServerHandler(svc *services.GameServerService) *GameServerHandler {
	return &GameServerHandler{svc: svc}
}

func (h *GameServerHandler) GetPlans(c *gin.Context) {
	plans := h.svc.GetPlans()
	c.JSON(http.StatusOK, gin.H{"data": plans})
}

func (h *GameServerHandler) ListTemplates(c *gin.Context) {
	gameType := c.Query("game_type")
	list, err := h.svc.ListTemplates(gameType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (h *GameServerHandler) ListVersions(c *gin.Context) {
	gameType := c.Query("game_type")
	software := c.Query("server_software")
	list, err := h.svc.ListVersions(gameType, software)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (h *GameServerHandler) List(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)
	gameType := c.Query("game_type")

	list, err := h.svc.List(userID, isAdmin, gameType)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": list})
}

func (h *GameServerHandler) Create(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	var input services.CreateGameServerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	server, err := h.svc.Create(userID, input, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": server})
}

func (h *GameServerHandler) Get(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	server, err := h.svc.Get(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": server})
}

func (h *GameServerHandler) Update(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var input services.UpdateGameServerInput
	if err := c.ShouldBindJSON(&input); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	server, err := h.svc.Update(id, userID, isAdmin, input)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": server})
}

func (h *GameServerHandler) Delete(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	if err := h.svc.Delete(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Game server deleted successfully"})
}

func (h *GameServerHandler) Start(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	if err := h.svc.Start(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Game server started"})
}

func (h *GameServerHandler) Stop(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	graceful := c.Query("graceful") != "false"
	if err := h.svc.Stop(id, userID, isAdmin, graceful, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Game server stopped"})
}

func (h *GameServerHandler) Restart(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	if err := h.svc.Restart(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Game server restarted"})
}

func (h *GameServerHandler) Reinstall(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	if err := h.svc.Reinstall(id, userID, isAdmin, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Game server reinstalled"})
}

func (h *GameServerHandler) GetMetrics(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	metrics, err := h.svc.GetMetrics(id, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": metrics})
}

func (h *GameServerHandler) GetLogs(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "50"))
	logs, err := h.svc.GetLogs(id, userID, isAdmin, lines)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": logs})
}

func (h *GameServerHandler) ExecuteCommand(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		Command string `json:"command" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	res, err := h.svc.ExecuteCommand(id, userID, isAdmin, body.Command, c.ClientIP())
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"response": res}})
}

func (h *GameServerHandler) GetConfiguration(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	config, err := h.svc.GetConfiguration(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": config})
}

func (h *GameServerHandler) UpdateConfiguration(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	if err := h.svc.UpdateConfiguration(id, userID, isAdmin, body, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Configuration updated"})
}

// Player Endpoints
func (h *GameServerHandler) ListPlayers(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	isAdmin := middleware.IsAdmin(c)

	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	players, err := h.svc.ListPlayers(id, userID, isAdmin)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": players})
}

func (h *GameServerHandler) KickPlayer(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		PlayerName string `json:"player_name" binding:"required"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	_ = h.svc.KickPlayer(id, userID, body.PlayerName, body.Reason)
	c.JSON(http.StatusOK, gin.H{"message": "Player kicked"})
}

func (h *GameServerHandler) BanPlayer(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		PlayerName string `json:"player_name" binding:"required"`
		Reason     string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	_ = h.svc.BanPlayer(id, userID, body.PlayerName, body.Reason)
	c.JSON(http.StatusOK, gin.H{"message": "Player banned"})
}

func (h *GameServerHandler) UnbanPlayer(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		PlayerName string `json:"player_name" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	_ = h.svc.UnbanPlayer(id, userID, body.PlayerName)
	c.JSON(http.StatusOK, gin.H{"message": "Player unbanned"})
}

func (h *GameServerHandler) SetOperator(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		PlayerName string `json:"player_name" binding:"required"`
		IsOperator bool   `json:"is_operator"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	_ = h.svc.SetOperator(id, userID, body.PlayerName, body.IsOperator)
	c.JSON(http.StatusOK, gin.H{"message": "Player operator status updated"})
}

func (h *GameServerHandler) SetWhitelist(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		PlayerName    string `json:"player_name" binding:"required"`
		IsWhitelisted bool   `json:"is_whitelisted"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	_ = h.svc.SetWhitelist(id, userID, body.PlayerName, body.IsWhitelisted)
	c.JSON(http.StatusOK, gin.H{"message": "Player whitelist status updated"})
}

// Plugin Endpoints
func (h *GameServerHandler) ListPlugins(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	plugins, err := h.svc.ListPlugins(id, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": plugins})
}

func (h *GameServerHandler) InstallPlugin(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		Name     string `json:"name" binding:"required"`
		FileName string `json:"file_name"`
		Version  string `json:"version"`
		Author   string `json:"author"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	plugin, err := h.svc.InstallPlugin(id, userID, body.Name, body.FileName, body.Version, body.Author)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": plugin})
}

func (h *GameServerHandler) TogglePlugin(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}
	pluginID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid plugin id"}})
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	_ = h.svc.TogglePlugin(id, pluginID, userID, body.Enabled)
	c.JSON(http.StatusOK, gin.H{"message": "Plugin status updated"})
}

func (h *GameServerHandler) RemovePlugin(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}
	pluginID, err := uuid.Parse(c.Param("pluginId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid plugin id"}})
		return
	}

	_ = h.svc.RemovePlugin(id, pluginID, userID)
	c.JSON(http.StatusOK, gin.H{"message": "Plugin removed"})
}

// Mod Endpoints
func (h *GameServerHandler) ListMods(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	mods, err := h.svc.ListMods(id, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": mods})
}

func (h *GameServerHandler) InstallMod(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		Name      string `json:"name" binding:"required"`
		FileName  string `json:"file_name"`
		Version   string `json:"version"`
		ModLoader string `json:"mod_loader"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	mod, err := h.svc.InstallMod(id, userID, body.Name, body.FileName, body.Version, body.ModLoader)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": mod})
}

func (h *GameServerHandler) ToggleMod(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}
	modID, err := uuid.Parse(c.Param("modId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid mod id"}})
		return
	}

	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	_ = h.svc.ToggleMod(id, modID, userID, body.Enabled)
	c.JSON(http.StatusOK, gin.H{"message": "Mod status updated"})
}

func (h *GameServerHandler) RemoveMod(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}
	modID, err := uuid.Parse(c.Param("modId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid mod id"}})
		return
	}

	_ = h.svc.RemoveMod(id, modID, userID)
	c.JSON(http.StatusOK, gin.H{"message": "Mod removed"})
}

// World Endpoints
func (h *GameServerHandler) ListWorlds(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	worlds, err := h.svc.ListWorlds(id, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": worlds})
}

func (h *GameServerHandler) CreateWorld(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		WorldName  string `json:"world_name"`
		Seed       string `json:"seed"`
		Difficulty string `json:"difficulty"`
		WorldSize  string `json:"world_size"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}

	world, err := h.svc.CreateWorld(id, userID, body.WorldName, body.Seed, body.Difficulty, body.WorldSize)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": world})
}

func (h *GameServerHandler) ResetWorld(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		WorldID string `json:"world_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	worldID, err := uuid.Parse(body.WorldID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid world id"}})
		return
	}

	_ = h.svc.ResetWorld(id, userID, worldID)
	c.JSON(http.StatusOK, gin.H{"message": "World reset initiated"})
}

func (h *GameServerHandler) DeleteWorld(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}
	worldID, err := uuid.Parse(c.Param("worldId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid world id"}})
		return
	}

	_ = h.svc.DeleteWorld(id, worldID, userID)
	c.JSON(http.StatusOK, gin.H{"message": "World deleted"})
}

// Backup Endpoints
func (h *GameServerHandler) ListBackups(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	backups, err := h.svc.ListBackups(id, userID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": backups})
}

func (h *GameServerHandler) CreateBackup(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}

	var body struct {
		BackupName string `json:"backup_name"`
	}
	_ = c.ShouldBindJSON(&body)

	backup, err := h.svc.CreateBackup(id, userID, body.BackupName, false)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": backup})
}

func (h *GameServerHandler) RestoreBackup(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}
	backupID, err := uuid.Parse(c.Param("backupId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid backup id"}})
		return
	}

	if err := h.svc.RestoreBackup(id, backupID, userID, c.ClientIP()); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": err.Error()}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Backup restored successfully"})
}

func (h *GameServerHandler) DeleteBackup(c *gin.Context) {
	userID := middleware.GetCurrentUserID(c)
	id, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid server id"}})
		return
	}
	backupID, err := uuid.Parse(c.Param("backupId"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"message": "invalid backup id"}})
		return
	}

	_ = h.svc.DeleteBackup(id, backupID, userID)
	c.JSON(http.StatusOK, gin.H{"message": "Backup deleted"})
}
