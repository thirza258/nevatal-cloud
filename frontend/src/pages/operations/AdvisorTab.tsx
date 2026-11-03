import React, { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  ArrowUpRight,
  BadgeDollarSign,
  CheckCircle2,
  Gauge,
  RefreshCw,
  Sparkles,
  TrendingDown,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import type { AdvisorReport, Optimization } from '../../types';
import {
  errorMessage,
  formatDate,
  formatMoney,
  resourceTypeLabel,
  riskClasses,
  statusClasses,
} from './utils';

interface AdvisorTabProps {
  onNavigate: (path: string, param?: string) => void;
}

const CATEGORY_LABELS: Record<string, string> = {
  waste: 'Waste',
  capacity: 'Capacity',
  rightsize: 'Rightsize',
  reliability: 'Reliability',
};

const categoryIcon = (category: string) => {
  switch (category) {
    case 'waste':
      return TrendingDown;
    case 'capacity':
      return Gauge;
    case 'reliability':
      return AlertTriangle;
    default:
      return Sparkles;
  }
};

export const AdvisorTab: React.FC<AdvisorTabProps> = ({ onNavigate }) => {
  const [report, setReport] = useState<AdvisorReport | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [category, setCategory] = useState<string>('all');

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiRequest<AdvisorReport>('/ai/advisor');
      setReport(res);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const optimizations = useMemo(() => {
    const list = report?.optimizations || [];
    if (category === 'all') return list;
    return list.filter((item) => item.category === category);
  }, [report, category]);

  const categories = useMemo(() => {
    const set = new Set((report?.optimizations || []).map((item) => item.category));
    return ['all', ...Array.from(set)];
  }, [report]);

  const diagnosisLink = (optimization: Optimization) => {
    if (!optimization.resource_id) return null;
    return `/dashboard/operations?tab=diagnostics&type=${encodeURIComponent(optimization.resource_type)}&id=${encodeURIComponent(optimization.resource_id)}`;
  };

  if (loading) {
    return (
      <div className="p-12 text-center text-zinc-500 flex flex-col items-center gap-2 text-xs">
        <RefreshCw className="w-5 h-5 animate-spin" />
        Analyzing usage and cost…
      </div>
    );
  }

  if (error && !report) {
    return (
      <div className="space-y-4">
        <div className="flex items-start gap-2 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
          <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
        <button
          onClick={load}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold"
        >
          <RefreshCw className="w-3.5 h-3.5" />
          Retry
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-white">Optimization advisor</h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Deterministic cost estimates and recommendations from observed usage. Every proposed change becomes an approval-bound plan.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-mono text-zinc-500">generated {formatDate(report?.generated_at)}</span>
          <button
            onClick={load}
            className="p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 transition"
            title="Refresh advisor"
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

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
        <div className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800">
          <div className="flex items-center gap-1.5 text-[10px] uppercase font-mono text-zinc-400">
            <BadgeDollarSign className="w-3.5 h-3.5 text-emerald-400" />
            Estimated monthly total
          </div>
          <div className="text-2xl font-semibold text-white mt-1">
            {formatMoney(report?.monthly_total || 0, report?.currency)}
          </div>
          <div className="text-[10px] text-zinc-500 mt-1">Rate card estimate, not an invoice.</div>
        </div>
        <div className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800">
          <div className="flex items-center gap-1.5 text-[10px] uppercase font-mono text-zinc-400">
            <CheckCircle2 className="w-3.5 h-3.5 text-blue-400" />
            Recommendations
          </div>
          <div className="text-2xl font-semibold text-white mt-1">{(report?.optimizations || []).length}</div>
          <div className="text-[10px] text-zinc-500 mt-1">Every recommendation requires review.</div>
        </div>
        <div className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800">
          <div className="flex items-center gap-1.5 text-[10px] uppercase font-mono text-zinc-400">
            <Sparkles className="w-3.5 h-3.5 text-violet-400" />
            Costed resources
          </div>
          <div className="text-2xl font-semibold text-white mt-1">{(report?.cost_resources || []).length}</div>
          <div className="text-[10px] text-zinc-500 mt-1">Including stopped storage and buckets.</div>
        </div>
      </div>

      {report?.quota && (
        <div className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800">
          <div className="text-[11px] font-semibold text-white mb-2">Quota headroom</div>
          <div className="grid grid-cols-1 sm:grid-cols-4 gap-3">
            {[
              ['vCPU', report.quota.used_vcpus, report.quota.max_vcpus, ''],
              ['Memory', report.quota.used_memory_mb, report.quota.max_memory_mb, ' MB'],
              ['Storage', report.quota.used_storage_gb, report.quota.max_storage_gb, ' GB'],
              ['Instances', report.quota.used_instances, report.quota.max_instances, ''],
            ].map(([label, used, max, suffix]) => {
              const percent = Number(max) > 0 ? Math.min(100, (Number(used) / Number(max)) * 100) : 0;
              return (
                <div key={String(label)}>
                  <div className="flex items-center justify-between text-[10px] font-mono text-zinc-400">
                    <span>{label}</span>
                    <span>
                      {String(used)}
                      {suffix} / {String(max)}
                      {suffix}
                    </span>
                  </div>
                  <div className="mt-1 h-1.5 rounded-full bg-zinc-800 overflow-hidden">
                    <div
                      className={`h-full rounded-full ${percent >= 90 ? 'bg-rose-500' : percent >= 75 ? 'bg-amber-500' : 'bg-emerald-500'}`}
                      style={{ width: `${percent}%` }}
                    />
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      <div className="flex items-center gap-1 overflow-x-auto">
        {categories.map((value) => (
          <button
            key={value}
            onClick={() => setCategory(value)}
            className={`px-2.5 py-1 rounded-md text-[11px] font-medium whitespace-nowrap transition ${
              category === value
                ? 'bg-zinc-800 text-white font-semibold'
                : 'text-zinc-400 hover:text-white hover:bg-zinc-900'
            }`}
          >
            {value === 'all' ? 'All recommendations' : CATEGORY_LABELS[value] || value}
            {value === 'all' && ` (${(report?.optimizations || []).length})`}
          </button>
        ))}
      </div>

      {optimizations.length === 0 ? (
        <div className="text-center py-12 px-4 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20">
          <CheckCircle2 className="w-10 h-10 text-emerald-500/60 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-zinc-200">No recommendations</h3>
          <p className="text-xs text-zinc-400 mt-1">The advisor found no waste or capacity issues in the current usage window.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          {optimizations.map((optimization) => {
            const Icon = categoryIcon(optimization.category);
            const link = diagnosisLink(optimization);
            return (
              <div key={optimization.id} className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800 space-y-2.5">
                <div className="flex items-start justify-between gap-2">
                  <div className="flex items-start gap-2 min-w-0">
                    <div className="w-7 h-7 rounded-md bg-zinc-950 border border-zinc-800 flex items-center justify-center flex-shrink-0">
                      <Icon className="w-3.5 h-3.5 text-zinc-300" />
                    </div>
                    <div className="min-w-0">
                      <h3 className="text-xs font-semibold text-white">{optimization.title}</h3>
                      <div className="text-[10px] font-mono text-zinc-500 mt-0.5">
                        {resourceTypeLabel(optimization.resource_type)} · {optimization.resource_name}
                      </div>
                    </div>
                  </div>
                  <span className={`px-2 py-0.5 rounded-md border text-[10px] font-mono ${riskClasses(optimization.risk)}`}>
                    {optimization.risk}
                  </span>
                </div>

                <div className="grid grid-cols-3 gap-2 text-[10px] font-mono">
                  <div className="p-2 rounded bg-zinc-950 border border-zinc-800">
                    <div className="text-zinc-500 uppercase">Current</div>
                    <div className="text-zinc-200 mt-0.5 break-words">{optimization.current || '—'}</div>
                  </div>
                  <div className="p-2 rounded bg-zinc-950 border border-zinc-800">
                    <div className="text-zinc-500 uppercase">Observed</div>
                    <div className="text-zinc-200 mt-0.5 break-words">{optimization.observed || '—'}</div>
                  </div>
                  <div className="p-2 rounded bg-zinc-950 border border-zinc-800">
                    <div className="text-zinc-500 uppercase">Proposed</div>
                    <div className="text-zinc-200 mt-0.5 break-words">{optimization.proposed || '—'}</div>
                  </div>
                </div>

                <div className="text-[11px] text-zinc-400">
                  <span className="font-mono text-[10px] uppercase text-zinc-500">Benefit: </span>
                  {optimization.expected_benefit}
                </div>

                <div className="flex items-center justify-between text-[10px] font-mono text-zinc-500">
                  <span className={`px-1.5 py-0.5 rounded border ${statusClasses(optimization.category)}`}>
                    {CATEGORY_LABELS[optimization.category] || optimization.category}
                  </span>
                  <span>{optimization.requires_approval ? 'approval required' : 'read-only'}</span>
                </div>

                {link && (
                  <div className="pt-2.5 border-t border-zinc-850">
                    <button
                      onClick={() => onNavigate(link)}
                      className="inline-flex items-center gap-1 text-[10px] text-zinc-300 hover:text-white"
                    >
                      Open diagnostics <ArrowUpRight className="w-3 h-3" />
                    </button>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}

      <div className="rounded-xl border border-zinc-800 overflow-hidden">
        <div className="px-3 py-2 border-b border-zinc-800 bg-zinc-950/60 text-[10px] uppercase font-mono text-zinc-400 flex items-center justify-between">
          <span>Cost breakdown</span>
          <span>{formatMoney(report?.monthly_total || 0, report?.currency)} / month</span>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-zinc-950 text-zinc-500 uppercase font-mono text-[10px] tracking-wider">
              <tr>
                <th className="px-3 py-2">Resource</th>
                <th className="px-3 py-2">Type</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2">Specs</th>
                <th className="px-3 py-2 text-right">Monthly</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-zinc-850">
              {(report?.cost_resources || []).length === 0 ? (
                <tr>
                  <td colSpan={5} className="px-3 py-8 text-center text-zinc-500">
                    No billable resources.
                  </td>
                </tr>
              ) : (
                (report?.cost_resources || []).map((item) => (
                  <tr key={`${item.resource_type}:${item.id}`} className="hover:bg-zinc-900/40">
                    <td className="px-3 py-2 text-zinc-200">{item.name}</td>
                    <td className="px-3 py-2 text-zinc-400">{resourceTypeLabel(item.resource_type)}</td>
                    <td className="px-3 py-2">
                      <span className={`px-2 py-0.5 rounded-md border text-[10px] font-mono ${statusClasses(item.status)}`}>
                        {item.status}
                      </span>
                    </td>
                    <td className="px-3 py-2 text-zinc-400 font-mono text-[10px]">
                      {item.resource_type === 'bucket'
                        ? `${item.size_bytes} bytes`
                        : `${item.vcpus} vCPU · ${item.memory_mb} MB · ${item.disk_gb} GB`}
                    </td>
                    <td className="px-3 py-2 text-right font-mono text-zinc-200">
                      {formatMoney(item.monthly, report?.currency)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
        {(report?.notes || []).length > 0 && (
          <div className="px-3 py-2 border-t border-zinc-800 text-[10px] text-zinc-500">
            {(report?.notes || []).join(' ')}
          </div>
        )}
      </div>
    </div>
  );
};
