import React, { useState } from 'react';
import {
  Workflow,
  ArrowLeft,
  CheckCircle2,
  AlertTriangle,
  ShieldCheck,
  Database,
  Cpu,
  Layers
} from 'lucide-react';

interface CreateAutomationProps {
  onNavigate: (path: string, param?: string) => void;
}

export const CreateAutomation: React.FC<CreateAutomationProps> = ({ onNavigate }) => {
  const [name, setName] = useState('');
  const [n8nVersion, setN8nVersion] = useState('1.71.0');
  const [cpuLimit, setCpuLimit] = useState(2);
  const [memoryMB, setMemoryMB] = useState(2048);
  const [storageGB, setStorageGB] = useState(20);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError('Instance name is required');
      return;
    }

    try {
      setLoading(true);
      setError(null);
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/automation/instances', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          name: name.trim(),
          n8n_version: n8nVersion,
          cpu_limit: cpuLimit,
          memory_limit_mb: memoryMB,
          storage_limit_gb: storageGB
        })
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to deploy n8n instance');

      onNavigate('/dashboard/automation', data.data.id);
    } catch (err: any) {
      setError(err.message);
      setLoading(false);
    }
  };

  return (
    <div className="max-w-3xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4 border-b border-slate-800 pb-4">
        <button
          onClick={() => onNavigate('/dashboard/automation')}
          className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-400 hover:text-white hover:bg-slate-800 transition cursor-pointer"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>
        <div>
          <h1 className="text-xl font-bold text-white tracking-tight">Deploy Dedicated n8n Instance</h1>
          <p className="text-xs text-slate-400">Full-featured workflow automation with isolated execution engine.</p>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Info notice */}
      <div className="p-4 rounded-xl bg-emerald-500/5 border border-emerald-500/20 text-xs text-emerald-300 space-y-1">
        <div className="font-semibold flex items-center gap-2 text-emerald-400">
          <ShieldCheck className="w-4 h-4" />
          Fully Managed Persistence & Security
        </div>
        <p className="text-slate-400 leading-relaxed">
          Every n8n deployment automatically provisions a dedicated PostgreSQL database schema, an isolated webhook proxy endpoint, and persistent AES-256 encryption keys for API credentials.
        </p>
      </div>

      <form onSubmit={handleSubmit} className="space-y-6">
        <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">Instance Name</label>
            <input
              type="text"
              required
              placeholder="e.g. Production Automations"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-emerald-500"
            />
            <span className="text-[11px] text-slate-500 mt-1 block">
              Assigned Access URL:{' '}
              <span className="text-emerald-400 font-mono">
                {name ? `automation-${name.toLowerCase().replace(/\s+/g, '-')}.apps.nevatal.id` : 'automation-your-name.apps.nevatal.id'}
              </span>
            </span>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">n8n Engine Version</label>
            <select
              value={n8nVersion}
              onChange={(e) => setN8nVersion(e.target.value)}
              className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-emerald-500"
            >
              <option value="1.71.0">v1.71.0 (Latest Stable - Recommended)</option>
              <option value="1.68.2">v1.68.2 (LTS)</option>
              <option value="1.65.0">v1.65.0 (Standard)</option>
            </select>
          </div>

          <div className="grid grid-cols-3 gap-4 pt-2">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">vCPU Limit</label>
              <select
                value={cpuLimit}
                onChange={(e) => setCpuLimit(parseInt(e.target.value))}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white"
              >
                <option value={1}>1 Core</option>
                <option value={2}>2 Cores (Recommended)</option>
                <option value={4}>4 Cores</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">RAM Limit</label>
              <select
                value={memoryMB}
                onChange={(e) => setMemoryMB(parseInt(e.target.value))}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white"
              >
                <option value={1024}>1 GB</option>
                <option value={2048}>2 GB (Recommended)</option>
                <option value={4096}>4 GB</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Storage</label>
              <select
                value={storageGB}
                onChange={(e) => setStorageGB(parseInt(e.target.value))}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white"
              >
                <option value={10}>10 GB NVMe</option>
                <option value={20}>20 GB NVMe</option>
                <option value={50}>50 GB NVMe</option>
              </select>
            </div>
          </div>
        </div>

        <div className="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={() => onNavigate('/dashboard/automation')}
            className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-sm font-medium transition cursor-pointer"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="px-6 py-2 bg-emerald-600 hover:bg-emerald-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-emerald-500/20 flex items-center gap-2 cursor-pointer"
          >
            {loading ? (
              <>
                <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                <span>Provisioning n8n...</span>
              </>
            ) : (
              <>
                <CheckCircle2 className="w-4 h-4" />
                <span>Deploy n8n Instance</span>
              </>
            )}
          </button>
        </div>
      </form>
    </div>
  );
};
