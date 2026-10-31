import React, { useState, useEffect, useRef } from 'react';
import {
  Gamepad2,
  ArrowLeft,
  Play,
  Square,
  RotateCw,
  Terminal,
  Settings,
  Users,
  Box,
  Globe2,
  Database,
  Cpu,
  Trash2,
  RefreshCw,
  Copy,
  Check,
  AlertTriangle,
  Download,
  Upload,
  Shield,
  Clock,
  HardDrive,
  Activity,
  Sliders,
  Send,
  UserCheck,
  UserX,
  AlertCircle,
  ExternalLink,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import {
  GameServer,
  GameServerMetrics,
  GameServerPlayer,
  GameServerPlugin,
  GameServerMod,
  GameServerWorld,
  GameServerBackup,
} from '../../types';

interface GameServerDetailsProps {
  serverId: string;
  onNavigate: (path: string, param?: string) => void;
}

type TabType =
  | 'overview'
  | 'console'
  | 'configuration'
  | 'players'
  | 'plugins'
  | 'worlds'
  | 'backups'
  | 'networking'
  | 'resources'
  | 'settings';

export const GameServerDetails: React.FC<GameServerDetailsProps> = ({ serverId, onNavigate }) => {
  const [server, setServer] = useState<GameServer | null>(null);
  const [metrics, setMetrics] = useState<GameServerMetrics | null>(null);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<TabType>('overview');
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [errorNotice, setErrorNotice] = useState<string | null>(null);

  // Console state
  const [consoleLogs, setConsoleLogs] = useState<string[]>([]);
  const [commandInput, setCommandInput] = useState('');
  const [commandHistory, setCommandHistory] = useState<string[]>([]);
  const [historyIndex, setHistoryIndex] = useState(-1);
  const [autoScroll, setAutoScroll] = useState(true);
  const consoleBottomRef = useRef<HTMLDivElement>(null);

  // Configuration state
  const [configSettings, setConfigSettings] = useState<Record<string, any>>({});
  const [savingConfig, setSavingConfig] = useState(false);

  // Players state
  const [players, setPlayers] = useState<GameServerPlayer[]>([]);
  const [newPlayerName, setNewPlayerName] = useState('');

  // Plugins & Mods state
  const [plugins, setPlugins] = useState<GameServerPlugin[]>([]);
  const [mods, setMods] = useState<GameServerMod[]>([]);

  // Worlds state
  const [worlds, setWorlds] = useState<GameServerWorld[]>([]);
  const [showResetModal, setShowResetModal] = useState(false);

  // Backups state
  const [backups, setBackups] = useState<GameServerBackup[]>([]);
  const [creatingBackup, setCreatingBackup] = useState(false);
  const [restoringBackupId, setRestoringBackupId] = useState<string | null>(null);

  // Settings form state
  const [editName, setEditName] = useState('');
  const [editMotd, setEditMotd] = useState('');
  const [editMaxPlayers, setEditMaxPlayers] = useState(10);
  const [editCustomDomain, setEditCustomDomain] = useState('');
  const [editAutoBackup, setEditAutoBackup] = useState(true);
  const [editBackupSchedule, setEditBackupSchedule] = useState('daily');
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [showReinstallModal, setShowReinstallModal] = useState(false);

  const fetchServer = async () => {
    try {
      const res = await apiRequest<GameServer>(`/game-servers/${serverId}`);
      setServer(res);
      setEditName(res.name);
      setEditMotd(res.motd);
      setEditMaxPlayers(res.max_players);
      setEditCustomDomain(res.custom_domain || '');
      setEditAutoBackup(res.auto_backup_enabled);
      setEditBackupSchedule(res.backup_schedule);
    } catch (err: any) {
      setErrorNotice(err.message || 'Failed to fetch game server details');
    }
  };

  const fetchMetrics = async () => {
    try {
      const res = await apiRequest<GameServerMetrics>(`/game-servers/${serverId}/metrics`);
      setMetrics(res);
    } catch {
      // Ignore metrics failure
    }
  };

  const fetchLogs = async () => {
    try {
      const res = await apiRequest<{ logs: string[] }>(`/game-servers/${serverId}/logs?lines=100`);
      if (res && res.logs) {
        setConsoleLogs(res.logs);
      }
    } catch {
      // Ignore log fetch error
    }
  };

  const fetchConfig = async () => {
    try {
      const res = await apiRequest<Record<string, any>>(`/game-servers/${serverId}/configuration`);
      setConfigSettings(res || {});
    } catch {
      // Ignore
    }
  };

  const fetchPlayers = async () => {
    try {
      const res = await apiRequest<GameServerPlayer[]>(`/game-servers/${serverId}/players`);
      setPlayers(res || []);
    } catch {
      // Ignore
    }
  };

  const fetchPluginsAndMods = async () => {
    try {
      const [pls, mds] = await Promise.all([
        apiRequest<GameServerPlugin[]>(`/game-servers/${serverId}/plugins`).catch(() => []),
        apiRequest<GameServerMod[]>(`/game-servers/${serverId}/mods`).catch(() => []),
      ]);
      setPlugins(pls || []);
      setMods(mds || []);
    } catch {
      // Ignore
    }
  };

  const fetchWorlds = async () => {
    try {
      const res = await apiRequest<GameServerWorld[]>(`/game-servers/${serverId}/worlds`);
      setWorlds(res || []);
    } catch {
      // Ignore
    }
  };

  const fetchBackups = async () => {
    try {
      const res = await apiRequest<GameServerBackup[]>(`/game-servers/${serverId}/backups`);
      setBackups(res || []);
    } catch {
      // Ignore
    }
  };

  const initialLoad = async () => {
    setLoading(true);
    await Promise.all([
      fetchServer(),
      fetchMetrics(),
      fetchLogs(),
      fetchConfig(),
      fetchPlayers(),
      fetchPluginsAndMods(),
      fetchWorlds(),
      fetchBackups(),
    ]);
    setLoading(false);
  };

  useEffect(() => {
    initialLoad();
    const interval = setInterval(() => {
      fetchMetrics();
      if (activeTab === 'console') {
        fetchLogs();
      }
    }, 5000);
    return () => clearInterval(interval);
  }, [serverId, activeTab]);

  useEffect(() => {
    if (autoScroll && consoleBottomRef.current) {
      consoleBottomRef.current.scrollIntoView({ behavior: 'smooth' });
    }
  }, [consoleLogs, autoScroll]);

  const handleCopy = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handleAction = async (action: 'start' | 'stop' | 'restart' | 'reinstall' | 'delete') => {
    try {
      setActionLoading(action);
      if (action === 'delete') {
        await apiRequest(`/game-servers/${serverId}`, { method: 'DELETE' });
        onNavigate('/dashboard/game-servers');
        return;
      }
      await apiRequest(`/game-servers/${serverId}/${action}`, { method: 'POST' });
      await fetchServer();
      await fetchMetrics();
      if (action === 'reinstall') {
        setShowReinstallModal(false);
      }
    } catch (err: any) {
      alert(`Action failed: ${err.message || 'Unknown error'}`);
    } finally {
      setActionLoading(null);
    }
  };

  const handleSendCommand = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!commandInput.trim()) return;
    const cmd = commandInput.trim();
    setCommandHistory((prev) => [cmd, ...prev]);
    setHistoryIndex(-1);
    setCommandInput('');

    try {
      const res = await apiRequest<{ response: string }>(`/game-servers/${serverId}/command`, {
        method: 'POST',
        body: JSON.stringify({ command: cmd }),
      });
      setConsoleLogs((prev) => [
        ...prev,
        `> ${cmd}`,
        res?.response ? `[Server] ${res.response}` : `[Server] Command executed.`,
      ]);
    } catch (err: any) {
      setConsoleLogs((prev) => [...prev, `> ${cmd}`, `[Error] ${err.message || 'Command failed'}`]);
    }
  };

  const handleConsoleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (commandHistory.length > 0 && historyIndex < commandHistory.length - 1) {
        const nextIndex = historyIndex + 1;
        setHistoryIndex(nextIndex);
        setCommandInput(commandHistory[nextIndex]);
      }
    } else if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (historyIndex > 0) {
        const nextIndex = historyIndex - 1;
        setHistoryIndex(nextIndex);
        setCommandInput(commandHistory[nextIndex]);
      } else if (historyIndex === 0) {
        setHistoryIndex(-1);
        setCommandInput('');
      }
    }
  };

  const handleSaveConfig = async () => {
    try {
      setSavingConfig(true);
      await apiRequest(`/game-servers/${serverId}/configuration`, {
        method: 'PUT',
        body: JSON.stringify({ settings: configSettings }),
      });
      alert('Configuration updated! Restart the server to apply changes.');
    } catch (err: any) {
      alert(`Failed to save configuration: ${err.message}`);
    } finally {
      setSavingConfig(false);
    }
  };

  const handleAddPlayer = async (role: 'whitelist' | 'op') => {
    if (!newPlayerName.trim()) return;
    try {
      await apiRequest(`/game-servers/${serverId}/players`, {
        method: 'POST',
        body: JSON.stringify({
          player_name: newPlayerName.trim(),
          is_whitelisted: role === 'whitelist',
          is_operator: role === 'op',
        }),
      });
      setNewPlayerName('');
      await fetchPlayers();
    } catch (err: any) {
      alert(`Failed to add player: ${err.message}`);
    }
  };

  const handleKickPlayer = async (playerName: string) => {
    try {
      await apiRequest(`/game-servers/${serverId}/command`, {
        method: 'POST',
        body: JSON.stringify({ command: `kick ${playerName} Kicked by server administrator` }),
      });
      await fetchPlayers();
    } catch (err: any) {
      alert(`Kick failed: ${err.message}`);
    }
  };

  const handleBanPlayer = async (playerName: string) => {
    try {
      await apiRequest(`/game-servers/${serverId}/command`, {
        method: 'POST',
        body: JSON.stringify({ command: `ban ${playerName} Banned by administrator` }),
      });
      await fetchPlayers();
    } catch (err: any) {
      alert(`Ban failed: ${err.message}`);
    }
  };

  const handleTogglePlugin = async (pluginId: string, currentStatus: boolean) => {
    try {
      await apiRequest(`/game-servers/${serverId}/plugins/${pluginId}`, {
        method: 'PATCH',
        body: JSON.stringify({ enabled: !currentStatus }),
      });
      await fetchPluginsAndMods();
    } catch (err: any) {
      alert(`Failed to toggle plugin: ${err.message}`);
    }
  };

  const handleCreateBackup = async () => {
    try {
      setCreatingBackup(true);
      await apiRequest(`/game-servers/${serverId}/backups`, {
        method: 'POST',
        body: JSON.stringify({ backup_name: `manual-backup-${Date.now()}` }),
      });
      await fetchBackups();
    } catch (err: any) {
      alert(`Failed to create backup: ${err.message}`);
    } finally {
      setCreatingBackup(false);
    }
  };

  const handleRestoreBackup = async (backupId: string) => {
    try {
      setRestoringBackupId(backupId);
      await apiRequest(`/game-servers/${serverId}/backups/${backupId}/restore`, {
        method: 'POST',
      });
      alert('World restored successfully! The server is restarting.');
      await fetchServer();
      await fetchBackups();
    } catch (err: any) {
      alert(`Failed to restore backup: ${err.message}`);
    } finally {
      setRestoringBackupId(null);
    }
  };

  const handleResetWorld = async () => {
    try {
      await apiRequest(`/game-servers/${serverId}/worlds/reset`, {
        method: 'POST',
        body: JSON.stringify({ confirm: true }),
      });
      setShowResetModal(false);
      alert('World has been reset. A new world generation will complete on next server boot.');
      await fetchWorlds();
      await fetchServer();
    } catch (err: any) {
      alert(`Failed to reset world: ${err.message}`);
    }
  };

  const handleSaveSettings = async () => {
    try {
      await apiRequest(`/game-servers/${serverId}`, {
        method: 'PUT',
        body: JSON.stringify({
          name: editName,
          motd: editMotd,
          max_players: editMaxPlayers,
          custom_domain: editCustomDomain,
          auto_backup_enabled: editAutoBackup,
          backup_schedule: editBackupSchedule,
        }),
      });
      alert('Server settings saved successfully.');
      await fetchServer();
    } catch (err: any) {
      alert(`Failed to update settings: ${err.message}`);
    }
  };

  if (loading || !server) {
    return (
      <div className="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
        <RefreshCw className="w-8 h-8 animate-spin text-violet-500" />
        <span className="text-sm font-mono">Connecting to game server instance...</span>
      </div>
    );
  }

  const connAddr = server.custom_domain || server.connection_address;

  return (
    <div className="space-y-6">
      {/* Top Header Bar */}
      <div>
        <button
          onClick={() => onNavigate('/dashboard/game-servers')}
          className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-slate-200 transition mb-3"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>All Game Servers</span>
        </button>

        <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-4 bg-slate-900/60 p-6 rounded-2xl border border-slate-800 shadow-xl backdrop-blur">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-2xl bg-violet-600/10 text-violet-400 flex items-center justify-center border border-violet-500/20 shrink-0">
              <Gamepad2 className="w-6 h-6" />
            </div>
            <div>
              <div className="flex items-center gap-3 flex-wrap">
                <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">
                  {server.name}
                </h1>
                {server.status === 'running' ? (
                  <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
                    Online
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-slate-500/10 text-slate-400 border border-slate-500/20 capitalize">
                    {server.status}
                  </span>
                )}
              </div>
              <div className="flex items-center gap-3 text-xs text-slate-400 mt-1 font-mono flex-wrap">
                <span className="capitalize">{server.game_type.replace('_', ' ')}</span>
                <span>•</span>
                <span>{server.server_software} v{server.game_version}</span>
                <span>•</span>
                <span>Node: {server.proxmox_node}</span>
              </div>
            </div>
          </div>

          {/* Quick Connection Box */}
          <div className="flex items-center gap-2 bg-slate-950/80 px-4 py-2.5 rounded-xl border border-slate-800 self-start lg:self-center">
            <span className="text-[11px] font-mono uppercase text-slate-500 font-semibold">Address:</span>
            <span className="text-sm font-mono text-violet-300 font-semibold select-all">
              {connAddr}
            </span>
            <button
              onClick={() => handleCopy(connAddr)}
              className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs transition ml-1"
              title="Copy connection string"
            >
              {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
            </button>
          </div>
        </div>
      </div>

      {/* 10 Navigation Tabs per PRD Section 53 */}
      <div className="flex items-center gap-1 border-b border-slate-800 pb-1 overflow-x-auto text-xs font-medium">
        <button
          onClick={() => setActiveTab('overview')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap ${
            activeTab === 'overview'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          Overview
        </button>
        <button
          onClick={() => setActiveTab('console')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === 'console'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          <Terminal className="w-3.5 h-3.5" />
          <span>Live Console</span>
        </button>
        <button
          onClick={() => setActiveTab('configuration')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap ${
            activeTab === 'configuration'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          Configuration
        </button>
        <button
          onClick={() => setActiveTab('players')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap flex items-center gap-1.5 ${
            activeTab === 'players'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          <Users className="w-3.5 h-3.5" />
          <span>Players ({server.online_player_count}/{server.max_players})</span>
        </button>
        <button
          onClick={() => setActiveTab('plugins')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap ${
            activeTab === 'plugins'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          Plugins & Mods ({plugins.length + mods.length})
        </button>
        <button
          onClick={() => setActiveTab('worlds')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap ${
            activeTab === 'worlds'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          Worlds
        </button>
        <button
          onClick={() => setActiveTab('backups')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap ${
            activeTab === 'backups'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          Backups ({backups.length})
        </button>
        <button
          onClick={() => setActiveTab('networking')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap ${
            activeTab === 'networking'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          Networking
        </button>
        <button
          onClick={() => setActiveTab('resources')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap ${
            activeTab === 'resources'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          Resources
        </button>
        <button
          onClick={() => setActiveTab('settings')}
          className={`px-3 py-2 rounded-lg transition whitespace-nowrap ${
            activeTab === 'settings'
              ? 'bg-slate-800 text-white shadow-sm'
              : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
          }`}
        >
          Settings
        </button>
      </div>

      {/* Tab 1: OVERVIEW */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* Action Toolbar */}
          <div className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800 flex items-center justify-between gap-4 flex-wrap">
            <div className="flex items-center gap-2">
              {server.status === 'running' ? (
                <>
                  <button
                    disabled={actionLoading === 'stop'}
                    onClick={() => handleAction('stop')}
                    className="px-3.5 py-2 rounded-xl bg-slate-800 hover:bg-rose-950/60 hover:text-rose-400 text-slate-200 text-xs font-semibold transition flex items-center gap-1.5"
                  >
                    <Square className="w-3.5 h-3.5 text-rose-400" />
                    <span>Stop Server</span>
                  </button>
                  <button
                    disabled={actionLoading === 'restart'}
                    onClick={() => handleAction('restart')}
                    className="px-3.5 py-2 rounded-xl bg-slate-800 hover:bg-amber-950/60 hover:text-amber-400 text-slate-200 text-xs font-semibold transition flex items-center gap-1.5"
                  >
                    <RotateCw className="w-3.5 h-3.5 text-amber-400" />
                    <span>Restart</span>
                  </button>
                </>
              ) : (
                <button
                  disabled={actionLoading === 'start'}
                  onClick={() => handleAction('start')}
                  className="px-4 py-2 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold transition flex items-center gap-1.5 shadow-md shadow-emerald-600/25"
                >
                  <Play className="w-3.5 h-3.5" />
                  <span>Start Server</span>
                </button>
              )}
            </div>

            <div className="flex items-center gap-2">
              <button
                onClick={() => setActiveTab('console')}
                className="px-3.5 py-2 rounded-xl bg-violet-600/10 hover:bg-violet-600/20 text-violet-300 border border-violet-500/20 text-xs font-medium transition flex items-center gap-1.5"
              >
                <Terminal className="w-3.5 h-3.5" />
                <span>Open Live Console</span>
              </button>
            </div>
          </div>

          {/* Metrics Grid per PRD Section 61 */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            <div className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800">
              <div className="flex items-center justify-between text-xs text-slate-400 font-mono mb-2">
                <span>CPU UTILIZATION</span>
                <Cpu className="w-4 h-4 text-violet-400" />
              </div>
              <div className="text-2xl font-bold text-white font-mono">
                {metrics ? `${metrics.cpu_percent.toFixed(1)}%` : '0.0%'}
              </div>
              <div className="mt-2 text-xs text-slate-400 font-mono">
                Allocated: {server.vcpus} vCPU cores
              </div>
            </div>

            <div className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800">
              <div className="flex items-center justify-between text-xs text-slate-400 font-mono mb-2">
                <span>MEMORY (RAM)</span>
                <Activity className="w-4 h-4 text-emerald-400" />
              </div>
              <div className="text-2xl font-bold text-white font-mono">
                {metrics ? `${(metrics.memory_usage_mb / 1024).toFixed(1)} GB` : '0.0 GB'}
              </div>
              <div className="mt-2 text-xs text-slate-400 font-mono">
                Limit: {Math.round(server.memory_mb / 1024)} GB Dedicated
              </div>
            </div>

            <div className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800">
              <div className="flex items-center justify-between text-xs text-slate-400 font-mono mb-2">
                <span>TICK PERFORMANCE (TPS)</span>
                <Clock className="w-4 h-4 text-amber-400" />
              </div>
              <div className="text-2xl font-bold text-emerald-400 font-mono">
                {metrics ? `${metrics.tps.toFixed(1)} TPS` : '20.0 TPS'}
              </div>
              <div className="mt-2 text-xs text-slate-400 font-mono">
                MSPT: {metrics ? `${metrics.mspt.toFixed(1)} ms` : '5.2 ms'} (Target &lt; 50ms)
              </div>
            </div>

            <div className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800">
              <div className="flex items-center justify-between text-xs text-slate-400 font-mono mb-2">
                <span>ACTIVE PLAYERS</span>
                <Users className="w-4 h-4 text-cyan-400" />
              </div>
              <div className="text-2xl font-bold text-white font-mono">
                {server.online_player_count} / {server.max_players}
              </div>
              <div className="mt-2 text-xs text-slate-400 font-mono">
                World: {server.world_name}
              </div>
            </div>
          </div>

          {/* Server Details Grid */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <div className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800 space-y-3">
              <h3 className="text-sm font-semibold text-white">Server Information</h3>
              <div className="divide-y divide-slate-800/60 text-xs">
                <div className="py-2 flex justify-between">
                  <span className="text-slate-400">Assigned Public Port</span>
                  <span className="text-slate-200 font-mono">{server.assigned_port} / {server.protocol}</span>
                </div>
                <div className="py-2 flex justify-between">
                  <span className="text-slate-400">Dedicated Node</span>
                  <span className="text-violet-400 font-mono">{server.proxmox_node} (Dell R640)</span>
                </div>
                <div className="py-2 flex justify-between">
                  <span className="text-slate-400">Uptime</span>
                  <span className="text-slate-200 font-mono">
                    {Math.floor(server.uptime_seconds / 3600)}h {Math.floor((server.uptime_seconds % 3600) / 60)}m
                  </span>
                </div>
                <div className="py-2 flex justify-between">
                  <span className="text-slate-400">Auto Backups</span>
                  <span className="text-emerald-400 font-medium">
                    {server.auto_backup_enabled ? `Enabled (${server.backup_schedule})` : 'Disabled'}
                  </span>
                </div>
              </div>
            </div>

            <div className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800 space-y-3">
              <h3 className="text-sm font-semibold text-white">Connection Setup</h3>
              <p className="text-xs text-slate-400">
                To connect in multiplayer, open your game client, select Direct Connect or Add Server, and paste:
              </p>
              <div className="p-3 rounded-xl bg-slate-950 font-mono text-xs text-violet-300 border border-slate-800 flex items-center justify-between">
                <span>{connAddr}</span>
                <button
                  onClick={() => handleCopy(connAddr)}
                  className="p-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300"
                >
                  <Copy className="w-3.5 h-3.5" />
                </button>
              </div>
              {server.game_type === 'minecraft_java' && (
                <div className="text-[11px] text-slate-500 font-mono">
                  DNS SRV target: <code>_minecraft._tcp.play.domain.com 3600 IN SRV 10 0 {server.assigned_port} minecraft.nevatal.id</code>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Tab 2: LIVE CONSOLE per PRD Section 51.6 */}
      {activeTab === 'console' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="text-xs font-semibold text-white font-mono">LIVE TERMINAL FEED</span>
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
            </div>
            <label className="flex items-center gap-2 text-xs text-slate-400 cursor-pointer">
              <input
                type="checkbox"
                checked={autoScroll}
                onChange={(e) => setAutoScroll(e.target.checked)}
                className="rounded accent-violet-500 w-3.5 h-3.5"
              />
              <span>Auto-scroll to bottom</span>
            </label>
          </div>

          {/* Terminal Box */}
          <div className="bg-slate-950 border border-slate-800 rounded-2xl p-4 font-mono text-xs text-slate-200 h-[480px] overflow-y-auto space-y-1">
            {consoleLogs.map((line, idx) => (
              <div
                key={idx}
                className={
                  line.startsWith('>')
                    ? 'text-cyan-400 font-semibold'
                    : line.includes('WARN')
                    ? 'text-amber-400'
                    : line.includes('ERROR') || line.includes('[Error]')
                    ? 'text-rose-400'
                    : 'text-slate-300'
                }
              >
                {line}
              </div>
            ))}
            <div ref={consoleBottomRef} />
          </div>

          {/* Command Input Field */}
          <form onSubmit={handleSendCommand} className="flex gap-2">
            <div className="relative flex-1">
              <span className="absolute left-3.5 top-2.5 text-violet-400 font-mono font-bold">&gt;</span>
              <input
                type="text"
                value={commandInput}
                onChange={(e) => setCommandInput(e.target.value)}
                onKeyDown={handleConsoleKeyDown}
                placeholder="Type server command (e.g. /op player, /whitelist add, /time set day)..."
                className="w-full bg-slate-900 border border-slate-800 rounded-xl pl-8 pr-4 py-2.5 text-xs font-mono text-slate-200 placeholder-slate-500 focus:outline-none focus:border-violet-500"
              />
            </div>
            <button
              type="submit"
              className="px-4 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold transition flex items-center gap-1.5 shadow-md shadow-violet-600/25 shrink-0"
            >
              <Send className="w-3.5 h-3.5" />
              <span>Send</span>
            </button>
          </form>
        </div>
      )}

      {/* Tab 3: CONFIGURATION per PRD Section 51.2 & 52.2 */}
      {activeTab === 'configuration' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-base font-semibold text-white">Game Properties & Gameplay Rules</h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Saved properties update <code>server.properties</code> directly on the hypervisor.
              </p>
            </div>
            <button
              disabled={savingConfig}
              onClick={handleSaveConfig}
              className="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold transition shadow-md shadow-violet-600/25"
            >
              {savingConfig ? 'Saving...' : 'Save Changes'}
            </button>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Difficulty</label>
              <input
                type="text"
                value={configSettings['difficulty'] || 'normal'}
                onChange={(e) => setConfigSettings({ ...configSettings, difficulty: e.target.value })}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Game Mode</label>
              <input
                type="text"
                value={configSettings['gamemode'] || 'survival'}
                onChange={(e) => setConfigSettings({ ...configSettings, gamemode: e.target.value })}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">PvP Allowed</label>
              <select
                value={String(configSettings['pvp'] ?? 'true')}
                onChange={(e) => setConfigSettings({ ...configSettings, pvp: e.target.value === 'true' })}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500"
              >
                <option value="true">True (Enabled)</option>
                <option value="false">False (Disabled)</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Whitelist Enforced</label>
              <select
                value={String(configSettings['white-list'] ?? 'false')}
                onChange={(e) => setConfigSettings({ ...configSettings, 'white-list': e.target.value === 'true' })}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500"
              >
                <option value="true">True (Enforce Whitelist)</option>
                <option value="false">False (Open Access)</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">View Distance (Chunks)</label>
              <input
                type="number"
                value={configSettings['view-distance'] || 10}
                onChange={(e) => setConfigSettings({ ...configSettings, 'view-distance': Number(e.target.value) })}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500 font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Simulation Distance</label>
              <input
                type="number"
                value={configSettings['simulation-distance'] || 8}
                onChange={(e) => setConfigSettings({ ...configSettings, 'simulation-distance': Number(e.target.value) })}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500 font-mono"
              />
            </div>
          </div>
        </div>
      )}

      {/* Tab 4: PLAYERS per PRD Section 51.4 & 52.3 */}
      {activeTab === 'players' && (
        <div className="space-y-6">
          <div className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800 flex items-center justify-between gap-4 flex-wrap">
            <div className="flex items-center gap-2">
              <input
                type="text"
                value={newPlayerName}
                onChange={(e) => setNewPlayerName(e.target.value)}
                placeholder="Enter player username..."
                className="bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500 w-64"
              />
              <button
                onClick={() => handleAddPlayer('whitelist')}
                className="px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium transition"
              >
                Whitelist Player
              </button>
              <button
                onClick={() => handleAddPlayer('op')}
                className="px-3 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-medium transition"
              >
                Grant Operator (OP)
              </button>
            </div>
          </div>

          {/* Players Table */}
          <div className="bg-slate-900/60 border border-slate-800 rounded-2xl overflow-hidden">
            <div className="p-4 border-b border-slate-800 font-semibold text-xs text-white">
              Player Directory ({players.length})
            </div>
            {players.length === 0 ? (
              <div className="p-8 text-center text-slate-400 text-xs">
                No player records found. Players are logged as they connect to the game server.
              </div>
            ) : (
              <div className="divide-y divide-slate-800/60">
                {players.map((p) => (
                  <div key={p.id} className="p-4 flex items-center justify-between gap-3 text-xs">
                    <div className="flex items-center gap-3">
                      <div className="w-8 h-8 rounded-lg bg-slate-800 flex items-center justify-center font-bold text-violet-400">
                        {p.player_name.slice(0, 2).toUpperCase()}
                      </div>
                      <div>
                        <div className="flex items-center gap-2">
                          <span className="font-semibold text-white">{p.player_name}</span>
                          {p.is_operator && (
                            <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-violet-500/10 text-violet-400 border border-violet-500/20">
                              OP
                            </span>
                          )}
                          {p.is_online && (
                            <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                              Online
                            </span>
                          )}
                          {p.is_banned && (
                            <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">
                              Banned
                            </span>
                          )}
                        </div>
                        <div className="text-[11px] text-slate-500 font-mono mt-0.5">
                          UUID: {p.player_uuid || 'offline-uuid'}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        onClick={() => handleKickPlayer(p.player_name)}
                        className="px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium transition"
                      >
                        Kick
                      </button>
                      <button
                        onClick={() => handleBanPlayer(p.player_name)}
                        className="px-2.5 py-1.5 rounded-lg bg-rose-950/40 hover:bg-rose-950/80 text-rose-400 border border-rose-800/40 font-medium transition"
                      >
                        Ban
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Tab 5: PLUGINS & MODS per PRD Section 51.3 & 52.4 */}
      {activeTab === 'plugins' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-white">Installed Plugins & Mods</h2>
            <div className="flex items-center gap-2">
              <button
                onClick={() => alert('JAR upload handler is active. Select files to upload to /plugins directory.')}
                className="flex items-center gap-1.5 px-3.5 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold transition"
              >
                <Upload className="w-3.5 h-3.5" />
                <span>Upload Plugin / Mod JAR</span>
              </button>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {plugins.map((pl) => (
              <div
                key={pl.id}
                className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800 flex items-center justify-between gap-3"
              >
                <div>
                  <div className="flex items-center gap-2">
                    <span className="font-semibold text-sm text-white">{pl.name}</span>
                    <span className="text-[11px] font-mono text-slate-400">v{pl.version}</span>
                  </div>
                  <p className="text-xs text-slate-400 mt-1">{pl.description || pl.filename}</p>
                </div>
                <button
                  onClick={() => handleTogglePlugin(pl.id, pl.enabled)}
                  className={`px-3 py-1.5 rounded-xl text-xs font-semibold transition ${
                    pl.enabled
                      ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                      : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  {pl.enabled ? 'Enabled' : 'Disabled'}
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Tab 6: WORLDS per PRD Section 51.5 */}
      {activeTab === 'worlds' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-white">World Storage Management</h2>
            <button
              onClick={() => setShowResetModal(true)}
              className="px-3.5 py-2 rounded-xl bg-rose-950/40 hover:bg-rose-950/80 text-rose-400 border border-rose-800/40 text-xs font-semibold transition"
            >
              Reset World
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {worlds.map((w) => (
              <div key={w.id} className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800 space-y-3">
                <div className="flex items-center justify-between">
                  <span className="font-semibold text-base text-white">{w.world_name}</span>
                  {w.is_active && (
                    <span className="px-2 py-0.5 rounded text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                      Active
                    </span>
                  )}
                </div>
                <div className="text-xs text-slate-400 font-mono space-y-1">
                  <div>Size: {(w.size_bytes / (1024 * 1024)).toFixed(1)} MB</div>
                  <div>Difficulty: {w.difficulty || 'normal'}</div>
                  <div>Seed: {w.world_seed || 'Default generated seed'}</div>
                </div>
              </div>
            ))}
          </div>

          {/* Reset World Confirmation Modal */}
          {showResetModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4">
              <div className="bg-slate-900 border border-rose-500/30 rounded-2xl p-6 max-w-md w-full space-y-4">
                <div className="w-12 h-12 rounded-xl bg-rose-500/10 text-rose-400 flex items-center justify-center">
                  <AlertTriangle className="w-6 h-6" />
                </div>
                <h3 className="text-lg font-bold text-white">Reset World progress?</h3>
                <p className="text-xs text-slate-400 leading-relaxed">
                  This action will delete all terrain, player buildings, inventories, and generated chunks.
                  A new random world will be created on the next reboot.
                </p>
                <div className="flex justify-end gap-2 pt-2">
                  <button
                    onClick={() => setShowResetModal(false)}
                    className="px-4 py-2 rounded-xl bg-slate-800 text-xs text-slate-300 font-medium"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={handleResetWorld}
                    className="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold transition"
                  >
                    Confirm Reset
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}

      {/* Tab 7: BACKUPS per PRD Section 55 */}
      {activeTab === 'backups' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h2 className="text-base font-semibold text-white">World Backups & Restore Points</h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Saved snapshots include full world directories, plugins, configs, and player state.
              </p>
            </div>
            <button
              disabled={creatingBackup}
              onClick={handleCreateBackup}
              className="flex items-center gap-1.5 px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold transition shadow-md shadow-violet-600/25"
            >
              <Database className="w-3.5 h-3.5" />
              <span>{creatingBackup ? 'Compressing World...' : 'Create Snapshot'}</span>
            </button>
          </div>

          <div className="bg-slate-900/60 border border-slate-800 rounded-2xl overflow-hidden divide-y divide-slate-800/60">
            {backups.length === 0 ? (
              <div className="p-8 text-center text-slate-400 text-xs">
                No backups recorded. Click "Create Snapshot" to generate an immediate restore point.
              </div>
            ) : (
              backups.map((b) => (
                <div key={b.id} className="p-4 flex items-center justify-between gap-3 text-xs">
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-white">{b.backup_name}</span>
                      {b.is_automated ? (
                        <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-blue-500/10 text-blue-400">
                          Scheduled
                        </span>
                      ) : (
                        <span className="px-1.5 py-0.5 rounded text-[10px] font-semibold bg-amber-500/10 text-amber-400">
                          Manual
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-3 text-[11px] text-slate-400 font-mono mt-1">
                      <span>{(b.file_size_bytes / (1024 * 1024)).toFixed(1)} MB</span>
                      <span>•</span>
                      <span>{new Date(b.created_at).toLocaleString()}</span>
                    </div>
                  </div>

                  <div className="flex items-center gap-2">
                    <button
                      disabled={restoringBackupId === b.id}
                      onClick={() => handleRestoreBackup(b.id)}
                      className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-200 font-medium transition"
                    >
                      {restoringBackupId === b.id ? 'Restoring...' : 'Restore'}
                    </button>
                  </div>
                </div>
              ))
            )}
          </div>
        </div>
      )}

      {/* Tab 8: NETWORKING per PRD Section 54 */}
      {activeTab === 'networking' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-6">
          <h2 className="text-base font-semibold text-white">Network Endpoints & DNS SRV</h2>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-2 text-xs font-mono">
              <span className="text-slate-400 font-sans font-medium">Assigned Port</span>
              <div className="text-xl font-bold text-violet-300">{server.assigned_port}</div>
              <div className="text-slate-500">Protocol: {server.protocol} (Direct Inbound)</div>
            </div>

            <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-2 text-xs font-mono">
              <span className="text-slate-400 font-sans font-medium">Public Connection String</span>
              <div className="text-xl font-bold text-violet-300">{server.connection_address}</div>
              <div className="text-slate-500">Nevatal Cloud Anycast Gateway</div>
            </div>
          </div>

          <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-3">
            <h3 className="text-xs font-semibold text-white uppercase tracking-wider">DNS SRV Custom Domain Setup</h3>
            <p className="text-xs text-slate-400 leading-relaxed">
              To use your own domain (e.g. <code>play.yourdomain.com</code>) without displaying the port number, add this SRV record in your DNS provider:
            </p>
            <div className="p-3 rounded-xl bg-slate-950 font-mono text-xs text-slate-300 border border-slate-800 overflow-x-auto select-all">
              _minecraft._tcp.play 3600 IN SRV 0 5 {server.assigned_port} minecraft.nevatal.id.
            </div>
          </div>
        </div>
      )}

      {/* Tab 9: RESOURCES */}
      {activeTab === 'resources' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-6">
          <h2 className="text-base font-semibold text-white">Bare-Metal Hardware Specs</h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs font-mono">
            <div className="p-4 rounded-xl bg-slate-900 border border-slate-800">
              <span className="text-slate-400">Processor</span>
              <div className="text-lg font-bold text-white mt-1">{server.vcpus} vCPU Cores</div>
              <div className="text-slate-500 mt-1">Dell PowerEdge R640 Xeon</div>
            </div>
            <div className="p-4 rounded-xl bg-slate-900 border border-slate-800">
              <span className="text-slate-400">Memory Allocation</span>
              <div className="text-lg font-bold text-white mt-1">{server.memory_mb / 1024} GB RAM</div>
              <div className="text-slate-500 mt-1">ECC DDR4 High-Bandwidth</div>
            </div>
            <div className="p-4 rounded-xl bg-slate-900 border border-slate-800">
              <span className="text-slate-400">Persistent Disk</span>
              <div className="text-lg font-bold text-white mt-1">{server.storage_gb} GB NVMe</div>
              <div className="text-slate-500 mt-1">local-zfs enterprise pool</div>
            </div>
          </div>
        </div>
      )}

      {/* Tab 10: SETTINGS */}
      {activeTab === 'settings' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-6">
          <h2 className="text-base font-semibold text-white">Server Configuration & Danger Zone</h2>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Server Display Name</label>
              <input
                type="text"
                value={editName}
                onChange={(e) => setEditName(e.target.value)}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Max Players Capacity</label>
              <input
                type="number"
                value={editMaxPlayers}
                onChange={(e) => setEditMaxPlayers(Number(e.target.value))}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500 font-mono"
              />
            </div>

            <div className="sm:col-span-2">
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Custom Domain Hostname</label>
              <input
                type="text"
                value={editCustomDomain}
                onChange={(e) => setEditCustomDomain(e.target.value)}
                placeholder="e.g. play.mydomain.com"
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-slate-200 focus:outline-none focus:border-violet-500 font-mono"
              />
            </div>
          </div>

          <div className="flex justify-start">
            <button
              onClick={handleSaveSettings}
              className="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-xs font-semibold transition"
            >
              Save Server Settings
            </button>
          </div>

          {/* Danger Zone */}
          <div className="pt-6 border-t border-slate-800 space-y-4">
            <h3 className="text-xs font-bold text-rose-400 uppercase tracking-wider">Danger Zone</h3>

            <div className="flex items-center justify-between p-4 rounded-xl bg-rose-950/20 border border-rose-900/30">
              <div>
                <span className="text-xs font-semibold text-white">Reinstall Server Software</span>
                <p className="text-[11px] text-slate-400">Re-downloads server binary files while preserving worlds.</p>
              </div>
              <button
                onClick={() => handleAction('reinstall')}
                className="px-3.5 py-2 rounded-xl bg-slate-800 hover:bg-rose-900 text-rose-300 text-xs font-semibold transition"
              >
                Reinstall
              </button>
            </div>

            <div className="flex items-center justify-between p-4 rounded-xl bg-rose-950/20 border border-rose-900/30">
              <div>
                <span className="text-xs font-semibold text-white">Delete Game Server</span>
                <p className="text-[11px] text-slate-400">Permanently deallocates port, container, and world storage.</p>
              </div>
              <button
                onClick={() => setShowDeleteModal(true)}
                className="px-3.5 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold transition"
              >
                Delete Server
              </button>
            </div>
          </div>

          {/* Delete Modal */}
          {showDeleteModal && (
            <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 backdrop-blur-sm p-4">
              <div className="bg-slate-900 border border-rose-500/30 rounded-2xl p-6 max-w-md w-full space-y-4">
                <h3 className="text-base font-bold text-white">Are you sure you want to delete this game server?</h3>
                <p className="text-xs text-slate-400">
                  This action is irreversible. The port {server.assigned_port} will be released and world files deleted.
                </p>
                <div className="flex justify-end gap-2 pt-2">
                  <button
                    onClick={() => setShowDeleteModal(false)}
                    className="px-4 py-2 rounded-xl bg-slate-800 text-xs text-slate-300 font-medium"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={() => handleAction('delete')}
                    className="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold transition"
                  >
                    Yes, Delete Server
                  </button>
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </div>
  );
};
