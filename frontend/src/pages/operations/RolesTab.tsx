import React, { useEffect, useState } from 'react';
import { AlertTriangle, Bot, RefreshCw, ShieldCheck, Wrench } from 'lucide-react';
import { apiRequest } from '../../lib/api';
import type { AgentRole } from '../../types';
import { errorMessage } from './utils';

export const RolesTab: React.FC = () => {
  const [roles, setRoles] = useState<AgentRole[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiRequest<AgentRole[]>('/ai/agents');
      setRoles(res || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-white">Agent roles</h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Logical roles classify tools and tasks. They never grant permissions: the policy engine, approval manager and execution engine remain the only authority.
          </p>
        </div>
        <button
          onClick={load}
          disabled={loading}
          className="self-start p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 transition"
          title="Refresh roles"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
        </button>
      </div>

      {error && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
          <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      <div className="p-3 rounded-lg bg-blue-500/5 border border-blue-500/20 text-blue-200 text-xs flex items-start gap-2">
        <ShieldCheck className="w-4 h-4 flex-shrink-0 mt-0.5" />
        <span>
          A role is descriptive metadata. A tool call still passes through validation, risk classification and approval exactly like any other
          orchestrator task.
        </span>
      </div>

      {loading ? (
        <div className="p-12 text-center text-zinc-500 flex flex-col items-center gap-2 text-xs">
          <RefreshCw className="w-5 h-5 animate-spin" />
          Loading roles…
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
          {roles.map((role) => (
            <div key={role.id} className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800 space-y-2.5">
              <div className="flex items-start gap-2">
                <div className="w-7 h-7 rounded-md bg-zinc-950 border border-zinc-800 flex items-center justify-center flex-shrink-0">
                  <Bot className="w-3.5 h-3.5 text-zinc-300" />
                </div>
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <h3 className="text-xs font-semibold text-white">{role.name}</h3>
                    <span className="text-[9px] font-mono text-zinc-500">{role.id}</span>
                  </div>
                  <p className="text-[11px] text-zinc-400 mt-0.5">{role.description}</p>
                </div>
              </div>
              <div>
                <div className="flex items-center gap-1.5 text-[10px] uppercase font-mono text-zinc-500 mb-1">
                  <Wrench className="w-3 h-3" />
                  Tools ({role.tools.length})
                </div>
                {role.tools.length === 0 ? (
                  <div className="text-[10px] text-zinc-500">No registry tools are bound to this role.</div>
                ) : (
                  <div className="flex flex-wrap gap-1">
                    {role.tools.map((tool) => (
                      <span
                        key={tool}
                        className="px-1.5 py-0.5 rounded bg-zinc-950 border border-zinc-800 text-[9px] font-mono text-zinc-300"
                      >
                        {tool}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};
