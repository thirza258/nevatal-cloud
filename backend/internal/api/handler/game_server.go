package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/nevatal/cloud/internal/api/middleware"
	"github.com/nevatal/cloud/internal/api/request"
	"github.com/nevatal/cloud/internal/api/response"
	"github.com/nevatal/cloud/internal/usecase"
)

// GameServerHandler serves /game-servers. Its errors use the code-less body
// {"error": {"message": ...}}. Sub-resource endpoints (players, plugins, mods,
// worlds, backups, metrics) answer 404 when the caller does not own the
// server.
type GameServerHandler struct {
	svc *usecase.GameServerUsecase
}

func NewGameServerHandler(svc *usecase.GameServerUsecase) *GameServerHandler {
	return &GameServerHandler{svc: svc}
}

// serverID parses :id or writes the 400 reply.
func serverID(c *gin.Context) (uuid.UUID, bool) {
	id, err := request.UUIDParam(c, "id")
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, "invalid server id")
		return uuid.Nil, false
	}
	return id, true
}

// childID parses a child path parameter or writes the 400 reply.
func childID(c *gin.Context, param, message string) (uuid.UUID, bool) {
	id, err := request.UUIDParam(c, param)
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, message)
		return uuid.Nil, false
	}
	return id, true
}

// bindMessage binds the JSON body or writes the 400 reply.
func bindMessage(c *gin.Context, dst any) bool {
	if err := request.BindJSON(c, dst); err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func (h *GameServerHandler) GetPlans(c *gin.Context) {
	response.OK(c, h.svc.GetPlans())
}

func (h *GameServerHandler) ListTemplates(c *gin.Context) {
	list, err := h.svc.ListTemplates(c.Request.Context(), c.Query("game_type"))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *GameServerHandler) ListVersions(c *gin.Context) {
	list, err := h.svc.ListVersions(c.Request.Context(), c.Query("game_type"), c.Query("server_software"))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *GameServerHandler) List(c *gin.Context) {
	list, err := h.svc.List(c.Request.Context(), middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.Query("game_type"))
	if err != nil {
		response.ErrorMessage(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.OK(c, list)
}

func (h *GameServerHandler) Create(c *gin.Context) {
	var req request.CreateGameServerInput
	if !bindMessage(c, &req) {
		return
	}

	server, err := h.svc.Create(c.Request.Context(), middleware.GetCurrentUserID(c), req.ToUsecase(), c.ClientIP())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Created(c, server)
}

func (h *GameServerHandler) Get(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	server, err := h.svc.Get(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusNotFound, err.Error())
		return
	}
	response.OK(c, server)
}

func (h *GameServerHandler) Update(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	var req request.UpdateGameServerInput
	if !bindMessage(c, &req) {
		return
	}

	server, err := h.svc.Update(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.ToUsecase())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, server)
}

// lifecycle serves the endpoints that change the server state and answer
// with a bare {"message": ...} body.
func (h *GameServerHandler) lifecycle(c *gin.Context, message string, action func(c *gin.Context, id uuid.UUID) error) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	if err := action(c, id); err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: message})
}

func (h *GameServerHandler) Delete(c *gin.Context) {
	h.lifecycle(c, "Game server deleted successfully", func(c *gin.Context, id uuid.UUID) error {
		return h.svc.Delete(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP())
	})
}

func (h *GameServerHandler) Start(c *gin.Context) {
	h.lifecycle(c, "Game server started", func(c *gin.Context, id uuid.UUID) error {
		return h.svc.Start(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP())
	})
}

func (h *GameServerHandler) Stop(c *gin.Context) {
	h.lifecycle(c, "Game server stopped", func(c *gin.Context, id uuid.UUID) error {
		graceful := c.Query("graceful") != "false"
		return h.svc.Stop(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), graceful, c.ClientIP())
	})
}

func (h *GameServerHandler) Restart(c *gin.Context) {
	h.lifecycle(c, "Game server restarted", func(c *gin.Context, id uuid.UUID) error {
		return h.svc.Restart(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP())
	})
}

func (h *GameServerHandler) Reinstall(c *gin.Context) {
	h.lifecycle(c, "Game server reinstalled", func(c *gin.Context, id uuid.UUID) error {
		return h.svc.Reinstall(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP())
	})
}

func (h *GameServerHandler) GetMetrics(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	metrics, err := h.svc.GetMetrics(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.OK(c, metrics)
}

func (h *GameServerHandler) GetLogs(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	lines, _ := strconv.Atoi(c.DefaultQuery("lines", "50"))
	logs, err := h.svc.GetLogs(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), lines)
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, logs)
}

func (h *GameServerHandler) ExecuteCommand(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	var req request.GameServerCommandRequest
	if !bindMessage(c, &req) {
		return
	}

	res, err := h.svc.ExecuteCommand(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Command, c.ClientIP())
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, response.CommandResult{Response: res})
}

func (h *GameServerHandler) GetConfiguration(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	config, err := h.svc.GetConfiguration(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, config)
}

func (h *GameServerHandler) UpdateConfiguration(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	var settings map[string]interface{}
	if !bindMessage(c, &settings) {
		return
	}

	if err := h.svc.UpdateConfiguration(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), settings, c.ClientIP()); err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Configuration updated"})
}

// Player Endpoints
func (h *GameServerHandler) ListPlayers(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	players, err := h.svc.ListPlayers(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, err.Error())
		return
	}
	response.OK(c, players)
}

// playerAction serves the player moderation endpoints: only an ownership
// failure is reported; other errors are ignored and the action acknowledged.
func (h *GameServerHandler) playerAction(c *gin.Context, body any, message string, action func(ctx *gin.Context, id, userID uuid.UUID, isAdmin bool) error) {
	id, ok := serverID(c)
	if !ok {
		return
	}
	if !bindMessage(c, body) {
		return
	}

	if writeIfDenied(c, action(c, id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))) {
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: message})
}

func (h *GameServerHandler) KickPlayer(c *gin.Context) {
	var req request.PlayerModerationRequest
	h.playerAction(c, &req, "Player kicked", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.KickPlayer(c.Request.Context(), id, userID, isAdmin, req.PlayerName, req.Reason)
	})
}

func (h *GameServerHandler) BanPlayer(c *gin.Context) {
	var req request.PlayerModerationRequest
	h.playerAction(c, &req, "Player banned", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.BanPlayer(c.Request.Context(), id, userID, isAdmin, req.PlayerName, req.Reason)
	})
}

func (h *GameServerHandler) UnbanPlayer(c *gin.Context) {
	var req request.PlayerNameRequest
	h.playerAction(c, &req, "Player unbanned", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.UnbanPlayer(c.Request.Context(), id, userID, isAdmin, req.PlayerName)
	})
}

func (h *GameServerHandler) SetOperator(c *gin.Context) {
	var req request.SetOperatorRequest
	h.playerAction(c, &req, "Player operator status updated", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.SetOperator(c.Request.Context(), id, userID, isAdmin, req.PlayerName, req.IsOperator)
	})
}

func (h *GameServerHandler) SetWhitelist(c *gin.Context) {
	var req request.SetWhitelistRequest
	h.playerAction(c, &req, "Player whitelist status updated", func(c *gin.Context, id, userID uuid.UUID, isAdmin bool) error {
		return h.svc.SetWhitelist(c.Request.Context(), id, userID, isAdmin, req.PlayerName, req.IsWhitelisted)
	})
}

// Plugin Endpoints
func (h *GameServerHandler) ListPlugins(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	plugins, err := h.svc.ListPlugins(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.OK(c, plugins)
}

func (h *GameServerHandler) InstallPlugin(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	var req request.InstallPluginRequest
	if !bindMessage(c, &req) {
		return
	}

	plugin, err := h.svc.InstallPlugin(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Name, req.FileName, req.Version, req.Author)
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.Created(c, plugin)
}

func (h *GameServerHandler) TogglePlugin(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}
	pluginID, ok := childID(c, "pluginId", "invalid plugin id")
	if !ok {
		return
	}

	var req request.ToggleRequest
	if !bindMessage(c, &req) {
		return
	}

	if writeIfDenied(c, h.svc.TogglePlugin(c.Request.Context(), id, pluginID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Enabled)) {
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Plugin status updated"})
}

func (h *GameServerHandler) RemovePlugin(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}
	pluginID, ok := childID(c, "pluginId", "invalid plugin id")
	if !ok {
		return
	}

	if writeIfDenied(c, h.svc.RemovePlugin(c.Request.Context(), id, pluginID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))) {
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Plugin removed"})
}

// Mod Endpoints
func (h *GameServerHandler) ListMods(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	mods, err := h.svc.ListMods(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.OK(c, mods)
}

func (h *GameServerHandler) InstallMod(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	var req request.InstallModRequest
	if !bindMessage(c, &req) {
		return
	}

	mod, err := h.svc.InstallMod(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Name, req.FileName, req.Version, req.ModLoader)
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.Created(c, mod)
}

func (h *GameServerHandler) ToggleMod(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}
	modID, ok := childID(c, "modId", "invalid mod id")
	if !ok {
		return
	}

	var req request.ToggleRequest
	if !bindMessage(c, &req) {
		return
	}

	if writeIfDenied(c, h.svc.ToggleMod(c.Request.Context(), id, modID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.Enabled)) {
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Mod status updated"})
}

func (h *GameServerHandler) RemoveMod(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}
	modID, ok := childID(c, "modId", "invalid mod id")
	if !ok {
		return
	}

	if writeIfDenied(c, h.svc.RemoveMod(c.Request.Context(), id, modID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))) {
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Mod removed"})
}

// World Endpoints
func (h *GameServerHandler) ListWorlds(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	worlds, err := h.svc.ListWorlds(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.OK(c, worlds)
}

func (h *GameServerHandler) CreateWorld(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	var req request.CreateWorldRequest
	if !bindMessage(c, &req) {
		return
	}

	world, err := h.svc.CreateWorld(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.WorldName, req.Seed, req.Difficulty, req.WorldSize)
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.Created(c, world)
}

func (h *GameServerHandler) ResetWorld(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	var req request.ResetWorldRequest
	if !bindMessage(c, &req) {
		return
	}
	worldID, err := uuid.Parse(req.WorldID)
	if err != nil {
		response.ErrorMessage(c, http.StatusBadRequest, "invalid world id")
		return
	}

	if writeIfDenied(c, h.svc.ResetWorld(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), worldID)) {
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "World reset initiated"})
}

func (h *GameServerHandler) DeleteWorld(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}
	worldID, ok := childID(c, "worldId", "invalid world id")
	if !ok {
		return
	}

	if writeIfDenied(c, h.svc.DeleteWorld(c.Request.Context(), id, worldID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))) {
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "World deleted"})
}

// Backup Endpoints
func (h *GameServerHandler) ListBackups(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	backups, err := h.svc.ListBackups(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.OK(c, backups)
}

func (h *GameServerHandler) CreateBackup(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}

	var req request.CreateGameServerBackupRequest
	_ = c.ShouldBindJSON(&req)

	backup, err := h.svc.CreateBackup(c.Request.Context(), id, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), req.BackupName, false)
	if err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.Created(c, backup)
}

func (h *GameServerHandler) RestoreBackup(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}
	backupID, ok := childID(c, "backupId", "invalid backup id")
	if !ok {
		return
	}

	if err := h.svc.RestoreBackup(c.Request.Context(), id, backupID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c), c.ClientIP()); err != nil {
		writeMessageError(c, err, http.StatusBadRequest)
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Backup restored successfully"})
}

func (h *GameServerHandler) DeleteBackup(c *gin.Context) {
	id, ok := serverID(c)
	if !ok {
		return
	}
	backupID, ok := childID(c, "backupId", "invalid backup id")
	if !ok {
		return
	}

	if writeIfDenied(c, h.svc.DeleteBackup(c.Request.Context(), id, backupID, middleware.GetCurrentUserID(c), middleware.IsAdmin(c))) {
		return
	}
	response.JSON(c, http.StatusOK, response.Message{Message: "Backup deleted"})
}
