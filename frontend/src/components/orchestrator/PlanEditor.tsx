import React, { useEffect, useMemo, useState } from 'react';
import { Braces, Save, Server, SlidersHorizontal, TriangleAlert, X } from 'lucide-react';
import type { OrchestratorPlan, PlanRevision, PlanSpecification } from '../../types';
import {
  Catalog,
  DescribedError,
  DiffEntry,
  describeError,
  diffJson,
  diffRevisions,
  errorCodeOf,
  fetchPlan,
  formatMemoryMb,
  isEditablePlan,
  loadCatalog,
  patchPlan,
} from '../../lib/orchestrator';
import { DiffList } from './RevisionDiff';
import { ErrorNote, Notice, Spinner } from './OrchestratorBits';

const VPS_TYPES = new Set(['vps', 'virtual_machine', 'instance']);
const HOSTNAME_RE = /^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;

const isRecord = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);
const clone = <T,>(value: T): T => JSON.parse(JSON.stringify(value ?? {})) as T;
const isPosInt = (v: unknown): v is number => typeof v === 'number' && Number.isInteger(v) && v > 0;

type VpsField = 'name' | 'vcpus' | 'memory_mb' | 'disk_gb' | 'template_id';

/** Client-side hints only; the policy engine re-validates on PATCH. Only fields present are checked. */
function vpsProblems(config: Record<string, unknown>): Partial<Record<VpsField, string>> {
  const p: Partial<Record<VpsField, string>> = {};
  if ('name' in config && (typeof config.name !== 'string' || !HOSTNAME_RE.test(config.name))) {
    p.name = 'Use 1-63 lowercase letters, digits or hyphens; start and end with a letter or digit.';
  }
  if ('vcpus' in config && (!isPosInt(config.vcpus) || config.vcpus > 64)) p.vcpus = 'Enter 1 to 64.';
  if ('memory_mb' in config && (!isPosInt(config.memory_mb) || config.memory_mb < 256)) p.memory_mb = 'At least 256 MB.';
  if ('disk_gb' in config && !isPosInt(config.disk_gb)) p.disk_gb = 'At least 1 GB.';
  if ('template_id' in config && (typeof config.template_id !== 'string' || !config.template_id)) p.template_id = 'Choose a template.';
  return p;
}

const inputClass = (bad?: boolean) =>
  `w-full bg-zinc-900 border rounded-md px-2.5 py-1.5 text-xs text-zinc-100 placeholder-zinc-600 focus:outline-none transition-colors ${
    bad ? 'border-rose-500/60 focus:border-rose-400' : 'border-zinc-800 focus:border-zinc-600'
  }`;

const Field: React.FC<{ label: string; htmlFor: string; error?: string; hint?: string; children: React.ReactNode }> = ({
  label,
  htmlFor,
  error,
  hint,
  children,
}) => (
  <div className="min-w-0">
    <label htmlFor={htmlFor} className="block text-[10px] font-mono uppercase tracking-wider text-zinc-500 mb-1">
      {label}
    </label>
    {children}
    {error ? (
      <p className="mt-1 text-[10.5px] text-rose-400">{error}</p>
    ) : hint ? (
      <p className="mt-1 text-[10.5px] text-zinc-500">{hint}</p>
    ) : null}
  </div>
);

export interface PlanEditorProps {
  plan: OrchestratorPlan;
  onCancel: () => void;
  onSaved: (plan: OrchestratorPlan) => void;
  /** Called after the editor re-fetched the plan (conflict / not editable). */
  onPlanReloaded: (plan: OrchestratorPlan) => void;
}

/**
 * Edits the current revision's `specification` (PATCH /ai/plans/:id {base_version, specification}).
 * One draft feeds both the form and the advanced JSON tab, so unknown fields survive a form edit.
 */
export const PlanEditor: React.FC<PlanEditorProps> = ({ plan, onCancel, onSaved, onPlanReloaded }) => {
  // Snapshot at open: later prop updates (live reloads) never reset the user's draft.
  const [baseRevision] = useState<PlanRevision | null>(plan.revision);
  const [baseVersion, setBaseVersion] = useState<number>(plan.current_version);
  const [draft, setDraft] = useState<PlanSpecification>(() => clone(plan.revision?.specification ?? {}));
  const [tab, setTab] = useState<'form' | 'json'>('form');
  const [jsonText, setJsonText] = useState('');
  const [jsonError, setJsonError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<DescribedError | null>(null);
  const [conflict, setConflict] = useState<{ latest: OrchestratorPlan; diff: DiffEntry[] } | null>(null);
  const [notEditable, setNotEditable] = useState(false);
  const [catalog, setCatalog] = useState<Catalog | null>(null);

  useEffect(() => {
    let live = true;
    loadCatalog().then((c) => live && setCatalog(c));
    return () => {
      live = false;
    };
  }, []);

  const rawResources: unknown[] = Array.isArray(draft.resources) ? draft.resources : [];
  const vpsEntries = rawResources
    .map((r, index) => ({ index, r: isRecord(r) ? r : {} }))
    .filter(({ r }) => VPS_TYPES.has(String(r.type ?? '').toLowerCase()));
  const otherCount = rawResources.length - vpsEntries.length;

  const problems = vpsEntries.map(({ r }) => vpsProblems(isRecord(r.config) ? r.config : {}));
  const hasProblems = problems.some((p) => Object.keys(p).length > 0);

  const changes = useMemo(
    () => (baseRevision ? diffJson(baseRevision.specification, draft, 'specification') : []),
    [baseRevision, draft]
  );

  const updateConfig = (index: number, patch: Record<string, unknown>) => {
    setDraft((prev) => {
      const next = clone(prev);
      const list = Array.isArray(next.resources) ? (next.resources as unknown[]) : [];
      const item = isRecord(list[index]) ? (list[index] as Record<string, unknown>) : {};
      const config = { ...(isRecord(item.config) ? item.config : {}) };
      for (const [k, v] of Object.entries(patch)) {
        if (v === undefined) delete config[k];
        else config[k] = v;
      }
      item.config = config;
      list[index] = item;
      next.resources = list as PlanSpecification['resources'];
      return next;
    });
  };

  const switchTab = (next: 'form' | 'json') => {
    if (next === tab) return;
    if (next === 'json') {
      setJsonText(JSON.stringify(draft, null, 2));
      setJsonError(null);
    } else if (jsonError) {
      return;
    }
    setTab(next);
  };

  const onJsonChange = (text: string) => {
    setJsonText(text);
    try {
      const parsed: unknown = JSON.parse(text);
      if (!isRecord(parsed)) {
        setJsonError('The specification must be a JSON object.');
        return;
      }
      setDraft(parsed as PlanSpecification);
      setJsonError(null);
    } catch (err) {
      setJsonError(err instanceof Error ? err.message : 'Invalid JSON');
    }
  };

  const save = async (version: number) => {
    setSaving(true);
    setError(null);
    try {
      const updated = await patchPlan(plan.id, version, draft);
      onSaved(updated);
    } catch (err) {
      const code = errorCodeOf(err);
      if (code === 'PLAN_VERSION_CONFLICT') {
        try {
          const latest = await fetchPlan(plan.id);
          onPlanReloaded(latest);
          setConflict({
            latest,
            diff: baseRevision && latest.revision ? diffRevisions(baseRevision, latest.revision) : [],
          });
          if (!isEditablePlan(latest.status)) setNotEditable(true);
        } catch (reloadErr) {
          setError(describeError(reloadErr));
        }
      } else if (code === 'PLAN_NOT_EDITABLE') {
        setNotEditable(true);
        setError(describeError(err));
        fetchPlan(plan.id).then(onPlanReloaded, () => {});
      } else {
        setError(describeError(err));
      }
    } finally {
      setSaving(false);
    }
  };

  const keepMineOverLatest = () => {
    if (!conflict) return;
    const version = conflict.latest.current_version;
    setBaseVersion(version);
    setConflict(null);
    void save(version);
  };

  if (!baseRevision) {
    return (
      <div className="p-3 space-y-3">
        <Notice tone="warning">This plan has no revision data, so it cannot be edited here.</Notice>
        <button type="button" onClick={onCancel} className="text-xs text-zinc-300 hover:text-white">
          Back to plan
        </button>
      </div>
    );
  }

  // Saving an approved plan supersedes its approval (the server returns it to awaiting_approval).
  const liveApproval = plan.approval && plan.approval.status === 'approved' ? plan.approval : null;
  const templates = (catalog?.templates ?? []).filter((t) => t.enabled !== false);
  const sizes = (catalog?.plans ?? []).filter((p) => p.enabled !== false);
  const canSave = !saving && !notEditable && !jsonError && !hasProblems && changes.length > 0 && !conflict;

  return (
    <div className="space-y-3" data-testid="plan-editor">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="text-xs font-semibold text-white">Edit plan</div>
          <div className="text-[10.5px] font-mono text-zinc-500 truncate">
            based on v{baseVersion} · saving creates a new revision and supersedes earlier approvals
          </div>
        </div>
        <button
          type="button"
          onClick={onCancel}
          title="Close the editor"
          className="p-1 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>

      {(liveApproval || plan.status === 'approved') && (
        <Notice tone="warning">
          <span data-testid="edit-cancels-approval">
            This plan is approved{liveApproval ? ` (v${liveApproval.version})` : ''}. Saving cancels that approval: you will
            need to review and approve the new revision before it can run.
          </span>
        </Notice>
      )}

      {conflict && (
        <div className="space-y-2 p-2.5 rounded-md border border-amber-500/30 bg-amber-500/5" data-testid="edit-conflict">
          <div className="flex items-start gap-2 text-[11px] text-amber-200">
            <TriangleAlert className="w-3.5 h-3.5 mt-px text-amber-400 flex-shrink-0" />
            <span>
              Version {conflict.latest.current_version} of this plan was saved while you were editing v{baseRevision.version}.
              Your edits are kept below. These are the changes in v{conflict.latest.current_version}:
            </span>
          </div>
          <DiffList
            entries={conflict.diff}
            fromLabel={`v${baseRevision.version}`}
            toLabel={`v${conflict.latest.current_version}`}
            emptyText="No field changes (only metadata changed)."
          />
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={keepMineOverLatest}
              disabled={notEditable || saving}
              className="px-2.5 py-1 rounded-md text-[11px] font-semibold bg-white text-zinc-950 hover:bg-zinc-200 disabled:opacity-50"
            >
              Save my edits over v{conflict.latest.current_version}
            </button>
            <button
              type="button"
              onClick={onCancel}
              className="px-2.5 py-1 rounded-md text-[11px] font-medium bg-zinc-900 border border-zinc-800 text-zinc-300 hover:bg-zinc-800"
            >
              Discard my edits
            </button>
          </div>
        </div>
      )}

      <div className="inline-flex p-0.5 rounded-md bg-zinc-900 border border-zinc-800" role="tablist">
        {(
          [
            ['form', 'Resources', SlidersHorizontal],
            ['json', 'Advanced JSON', Braces],
          ] as const
        ).map(([key, label, Icon]) => (
          <button
            key={key}
            type="button"
            role="tab"
            aria-selected={tab === key}
            onClick={() => switchTab(key)}
            disabled={key === 'form' && tab === 'json' && !!jsonError}
            title={key === 'form' && jsonError ? 'Fix the JSON first' : undefined}
            className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded text-[11px] font-medium transition-colors disabled:opacity-50 ${
              tab === key ? 'bg-zinc-800 text-white' : 'text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <Icon className="w-3 h-3" />
            {label}
          </button>
        ))}
      </div>

      {tab === 'form' ? (
        <div className="space-y-3">
          {vpsEntries.length === 0 && (
            <Notice tone="info">This plan has no VPS resources. Use Advanced JSON to edit its specification.</Notice>
          )}
          {catalog?.error && (
            <Notice tone="warning">The template and size catalog could not be loaded. You can still type IDs.</Notice>
          )}
          {vpsEntries.map(({ index, r }, i) => {
            const config = isRecord(r.config) ? r.config : {};
            const p = problems[i] ?? {};
            const id = (f: string) => `plan-edit-${index}-${f}`;
            const selectedSize = sizes.find((s) => s.id === config.plan_id);
            const sizeMismatch =
              selectedSize &&
              ((typeof config.vcpus === 'number' && config.vcpus !== selectedSize.vcpus) ||
                (typeof config.memory_mb === 'number' && config.memory_mb !== selectedSize.memory_mb) ||
                (typeof config.disk_gb === 'number' && config.disk_gb !== selectedSize.disk_gb));
            const templateKnown = templates.some((t) => t.id === config.template_id);
            const sizeKnown = sizes.some((s) => s.id === config.plan_id);
            const num = (v: string) => (v.trim() === '' ? null : Number.parseInt(v, 10));
            return (
              <fieldset
                key={index}
                className="rounded-lg border border-zinc-800 bg-zinc-950/60 p-2.5 space-y-2.5"
                data-testid={`vps-form-${String(r.key ?? index)}`}
              >
                <legend className="px-1 flex items-center gap-1.5 text-[11px] font-semibold text-zinc-200">
                  <Server className="w-3 h-3 text-zinc-400" />
                  <span className="font-mono">{String(r.key ?? `resource ${index + 1}`)}</span>
                  <span className="text-[10px] font-mono text-zinc-500">{String(r.action ?? '')}</span>
                </legend>
                <Field label="Name" htmlFor={id('name')} error={p.name}>
                  <input
                    id={id('name')}
                    value={typeof config.name === 'string' ? config.name : ''}
                    onChange={(e) => updateConfig(index, { name: e.target.value.toLowerCase() })}
                    className={inputClass(!!p.name)}
                    spellCheck={false}
                    autoComplete="off"
                  />
                </Field>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                  <Field label="Template" htmlFor={id('template')} error={p.template_id}>
                    {templates.length > 0 ? (
                      <select
                        id={id('template')}
                        value={typeof config.template_id === 'string' ? config.template_id : ''}
                        onChange={(e) => updateConfig(index, { template_id: e.target.value })}
                        className={inputClass(!!p.template_id)}
                      >
                        {!templateKnown && (
                          <option value={typeof config.template_id === 'string' ? config.template_id : ''}>
                            {config.template_id ? `Current (${String(config.template_id).slice(0, 8)}…)` : 'Choose…'}
                          </option>
                        )}
                        {templates.map((t) => (
                          <option key={t.id} value={t.id}>
                            {t.name}
                          </option>
                        ))}
                      </select>
                    ) : (
                      <input
                        id={id('template')}
                        value={typeof config.template_id === 'string' ? config.template_id : ''}
                        onChange={(e) => updateConfig(index, { template_id: e.target.value.trim() })}
                        className={`${inputClass(!!p.template_id)} font-mono`}
                        placeholder="template id"
                      />
                    )}
                  </Field>
                  <Field
                    label="Size"
                    htmlFor={id('size')}
                    hint={sizeMismatch ? 'The values below differ from this size; the policy check decides.' : undefined}
                  >
                    <select
                      id={id('size')}
                      value={typeof config.plan_id === 'string' ? config.plan_id : ''}
                      onChange={(e) => {
                        const size = sizes.find((s) => s.id === e.target.value);
                        if (!e.target.value) updateConfig(index, { plan_id: undefined });
                        else if (size)
                          updateConfig(index, {
                            plan_id: size.id,
                            vcpus: size.vcpus,
                            memory_mb: size.memory_mb,
                            disk_gb: size.disk_gb,
                          });
                        else updateConfig(index, { plan_id: e.target.value });
                      }}
                      className={inputClass(false)}
                    >
                      <option value="">Custom</option>
                      {!sizeKnown && typeof config.plan_id === 'string' && config.plan_id && (
                        <option value={config.plan_id}>Current ({config.plan_id.slice(0, 8)}…)</option>
                      )}
                      {sizes.map((s) => (
                        <option key={s.id} value={s.id}>
                          {s.name} · {s.vcpus} vCPU · {formatMemoryMb(s.memory_mb)} · {s.disk_gb} GB
                        </option>
                      ))}
                    </select>
                  </Field>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <Field label="vCPU" htmlFor={id('vcpus')} error={p.vcpus}>
                    <input
                      id={id('vcpus')}
                      type="number"
                      min={1}
                      max={64}
                      inputMode="numeric"
                      value={typeof config.vcpus === 'number' ? config.vcpus : ''}
                      onChange={(e) => updateConfig(index, { vcpus: num(e.target.value) })}
                      className={inputClass(!!p.vcpus)}
                    />
                  </Field>
                  <Field
                    label="Memory MB"
                    htmlFor={id('memory')}
                    error={p.memory_mb}
                    hint={typeof config.memory_mb === 'number' ? `= ${formatMemoryMb(config.memory_mb)}` : undefined}
                  >
                    <input
                      id={id('memory')}
                      type="number"
                      min={256}
                      step={256}
                      inputMode="numeric"
                      value={typeof config.memory_mb === 'number' ? config.memory_mb : ''}
                      onChange={(e) => updateConfig(index, { memory_mb: num(e.target.value) })}
                      className={inputClass(!!p.memory_mb)}
                    />
                  </Field>
                  <Field label="Disk GB" htmlFor={id('disk')} error={p.disk_gb}>
                    <input
                      id={id('disk')}
                      type="number"
                      min={1}
                      inputMode="numeric"
                      value={typeof config.disk_gb === 'number' ? config.disk_gb : ''}
                      onChange={(e) => updateConfig(index, { disk_gb: num(e.target.value) })}
                      className={inputClass(!!p.disk_gb)}
                    />
                  </Field>
                </div>
              </fieldset>
            );
          })}
          {otherCount > 0 && (
            <p className="text-[10.5px] text-zinc-500">
              {otherCount} other resource{otherCount === 1 ? '' : 's'} can be edited in Advanced JSON.
            </p>
          )}
        </div>
      ) : (
        <div className="space-y-1.5">
          <label htmlFor="plan-edit-json" className="block text-[10px] font-mono uppercase tracking-wider text-zinc-500">
            specification (JSON)
          </label>
          <textarea
            id="plan-edit-json"
            value={jsonText}
            onChange={(e) => onJsonChange(e.target.value)}
            spellCheck={false}
            rows={16}
            className={`${inputClass(!!jsonError)} font-mono text-[11px] leading-relaxed resize-y min-h-[12rem]`}
          />
          {jsonError ? (
            <p className="text-[10.5px] text-rose-400 font-mono break-words" data-testid="json-error">
              {jsonError}
            </p>
          ) : (
            <p className="text-[10.5px] text-zinc-500">Tasks are re-derived from the specification by the planner.</p>
          )}
        </div>
      )}

      <div className="space-y-1.5">
        <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500">Your changes</div>
        <DiffList entries={changes} fromLabel={`v${baseRevision.version}`} toLabel="your edit" emptyText="No changes yet." />
      </div>

      {error && <ErrorNote error={error} onDismiss={() => setError(null)} />}

      <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
        <button
          type="button"
          onClick={onCancel}
          className="px-3 py-1.5 rounded-md text-xs font-medium text-zinc-300 bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 transition-colors"
        >
          Cancel
        </button>
        <button
          type="button"
          onClick={() => void save(baseVersion)}
          disabled={!canSave}
          data-testid="plan-edit-save"
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold bg-white text-zinc-950 hover:bg-zinc-200 transition-colors disabled:opacity-50"
        >
          {saving ? <Spinner /> : <Save className="w-3.5 h-3.5" />}
          <span>Save as new revision</span>
        </button>
      </div>
    </div>
  );
};
