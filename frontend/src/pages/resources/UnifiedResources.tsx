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
  }>({ total: 0, vps: 0, apps: 0, automations: 0, ai_agents: 0, buckets: 0 });
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
      default:
        return type;
    }
  };

  const getStatusBadge = (status: string) => {
    const s = status.toLowerCase();
    if (s === 'running' || s === 'active' || s === 'completed') {
      return (
        <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center gap-1">
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
          <span className="capitalize">{status}</span>
        </span>
      );
    }
    if (s === 'provisioning' || s === 'starting' || s === 'queued') {
      return (
        <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20 flex items-center gap-1">
          <RefreshCw className="w-3 h-3 animate-spin" />
          <span className="capitalize">{status}</span>
        </span>
      );
    }
    if (s === 'stopped') {
      return (
        <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-slate-500/10 text-slate-400 border border-slate-500/20">
          Stopped
        </span>
      );
    }
    return (
      <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-rose-500/10 text-rose-400 border border-rose-500/20 capitalize">
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
      default:
        break;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-900/60 p-6 rounded-2xl border border-slate-800 backdrop-blur">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">Cloud Resources</h1>
            <span className="px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 text-xs font-semibold">
              Unified Inventory
            </span>
          </div>
          <p className="text-xs sm:text-sm text-slate-400 mt-1">
            Aggregate overview of virtual machines, applications, automations, and storage provisioned by Nevatal AI Cloud Agent.
          </p>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={loadResources}
            disabled={loading}
            className="p-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-700/60 transition"
            title="Refresh Resources"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => onNavigate('/dashboard/ai')}
            className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs shadow-lg shadow-blue-500/25 transition"
          >
            <Sparkles className="w-3.5 h-3.5" />
            <span>Ask AI Agent to Build</span>
          </button>
        </div>
      </div>

      {/* Metrics Row */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800">
          <div className="text-[10px] uppercase font-bold text-slate-500">Total Resources</div>
          <div className="text-xl font-bold text-white mt-1">{counts.total}</div>
        </div>
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800">
          <div className="text-[10px] uppercase font-bold text-blue-400">Compute VPS</div>
          <div className="text-xl font-bold text-white mt-1">{counts.vps}</div>
        </div>
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800">
          <div className="text-[10px] uppercase font-bold text-emerald-400">Web Apps</div>
          <div className="text-xl font-bold text-white mt-1">{counts.apps}</div>
        </div>
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800">
          <div className="text-[10px] uppercase font-bold text-amber-400">n8n Automations</div>
          <div className="text-xl font-bold text-white mt-1">{counts.automations}</div>
        </div>
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800">
          <div className="text-[10px] uppercase font-bold text-purple-400">AI Agents</div>
          <div className="text-xl font-bold text-white mt-1">{counts.ai_agents}</div>
        </div>
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800">
          <div className="text-[10px] uppercase font-bold text-cyan-400">S3 Buckets</div>
          <div className="text-xl font-bold text-white mt-1">{counts.buckets}</div>
        </div>
      </div>

      {/* Search & Filter Bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        {/* Type Filter Tabs */}
        <div className="flex items-center gap-1.5 overflow-x-auto pb-1 sm:pb-0">
          {[
            { id: 'all', label: 'All Resources' },
            { id: 'vps', label: 'VPS', icon: Server },
            { id: 'app', label: 'Apps', icon: Globe },
            { id: 'automation', label: 'n8n', icon: Workflow },
            { id: 'ai_agent', label: 'OpenClaw', icon: Bot },
            { id: 'bucket', label: 'S3 Buckets', icon: Database },
          ].map((tab) => {
            const active = filterType === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setFilterType(tab.id)}
                className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium whitespace-nowrap transition ${
                  active
                    ? 'bg-blue-600 text-white shadow-sm'
                    : 'bg-slate-900/80 hover:bg-slate-800 text-slate-400 hover:text-white border border-slate-800'
                }`}
              >
                {tab.icon && <tab.icon className="w-3.5 h-3.5" />}
                <span>{tab.label}</span>
              </button>
            );
          })}
        </div>

        {/* Search */}
        <div className="relative min-w-[240px]">
          <Search className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            placeholder="Search by name, IP, or endpoint..."
            className="w-full bg-slate-900/80 border border-slate-800 rounded-xl pl-9 pr-4 py-2 text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>

      {/* Resources Table */}
      <div className="bg-slate-900/60 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-950/60 text-slate-400 uppercase text-[10px] tracking-wider border-b border-slate-800">
              <tr>
                <th className="px-5 py-3.5">Resource</th>
                <th className="px-4 py-3.5">Type</th>
                <th className="px-4 py-3.5">Status</th>
                <th className="px-4 py-3.5">Endpoint / Access</th>
                <th className="px-4 py-3.5">Details</th>
                <th className="px-4 py-3.5">Created</th>
                <th className="px-5 py-3.5 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-slate-200">
              {filtered.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-5 py-12 text-center text-slate-500">
                    No matching resources found. Use the AI Cloud Agent to spin up your first service!
                  </td>
                </tr>
              ) : (
                filtered.map((item) => (
                  <tr
                    key={item.id}
                    onClick={() => handleOpenResource(item)}
                    className="hover:bg-slate-800/40 cursor-pointer transition"
                  >
                    <td className="px-5 py-3.5">
                      <div className="flex items-center gap-2.5">
                        <div className="w-8 h-8 rounded-lg bg-slate-800/80 border border-slate-700/60 flex items-center justify-center">
                          {getIcon(item.type)}
                        </div>
                        <div>
                          <div className="font-bold text-white text-xs">{item.name}</div>
                          <div className="text-[10px] text-slate-400 font-mono truncate max-w-[140px]">{item.id}</div>
                        </div>
                      </div>
                    </td>

                    <td className="px-4 py-3.5 text-slate-300 font-medium">{getTypeLabel(item.type)}</td>

                    <td className="px-4 py-3.5">{getStatusBadge(item.status)}</td>

                    <td className="px-4 py-3.5 font-mono text-[11px] text-slate-300">
                      {item.endpoint ? (
                        <span className="truncate max-w-[200px] block text-blue-400">{item.endpoint}</span>
                      ) : (
                        <span className="text-slate-500">-</span>
                      )}
                    </td>

                    <td className="px-4 py-3.5 text-slate-400">{item.specs || '-'}</td>

                    <td className="px-4 py-3.5 text-slate-500 font-mono text-[11px]">{item.created_at}</td>

                    <td className="px-5 py-3.5 text-right">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOpenResource(item);
                        }}
                        className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition"
                        title="Manage Resource"
                      >
                        <ChevronRight className="w-4 h-4" />
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
