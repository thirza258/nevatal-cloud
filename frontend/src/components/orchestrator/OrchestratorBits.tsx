import React from 'react';
import {
  Ban,
  CircleAlert,
  CircleCheck,
  CircleDashed,
  CircleMinus,
  CircleX,
  Clock,
  Info,
  LoaderCircle,
  Lock,
  RotateCw,
  ShieldCheck,
  ShieldQuestion,
  ShieldX,
  TriangleAlert,
  Undo2,
  X,
} from 'lucide-react';
import { Modal } from '../ui/Modal';
import {
  DescribedError,
  RISK_META,
  TASK_STATUS_LABEL,
  TONE_BADGE,
  TONE_DOT,
  Tone,
  executionStatusTone,
  planStatusLabel,
  planStatusTone,
  redactSecrets,
} from '../../lib/orchestrator';
import type { RiskLevel } from '../../types';

const BADGE_BASE = 'inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border font-mono text-[11px] whitespace-nowrap';

export const ToneBadge: React.FC<{ tone: Tone; label: string; pulse?: boolean; title?: string; className?: string }> = ({
  tone,
  label,
  pulse = false,
  title,
  className = '',
}) => (
  <span className={`${BADGE_BASE} ${TONE_BADGE[tone]} ${className}`} title={title}>
    <span className={`inline-block w-1.5 h-1.5 rounded-full ${TONE_DOT[tone]} ${pulse ? 'animate-pulse' : ''}`} />
    <span>{label}</span>
  </span>
);

export const RiskBadge: React.FC<{ risk: RiskLevel; withLabel?: boolean; compact?: boolean; className?: string }> = ({
  risk,
  withLabel = true,
  compact = false,
  className = '',
}) => {
  const meta = RISK_META[risk];
  const base = compact
    ? 'inline-flex items-center gap-1 px-1.5 py-px rounded border font-mono text-[10px] whitespace-nowrap'
    : BADGE_BASE;
  return (
    <span className={`${base} ${meta.badge} ${className}`} title={`${meta.label} risk: ${meta.approval}`} data-risk={risk}>
      <span className={`inline-block w-1.5 h-1.5 rounded-full ${meta.dot}`} />
      <span>{withLabel ? `${meta.label.toLowerCase()} risk` : meta.label.toLowerCase()}</span>
    </span>
  );
};

export const PlanStatusBadge: React.FC<{ status: string }> = ({ status }) => {
  const tone = planStatusTone(status);
  return <ToneBadge tone={tone} label={planStatusLabel(status)} pulse={status === 'executing' || status === 'verifying'} />;
};

export const ExecutionStatusBadge: React.FC<{ status: string }> = ({ status }) => (
  <ToneBadge
    tone={executionStatusTone(status)}
    label={(status || 'unknown').replace(/_/g, ' ')}
    pulse={status === 'running' || status === 'verifying' || status === 'cancelling'}
  />
);

/** Icon for each task status (contract §1). */
export const TaskStatusIcon: React.FC<{ status: string; className?: string }> = ({ status, className = 'w-4 h-4' }) => {
  const label = TASK_STATUS_LABEL[status] ?? status;
  const common = `${className} flex-shrink-0`;
  switch (status) {
    case 'succeeded':
      return <CircleCheck className={`${common} text-emerald-400`} aria-label={label} />;
    case 'failed':
      return <CircleX className={`${common} text-rose-400`} aria-label={label} />;
    case 'running':
      return <LoaderCircle className={`${common} text-sky-400 animate-spin`} aria-label={label} />;
    case 'retrying':
      return <RotateCw className={`${common} text-amber-400`} aria-label={label} />;
    case 'blocked':
      return <Lock className={`${common} text-zinc-500`} aria-label={label} />;
    case 'cancelled':
      return <Ban className={`${common} text-zinc-500`} aria-label={label} />;
    case 'skipped':
      return <CircleMinus className={`${common} text-zinc-500`} aria-label={label} />;
    case 'compensating':
      return <Undo2 className={`${common} text-amber-400 animate-pulse`} aria-label={label} />;
    case 'queued':
      return <CircleDashed className={`${common} text-zinc-500`} aria-label={label} />;
    default:
      return <Clock className={`${common} text-zinc-500`} aria-label={label} />;
  }
};

/** Verification outcome. `unverified` is amber and never uses a check mark: it is not a success. */
export const VerificationBadge: React.FC<{ status: string; className?: string }> = ({ status, className = '' }) => {
  if (status === 'verified') {
    return (
      <span className={`${BADGE_BASE} ${TONE_BADGE.emerald} ${className}`} data-verification="verified">
        <ShieldCheck className="w-3 h-3" />
        <span>verified</span>
      </span>
    );
  }
  if (status === 'failed') {
    return (
      <span className={`${BADGE_BASE} ${TONE_BADGE.rose} ${className}`} data-verification="failed">
        <ShieldX className="w-3 h-3" />
        <span>verification failed</span>
      </span>
    );
  }
  return (
    <span
      className={`${BADGE_BASE} ${TONE_BADGE.amber} ${className}`}
      data-verification="unverified"
      title="The result could not be confirmed (for example on a mock hypervisor or when the check could not reach the resource)."
    >
      <ShieldQuestion className="w-3 h-3" />
      <span>unverified</span>
    </span>
  );
};

export const SectionTitle: React.FC<{
  icon?: React.ComponentType<{ className?: string }>;
  children: React.ReactNode;
  right?: React.ReactNode;
}> = ({ icon: Icon, children, right }) => (
  <div className="flex items-center justify-between gap-2 mb-1.5">
    <div className="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-500 min-w-0">
      {Icon && <Icon className="w-3 h-3 flex-shrink-0" />}
      <span className="truncate">{children}</span>
    </div>
    {right}
  </div>
);

const NOTICE_STYLES: Record<'info' | 'success' | 'warning' | 'error', { box: string; icon: React.ComponentType<{ className?: string }>; iconClass: string }> = {
  info: { box: 'bg-sky-500/5 border-sky-500/20 text-sky-200', icon: Info, iconClass: 'text-sky-400' },
  success: { box: 'bg-emerald-500/5 border-emerald-500/20 text-emerald-200', icon: CircleCheck, iconClass: 'text-emerald-400' },
  warning: { box: 'bg-amber-500/5 border-amber-500/20 text-amber-200', icon: TriangleAlert, iconClass: 'text-amber-400' },
  error: { box: 'bg-rose-500/5 border-rose-500/20 text-rose-200', icon: CircleAlert, iconClass: 'text-rose-400' },
};

export type NoticeTone = keyof typeof NOTICE_STYLES;

export const Notice: React.FC<{
  tone: NoticeTone;
  children: React.ReactNode;
  onDismiss?: () => void;
  className?: string;
  role?: 'status' | 'alert';
}> = ({ tone, children, onDismiss, className = '', role }) => {
  const style = NOTICE_STYLES[tone];
  const Icon = style.icon;
  return (
    <div
      role={role ?? (tone === 'error' ? 'alert' : 'status')}
      className={`flex items-start gap-2 p-2.5 rounded-md border text-[11px] leading-relaxed ${style.box} ${className}`}
    >
      <Icon className={`w-3.5 h-3.5 mt-px flex-shrink-0 ${style.iconClass}`} />
      <div className="flex-1 min-w-0 break-words">{children}</div>
      {onDismiss && (
        <button
          type="button"
          onClick={onDismiss}
          title="Dismiss"
          className="p-0.5 -m-0.5 rounded text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
        >
          <X className="w-3 h-3" />
        </button>
      )}
    </div>
  );
};

/** An API error with its code, the contract hint and any per-task details. */
export const ErrorNote: React.FC<{ error: DescribedError; onDismiss?: () => void; className?: string }> = ({
  error,
  onDismiss,
  className = '',
}) => (
  <Notice tone="error" onDismiss={onDismiss} className={className}>
    <div className="space-y-1">
      <div>
        <span className="text-rose-100">{error.message}</span>
        {error.code && error.code !== 'API_ERROR' && error.code !== 'NETWORK_ERROR' && (
          <span className="ml-1.5 font-mono text-[10px] text-rose-300/80" data-error-code={error.code}>
            {error.code}
          </span>
        )}
      </div>
      {error.hint && <div className="text-rose-200/80">{error.hint}</div>}
      {error.details.length > 0 && (
        <ul className="list-disc pl-4 space-y-0.5 text-rose-200/90">
          {error.details.map((d) => (
            <li key={d} className="break-words">
              {d}
            </li>
          ))}
        </ul>
      )}
    </div>
  </Notice>
);

/** Pretty JSON (secrets masked) in a scrollable block that never widens the page. */
export const JsonBlock: React.FC<{ value: unknown; maxHeight?: string }> = ({ value, maxHeight = 'max-h-56' }) => (
  <pre
    className={`${maxHeight} overflow-auto rounded-md bg-zinc-950 border border-zinc-800 p-2 font-mono text-[10.5px] leading-relaxed text-zinc-300 whitespace-pre-wrap break-all`}
  >
    {JSON.stringify(redactSecrets(value), null, 2)}
  </pre>
);

export const Spinner: React.FC<{ className?: string }> = ({ className = 'w-3.5 h-3.5' }) => (
  <LoaderCircle className={`${className} animate-spin`} />
);

/** Confirmation dialog built on Modal (replaces window.confirm). */
export const ConfirmDialog: React.FC<{
  open: boolean;
  title: string;
  description?: string;
  confirmLabel: string;
  tone?: 'danger' | 'neutral';
  busy?: boolean;
  error?: DescribedError | null;
  confirmDisabled?: boolean;
  onConfirm: () => void;
  onClose: () => void;
  children?: React.ReactNode;
}> = ({ open, title, description, confirmLabel, tone = 'neutral', busy = false, error, confirmDisabled, onConfirm, onClose, children }) => (
  <Modal isOpen={open} onClose={busy ? () => {} : onClose} title={title} description={description} maxWidth="md">
    <div className="space-y-3">
      {children}
      {error && <ErrorNote error={error} />}
      <div className="flex flex-wrap items-center justify-end gap-2 pt-1">
        <button
          type="button"
          onClick={onClose}
          disabled={busy}
          className="px-3 py-1.5 rounded-md text-xs font-medium text-zinc-300 bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 transition-colors disabled:opacity-50"
        >
          Keep
        </button>
        <button
          type="button"
          onClick={onConfirm}
          disabled={busy || confirmDisabled}
          data-testid="confirm-dialog-confirm"
          className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-xs font-semibold transition-colors disabled:opacity-50 ${
            tone === 'danger'
              ? 'bg-rose-600 hover:bg-rose-500 text-white'
              : 'bg-white hover:bg-zinc-200 text-zinc-950'
          }`}
        >
          {busy && <Spinner />}
          <span>{confirmLabel}</span>
        </button>
      </div>
    </div>
  </Modal>
);
