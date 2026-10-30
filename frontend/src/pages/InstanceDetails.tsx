import React, { useState, useEffect } from 'react';
import { Instance, InstanceMetrics, AuditLog } from '../types';
import { apiRequest } from '../lib/api';
import { StatusBadge } from '../components/ui/Badge';
import { Modal } from '../components/ui/Modal';
import {
  Server,
  ArrowLeft,
  Copy,
  Check,
  Play,
  Square,
  RotateCcw,
  Trash2,
  Cpu,
  HardDrive,
  Activity,
  Terminal,
  Shield,
  Clock,
  AlertTriangle,
  RefreshCw,
} from 'lucide-react';

interface InstanceDetailsProps {
  instanceId: string;
  onNavigate: (path: string, param?: string) => void;
}

export const InstanceDetails: React.FC<InstanceDetailsProps> = ({ instanceId, onNavigate }) => {
  const [instance, setInstance] = useState<Instance | null>(null);
  const [metrics, setMetrics] = useState<InstanceMetrics | null>(null);
  const [activity, setActivity] = useState<AuditLog[]>([]);
  const [loading, setLoading] = useState(true);
  const [copied, setCopied] = useState(false);
  const [actionLoading, setActionLoading] = useState(false);

  // Deletion modal
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [confirmName, setConfirmName] = useState('');
  const [deleting, setDeleting] = useState(false);

  const fetchInstanceData = async () => {
    try {
      const instData = await apiRequest<Instance>(`/instances/${instanceId}`);
      setInstance(instData);

      // Fetch live metrics
      const metricData = await apiRequest<InstanceMetrics>(`/instances/${instanceId}/metrics`).catch(
        () => null
      );
      if (metricData) setMetrics(metricData);

      // Fetch activity
      const activityData = await apiRequest<AuditLog[]>(`/instances/${instanceId}/activity`).catch(
        () => []
      );
      setActivity(activityData || []);
    } catch (err) {
      console.error('Failed to load instance:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchInstanceData();
    const interval = setInterval(fetchInstanceData, 5000);
    return () => clearInterval(interval);
  }, [instanceId]);

  const handleAction = async (action: string) => {
    setActionLoading(true);
    try {
      await apiRequest(`/instances/${instanceId}/${action}`, { method: 'POST' });
      await fetchInstanceData();
    } catch (err: any) {
      alert(err.message || `Failed to ${action} instance`);
    } finally {
      setActionLoading(false);
    }
  };

  const handleDelete = async () => {
    setDeleting(true);
    try {
      await apiRequest(`/instances/${instanceId}`, { method: 'DELETE' });
      onNavigate('/dashboard/instances');
    } catch (err: any) {
      alert(err.message || 'Failed to delete instance');
      setDeleting(false);
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (loading && !instance) {
    return (
      <div className="flex items-center justify-center py-20 text-slate-400">
        <RefreshCw className="w-6 h-6 animate-spin text-blue-500 mr-2" />
        <span>Loading instance details...</span>
      </div>
    );
  }

  if (!instance) {
    return (
      <div className="p-12 text-center text-slate-400 space-y-4">
        <AlertTriangle className="w-12 h-12 text-amber-500 mx-auto" />
        <h3 className="text-lg font-semibold text-white">Instance not found</h3>
        <p className="text-sm text-slate-500">The requested instance does not exist or has been removed.</p>
        <button
          onClick={() => onNavigate('/dashboard/instances')}
          className="px-4 py-2 rounded-xl bg-blue-600 text-xs font-semibold text-white hover:bg-blue-500"
        >
          Return to instances
        </button>
      </div>
    );
  }

  const sshCommand = instance.private_ip
    ? `ssh ${instance.ssh_username}@${instance.private_ip}`
    : `ssh ${instance.ssh_username}@<pending-ip>`;

  return (
    <div className="space-y-6">
      {/* Top back button & Actions Header */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <button
            onClick={() => onNavigate('/dashboard/instances')}
            className="p-2 rounded-xl bg-slate-900 border border-slate-800 text-slate-400 hover:text-white hover:bg-slate-800 transition"
          >
            <ArrowLeft className="w-4 h-4" />
          </button>
          <div>
            <div className="flex items-center gap-3">
              <h1 className="text-2xl font-bold text-white tracking-tight">{instance.name}</h1>
              <StatusBadge status={instance.status} />
            </div>
            <p className="text-xs text-slate-400 font-mono mt-0.5">
              ID: {instance.id} • Hostname: {instance.hostname}
            </p>
          </div>
        </div>

        {/* Lifecycle Action Buttons */}
        <div className="flex items-center gap-2">
          {instance.status === 'stopped' && (
            <button
              onClick={() => handleAction('start')}
              disabled={actionLoading}
              className="flex items-center gap-1.5 px-3 py-2 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold shadow-sm transition disabled:opacity-50"
            >
              <Play className="w-3.5 h-3.5" />
              <span>Start</span>
            </button>
          )}

          {instance.status === 'running' && (
            <>
              <button
                onClick={() => handleAction('shutdown')}
                disabled={actionLoading}
                className="flex items-center gap-1.5 px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700/80 transition disabled:opacity-50"
              >
                <Square className="w-3.5 h-3.5" />
                <span>Shutdown</span>
              </button>
              <button
                onClick={() => handleAction('restart')}
                disabled={actionLoading}
                className="flex items-center gap-1.5 px-3 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-semibold border border-slate-700/80 transition disabled:opacity-50"
              >
                <RotateCcw className="w-3.5 h-3.5" />
                <span>Restart</span>
              </button>
            </>
          )}

          <button
            onClick={() => setShowDeleteModal(true)}
            className="flex items-center gap-1.5 px-3 py-2 rounded-xl bg-rose-950/60 hover:bg-rose-900/80 text-rose-300 border border-rose-800/80 text-xs font-semibold transition"
          >
            <Trash2 className="w-3.5 h-3.5" />
            <span>Destroy</span>
          </button>
        </div>
      </div>

      {/* Main Grid: Overview & SSH Connection */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left Column: Specs & Info */}
        <div className="lg:col-span-2 space-y-6">
          {/* Overview Cards */}
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm">
            <h3 className="text-sm font-bold text-white mb-4 flex items-center gap-2">
              <Server className="w-4 h-4 text-blue-400" />
              <span>Hardware & Infrastructure</span>
            </h3>

            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
              <div className="bg-slate-950/60 border border-slate-800/80 rounded-xl p-3.5">
                <span className="text-[11px] font-semibold uppercase text-slate-500 block">
                  vCPU Allocation
                </span>
                <span className="text-lg font-bold text-white mt-1 block">
                  {instance.vcpus} Cores
                </span>
                <span className="text-[11px] text-slate-500">Dedicated compute</span>
              </div>

              <div className="bg-slate-950/60 border border-slate-800/80 rounded-xl p-3.5">
                <span className="text-[11px] font-semibold uppercase text-slate-500 block">
                  Memory (RAM)
                </span>
                <span className="text-lg font-bold text-white mt-1 block">
                  {Math.round(instance.memory_mb / 1024)} GB
                </span>
                <span className="text-[11px] text-slate-500">{instance.memory_mb} MB allocated</span>
              </div>

              <div className="bg-slate-950/60 border border-slate-800/80 rounded-xl p-3.5">
                <span className="text-[11px] font-semibold uppercase text-slate-500 block">
                  Storage (Disk)
                </span>
                <span className="text-lg font-bold text-white mt-1 block">
                  {instance.disk_gb} GB SSD
                </span>
                <span className="text-[11px] text-slate-500">ZFS mirrored pool</span>
              </div>

              <div className="bg-slate-950/60 border border-slate-800/80 rounded-xl p-3.5">
                <span className="text-[11px] font-semibold uppercase text-slate-500 block">
                  Proxmox Node
                </span>
                <span className="text-sm font-bold text-white mt-1 truncate block">
                  {instance.node?.proxmox_node_name || 'pve-dell-r640'}
                </span>
                <span className="text-[11px] text-slate-500 font-mono">
                  VMID #{instance.proxmox_vmid || '—'}
                </span>
              </div>
            </div>

            <div className="mt-6 pt-5 border-t border-slate-800 grid grid-cols-1 sm:grid-cols-3 gap-4 text-xs">
              <div>
                <span className="text-slate-500 block">Operating System</span>
                <span className="text-slate-200 font-medium">
                  {instance.os_template?.name || 'Ubuntu 24.04 LTS'}
                </span>
              </div>
              <div>
                <span className="text-slate-500 block">Default SSH User</span>
                <span className="text-slate-200 font-mono">{instance.ssh_username}</span>
              </div>
              <div>
                <span className="text-slate-500 block">Created On</span>
                <span className="text-slate-200">
                  {new Date(instance.created_at).toLocaleDateString([], {
                    year: 'numeric',
                    month: 'short',
                    day: 'numeric',
                    hour: '2-digit',
                    minute: '2-digit',
                  })}
                </span>
              </div>
            </div>
          </div>

          {/* Live Guest Resource Monitoring */}
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm">
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-bold text-white flex items-center gap-2">
                <Activity className="w-4 h-4 text-blue-400" />
                <span>Guest Resource Monitoring (Live Proxmox VE)</span>
              </h3>
              <span className="text-[11px] text-slate-500 flex items-center gap-1 font-mono">
                <span className="w-2 h-2 rounded-full bg-emerald-400 inline-block" />
                QEMU Agent
              </span>
            </div>

            {metrics && metrics.available ? (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4">
                  <div className="flex items-center justify-between text-xs text-slate-400 mb-2">
                    <span>CPU Utilization</span>
                    <Cpu className="w-3.5 h-3.5 text-blue-400" />
                  </div>
                  <div className="text-xl font-bold text-white font-mono">
                    {((metrics.cpu_usage || 0) * 100).toFixed(1)}%
                  </div>
                  <div className="w-full bg-slate-800 h-1.5 rounded-full mt-3 overflow-hidden">
                    <div
                      className="bg-blue-500 h-full rounded-full transition-all duration-300"
                      style={{ width: `${Math.min(100, (metrics.cpu_usage || 0) * 100)}%` }}
                    />
                  </div>
                </div>

                <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4">
                  <div className="flex items-center justify-between text-xs text-slate-400 mb-2">
                    <span>Memory Usage</span>
                    <Server className="w-3.5 h-3.5 text-purple-400" />
                  </div>
                  <div className="text-xl font-bold text-white font-mono">
                    {Math.round((metrics.memory_used || 0) / (1024 * 1024))} MB
                  </div>
                  <div className="w-full bg-slate-800 h-1.5 rounded-full mt-3 overflow-hidden">
                    <div
                      className="bg-purple-500 h-full rounded-full transition-all duration-300"
                      style={{
                        width: `${Math.min(
                          100,
                          ((metrics.memory_used || 1) / (metrics.memory_max || 1)) * 100
                        )}%`,
                      }}
                    />
                  </div>
                </div>

                <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4">
                  <div className="flex items-center justify-between text-xs text-slate-400 mb-2">
                    <span>Uptime</span>
                    <Clock className="w-3.5 h-3.5 text-emerald-400" />
                  </div>
                  <div className="text-xl font-bold text-white font-mono">
                    {Math.floor((metrics.uptime || 0) / 3600)}h{' '}
                    {Math.floor(((metrics.uptime || 0) % 3600) / 60)}m
                  </div>
                  <span className="text-[11px] text-slate-500 mt-2 block">Continuous hypervisor run</span>
                </div>
              </div>
            ) : (
              <div className="bg-slate-950/40 border border-slate-800/80 rounded-xl p-6 text-center text-xs text-slate-500">
                {metrics?.reason || 'Guest metrics are only reported while instance is in running state.'}
              </div>
            )}
          </div>
        </div>

        {/* Right Column: Connection Details & Danger Zone */}
        <div className="space-y-6">
          {/* SSH Connection Card */}
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm">
            <h3 className="text-sm font-bold text-white mb-3 flex items-center gap-2">
              <Terminal className="w-4 h-4 text-emerald-400" />
              <span>Connect via SSH</span>
            </h3>

            <p className="text-xs text-slate-400 mb-4 leading-relaxed">
              Use your registered SSH key to access this Linux virtual machine.
            </p>

            <div className="space-y-3">
              <div>
                <span className="text-[11px] font-semibold uppercase text-slate-500 block mb-1">
                  Private Network IP
                </span>
                <span className="font-mono text-sm text-emerald-400 bg-slate-950 px-3 py-1.5 rounded-xl border border-slate-800 block">
                  {instance.private_ip || 'Allocating lease...'}
                </span>
              </div>

              <div>
                <span className="text-[11px] font-semibold uppercase text-slate-500 block mb-1">
                  Terminal Command
                </span>
                <div className="relative">
                  <input
                    type="text"
                    readOnly
                    value={sshCommand}
                    className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-slate-200 font-mono pr-10 focus:outline-none select-all"
                  />
                  <button
                    onClick={() => copyToClipboard(sshCommand)}
                    className="absolute right-2 top-1/2 -translate-y-1/2 p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition"
                    title="Copy command"
                  >
                    {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>
            </div>

            <div className="mt-4 p-3 rounded-xl bg-slate-950/60 border border-slate-800 text-[11px] text-slate-400 space-y-1">
              <p className="font-semibold text-slate-300">Default Credentials:</p>
              <p>Username: <code className="text-blue-400">{instance.ssh_username}</code></p>
              <p>Port: <code className="text-blue-400">22</code></p>
            </div>
          </div>

          {/* Activity / Audit for this instance */}
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm">
            <h3 className="text-sm font-bold text-white mb-3 flex items-center gap-2">
              <Clock className="w-4 h-4 text-blue-400" />
              <span>Instance Events</span>
            </h3>

            <div className="space-y-2.5 max-h-56 overflow-y-auto pr-1">
              {activity.length === 0 ? (
                <p className="text-xs text-slate-500 text-center py-4">No events logged yet.</p>
              ) : (
                activity.map((item) => (
                  <div key={item.id} className="text-xs p-2 rounded-lg bg-slate-950/50 border border-slate-800/80">
                    <div className="flex items-center justify-between font-mono text-[11px]">
                      <span className="text-blue-400">{item.action}</span>
                      <span className="text-emerald-400">{item.result}</span>
                    </div>
                    <div className="text-[10px] text-slate-500 mt-0.5">
                      {new Date(item.created_at).toLocaleString()}
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>

          {/* Danger Zone */}
          <div className="bg-slate-900 border border-rose-900/50 rounded-2xl p-6 shadow-sm">
            <h3 className="text-sm font-bold text-rose-400 mb-2 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4" />
              <span>Danger Zone</span>
            </h3>
            <p className="text-xs text-slate-400 mb-4">
              Permanently destroy this virtual machine and remove all assigned virtual storage from the hypervisor.
            </p>
            <button
              onClick={() => setShowDeleteModal(true)}
              className="w-full py-2 px-3 rounded-xl bg-rose-600/20 hover:bg-rose-600 text-rose-300 hover:text-white border border-rose-700/60 font-semibold text-xs transition"
            >
              Destroy Instance
            </button>
          </div>
        </div>
      </div>

      {/* Delete Confirmation Modal */}
      <Modal
        isOpen={showDeleteModal}
        onClose={() => setShowDeleteModal(false)}
        title="Destroy Virtual Machine"
        description="This action is permanent and cannot be undone."
      >
        <div className="space-y-4">
          <p className="text-xs text-slate-300">
            Type <strong className="text-white font-mono">{instance.name}</strong> to confirm destruction of this instance and release its quota.
          </p>
          <input
            type="text"
            value={confirmName}
            onChange={(e) => setConfirmName(e.target.value)}
            placeholder={instance.name}
            className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-rose-500 font-mono"
          />
          <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              onClick={() => setShowDeleteModal(false)}
              className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold"
            >
              Cancel
            </button>
            <button
              onClick={handleDelete}
              disabled={confirmName !== instance.name || deleting}
              className="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold disabled:opacity-50 transition"
            >
              {deleting ? 'Destroying...' : 'Confirm & Destroy'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
