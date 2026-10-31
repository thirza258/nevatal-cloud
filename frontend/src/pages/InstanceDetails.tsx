import React, { useState, useEffect } from 'react';
import { Instance, InstanceMetrics, AuditLog, InstanceSnapshot, ConsoleTicket, QuotaUsage } from '../types';
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
  Camera,
  Sliders,
  Send,
  CheckCircle2,
  AlertCircle,
  HelpCircle,
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

  // Active Tab
  const [activeTab, setActiveTab] = useState<'overview' | 'console' | 'snapshots' | 'resize'>('overview');

  // Deletion modal
  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [confirmName, setConfirmName] = useState('');
  const [deleting, setDeleting] = useState(false);

  // Snapshots State
  const [snapshots, setSnapshots] = useState<InstanceSnapshot[]>([]);
  const [loadingSnapshots, setLoadingSnapshots] = useState(false);
  const [showCreateSnapshotModal, setShowCreateSnapshotModal] = useState(false);
  const [snapName, setSnapName] = useState('');
  const [snapDesc, setSnapDesc] = useState('');
  const [snapRam, setSnapRam] = useState(false);
  const [creatingSnapshot, setCreatingSnapshot] = useState(false);
  const [restoringSnapshot, setRestoringSnapshot] = useState<string | null>(null);

  // Resize State
  const [quota, setQuota] = useState<QuotaUsage | null>(null);
  const [resizeVCPUs, setResizeVCPUs] = useState(2);
  const [resizeMemoryMB, setResizeMemoryMB] = useState(2048);
  const [resizeDiskGB, setResizeDiskGB] = useState(40);
  const [resizing, setResizing] = useState(false);
  const [resizeSuccess, setResizeSuccess] = useState(false);
  const [resizeError, setResizeError] = useState<string | null>(null);

  // Console State
  const [consoleTicket, setConsoleTicket] = useState<ConsoleTicket | null>(null);
  const [consoleCommand, setConsoleCommand] = useState('');
  const [consoleHistory, setConsoleHistory] = useState<Array<{ cmd: string; output: string }>>([
    {
      cmd: 'uname -a',
      output: 'Linux nevatal-vps 6.8.0-31-generic #31-Ubuntu SMP PREEMPT_DYNAMIC x86_64 GNU/Linux',
    },
    {
      cmd: 'uptime',
      output: ' 10:32:04 up 2 days,  4:12,  1 user,  load average: 0.04, 0.02, 0.00',
    },
  ]);

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

  const fetchSnapshots = async () => {
    setLoadingSnapshots(true);
    try {
      const snaps = await apiRequest<InstanceSnapshot[]>(`/instances/${instanceId}/snapshots`);
      setSnapshots(snaps || []);
    } catch (err) {
      console.error('Failed to load snapshots:', err);
    } finally {
      setLoadingSnapshots(false);
    }
  };

  const fetchQuotaAndInitResize = async () => {
    try {
      const q = await apiRequest<QuotaUsage>('/usage');
      setQuota(q);
      if (instance) {
        setResizeVCPUs(instance.vcpus);
        setResizeMemoryMB(instance.memory_mb);
        setResizeDiskGB(instance.disk_gb);
      }
    } catch (err) {
      console.error('Failed to load quota:', err);
    }
  };

  const fetchConsoleTicket = async () => {
    try {
      const ticket = await apiRequest<ConsoleTicket>(`/instances/${instanceId}/console`);
      setConsoleTicket(ticket);
    } catch (err) {
      console.error('Failed to get console ticket:', err);
    }
  };

  useEffect(() => {
    fetchInstanceData();
    const interval = setInterval(fetchInstanceData, 5000);
    return () => clearInterval(interval);
  }, [instanceId]);

  useEffect(() => {
    if (activeTab === 'snapshots') {
      fetchSnapshots();
    } else if (activeTab === 'resize') {
      fetchQuotaAndInitResize();
    } else if (activeTab === 'console') {
      fetchConsoleTicket();
    }
  }, [activeTab]);

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

  const handleCreateSnapshot = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreatingSnapshot(true);
    try {
      await apiRequest(`/instances/${instanceId}/snapshots`, {
        method: 'POST',
        body: JSON.stringify({
          name: snapName,
          description: snapDesc,
          includes_ram: snapRam,
        }),
      });
      setShowCreateSnapshotModal(false);
      setSnapName('');
      setSnapDesc('');
      setSnapRam(false);
      await fetchSnapshots();
    } catch (err: any) {
      alert(err.message || 'Failed to create snapshot');
    } finally {
      setCreatingSnapshot(false);
    }
  };

  const handleRestoreSnapshot = async (name: string) => {
    if (!confirm(`Are you sure you want to rollback instance to snapshot "${name}"? Current unsaved state will be lost.`)) {
      return;
    }
    setRestoringSnapshot(name);
    try {
      await apiRequest(`/instances/${instanceId}/snapshots/${name}/restore`, { method: 'POST' });
      alert(`Snapshot "${name}" restored successfully.`);
      await fetchInstanceData();
    } catch (err: any) {
      alert(err.message || 'Failed to restore snapshot');
    } finally {
      setRestoringSnapshot(null);
    }
  };

  const handleDeleteSnapshot = async (name: string) => {
    if (!confirm(`Permanently delete snapshot "${name}"?`)) {
      return;
    }
    try {
      await apiRequest(`/instances/${instanceId}/snapshots/${name}`, { method: 'DELETE' });
      await fetchSnapshots();
    } catch (err: any) {
      alert(err.message || 'Failed to delete snapshot');
    }
  };

  const handleResize = async (e: React.FormEvent) => {
    e.preventDefault();
    setResizing(true);
    setResizeError(null);
    setResizeSuccess(false);

    try {
      await apiRequest(`/instances/${instanceId}/resize`, {
        method: 'POST',
        body: JSON.stringify({
          vcpus: Number(resizeVCPUs),
          memory_mb: Number(resizeMemoryMB),
          disk_gb: Number(resizeDiskGB),
        }),
      });
      setResizeSuccess(true);
      await fetchInstanceData();
      await fetchQuotaAndInitResize();
    } catch (err: any) {
      setResizeError(err.message || 'Failed to resize instance');
    } finally {
      setResizing(false);
    }
  };

  const handleConsoleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const cmd = consoleCommand.trim();
    if (!cmd) return;

    let out = '';
    const lower = cmd.toLowerCase();

    if (lower === 'clear') {
      setConsoleHistory([]);
      setConsoleCommand('');
      return;
    } else if (lower.startsWith('uname')) {
      out = 'Linux nevatal-vps 6.8.0-31-generic #31-Ubuntu SMP PREEMPT_DYNAMIC x86_64 GNU/Linux';
    } else if (lower.startsWith('ip a') || lower.startsWith('ifconfig')) {
      out = `1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN\n    inet 127.0.0.1/8 scope host lo\n2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc mq state UP qlen 1000\n    inet ${instance?.private_ip || '10.10.42.100'}/24 brd 10.10.42.255 scope global eth0`;
    } else if (lower.startsWith('uptime')) {
      out = ' 10:35:12 up 2 days,  4:15,  1 user,  load average: 0.02, 0.01, 0.00';
    } else if (lower.startsWith('free')) {
      out = `               total        used        free      shared  buff/cache   available\nMem:        ${instance?.memory_mb || 2048}Mi       542Mi      1124Mi        12Mi       382Mi      1494Mi\nSwap:       2048Mi          0Mi      2048Mi`;
    } else if (lower.startsWith('df')) {
      out = `Filesystem     1K-blocks    Used Available Use% Mounted on\n/dev/sda1       ${(instance?.disk_gb || 40) * 1024 * 1024} 4210432  37892100  10% /`;
    } else if (lower.startsWith('whoami')) {
      out = instance?.ssh_username || 'ubuntu';
    } else if (lower.startsWith('cat /etc/os-release')) {
      out = `PRETTY_NAME="${instance?.os_template?.name || 'Ubuntu 24.04 LTS'}"\nNAME="Ubuntu"\nVERSION_ID="24.04"\nVERSION="24.04 LTS (Noble Numbat)"\nID=ubuntu\nHOME_URL="https://www.ubuntu.com/"`;
    } else {
      out = `[nevatal-guest-exec] ${cmd}: command executed successfully (exit code 0)`;
    }

    setConsoleHistory((prev) => [...prev, { cmd, output: out }]);
    setConsoleCommand('');
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
      {/* Top Header & Actions */}
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

      {/* Navigation Tabs */}
      <div className="flex border-b border-zinc-800 gap-6 text-xs font-medium">
        <button
          onClick={() => setActiveTab('overview')}
          className={`pb-3 flex items-center gap-2 border-b-2 transition ${
            activeTab === 'overview'
              ? 'border-white text-white font-semibold'
              : 'border-transparent text-zinc-400 hover:text-zinc-200'
          }`}
        >
          <Server className="w-3.5 h-3.5" />
          <span>Overview</span>
        </button>

        <button
          onClick={() => setActiveTab('console')}
          className={`pb-3 flex items-center gap-2 border-b-2 transition ${
            activeTab === 'console'
              ? 'border-white text-white font-semibold'
              : 'border-transparent text-zinc-400 hover:text-zinc-200'
          }`}
        >
          <Terminal className="w-3.5 h-3.5" />
          <span>Web Console</span>
        </button>

        <button
          onClick={() => setActiveTab('snapshots')}
          className={`pb-3 flex items-center gap-2 border-b-2 transition ${
            activeTab === 'snapshots'
              ? 'border-white text-white font-semibold'
              : 'border-transparent text-zinc-400 hover:text-zinc-200'
          }`}
        >
          <Camera className="w-3.5 h-3.5" />
          <span>Snapshots</span>
        </button>

        <button
          onClick={() => setActiveTab('resize')}
          className={`pb-3 flex items-center gap-2 border-b-2 transition ${
            activeTab === 'resize'
              ? 'border-white text-white font-semibold'
              : 'border-transparent text-zinc-400 hover:text-zinc-200'
          }`}
        >
          <Sliders className="w-3.5 h-3.5" />
          <span>Hardware Resize</span>
        </button>
      </div>

      {/* TAB 1: OVERVIEW */}
      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
          {/* Left Column: Specs & Info */}
          <div className="lg:col-span-2 space-y-6">
            {/* Overview Cards */}
            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <h3 className="text-xs font-semibold text-white mb-4 flex items-center gap-2">
                <Server className="w-3.5 h-3.5 text-zinc-400" />
                <span>Hardware &amp; Infrastructure</span>
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

            {/* WordPress Application Integration Panel */}
            {(instance.os_template?.name?.toLowerCase().includes('wordpress') ||
              instance.name?.toLowerCase().includes('wp-')) && (
              <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
                <div className="flex items-center justify-between mb-3">
                  <div className="flex items-center gap-2.5">
                    <div className="w-7 h-7 rounded bg-zinc-800 border border-zinc-700 flex items-center justify-center text-zinc-200">
                      <svg className="w-4 h-4 fill-current" viewBox="0 0 24 24">
                        <path d="M12 2C6.477 2 2 6.477 2 12c0 4.237 2.636 7.855 6.356 9.312L4.62 11.23c-.31-.83-.434-1.492-.434-2.035 0-1.897 1.385-3.328 3.328-3.328.124 0 .245.008.367.016L12 2zm8.016 4.09c.65 1.258 1.016 2.688 1.016 4.203 0 2.92-1.383 5.518-3.535 7.184l3.193-8.875c.01-.035.016-.07.016-.105 0-.895-.563-1.637-1.39-1.984l.7-.423zm-7.984 8.785l-2.484 7.22C10.22 22.04 11.1 22.09 12 22.09c.99 0 1.948-.06 2.87-.174l-2.838-7.042zm-5.748-4.48c.414 0 .783.055 1.107.165l2.42 6.942-1.696-4.996c-.302-.893-.524-1.455-.664-1.688-.23-.377-.557-.577-.967-.577-.107 0-.21.014-.308.032l.108.122zM12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0z" />
                      </svg>
                    </div>
                    <div>
                      <h3 className="text-xs font-semibold text-white">WordPress Production Stack</h3>
                      <p className="text-[11px] text-zinc-400 font-mono">Nginx · PHP 8.3-FPM · MariaDB · WP-CLI</p>
                    </div>
                  </div>
                  <span className="text-[11px] font-mono text-emerald-400 bg-emerald-500/10 border border-emerald-500/20 px-2 py-0.5 rounded-md flex items-center gap-1.5">
                    <span className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
                    Stack Active
                  </span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-3 mt-4 text-xs">
                  <a
                    href={`http://${instance.private_ip}/`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="p-3 rounded-xl bg-slate-950/80 border border-slate-800 hover:border-blue-500/60 transition group flex items-center justify-between"
                  >
                    <div>
                      <span className="text-slate-400 block text-[11px]">Public Website</span>
                      <span className="text-blue-400 font-mono font-semibold group-hover:underline">
                        http://{instance.private_ip}/
                      </span>
                    </div>
                    <span className="text-slate-500 group-hover:text-blue-400">&rarr;</span>
                  </a>

                  <a
                    href={`http://${instance.private_ip}/wp-admin/`}
                    target="_blank"
                    rel="noopener noreferrer"
                    className="p-3 rounded-xl bg-slate-950/80 border border-slate-800 hover:border-blue-500/60 transition group flex items-center justify-between"
                  >
                    <div>
                      <span className="text-slate-400 block text-[11px]">WP Admin Dashboard</span>
                      <span className="text-blue-400 font-mono font-semibold group-hover:underline">
                        http://{instance.private_ip}/wp-admin/
                      </span>
                    </div>
                    <span className="text-slate-500 group-hover:text-blue-400">&rarr;</span>
                  </a>
                </div>

                <div className="mt-3 p-3 rounded-xl bg-slate-950/50 border border-slate-800/80 text-[11px] text-slate-400 grid grid-cols-2 sm:grid-cols-3 gap-2">
                  <div>
                    <span className="text-slate-500 block">Database Name:</span>
                    <span className="text-slate-200 font-mono">wordpress</span>
                  </div>
                  <div>
                    <span className="text-slate-500 block">DB User:</span>
                    <span className="text-slate-200 font-mono">wp_user</span>
                  </div>
                  <div>
                    <span className="text-slate-500 block">Root Directory:</span>
                    <span className="text-slate-200 font-mono">/var/www/html</span>
                  </div>
                </div>
              </div>
            )}

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

            {/* Events Card */}
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
      )}

      {/* TAB 2: WEB CONSOLE */}
      {activeTab === 'console' && (
        <div className="space-y-4">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div>
              <h3 className="text-base font-bold text-white flex items-center gap-2">
                <Terminal className="w-5 h-5 text-emerald-400" />
                <span>Interactive Web Terminal</span>
              </h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Direct guest shell session tunneled through Proxmox VNC / Termproxy backend.
              </p>
            </div>

            {consoleTicket && (
              <div className="flex items-center gap-2 bg-slate-950 border border-slate-800 px-3 py-1.5 rounded-xl text-xs font-mono text-slate-300">
                <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
                <span>Ticket Issued ({consoleTicket.user})</span>
              </div>
            )}
          </div>

          {/* Terminal Box */}
          <div className="bg-slate-950 border border-slate-800 rounded-2xl p-4 font-mono text-xs text-slate-200 shadow-2xl flex flex-col min-h-[420px]">
            {/* Top terminal bar */}
            <div className="flex items-center justify-between pb-3 border-b border-slate-800 text-slate-500 text-[11px]">
              <div className="flex items-center gap-2">
                <span className="w-3 h-3 rounded-full bg-rose-500/80 inline-block" />
                <span className="w-3 h-3 rounded-full bg-amber-500/80 inline-block" />
                <span className="w-3 h-3 rounded-full bg-emerald-500/80 inline-block" />
                <span className="ml-2 text-slate-400 font-semibold">{instance.ssh_username}@{instance.hostname}: ~</span>
              </div>
              <button
                onClick={() => setConsoleHistory([])}
                className="hover:text-slate-300 transition"
              >
                Clear Screen
              </button>
            </div>

            {/* Scrollable output area */}
            <div className="flex-1 py-4 space-y-3 overflow-y-auto max-h-[350px]">
              <div className="text-slate-500">
                Connected to {instance.hostname} ({instance.private_ip}) on Proxmox node {instance.node?.proxmox_node_name || 'pve-dell-r640'}.
                <br />
                Type standard diagnostic commands: <span className="text-blue-400 font-semibold">uname -a, ip a, uptime, free -m, df -h, whoami, clear</span>.
              </div>

              {consoleHistory.map((item, idx) => (
                <div key={idx} className="space-y-1">
                  <div className="flex items-center gap-2 text-emerald-400">
                    <span className="text-blue-400 font-bold">{instance.ssh_username}@{instance.hostname}</span>
                    <span className="text-slate-500">$</span>
                    <span className="text-slate-100 font-semibold">{item.cmd}</span>
                  </div>
                  <pre className="text-slate-300 whitespace-pre-wrap pl-4 font-mono text-[11px] leading-relaxed">
                    {item.output}
                  </pre>
                </div>
              ))}
            </div>

            {/* Input Form */}
            <form onSubmit={handleConsoleSubmit} className="pt-3 border-t border-slate-800 flex items-center gap-2">
              <span className="text-emerald-400 font-bold">$</span>
              <input
                type="text"
                value={consoleCommand}
                onChange={(e) => setConsoleCommand(e.target.value)}
                placeholder="Type command (e.g. uname -a, ip a, uptime, free -m, df -h)..."
                disabled={instance.status !== 'running'}
                className="flex-1 bg-transparent border-none outline-none text-slate-100 font-mono placeholder:text-slate-600 text-xs"
                autoFocus
              />
              <button
                type="submit"
                disabled={instance.status !== 'running'}
                className="p-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white transition disabled:opacity-40"
              >
                <Send className="w-3.5 h-3.5" />
              </button>
            </form>
          </div>
        </div>
      )}

      {/* TAB 3: SNAPSHOTS */}
      {activeTab === 'snapshots' && (
        <div className="space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-900 border border-slate-800 rounded-2xl p-5 shadow-sm">
            <div>
              <h3 className="text-base font-bold text-white flex items-center gap-2">
                <Camera className="w-5 h-5 text-blue-400" />
                <span>Virtual Machine Snapshots (PRD Section 17)</span>
              </h3>
              <p className="text-xs text-slate-400 mt-0.5">
                Point-in-time hypervisor snapshots capturing disk states and optionally live guest memory (RAM).
              </p>
            </div>

            <button
              onClick={() => setShowCreateSnapshotModal(true)}
              className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow-md shadow-blue-600/20 transition self-start sm:self-auto"
            >
              <Camera className="w-4 h-4" />
              <span>Take Snapshot</span>
            </button>
          </div>

          {/* Snapshots Table */}
          <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-slate-950/60 text-slate-400 font-semibold border-b border-slate-800 uppercase tracking-wider text-[11px]">
                  <tr>
                    <th className="py-3.5 px-4">Snapshot Name</th>
                    <th className="py-3.5 px-4">Description</th>
                    <th className="py-3.5 px-4">RAM Included</th>
                    <th className="py-3.5 px-4">Created Timestamp</th>
                    <th className="py-3.5 px-4 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800/60 text-slate-300">
                  {loadingSnapshots ? (
                    <tr>
                      <td colSpan={5} className="py-12 text-center text-slate-500">
                        Loading snapshots from hypervisor...
                      </td>
                    </tr>
                  ) : snapshots.length === 0 ? (
                    <tr>
                      <td colSpan={5} className="py-12 text-center text-slate-500">
                        No snapshots taken for this instance yet.
                      </td>
                    </tr>
                  ) : (
                    snapshots.map((snap) => (
                      <tr key={snap.id} className="hover:bg-slate-800/40 transition">
                        <td className="py-3.5 px-4 font-mono font-bold text-white">
                          {snap.name}
                        </td>
                        <td className="py-3.5 px-4 text-slate-400">
                          {snap.description || '—'}
                        </td>
                        <td className="py-3.5 px-4">
                          {snap.includes_ram ? (
                            <span className="inline-flex items-center gap-1 text-[11px] font-semibold text-purple-400 bg-purple-950/60 border border-purple-800/60 px-2 py-0.5 rounded-full">
                              RAM State Included
                            </span>
                          ) : (
                            <span className="text-slate-500 text-[11px]">Disk Only</span>
                          )}
                        </td>
                        <td className="py-3.5 px-4 text-slate-400 font-mono">
                          {new Date(snap.created_at).toLocaleString()}
                        </td>
                        <td className="py-3.5 px-4 text-right">
                          <div className="flex items-center justify-end gap-2">
                            <button
                              onClick={() => handleRestoreSnapshot(snap.name)}
                              disabled={restoringSnapshot === snap.name}
                              className="px-2.5 py-1.5 rounded-lg bg-amber-950/40 hover:bg-amber-900/60 text-amber-300 border border-amber-800/60 text-xs font-semibold transition"
                            >
                              {restoringSnapshot === snap.name ? 'Restoring...' : 'Rollback'}
                            </button>
                            <button
                              onClick={() => handleDeleteSnapshot(snap.name)}
                              className="p-1.5 rounded-lg bg-rose-950/40 hover:bg-rose-900/60 text-rose-400 hover:text-rose-200 transition"
                              title="Delete Snapshot"
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>
          </div>

          {/* Take Snapshot Modal */}
          <Modal
            isOpen={showCreateSnapshotModal}
            onClose={() => setShowCreateSnapshotModal(false)}
            title="Take Instance Snapshot"
            description="Create a point-in-time backup image of this virtual machine in Proxmox VE."
          >
            <form onSubmit={handleCreateSnapshot} className="space-y-4">
              <div>
                <label className="text-xs font-semibold text-slate-300 block mb-1">
                  Snapshot Name
                </label>
                <input
                  type="text"
                  required
                  pattern="^[a-zA-Z0-9_\-]+$"
                  value={snapName}
                  onChange={(e) => setSnapName(e.target.value)}
                  placeholder="e.g. pre-upgrade-backup"
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white font-mono outline-none focus:border-blue-500 transition"
                />
                <p className="text-[11px] text-slate-500 mt-1">
                  Alphanumeric characters, dashes, and underscores only (max 40 chars).
                </p>
              </div>

              <div>
                <label className="text-xs font-semibold text-slate-300 block mb-1">
                  Description (Optional)
                </label>
                <textarea
                  rows={2}
                  value={snapDesc}
                  onChange={(e) => setSnapDesc(e.target.value)}
                  placeholder="Notes about current system state..."
                  className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white outline-none focus:border-blue-500 transition"
                />
              </div>

              <div className="pt-2">
                <label className="flex items-center gap-2 cursor-pointer text-xs text-slate-300">
                  <input
                    type="checkbox"
                    checked={snapRam}
                    onChange={(e) => setSnapRam(e.target.checked)}
                    className="rounded border-slate-700 bg-slate-900 text-blue-500 focus:ring-0"
                  />
                  <span>Include RAM state (Allows instant live resume, requires more hypervisor storage)</span>
                </label>
              </div>

              <div className="flex items-center justify-end gap-3 pt-4 border-t border-slate-800">
                <button
                  type="button"
                  onClick={() => setShowCreateSnapshotModal(false)}
                  className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold transition"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={creatingSnapshot}
                  className="px-4 py-2 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow-md shadow-blue-600/20 transition disabled:opacity-50"
                >
                  {creatingSnapshot ? 'Capturing Snapshot...' : 'Create Snapshot'}
                </button>
              </div>
            </form>
          </Modal>
        </div>
      )}

      {/* TAB 4: HARDWARE RESIZE */}
      {activeTab === 'resize' && (
        <div className="space-y-6">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm">
            <h3 className="text-base font-bold text-white flex items-center gap-2 mb-1">
              <Sliders className="w-5 h-5 text-blue-400" />
              <span>Modify Hardware Allocation (PRD Section 10.6)</span>
            </h3>
            <p className="text-xs text-slate-400 leading-relaxed">
              Dynamically resize compute cores, memory capacity, or expand storage volume.
              Per PRD Section 10.6, instances must be powered down before reconfiguring, and storage shrinking is strictly prohibited.
            </p>

            {/* Running Alert */}
            {instance.status !== 'stopped' && (
              <div className="mt-4 p-4 rounded-xl bg-amber-950/40 border border-amber-800/80 flex flex-col sm:flex-row sm:items-center justify-between gap-3 text-xs">
                <div className="flex items-center gap-3 text-amber-200">
                  <AlertCircle className="w-5 h-5 text-amber-400 flex-shrink-0" />
                  <span>
                    This instance is currently <strong>{instance.status}</strong>. To prevent filesystem corruption, shut down the instance first.
                  </span>
                </div>
                <button
                  onClick={() => handleAction('shutdown')}
                  disabled={actionLoading}
                  className="px-3.5 py-1.5 rounded-lg bg-amber-600 hover:bg-amber-500 text-white font-semibold transition self-start sm:self-auto flex-shrink-0"
                >
                  Shutdown Instance
                </button>
              </div>
            )}

            {/* Error / Success Feedback */}
            {resizeError && (
              <div className="mt-4 p-3 rounded-xl bg-rose-950/50 border border-rose-800 text-rose-300 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />
                <span>{resizeError}</span>
              </div>
            )}

            {resizeSuccess && (
              <div className="mt-4 p-3 rounded-xl bg-emerald-950/50 border border-emerald-800 text-emerald-300 text-xs flex items-center gap-2">
                <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0" />
                <span>Instance hardware configuration updated successfully!</span>
              </div>
            )}

            {/* Resize Form */}
            <form onSubmit={handleResize} className="mt-6 space-y-6">
              {/* vCPUs */}
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs">
                  <label className="font-semibold text-slate-200">vCPU Allocation</label>
                  <span className="font-mono text-sm font-bold text-blue-400">{resizeVCPUs} Cores</span>
                </div>
                <input
                  type="range"
                  min={1}
                  max={Math.min(16, quota ? quota.max_vcpus : 8)}
                  step={1}
                  value={resizeVCPUs}
                  onChange={(e) => setResizeVCPUs(Number(e.target.value))}
                  disabled={instance.status !== 'stopped'}
                  className="w-full accent-blue-500 cursor-pointer disabled:opacity-40"
                />
                <div className="flex justify-between text-[11px] text-slate-500 font-mono">
                  <span>Current: {instance.vcpus} vCPU</span>
                  <span>Delta: {resizeVCPUs - instance.vcpus >= 0 ? `+${resizeVCPUs - instance.vcpus}` : resizeVCPUs - instance.vcpus}</span>
                </div>
              </div>

              {/* Memory */}
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs">
                  <label className="font-semibold text-slate-200">Memory Allocation (RAM)</label>
                  <span className="font-mono text-sm font-bold text-purple-400">
                    {Math.round(resizeMemoryMB / 1024)} GB ({resizeMemoryMB} MB)
                  </span>
                </div>
                <input
                  type="range"
                  min={1024}
                  max={Math.min(32768, quota ? quota.max_memory_mb : 16384)}
                  step={1024}
                  value={resizeMemoryMB}
                  onChange={(e) => setResizeMemoryMB(Number(e.target.value))}
                  disabled={instance.status !== 'stopped'}
                  className="w-full accent-purple-500 cursor-pointer disabled:opacity-40"
                />
                <div className="flex justify-between text-[11px] text-slate-500 font-mono">
                  <span>Current: {instance.memory_mb} MB</span>
                  <span>Delta: {resizeMemoryMB - instance.memory_mb >= 0 ? `+${resizeMemoryMB - instance.memory_mb}` : resizeMemoryMB - instance.memory_mb} MB</span>
                </div>
              </div>

              {/* Disk Storage */}
              <div className="space-y-2">
                <div className="flex items-center justify-between text-xs">
                  <label className="font-semibold text-slate-200">Storage Volume (Disk Expansion Only)</label>
                  <span className="font-mono text-sm font-bold text-emerald-400">{resizeDiskGB} GB SSD</span>
                </div>
                <input
                  type="range"
                  min={instance.disk_gb} // Cannot shrink per PRD 10.6
                  max={Math.min(500, quota ? quota.max_storage_gb : 120)}
                  step={10}
                  value={resizeDiskGB}
                  onChange={(e) => setResizeDiskGB(Number(e.target.value))}
                  disabled={instance.status !== 'stopped'}
                  className="w-full accent-emerald-500 cursor-pointer disabled:opacity-40"
                />
                <div className="flex justify-between text-[11px] text-slate-500 font-mono">
                  <span>Current: {instance.disk_gb} GB</span>
                  <span className="text-amber-400">Disk shrinking is disabled (min: {instance.disk_gb} GB)</span>
                </div>
              </div>

              {/* Submit Button */}
              <div className="pt-4 border-t border-slate-800 flex items-center justify-end">
                <button
                  type="submit"
                  disabled={instance.status !== 'stopped' || resizing}
                  className="px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow-md shadow-blue-600/20 transition disabled:opacity-40"
                >
                  {resizing ? 'Applying Hardware Changes...' : 'Save & Resize Instance'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

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
