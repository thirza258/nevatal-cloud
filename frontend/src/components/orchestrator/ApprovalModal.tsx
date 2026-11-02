import React, { useMemo, useState } from 'react';
import { Gauge, Layers, ListTree, LifeBuoy, ShieldAlert, ShieldCheck, TriangleAlert, Users, Zap } from 'lucide-react';
import { Modal } from '../ui/Modal';
import type { ApprovePlanResult, OrchestratorPlan, OrchestratorTool, PlanRevision } from '../../types';
import {
  ApproveRequest,
  DescribedError,
  DiffEntry,
  RECOVERY_NOTES,
  RISK_META,
  approvePlan,
  consequencesFor,
  describeError,
  diffRevisions,
  errorCodeOf,
  expectedConfirmation,
  fetchPlan,
  isAwaitingApproval,
  shortHash,
  specResources,
} from '../../lib/orchestrator';
import { ErrorNote, Notice, RiskBadge, SectionTitle, Spinner } from './OrchestratorBits';
import { EstimatesGrid, TaskDagList } from './PlanPanel';
import { DiffList } from './RevisionDiff';

export interface ApprovalModalProps {
  /** Mount only while open: the revision shown is snapshotted on mount. */
  plan: OrchestratorPlan;
  tools: OrchestratorTool[];
  isAdmin: boolean;
  mockHypervisor: boolean;
  onClose: () => void;
  onApproved: (result: ApprovePlanResult, executeNow: boolean) => void;
  /** The modal re-fetched the plan (PLAN_VERSION_MISMATCH): the caller should show the new revision too. */
  onPlanReloaded: (plan: OrchestratorPlan) => void;
}

const RISK_HEADLINE: Record<string, string> = {
  read: 'Reads data only.',
  low: 'Inspects or plans only.',
  medium: 'Creates or starts resources.',
  high: 'Changes running resources. Services can be interrupted.',
  destructive: 'Deletes or overwrites data. This cannot be undone.',
};

/**
 * Approval for one revision. Always sends `version` + `spec_hash` of the revision shown; high adds
 * `acknowledge_risks`, destructive adds `acknowledge_risks` + the typed `confirmation`.
 */
export const ApprovalModal: React.FC<ApprovalModalProps> = ({
  plan,
  tools,
  isAdmin,
  mockHypervisor,
  onClose,
  onApproved,
  onPlanReloaded,
}) => {
  const [snap, setSnap] = useState<PlanRevision | null>(plan.revision);
  const [planStatus, setPlanStatus] = useState(plan.status);
  const [ack, setAck] = useState(false);
  const [typed, setTyped] = useState('');
  const [executeNow, setExecuteNow] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<DescribedError | null>(null);
  const [changed, setChanged] = useState<{ from: number; to: number; diff: DiffEntry[] } | null>(null);

  const toolById = useMemo(() => new Map(tools.map((t) => [t.id, t])), [tools]);

  if (!snap) {
    return (
      <Modal isOpen onClose={onClose} title="Approve plan" maxWidth="lg">
        <Notice tone="warning">This plan has no revision data, so it cannot be approved here.</Notice>
      </Modal>
    );
  }

  const risk = snap.risk_level;
  const meta = RISK_META[risk];
  const needsAck = risk === 'high' || risk === 'destructive';
  const destructive = risk === 'destructive';
  const expected = expectedConfirmation(snap);
  const typedOk = !destructive || (expected !== null ? typed === expected : typed.trim().length > 0);
  const validationFailed = snap.validation?.status === 'failed';
  const stillAwaiting = isAwaitingApproval(planStatus) || planStatus === 'draft' || planStatus === 'approved';
  const canSubmit = !submitting && (!needsAck || ack) && typedOk && !validationFailed && stillAwaiting;

  const resources = specResources(snap.specification);
  const consequences = consequencesFor(snap.tasks);
  const adminTools = Array.from(new Set(snap.tasks.map((t) => t.tool))).filter((id) => toolById.get(id)?.requires_admin);
  const approvalTasks = snap.tasks.filter((t) => t.risk === 'medium' || t.risk === 'high' || t.risk === 'destructive');
  const confirmError = error && (error.code === 'CONFIRMATION_MISMATCH' || error.code === 'CONFIRMATION_REQUIRED');
  const ackError = error?.code === 'RISK_ACK_REQUIRED';

  const submit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    setError(null);
    const body: ApproveRequest = {
      version: snap.version,
      spec_hash: snap.spec_hash,
      acknowledge_risks: needsAck ? ack : false,
      execute: executeNow,
    };
    if (destructive) body.confirmation = typed;
    try {
      const result = await approvePlan(plan.id, body);
      onApproved(result, executeNow);
    } catch (err) {
      if (errorCodeOf(err) === 'PLAN_VERSION_MISMATCH') {
        try {
          const latest = await fetchPlan(plan.id);
          onPlanReloaded(latest);
          setPlanStatus(latest.status);
          if (latest.revision) {
            setChanged({
              from: snap.version,
              to: latest.revision.version,
              diff: diffRevisions(snap, latest.revision),
            });
            setSnap(latest.revision);
          } else {
            setError(describeError(err));
          }
          // Everything the user confirmed applied to the old revision.
          setAck(false);
          setTyped('');
        } catch (reloadErr) {
          setError(describeError(reloadErr));
        }
      } else {
        setError(describeError(err));
      }
    } finally {
      setSubmitting(false);
    }
  };

  const approveLabel = executeNow ? (destructive ? 'Approve & delete' : 'Approve & run') : 'Approve';
  // The controls sit below the details: say what is missing next to the disabled button.
  const blockedReason = submitting
    ? null
    : !stillAwaiting
      ? 'This plan can no longer be approved.'
      : validationFailed
        ? 'Fix the validation errors first.'
        : needsAck && !ack
          ? 'Tick “I understand the risks” below.'
          : !typedOk
            ? 'Type the resource names below to confirm.'
            : null;

  return (
    <Modal
      isOpen
      onClose={submitting ? () => {} : onClose}
      title={destructive ? 'Approve destructive plan' : 'Approve plan'}
      description={plan.title}
      maxWidth="xl"
    >
      <div
        className="max-h-[calc(100dvh-10rem)] overflow-y-auto overflow-x-hidden -mx-1 px-1 space-y-4"
        data-testid="approval-modal"
        data-risk={risk}
        data-version={snap.version}
      >
        {changed && (
          <div className="space-y-2 p-2.5 rounded-md border border-amber-500/30 bg-amber-500/5" data-testid="approval-plan-changed">
            <div className="flex items-start gap-2 text-[11px] text-amber-200">
              <TriangleAlert className="w-3.5 h-3.5 mt-px text-amber-400 flex-shrink-0" />
              <span>
                The plan changed while you were reviewing it (v{changed.from} → v{changed.to}). Nothing was approved. Review
                the changes and the updated details below, then approve again.
              </span>
            </div>
            <DiffList entries={changed.diff} fromLabel={`v${changed.from}`} toLabel={`v${changed.to}`} emptyText="No field changes." />
          </div>
        )}

        {!stillAwaiting && (
          <Notice tone="warning">This plan is {planStatus.replace(/_/g, ' ')} and can no longer be approved.</Notice>
        )}

        {/* Risk banner */}
        <div className={`rounded-md border p-2.5 space-y-1 ${meta.panel}`}>
          <div className="flex flex-wrap items-center gap-2">
            <RiskBadge risk={risk} />
            <span className={`text-xs font-semibold ${meta.text}`}>{RISK_HEADLINE[risk]}</span>
          </div>
          {destructive && (
            <p className="text-[11px] text-rose-200 leading-relaxed" data-testid="destructive-warning">
              <ShieldAlert className="inline w-3.5 h-3.5 -mt-0.5 mr-1 text-rose-400" />
              This permanently removes data. Nevatal cannot restore it unless a snapshot or backup exists.
            </p>
          )}
        </div>

        {/* Operation */}
        <section>
          <SectionTitle icon={Zap}>Operation</SectionTitle>
          <p className="text-xs text-zinc-200 leading-relaxed break-words">{snap.summary || plan.title}</p>
          <p className="mt-0.5 text-[10px] font-mono text-zinc-500 break-all">
            revision v{snap.version} · {shortHash(snap.spec_hash)}
          </p>
        </section>

        {/* Resources affected */}
        <section>
          <SectionTitle icon={Layers}>Resources affected</SectionTitle>
          {resources.length === 0 ? (
            <p className="text-[11px] text-zinc-500 italic">No resources listed.</p>
          ) : (
            <ul className="space-y-1">
              {resources.map((r) => (
                <li key={r.key} className="flex flex-wrap items-center gap-1.5 text-[11px]">
                  <span
                    className={`font-mono text-[10px] px-1.5 py-px rounded border ${
                      r.action === 'delete'
                        ? 'border-rose-500/30 bg-rose-500/10 text-rose-300'
                        : 'border-zinc-800 bg-zinc-900 text-zinc-400'
                    }`}
                  >
                    {r.action || 'use'}
                  </span>
                  <span className="text-zinc-100 font-medium break-all">{String(r.config.name ?? r.key)}</span>
                  <span className="text-[10px] font-mono text-zinc-500 uppercase">{r.type}</span>
                </li>
              ))}
            </ul>
          )}
        </section>

        {/* Estimates */}
        <section>
          <SectionTitle icon={Gauge}>Estimates</SectionTitle>
          <EstimatesGrid estimates={snap.estimated_resources} />
        </section>

        {/* Consequences */}
        <section>
          <SectionTitle icon={TriangleAlert}>Consequences</SectionTitle>
          {consequences.length === 0 ? (
            <p className="text-[11px] text-zinc-400">Nothing is changed.</p>
          ) : (
            <ul className="space-y-1">
              {consequences.map((c) => (
                <li key={c.tool} className="flex items-start gap-1.5 text-[11px] text-zinc-300">
                  <span className={`mt-1.5 w-1.5 h-1.5 rounded-full flex-shrink-0 ${RISK_META[c.risk].dot}`} />
                  <span className="min-w-0 break-words">
                    <code className="font-mono text-[10px] text-zinc-500 mr-1">{c.tool}</code>
                    {c.text}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>

        {/* Dependencies */}
        <section>
          <SectionTitle icon={ListTree}>Steps and dependencies</SectionTitle>
          <TaskDagList tasks={snap.tasks} compact />
        </section>

        {/* Permissions */}
        <section>
          <SectionTitle icon={Users}>Permissions</SectionTitle>
          <ul className="space-y-1 text-[11px] text-zinc-300">
            <li>
              Required: <span className={meta.text}>{meta.approval}</span>
              {meta.expiryMinutes > 0 && <> · the approval expires after {meta.expiryMinutes} min</>}.
            </li>
            {approvalTasks.length > 0 && (
              <li className="text-zinc-400">
                {approvalTasks.length === 1 ? '1 step changes' : `${approvalTasks.length} steps change`} infrastructure; the policy
                and quota are checked again when it runs.
              </li>
            )}
            {adminTools.length > 0 && (
              <li className={isAdmin ? 'text-zinc-400' : 'text-amber-300'} data-testid="admin-note">
                Admin only: {adminTools.join(', ')}.{' '}
                {isAdmin ? 'You are an admin.' : 'You are not an admin, so the server will refuse this (ADMIN_REQUIRED).'}
              </li>
            )}
          </ul>
        </section>

        {/* Recovery */}
        <section>
          <SectionTitle icon={LifeBuoy}>If something fails</SectionTitle>
          <p className="text-[11px] text-zinc-400 leading-relaxed">{RECOVERY_NOTES[risk]}</p>
          {mockHypervisor && (
            <p className="mt-1 text-[11px] text-amber-300/90">
              This deployment uses a mock hypervisor: verification will report results as unverified.
            </p>
          )}
        </section>

        {/* Validation */}
        {snap.validation && (snap.validation.errors.length > 0 || snap.validation.warnings.length > 0) && (
          <section>
            <SectionTitle icon={ShieldCheck}>Validation</SectionTitle>
            <ul className="space-y-1">
              {snap.validation.errors.map((e, i) => (
                <li key={`e${i}`} className="text-[11px] text-rose-300 break-words">
                  {e.task_key && <code className="font-mono text-[10px] mr-1">{e.task_key}</code>}
                  {e.message}
                </li>
              ))}
              {snap.validation.warnings.map((w, i) => (
                <li key={`w${i}`} className="text-[11px] text-amber-300 break-words">
                  {w.task_key && <code className="font-mono text-[10px] mr-1">{w.task_key}</code>}
                  {w.message}
                </li>
              ))}
            </ul>
            {validationFailed && (
              <p className="mt-1 text-[11px] text-rose-300">The policy check failed. Edit the plan before approving.</p>
            )}
          </section>
        )}

        {/* Controls */}
        <div className="space-y-3 border-t border-zinc-800 pt-3">
          <fieldset className="space-y-1.5">
            <legend className="text-[10px] font-mono uppercase tracking-wider text-zinc-500 mb-1">After approval</legend>
            <label className="flex items-start gap-2 text-[11.5px] text-zinc-200 cursor-pointer">
              <input
                type="radio"
                name="approval-execute"
                checked={executeNow}
                onChange={() => setExecuteNow(true)}
                className="mt-0.5 accent-white"
                data-testid="approve-run-now"
              />
              <span>
                Approve and run now <span className="text-zinc-500">(recommended)</span>
              </span>
            </label>
            <label className="flex items-start gap-2 text-[11.5px] text-zinc-200 cursor-pointer">
              <input
                type="radio"
                name="approval-execute"
                checked={!executeNow}
                onChange={() => setExecuteNow(false)}
                className="mt-0.5 accent-white"
                data-testid="approve-only"
              />
              <span>
                Approve only <span className="text-zinc-500">(run it later from the plan panel)</span>
              </span>
            </label>
          </fieldset>

          {needsAck && (
            <label
              className={`flex items-start gap-2 p-2 rounded-md border text-[11.5px] cursor-pointer ${
                ackError ? 'border-rose-500/50 bg-rose-500/5 text-rose-200' : 'border-zinc-800 bg-zinc-900/60 text-zinc-200'
              }`}
            >
              <input
                type="checkbox"
                checked={ack}
                onChange={(e) => setAck(e.target.checked)}
                className="mt-0.5 accent-rose-500"
                data-testid="approve-ack"
              />
              <span>I understand the risks{destructive ? ' and that this cannot be undone' : ''}.</span>
            </label>
          )}

          {destructive && (
            <div className="space-y-1">
              <label htmlFor="approve-confirmation" className="block text-[11px] text-zinc-300">
                {expected ? (
                  <>
                    Type <code className="font-mono text-rose-300 break-all">{expected}</code> to confirm
                  </>
                ) : (
                  <>Type the name of each resource that will be deleted, separated by ", "</>
                )}
              </label>
              <input
                id="approve-confirmation"
                value={typed}
                onChange={(e) => setTyped(e.target.value)}
                autoComplete="off"
                autoCapitalize="off"
                autoCorrect="off"
                spellCheck={false}
                data-testid="approve-confirmation"
                className={`w-full bg-zinc-900 border rounded-md px-2.5 py-1.5 font-mono text-xs text-zinc-100 focus:outline-none ${
                  confirmError || (typed && !typedOk) ? 'border-rose-500/60 focus:border-rose-400' : 'border-zinc-800 focus:border-zinc-600'
                }`}
              />
              {typed && !typedOk && expected && (
                <p className="text-[10.5px] text-rose-400">Does not match. Names are case-sensitive.</p>
              )}
            </div>
          )}

          {error && <ErrorNote error={error} onDismiss={() => setError(null)} />}
        </div>

        <div className="sticky bottom-0 -mx-1 px-1 py-2 bg-zinc-950 border-t border-zinc-800 flex flex-wrap items-center justify-end gap-2">
          {blockedReason && (
            <span className="mr-auto text-[10.5px] text-zinc-400 min-w-0" data-testid="approve-blocked-reason">
              {blockedReason}
            </span>
          )}
          <button
            type="button"
            onClick={onClose}
            disabled={submitting}
            className="px-3 py-1.5 rounded-md text-xs font-medium text-zinc-300 bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            type="button"
            onClick={submit}
            disabled={!canSubmit}
            data-testid="approve-submit"
            className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold transition-colors disabled:opacity-50 ${
              destructive ? 'bg-rose-600 hover:bg-rose-500 text-white' : 'bg-white hover:bg-zinc-200 text-zinc-950'
            }`}
          >
            {submitting ? <Spinner /> : <ShieldCheck className="w-3.5 h-3.5" />}
            <span>{approveLabel}</span>
          </button>
        </div>
      </div>
    </Modal>
  );
};
