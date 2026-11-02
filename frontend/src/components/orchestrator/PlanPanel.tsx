import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  ArrowRight,
  CircleCheck,
  ClipboardList,
  CornerDownRight,
  GitCompareArrows,
  Gauge,
  History,
  Layers,
  ListTree,
  Pencil,
  Play,
  RefreshCw,
  ShieldAlert,
  ShieldCheck,
  ThumbsDown,
  X,
} from 'lucide-react';
import type {
  ExecutionSummary,
  OrchestratorPlan,
  OrchestratorTool,
  PlanRevision,
  PlanRevisionSummary,
  PlanValidationIssue,
} from '../../types';
import {
  Catalog,
  DescribedError,
  RISK_META,
  approvalStatusTone,
  describeError,
  describeQuantity,
  diffRevisions,
  fetchPlanRevision,
  fetchPlanRevisions,
  formatDateTime,
  formatMemoryMb,
  formatRelative,
  isAwaitingApproval,
  isEditablePlan,
  loadCatalog,
  orderTasks,
  parseTime,
  shortHash,
  shortId,
  specResources,
  taskDepths,
  validatePlan,
} from '../../lib/orchestrator';
import {
  ErrorNote,
  Notice,
  NoticeTone,
  PlanStatusBadge,
  RiskBadge,
  SectionTitle,
  Spinner,
  ToneBadge,
} from './OrchestratorBits';
import { DiffList } from './RevisionDiff';
import { PlanEditor } from './PlanEditor';

export interface PanelNotice {
  tone: NoticeTone;
  text: string;
}

export interface PlanPanelProps {
  planId: string;
  plan: OrchestratorPlan | null;
  loading: boolean;
  error: DescribedError | null;
  notice: PanelNotice | null;
  onDismissNotice: () => void;
  tools: OrchestratorTool[];
  /** An active execution of this plan, if any (from the conversation's executions). */
  activeExecution: ExecutionSummary | null;
  onReload: () => void;
  onApprove: () => void;
  /** read/low plans: one-click approve + execute (no modal). Throws on failure. */
  onQuickRun: () => Promise<void>;
  /** Run an "approve only" approval. Throws on failure. */
  onRun: (approvalId: string) => Promise<void>;
  onReject: () => void;
  onPlanChanged: (plan: OrchestratorPlan, message?: string) => void;
  onShowExecution: (executionId: string) => void;
  onClose: () => void;
}

const btn = {
  primary:
    'inline-flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold bg-white text-zinc-950 hover:bg-zinc-200 transition-colors disabled:opacity-50',
  secondary:
    'inline-flex items-center justify-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium bg-zinc-900 border border-zinc-800 text-zinc-300 hover:bg-zinc-800 hover:text-white transition-colors disabled:opacity-50',
  danger:
    'inline-flex items-center justify-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium bg-rose-500/10 border border-rose-500/20 text-rose-300 hover:bg-rose-500/20 transition-colors disabled:opacity-50',
};

const configLabel: Record<string, string> = {
  name: 'Name',
  template_id: 'Template',
  plan_id: 'Size',
  vcpus: 'vCPU',
  memory_mb: 'Memory',
  disk_gb: 'Disk',
  ssh_key_ids: 'SSH keys',
};
const CONFIG_ORDER = ['name', 'template_id', 'plan_id', 'vcpus', 'memory_mb', 'disk_gb', 'ssh_key_ids'];

function configRows(config: Record<string, unknown>, catalog: Catalog | null): { key: string; label: string; value: string; mono?: boolean }[] {
  const keys = [
    ...CONFIG_ORDER.filter((k) => k in config),
    ...Object.keys(config)
      .filter((k) => !CONFIG_ORDER.includes(k))
      .sort(),
  ];
  return keys.map((k) => {
    const v = config[k];
    if (k === 'template_id' && typeof v === 'string') {
      const t = catalog?.templates.find((x) => x.id === v);
      return { key: k, label: configLabel[k], value: t ? t.name : `${v.slice(0, 8)}…`, mono: !t };
    }
    if (k === 'plan_id' && typeof v === 'string') {
      const p = catalog?.plans.find((x) => x.id === v);
      return { key: k, label: configLabel[k], value: p ? p.name : `${v.slice(0, 8)}…`, mono: !p };
    }
    if (k === 'memory_mb') return { key: k, label: configLabel[k], value: formatMemoryMb(v) };
    if (k === 'disk_gb') return { key: k, label: configLabel[k], value: `${String(v)} GB` };
    if (k === 'ssh_key_ids') {
      const n = Array.isArray(v) ? v.length : 0;
      return { key: k, label: configLabel[k], value: n === 0 ? 'none' : `${n} key${n === 1 ? '' : 's'}` };
    }
    const d = describeQuantity(k, v);
    return { key: k, label: configLabel[k] ?? d.label, value: d.value, mono: typeof v === 'object' };
  });
}

/** Validation errors/warnings with their task keys (clicking a key highlights the task). */
const ValidationBox: React.FC<{
  revision: PlanRevision;
  onFocusTask: (key: string) => void;
  onRevalidate?: () => void;
  revalidating?: boolean;
}> = ({ revision, onFocusTask, onRevalidate, revalidating }) => {
  const v = revision.validation;
  const status = v?.status ?? 'not run';
  const tone = status === 'passed' ? 'emerald' : status === 'warnings' ? 'amber' : status === 'failed' ? 'rose' : 'zinc';
  const Row: React.FC<{ issue: PlanValidationIssue; kind: 'error' | 'warning' }> = ({ issue, kind }) => (
    <li
      className={`flex flex-wrap items-start gap-x-1.5 gap-y-0.5 text-[11px] ${kind === 'error' ? 'text-rose-200' : 'text-amber-200'}`}
      data-validation={kind}
    >
      {issue.code && <span className="font-mono text-[10px] opacity-80">{issue.code}</span>}
      <span className="min-w-0 break-words flex-1">{issue.message}</span>
      {issue.task_key && (
        <button
          type="button"
          onClick={() => onFocusTask(issue.task_key!)}
          className="font-mono text-[10px] px-1.5 py-px rounded border border-zinc-700 bg-zinc-900 text-zinc-300 hover:text-white max-w-full truncate"
          title="Show this task"
        >
          {issue.task_key}
        </button>
      )}
    </li>
  );
  return (
    <section>
      <SectionTitle
        icon={ShieldCheck}
        right={
          onRevalidate && (
            <button
              type="button"
              onClick={onRevalidate}
              disabled={revalidating}
              className="inline-flex items-center gap-1 text-[10.5px] text-zinc-400 hover:text-white disabled:opacity-50"
            >
              <RefreshCw className={`w-3 h-3 ${revalidating ? 'animate-spin' : ''}`} />
              Re-validate
            </button>
          )
        }
      >
        Validation
      </SectionTitle>
      <div className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2 space-y-1.5" data-testid="plan-validation">
        <div className="flex flex-wrap items-center gap-2">
          <ToneBadge tone={tone} label={status} />
          {v?.checked_at && <span className="text-[10px] font-mono text-zinc-500">checked {formatDateTime(v.checked_at)}</span>}
        </div>
        {v && v.errors.length > 0 && (
          <ul className="space-y-1">
            {v.errors.map((e, i) => (
              <Row key={`e${i}`} issue={e} kind="error" />
            ))}
          </ul>
        )}
        {v && v.warnings.length > 0 && (
          <ul className="space-y-1">
            {v.warnings.map((w, i) => (
              <Row key={`w${i}`} issue={w} kind="warning" />
            ))}
          </ul>
        )}
      </div>
    </section>
  );
};

/** Task DAG as an indented list with "after:" chips. */
export const TaskDagList: React.FC<{
  tasks: PlanRevision['tasks'];
  highlightKey?: string | null;
  issuesByKey?: Map<string, 'error' | 'warning'>;
  compact?: boolean;
}> = ({ tasks, highlightKey, issuesByKey, compact = false }) => {
  const ordered = useMemo(() => orderTasks(tasks), [tasks]);
  const depths = useMemo(() => taskDepths(tasks), [tasks]);
  const titleOf = useMemo(() => new Map(tasks.map((t) => [t.key, t.title])), [tasks]);
  if (tasks.length === 0) return <div className="text-[11px] text-zinc-500 italic">No tasks.</div>;
  return (
    <ol className="space-y-1" data-testid="plan-tasks">
      {ordered.map((t, i) => {
        const depth = Math.min(depths.get(t.key) ?? 0, 4);
        const issue = issuesByKey?.get(t.key);
        const hl = highlightKey === t.key;
        return (
          <li
            key={t.key}
            data-task-key={t.key}
            className={`rounded-md border px-2 py-1.5 transition-colors ${
              hl ? 'border-sky-500/50 bg-sky-500/5' : 'border-zinc-800 bg-zinc-950/60'
            }`}
            style={{ marginLeft: `${depth * 10}px` }}
          >
            <div className="flex items-start gap-1.5 min-w-0">
              {depth > 0 ? (
                <CornerDownRight className="w-3 h-3 mt-0.5 text-zinc-600 flex-shrink-0" />
              ) : (
                <span className="w-3 text-[10px] font-mono text-zinc-600 flex-shrink-0 text-right">{i + 1}</span>
              )}
              <div className="flex-1 min-w-0">
                <div className="flex flex-wrap items-center gap-1.5">
                  <span className="text-[11.5px] text-zinc-100 break-words min-w-0">{t.title}</span>
                  <RiskBadge risk={t.risk} withLabel={false} compact />
                  {issue && (
                    <span
                      className={`text-[10px] font-mono ${issue === 'error' ? 'text-rose-400' : 'text-amber-400'}`}
                      title={`This task has a validation ${issue}`}
                    >
                      {issue}
                    </span>
                  )}
                </div>
                {!compact && <div className="text-[10px] font-mono text-zinc-500 break-all">{t.tool}</div>}
                {t.depends_on.length > 0 && (
                  <div className="mt-0.5 flex flex-wrap items-center gap-1">
                    <span className="text-[10px] text-zinc-500">after</span>
                    {t.depends_on.map((dep) => (
                      <span
                        key={dep}
                        title={titleOf.get(dep) ?? dep}
                        className="font-mono text-[10px] px-1.5 py-px rounded border border-zinc-800 bg-zinc-900 text-zinc-400 max-w-full truncate"
                      >
                        {dep}
                      </span>
                    ))}
                  </div>
                )}
              </div>
            </div>
          </li>
        );
      })}
    </ol>
  );
};

export const EstimatesGrid: React.FC<{ estimates: Record<string, unknown> }> = ({ estimates }) => {
  const entries = Object.entries(estimates);
  if (entries.length === 0) return <div className="text-[11px] text-zinc-500 italic">No estimate.</div>;
  return (
    <div className="grid grid-cols-2 gap-1.5" data-testid="plan-estimates">
      {entries.map(([k, v]) => {
        const d = describeQuantity(k, v);
        return (
          <div key={k} className="rounded-md border border-zinc-800 bg-zinc-950/60 px-2 py-1.5 min-w-0">
            <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500 truncate">{d.label}</div>
            <div className="text-xs font-semibold text-zinc-100 truncate">{d.value}</div>
          </div>
        );
      })}
    </div>
  );
};

export const ResourceList: React.FC<{ revision: PlanRevision; catalog: Catalog | null }> = ({ revision, catalog }) => {
  const resources = specResources(revision.specification);
  if (resources.length === 0) return <div className="text-[11px] text-zinc-500 italic">No resources in the specification.</div>;
  return (
    <div className="space-y-1.5" data-testid="plan-resources">
      {resources.map((r) => (
        <div key={r.key} className="rounded-md border border-zinc-800 bg-zinc-950/60 p-2 space-y-1.5 min-w-0">
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="text-xs font-semibold text-white break-all">{String(r.config.name ?? r.key)}</span>
            <span className="text-[10px] font-mono px-1.5 py-px rounded border border-zinc-800 bg-zinc-900 text-zinc-400 uppercase">
              {r.type}
            </span>
            {r.action && (
              <span
                className={`text-[10px] font-mono ${
                  r.action === 'delete' ? 'text-rose-400' : r.action === 'create' ? 'text-emerald-400' : 'text-amber-400'
                }`}
              >
                {r.action}
              </span>
            )}
          </div>
          <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-[11px]">
            {configRows(r.config, catalog).map((row) => (
              <React.Fragment key={row.key}>
                <dt className="text-zinc-500">{row.label}</dt>
                <dd className={`text-zinc-200 break-all ${row.mono ? 'font-mono text-[10.5px]' : ''}`}>{row.value}</dd>
              </React.Fragment>
            ))}
          </dl>
        </div>
      ))}
    </div>
  );
};

const ApprovalBox: React.FC<{
  plan: OrchestratorPlan;
  activeExecution: ExecutionSummary | null;
  busy: 'run' | 'quick' | 'validate' | null;
  onApprove: () => void;
  onRun: (approvalId: string) => void;
  onQuickRun: () => void;
  onEdit: (() => void) | null;
  onReject: (() => void) | null;
  onShowExecution: (id: string) => void;
}> = ({ plan, activeExecution, busy, onApprove, onRun, onQuickRun, onEdit, onReject, onShowExecution }) => {
  const rev = plan.revision;
  const approval = plan.approval;
  const risk = rev?.risk_level ?? 'medium';
  const now = Date.now();
  const expired = !!approval && (approval.status === 'expired' || (parseTime(approval.expires_at) ?? Infinity) < now);
  const usable = !!approval && approval.status === 'approved' && !expired && approval.version === plan.current_version;
  const awaiting = isAwaitingApproval(plan.status) || plan.status === 'draft';
  const reapprovable = plan.status === 'approved' && !usable;
  const validationFailed = rev?.validation?.status === 'failed';
  const needsApproval = rev ? rev.requires_approval : true;
  const canApprove = !!rev && needsApproval && (awaiting || reapprovable) && !activeExecution;
  const canQuickRun = !!rev && !needsApproval && awaiting && !activeExecution;

  let line: React.ReactNode;
  let tone: NoticeTone = 'info';
  if (activeExecution) {
    line = <>An execution of this plan is running.</>;
  } else if (usable) {
    tone = 'success';
    line = (
      <>
        Approved v{approval!.version} · expires {formatRelative(approval!.expires_at)}. Run it when you are ready.
      </>
    );
  } else if (approval && approval.status === 'superseded') {
    tone = 'warning';
    line = (
      <>
        The approval was for v{approval.version}; the plan changed (now v{plan.current_version}). Review and approve again.
      </>
    );
  } else if (approval && expired && approval.status !== 'consumed') {
    tone = 'warning';
    line = <>The approval for v{approval.version} expired. Approve again to run it.</>;
  } else if (plan.status === 'rejected' || approval?.status === 'rejected') {
    tone = 'error';
    line = <>This plan was rejected.</>;
  } else if (awaiting && !needsApproval) {
    line = <>{RISK_META[risk].label} risk: no approval needed. Run it when you are ready.</>;
  } else if (awaiting) {
    tone = 'warning';
    line = (
      <>
        Waiting for your approval · {RISK_META[risk].approval}
        {RISK_META[risk].expiryMinutes > 0 && <> · valid {RISK_META[risk].expiryMinutes} min</>}.
      </>
    );
  } else if (approval?.status === 'consumed') {
    line = <>Approval v{approval.version} was used by an execution.</>;
  } else {
    line = <>Plan status: {plan.status.replace(/_/g, ' ')}.</>;
  }

  return (
    <section className="space-y-2" data-testid="approval-box">
      <Notice tone={tone}>
        <span>{line}</span>
        {approval && (
          <span className="ml-1.5 inline-flex align-middle">
            <ToneBadge tone={approvalStatusTone(expired && approval.status === 'approved' ? 'expired' : approval.status)} label={`approval ${expired && approval.status === 'approved' ? 'expired' : approval.status}`} />
          </span>
        )}
      </Notice>
      {validationFailed && (canApprove || canQuickRun) && (
        <p className="text-[10.5px] text-rose-300">Fix the validation errors (edit the plan) before approving.</p>
      )}
      <div className="flex flex-wrap gap-1.5">
        {canApprove && (
          <button type="button" onClick={onApprove} disabled={validationFailed} className={btn.primary} data-testid="plan-approve">
            <ShieldAlert className="w-3.5 h-3.5" />
            <span>{reapprovable ? 'Approve again…' : 'Review & approve…'}</span>
          </button>
        )}
        {canQuickRun && (
          <button type="button" onClick={onQuickRun} disabled={busy !== null || validationFailed} className={btn.primary} data-testid="plan-quick-run">
            {busy === 'quick' ? <Spinner /> : <Play className="w-3.5 h-3.5" />}
            <span>Run</span>
          </button>
        )}
        {usable && !activeExecution && (
          <button type="button" onClick={() => onRun(approval!.id)} disabled={busy !== null} className={btn.primary} data-testid="plan-run">
            {busy === 'run' ? <Spinner /> : <Play className="w-3.5 h-3.5" />}
            <span>Run</span>
          </button>
        )}
        {(activeExecution || plan.latest_execution_id) && (
          <button
            type="button"
            onClick={() => onShowExecution(activeExecution?.id ?? plan.latest_execution_id!)}
            className={btn.secondary}
          >
            <ArrowRight className="w-3.5 h-3.5" />
            <span>{activeExecution ? 'Watch execution' : 'Show execution'}</span>
          </button>
        )}
        {onEdit && (
          <button type="button" onClick={onEdit} className={btn.secondary} data-testid="plan-edit">
            <Pencil className="w-3.5 h-3.5" />
            <span>Edit</span>
          </button>
        )}
        {onReject && (
          <button type="button" onClick={onReject} className={btn.danger} data-testid="plan-reject">
            <ThumbsDown className="w-3.5 h-3.5" />
            <span>Reject</span>
          </button>
        )}
      </div>
    </section>
  );
};

/**
 * Right-hand panel for the active plan: summary, approval state + actions, validation, resources,
 * task DAG, estimates, revision selector with a diff between any two revisions, and the editor.
 */
export const PlanPanel: React.FC<PlanPanelProps> = ({
  planId,
  plan,
  loading,
  error,
  notice,
  onDismissNotice,
  activeExecution,
  onReload,
  onApprove,
  onQuickRun,
  onRun,
  onReject,
  onPlanChanged,
  onShowExecution,
  onClose,
}) => {
  const [revisions, setRevisions] = useState<PlanRevisionSummary[] | null>(null);
  const [revCache, setRevCache] = useState<Record<number, PlanRevision>>({});
  const [revError, setRevError] = useState<DescribedError | null>(null);
  const [viewVersion, setViewVersion] = useState<number | null>(null);
  const [compare, setCompare] = useState<{ from: number; to: number } | null>(null);
  const [editing, setEditing] = useState(false);
  const [busy, setBusy] = useState<'run' | 'quick' | 'validate' | null>(null);
  const [actionError, setActionError] = useState<DescribedError | null>(null);
  const [highlightKey, setHighlightKey] = useState<string | null>(null);
  const [catalog, setCatalog] = useState<Catalog | null>(null);
  const loadingVersions = useRef(new Set<number>());
  const bodyRef = useRef<HTMLDivElement>(null);

  // New plan: reset everything that belongs to the previous one.
  useEffect(() => {
    setRevisions(null);
    setRevCache({});
    setRevError(null);
    setViewVersion(null);
    setCompare(null);
    setEditing(false);
    setActionError(null);
    setHighlightKey(null);
    loadingVersions.current.clear();
  }, [planId]);

  useEffect(() => {
    let live = true;
    loadCatalog().then((c) => live && setCatalog(c));
    return () => {
      live = false;
    };
  }, []);

  const currentVersion = plan?.current_version ?? 0;
  const currentRevision = plan?.revision ?? null;

  // Revision list (refreshed whenever a new revision becomes current).
  useEffect(() => {
    if (!plan) return;
    const controller = new AbortController();
    fetchPlanRevisions(plan.id, controller.signal)
      .then((list) => {
        setRevisions(list);
        setRevError(null);
      })
      .catch((err) => {
        if (!controller.signal.aborted) setRevError(describeError(err, 'Could not load the revision history'));
      });
    return () => controller.abort();
  }, [plan?.id, currentVersion]);

  useEffect(() => {
    if (currentRevision) setRevCache((c) => (c[currentRevision.version] === currentRevision ? c : { ...c, [currentRevision.version]: currentRevision }));
  }, [currentRevision]);

  const ensureRevision = useCallback(
    (version: number) => {
      if (!plan || revCache[version] || loadingVersions.current.has(version)) return;
      loadingVersions.current.add(version);
      fetchPlanRevision(plan.id, version)
        .then((rev) => setRevCache((c) => ({ ...c, [version]: rev })))
        .catch((err) => setRevError(describeError(err, `Could not load v${version}`)))
        .finally(() => loadingVersions.current.delete(version));
    },
    [plan, revCache]
  );

  useEffect(() => {
    if (viewVersion !== null && viewVersion !== currentVersion) ensureRevision(viewVersion);
  }, [viewVersion, currentVersion, ensureRevision]);

  useEffect(() => {
    if (compare) {
      ensureRevision(compare.from);
      ensureRevision(compare.to);
    }
  }, [compare, ensureRevision]);

  const focusTask = (key: string) => {
    setHighlightKey(key);
    requestAnimationFrame(() => {
      const el = bodyRef.current?.querySelector(`[data-task-key="${CSS.escape(key)}"]`);
      el?.scrollIntoView({ behavior: 'smooth', block: 'center' });
    });
  };

  const viewing = viewVersion !== null && viewVersion !== currentVersion ? revCache[viewVersion] ?? null : currentRevision;
  const viewingOld = viewVersion !== null && viewVersion !== currentVersion;

  const issuesByKey = useMemo(() => {
    const m = new Map<string, 'error' | 'warning'>();
    for (const w of viewing?.validation?.warnings ?? []) if (w.task_key) m.set(w.task_key, 'warning');
    for (const e of viewing?.validation?.errors ?? []) if (e.task_key) m.set(e.task_key, 'error');
    return m;
  }, [viewing]);

  const versionOptions = useMemo(() => {
    const list = revisions ? revisions.map((r) => r.version) : [];
    if (currentVersion && !list.includes(currentVersion)) list.unshift(currentVersion);
    return list.sort((a, b) => b - a);
  }, [revisions, currentVersion]);

  const run = async (kind: 'run' | 'quick', fn: () => Promise<void>) => {
    setBusy(kind);
    setActionError(null);
    try {
      await fn();
    } catch (err) {
      setActionError(describeError(err));
    } finally {
      setBusy(null);
    }
  };

  const revalidate = async () => {
    if (!plan) return;
    setBusy('validate');
    setActionError(null);
    try {
      await validatePlan(plan.id);
      onReload();
    } catch (err) {
      setActionError(describeError(err));
    } finally {
      setBusy(null);
    }
  };

  const header = (
    <div className="flex items-start justify-between gap-2 px-3 py-2.5 border-b border-zinc-800">
      <div className="min-w-0">
        <div className="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-500">
          <ClipboardList className="w-3 h-3" />
          <span>Plan</span>
          <span className="text-zinc-600 normal-case">{shortId(planId)}</span>
        </div>
        <div className="text-sm font-semibold text-white leading-snug break-words" data-testid="plan-title">
          {plan?.title ?? (loading ? 'Loading plan…' : 'Plan')}
        </div>
      </div>
      <div className="flex items-center gap-0.5 flex-shrink-0">
        <button
          type="button"
          onClick={onReload}
          disabled={loading}
          title="Reload plan"
          className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-50"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
        </button>
        <button
          type="button"
          onClick={onClose}
          title="Close plan"
          className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
          data-testid="plan-close"
        >
          <X className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  );

  if (!plan) {
    return (
      <div className="flex flex-col h-full min-h-0">
        {header}
        <div className="p-3 space-y-3">
          {loading ? (
            <div className="flex items-center gap-2 text-xs text-zinc-400">
              <Spinner /> Loading plan…
            </div>
          ) : error ? (
            <>
              <ErrorNote error={error} />
              <button type="button" onClick={onReload} className={btn.secondary}>
                <RefreshCw className="w-3.5 h-3.5" /> Retry
              </button>
            </>
          ) : null}
        </div>
      </div>
    );
  }

  const rev = currentRevision;
  const editable = isEditablePlan(plan.status) && !!rev && !activeExecution;
  const rejectable = (isAwaitingApproval(plan.status) || plan.status === 'draft' || plan.status === 'approved') && !activeExecution;

  return (
    <div className="flex flex-col h-full min-h-0" data-testid="plan-panel" data-plan-id={plan.id} data-plan-version={currentVersion}>
      {header}
      <div ref={bodyRef} className="flex-1 overflow-y-auto overflow-x-hidden p-3 space-y-4">
        {notice && (
          <Notice tone={notice.tone} onDismiss={onDismissNotice}>
            <span data-testid="plan-notice">{notice.text}</span>
          </Notice>
        )}
        {error && <ErrorNote error={error} />}

        {editing && rev ? (
          <PlanEditor
            plan={plan}
            onCancel={() => setEditing(false)}
            onSaved={(updated) => {
              const hadApproval = plan.approval?.status === 'approved' || plan.status === 'approved';
              setEditing(false);
              setViewVersion(null);
              onPlanChanged(
                updated,
                hadApproval
                  ? `Saved as v${updated.current_version}. The previous approval was cancelled; review and approve the new revision.`
                  : `Saved as v${updated.current_version}. Earlier approvals no longer apply.`
              );
              // Re-read status + approval as the server now sees them (approval superseded, plan back to awaiting_approval).
              onReload();
            }}
            onPlanReloaded={(latest) => onPlanChanged(latest)}
          />
        ) : (
          <>
            <div className="flex flex-wrap items-center gap-1.5">
              <PlanStatusBadge status={plan.status} />
              {rev && <RiskBadge risk={rev.risk_level} />}
              <span className="text-[10.5px] font-mono text-zinc-400" data-testid="plan-version">
                v{currentVersion}
              </span>
              {rev?.spec_hash && (
                <span className="text-[10px] font-mono text-zinc-600 truncate max-w-full" title={rev.spec_hash}>
                  {shortHash(rev.spec_hash)}
                </span>
              )}
            </div>

            {/* Revision selector + compare */}
            <section className="space-y-2">
              <div className="flex flex-wrap items-center gap-1.5">
                <History className="w-3 h-3 text-zinc-500" />
                <label htmlFor="plan-revision-select" className="text-[10.5px] text-zinc-400">
                  Revision
                </label>
                <select
                  id="plan-revision-select"
                  value={viewVersion ?? currentVersion}
                  onChange={(e) => setViewVersion(Number(e.target.value))}
                  className="bg-zinc-900 border border-zinc-800 rounded-md px-1.5 py-0.5 text-[11px] text-zinc-200 focus:outline-none focus:border-zinc-600"
                >
                  {versionOptions.map((v) => {
                    const meta = revisions?.find((r) => r.version === v);
                    return (
                      <option key={v} value={v}>
                        v{v}
                        {v === currentVersion ? ' (current)' : ''}
                        {meta ? ` · ${meta.created_by}` : ''}
                      </option>
                    );
                  })}
                </select>
                {versionOptions.length > 1 && (
                  <button
                    type="button"
                    onClick={() =>
                      setCompare((c) =>
                        c
                          ? null
                          : {
                              from: versionOptions.find((v) => v < (viewVersion ?? currentVersion)) ?? versionOptions[versionOptions.length - 1],
                              to: viewVersion ?? currentVersion,
                            }
                      )
                    }
                    aria-pressed={!!compare}
                    className={`inline-flex items-center gap-1 px-1.5 py-0.5 rounded-md border text-[10.5px] transition-colors ${
                      compare ? 'bg-zinc-800 border-zinc-700 text-white' : 'bg-zinc-900 border-zinc-800 text-zinc-400 hover:text-zinc-200'
                    }`}
                    data-testid="plan-compare-toggle"
                  >
                    <GitCompareArrows className="w-3 h-3" />
                    Compare
                  </button>
                )}
              </div>
              {revError && <ErrorNote error={revError} onDismiss={() => setRevError(null)} />}
              {compare && (
                <div className="rounded-md border border-zinc-800 bg-zinc-900/40 p-2 space-y-2" data-testid="plan-compare">
                  <div className="flex flex-wrap items-center gap-1.5 text-[10.5px] text-zinc-400">
                    <span>From</span>
                    <select
                      aria-label="Compare from version"
                      value={compare.from}
                      onChange={(e) => setCompare({ ...compare, from: Number(e.target.value) })}
                      className="bg-zinc-900 border border-zinc-800 rounded px-1 py-0.5 text-[11px] text-zinc-200"
                    >
                      {versionOptions.map((v) => (
                        <option key={v} value={v}>
                          v{v}
                        </option>
                      ))}
                    </select>
                    <ArrowRight className="w-3 h-3" />
                    <select
                      aria-label="Compare to version"
                      value={compare.to}
                      onChange={(e) => setCompare({ ...compare, to: Number(e.target.value) })}
                      className="bg-zinc-900 border border-zinc-800 rounded px-1 py-0.5 text-[11px] text-zinc-200"
                    >
                      {versionOptions.map((v) => (
                        <option key={v} value={v}>
                          v{v}
                        </option>
                      ))}
                    </select>
                  </div>
                  {revCache[compare.from] && revCache[compare.to] ? (
                    <DiffList
                      entries={diffRevisions(revCache[compare.from], revCache[compare.to])}
                      fromLabel={`v${compare.from}`}
                      toLabel={`v${compare.to}`}
                      emptyText={compare.from === compare.to ? 'Same revision.' : 'No field changes.'}
                    />
                  ) : (
                    <div className="flex items-center gap-2 text-[11px] text-zinc-500">
                      <Spinner className="w-3 h-3" /> Loading revisions…
                    </div>
                  )}
                </div>
              )}
              {viewingOld && (
                <Notice tone="info">
                  Viewing v{viewVersion} (read only). Approval always applies to the current revision, v{currentVersion}.{' '}
                  <button type="button" onClick={() => setViewVersion(null)} className="underline hover:text-white">
                    Show current
                  </button>
                </Notice>
              )}
            </section>

            {viewing ? (
              <>
                <p className="text-xs text-zinc-300 leading-relaxed break-words" data-testid="plan-summary">
                  {viewing.summary || plan.description || 'No summary.'}
                </p>
                {viewing.created_at && (
                  <div className="text-[10px] font-mono text-zinc-500 -mt-3">
                    v{viewing.version} by {viewing.created_by} · {formatDateTime(viewing.created_at)}
                  </div>
                )}

                {!viewingOld && (
                  <>
                    <ApprovalBox
                      plan={plan}
                      activeExecution={activeExecution}
                      busy={busy}
                      onApprove={onApprove}
                      onRun={(approvalId) => run('run', () => onRun(approvalId))}
                      onQuickRun={() => run('quick', onQuickRun)}
                      onEdit={editable ? () => setEditing(true) : null}
                      onReject={rejectable ? onReject : null}
                      onShowExecution={onShowExecution}
                    />
                    {actionError && <ErrorNote error={actionError} onDismiss={() => setActionError(null)} />}
                  </>
                )}

                <ValidationBox
                  revision={viewing}
                  onFocusTask={focusTask}
                  onRevalidate={!viewingOld && editable ? revalidate : undefined}
                  revalidating={busy === 'validate'}
                />

                <section>
                  <SectionTitle icon={Layers}>Resources</SectionTitle>
                  <ResourceList revision={viewing} catalog={catalog} />
                </section>

                <section>
                  <SectionTitle icon={ListTree} right={<span className="text-[10px] font-mono text-zinc-600">{viewing.tasks.length} tasks</span>}>
                    Tasks
                  </SectionTitle>
                  <TaskDagList tasks={viewing.tasks} highlightKey={highlightKey} issuesByKey={issuesByKey} />
                </section>

                <section>
                  <SectionTitle icon={Gauge}>Estimated resources</SectionTitle>
                  <EstimatesGrid estimates={viewing.estimated_resources} />
                </section>

                {viewing.confirmation_required && (
                  <section>
                    <SectionTitle icon={CircleCheck}>Confirmation</SectionTitle>
                    <p className="text-[11px] text-zinc-400">
                      Approving needs the exact name{viewing.confirmation_required.expected.length > 1 ? 's' : ''}:{' '}
                      <code className="font-mono text-rose-300 break-all">{viewing.confirmation_required.expected.join(', ')}</code>
                    </p>
                  </section>
                )}
              </>
            ) : viewingOld ? (
              <div className="flex items-center gap-2 text-[11px] text-zinc-500">
                <Spinner className="w-3 h-3" /> Loading v{viewVersion}…
              </div>
            ) : (
              <Notice tone="warning">
                This plan has no revision data yet, so it cannot be reviewed or approved here. Reload, or ask the agent to plan
                again.
              </Notice>
            )}
          </>
        )}
      </div>
    </div>
  );
};
