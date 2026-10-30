import React, { useEffect, useState } from 'react';
import {
  Bot,
  Plus,
  ExternalLink,
  RotateCw,
  Play,
  Square,
  Sparkles,
  MessageSquare,
  Cpu,
  Layers,
  Shield,
  ArrowUpRight
} from 'lucide-react';
import { OpenClawInstance } from '../../types';

interface OpenClawListProps {
  onNavigate: (path: string, param?: string) => void;
}

export const OpenClawList: React.FC<OpenClawListProps> = ({ onNavigate }) => {
  const [instances, setInstances] = useState<OpenClawInstance[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const fetchInstances = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/ai-agents/openclaw/instances', {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to load AI instances');
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
            <h1 className="text-2xl font-bold tracking-tight text-white">Autonomous AI Agents</h1>
            <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-purple-500/10 text-purple-400 border border-purple-500/20 flex items-center gap-1">
              <Sparkles className="w-3 h-3" /> OpenClaw Gateway
            </span>
          </div>
          <p className="text-sm text-slate-400 mt-1">
            Personal AI assistants with persistent workspaces, multi-provider model routing, and direct Telegram & Discord bots.
          </p>
        </div>

        <button
          onClick={() => onNavigate('/dashboard/ai-agents/new')}
          className="inline-flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-purple-500/20 cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          Deploy AI Assistant
        </button>
      </div>

      {loading ? (
        <div className="p-12 text-center text-slate-500 flex flex-col items-center justify-center gap-3">
          <div className="w-8 h-8 border-2 border-purple-500 border-t-transparent rounded-full animate-spin" />
          <span className="text-sm">Loading AI agents...</span>
        </div>
      ) : instances.length === 0 ? (
        <div className="text-center py-16 px-4 rounded-xl border border-dashed border-slate-800 bg-slate-900/30">
          <Bot className="w-12 h-12 text-slate-600 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-slate-200">No AI Assistants Deployed</h3>
          <p className="text-sm text-slate-400 max-w-md mx-auto mt-1 mb-6">
            Launch your own dedicated OpenClaw instance to automate tasks, connect Telegram/Discord channels, and execute complex workflows.
          </p>
          <button
            onClick={() => onNavigate('/dashboard/ai-agents/new')}
            className="inline-flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-sm font-medium transition cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            Deploy First AI Assistant
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {instances.map((inst) => (
            <div
              key={inst.id}
              onClick={() => onNavigate('/dashboard/ai-agents', inst.id)}
              className="bg-slate-900/80 border border-slate-800/80 hover:border-slate-700/80 rounded-xl p-5 transition duration-150 cursor-pointer flex flex-col justify-between group shadow-sm hover:shadow-md"
            >
              <div>
                <div className="flex items-start justify-between gap-3 mb-3">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-lg bg-purple-500/10 border border-purple-500/20 flex items-center justify-center text-purple-400 group-hover:bg-purple-600 group-hover:text-white transition">
                      <Bot className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="text-base font-semibold text-white group-hover:text-purple-400 transition">
                        {inst.name}
                      </h3>
                      <div className="flex items-center gap-1.5 text-xs text-slate-400">
                        <span className="font-semibold text-purple-400 capitalize">{inst.ai_provider}</span>
                        <span>•</span>
                        <span className="font-mono">{inst.default_model}</span>
                      </div>
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

                {/* Gateway endpoint */}
                <div className="mt-4 p-2.5 rounded-lg bg-slate-950/60 border border-slate-800/80 text-xs">
                  <span className="text-slate-500 block mb-1">Gateway Endpoint:</span>
                  <div className="text-purple-400 font-mono flex items-center justify-between">
                    <span className="truncate">{inst.gateway_endpoint}</span>
                    <ExternalLink className="w-3 h-3 flex-shrink-0 ml-1" />
                  </div>
                </div>

                {/* Integrations channels */}
                <div className="mt-4 flex items-center gap-2 flex-wrap">
                  {inst.integrations && inst.integrations.length > 0 ? (
                    inst.integrations.map((integ) => (
                      <span
                        key={integ.id}
                        className="text-[11px] px-2 py-0.5 rounded-md bg-slate-800/80 border border-slate-700/60 text-slate-300 flex items-center gap-1"
                      >
                        <MessageSquare className="w-3 h-3 text-purple-400" />
                        <span className="capitalize">{integ.channel}:</span>
                        <span className="font-mono text-white">{integ.identifier}</span>
                      </span>
                    ))
                  ) : (
                    <span className="text-[11px] text-slate-500 italic">No channels linked</span>
                  )}
                </div>
              </div>

              <div className="mt-5 pt-4 border-t border-slate-800/60 flex items-center justify-between text-xs text-slate-500">
                <span>{inst.active_skills_count} Skills • {inst.cpu_limit} vCPU</span>
                <span className="text-slate-400 group-hover:text-purple-400 flex items-center gap-1 font-medium">
                  Configure <ArrowUpRight className="w-3 h-3" />
                </span>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
