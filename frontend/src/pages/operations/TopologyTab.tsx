import React, { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  ArrowRight,
  Boxes,
  Network,
  RefreshCw,
  Stethoscope,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import type { TopologyEdge, TopologyNode, TopologyView } from '../../types';
import { errorMessage, formatDate, resourceTypeLabel, statusClasses } from './utils';

interface TopologyTabProps {
  onNavigate: (path: string, param?: string) => void;
}

const TYPE_ORDER = ['vps', 'app', 'database', 'n8n', 'openclaw', 'game_server', 'android', 'bucket', 'domain'];

export const TopologyTab: React.FC<TopologyTabProps> = ({ onNavigate }) => {
  const [view, setView] = useState<TopologyView | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiRequest<TopologyView>('/ai/topology');
      setView(res);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const grouped = useMemo(() => {
    const groups = new Map<string, TopologyNode[]>();
    for (const node of view?.nodes || []) {
      const list = groups.get(node.type) || [];
      list.push(node);
      groups.set(node.type, list);
    }
    return Array.from(groups.entries()).sort(([a], [b]) => {
      const ai = TYPE_ORDER.indexOf(a);
      const bi = TYPE_ORDER.indexOf(b);
      return (ai === -1 ? 99 : ai) - (bi === -1 ? 99 : bi);
    });
  }, [view]);

  const nodeName = (edge: TopologyEdge) =>
    view?.nodes.find((node) => node.id === edge.to)?.name || edge.to_name || edge.to;

  if (loading) {
    return (
      <div className="p-12 text-center text-zinc-500 flex flex-col items-center gap-2 text-xs">
        <RefreshCw className="w-5 h-5 animate-spin" />
        Building topology…
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-white">Resource topology</h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Live resources plus the relationships recorded by successful orchestrator executions.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-mono text-zinc-500">generated {formatDate(view?.generated_at)}</span>
          <button
            onClick={load}
            className="p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 transition"
            title="Refresh topology"
          >
            <RefreshCw className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
          <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Nodes</div>
          <div className="text-xl font-semibold text-white mt-1">{(view?.nodes || []).length}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Relationships</div>
          <div className="text-xl font-semibold text-white mt-1">{(view?.edges || []).length}</div>
        </div>
        <div className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
          <div className="text-[10px] uppercase font-mono text-zinc-400">Resource types</div>
          <div className="text-xl font-semibold text-white mt-1">{grouped.length}</div>
        </div>
      </div>

      {(view?.nodes || []).length === 0 ? (
        <div className="text-center py-14 px-4 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20">
          <Network className="w-10 h-10 text-zinc-600 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-zinc-200">No resources in the graph</h3>
          <p className="text-xs text-zinc-400 mt-1">Create a resource or deploy a template to populate the topology.</p>
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            {grouped.map(([type, nodes]) => (
              <div key={type} className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800">
                <div className="flex items-center gap-2 mb-2">
                  <Boxes className="w-3.5 h-3.5 text-zinc-400" />
                  <span className="text-xs font-semibold text-white">{resourceTypeLabel(type)}</span>
                  <span className="text-[10px] font-mono text-zinc-500">({nodes.length})</span>
                </div>
                <div className="space-y-1.5">
                  {nodes.map((node) => (
                    <div key={node.id} className="flex items-center justify-between gap-2 p-2 rounded-md bg-zinc-950 border border-zinc-850">
                      <div className="min-w-0">
                        <div className="text-[11px] text-zinc-200 truncate">{node.name}</div>
                        <div className="text-[9px] font-mono text-zinc-500 truncate">
                          {Object.entries(node.metadata || {})
                            .filter(([, value]) => value !== null && value !== undefined && value !== '')
                            .map(([key, value]) => `${key}: ${String(value)}`)
                            .join(' · ') || node.id}
                        </div>
                      </div>
                      <div className="flex items-center gap-1.5 flex-shrink-0">
                        <span className={`px-1.5 py-0.5 rounded border text-[9px] font-mono ${statusClasses(node.status)}`}>
                          {node.status}
                        </span>
                        <button
                          onClick={() =>
                            onNavigate(
                              `/dashboard/operations?tab=diagnostics&type=${encodeURIComponent(type)}&id=${encodeURIComponent(node.id)}`
                            )
                          }
                          className="p-1 rounded bg-zinc-900 hover:bg-zinc-800 text-zinc-400 hover:text-white border border-zinc-800"
                          title="Diagnose resource"
                        >
                          <Stethoscope className="w-3 h-3" />
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>

          <div className="rounded-xl border border-zinc-800 overflow-hidden">
            <div className="px-3 py-2 border-b border-zinc-800 bg-zinc-950/60 text-[10px] uppercase font-mono text-zinc-400">
              Relationships
            </div>
            {(view?.edges || []).length === 0 ? (
              <div className="px-3 py-6 text-center text-xs text-zinc-500">
                No relationships recorded yet. Successful plans link applications to databases, buckets and hosts.
              </div>
            ) : (
              <div className="divide-y divide-zinc-850">
                {(view?.edges || []).map((edge, index) => (
                  <div key={`${edge.from}:${edge.to}:${index}`} className="px-3 py-2 flex flex-wrap items-center gap-1.5 text-[10px] font-mono">
                    <span className="text-zinc-200">
                      {view?.nodes.find((node) => node.id === edge.from)?.name || edge.from}
                    </span>
                    <span className="text-zinc-500">({resourceTypeLabel(edge.from_type)})</span>
                    <ArrowRight className="w-3 h-3 text-zinc-500" />
                    <span className="px-1.5 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-zinc-400">
                      {edge.relation}
                    </span>
                    <ArrowRight className="w-3 h-3 text-zinc-500" />
                    <span className="text-zinc-200">{nodeName(edge)}</span>
                    <span className="text-zinc-500">({resourceTypeLabel(edge.to_type)})</span>
                  </div>
                ))}
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
};
