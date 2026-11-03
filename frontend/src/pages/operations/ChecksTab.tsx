import React, { useEffect, useMemo, useState } from 'react';
import {
  Activity,
  AlertTriangle,
  Check,
  ChevronDown,
  ChevronRight,
  Clock,
  Play,
  Plus,
  RefreshCw,
  Trash2,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { Modal } from '../../components/ui/Modal';
import type { CheckView, CreateCheckInput, UnifiedResourceItem } from '../../types';
import {
  CHECK_TYPES,
  checkTypeMeta,
  errorMessage,
  formatDate,
  formatRelative,
  resourceTypeLabel,
  statusClasses,
  unifiedToResourceType,
} from './utils';

interface ChecksTabProps {
  onNavigate: (path: string, param?: string) => void;
}

const emptyForm = {
  name: '',
  check_type: 'instance.health',
  resource_type: 'vps',
  resource_id: '',
  interval_seconds: 300,
  enabled: true,
};

export const ChecksTab: React.FC<ChecksTabProps> = ({ onNavigate }) => {
  const [checks, setChecks] = useState<CheckView[]>([]);
  const [resources, setResources] = useState<UnifiedResourceItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState({ ...emptyForm });
  const [creating, setCreating] = useState(false);

  const [expanded, setExpanded] = useState<string | null>(null);
  const [deleting, setDeleting] = useState<CheckView | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const [list, inventory] = await Promise.all([
        apiRequest<CheckView[]>('/ai/checks'),
        apiRequest<{ resources: UnifiedResourceItem[] }>('/resources'),
      ]);
      setChecks(list || []);
      setResources(inventory.resources || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const meta = useMemo(() => checkTypeMeta(form.check_type), [form.check_type]);

  const resourceOptions = useMemo(() => {
    if (!meta || form.resource_type === 'account') return [];
    return resources.filter((resource) => unifiedToResourceType(resource.type) === form.resource_type);
  }, [resources, meta, form.resource_type]);

  const showNotice = (message: string) => {
    setNotice(message);
    window.setTimeout(() => setNotice(null), 4000);
  };

  const selectCheckType = (value: string) => {
    const next = checkTypeMeta(value);
    setForm({
      ...form,
      check_type: value,
      resource_type: next?.resourceTypes[0] || 'vps',
      resource_id: '',
    });
  };

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreating(true);
    setError(null);
    try {
      if (form.resource_type !== 'account' && !form.resource_id) {
        throw new Error('Choose a resource to monitor.');
      }
      const payload: CreateCheckInput = {
        name: form.name.trim() || form.check_type,
        check_type: form.check_type,
        resource_type: form.resource_type,
        resource_id: form.resource_type === 'account' ? '' : form.resource_id,
        interval_seconds: Number(form.interval_seconds),
        enabled: form.enabled,
      };
      // For account checks the backend binds the resource to the caller.
      const body: Record<string, unknown> = { ...payload };
      if (form.resource_type === 'account') delete body.resource_id;
      await apiRequest('/ai/checks', { method: 'POST', body: JSON.stringify(body) });
      setShowCreate(false);
      setForm({ ...emptyForm });
      showNotice('Check created and scheduled.');
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const toggle = async (check: CheckView) => {
    setBusy(`toggle:${check.id}`);
    setError(null);
    try {
      await apiRequest(`/ai/checks/${check.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ enabled: !check.enabled }),
      });
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const runNow = async (check: CheckView) => {
    setBusy(`run:${check.id}`);
    setError(null);
    try {
      await apiRequest(`/ai/checks/${check.id}/run`, { method: 'POST' });
      showNotice(`${check.name}: check executed.`);
      await load();
      setExpanded(check.id);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const remove = async () => {
    if (!deleting) return;
    setBusy(`delete:${deleting.id}`);
    setError(null);
    try {
      await apiRequest(`/ai/checks/${deleting.id}`, { method: 'DELETE' });
      setDeleting(null);
      showNotice('Check deleted.');
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const resourceName = (check: CheckView) => {
    const found = resources.find((resource) => resource.id === check.resource_id);
    if (found) return found.name;
    if (check.resource_type === 'account') return 'Account';
    return check.resource_id;
  };

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-white">Scheduled checks</h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Recurring health, backup and quota checks. Failures create notifications and can trigger policies; checks never mutate infrastructure.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={load}
            disabled={loading}
            className="p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 transition"
            title="Refresh checks"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold transition"
          >
            <Plus className="w-3.5 h-3.5" />
            New check
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
          <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
      )}
      {notice && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
          <Check className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{notice}</span>
        </div>
      )}

      {loading ? (
        <div className="p-12 text-center text-zinc-500 flex flex-col items-center gap-2 text-xs">
          <RefreshCw className="w-5 h-5 animate-spin" />
          Loading checks…
        </div>
      ) : checks.length === 0 ? (
        <div className="text-center py-14 px-4 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20">
          <Activity className="w-10 h-10 text-zinc-600 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-zinc-200">No scheduled checks</h3>
          <p className="text-xs text-zinc-400 max-w-md mx-auto mt-1 mb-5">
            Add a recurring check to detect stopped instances, failed deployments or stale backups before they become incidents.
          </p>
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold"
          >
            <Plus className="w-3.5 h-3.5" />
            Create check
          </button>
        </div>
      ) : (
        <div className="rounded-xl border border-zinc-800 overflow-hidden bg-zinc-900/30">
          <div className="divide-y divide-zinc-850">
            {checks.map((check) => {
              const checkMeta = checkTypeMeta(check.check_type);
              const isExpanded = expanded === check.id;
              return (
                <div key={check.id}>
                  <div className="p-3 flex flex-col sm:flex-row sm:items-center gap-2 sm:gap-3">
                    <button
                      onClick={() => setExpanded(isExpanded ? null : check.id)}
                      className="flex items-center gap-2 min-w-0 flex-1 text-left"
                    >
                      {isExpanded ? (
                        <ChevronDown className="w-3.5 h-3.5 text-zinc-500 flex-shrink-0" />
                      ) : (
                        <ChevronRight className="w-3.5 h-3.5 text-zinc-500 flex-shrink-0" />
                      )}
                      <div className="min-w-0">
                        <div className="flex items-center gap-2 flex-wrap">
                          <span className="text-xs font-semibold text-white truncate">{check.name}</span>
                          <span className={`px-2 py-0.5 rounded-md border text-[10px] font-mono ${statusClasses(check.status)}`}>
                            {check.status}
                          </span>
                          {!check.enabled && (
                            <span className="px-2 py-0.5 rounded-md bg-zinc-900 text-zinc-500 border border-zinc-800 text-[10px] font-mono">
                              paused
                            </span>
                          )}
                        </div>
                        <div className="text-[10px] text-zinc-500 font-mono mt-0.5 truncate">
                          {checkMeta?.label || check.check_type} · {resourceTypeLabel(check.resource_type)} {resourceName(check)} · every{' '}
                          {check.interval_seconds >= 3600
                            ? `${(check.interval_seconds / 3600).toFixed(1)}h`
                            : `${Math.round(check.interval_seconds / 60)}m`}{' '}
                          · last run {formatRelative(check.last_run_at)}
                        </div>
                      </div>
                    </button>
                    <div className="flex items-center gap-1.5 flex-shrink-0 sm:justify-end">
                      <button
                        onClick={() => runNow(check)}
                        disabled={busy === `run:${check.id}`}
                        className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition disabled:opacity-50"
                      >
                        <Play className="w-3 h-3" /> Run
                      </button>
                      <button
                        onClick={() => toggle(check)}
                        disabled={busy === `toggle:${check.id}`}
                        className="px-2 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition disabled:opacity-50"
                      >
                        {check.enabled ? 'Pause' : 'Resume'}
                      </button>
                      <button
                        onClick={() => setDeleting(check)}
                        className="p-1 rounded-md bg-zinc-850 hover:bg-rose-500/20 text-zinc-400 hover:text-rose-300 border border-zinc-800"
                        title="Delete check"
                      >
                        <Trash2 className="w-3 h-3" />
                      </button>
                    </div>
                  </div>
                  {isExpanded && (
                    <div className="px-8 pb-3 space-y-2">
                      <div className="grid grid-cols-1 sm:grid-cols-3 gap-2 text-[10px] font-mono text-zinc-400">
                        <div className="p-2 rounded bg-zinc-950 border border-zinc-800">
                          <div className="text-zinc-500 uppercase">Last run</div>
                          <div className="text-zinc-200 mt-0.5">{formatDate(check.last_run_at)}</div>
                        </div>
                        <div className="p-2 rounded bg-zinc-950 border border-zinc-800">
                          <div className="text-zinc-500 uppercase">Next run</div>
                          <div className="text-zinc-200 mt-0.5">{formatDate(check.next_run_at)}</div>
                        </div>
                        <div className="p-2 rounded bg-zinc-950 border border-zinc-800">
                          <div className="text-zinc-500 uppercase">Consecutive failures</div>
                          <div className="text-zinc-200 mt-0.5">{check.consecutive_failures}</div>
                        </div>
                      </div>
                      {check.last_result && (
                        <div className="p-2.5 rounded bg-zinc-950 border border-zinc-800 text-[11px]">
                          <div className="flex items-center gap-1.5 text-zinc-400">
                            <Clock className="w-3 h-3" />
                            <span className="font-mono text-[10px] uppercase">Last result</span>
                            <span className={`px-1.5 py-0.5 rounded border text-[10px] font-mono ${statusClasses(check.last_result.status)}`}>
                              {check.last_result.status || check.status}
                            </span>
                          </div>
                          <div className="text-zinc-200 mt-1">{check.last_result.message || 'No message.'}</div>
                          {check.last_result.facts && Object.keys(check.last_result.facts).length > 0 && (
                            <div className="mt-2 grid grid-cols-2 sm:grid-cols-4 gap-1.5">
                              {Object.entries(check.last_result.facts).map(([key, value]) => (
                                <div key={key} className="p-1.5 rounded bg-zinc-900 border border-zinc-850">
                                  <div className="text-[9px] uppercase font-mono text-zinc-500">{key}</div>
                                  <div className="text-[10px] font-mono text-zinc-200 break-all">
                                    {typeof value === 'string' ? value : JSON.stringify(value)}
                                  </div>
                                </div>
                              ))}
                            </div>
                          )}
                        </div>
                      )}
                      <div className="flex justify-end">
                        <button
                          onClick={() =>
                            onNavigate(`/dashboard/operations?tab=diagnostics&type=${encodeURIComponent(check.resource_type)}&id=${encodeURIComponent(check.resource_id)}`)
                          }
                          className="text-[10px] text-zinc-400 hover:text-white underline underline-offset-2"
                        >
                          Open diagnostics
                        </button>
                      </div>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      <Modal
        isOpen={showCreate}
        onClose={() => setShowCreate(false)}
        title="New scheduled check"
        description="Checks run automatically on the monitor loop. Runs are claimed atomically, so replicas never duplicate work."
        maxWidth="lg"
      >
        <form onSubmit={create} className="space-y-3">
          <div>
            <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Name</label>
            <input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder={form.check_type}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
            />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Check type</label>
              <select
                value={form.check_type}
                onChange={(e) => selectCheckType(e.target.value)}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              >
                {CHECK_TYPES.map((item) => (
                  <option key={item.value} value={item.value}>
                    {item.label}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Resource type</label>
              <select
                value={form.resource_type}
                onChange={(e) => setForm({ ...form, resource_type: e.target.value, resource_id: '' })}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              >
                {(meta?.resourceTypes || []).map((type) => (
                  <option key={type} value={type}>
                    {resourceTypeLabel(type)}
                  </option>
                ))}
              </select>
            </div>
          </div>
          {meta?.description && <p className="text-[10px] text-zinc-500">{meta.description}</p>}
          {form.resource_type === 'account' ? (
            <p className="text-[11px] text-zinc-400 p-2 rounded bg-zinc-900 border border-zinc-800">
              Account checks monitor the caller's quota usage.
            </p>
          ) : resourceOptions.length > 0 ? (
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Resource</label>
              <select
                value={form.resource_id}
                onChange={(e) => setForm({ ...form, resource_id: e.target.value })}
                required
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              >
                <option value="">Select a resource…</option>
                {resourceOptions.map((resource) => (
                  <option key={resource.id} value={resource.id}>
                    {resource.name} · {resourceTypeLabel(unifiedToResourceType(resource.type))}
                  </option>
                ))}
              </select>
            </div>
          ) : (
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Resource ID</label>
              <input
                value={form.resource_id}
                onChange={(e) => setForm({ ...form, resource_id: e.target.value })}
                placeholder="UUID of the resource"
                required
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 font-mono focus:outline-none focus:border-zinc-600"
              />
              <p className="text-[10px] text-zinc-500 mt-1">
                No {resourceTypeLabel(form.resource_type).toLowerCase()} was found in the inventory.
              </p>
            </div>
          )}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Interval (seconds)</label>
              <input
                type="number"
                min={60}
                max={86400}
                value={form.interval_seconds}
                onChange={(e) => setForm({ ...form, interval_seconds: Number(e.target.value) })}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              />
              <p className="text-[10px] text-zinc-500 mt-1">60–86400 seconds.</p>
            </div>
            <label className="flex items-end gap-2 text-xs text-zinc-300 pb-5">
              <input
                type="checkbox"
                checked={form.enabled}
                onChange={(e) => setForm({ ...form, enabled: e.target.checked })}
                className="accent-white"
              />
              Start enabled
            </label>
          </div>
          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={creating}
              className="px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold disabled:opacity-50"
            >
              {creating ? 'Creating…' : 'Create check'}
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        isOpen={!!deleting}
        onClose={() => setDeleting(null)}
        title={`Delete ${deleting?.name || 'check'}?`}
        description="Its history is removed. New failures will no longer create notifications."
      >
        <div className="flex justify-end gap-2">
          <button
            onClick={() => setDeleting(null)}
            className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs"
          >
            Cancel
          </button>
          <button
            onClick={remove}
            disabled={busy === `delete:${deleting?.id}`}
            className="px-3 py-1.5 rounded-md bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold disabled:opacity-50"
          >
            Delete check
          </button>
        </div>
      </Modal>
    </div>
  );
};
