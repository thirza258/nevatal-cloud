import React, { useEffect, useState } from 'react';
import {
  Workflow,
  ArrowLeft,
  ExternalLink,
  Play,
  Square,
  RotateCw,
  Copy,
  Check,
  ShieldCheck,
  HardDrive,
  Download,
  Trash2,
  Terminal,
  Clock,
  Layers,
  Database
} from 'lucide-react';
import { AutomationInstance, AutomationBackup } from '../../types';

interface AutomationDetailsProps {
  instanceId: string;
  onNavigate: (path: string, param?: string) => void;
}

export const AutomationDetails: React.FC<AutomationDetailsProps> = ({ instanceId, onNavigate }) => {
  const [inst, setInst] = useState<AutomationInstance | null>(null);
  const [backups, setBackups] = useState<AutomationBackup[]>([]);
  const [logs, setLogs] = useState<string[]>([]);
  const [activeTab, setActiveTab] = useState<'overview' | 'backups' | 'logs'>('overview');
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [copiedKey, setCopiedKey] = useState(false);

  const fetchInstanceData = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/automation/instances/${instanceId}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to fetch automation instance');
      setInst(data.data);
      setBackups(data.data.backups || []);

      // Fetch logs
      const logRes = await fetch(`/api/v1/automation/instances/${instanceId}/logs`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const logData = await logRes.json();
      if (logRes.ok) setLogs(logData.data || []);
    } catch (err: any) {
      console.error(err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchInstanceData();
  }, [instanceId]);

  const handlePower = async (action: 'start' | 'stop' | 'restart') => {
    try {
      setActionLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/automation/instances/${instanceId}/${action}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error(`Action ${action} failed`);
      await fetchInstanceData();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setActionLoading(false);
    }
  };

  const handleCreateBackup = async () => {
    try {
      setActionLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/automation/instances/${instanceId}/backups`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Backup creation failed');
      await fetchInstanceData();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setActionLoading(false);
    }
  };

  const handleDelete = async () => {
    if (!confirm(`Are you sure you want to delete ${inst?.name}? All workflows and executions will be permanently removed.`)) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/automation/instances/${instanceId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Delete failed');
      onNavigate('/dashboard/automation');
    } catch (err: any) {
      alert(err.message);
    }
  };

  const copyEncryptionKey = () => {
    if (inst?.encryption_key) {
      navigator.clipboard.writeText(inst.encryption_key);
      setCopiedKey(true);
      setTimeout(() => setCopiedKey(false), 2000);
    }
  };

  if (loading) {
    return (
      <div className="p-12 text-center text-slate-500 flex flex-col items-center justify-center gap-3">
        <div className="w-8 h-8 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-sm">Loading instance...</span>
      </div>
    );
  }

  if (!inst) {
    return (
      <div className="p-6 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400">
        <h3 className="font-semibold mb-2">Automation Instance Not Found</h3>
        <button
          onClick={() => onNavigate('/dashboard/automation')}
          className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-white rounded-lg text-xs"
        >
          Back to Automations
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Top Bar Navigation */}
      <div className="flex items-center gap-3 text-xs text-slate-400">
        <button
          onClick={() => onNavigate('/dashboard/automation')}
          className="hover:text-white flex items-center gap-1 cursor-pointer"
        >
          <ArrowLeft className="w-3.5 h-3.5" /> Workflow Automation
        </button>
        <span>/</span>
        <span className="text-slate-200 font-medium">{inst.name}</span>
      </div>

      {/* Hero Overview */}
      <div className="bg-slate-900/80 border border-slate-800/80 rounded-xl p-6">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-emerald-600/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400">
              <Workflow className="w-6 h-6" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-xl font-bold text-white tracking-tight">{inst.name}</h1>
                <span
                  className={`text-xs px-2.5 py-0.5 rounded-full font-medium border flex items-center gap-1.5 ${
                    inst.status === 'running'
                      ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                      : 'bg-slate-700/30 text-slate-400 border-slate-700/50'
                  }`}
                >
                  {inst.status === 'running' && <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />}
                  {inst.status}
                </span>
              </div>

              <div className="flex items-center gap-3 mt-2 text-xs">
                <a
                  href={inst.dashboard_url}
                  target="_blank"
                  rel="noreferrer"
                  className="text-emerald-400 hover:text-emerald-300 font-mono flex items-center gap-1"
                >
                  {inst.dashboard_url}
                  <ExternalLink className="w-3 h-3" />
                </a>
                <span className="text-slate-600">•</span>
                <span className="text-slate-400 font-mono">v{inst.n8n_version}</span>
                <span className="text-slate-600">•</span>
                <span className="text-slate-400">{inst.cpu_limit} vCPU / {(inst.memory_limit_mb / 1024).toFixed(1)} GB RAM</span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <a
              href={inst.dashboard_url}
              target="_blank"
              rel="noreferrer"
              className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-medium transition flex items-center gap-1.5 shadow-sm"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              Open n8n UI
            </a>

            {inst.status === 'running' ? (
              <button
                onClick={() => handlePower('stop')}
                disabled={actionLoading}
                className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
                title="Stop instance"
              >
                <Square className="w-4 h-4 text-amber-400" />
              </button>
            ) : (
              <button
                onClick={() => handlePower('start')}
                disabled={actionLoading}
                className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
                title="Start instance"
              >
                <Play className="w-4 h-4 text-emerald-400" />
              </button>
            )}

            <button
              onClick={() => handlePower('restart')}
              disabled={actionLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
              title="Restart instance"
            >
              <RotateCw className="w-4 h-4 text-blue-400" />
            </button>

            <button
              onClick={handleDelete}
              className="p-2 rounded-lg bg-red-500/10 hover:bg-red-500/20 text-red-400 transition cursor-pointer"
              title="Delete instance"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-slate-800 flex items-center gap-2">
        {[
          { id: 'overview', label: 'Overview & Security', icon: ShieldCheck },
          { id: 'backups', label: 'Automated Backups', icon: HardDrive },
          { id: 'logs', label: 'Engine Runtime Logs', icon: Terminal }
        ].map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as any)}
              className={`flex items-center gap-2 px-4 py-2.5 text-xs font-medium border-b-2 transition cursor-pointer ${
                activeTab === tab.id
                  ? 'border-emerald-500 text-emerald-400'
                  : 'border-transparent text-slate-400 hover:text-slate-200'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {activeTab === 'overview' && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
          {/* Encryption Key Card */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-3">
            <div className="flex items-center gap-2 text-sm font-semibold text-white">
              <ShieldCheck className="w-4 h-4 text-emerald-400" />
              Credential Encryption Key
            </div>
            <p className="text-xs text-slate-400 leading-relaxed">
              n8n encrypts all saved API credentials, OAuth tokens, and sensitive workflow parameters with this persistent key. Keep it safe in case of disaster recovery.
            </p>
            <div className="flex items-center gap-2 p-2.5 rounded-lg bg-slate-950 border border-slate-800 font-mono text-xs">
              <span className="flex-1 text-slate-300 truncate">{inst.encryption_key || '••••••••••••••••••••••••••••••••'}</span>
              <button
                onClick={copyEncryptionKey}
                className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs flex items-center gap-1 transition cursor-pointer"
              >
                {copiedKey ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                {copiedKey ? 'Copied' : 'Copy'}
              </button>
            </div>
          </div>

          {/* Database Persistence Card */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-3">
            <div className="flex items-center gap-2 text-sm font-semibold text-white">
              <Database className="w-4 h-4 text-blue-400" />
              PostgreSQL Schema & Persistence
            </div>
            <div className="space-y-2 text-xs">
              <div className="flex justify-between py-1 border-b border-slate-800/60">
                <span className="text-slate-400">Database Schema:</span>
                <span className="font-mono text-white">{inst.database_name}</span>
              </div>
              <div className="flex justify-between py-1 border-b border-slate-800/60">
                <span className="text-slate-400">Storage Allocated:</span>
                <span className="font-mono text-white">{inst.storage_limit_gb} GB NVMe</span>
              </div>
              <div className="flex justify-between py-1 border-b border-slate-800/60">
                <span className="text-slate-400">Disk Used:</span>
                <span className="font-mono text-emerald-400">{inst.storage_used_gb} GB</span>
              </div>
              <div className="flex justify-between py-1">
                <span className="text-slate-400">Webhook Proxy:</span>
                <span className="font-mono text-slate-300">Enabled (TLS/HTTPS)</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {activeTab === 'backups' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-slate-200">Database Dumps & Snapshots</h3>
            <button
              onClick={handleCreateBackup}
              disabled={actionLoading}
              className="px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-medium transition flex items-center gap-1.5 cursor-pointer disabled:opacity-50"
            >
              <HardDrive className="w-3.5 h-3.5" />
              Create Backup Now
            </button>
          </div>

          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl overflow-hidden">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 uppercase tracking-wider border-b border-slate-800 font-mono">
                <tr>
                  <th className="py-3 px-4">Backup Name</th>
                  <th className="py-3 px-4">File Reference</th>
                  <th className="py-3 px-4">Size</th>
                  <th className="py-3 px-4">Status</th>
                  <th className="py-3 px-4">Created At</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-slate-300">
                {backups.length === 0 ? (
                  <tr>
                    <td colSpan={5} className="py-6 text-center text-slate-500">
                      No backups generated yet. Click "Create Backup Now" above.
                    </td>
                  </tr>
                ) : (
                  backups.map((b) => (
                    <tr key={b.id} className="hover:bg-slate-800/30 transition">
                      <td className="py-3 px-4 font-medium text-white">{b.name}</td>
                      <td className="py-3 px-4 font-mono text-slate-400">{b.backup_reference}</td>
                      <td className="py-3 px-4 font-mono">{(b.size_bytes / (1024 * 1024)).toFixed(1)} MB</td>
                      <td className="py-3 px-4">
                        <span className="capitalize px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[10px]">
                          {b.status}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-slate-500">{new Date(b.created_at).toLocaleString()}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {activeTab === 'logs' && (
        <div className="bg-slate-950 border border-slate-800 rounded-xl p-4 font-mono text-xs">
          <div className="flex items-center justify-between pb-3 border-b border-slate-800 text-slate-400 mb-3">
            <span className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
              Live n8n Node Process Logs
            </span>
            <span className="text-[11px] text-slate-500">PID 1 (Active)</span>
          </div>
          <pre className="text-emerald-400/90 whitespace-pre-wrap leading-relaxed max-h-[480px] overflow-y-auto">
            {logs.join('\n')}
          </pre>
        </div>
      )}
    </div>
  );
};
