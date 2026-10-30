import React, { useState, useEffect } from 'react';
import { apiRequest } from '../../lib/api';
import { StatsCard } from '../../components/ui/StatsCard';
import { ProgressBar } from '../../components/ui/ProgressBar';
import {
  Shield,
  Users,
  Server,
  Cpu,
  HardDrive,
  RefreshCw,
  AlertTriangle,
  Activity,
  CheckCircle2,
} from 'lucide-react';

export const AdminOverview: React.FC = () => {
  const [data, setData] = useState<any>(null);
  const [loading, setLoading] = useState(true);

  const fetchOverview = async () => {
    try {
      const res = await apiRequest<any>('/admin/overview');
      setData(res);
    } catch (err) {
      console.error('Failed to load admin overview:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchOverview();
    const interval = setInterval(fetchOverview, 6000);
    return () => clearInterval(interval);
  }, []);

  if (loading && !data) {
    return (
      <div className="flex items-center justify-center py-20 text-slate-400">
        <RefreshCw className="w-6 h-6 animate-spin text-amber-500 mr-2" />
        <span>Loading cluster telemetry...</span>
      </div>
    );
  }

  const nodeStatus = data?.node_status;
  const storage = data?.storage?.[0];

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight flex items-center gap-2">
            <span>Infrastructure Administrator Overview</span>
            <span className="text-xs px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20 font-mono">
              Cluster Node: {data?.node?.proxmox_node_name || 'pve-dell-r640'}
            </span>
          </h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-0.5">
            Real-time physical capacity, user allocations, and hypervisor health
          </p>
        </div>
      </div>

      {/* Cluster Overview Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatsCard
          title="Total Users"
          value={data?.total_users || 0}
          subtext="Registered accounts"
          icon={<Users className="w-5 h-5 text-blue-400" />}
        />
        <StatsCard
          title="Total VMs"
          value={data?.total_instances || 0}
          subtext={`${data?.running_instances || 0} running • ${data?.stopped_instances || 0} stopped`}
          icon={<Server className="w-5 h-5 text-emerald-400" />}
        />
        <StatsCard
          title="Active Jobs"
          value={data?.active_jobs || 0}
          subtext="Queue in flight"
          icon={<RefreshCw className="w-5 h-5 text-amber-400" />}
        />
        <StatsCard
          title="Failed Jobs"
          value={data?.failed_jobs || 0}
          subtext="Requires inspection"
          icon={<AlertTriangle className="w-5 h-5 text-rose-400" />}
        />
      </div>

      {/* Physical Hypervisor Telemetry (Dell PowerEdge R640) */}
      <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm space-y-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-slate-800 pb-4">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-slate-800 flex items-center justify-center text-amber-400">
              <Cpu className="w-5 h-5" />
            </div>
            <div>
              <h2 className="text-sm font-bold text-white">Dell PowerEdge R640 — Physical Node</h2>
              <p className="text-xs text-slate-500 font-mono">
                {nodeStatus?.pveversion || 'Proxmox VE 8.2'} • Kernel: {nodeStatus?.kversion || 'Linux 6.8.12'}
              </p>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse" />
            <span className="text-xs text-emerald-400 font-semibold font-mono">NODE ONLINE</span>
          </div>
        </div>

        {nodeStatus && (
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-3">
              <div className="flex items-center justify-between text-xs">
                <span className="text-slate-400 font-medium">CPU Load</span>
                <span className="text-blue-400 font-bold font-mono">
                  {((nodeStatus.cpu || 0) * 100).toFixed(1)}%
                </span>
              </div>
              <div className="h-2 w-full bg-slate-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-blue-500 rounded-full"
                  style={{ width: `${Math.min(100, (nodeStatus.cpu || 0) * 100)}%` }}
                />
              </div>
              <p className="text-[11px] text-slate-500">64 Hardware Threads available</p>
            </div>

            <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-3">
              <div className="flex items-center justify-between text-xs">
                <span className="text-slate-400 font-medium">Memory (RAM)</span>
                <span className="text-purple-400 font-bold font-mono">
                  {Math.round((nodeStatus.memory?.used || 0) / (1024 * 1024 * 1024))} /{' '}
                  {Math.round((nodeStatus.memory?.total || 1) / (1024 * 1024 * 1024))} GB
                </span>
              </div>
              <div className="h-2 w-full bg-slate-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-purple-500 rounded-full"
                  style={{
                    width: `${Math.min(
                      100,
                      ((nodeStatus.memory?.used || 0) / (nodeStatus.memory?.total || 1)) * 100
                    )}%`,
                  }}
                />
              </div>
              <p className="text-[11px] text-slate-500">256 GB ECC DDR4 Server Memory</p>
            </div>

            <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4 space-y-3">
              <div className="flex items-center justify-between text-xs">
                <span className="text-slate-400 font-medium">Storage Pool ({storage?.storage || 'local-zfs'})</span>
                <span className="text-emerald-400 font-bold font-mono">
                  {Math.round((storage?.used || 0) / (1024 * 1024 * 1024 * 1024))} /{' '}
                  {Math.round((storage?.total || 1) / (1024 * 1024 * 1024 * 1024))} TB
                </span>
              </div>
              <div className="h-2 w-full bg-slate-800 rounded-full overflow-hidden">
                <div
                  className="h-full bg-emerald-500 rounded-full"
                  style={{
                    width: `${Math.min(
                      100,
                      ((storage?.used || 0) / (storage?.total || 1)) * 100
                    )}%`,
                  }}
                />
              </div>
              <p className="text-[11px] text-slate-500">ZFS Enterprise NVMe / SAS Storage</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};
