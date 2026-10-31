import React, { useState, useEffect } from 'react';
import {
  Server,
  Globe,
  Workflow,
  Bot,
  Database,
  Search,
  RefreshCw,
  ExternalLink,
  ChevronRight,
  Layers,
  Sparkles,
  Cpu,
  PlusCircle,
  HardDrive,
  Gamepad2,
  Smartphone,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { UnifiedResourceItem, QuotaUsage } from '../../types';

interface UnifiedResourcesProps {
  onNavigate: (path: string, param?: string) => void;
}

export const UnifiedResources: React.FC<UnifiedResourcesProps> = ({ onNavigate }) => {
  const [resources, setResources] = useState<UnifiedResourceItem[]>([]);
  const [counts, setCounts] = useState<{
    total: number;
    vps: number;
    apps: number;
    automations: number;
    ai_agents: number;
    buckets: number;
    game_servers: number;
    android_instances: number;
  }>({ total: 0, vps: 0, apps: 0, automations: 0, ai_agents: 0, buckets: 0, game_servers: 0, android_instances: 0 });
  const [usage, setUsage] = useState<QuotaUsage | null>(null);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [filterType, setFilterType] = useState<string>('all');

  const loadResources = async () => {
    try {
      setLoading(true);
      const res = await apiRequest<{
        resources: UnifiedResourceItem[];
        usage: QuotaUsage;
        counts: any;
      }>('/resources');
      setResources(res.resources || []);
      setCounts(res.counts || { total: 0, vps: 0, apps: 0, automations: 0, ai_agents: 0, buckets: 0 });
      setUsage(res.usage);
    } catch (err) {
      console.error('Failed to load cloud resources:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadResources();
  }, []);

  const filtered = resources.filter((r) => {
    const matchesSearch = r.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (r.endpoint && r.endpoint.toLowerCase().includes(searchTerm.toLowerCase())) ||
      (r.specs && r.specs.toLowerCase().includes(searchTerm.toLowerCase()));

    if (filterType === 'all') return matchesSearch;
    return matchesSearch && r.type === filterType;
  });

  const getIcon = (type: string) => {
    switch (type) {
      case 'vps':
        return <Server className="w-4 h-4 text-blue-400" />;
      case 'app':
        return <Globe className="w-4 h-4 text-emerald-400" />;
      case 'automation':
        return <Workflow className="w-4 h-4 text-amber-400" />;
      case 'ai_agent':
        return <Bot className="w-4 h-4 text-purple-400" />;
      case 'bucket':
        return <Database className="w-4 h-4 text-cyan-400" />;
      case 'game_server':
        return <Gamepad2 className="w-4 h-4 text-violet-400" />;
      case 'android_instance':
        return <Smartphone className="w-4 h-4 text-emerald-400" />;
      default:
        return <Layers className="w-4 h-4 text-slate-400" />;
    }
  };

  const getTypeLabel = (type: string) => {
    switch (type) {
      case 'vps':
        return 'Compute VPS';
      case 'app':
        return 'Web Application';
      case 'automation':
        return 'n8n Automation';
      case 'ai_agent':
        return 'OpenClaw Agent';
      case 'bucket':
        return 'S3 Storage Bucket';
      case 'game_server':
        return 'Game Server';
      case 'android_instance':
        return 'Android Cloud';
      default:
        return type;
    }
  };

  const getStatusBadge = (status: string) => {
    const s = status.toLowerCase();
    if (s === 'running' || s === 'active' || s === 'completed') {
      return (
        <span className="px-2 py-0.5 rounded-md text-[11px] font-mono bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 inline-flex items-center gap-1.5">
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
          <span className="capitalize">{status}</span>
        </span>
      );
    }
    if (s === 'provisioning' || s === 'starting' || s === 'queued') {
      return (
        <span className="px-2 py-0.5 rounded-md text-[11px] font-mono bg-blue-500/10 text-blue-400 border border-blue-500/20 inline-flex items-center gap-1.5">
          <RefreshCw className="w-3 h-3 animate-spin" />
          <span className="capitalize">{status}</span>
        </span>
      );
    }
    if (s === 'stopped') {
      return (
        <span className="px-2 py-0.5 rounded-md text-[11px] font-mono bg-zinc-900 text-zinc-400 border border-zinc-800">
          Stopped
        </span>
      );
    }
    return (
      <span className="px-2 py-0.5 rounded-md text-[11px] font-mono bg-rose-500/10 text-rose-400 border border-rose-500/20 capitalize">
        {status}
      </span>
    );
  };

  const handleOpenResource = (r: UnifiedResourceItem) => {
    switch (r.type) {
      case 'vps':
        onNavigate('/dashboard/instances', r.id);
        break;
      case 'app':
        onNavigate('/dashboard/apps', r.id);
        break;
      case 'automation':
        onNavigate('/dashboard/automation', r.id);
        break;
      case 'ai_agent':
        onNavigate('/dashboard/ai-agents', r.id);
        break;
      case 'bucket':
        onNavigate('/dashboard/storage/files', r.id);
        break;
      case 'game_server':
        onNavigate('/dashboard/game-servers', r.id);
        break;
      case 'android_instance':
        onNavigate('/dashboard/android', r.id);
        break;
      default:
        break;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-zinc-800 pb-5">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl sm:text-2xl font-semibold text-white tracking-tight">Cloud Resources</h1>
            <span className="px-2 py-0.5 rounded-md bg-zinc-900 border border-zinc-800 text-zinc-400 font-mono text-[10px]">
              unified
            </span>
          </div>
          <p className="text-xs text-zinc-400 mt-1 font-mono">
            Inventory of virtual machines, applications, automations, and storage provisioned on the hypervisor
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={loadResources}
            disabled={loading}
            className="p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 hover:text-white border border-zinc-800 transition"
            title="Refresh Resources"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => onNavigate('/dashboard/ai')}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 font-medium text-xs transition"
          >
            <Sparkles className="w-3.5 h-3.5" />
            <span>AI Cloud Agent</span>
          </button>
        </div>
      </div>

      {/* Metrics Row */}
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-8 gap-2.5">
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Total</div>
          <div className="text-xl font-semibold text-white mt-1">{counts.total}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Compute VPS</div>
          <div className="text-xl font-semibold text-white mt-1">{counts.vps}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Web Apps</div>
          <div className="text-xl font-semibold text-white mt-1">{counts.apps}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Automations</div>
          <div className="text-xl font-semibold text-white mt-1">{counts.automations}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">AI Agents</div>
          <div className="text-xl font-semibold text-white mt-1">{counts.ai_agents}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">S3 Buckets</div>
          <div className="text-xl font-semibold text-white mt-1">{counts.buckets}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Game Servers</div>
          <div className="text-xl font-semibold text-white mt-1">{counts.game_servers}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Android Cloud</div>
          <div className="text-xl font-semibold text-white mt-1">{counts.android_instances || 0}</div>
        </div>
      </div>

      {/* Search & Filter Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        {/* Type Filter Tabs */}
        <div className="flex items-center gap-1 overflow-x-auto pb-1 sm:pb-0">
          {[
            { id: 'all', label: 'All' },
            { id: 'vps', label: 'VPS', icon: Server },
            { id: 'app', label: 'Apps', icon: Globe },
            { id: 'automation', label: 'n8n', icon: Workflow },
            { id: 'ai_agent', label: 'OpenClaw', icon: Bot },
            { id: 'bucket', label: 'S3', icon: Database },
            { id: 'game_server', label: 'Games', icon: Gamepad2 },
            { id: 'android_instance', label: 'Android', icon: Smartphone },
          ].map((tab) => {
            const active = filterType === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setFilterType(tab.id)}
                className={`flex items-center gap-1.5 px-2.5 py-1 rounded-md text-xs font-medium whitespace-nowrap transition ${
                  active
                    ? 'bg-zinc-800 text-white font-semibold'
                    : 'text-zinc-400 hover:text-white hover:bg-zinc-850'
                }`}
              >
                {tab.icon && <tab.icon className="w-3 h-3 text-zinc-400" />}
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>

        {/* Search */}
        <div className="relative min-w-[220px]">
          <Search className="w-3.5 h-3.5 text-zinc-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Search resources..."
            className="w-full bg-zinc-950 border border-zinc-800 rounded-md pl-8 pr-2.5 py-1 text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
          />
        </div>
      </div>

      {/* Resources Table */}
      <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-zinc-950 text-zinc-400 uppercase font-mono text-[10px] tracking-wider border-b border-zinc-800">
              <tr>
                <th className="px-3.5 py-2.5">Resource</th>
                <th className="px-3.5 py-2.5">Type</th>
                <th className="px-3.5 py-2.5">Status</th>
                <th className="px-3.5 py-2.5">Endpoint / Access</th>
                <th className="px-3.5 py-2.5">Details</th>
                <th className="px-3.5 py-2.5">Created</th>
                <th className="px-3.5 py-2.5 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-850 text-zinc-200">
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-3.5 py-10 text-center text-zinc-500">
                    No matching resources found.
                  </td>
                </tr>
              ) : (
                filtered.map((item) => (
                  <tr
                    key={item.id}
                    onClick={() => handleOpenResource(item)}
                    className="hover:bg-zinc-900/60 cursor-pointer transition"
                  >
                    <td className="px-3.5 py-3">
                      <div className="flex items-center gap-2">
                        <div className="w-7 h-7 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-400">
                          {getIcon(item.type)}
                        </div>
                        <div>
                          <div className="font-medium text-white text-xs">{item.name}</div>
                          <div className="text-[10px] text-zinc-400 font-mono truncate max-w-[140px]">{item.id}</div>
                        </div>
                      </div>
                    </td>

                    <td className="px-3.5 py-3 text-zinc-300 font-medium">{getTypeLabel(item.type)}</td>

                    <td className="px-3.5 py-3">{getStatusBadge(item.status)}</td>

                    <td className="px-3.5 py-3 font-mono text-[11px] text-zinc-300">
                      {item.endpoint ? (
                        <span className="truncate max-w-[200px] block text-zinc-200 hover:underline">{item.endpoint}</span>
                      ) : (
                        <span className="text-zinc-600">-</span>
                      )}
                    </td>

                    <td className="px-3.5 py-3 text-zinc-400 text-[11px]">{item.specs || '-'}</td>

                    <td className="px-3.5 py-3 text-zinc-400 font-mono text-[10px]">{item.created_at}</td>

                    <td className="px-3.5 py-3 text-right">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOpenResource(item);
                        }}
                        className="p-1 rounded bg-zinc-850 hover:bg-zinc-800 text-zinc-400 hover:text-white transition"
                        title="Manage Resource"
                      >
                        <ChevronRight className="w-3.5 h-3.5" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
