import React, { useMemo, useState } from 'react';
import { ChevronDown, ChevronRight, RefreshCw, Search, Timer, Wrench } from 'lucide-react';
import type { OrchestratorTool, RiskLevel } from '../../types';
import { DescribedError, RISK_LEVELS, RISK_META, riskRank } from '../../lib/orchestrator';
import { ErrorNote, JsonBlock, RiskBadge, Spinner } from './OrchestratorBits';

/** Read-only view of GET /ai/tools (the registry MCP `tools/list` also serves). */
export const ToolsCatalog: React.FC<{
  tools: OrchestratorTool[] | null;
  error: DescribedError | null;
  loading: boolean;
  onReload: () => void;
}> = ({ tools, error, loading, onReload }) => {
  const [query, setQuery] = useState('');
  const [risk, setRisk] = useState<RiskLevel | 'all'>('all');
  const [open, setOpen] = useState<string | null>(null);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return (tools ?? [])
      .filter((t) => risk === 'all' || t.risk === risk)
      .filter(
        (t) =>
          !q ||
          [t.id, t.title, t.description, ...t.resource_types].some((v) => v.toLowerCase().includes(q))
      )
      .sort((a, b) => riskRank(a.risk) - riskRank(b.risk) || a.id.localeCompare(b.id));
  }, [tools, query, risk]);

  return (
    <div className="flex flex-col h-full min-h-0" data-testid="tools-catalog">
      <div className="px-3 py-2.5 border-b border-zinc-800 space-y-2">
        <div className="flex items-center justify-between gap-2">
          <div className="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-500">
            <Wrench className="w-3 h-3" />
            <span>Tool registry</span>
            {tools && <span className="text-zinc-600">({tools.length})</span>}
          </div>
          <button
            type="button"
            onClick={onReload}
            disabled={loading}
            title="Reload tools"
            className="p-1 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
        <p className="text-[10.5px] text-zinc-500 leading-relaxed">
          Read tools run immediately. Every other tool only becomes a plan that you approve here; the agent and MCP clients
          cannot approve it.
        </p>
        <div className="relative">
          <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search tools…"
            className="w-full bg-zinc-900 border border-zinc-800 rounded-md pl-8 pr-3 py-1.5 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
          />
        </div>
        <div className="flex flex-wrap gap-1">
          {(['all', ...RISK_LEVELS] as const).map((r) => (
            <button
              key={r}
              type="button"
              onClick={() => setRisk(r)}
              aria-pressed={risk === r}
              className={`px-2 py-0.5 rounded-md border text-[10.5px] font-mono transition-colors ${
                risk === r ? 'bg-zinc-800 border-zinc-700 text-white' : 'bg-zinc-900 border-zinc-800 text-zinc-400 hover:text-zinc-200'
              }`}
            >
              {r === 'all' ? 'all' : RISK_META[r].label.toLowerCase()}
            </button>
          ))}
        </div>
      </div>
      <div className="flex-1 overflow-y-auto overflow-x-hidden p-3 space-y-1.5">
        {error && <ErrorNote error={error} />}
        {loading && !tools && (
          <div className="flex items-center gap-2 text-xs text-zinc-400">
            <Spinner /> Loading tools…
          </div>
        )}
        {tools && filtered.length === 0 && <div className="text-xs text-zinc-500 text-center py-6">No tools match.</div>}
        {filtered.map((t) => {
          const expanded = open === t.id;
          return (
            <div key={t.id} className="rounded-md border border-zinc-800 bg-zinc-950/60" data-tool-id={t.id}>
              <button
                type="button"
                onClick={() => setOpen(expanded ? null : t.id)}
                aria-expanded={expanded}
                className="w-full text-left px-2 py-1.5 flex items-start gap-2"
              >
                {expanded ? (
                  <ChevronDown className="w-3.5 h-3.5 mt-0.5 text-zinc-500 flex-shrink-0" />
                ) : (
                  <ChevronRight className="w-3.5 h-3.5 mt-0.5 text-zinc-500 flex-shrink-0" />
                )}
                <div className="flex-1 min-w-0 space-y-0.5">
                  <div className="flex flex-wrap items-center gap-1.5">
                    <code className="font-mono text-[11px] text-zinc-100 break-all">{t.id}</code>
                    <RiskBadge risk={t.risk} withLabel={false} compact />
                    {t.requires_admin && (
                      <span className="text-[10px] font-mono px-1.5 py-px rounded border border-orange-500/25 bg-orange-500/10 text-orange-300">
                        admin
                      </span>
                    )}
                  </div>
                  <div className="text-[11px] text-zinc-400 break-words">{t.title}</div>
                </div>
              </button>
              {expanded && (
                <div className="border-t border-zinc-800/80 px-2 py-2 space-y-2 text-[11px]">
                  {t.description && <p className="text-zinc-300 leading-relaxed break-words">{t.description}</p>}
                  <div className="flex flex-wrap gap-x-3 gap-y-1 text-[10.5px] font-mono text-zinc-500">
                    <span>{t.idempotent ? 'idempotent' : 'not idempotent'}</span>
                    {t.timeout_seconds > 0 && (
                      <span className="inline-flex items-center gap-1">
                        <Timer className="w-3 h-3" />
                        {t.timeout_seconds}s timeout
                      </span>
                    )}
                    {t.resource_types.length > 0 && <span>types: {t.resource_types.join(', ')}</span>}
                    <span>{RISK_META[t.risk].approval}</span>
                  </div>
                  {t.input_schema !== null && t.input_schema !== undefined && (
                    <div className="space-y-1">
                      <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500">Input schema</div>
                      <JsonBlock value={t.input_schema} maxHeight="max-h-48" />
                    </div>
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};
