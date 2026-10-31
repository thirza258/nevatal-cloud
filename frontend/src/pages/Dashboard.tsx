import React, { useState, useEffect } from 'react';
import { useAuth } from '../context/AuthContext';
import { DashboardSummary, Instance } from '../types';
import { apiRequest } from '../lib/api';
import { StatsCard } from '../components/ui/StatsCard';
import { ProgressBar } from '../components/ui/ProgressBar';
import { StatusBadge } from '../components/ui/Badge';
import {
  Server,
  Activity,
  PlusCircle,
  Cpu,
  Layers,
  ArrowRight,
  RefreshCw,
  HardDrive,
  Clock,
  Play,
  Square,
  RotateCcw,
  Sparkles,
  Globe,
  Workflow,
  Bot,
  Database,
  Zap,
} from 'lucide-react';

interface DashboardProps {
  onNavigate: (path: string, param?: string) => void;
}

export const Dashboard: React.FC<DashboardProps> = ({ onNavigate }) => {
  const { user } = useAuth();
  const [data, setData] = useState<DashboardSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [aiPrompt, setAiPrompt] = useState('');

  const fetchSummary = async (showRefreshing = false) => {
    if (showRefreshing) setRefreshing(true);
    try {
      const res = await apiRequest<DashboardSummary>('/dashboard');
      setData(res);
    } catch (err) {
      console.error('Failed to load dashboard:', err);
    } finally {
      setLoading(false);
      setRefreshing(false);
    }
  };

  useEffect(() => {
    fetchSummary();
    const interval = setInterval(() => fetchSummary(false), 8000);
    return () => clearInterval(interval);
  }, []);

  const handleAction = async (e: React.MouseEvent, instanceId: string, action: string) => {
    e.stopPropagation();
    try {
      await apiRequest(`/instances/${instanceId}/${action}`, { method: 'POST' });
      fetchSummary(false);
    } catch (err: any) {
      alert(err.message || `Failed to ${action} instance`);
    }
  };

  if (loading && !data) {
    return (
      <div className="flex items-center justify-center py-20 text-slate-400">
        <RefreshCw className="w-6 h-6 animate-spin text-blue-500 mr-2" />
        <span>Loading cloud control plane...</span>
      </div>
    );
  }

  const usage = data?.usage;

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-zinc-800 pb-5">
        <div>
          <h1 className="text-xl sm:text-2xl font-semibold text-white tracking-tight">
            Overview
          </h1>
          <p className="text-xs text-zinc-400 mt-1 font-mono">
            Proxmox VE virtualization cluster operational · Dell PowerEdge R640
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => fetchSummary(true)}
            disabled={refreshing}
            className="p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 hover:text-white border border-zinc-800 transition"
            title="Refresh status"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => onNavigate('/dashboard/instances/new')}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 font-medium text-xs transition"
          >
            <PlusCircle className="w-3.5 h-3.5" />
            <span>Deploy Instance</span>
          </button>
        </div>
      </div>

      {/* Command & Quick Actions Bar */}
      <div className="p-3.5 rounded-lg bg-zinc-900/40 border border-zinc-800 space-y-3">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-2">
            <Sparkles className="w-3.5 h-3.5 text-zinc-400" />
            <span className="text-xs font-medium text-zinc-200">Nevatal AI Cloud Agent</span>
          </div>
          <button
            onClick={() => onNavigate('/dashboard/ai')}
            className="text-[11px] text-zinc-400 hover:text-white flex items-center gap-1 transition"
          >
            <span>Agent Console</span>
            <ArrowRight className="w-3 h-3" />
          </button>
        </div>

        {/* Natural Language Prompt Bar */}
        <form
          onSubmit={(e) => {
            e.preventDefault();
            onNavigate('/dashboard/ai');
          }}
          className="flex items-center gap-2"
        >
          <input
            type="text"
            value={aiPrompt}
            onChange={(e) => setAiPrompt(e.target.value)}
            placeholder="Plan or provision infrastructure (e.g. 'Deploy Ubuntu 24.04 with 2 vCPUs' or 'Create WordPress')..."
            className="flex-1 bg-zinc-950 border border-zinc-800 rounded-md px-3 py-2 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-600"
          />
          <button
            type="submit"
            className="px-3 py-2 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 font-medium text-xs transition flex items-center gap-1.5 flex-shrink-0"
          >
            <span>Ask Agent</span>
            <ArrowRight className="w-3.5 h-3.5" />
          </button>
        </form>

        {/* Quick Builder Presets */}
        <div className="flex items-center gap-1.5 overflow-x-auto text-[11px] pt-0.5">
          <span className="text-[10px] font-mono uppercase text-zinc-500 mr-1">Quick:</span>
          <button
            onClick={() => onNavigate('/dashboard/instances/new')}
            className="flex items-center gap-1 px-2.5 py-1 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-800 transition"
          >
            <Server className="w-3 h-3 text-zinc-400" />
            <span>VPS</span>
          </button>
          <button
            onClick={() => onNavigate('/dashboard/apps/new')}
            className="flex items-center gap-1 px-2.5 py-1 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-800 transition"
          >
            <Globe className="w-3 h-3 text-zinc-400" />
            <span>App</span>
          </button>
          <button
            onClick={() => onNavigate('/dashboard/automation/new')}
            className="flex items-center gap-1 px-2.5 py-1 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-800 transition"
          >
            <Workflow className="w-3 h-3 text-zinc-400" />
            <span>n8n</span>
          </button>
          <button
            onClick={() => onNavigate('/dashboard/game-servers/create')}
            className="flex items-center gap-1 px-2.5 py-1 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-800 transition"
          >
            <Server className="w-3 h-3 text-zinc-400" />
            <span>Game Server</span>
          </button>
          <button
            onClick={() => onNavigate('/dashboard/android/create')}
            className="flex items-center gap-1 px-2.5 py-1 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-800 transition"
          >
            <Server className="w-3 h-3 text-zinc-400" />
            <span>Android Cloud</span>
          </button>
          <button
            onClick={() => onNavigate('/dashboard/storage')}
            className="flex items-center gap-1 px-2.5 py-1 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-800 transition"
          >
            <Database className="w-3 h-3 text-zinc-400" />
            <span>Storage</span>
          </button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatsCard
          title="Total Instances"
          value={data?.instances.total || 0}
          subtext="Allocated virtual machines"
          icon={<Server className="w-5 h-5 text-blue-400" />}
        />
        <StatsCard
          title="Running"
          value={data?.instances.running || 0}
          subtext="Active & accepting traffic"
          icon={<Cpu className="w-5 h-5 text-emerald-400" />}
          trend="Online"
          trendPositive={true}
        />
        <StatsCard
          title="Stopped"
          value={data?.instances.stopped || 0}
          subtext="Powered down"
          icon={<Layers className="w-5 h-5 text-slate-400" />}
        />
        <StatsCard
          title="Provisioning"
          value={data?.instances.provisioning || 0}
          subtext="Jobs currently in flight"
          icon={<RefreshCw className="w-5 h-5 text-amber-400" />}
        />
      </div>

      {/* Quotas & Capacity Overview */}
      {usage && (
        <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-4">
          <div className="flex items-center justify-between mb-3.5">
            <div>
              <h3 className="text-xs font-mono uppercase tracking-wider text-zinc-400">
                Account Quota Utilization
              </h3>
            </div>
            <button
              onClick={() => onNavigate('/dashboard/usage')}
              className="text-xs text-zinc-400 hover:text-white flex items-center gap-1 transition"
            >
              <span>Quota Details</span>
              <ArrowRight className="w-3 h-3" />
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-5">
            <ProgressBar
              label="vCPU Cores"
              used={usage.used_vcpus}
              total={usage.max_vcpus}
              unit="vCPUs"
            />
            <ProgressBar
              label="RAM Memory"
              used={Math.round(usage.used_memory_mb / 1024)}
              total={Math.round(usage.max_memory_mb / 1024)}
              unit="GB"
            />
            <ProgressBar
              label="SSD Storage"
              used={usage.used_storage_gb}
              total={usage.max_storage_gb}
              unit="GB"
            />
            <ProgressBar
              label="Instances"
              used={usage.used_instances}
              total={usage.max_instances}
              unit="VMs"
            />
          </div>
        </div>
      )}

      {/* Recent Instances & Activity Split */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Recent Instances Table */}
        <div className="lg:col-span-2 bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
          <div className="p-3.5 border-b border-zinc-800 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Server className="w-3.5 h-3.5 text-zinc-400" />
              <h3 className="text-xs font-medium text-white">Recent Instances</h3>
            </div>
            <button
              onClick={() => onNavigate('/dashboard/instances')}
              className="text-xs text-zinc-400 hover:text-white flex items-center gap-1 transition"
            >
              <span>View All</span>
              <ArrowRight className="w-3 h-3" />
            </button>
          </div>

          <div className="overflow-x-auto">
            {!data?.recent_instances || data.recent_instances.length === 0 ? (
              <div className="p-8 text-center text-zinc-500 space-y-2">
                <Server className="w-8 h-8 text-zinc-600 mx-auto" />
                <p className="text-xs">No instances deployed yet.</p>
                <button
                  onClick={() => onNavigate('/dashboard/instances/new')}
                  className="inline-flex items-center gap-1 text-xs text-white hover:underline font-medium"
                >
                  Deploy your first Linux VM
                </button>
              </div>
            ) : (
              <table className="w-full text-left border-collapse text-xs">
                <thead>
                  <tr className="border-b border-zinc-800 bg-zinc-950 text-zinc-400 font-mono text-[10px] uppercase tracking-wider">
                    <th className="py-2.5 px-3.5">Name</th>
                    <th className="py-2.5 px-3.5">Status</th>
                    <th className="py-2.5 px-3.5">Specs</th>
                    <th className="py-2.5 px-3.5">IP Address</th>
                    <th className="py-2.5 px-3.5 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-850">
                  {data.recent_instances.map((inst) => (
                    <tr
                      key={inst.id}
                      onClick={() => onNavigate('/dashboard/instances', inst.id)}
                      className="hover:bg-zinc-900/70 cursor-pointer transition-colors"
                    >
                      <td className="py-3 px-3.5">
                        <div className="font-medium text-white">{inst.name}</div>
                        <div className="text-[10px] text-zinc-400 font-mono">
                          {inst.os_template?.distribution} {inst.os_template?.version}
                        </div>
                      </td>
                      <td className="py-3 px-3.5">
                        <StatusBadge status={inst.status} size="sm" />
                      </td>
                      <td className="py-3 px-3.5 font-mono text-zinc-300 text-[11px]">
                        {inst.vcpus} vCPU • {Math.round(inst.memory_mb / 1024)}GB • {inst.disk_gb}GB
                      </td>
                      <td className="py-3 px-3.5 font-mono text-zinc-300 text-[11px]">
                        {inst.private_ip || <span className="text-zinc-600">—</span>}
                      </td>
                      <td className="py-3 px-3.5 text-right">
                        <div className="inline-flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                          {inst.status === 'stopped' && (
                            <button
                              onClick={(e) => handleAction(e, inst.id, 'start')}
                              className="p-1.5 rounded bg-zinc-850 hover:bg-zinc-800 text-emerald-400 border border-zinc-750 transition"
                              title="Start VM"
                            >
                              <Play className="w-3 h-3" />
                            </button>
                          )}
                          {inst.status === 'running' && (
                            <>
                              <button
                                onClick={(e) => handleAction(e, inst.id, 'shutdown')}
                                className="p-1.5 rounded bg-zinc-850 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-750 transition"
                                title="Graceful Shutdown"
                              >
                                <Square className="w-3 h-3" />
                              </button>
                              <button
                                onClick={(e) => handleAction(e, inst.id, 'restart')}
                                className="p-1.5 rounded bg-zinc-850 hover:bg-zinc-800 text-zinc-300 hover:text-white border border-zinc-750 transition"
                                title="Restart VM"
                              >
                                <RotateCcw className="w-3 h-3" />
                              </button>
                            </>
                          )}
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        {/* Recent Activity Feed */}
        <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-3.5 space-y-3">
          <div className="flex items-center justify-between pb-2.5 border-b border-zinc-800">
            <div className="flex items-center gap-2">
              <Activity className="w-3.5 h-3.5 text-zinc-400" />
              <h3 className="text-xs font-medium text-white">Recent Activity</h3>
            </div>
            <button
              onClick={() => onNavigate('/dashboard/activity')}
              className="text-[11px] text-zinc-400 hover:text-white"
            >
              Full Log
            </button>
          </div>

          <div className="space-y-2.5">
            {!data?.recent_activity || data.recent_activity.length === 0 ? (
              <p className="text-xs text-zinc-500 text-center py-6">No recent events recorded.</p>
            ) : (
              data.recent_activity.slice(0, 6).map((item) => (
                <div key={item.id} className="flex items-start gap-2 text-xs">
                  <div className="w-1.5 h-1.5 rounded-full bg-zinc-500 mt-1.5 flex-shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-zinc-200 font-mono text-[11px] truncate">
                      {item.action}
                    </p>
                    <span className="text-[10px] text-zinc-400 font-mono">
                      {new Date(item.created_at).toLocaleTimeString([], {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                    </span>
                  </div>
                  <span
                    className={`text-[10px] px-1.5 py-0.5 rounded font-mono border ${
                      item.result === 'success'
                        ? 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20'
                        : 'text-rose-400 bg-rose-500/10 border-rose-500/20'
                    }`}
                  >
                    {item.result}
                  </span>
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  );
};
