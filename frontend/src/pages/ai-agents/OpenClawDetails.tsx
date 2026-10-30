import React, { useEffect, useState } from 'react';
import {
  Bot,
  ArrowLeft,
  ExternalLink,
  Play,
  Square,
  RotateCw,
  Copy,
  Check,
  Sparkles,
  MessageSquare,
  Key,
  Shield,
  Terminal,
  Trash2,
  HardDrive,
  Plus
} from 'lucide-react';
import { OpenClawInstance, OpenClawIntegration, OpenClawBackup } from '../../types';

interface OpenClawDetailsProps {
  instanceId: string;
  onNavigate: (path: string, param?: string) => void;
}

export const OpenClawDetails: React.FC<OpenClawDetailsProps> = ({ instanceId, onNavigate }) => {
  const [inst, setInst] = useState<OpenClawInstance | null>(null);
  const [integrations, setIntegrations] = useState<OpenClawIntegration[]>([]);
  const [backups, setBackups] = useState<OpenClawBackup[]>([]);
  const [logs, setLogs] = useState<string[]>([]);
  const [activeTab, setActiveTab] = useState<'gateway' | 'config' | 'channels' | 'logs'>('gateway');
  const [loading, setLoading] = useState(true);
  const [actionLoading, setActionLoading] = useState(false);
  const [copiedToken, setCopiedToken] = useState(false);

  // New integration form
  const [newChannel, setNewChannel] = useState<'telegram' | 'discord' | 'slack'>('telegram');
  const [newIdentifier, setNewIdentifier] = useState('');

  // Config form
  const [provider, setProvider] = useState('');
  const [model, setModel] = useState('');
  const [apiKey, setApiKey] = useState('');

  const fetchInstanceData = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/ai-agents/openclaw/instances/${instanceId}`, {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to fetch OpenClaw instance');
      setInst(data.data);
      setIntegrations(data.data.integrations || []);
      setBackups(data.data.backups || []);
      setProvider(data.data.ai_provider);
      setModel(data.data.default_model);

      // Fetch logs
      const logRes = await fetch(`/api/v1/ai-agents/openclaw/instances/${instanceId}/logs`, {
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
      const res = await fetch(`/api/v1/ai-agents/openclaw/instances/${instanceId}/${action}`, {
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

  const handleAddChannel = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newIdentifier.trim()) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/ai-agents/openclaw/instances/${instanceId}/integrations`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          channel: newChannel,
          identifier: newIdentifier.trim()
        })
      });
      if (!res.ok) throw new Error('Failed to link channel');
      setNewIdentifier('');
      await fetchInstanceData();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleDeleteChannel = async (channelId: string) => {
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/ai-agents/openclaw/instances/${instanceId}/integrations/${channelId}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to remove channel');
      await fetchInstanceData();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleUpdateConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/ai-agents/openclaw/instances/${instanceId}/config`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          provider,
          model,
          api_key: apiKey
        })
      });
      if (!res.ok) throw new Error('Failed to update config');
      alert('Model configuration updated successfully');
      setApiKey('');
      await fetchInstanceData();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const copyToken = () => {
    if (inst?.gateway_auth_token) {
      navigator.clipboard.writeText(inst.gateway_auth_token);
      setCopiedToken(true);
      setTimeout(() => setCopiedToken(false), 2000);
    }
  };

  if (loading) {
    return (
      <div className="p-12 text-center text-slate-500 flex flex-col items-center justify-center gap-3">
        <div className="w-8 h-8 border-2 border-purple-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-sm">Loading AI Assistant...</span>
      </div>
    );
  }

  if (!inst) {
    return (
      <div className="p-6 rounded-xl bg-red-500/10 border border-red-500/20 text-red-400">
        <h3 className="font-semibold mb-2">Agent Not Found</h3>
        <button
          onClick={() => onNavigate('/dashboard/ai-agents')}
          className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-white rounded-lg text-xs"
        >
          Back to AI Agents
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Top Bar Navigation */}
      <div className="flex items-center gap-3 text-xs text-slate-400">
        <button
          onClick={() => onNavigate('/dashboard/ai-agents')}
          className="hover:text-white flex items-center gap-1 cursor-pointer"
        >
          <ArrowLeft className="w-3.5 h-3.5" /> AI Agents
        </button>
        <span>/</span>
        <span className="text-slate-200 font-medium">{inst.name}</span>
      </div>

      {/* Hero Overview */}
      <div className="bg-slate-900/80 border border-slate-800/80 rounded-xl p-6">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="flex items-start gap-4">
            <div className="w-12 h-12 rounded-xl bg-purple-600/10 border border-purple-500/20 flex items-center justify-center text-purple-400">
              <Bot className="w-6 h-6" />
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
                <span className="font-semibold text-purple-400 capitalize">{inst.ai_provider}</span>
                <span className="text-slate-600">•</span>
                <span className="font-mono text-slate-300">{inst.default_model}</span>
                <span className="text-slate-600">•</span>
                <span className="text-slate-400">{inst.cpu_limit} vCPU / {(inst.memory_limit_mb / 1024).toFixed(1)} GB RAM</span>
                <span className="text-slate-600">•</span>
                <span className="text-slate-400">{inst.active_skills_count} Tools Loaded</span>
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            {inst.status === 'running' ? (
              <button
                onClick={() => handlePower('stop')}
                disabled={actionLoading}
                className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
                title="Stop agent"
              >
                <Square className="w-4 h-4 text-amber-400" />
              </button>
            ) : (
              <button
                onClick={() => handlePower('start')}
                disabled={actionLoading}
                className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
                title="Start agent"
              >
                <Play className="w-4 h-4 text-emerald-400" />
              </button>
            )}

            <button
              onClick={() => handlePower('restart')}
              disabled={actionLoading}
              className="p-2 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition cursor-pointer"
              title="Restart agent"
            >
              <RotateCw className="w-4 h-4 text-purple-400" />
            </button>
          </div>
        </div>
      </div>

      {/* Tabs */}
      <div className="border-b border-slate-800 flex items-center gap-2">
        {[
          { id: 'gateway', label: 'Gateway & API Keys', icon: Key },
          { id: 'channels', label: 'Telegram & Discord Bots', icon: MessageSquare },
          { id: 'config', label: 'Model Configuration', icon: Sparkles },
          { id: 'logs', label: 'Agent Activity Logs', icon: Terminal }
        ].map((tab) => {
          const Icon = tab.icon;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as any)}
              className={`flex items-center gap-2 px-4 py-2.5 text-xs font-medium border-b-2 transition cursor-pointer ${
                activeTab === tab.id
                  ? 'border-purple-500 text-purple-400'
                  : 'border-transparent text-slate-400 hover:text-slate-200'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {activeTab === 'gateway' && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
          {/* Gateway Card */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-3">
            <div className="flex items-center gap-2 text-sm font-semibold text-white">
              <Shield className="w-4 h-4 text-purple-400" />
              Gateway Authentication Token
            </div>
            <p className="text-xs text-slate-400 leading-relaxed">
              Use this bearer token to authenticate remote requests, CLI interactions, and automated cron jobs to your OpenClaw agent instance.
            </p>
            <div className="flex items-center gap-2 p-2.5 rounded-lg bg-slate-950 border border-slate-800 font-mono text-xs">
              <span className="flex-1 text-slate-300 truncate">{inst.gateway_auth_token || 'gw_sec_••••••••••••••••'}</span>
              <button
                onClick={copyToken}
                className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs flex items-center gap-1 transition cursor-pointer"
              >
                {copiedToken ? <Check className="w-3.5 h-3.5 text-purple-400" /> : <Copy className="w-3.5 h-3.5" />}
                {copiedToken ? 'Copied' : 'Copy'}
              </button>
            </div>
          </div>

          {/* cURL Example */}
          <div className="bg-slate-950 border border-slate-800 rounded-xl p-4 font-mono text-xs">
            <span className="text-slate-500 block mb-2 font-sans font-semibold">Example cURL Request:</span>
            <pre className="text-purple-400/90 whitespace-pre-wrap leading-relaxed">
{`curl -X POST ${inst.gateway_endpoint} \\
  -H "Authorization: Bearer ${inst.gateway_auth_token || '$TOKEN'}" \\
  -H "Content-Type: application/json" \\
  -d '{"prompt": "Run weekly server performance audit"}'`}
            </pre>
          </div>
        </div>
      )}

      {activeTab === 'channels' && (
        <div className="space-y-5">
          <form onSubmit={handleAddChannel} className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-4">
            <h3 className="text-xs font-semibold text-slate-200 uppercase tracking-wider mb-3">
              Connect External Channel
            </h3>
            <div className="flex flex-col sm:flex-row items-center gap-3">
              <select
                value={newChannel}
                onChange={(e) => setNewChannel(e.target.value as any)}
                className="w-full sm:w-44 px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
              >
                <option value="telegram">Telegram Bot</option>
                <option value="discord">Discord Bot</option>
                <option value="slack">Slack Workspace</option>
              </select>
              <input
                type="text"
                placeholder={newChannel === 'telegram' ? '@YourBotName or Bot Token' : 'Server Channel / Webhook ID'}
                required
                value={newIdentifier}
                onChange={(e) => setNewIdentifier(e.target.value)}
                className="w-full sm:flex-1 px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
              />
              <button
                type="submit"
                className="w-full sm:w-auto px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-xs font-medium transition cursor-pointer"
              >
                Link Channel
              </button>
            </div>
          </form>

          {/* Channels list */}
          <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl overflow-hidden">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 uppercase tracking-wider border-b border-slate-800 font-mono">
                <tr>
                  <th className="py-3 px-4">Channel</th>
                  <th className="py-3 px-4">Identifier / Handle</th>
                  <th className="py-3 px-4">Status</th>
                  <th className="py-3 px-4 text-right">Action</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-slate-300">
                {integrations.length === 0 ? (
                  <tr>
                    <td colSpan={4} className="py-6 text-center text-slate-500">
                      No channels connected yet. Link Telegram or Discord above to chat with your agent.
                    </td>
                  </tr>
                ) : (
                  integrations.map((integ) => (
                    <tr key={integ.id} className="hover:bg-slate-800/30 transition">
                      <td className="py-3 px-4 capitalize font-medium text-white flex items-center gap-2">
                        <MessageSquare className="w-3.5 h-3.5 text-purple-400" />
                        {integ.channel}
                      </td>
                      <td className="py-3 px-4 font-mono text-slate-300">{integ.identifier}</td>
                      <td className="py-3 px-4">
                        <span className="capitalize px-2 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-[10px]">
                          {integ.status}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-right">
                        <button
                          onClick={() => handleDeleteChannel(integ.id)}
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

      {activeTab === 'config' && (
        <form onSubmit={handleUpdateConfig} className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4 max-w-xl">
          <h3 className="text-sm font-semibold text-slate-200">Update Primary LLM Provider</h3>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">Provider</label>
            <select
              value={provider}
              onChange={(e) => setProvider(e.target.value)}
              className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
            >
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="gemini">Google Gemini</option>
              <option value="deepseek">DeepSeek</option>
              <option value="ollama">Ollama Local</option>
            </select>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">Model Identifier</label>
            <input
              type="text"
              required
              value={model}
              onChange={(e) => setModel(e.target.value)}
              className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">Update API Key (Optional)</label>
            <input
              type="password"
              placeholder="Leave empty to keep existing key"
              value={apiKey}
              onChange={(e) => setApiKey(e.target.value)}
              className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
            />
          </div>

          <button
            type="submit"
            className="px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-xs font-medium transition cursor-pointer"
          >
            Save Model Settings
          </button>
        </form>
      )}

      {activeTab === 'logs' && (
        <div className="bg-slate-950 border border-slate-800 rounded-xl p-4 font-mono text-xs">
          <div className="flex items-center justify-between pb-3 border-b border-slate-800 text-slate-400 mb-3">
            <span className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-purple-400 animate-pulse" />
              OpenClaw Agent Core Daemon Logs
            </span>
            <span className="text-[11px] text-slate-500">Live Gateway</span>
          </div>
          <pre className="text-purple-300/90 whitespace-pre-wrap leading-relaxed max-h-[480px] overflow-y-auto">
            {logs.join('\n')}
          </pre>
        </div>
      )}
    </div>
  );
};
