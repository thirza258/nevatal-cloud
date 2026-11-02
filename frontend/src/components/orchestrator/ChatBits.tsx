import React from 'react';
import { CircleAlert, CircleCheck, ClipboardList, Cpu, Eye, ShieldAlert, Sliders, X } from 'lucide-react';
import type { StructuredPlanView, StructuredQuestion, UIComponentsPayload, UIProgressStep, WorkflowState } from '../../types';
import { isAwaitingApproval } from '../../lib/orchestrator';
import { CommandBlock } from '../docker/DockerBits';
import { PlanStatusBadge } from './OrchestratorBits';

export function parseUIComponents(raw?: string): UIComponentsPayload | null {
  if (!raw) return null;
  try {
    const parsed: unknown = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? (parsed as UIComponentsPayload) : null;
  } catch {
    return null;
  }
}

const STATE_BADGES: Partial<Record<WorkflowState, { label: string; cls: string }>> = {
  UnderstandingRequest: { label: 'Understanding', cls: 'bg-sky-500/10 text-sky-400 border-sky-500/20' },
  ClarifyingRequirements: { label: 'Clarifying', cls: 'bg-amber-500/10 text-amber-400 border-amber-500/20' },
  Planning: { label: 'Planning', cls: 'bg-zinc-800 text-zinc-300 border-zinc-700' },
  AwaitingConfirmation: { label: 'Awaiting approval', cls: 'bg-amber-500/10 text-amber-300 border-amber-500/30' },
  Provisioning: { label: 'Executing', cls: 'bg-sky-500/10 text-sky-300 border-sky-500/30' },
  Validating: { label: 'Verifying', cls: 'bg-sky-500/10 text-sky-300 border-sky-500/30' },
  Recovery: { label: 'Recovery', cls: 'bg-amber-500/10 text-amber-300 border-amber-500/30' },
  Completed: { label: 'Completed', cls: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20' },
  Cancelled: { label: 'Cancelled', cls: 'bg-zinc-900 text-zinc-400 border-zinc-800' },
  Failed: { label: 'Failed', cls: 'bg-rose-500/10 text-rose-400 border-rose-500/20' },
};

export const WorkflowStateBadge: React.FC<{ state?: WorkflowState; compact?: boolean }> = ({ state, compact = false }) => {
  const b = state ? STATE_BADGES[state] : undefined;
  if (!b) return null;
  if (compact) {
    // Sidebar: a dot keeps the conversation title readable; the label stays available as a tooltip.
    return (
      <span title={b.label} aria-label={b.label} className={`inline-block w-2 h-2 rounded-full border flex-shrink-0 ${b.cls}`} />
    );
  }
  return <span className={`px-1.5 py-0.5 rounded-md text-[10px] font-mono border whitespace-nowrap ${b.cls}`}>{b.label}</span>;
};

export const QuestionsCard: React.FC<{
  questions: StructuredQuestion[];
  disabled: boolean;
  onAnswer: (key: string, value: string) => void;
}> = ({ questions, disabled, onAnswer }) => (
  <div className="space-y-2.5 p-3 rounded-lg bg-zinc-950 border border-zinc-800">
    <div className="text-xs font-medium text-white tracking-tight flex items-center gap-1.5">
      <Sliders className="w-3.5 h-3.5 text-zinc-400" />
      <span>Select configuration options</span>
    </div>
    {questions.map((q) => (
      <div key={q.id} className="space-y-1.5">
        <div className="text-xs font-semibold text-zinc-200">{q.title}</div>
        {q.description && <div className="text-[11px] text-zinc-400">{q.description}</div>}
        <div className="flex flex-wrap gap-1.5 pt-0.5">
          {q.options?.map((opt) => (
            <button
              key={opt.id}
              type="button"
              disabled={disabled}
              onClick={() => onAnswer(q.key, opt.value)}
              className={`px-2.5 py-1 rounded-md text-xs font-medium transition-colors border flex items-center gap-1.5 disabled:opacity-50 ${
                opt.is_default
                  ? 'bg-white/10 text-white border-zinc-600 hover:bg-white/15'
                  : 'bg-zinc-900 text-zinc-300 border-zinc-800 hover:bg-zinc-800'
              }`}
            >
              <span>{opt.label}</span>
              {opt.is_default && <span className="text-[9px] px-1 rounded bg-zinc-700 text-zinc-200 font-mono">default</span>}
            </button>
          ))}
        </div>
      </div>
    ))}
  </div>
);

/** Chat card for a plan proposal. Approval always goes through the plan panel + approval modal. */
export const PlanProposalCard: React.FC<{
  proposal: StructuredPlanView;
  /** Live status from GET /ai/conversations/:id/plans (the card's own status is a snapshot). */
  liveStatus?: string;
  liveVersion?: number;
  selected: boolean;
  onReview: () => void;
  onApprove: () => void;
  onModify: () => void;
  onCancel: () => void;
}> = ({ proposal, liveStatus, liveVersion, selected, onReview, onApprove, onModify, onCancel }) => {
  const status = liveStatus ?? proposal.status;
  const awaiting = isAwaitingApproval(status) || status === 'draft';
  const allocations = Object.entries(proposal.estimated_allocations || {});
  return (
    <div
      className={`p-3 rounded-lg bg-zinc-950 border space-y-3 ${selected ? 'border-sky-500/40' : 'border-zinc-800'}`}
      data-testid="plan-card"
      data-plan-id={proposal.plan_id}
    >
      <div className="flex flex-wrap items-start justify-between gap-2 border-b border-zinc-800 pb-2.5">
        <div className="flex items-start gap-2 min-w-0">
          <div className="w-6 h-6 rounded bg-zinc-900 flex items-center justify-center text-zinc-300 border border-zinc-800 flex-shrink-0">
            <Cpu className="w-3.5 h-3.5" />
          </div>
          <div className="min-w-0">
            <h4 className="text-xs font-semibold text-white tracking-tight break-words">{proposal.title}</h4>
            <span className="text-[11px] text-zinc-400 font-mono break-words">
              v{liveVersion ?? proposal.version} · {proposal.summary}
            </span>
          </div>
        </div>
        <PlanStatusBadge status={status} />
      </div>

      {allocations.length > 0 && (
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-1.5">
          {allocations.map(([k, v]) => (
            <div key={k} className="px-2 py-1.5 rounded-md bg-zinc-900/60 border border-zinc-800 min-w-0">
              <div className="text-[10px] uppercase font-mono text-zinc-500 truncate">{k.replace(/_/g, ' ')}</div>
              <div className="text-xs font-semibold text-zinc-200 truncate">{String(v)}</div>
            </div>
          ))}
        </div>
      )}

      {proposal.resources?.length > 0 && (
        <ul className="space-y-1">
          {proposal.resources.map((res, idx) => (
            <li key={idx} className="flex flex-wrap items-center gap-x-2 gap-y-0.5 text-[11px] min-w-0">
              <span className="font-semibold text-zinc-100 break-all">{res.name}</span>
              <span className="text-[10px] text-zinc-500 font-mono uppercase">{res.type}</span>
              <span className="text-zinc-400 break-words min-w-0">{res.summary}</span>
            </li>
          ))}
        </ul>
      )}

      <div className="flex flex-wrap items-center gap-1.5 pt-2 border-t border-zinc-800">
        {awaiting && (
          <button
            type="button"
            onClick={onApprove}
            className="inline-flex items-center gap-1.5 py-1.5 px-3 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 font-semibold text-xs transition-colors"
            data-testid="plan-card-approve"
          >
            <ShieldAlert className="w-3.5 h-3.5" />
            Review &amp; approve…
          </button>
        )}
        <button
          type="button"
          onClick={onReview}
          className="inline-flex items-center gap-1.5 py-1.5 px-2.5 rounded-md bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 text-zinc-300 font-medium text-xs transition-colors"
          data-testid="plan-card-review"
        >
          <Eye className="w-3.5 h-3.5" />
          {awaiting ? 'Details' : 'Open plan'}
        </button>
        {awaiting && (
          <>
            <button
              type="button"
              onClick={onModify}
              className="py-1.5 px-2.5 rounded-md bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 text-zinc-300 font-medium text-xs transition-colors"
            >
              Ask for changes
            </button>
            <button
              type="button"
              onClick={onCancel}
              className="inline-flex items-center gap-1 py-1.5 px-2.5 rounded-md bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/20 font-medium text-xs transition-colors"
            >
              <X className="w-3 h-3" />
              Cancel
            </button>
          </>
        )}
      </div>
    </div>
  );
};

/** Progress steps attached to messages by the pre-orchestrator flow (old conversations). */
export const LegacyProgress: React.FC<{ steps: UIProgressStep[] }> = ({ steps }) => (
  <div className="p-3 rounded-lg bg-zinc-950 border border-zinc-800 space-y-2">
    <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500">Provisioning steps (earlier run)</div>
    <div className="space-y-1.5">
      {steps.map((step) => {
        const done = step.status === 'completed';
        const prog = step.status === 'in_progress';
        const fail = step.status === 'failed';
        return (
          <div key={step.step_id} className="flex items-center gap-2 text-xs min-w-0">
            {done && <CircleCheck className="w-3.5 h-3.5 text-emerald-400 flex-shrink-0" />}
            {prog && <div className="w-3 h-3 rounded-full border-2 border-sky-400 border-t-transparent animate-spin flex-shrink-0" />}
            {fail && <CircleAlert className="w-3.5 h-3.5 text-rose-400 flex-shrink-0" />}
            {!done && !prog && !fail && <div className="w-3 h-3 rounded-full border border-zinc-700 flex-shrink-0" />}
            <span className={`min-w-0 break-words ${done ? 'text-zinc-200' : prog ? 'text-sky-300' : 'text-zinc-500'}`}>{step.title}</span>
          </div>
        );
      })}
    </div>
  </div>
);

/** Result data attached by the pre-orchestrator flow. It was not verified, so it is shown neutrally. */
export const LegacyResult: React.FC<{ data: Record<string, unknown>; onOpenResources: () => void }> = ({ data, onOpenResources }) => (
  <div className="p-3 rounded-lg bg-zinc-950 border border-zinc-800 space-y-2">
    <div className="flex items-center gap-1.5 text-xs font-semibold text-zinc-200">
      <ClipboardList className="w-3.5 h-3.5 text-zinc-400" />
      Provisioning result
    </div>
    {typeof data.ssh_command === 'string' && data.ssh_command && <CommandBlock cmd={data.ssh_command} label="SSH" />}
    <button
      type="button"
      onClick={onOpenResources}
      className="px-2.5 py-1 rounded-md bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 text-zinc-200 text-xs font-medium transition-colors"
    >
      Open resources
    </button>
  </div>
);
