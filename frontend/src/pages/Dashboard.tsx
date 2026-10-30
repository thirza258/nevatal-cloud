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
} from 'lucide-react';

interface DashboardProps {
  onNavigate: (path: string, param?: string) => void;
}

export const Dashboard: React.FC<DashboardProps> = ({ onNavigate }) => {
  const { user } = useAuth();
  const [data, setData] = useState<DashboardSummary | null>(null);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);

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
      {/* Welcome Banner */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 bg-gradient-to-r from-slate-900 via-slate-900 to-blue-950/30 p-6 rounded-2xl border border-slate-800 shadow-sm">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">
            Welcome back, {user?.display_name}!
          </h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-1">
            Proxmox VE virtualization cluster is operational. Manage and deploy your virtual servers.
          </p>
        </div>
        <div className="flex items-center gap-3">
          <button
            onClick={() => fetchSummary(true)}
            disabled={refreshing}
            className="p-2.5 rounded-xl bg-slate-800 hover:bg-slate-700/80 text-slate-300 hover:text-white border border-slate-700/60 transition"
            title="Refresh status"
          >
            <RefreshCw className={`w-4 h-4 ${refreshing ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => onNavigate('/dashboard/instances/new')}
            className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm shadow-lg shadow-blue-500/25 transition-all"
          >
            <PlusCircle className="w-4 h-4" />
            <span>Deploy Instance</span>
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
        <div className="bg-slate-900/80 border border-slate-800 rounded-2xl p-6 shadow-sm">
          <div className="flex items-center justify-between mb-4">
            <div>
              <h3 className="text-sm font-bold text-white tracking-tight flex items-center gap-2">
                <span>Account Quota Utilization</span>
                <span className="text-xs font-normal text-slate-400">
                  (Enforced per PRD limits)
                </span>
              </h3>
            </div>
            <button
              onClick={() => onNavigate('/dashboard/usage')}
              className="text-xs text-blue-400 hover:text-blue-300 font-medium flex items-center gap-1"
            >
              <span>Quota Details</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
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
        <div className="lg:col-span-2 bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
          <div className="p-5 border-b border-slate-800 flex items-center justify-between">
            <div className="flex items-center gap-2">
              <Server className="w-4 h-4 text-blue-400" />
              <h3 className="text-sm font-semibold text-white">Recent Instances</h3>
            </div>
            <button
              onClick={() => onNavigate('/dashboard/instances')}
              className="text-xs text-blue-400 hover:text-blue-300 font-medium flex items-center gap-1"
            >
              <span>View All</span>
              <ArrowRight className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="overflow-x-auto">
            {!data?.recent_instances || data.recent_instances.length === 0 ? (
              <div className="p-10 text-center text-slate-400 space-y-3">
                <Server className="w-10 h-10 text-slate-600 mx-auto" />
                <p className="text-sm">No instances deployed yet.</p>
                <button
                  onClick={() => onNavigate('/dashboard/instances/new')}
                  className="inline-flex items-center gap-1 text-xs text-blue-400 font-medium hover:underline"
                >
                  Deploy your first Linux VM
                </button>
              </div>
            ) : (
              <table className="w-full text-left border-collapse text-xs">
                <thead>
                  <tr className="border-b border-slate-800 bg-slate-950/40 text-slate-400 font-medium uppercase tracking-wider">
                    <th className="py-3 px-4">Name</th>
                    <th className="py-3 px-4">Status</th>
                    <th className="py-3 px-4">Specs</th>
                    <th className="py-3 px-4">IP Address</th>
                    <th className="py-3 px-4 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60">
                  {data.recent_instances.map((inst) => (
                    <tr
                      key={inst.id}
                      onClick={() => onNavigate('/dashboard/instances', inst.id)}
                      className="hover:bg-slate-800/40 cursor-pointer transition-colors"
                    >
                      <td className="py-3.5 px-4">
                        <div className="font-semibold text-white">{inst.name}</div>
                        <div className="text-[11px] text-slate-500 font-mono">
                          {inst.os_template?.distribution} {inst.os_template?.version}
                        </div>
                      </td>
                      <td className="py-3.5 px-4">
                        <StatusBadge status={inst.status} size="sm" />
                      </td>
                      <td className="py-3.5 px-4 font-mono text-slate-300">
                        {inst.vcpus} vCPU • {Math.round(inst.memory_mb / 1024)}GB • {inst.disk_gb}GB
                      </td>
                      <td className="py-3.5 px-4 font-mono text-slate-300">
                        {inst.private_ip || <span className="text-slate-500">—</span>}
                      </td>
                      <td className="py-3.5 px-4 text-right">
                        <div className="inline-flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                          {inst.status === 'stopped' && (
                            <button
                              onClick={(e) => handleAction(e, inst.id, 'start')}
                              className="p-1.5 rounded-lg bg-emerald-950/60 text-emerald-400 hover:bg-emerald-900 border border-emerald-800/60 transition"
                              title="Start VM"
                            >
                              <Play className="w-3.5 h-3.5" />
                            </button>
                          )}
                          {inst.status === 'running' && (
                            <>
                              <button
                                onClick={(e) => handleAction(e, inst.id, 'shutdown')}
                                className="p-1.5 rounded-lg bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 transition"
                                title="Graceful Shutdown"
                              >
                                <Square className="w-3.5 h-3.5" />
                              </button>
                              <button
                                onClick={(e) => handleAction(e, inst.id, 'restart')}
                                className="p-1.5 rounded-lg bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 transition"
                                title="Restart VM"
                              >
                                <RotateCcw className="w-3.5 h-3.5" />
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
        <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm space-y-4">
          <div className="flex items-center justify-between pb-3 border-b border-slate-800">
            <div className="flex items-center gap-2">
              <Activity className="w-4 h-4 text-blue-400" />
              <h3 className="text-sm font-semibold text-white">Recent Activity</h3>
            </div>
            <button
              onClick={() => onNavigate('/dashboard/activity')}
              className="text-xs text-blue-400 hover:text-blue-300 font-medium"
            >
              Full Log
            </button>
          </div>

          <div className="space-y-3">
            {!data?.recent_activity || data.recent_activity.length === 0 ? (
              <p className="text-xs text-slate-500 text-center py-6">No recent events recorded.</p>
            ) : (
              data.recent_activity.slice(0, 6).map((item) => (
                <div key={item.id} className="flex items-start gap-2.5 text-xs">
                  <div className="w-1.5 h-1.5 rounded-full bg-blue-500 mt-1.5 flex-shrink-0" />
                  <div className="flex-1 min-w-0">
                    <p className="text-slate-200 font-medium truncate">
                      <span className="font-mono text-blue-400 mr-1">{item.action}</span>
                    </p>
                    <span className="text-[10px] text-slate-500">
                      {new Date(item.created_at).toLocaleTimeString([], {
                        hour: '2-digit',
                        minute: '2-digit',
                        second: '2-digit',
                      })}
                    </span>
                  </div>
                  <span
                    className={`text-[10px] px-1.5 py-0.5 rounded font-mono ${
                      item.result === 'success'
                        ? 'text-emerald-400 bg-emerald-950/40 border border-emerald-800/40'
                        : 'text-rose-400 bg-rose-950/40 border border-rose-800/40'
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
