import React, { useState } from 'react';
import {
  Bot,
  ArrowLeft,
  CheckCircle2,
  AlertTriangle,
  Sparkles,
  Key,
  Layers,
  Cpu,
  ShieldAlert
} from 'lucide-react';

interface CreateOpenClawProps {
  onNavigate: (path: string, param?: string) => void;
}

export const CreateOpenClaw: React.FC<CreateOpenClawProps> = ({ onNavigate }) => {
  const [name, setName] = useState('');
  const [version, setVersion] = useState('2.4.0');
  const [provider, setProvider] = useState('openai');
  const [model, setModel] = useState('gpt-4o');
  const [apiKey, setApiKey] = useState('');
  const [cpuLimit, setCpuLimit] = useState(2);
  const [memoryMB, setMemoryMB] = useState(4096);
  const [storageGB, setStorageGB] = useState(30);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const modelOptions: Record<string, string[]> = {
    openai: ['gpt-4o', 'gpt-4o-mini', 'o3-mini', 'chatgpt-4o-latest'],
    anthropic: ['claude-3-5-sonnet-latest', 'claude-3-5-haiku-latest', 'claude-3-opus-latest'],
    gemini: ['gemini-2.0-flash', 'gemini-1.5-pro', 'gemini-1.5-flash'],
    deepseek: ['deepseek-chat', 'deepseek-reasoner'],
    ollama: ['llama3.3:70b', 'qwen2.5:32b', 'deepseek-r1:14b', 'mistral:latest']
  };

  const handleProviderChange = (p: string) => {
    setProvider(p);
    setModel(modelOptions[p][0]);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError('Assistant name is required');
      return;
    }

    try {
      setLoading(true);
      setError(null);
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/ai-agents/openclaw/instances', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          name: name.trim(),
          version,
          ai_provider: provider,
          default_model: model,
          api_key: apiKey.trim(),
          cpu_limit: cpuLimit,
          memory_limit_mb: memoryMB,
          storage_limit_gb: storageGB
        })
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to deploy AI Assistant');

      onNavigate('/dashboard/ai-agents', data.data.id);
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
          onClick={() => onNavigate('/dashboard/ai-agents')}
          className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-400 hover:text-white hover:bg-slate-800 transition cursor-pointer"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>
        <div>
          <h1 className="text-xl font-bold text-white tracking-tight">Deploy OpenClaw AI Assistant</h1>
          <p className="text-xs text-slate-400">Personal autonomous agent sandbox with webhook & bot integrations.</p>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Step 1: Identity & Version */}
        <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4">
          <h2 className="text-sm font-semibold text-slate-200 uppercase tracking-wider flex items-center gap-2">
            <Bot className="w-4 h-4 text-purple-400" />
            1. Assistant Identity
          </h2>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">Assistant Name</label>
            <input
              type="text"
              required
              placeholder="e.g. Chief Research Assistant"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-purple-500"
            />
            <span className="text-[11px] text-slate-500 mt-1 block">
              Gateway Endpoint URL:{' '}
              <span className="text-purple-400 font-mono">
                {name ? `agent-${name.toLowerCase().replace(/\s+/g, '-')}.apps.nevatal.id/gateway` : 'agent-name.apps.nevatal.id/gateway'}
              </span>
            </span>
          </div>
        </div>

        {/* Step 2: Model Routing */}
        <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4">
          <h2 className="text-sm font-semibold text-slate-200 uppercase tracking-wider flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-purple-400" />
            2. Primary LLM Provider & Routing
          </h2>

          <div className="grid grid-cols-2 sm:grid-cols-5 gap-2.5">
            {[
              { id: 'openai', name: 'OpenAI' },
              { id: 'anthropic', name: 'Anthropic' },
              { id: 'gemini', name: 'Google Gemini' },
              { id: 'deepseek', name: 'DeepSeek' },
              { id: 'ollama', name: 'Ollama Local' }
            ].map((p) => (
              <button
                type="button"
                key={p.id}
                onClick={() => handleProviderChange(p.id)}
                className={`p-3 rounded-lg border text-center transition cursor-pointer ${
                  provider === p.id
                    ? 'bg-purple-600/10 border-purple-500 text-white font-medium shadow-sm'
                    : 'bg-slate-950/40 border-slate-800/80 text-slate-400 hover:border-slate-700'
                }`}
              >
                <div className="text-xs">{p.name}</div>
              </button>
            ))}
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 pt-2">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Default Model</label>
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-purple-500 font-mono"
              >
                {modelOptions[provider]?.map((m) => (
                  <option key={m} value={m}>
                    {m}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Provider API Key</label>
              <input
                type="password"
                placeholder={provider === 'ollama' ? 'Optional for local Ollama' : 'sk-...'}
                value={apiKey}
                onChange={(e) => setApiKey(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-purple-500 font-mono"
              />
            </div>
          </div>
        </div>

        {/* Step 3: Resources */}
        <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4">
          <h2 className="text-sm font-semibold text-slate-200 uppercase tracking-wider flex items-center gap-2">
            <Cpu className="w-4 h-4 text-purple-400" />
            3. Workspace Sandbox Specs
          </h2>

          <div className="grid grid-cols-3 gap-4">
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
                <option value={2048}>2 GB</option>
                <option value={4096}>4 GB (Recommended)</option>
                <option value={8192}>8 GB</option>
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Storage</label>
              <select
                value={storageGB}
                onChange={(e) => setStorageGB(parseInt(e.target.value))}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white"
              >
                <option value={20}>20 GB NVMe</option>
                <option value={30}>30 GB NVMe (Recommended)</option>
                <option value={60}>60 GB NVMe</option>
              </select>
            </div>
          </div>
        </div>

        {/* Submit */}
        <div className="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={() => onNavigate('/dashboard/ai-agents')}
            className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-sm font-medium transition cursor-pointer"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="px-6 py-2 bg-purple-600 hover:bg-purple-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-purple-500/20 flex items-center gap-2 cursor-pointer"
          >
            {loading ? (
              <>
                <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                <span>Initializing Agent...</span>
              </>
            ) : (
              <>
                <CheckCircle2 className="w-4 h-4" />
                <span>Deploy AI Assistant</span>
              </>
            )}
          </button>
        </div>
      </form>
    </div>
  );
};
