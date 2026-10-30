import React, { useEffect, useState } from 'react';
import {
  Globe,
  Plus,
  GitBranch,
  ExternalLink,
  RefreshCw,
  Terminal,
  Activity,
  Layers,
  CheckCircle2,
  AlertTriangle,
  Clock,
  ArrowUpRight,
  Server
} from 'lucide-react';
import { Application } from '../../types';

interface AppsListProps {
  onNavigate: (path: string, param?: string) => void;
}

export const AppsList: React.FC<AppsListProps> = ({ onNavigate }) => {
  const [apps, setApps] = useState<Application[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);
  const [deployingId, setDeployingId] = useState<string | null>(null);

  const fetchApps = async () => {
    try {
      setLoading(true);
      setError(null);
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/apps', {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to load applications');
      setApps(data.data || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchApps();
  }, []);

  const handleTriggerDeploy = async (e: React.MouseEvent, app: Application) => {
    e.stopPropagation();
    try {
      setDeployingId(app.id);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/apps/${app.id}/deploy`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({ commit_message: 'Manual dashboard redeploy' })
      });
      if (!res.ok) throw new Error('Failed to trigger deployment');
      await fetchApps();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setDeployingId(null);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-800 pb-5">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-white">Websites & Applications</h1>
            <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20">
              {apps.length} Active
            </span>
          </div>
          <p className="text-sm text-slate-400 mt-1">
            Automated Git deployments, continuous builds, and isolated micro-containers.
          </p>
        </div>

        <button
          onClick={() => onNavigate('/dashboard/apps/new')}
          className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-blue-500/20 cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          Deploy Application
        </button>
      </div>

      {error && (
        <div className="p-4 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {loading ? (
        <div className="p-12 text-center text-slate-500 flex flex-col items-center justify-center gap-3">
          <div className="w-8 h-8 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
          <span className="text-sm">Loading applications...</span>
        </div>
      ) : apps.length === 0 ? (
        <div className="text-center py-16 px-4 rounded-xl border border-dashed border-slate-800 bg-slate-900/30">
          <Globe className="w-12 h-12 text-slate-600 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-slate-200">No applications deployed yet</h3>
          <p className="text-sm text-slate-400 max-w-md mx-auto mt-1 mb-6">
            Connect your Git repository or choose a runtime template to deploy your website automatically in seconds.
          </p>
          <button
            onClick={() => onNavigate('/dashboard/apps/new')}
            className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition"
          >
            <Plus className="w-4 h-4" />
            Deploy First Application
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {apps.map((app) => {
            const isDeploying = deployingId === app.id || app.status === 'building' || app.status === 'deploying';
            const latestDeploy = app.deployments && app.deployments.length > 0 ? app.deployments[0] : null;

            return (
              <div
                key={app.id}
                onClick={() => onNavigate('/dashboard/apps', app.id)}
                className="bg-slate-900/80 border border-slate-800/80 hover:border-slate-700/80 rounded-xl p-5 transition duration-150 cursor-pointer flex flex-col justify-between group shadow-sm hover:shadow-md"
              >
                <div>
                  <div className="flex items-start justify-between gap-3 mb-3">
                    <div className="flex items-center gap-3">
                      <div className="w-10 h-10 rounded-lg bg-blue-500/10 border border-blue-500/20 flex items-center justify-center text-blue-400 group-hover:bg-blue-600 group-hover:text-white transition">
                        <Globe className="w-5 h-5" />
                      </div>
                      <div>
                        <h3 className="text-base font-semibold text-white group-hover:text-blue-400 transition">
                          {app.name}
                        </h3>
                        <span className="text-xs font-mono text-slate-400 uppercase tracking-wider">
                          {app.application_type}
                        </span>
                      </div>
                    </div>

                    <span
                      className={`text-xs px-2.5 py-0.5 rounded-full font-medium border flex items-center gap-1.5 ${
                        app.status === 'running'
                          ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                          : app.status === 'building' || app.status === 'deploying'
                          ? 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                          : 'bg-slate-700/30 text-slate-400 border-slate-700/50'
                      }`}
                    >
                      {app.status === 'running' && <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />}
                      {app.status}
                    </span>
                  </div>

                  {/* Production URL */}
                  <div className="mt-4 p-2.5 rounded-lg bg-slate-950/60 border border-slate-800/80 text-xs">
                    <span className="text-slate-500 block mb-1">Production URL:</span>
                    <a
                      href={`https://${app.assigned_subdomain}`}
                      target="_blank"
                      rel="noreferrer"
                      onClick={(e) => e.stopPropagation()}
                      className="text-blue-400 hover:text-blue-300 font-mono flex items-center gap-1 truncate"
                    >
                      https://{app.assigned_subdomain}
                      <ExternalLink className="w-3 h-3 flex-shrink-0" />
                    </a>
                  </div>

                  {/* Git Source */}
                  <div className="mt-3 flex items-center gap-2 text-xs text-slate-400">
                    <GitBranch className="w-3.5 h-3.5 text-slate-500" />
                    <span className="font-mono text-slate-300">{app.production_branch}</span>
                    {latestDeploy && (
                      <span className="text-slate-500 text-[11px] truncate">
                        • {latestDeploy.commit_message || latestDeploy.source_commit.substring(0, 7)}
                      </span>
                    )}
                  </div>
                </div>

                <div className="mt-5 pt-4 border-t border-slate-800/60 flex items-center justify-between">
                  <div className="flex items-center gap-3 text-xs text-slate-500">
                    <span>{app.vcpus} vCPU</span>
                    <span>•</span>
                    <span>{(app.memory_mb / 1024).toFixed(1)} GB RAM</span>
                  </div>

                  <div className="flex items-center gap-2">
                    <button
                      onClick={(e) => handleTriggerDeploy(e, app)}
                      disabled={isDeploying}
                      title="Trigger new deployment"
                      className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition disabled:opacity-50"
                    >
                      <RefreshCw className={`w-4 h-4 ${isDeploying ? 'animate-spin text-blue-400' : ''}`} />
                    </button>
                    <span className="text-xs text-slate-400 font-medium group-hover:text-blue-400 flex items-center gap-1">
                      Manage <ArrowUpRight className="w-3 h-3" />
                    </span>
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
