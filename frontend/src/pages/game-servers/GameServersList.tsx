import React, { useState, useEffect } from 'react';
import {
  Gamepad2,
  PlusCircle,
  Search,
  RefreshCw,
  Copy,
  Check,
  Play,
  Square,
  RotateCw,
  Terminal,
  Shield,
  Users,
  HardDrive,
  Cpu,
  Layers,
  Sparkles,
  ArrowRight,
  Database,
  Download,
  Trash2,
  Calendar,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import {
  GameServer,
  GameServerTemplate,
  GameServerBackup,
} from '../../types';

interface GameServersListProps {
  onNavigate: (path: string, param?: string) => void;
}

export const GameServersList: React.FC<GameServersListProps> = ({ onNavigate }) => {
  const [servers, setServers] = useState<GameServer[]>([]);
  const [templates, setTemplates] = useState<GameServerTemplate[]>([]);
  const [backups, setBackups] = useState<GameServerBackup[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'all' | 'minecraft' | 'terraria' | 'backups' | 'templates'>('all');
  const [searchTerm, setSearchTerm] = useState('');
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const fetchData = async () => {
    try {
      setLoading(true);
      const [serversRes, templatesRes] = await Promise.all([
        apiRequest<GameServer[]>('/game-servers'),
        apiRequest<GameServerTemplate[]>('/game-servers/templates'),
      ]);
      setServers(serversRes || []);
      setTemplates(templatesRes || []);

      // Collect backups across all servers
      const allBackups: GameServerBackup[] = [];
      if (serversRes && serversRes.length > 0) {
        for (const s of serversRes) {
          try {
            const bks = await apiRequest<GameServerBackup[]>(`/game-servers/${s.id}/backups`);
            if (bks) {
              allBackups.push(...bks);
            }
          } catch {
            // Ignore per-server backup fetch errors
          }
        }
      }
      setBackups(allBackups);
    } catch (err) {
      console.error('Failed to load game servers data:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const handleAction = async (serverId: string, action: 'start' | 'stop' | 'restart', e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      setActionLoading(`${serverId}-${action}`);
      await apiRequest(`/game-servers/${serverId}/${action}`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(`Action failed: ${err.message || 'Unknown error'}`);
    } finally {
      setActionLoading(null);
    }
  };

  const filteredServers = servers.filter((s) => {
    const matchesSearch =
      s.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      s.connection_address.toLowerCase().includes(searchTerm.toLowerCase()) ||
      s.server_software.toLowerCase().includes(searchTerm.toLowerCase());

    if (!matchesSearch) return false;

    if (activeTab === 'minecraft') {
      return s.game_type === 'minecraft_java' || s.game_type === 'minecraft_bedrock';
    }
    if (activeTab === 'terraria') {
      return s.game_type === 'terraria';
    }
    return true;
  });

  const getStatusBadge = (status: string) => {
    switch (status) {
      case 'running':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
            Online
          </span>
        );
      case 'provisioning':
      case 'restarting':
      case 'reinstalling':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20">
            <RefreshCw className="w-3 h-3 animate-spin" />
            <span className="capitalize">{status}</span>
          </span>
        );
      case 'stopped':
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-slate-500/10 text-slate-400 border border-slate-500/20">
            <span className="w-1.5 h-1.5 rounded-full bg-slate-500" />
            Offline
          </span>
        );
      default:
        return (
          <span className="inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium bg-rose-500/10 text-rose-400 border border-rose-500/20 capitalize">
            {status}
          </span>
        );
    }
  };

  const getGameBadge = (type: string, software: string) => {
    if (type === 'minecraft_java') {
      return (
        <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-emerald-950/70 text-emerald-400 border border-emerald-800/60 uppercase">
          Minecraft Java • {software}
        </span>
      );
    }
    if (type === 'minecraft_bedrock') {
      return (
        <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-teal-950/70 text-teal-400 border border-teal-800/60 uppercase">
          Minecraft Bedrock
        </span>
      );
    }
    return (
      <span className="px-2 py-0.5 rounded text-[11px] font-semibold bg-amber-950/70 text-amber-400 border border-amber-800/60 uppercase">
        Terraria • {software}
      </span>
    );
  };

  return (
    <div className="space-y-6">
      {/* Header Banner */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-gradient-to-r from-slate-900 via-slate-900/90 to-indigo-950/40 p-6 rounded-2xl border border-slate-800 shadow-xl backdrop-blur">
        <div>
          <div className="flex items-center gap-2.5">
            <div className="w-10 h-10 rounded-xl bg-violet-600/20 text-violet-400 flex items-center justify-center border border-violet-500/30">
              <Gamepad2 className="w-5 h-5" />
            </div>
            <div>
              <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">Game Servers</h1>
              <p className="text-sm text-slate-400">
                Managed Minecraft & Terraria servers on Dell PowerEdge R640 high-frequency bare-metal.
              </p>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={() => onNavigate('/dashboard/ai')}
            className="flex items-center gap-2 px-3.5 py-2 rounded-xl bg-slate-800/90 hover:bg-slate-800 text-slate-300 text-sm font-medium border border-slate-700/80 transition shadow-sm"
          >
            <Sparkles className="w-4 h-4 text-violet-400" />
            <span>AI Provision</span>
          </button>
          <button
            onClick={() => onNavigate('/dashboard/game-servers/new')}
            className="flex items-center gap-2 px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-sm font-semibold transition shadow-lg shadow-violet-600/25"
          >
            <PlusCircle className="w-4 h-4" />
            <span>Create Server</span>
          </button>
        </div>
      </div>

      {/* Navigation Sub-Tabs per PRD Section 49.3 */}
      <div className="flex items-center justify-between gap-4 border-b border-slate-800 pb-2 overflow-x-auto">
        <div className="flex items-center gap-1.5 min-w-max">
          <button
            onClick={() => setActiveTab('all')}
            className={`px-3.5 py-1.5 rounded-lg text-sm font-medium transition ${
              activeTab === 'all'
                ? 'bg-slate-800 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
            }`}
          >
            All Servers ({servers.length})
          </button>
          <button
            onClick={() => setActiveTab('minecraft')}
            className={`px-3.5 py-1.5 rounded-lg text-sm font-medium transition ${
              activeTab === 'minecraft'
                ? 'bg-slate-800 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
            }`}
          >
            Minecraft ({servers.filter((s) => s.game_type.startsWith('minecraft')).length})
          </button>
          <button
            onClick={() => setActiveTab('terraria')}
            className={`px-3.5 py-1.5 rounded-lg text-sm font-medium transition ${
              activeTab === 'terraria'
                ? 'bg-slate-800 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
            }`}
          >
            Terraria ({servers.filter((s) => s.game_type === 'terraria').length})
          </button>
          <button
            onClick={() => setActiveTab('backups')}
            className={`px-3.5 py-1.5 rounded-lg text-sm font-medium transition ${
              activeTab === 'backups'
                ? 'bg-slate-800 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
            }`}
          >
            Backups ({backups.length})
          </button>
          <button
            onClick={() => setActiveTab('templates')}
            className={`px-3.5 py-1.5 rounded-lg text-sm font-medium transition ${
              activeTab === 'templates'
                ? 'bg-slate-800 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/40'
            }`}
          >
            Templates ({templates.length})
          </button>
        </div>

        {activeTab !== 'backups' && activeTab !== 'templates' && (
          <div className="relative w-64 max-w-full">
            <Search className="w-4 h-4 absolute left-3 top-2.5 text-slate-500" />
            <input
              type="text"
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              placeholder="Search server or IP..."
              className="w-full bg-slate-900 border border-slate-800 rounded-xl pl-9 pr-3 py-1.5 text-xs text-slate-200 placeholder-slate-500 focus:outline-none focus:border-violet-500"
            />
          </div>
        )}
      </div>

      {/* Main Tab Content */}
      {loading ? (
        <div className="py-16 flex flex-col items-center justify-center gap-3 text-slate-400">
          <RefreshCw className="w-8 h-8 animate-spin text-violet-500" />
          <span className="text-sm font-mono">Loading game instances...</span>
        </div>
      ) : activeTab === 'templates' ? (
        /* Templates Tab */
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {templates.map((tpl) => (
            <div
              key={tpl.id}
              className="bg-slate-900/60 border border-slate-800 rounded-2xl p-5 hover:border-slate-700 transition flex flex-col justify-between"
            >
              <div>
                <div className="flex items-center justify-between mb-3">
                  <span className="text-2xl">{tpl.icon}</span>
                  <span className="px-2 py-0.5 rounded text-[10px] font-mono font-medium uppercase bg-slate-800 text-slate-300 border border-slate-700">
                    Port {tpl.default_port}/{tpl.protocol}
                  </span>
                </div>
                <h3 className="text-base font-semibold text-white">{tpl.name}</h3>
                <p className="text-xs text-slate-400 mt-1 line-clamp-2">{tpl.description}</p>
                <div className="mt-4 flex items-center gap-2">
                  <span className="text-[11px] font-mono text-slate-400 bg-slate-800/80 px-2 py-1 rounded">
                    Default Version: {tpl.default_version}
                  </span>
                </div>
              </div>

              <div className="mt-5 pt-4 border-t border-slate-800/80 flex items-center justify-end">
                <button
                  onClick={() => onNavigate('/dashboard/game-servers/new')}
                  className="flex items-center gap-1.5 text-xs font-medium text-violet-400 hover:text-violet-300 transition"
                >
                  <span>Launch Template</span>
                  <ArrowRight className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : activeTab === 'backups' ? (
        /* Backups Tab */
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl overflow-hidden">
          <div className="p-4 border-b border-slate-800 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white flex items-center gap-2">
              <Database className="w-4 h-4 text-violet-400" />
              <span>Automated & Manual World Backups</span>
            </h3>
            <span className="text-xs text-slate-400 font-mono">{backups.length} archives recorded</span>
          </div>

          {backups.length === 0 ? (
            <div className="p-8 text-center text-slate-400 text-sm">
              No backups created yet. World backups are created automatically or on demand inside each server console.
            </div>
          ) : (
            <div className="divide-y divide-slate-800/60">
              {backups.map((b) => (
                <div key={b.id} className="p-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3 hover:bg-slate-800/20 transition">
                  <div className="flex items-start gap-3">
                    <div className="w-9 h-9 rounded-xl bg-violet-600/10 text-violet-400 flex items-center justify-center border border-violet-500/20 shrink-0">
                      <Database className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-medium text-sm text-white">{b.backup_name}</span>
                        {b.is_automated ? (
                          <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20">
                            Auto
                          </span>
                        ) : (
                          <span className="px-1.5 py-0.5 rounded text-[10px] font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">
                            Manual
                          </span>
                        )}
                      </div>
                      <div className="flex items-center gap-3 text-xs text-slate-400 mt-1 font-mono">
                        <span>World: {b.world_name}</span>
                        <span>•</span>
                        <span>{(b.file_size_bytes / (1024 * 1024)).toFixed(1)} MB</span>
                        <span>•</span>
                        <span>{new Date(b.created_at).toLocaleString()}</span>
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-2 self-end sm:self-center">
                    <button
                      onClick={() => onNavigate('/dashboard/game-servers', b.server_id)}
                      className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-xs text-slate-300 font-medium transition"
                    >
                      Manage in Server
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      ) : filteredServers.length === 0 ? (
        /* Empty State */
        <div className="bg-slate-900/40 border border-slate-800/80 rounded-2xl p-12 text-center flex flex-col items-center max-w-lg mx-auto">
          <div className="w-14 h-14 rounded-2xl bg-violet-600/10 text-violet-400 flex items-center justify-center border border-violet-500/20 mb-4">
            <Gamepad2 className="w-7 h-7" />
          </div>
          <h3 className="text-lg font-semibold text-white">No game servers deployed</h3>
          <p className="text-sm text-slate-400 mt-2 leading-relaxed">
            Provision high-performance Minecraft Java, Bedrock, or Terraria servers in under 60 seconds with persistent worlds and DDoS protection.
          </p>
          <div className="flex items-center gap-3 mt-6">
            <button
              onClick={() => onNavigate('/dashboard/game-servers/new')}
              className="px-4 py-2 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-sm font-semibold transition shadow-lg shadow-violet-600/20 flex items-center gap-2"
            >
              <PlusCircle className="w-4 h-4" />
              <span>Create Game Server</span>
            </button>
          </div>
        </div>
      ) : (
        /* Server Cards Grid */
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {filteredServers.map((server) => (
            <div
              key={server.id}
              onClick={() => onNavigate('/dashboard/game-servers', server.id)}
              className="bg-slate-900/60 hover:bg-slate-900 border border-slate-800 hover:border-violet-500/40 rounded-2xl p-5 transition cursor-pointer flex flex-col justify-between group shadow-sm"
            >
              {/* Card Header */}
              <div>
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-start gap-3">
                    <div className="w-10 h-10 rounded-xl bg-violet-600/10 text-violet-400 flex items-center justify-center border border-violet-500/20 shrink-0 group-hover:scale-105 transition">
                      <Gamepad2 className="w-5 h-5" />
                    </div>
                    <div>
                      <div className="flex items-center gap-2 flex-wrap">
                        <h3 className="font-semibold text-base text-white group-hover:text-violet-300 transition">
                          {server.name}
                        </h3>
                        {getStatusBadge(server.status)}
                      </div>
                      <div className="flex items-center gap-2 mt-1">
                        {getGameBadge(server.game_type, server.server_software)}
                        <span className="text-[11px] font-mono text-slate-400">
                          v{server.game_version}
                        </span>
                      </div>
                    </div>
                  </div>
                </div>

                {/* Connection Address with Copy Button */}
                <div className="mt-4 p-2.5 rounded-xl bg-slate-950/80 border border-slate-800/80 flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2 overflow-hidden">
                    <span className="text-[11px] font-mono uppercase text-slate-500 font-semibold shrink-0">Address:</span>
                    <span className="text-xs font-mono text-violet-300 truncate select-all">
                      {server.custom_domain ? server.custom_domain : server.connection_address}
                    </span>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleCopy(
                        server.custom_domain || server.connection_address,
                        server.id
                      );
                    }}
                    className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs transition shrink-0"
                    title="Copy connection address"
                  >
                    {copiedId === server.id ? (
                      <Check className="w-3.5 h-3.5 text-emerald-400" />
                    ) : (
                      <Copy className="w-3.5 h-3.5" />
                    )}
                  </button>
                </div>

                {/* Stats Bar */}
                <div className="mt-4 grid grid-cols-3 gap-2 py-2 px-3 rounded-xl bg-slate-800/30 text-xs font-mono text-slate-400">
                  <div className="flex items-center gap-1.5">
                    <Users className="w-3.5 h-3.5 text-slate-500" />
                    <span>
                      <strong className="text-slate-200">{server.online_player_count}</strong>/{server.max_players}
                    </span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <Cpu className="w-3.5 h-3.5 text-slate-500" />
                    <span>{server.vcpus} vCPU • {Math.round(server.memory_mb / 1024)}GB</span>
                  </div>
                  <div className="flex items-center gap-1.5">
                    <HardDrive className="w-3.5 h-3.5 text-slate-500" />
                    <span>{server.storage_gb} GB NVMe</span>
                  </div>
                </div>
              </div>

              {/* Card Footer Actions */}
              <div className="mt-5 pt-3 border-t border-slate-800/70 flex items-center justify-between">
                <div className="flex items-center gap-1.5">
                  {server.status === 'running' ? (
                    <>
                      <button
                        disabled={actionLoading === `${server.id}-stop`}
                        onClick={(e) => handleAction(server.id, 'stop', e)}
                        className="px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-rose-950/60 hover:text-rose-400 text-slate-300 text-xs font-medium transition flex items-center gap-1"
                        title="Stop server"
                      >
                        <Square className="w-3 h-3 text-rose-400" />
                        <span>Stop</span>
                      </button>
                      <button
                        disabled={actionLoading === `${server.id}-restart`}
                        onClick={(e) => handleAction(server.id, 'restart', e)}
                        className="px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-amber-950/60 hover:text-amber-400 text-slate-300 text-xs font-medium transition flex items-center gap-1"
                        title="Restart server"
                      >
                        <RotateCw className="w-3 h-3 text-amber-400" />
                        <span>Restart</span>
                      </button>
                    </>
                  ) : (
                    <button
                      disabled={actionLoading === `${server.id}-start`}
                      onClick={(e) => handleAction(server.id, 'start', e)}
                      className="px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition flex items-center gap-1 shadow-sm"
                      title="Start server"
                    >
                      <Play className="w-3 h-3" />
                      <span>Start</span>
                    </button>
                  )}
                </div>

                <div className="flex items-center gap-2">
                  <span className="text-xs text-slate-400 font-medium group-hover:text-white transition flex items-center gap-1">
                    <span>Manage Server</span>
                    <ArrowRight className="w-3.5 h-3.5" />
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
