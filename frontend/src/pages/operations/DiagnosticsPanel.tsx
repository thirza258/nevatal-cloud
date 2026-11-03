import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  ArrowUpRight,
  CheckCircle2,
  ChevronRight,
  ClipboardList,
  Play,
  RefreshCw,
  Search,
  ShieldAlert,
  Stethoscope,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { Modal } from '../../components/ui/Modal';
import type {
  DiagnosticRecommendation,
  DiagnosticResult,
  RemediationInput,
  RemediationPlan,
  UnifiedResourceItem,
} from '../../types';
import {
  errorMessage,
  formatDate,
  resourceTypeLabel,
  riskClasses,
  statusClasses,
  unifiedToResourceType,
} from './utils';

interface DiagnosticsPanelProps {
  onNavigate: (path: string, param?: string) => void;
  initialType?: string;
  initialId?: string;
  initialName?: string;
}

const RESOURCE_TYPES = ['vps', 'app', 'n8n', 'openclaw', 'game_server', 'android', 'database', 'bucket', 'domain'];

const REMEDIABLE_ACTIONS = new Set(['start', 'stop', 'restart', 'backup.create', 'delete', 'deploy', 'resize']);

export const DiagnosticsPanel: React.FC<DiagnosticsPanelProps> = ({
  onNavigate,
  initialType,
  initialId,
  initialName,
}) => {
  const [resources, setResources] = useState<UnifiedResourceItem[]>([]);
  const [resourceType, setResourceType] = useState(initialType || 'vps');
  const [resourceId, setResourceId] = useState(initialId || '');
  const [resourceName, setResourceName] = useState(initialName || '');
  const [result, setResult] = useState<DiagnosticResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [selected, setSelected] = useState<DiagnosticRecommendation | null>(null);
  const [vcpus, setVcpus] = useState('');
  const [memoryMB, setMemoryMB] = useState('');
  const [diskGB, setDiskGB] = useState('');
  const [remediating, setRemediating] = useState(false);

  useEffect(() => {
    let cancelled = false;
    apiRequest<{ resources: UnifiedResourceItem[] }>('/resources')
      .then((res) => {
        if (!cancelled) setResources(res.resources || []);
      })
      .catch(() => {
        if (!cancelled) setResources([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const diagnose = useCallback(async (type: string, id: string) => {
    if (!type || !id) {
      setError('Choose a resource to diagnose.');
      return;
    }
    setLoading(true);
    setError(null);
    setResult(null);
    try {
      const res = await apiRequest<DiagnosticResult>(
        `/ai/diagnostics/${encodeURIComponent(type)}/${encodeURIComponent(id)}`
      );
      setResult(res);
      setResourceType(res.resource_type);
      setResourceId(res.resource_id);
      setResourceName(res.name);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    if (initialType && initialId) {
      setResourceType(initialType);
      setResourceId(initialId);
      setResourceName(initialName || '');
      void diagnose(initialType, initialId);
    }
  }, [initialType, initialId, initialName, diagnose]);

  const options = useMemo(
    () => resources.filter((resource) => unifiedToResourceType(resource.type) === resourceType),
    [resources, resourceType]
  );

  const selectedResource = useMemo(
    () => resources.find((resource) => resource.id === resourceId),
    [resources, resourceId]
  );

  const selectType = (type: string) => {
    setResourceType(type);
    setResourceId('');
    setResourceName('');
    setResult(null);
  };

  const selectResource = (id: string) => {
    setResourceId(id);
    const found = resources.find((resource) => resource.id === id);
    setResourceName(found?.name || '');
    setResult(null);
  };

  const submitRemediation = async () => {
    if (!selected || !result) return;
    setRemediating(true);
    setError(null);
    try {
      const input: RemediationInput = { action: selected.action };
      if (selected.action === 'resize') {
        if (vcpus) input.vcpus = Number(vcpus);
        if (memoryMB) input.memory_mb = Number(memoryMB);
        if (diskGB) input.disk_gb = Number(diskGB);
      }
      const plan = await apiRequest<RemediationPlan>(
        `/ai/diagnostics/${encodeURIComponent(result.resource_type)}/${encodeURIComponent(result.resource_id)}/remediate`,
        { method: 'POST', body: JSON.stringify(input) }
      );
      setSelected(null);
      onNavigate(
        `/dashboard/ai/conversations/${plan.conversation_id}?plan=${plan.id}`,
        plan.id
      );
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setRemediating(false);
    }
  };

  const resourceLabel = (resource: UnifiedResourceItem) =>
    `${resource.name} · ${resourceTypeLabel(unifiedToResourceType(resource.type))}`;

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-sm font-semibold text-white">Resource diagnostics</h2>
        <p className="text-xs text-zinc-400 mt-0.5">
          Evidence, ranked hypotheses and remediation plans from observed platform data. Nothing is changed until the resulting plan is approved.
        </p>
      </div>

      <div className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800 space-y-3">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
          <div>
            <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Resource type</label>
            <select
              value={resourceType}
              onChange={(e) => selectType(e.target.value)}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
            >
              {RESOURCE_TYPES.map((type) => (
                <option key={type} value={type}>
                  {resourceTypeLabel(type)}
                </option>
              ))}
            </select>
          </div>
          <div>
            <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Resource</label>
            {options.length > 0 ? (
              <select
                value={resourceId}
                onChange={(e) => selectResource(e.target.value)}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              >
                <option value="">Select a resource…</option>
                {options.map((resource) => (
                  <option key={resource.id} value={resource.id}>
                    {resourceLabel(resource)}
                  </option>
                ))}
              </select>
            ) : (
              <input
                value={resourceId}
                onChange={(e) => setResourceId(e.target.value)}
                placeholder="Resource UUID"
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs font-mono text-zinc-100 focus:outline-none focus:border-zinc-600"
              />
            )}
          </div>
          <div className="flex items-end">
            <button
              onClick={() => diagnose(resourceType, resourceId)}
              disabled={loading || !resourceId}
              className="w-full inline-flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold disabled:opacity-40 transition"
            >
              {loading ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Stethoscope className="w-3.5 h-3.5" />}
              {loading ? 'Collecting evidence…' : 'Run diagnostics'}
            </button>
          </div>
        </div>
        {(selectedResource || resourceName) && (
          <div className="text-[10px] font-mono text-zinc-500 flex items-center gap-1.5">
            <Search className="w-3 h-3" />
            {selectedResource?.name || resourceName} · {selectedResource?.id || resourceId}
          </div>
        )}
      </div>

      {error && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
          <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
      )}

      {!result && !loading && !error && (
        <div className="text-center py-14 px-4 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20">
          <Stethoscope className="w-10 h-10 text-zinc-600 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-zinc-200">No diagnosis yet</h3>
          <p className="text-xs text-zinc-400 max-w-md mx-auto mt-1">
            Pick a resource and run diagnostics. The evidence is deterministic; a remediation is only ever proposed as an approval-bound plan.
          </p>
        </div>
      )}

      {result && (
        <div className="space-y-3">
          <div className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800">
            <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
              <div>
                <div className="flex items-center gap-2 flex-wrap">
                  <h3 className="text-sm font-semibold text-white">{result.name}</h3>
                  <span className={`px-2 py-0.5 rounded-md border text-[10px] font-mono ${statusClasses(result.health)}`}>
                    {result.health}
                  </span>
                  <span className="text-[10px] font-mono text-zinc-500">{resourceTypeLabel(result.resource_type)}</span>
                </div>
                <div className="text-[10px] font-mono text-zinc-500 mt-1">
                  observed {formatDate(result.observed_at)} · {result.resource_id}
                </div>
              </div>
              <button
                onClick={() => diagnose(result.resource_type, result.resource_id)}
                className="self-start inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px]"
              >
                <RefreshCw className="w-3 h-3" />
                Re-run
              </button>
            </div>

            <div className="mt-3 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2">
              {result.facts.map((fact) => (
                <div key={`${fact.name}:${fact.value}`} className="p-2 rounded-md bg-zinc-950 border border-zinc-800">
                  <div className="flex items-center justify-between text-[9px] uppercase font-mono text-zinc-500">
                    <span>{fact.name}</span>
                    <span className={fact.status === 'critical' ? 'text-rose-400' : fact.status === 'warning' ? 'text-amber-400' : 'text-zinc-500'}>
                      {fact.status}
                    </span>
                  </div>
                  <div className="text-[11px] font-mono text-zinc-200 mt-0.5 break-all">{fact.value}</div>
                </div>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            <div className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800">
              <div className="flex items-center gap-1.5 text-[10px] uppercase font-mono text-zinc-400 mb-2">
                <ShieldAlert className="w-3.5 h-3.5" />
                Hypotheses
              </div>
              {result.hypotheses.length === 0 ? (
                <div className="text-xs text-zinc-500">No cause stands out from the evidence.</div>
              ) : (
                <div className="space-y-2">
                  {result.hypotheses.map((hypothesis, index) => (
                    <div key={`${hypothesis.cause}:${index}`} className="p-2.5 rounded-md bg-zinc-950 border border-zinc-800">
                      <div className="flex items-start justify-between gap-2">
                        <div className="text-[11px] text-zinc-200">{hypothesis.cause}</div>
                        <span
                          className={`px-1.5 py-0.5 rounded border text-[9px] font-mono ${
                            hypothesis.confidence === 'high'
                              ? 'bg-rose-500/10 text-rose-300 border-rose-500/20'
                              : hypothesis.confidence === 'medium'
                                ? 'bg-amber-500/10 text-amber-300 border-amber-500/20'
                                : 'bg-zinc-900 text-zinc-400 border-zinc-800'
                          }`}
                        >
                          {hypothesis.confidence}
                        </span>
                      </div>
                      <ul className="mt-1 space-y-0.5">
                        {hypothesis.evidence.map((evidence) => (
                          <li key={evidence} className="text-[10px] font-mono text-zinc-500 flex items-start gap-1">
                            <ChevronRight className="w-3 h-3 flex-shrink-0 mt-0.5" />
                            {evidence}
                          </li>
                        ))}
                      </ul>
                    </div>
                  ))}
                </div>
              )}
            </div>

            <div className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800">
              <div className="flex items-center gap-1.5 text-[10px] uppercase font-mono text-zinc-400 mb-2">
                <ClipboardList className="w-3.5 h-3.5" />
                Recommendations
              </div>
              {result.recommendations.length === 0 ? (
                <div className="text-xs text-zinc-500">No remediation is recommended.</div>
              ) : (
                <div className="space-y-2">
                  {result.recommendations.map((recommendation, index) => {
                    const canRemediate = REMEDIABLE_ACTIONS.has(recommendation.action);
                    return (
                      <div
                        key={`${recommendation.action}:${index}`}
                        className="p-2.5 rounded-md bg-zinc-950 border border-zinc-800"
                      >
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <div className="text-[11px] text-zinc-200">{recommendation.label}</div>
                            <div className="text-[9px] font-mono text-zinc-500 mt-0.5">
                              action {recommendation.action} · tool {recommendation.tool || '—'}
                            </div>
                          </div>
                          <span className={`px-1.5 py-0.5 rounded border text-[9px] font-mono ${riskClasses(recommendation.risk)}`}>
                            {recommendation.risk}
                          </span>
                        </div>
                        <div className="mt-2 flex items-center justify-between">
                          <span className="text-[9px] font-mono text-zinc-500">
                            {recommendation.requires_approval ? 'approval required' : 'read-only'}
                          </span>
                          {canRemediate ? (
                            <button
                              onClick={() => {
                                setSelected(recommendation);
                                setVcpus('');
                                setMemoryMB('');
                                setDiskGB('');
                              }}
                              className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-[10px] font-semibold"
                            >
                              <Play className="w-3 h-3" />
                              Create plan
                            </button>
                          ) : (
                            <span className="text-[9px] font-mono text-zinc-600">no plan action</span>
                          )}
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

          {result.health === 'healthy' && (
            <div className="flex items-start gap-2 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
              <CheckCircle2 className="w-4 h-4 flex-shrink-0 mt-0.5" />
              <span>All observed checks are within their thresholds.</span>
            </div>
          )}
        </div>
      )}

      <Modal
        isOpen={!!selected}
        onClose={() => setSelected(null)}
        title={selected?.label || 'Create remediation plan'}
        description="This creates a plan in the AI diagnostics workspace. Nothing is executed until you approve the plan."
        maxWidth="lg"
      >
        {selected && (
          <div className="space-y-3">
            <div className="flex items-center gap-2 text-xs text-zinc-300">
              <span className={`px-2 py-0.5 rounded border text-[10px] font-mono ${riskClasses(selected.risk)}`}>
                {selected.risk} risk
              </span>
              <span className="font-mono text-[10px] text-zinc-500">action {selected.action}</span>
            </div>
            {selected.action === 'resize' && (
              <div className="grid grid-cols-3 gap-2">
                <div>
                  <label className="block text-[10px] font-mono uppercase text-zinc-400 mb-1">vCPUs</label>
                  <input
                    type="number"
                    min={0}
                    value={vcpus}
                    onChange={(e) => setVcpus(e.target.value)}
                    placeholder="keep current"
                    className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
                  />
                </div>
                <div>
                  <label className="block text-[10px] font-mono uppercase text-zinc-400 mb-1">Memory MB</label>
                  <input
                    type="number"
                    min={0}
                    value={memoryMB}
                    onChange={(e) => setMemoryMB(e.target.value)}
                    placeholder="keep current"
                    className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
                  />
                </div>
                <div>
                  <label className="block text-[10px] font-mono uppercase text-zinc-400 mb-1">Disk GB</label>
                  <input
                    type="number"
                    min={0}
                    value={diskGB}
                    onChange={(e) => setDiskGB(e.target.value)}
                    placeholder="keep current"
                    className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
                  />
                </div>
              </div>
            )}
            <p className="text-[11px] text-zinc-400">
              The plan is policy-validated and approval-bound. Destructive actions require typed confirmation in the AI workspace.
            </p>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => setSelected(null)}
                className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs"
              >
                Cancel
              </button>
              <button
                onClick={submitRemediation}
                disabled={remediating}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold disabled:opacity-50"
              >
                {remediating ? 'Creating plan…' : 'Create plan'}
                <ArrowUpRight className="w-3 h-3" />
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};
