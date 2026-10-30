import React, { useEffect, useState } from 'react';
import {
  Globe,
  ArrowLeft,
  ExternalLink,
  Play,
  Square,
  RotateCw,
  RefreshCw,
  Terminal,
  History,
  Lock,
  Plus,
  Trash2,
  CheckCircle2,
  AlertTriangle,
  GitBranch,
  Clock,
  Eye,
  EyeOff
} from 'lucide-react';
import { Application, Deployment, ApplicationEnvVar } from '../../types';

interface AppDetailsProps {
  appId: string;
  onNavigate: (path: string, param?: string) => void;
}

export const AppDetails: React.FC<AppDetailsProps> = ({ appId, onNavigate }) => {
  const [app, setApp] = useState<Application | null>(null);
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [envVars, setEnvVars] = useState<ApplicationEnvVar[]>([]);
  const [activeTab, setActiveTab] = useState<'deployments' | 'logs' | 'env' | 'settings'>('deployments');
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState(false);

  // New Env Var State
  const [newKey, setNewKey] = useState('');
  const [newVal, setNewVal] = useState('');
  const [isSecret, setIsSecret] = useState(false);

  const fetchAppData = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/apps/${appId}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to fetch application');
      setApp(data.data);
      setDeployments(data.data.deployments || []);
      setEnvVars(data.data.env_vars || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAppData();
  }, [appId]);

  const handlePower = async (action: 'start' | 'stop' | 'restart') => {
    try {
      setActionLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/apps/${appId}/power/${action}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error(`Failed to ${action} application`);
      await fetchAppData();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setActionLoading(false);
    }
  };

  const handleDeploy = async () => {
    try {
      setActionLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/apps/${appId}/deploy`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({ commit_message: 'Dashboard manual trigger' })
      });
      if (!res.ok) throw new Error('Failed to trigger deployment');
      await fetchAppData();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setActionLoading(false);
    }
  };

  const handleRollback = async (deploymentId: string) => {
    if (!confirm('Are you sure you want to rollback to this previous build?')) return;
    try {
      setActionLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/apps/${appId}/rollback/${deploymentId}`, {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Rollback failed');
      await fetchAppData();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setActionLoading(false);
    }
  };

  const handleSaveEnvVar = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newKey.trim()) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/apps/${appId}/env`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          key: newKey.trim(),
          value: newVal,
          is_secret: isSecret
        })
      });
      if (!res.ok) throw new Error('Failed to save environment variable');
      setNewKey('');
      setNewVal('');
      setIsSecret(false);
      await fetchAppData();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleDeleteEnvVar = async (key: string) => {
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/apps/${appId}/env/${key}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to delete variable');
      await fetchAppData();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleDeleteApp = async () => {
    if (!confirm(`Are you sure you want to delete ${app?.name}? This cannot be undone.`)) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/apps/${appId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to delete application');
      onNavigate('/dashboard/apps');
    } catch (err: any) {
      alert(err.message);
    }
  };

  if (loading) {
    return (
      <div className="p-12 text-center text-slate-500 flex flex-col items-center justify-center gap-3">
        <div className="w-8 h-8 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-sm">Loading application details...</span>
      </div>
    );
  }

  if (error || !app) {
    return (
      <div className="p-6 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400">
        <h3 className="font-semibold mb-2">Error loading application</h3>
        <p className="text-sm mb-4">{error || 'Application not found'}</p>
        <button
          onClick={() => onNavigate('/dashboard/apps')}
          className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-white rounded-lg text-xs"
        >
          Back to Applications
        </button>
      </div>
    );
  }

  const activeDeployment = deployments.length > 0 ? deployments[0] : null;

  return (
    <div className="space-y-6">
      {/* Top Bar Navigation */}
      <div className="flex items-center gap-3 text-xs text-slate-400">
        <button
          onClick={() => onNavigate('/dashboard/apps')}
          className="hover:text-white flex items-center gap-1 cursor-pointer"
        >
          <ArrowLeft className="w-3.5 h-3.5" /> Applications
        </button>
        <span>/</span>
        <span className="text-slate-200 font-medium">{app.name}</span>
      </div>

      {/* Hero Overview */}
      <div className="bg-slate-900/80 border border-slate-800/80 rounded-xl p-6">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-blue-600/10 border border-blue-500/20 flex items-center justify-center text-blue-400">
              <Globe className="w-6 h-6" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h1 className="text-xl font-bold text-white tracking-tight">{app.name}</h1>
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

              <div className="flex items-center gap-3 mt-2 text-xs">
                <a
                  href={`https://${app.assigned_subdomain}`}
                  target="_blank"
                  rel="noreferrer"
                  className="text-blue-400 hover:text-blue-300 font-mono flex items-center gap-1"
                >
                  https://{app.assigned_subdomain}
                  <ExternalLink className="w-3 h-3" />
                </a>
                <span className="text-slate-600">•</span>
                <span className="font-mono text-slate-400 uppercase">{app.application_type}</span>
                <span className="text-slate-600">•</span>
                <span className="text-slate-400">{app.vcpus} vCPU / {(app.memory_mb / 1024).toFixed(1)} GB RAM</span>
              </div>
            </div>
          </div>

          {/* Action Buttons */}
          <div className="flex items-center gap-2">
            <button
              onClick={handleDeploy}
              disabled={actionLoading}
              className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-medium transition flex items-center gap-1.5 shadow-sm cursor-pointer disabled:opacity-50"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${actionLoading ? 'animate-spin' : ''}`} />
              Deploy Latest
            </button>
            {app.status === 'running' ? (
              <button
                onClick={() => handlePower('stop')}
                disabled={actionLoading}
                className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
                title="Stop application"
              >
                <Square className="w-4 h-4 text-amber-400" />
              </button>
            ) : (
              <button
                onClick={() => handlePower('start')}
                disabled={actionLoading}
                className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
                title="Start application"
              >
                <Play className="w-4 h-4 text-emerald-400" />
              </button>
            )}
            <button
              onClick={() => handlePower('restart')}
              disabled={actionLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
              title="Restart application"
            >
              <RotateCw className="w-4 h-4 text-blue-400" />
            </button>
            <button
              onClick={handleDeleteApp}
              className="p-2 rounded-lg bg-red-500/10 hover:bg-red-500/20 text-red-400 transition cursor-pointer"
              title="Delete application"
            >
              <Trash2 className="w-4 h-4" />
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-slate-800 flex items-center gap-2">
        {[
          { id: 'deployments', label: 'Deployments & Rollback', icon: History },
          { id: 'logs', label: 'Build & Runtime Logs', icon: Terminal },
          { id: 'env', label: 'Environment Variables', icon: Lock }
        ].map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as any)}
              className={`flex items-center gap-2 px-4 py-2.5 text-xs font-medium border-b-2 transition cursor-pointer ${
                activeTab === tab.id
                  ? 'border-blue-500 text-blue-400'
                  : 'border-transparent text-slate-400 hover:text-slate-200'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {/* Tab: Deployments */}
      {activeTab === 'deployments' && (
        <div className="space-y-4">
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl overflow-hidden">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 uppercase tracking-wider border-b border-slate-800 font-mono">
                <tr>
                  <th className="py-3 px-4">Deployment</th>
                  <th className="py-3 px-4">Commit / Message</th>
                  <th className="py-3 px-4">Branch</th>
                  <th className="py-3 px-4">Status</th>
                  <th className="py-3 px-4">Date</th>
                  <th className="py-3 px-4 text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-slate-300">
                {deployments.map((dep, idx) => {
                  const isCurrent = idx === 0 && dep.status === 'active';
                  return (
                    <tr key={dep.id} className="hover:bg-slate-800/30 transition">
                      <td className="py-3 px-4 font-mono text-blue-400">
                        {dep.source_commit.substring(0, 7)}
                        {isCurrent && (
                          <span className="ml-2 text-[10px] px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                            Current
                          </span>
                        )}
                      </td>
                      <td className="py-3 px-4 font-medium text-slate-200 truncate max-w-xs">
                        {dep.commit_message || 'Deployment artifact'}
                      </td>
                      <td className="py-3 px-4 font-mono text-slate-400">{dep.source_branch}</td>
                      <td className="py-3 px-4">
                        <span className="capitalize px-2 py-0.5 rounded bg-slate-800 text-slate-300">
                          {dep.status}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-slate-500">
                        {new Date(dep.created_at).toLocaleString()}
                      </td>
                      <td className="py-3 px-4 text-right">
                        {!isCurrent && (
                          <button
                            onClick={() => handleRollback(dep.id)}
                            className="px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded text-xs transition cursor-pointer"
                          >
                            Rollback to this
                          </button>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* Tab: Logs */}
      {activeTab === 'logs' && (
        <div className="bg-slate-950 border border-slate-800 rounded-xl p-4 font-mono text-xs">
          <div className="flex items-center justify-between pb-3 border-b border-slate-800 text-slate-400 mb-3">
            <span className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
              Build Output Pipeline ({activeDeployment?.source_commit.substring(0, 7) || 'live'})
            </span>
            <span className="text-[11px] text-slate-500">Auto-refresh active</span>
          </div>
          <pre className="text-emerald-400/90 whitespace-pre-wrap leading-relaxed max-h-[480px] overflow-y-auto">
            {activeDeployment?.build_logs || '[info] No deployment logs available.'}
          </pre>
        </div>
      )}

      {/* Tab: Environment Variables */}
      {activeTab === 'env' && (
        <div className="space-y-5">
          {/* Add form */}
          <form onSubmit={handleSaveEnvVar} className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-4">
            <h3 className="text-xs font-semibold text-slate-200 uppercase tracking-wider mb-3">
              Add Environment Variable
            </h3>
            <div className="flex flex-col sm:flex-row items-center gap-3">
              <input
                type="text"
                placeholder="VARIABLE_NAME"
                required
                value={newKey}
                onChange={(e) => setNewKey(e.target.value)}
                className="w-full sm:flex-1 px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono uppercase"
              />
              <input
                type="text"
                placeholder="Value"
                required
                value={newVal}
                onChange={(e) => setNewVal(e.target.value)}
                className="w-full sm:flex-1 px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
              />
              <label className="flex items-center gap-2 text-xs text-slate-400 cursor-pointer select-none">
                <input
                  type="checkbox"
                  checked={isSecret}
                  onChange={(e) => setIsSecret(e.target.checked)}
                  className="rounded bg-slate-950 border-slate-800 text-blue-500"
                />
                Secret
              </label>
              <button
                type="submit"
                className="w-full sm:w-auto px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-medium transition cursor-pointer"
              >
                Save
              </button>
            </div>
          </form>

          {/* List */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl overflow-hidden">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 uppercase tracking-wider border-b border-slate-800 font-mono">
                <tr>
                  <th className="py-3 px-4">Key</th>
                  <th className="py-3 px-4">Value</th>
                  <th className="py-3 px-4">Type</th>
                  <th className="py-3 px-4 text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-slate-300">
                {envVars.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="py-6 text-center text-slate-500">
                      No environment variables configured.
                    </td>
                  </tr>
                ) : (
                  envVars.map((env) => (
                    <tr key={env.id} className="hover:bg-slate-800/30 transition">
                      <td className="py-3 px-4 font-mono font-medium text-white">{env.key}</td>
                      <td className="py-3 px-4 font-mono text-slate-400">{env.value}</td>
                      <td className="py-3 px-4">
                        {env.is_secret ? (
                          <span className="px-2 py-0.5 rounded bg-amber-500/10 text-amber-400 border border-amber-500/20 text-[10px]">
                            Secret
                          </span>
                        ) : (
                          <span className="px-2 py-0.5 rounded bg-slate-800 text-slate-400 text-[10px]">
                            Plain Text
                          </span>
                        )}
                      </td>
                      <td className="py-3 px-4 text-right">
                        <button
                          onClick={() => handleDeleteEnvVar(env.key)}
                          className="p-1 text-slate-500 hover:text-red-400 transition cursor-pointer"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};
