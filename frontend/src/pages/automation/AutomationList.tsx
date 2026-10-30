import React, { useEffect, useState } from 'react';
import {
  Workflow,
  Plus,
  ExternalLink,
  RotateCw,
  Play,
  Square,
  HardDrive,
  Activity,
  Layers,
  ShieldCheck,
  Cpu,
  Trash2,
  ArrowUpRight
} from 'lucide-react';
import { AutomationInstance } from '../../types';

interface AutomationListProps {
  onNavigate: (path: string, param?: string) => void;
}

export const AutomationList: React.FC<AutomationListProps> = ({ onNavigate }) => {
  const [instances, setInstances] = useState<AutomationInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchInstances = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/automation/instances', {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to load automation instances');
      setInstances(data.data || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchInstances();
  }, []);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-800 pb-5">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-white">Workflow Automation</h1>
            <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
              n8n Powered
            </span>
          </div>
          <p className="text-sm text-slate-400 mt-1">
            Dedicated n8n orchestration instances backed by persistent PostgreSQL and automatic daily backups.
          </p>
        </div>

        <button
          onClick={() => onNavigate('/dashboard/automation/new')}
          className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-emerald-500/20 cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          Deploy n8n Instance
        </button>
      </div>

      {loading ? (
        <div className="p-12 text-center text-slate-500 flex flex-col items-center justify-center gap-3">
          <div className="w-8 h-8 border-2 border-emerald-500 border-t-transparent rounded-full animate-spin" />
          <span className="text-sm">Loading automation environments...</span>
        </div>
      ) : instances.length === 0 ? (
        <div className="text-center py-16 px-4 rounded-xl border border-dashed border-slate-800 bg-slate-900/30">
          <Workflow className="w-12 h-12 text-slate-600 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-slate-200">No automation instances active</h3>
          <p className="text-sm text-slate-400 max-w-md mx-auto mt-1 mb-6">
            Spin up a production-ready n8n environment with integrated database storage and webhook listeners.
          </p>
          <button
            onClick={() => onNavigate('/dashboard/automation/new')}
            className="inline-flex items-center gap-2 px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-sm font-medium transition cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            Deploy First n8n Instance
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {instances.map((inst) => (
            <div
              key={inst.id}
              onClick={() => onNavigate('/dashboard/automation', inst.id)}
              className="bg-slate-900/80 border border-slate-800/80 hover:border-slate-700/80 rounded-xl p-5 transition duration-150 cursor-pointer flex flex-col justify-between group shadow-sm hover:shadow-md"
            >
              <div>
                <div className="flex items-start justify-between gap-3 mb-3">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-lg bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 group-hover:bg-emerald-600 group-hover:text-white transition">
                      <Workflow className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="text-base font-semibold text-white group-hover:text-emerald-400 transition">
                        {inst.name}
                      </h3>
                      <span className="text-xs font-mono text-slate-400">
                        v{inst.n8n_version}
                      </span>
                    </div>
                  </div>

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

                {/* Dashboard URL */}
                <div className="mt-4 p-2.5 rounded-lg bg-slate-950/60 border border-slate-800/80 text-xs">
                  <span className="text-slate-500 block mb-1">Access URL:</span>
                  <a
                    href={inst.dashboard_url}
                    target="_blank"
                    rel="noreferrer"
                    onClick={(e) => e.stopPropagation()}
                    className="text-emerald-400 hover:text-emerald-300 font-mono flex items-center gap-1 truncate"
                  >
                    {inst.dashboard_url}
                    <ExternalLink className="w-3 h-3 flex-shrink-0" />
                  </a>
                </div>

                {/* Stats */}
                <div className="grid grid-cols-2 gap-2 mt-4 text-xs">
                  <div className="p-2 rounded bg-slate-950/40 border border-slate-800/60">
                    <span className="text-slate-500 block">Workflows</span>
                    <span className="font-semibold text-slate-200">{inst.active_workflows_count} Active</span>
                  </div>
                  <div className="p-2 rounded bg-slate-950/40 border border-slate-800/60">
                    <span className="text-slate-500 block">Executions</span>
                    <span className="font-semibold text-slate-200">{inst.total_executions} Total</span>
                  </div>
                </div>
              </div>

              <div className="mt-5 pt-4 border-t border-slate-800/60 flex items-center justify-between text-xs text-slate-500">
                <span>{inst.cpu_limit} vCPU • {(inst.memory_limit_mb / 1024).toFixed(1)} GB RAM</span>
                <span className="text-slate-400 group-hover:text-emerald-400 flex items-center gap-1 font-medium">
                  Open Console <ArrowUpRight className="w-3 h-3" />
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
