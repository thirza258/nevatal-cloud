import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  ArrowUpRight,
  ChevronDown,
  ChevronRight,
  CircleStop,
  ClipboardList,
  LifeBuoy,
  RotateCw,
  ScrollText,
  Server,
  ShieldQuestion,
  TriangleAlert,
  Workflow,
} from 'lucide-react';
import type { Execution, ExecutionEvent, ExecutionSummary, ExecutionTask } from '../../types';
import {
  DescribedError,
  EVENTS_PAGE_LIMIT,
  StreamUnsupportedError,
  TASK_STATUS_LABEL,
  cancelExecution,
  describeError,
  durationOf,
  fetchExecution,
  fetchExecutionEvents,
  formatClock,
  formatRelative,
  isAbortError,
  isActiveExecution,
  recoverExecution,
  retryExecutionTask,
  shortId,
  streamExecutionEvents,
} from '../../lib/orchestrator';
import { CommandBlock, CopyButton } from '../docker/DockerBits';
import {
  ConfirmDialog,
  ErrorNote,
  ExecutionStatusBadge,
  JsonBlock,
  Notice,
  RiskBadge,
  Spinner,
  TaskStatusIcon,
  ToneBadge,
  VerificationBadge,
} from './OrchestratorBits';

// ---------------------------------------------------------------------------------------------
// Live updates: fetch-streamed SSE (Bearer header), resume with Last-Event-ID, poll as fallback
// ---------------------------------------------------------------------------------------------

export type LiveMode = 'idle' | 'connecting' | 'live' | 'reconnecting' | 'polling' | 'finished';

const POLL_INTERVAL_MS = 2000;
const MAX_STREAM_FAILURES = 2;
/** Once the execution is terminal, give the stream this long to deliver `execution.finished`. */
const TERMINAL_GRACE_MS = 10_000;

interface LiveOptions {
  executionId: string;
  /** Latched by the caller: stays true until `onFinished`, so a status refresh never cuts a replay short. */
  enabled: boolean;
  /** Bump to restart after a retry/recovery re-activated a finished execution. */
  restartKey: number;
  /** The last fetched execution object is terminal. */
  terminal: boolean;
  getAfter: () => number;
  onEvent: (event: ExecutionEvent) => void;
  advance: (nextAfter: number) => void;
  isTerminal: () => boolean;
  onFinished: () => void;
}

/**
 * Live events for one execution: SSE over fetch (Bearer header), resume with Last-Event-ID after a
 * close or stall, JSON polling every 2 s when streaming is unavailable or keeps failing. Ends on
 * `execution.finished`, or when the execution is terminal and the JSON form has caught up.
 */
function useExecutionLive(opts: LiveOptions): LiveMode {
  const [mode, setMode] = useState<LiveMode>('idle');
  const ref = useRef(opts);
  ref.current = opts;
  const connRef = useRef<AbortController | null>(null);
  const { executionId, enabled, restartKey, terminal } = opts;

  useEffect(() => {
    if (!enabled) {
      setMode((m) => (m === 'finished' ? m : 'idle'));
      return;
    }
    const controller = new AbortController();
    const { signal } = controller;
    const sleep = (ms: number) =>
      new Promise<void>((resolve) => {
        const timer = setTimeout(resolve, ms);
        signal.addEventListener(
          'abort',
          () => {
            clearTimeout(timer);
            resolve();
          },
          { once: true }
        );
      });

    const run = async () => {
      let finished = false;
      let polling = false;
      let failures = 0;
      const handle = (ev: ExecutionEvent) => {
        ref.current.onEvent(ev);
        if (ev.type === 'execution.finished') finished = true;
      };

      while (!signal.aborted && !finished) {
        if (!polling) {
          setMode(failures === 0 ? 'connecting' : 'reconnecting');
          let received = 0;
          const startedAt = Date.now();
          const conn = new AbortController();
          const abortConn = () => conn.abort();
          signal.addEventListener('abort', abortConn, { once: true });
          connRef.current = conn;
          try {
            const result = await streamExecutionEvents(executionId, {
              after: ref.current.getAfter(),
              signal: conn.signal,
              onOpen: () => setMode('live'),
              onEvent: (ev) => {
                received++;
                handle(ev);
              },
            });
            if (result === 'finished' || finished) break;
            // Closed early (restart/shutdown) or stalled: resume with Last-Event-ID. A stream that
            // delivered nothing (closed at once, or stalled) counts as a failure, so a server that
            // keeps refusing ends in polling.
            const quiet = received === 0 && (result === 'stalled' || Date.now() - startedAt <= 5000);
            failures = quiet ? failures + 1 : 0;
          } catch (err) {
            if (signal.aborted) return;
            if (err instanceof StreamUnsupportedError || conn.signal.aborted) {
              // Unsupported, or the run ended but the stream stayed open: finish through the JSON form.
              polling = true;
              continue;
            }
            // A connection that broke after delivering events is a reconnect, not a failure.
            failures = received > 0 ? 0 : failures + 1;
          } finally {
            signal.removeEventListener('abort', abortConn);
            if (connRef.current === conn) connRef.current = null;
          }
          if (ref.current.isTerminal() || failures >= MAX_STREAM_FAILURES) {
            polling = true; // catch up (or keep going) through the JSON form
            continue;
          }
          setMode('reconnecting');
          await sleep(1000 * Math.max(1, failures));
        } else {
          setMode('polling');
          let caughtUp = false;
          try {
            for (;;) {
              const page = await fetchExecutionEvents(executionId, ref.current.getAfter(), signal);
              page.events.forEach(handle);
              ref.current.advance(page.nextAfter);
              if (finished || page.events.length < EVENTS_PAGE_LIMIT) break;
            }
            caughtUp = true;
          } catch (err) {
            if (signal.aborted || isAbortError(err)) return;
          }
          if (finished || (caughtUp && ref.current.isTerminal())) break;
          await sleep(POLL_INTERVAL_MS);
        }
      }
      if (!signal.aborted) {
        setMode('finished');
        ref.current.onFinished();
      }
    };

    void run();
    return () => controller.abort();
  }, [executionId, enabled, restartKey]);

  // The execution ended but the stream is still open (no `execution.finished`): stop waiting for it.
  useEffect(() => {
    if (!enabled || !terminal) return;
    const timer = setTimeout(() => connRef.current?.abort(), TERMINAL_GRACE_MS);
    return () => clearTimeout(timer);
  }, [enabled, terminal]);

  return mode;
}

const LIVE_LABEL: Record<LiveMode, { label: string; dot: string } | null> = {
  idle: null,
  finished: null,
  connecting: { label: 'Connecting…', dot: 'bg-zinc-400 animate-pulse' },
  live: { label: 'Live', dot: 'bg-emerald-500 animate-pulse' },
  reconnecting: { label: 'Reconnecting…', dot: 'bg-amber-400 animate-pulse' },
  polling: { label: 'Polling every 2 s', dot: 'bg-amber-400' },
};

// ---------------------------------------------------------------------------------------------
// Pieces
// ---------------------------------------------------------------------------------------------

const REFRESH_EVENT_RE = /^(task|execution|verification|approval)\./;
const MAX_EVENTS = 2000;

function overallVerification(tasks: ExecutionTask[]): 'verified' | 'unverified' | 'failed' | null {
  const statuses = tasks.map((t) => t.verification?.status).filter((s): s is string => !!s);
  if (statuses.length === 0) return null;
  if (statuses.includes('failed')) return 'failed';
  if (statuses.every((s) => s === 'verified')) return 'verified';
  return 'unverified';
}

const SEVERITY_CLASS: Record<string, string> = {
  info: 'text-zinc-400',
  warning: 'text-amber-300',
  error: 'text-rose-300',
};

const EventList: React.FC<{ events: ExecutionEvent[]; showTask?: boolean; empty: string }> = ({ events, showTask, empty }) =>
  events.length === 0 ? (
    <div className="text-[11px] text-zinc-500 italic">{empty}</div>
  ) : (
    <ol className="space-y-0.5 max-h-56 overflow-y-auto pr-1" data-testid="event-list">
      {events.map((e) => (
        <li key={e.seq} className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-2 text-[10.5px] leading-relaxed" data-seq={e.seq}>
          <span className="font-mono text-zinc-600">{formatClock(e.created_at)}</span>
          <span className={`min-w-0 break-words ${SEVERITY_CLASS[e.severity] ?? 'text-zinc-400'}`}>
            <span className="font-mono text-[10px] text-zinc-500 mr-1">{e.type}</span>
            {showTask && e.task_key && <span className="font-mono text-[10px] text-zinc-500 mr-1">[{e.task_key}]</span>}
            {e.message}
          </span>
        </li>
      ))}
    </ol>
  );

const TaskRow: React.FC<{
  task: ExecutionTask;
  events: ExecutionEvent[];
  expanded: boolean;
  onToggle: () => void;
  onRetry: (() => void) | null;
  retrying: boolean;
  now: number;
  eventsLoading: boolean;
}> = ({ task, events, expanded, onToggle, onRetry, retrying, now, eventsLoading }) => {
  const duration = durationOf(task.started_at, task.finished_at, now);
  const attempts = task.max_attempts > 0 ? `attempt ${task.attempt}/${task.max_attempts}` : task.attempt > 0 ? `attempt ${task.attempt}` : null;
  const taskEvents = events.filter((e) => e.task_key === task.key);
  return (
    <li
      className={`rounded-md border transition-colors ${
        task.status === 'failed'
          ? 'border-rose-500/25 bg-rose-500/[0.03]'
          : task.status === 'running'
            ? 'border-sky-500/25 bg-sky-500/[0.03]'
            : 'border-zinc-800 bg-zinc-950/60'
      }`}
      data-task-key={task.key}
      data-task-status={task.status}
    >
      <div className="flex items-start gap-2 px-2 py-1.5">
        <button
          type="button"
          onClick={onToggle}
          aria-expanded={expanded}
          className="flex-1 min-w-0 flex items-start gap-2 text-left"
          title={TASK_STATUS_LABEL[task.status] ?? task.status}
        >
          <TaskStatusIcon status={task.status} className="w-4 h-4 mt-px" />
          <div className="flex-1 min-w-0">
            <div className="flex flex-wrap items-center gap-x-1.5 gap-y-0.5">
              <span className="text-[11.5px] text-zinc-100 break-words min-w-0">{task.title}</span>
              {task.verification && <VerificationBadge status={task.verification.status} />}
            </div>
            <div className="flex flex-wrap items-center gap-x-1.5 text-[10px] font-mono text-zinc-500">
              <span>{(TASK_STATUS_LABEL[task.status] ?? task.status).toLowerCase()}</span>
              {attempts && <span data-testid="task-attempt">· {attempts}</span>}
              {duration && <span>· {duration}</span>}
              {task.status === 'retrying' && task.next_attempt_at && <span>· next {formatRelative(task.next_attempt_at, now)}</span>}
            </div>
            {task.error && !expanded && (
              <div className="text-[10.5px] text-rose-300 break-words line-clamp-2">{task.error.message}</div>
            )}
          </div>
          {expanded ? (
            <ChevronDown className="w-3.5 h-3.5 mt-0.5 text-zinc-500 flex-shrink-0" />
          ) : (
            <ChevronRight className="w-3.5 h-3.5 mt-0.5 text-zinc-500 flex-shrink-0" />
          )}
        </button>
        {onRetry && (
          <button
            type="button"
            onClick={onRetry}
            disabled={retrying}
            className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md border border-zinc-700 bg-zinc-900 text-[10.5px] font-medium text-zinc-200 hover:bg-zinc-800 disabled:opacity-50 flex-shrink-0"
            data-testid="task-retry"
          >
            {retrying ? <Spinner className="w-3 h-3" /> : <RotateCw className="w-3 h-3" />}
            Retry
          </button>
        )}
      </div>
      {expanded && (
        <div className="border-t border-zinc-800/80 px-2 py-2 space-y-2" data-testid="task-details">
          <dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-0.5 text-[10.5px]">
            <dt className="text-zinc-500">Key</dt>
            <dd className="font-mono text-zinc-300 break-all">{task.key}</dd>
            <dt className="text-zinc-500">Tool</dt>
            <dd className="font-mono text-zinc-300 break-all flex flex-wrap items-center gap-1.5">
              {task.tool || '—'} <RiskBadge risk={task.risk} withLabel={false} />
            </dd>
            {task.depends_on.length > 0 && (
              <>
                <dt className="text-zinc-500">After</dt>
                <dd className="font-mono text-zinc-400 break-all">{task.depends_on.join(', ')}</dd>
              </>
            )}
            <dt className="text-zinc-500">Started</dt>
            <dd className="font-mono text-zinc-300">{formatClock(task.started_at)}</dd>
            <dt className="text-zinc-500">Finished</dt>
            <dd className="font-mono text-zinc-300">{formatClock(task.finished_at)}</dd>
          </dl>
          {task.error && (
            <div className="rounded-md border border-rose-500/20 bg-rose-500/5 p-2 text-[11px]" data-testid="task-error">
              <span className="font-mono text-[10px] text-rose-300/80 mr-1.5">{task.error.code}</span>
              <span className="text-rose-200 break-words">{task.error.message}</span>
            </div>
          )}
          {task.verification && (
            <div className="space-y-1">
              <div className="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-500">
                Verification <VerificationBadge status={task.verification.status} />
              </div>
              {task.verification.checks.length === 0 ? (
                <div className="text-[10.5px] text-zinc-500 italic">No checks reported.</div>
              ) : (
                <ul className="space-y-0.5">
                  {task.verification.checks.map((c, i) => (
                    <li key={`${c.name}-${i}`} className="flex items-start gap-1.5 text-[10.5px]" data-check-status={c.status}>
                      <span
                        className={`font-mono text-[10px] px-1 rounded border flex-shrink-0 ${
                          c.status === 'pass'
                            ? 'border-emerald-500/20 text-emerald-400'
                            : c.status === 'fail'
                              ? 'border-rose-500/20 text-rose-400'
                              : 'border-zinc-700 text-zinc-400'
                        }`}
                      >
                        {c.status}
                      </span>
                      <span className="text-zinc-300 font-mono break-all">{c.name}</span>
                      {c.detail && <span className="text-zinc-500 break-words min-w-0">{c.detail}</span>}
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )}
          {task.output && Object.keys(task.output).length > 0 && (
            <div className="space-y-1">
              <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500">Output</div>
              <JsonBlock value={task.output} maxHeight="max-h-40" />
            </div>
          )}
          <div className="space-y-1">
            <div className="flex items-center gap-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-500">
              Events {eventsLoading && <Spinner className="w-3 h-3" />}
            </div>
            <EventList events={taskEvents} empty={eventsLoading ? 'Loading…' : 'No events for this task yet.'} />
          </div>
        </div>
      )}
    </li>
  );
};

const ReportCard: React.FC<{
  execution: Execution;
  verification: 'verified' | 'unverified' | 'failed' | null;
  onNavigate: (path: string, param?: string) => void;
}> = ({ execution, verification, onNavigate }) => {
  const report = execution.report;
  if (!report) return null;
  const status = execution.status;
  let title = 'Finished';
  let box = 'border-zinc-800 bg-zinc-900/40';
  let titleClass = 'text-zinc-200';
  if (status === 'completed') {
    if (verification === 'verified') {
      title = 'Completed and verified';
      box = 'border-emerald-500/25 bg-emerald-500/5';
      titleClass = 'text-emerald-300';
    } else if (verification === 'failed') {
      title = 'Completed, but verification failed';
      box = 'border-rose-500/25 bg-rose-500/5';
      titleClass = 'text-rose-300';
    } else {
      title = 'Completed, not verified';
      box = 'border-amber-500/25 bg-amber-500/5';
      titleClass = 'text-amber-300';
    }
  } else if (status === 'partially_completed') {
    title = 'Partially completed';
    box = 'border-amber-500/25 bg-amber-500/5';
    titleClass = 'text-amber-300';
  } else if (status === 'failed') {
    title = 'Failed';
    box = 'border-rose-500/25 bg-rose-500/5';
    titleClass = 'text-rose-300';
  } else if (status === 'cancelled') {
    title = 'Cancelled';
  }

  return (
    <div className={`rounded-md border p-2.5 space-y-2.5 ${box}`} data-testid="execution-report">
      <div className="flex flex-wrap items-center gap-2">
        <ClipboardList className={`w-3.5 h-3.5 ${titleClass}`} />
        <span className={`text-xs font-semibold ${titleClass}`}>{title}</span>
        {verification && <VerificationBadge status={verification} />}
      </div>
      {report.summary && <p className="text-[11.5px] text-zinc-300 leading-relaxed break-words">{report.summary}</p>}
      {verification === 'unverified' && (
        <p className="text-[11px] text-amber-200/90 flex items-start gap-1.5">
          <ShieldQuestion className="w-3.5 h-3.5 mt-px flex-shrink-0 text-amber-400" />
          Some checks could not confirm the result. Check the resources yourself before you rely on them.
        </p>
      )}
      {report.resources.length > 0 && (
        <ul className="space-y-1.5">
          {report.resources.map((r) => (
            <li key={`${r.type}:${r.id}`} className="rounded-md border border-zinc-800 bg-zinc-950/70 p-2 space-y-1.5 min-w-0">
              <div className="flex flex-wrap items-center gap-1.5">
                <Server className="w-3 h-3 text-zinc-400" />
                <span className="text-[11.5px] font-semibold text-white break-all">{r.name}</span>
                <span className="text-[10px] font-mono text-zinc-500 uppercase">{r.type}</span>
                {r.status && <ToneBadge tone={r.status === 'running' || r.status === 'active' ? 'emerald' : 'zinc'} label={r.status} />}
                {(r.type === 'vps' || r.type === 'instance') && r.id && (
                  <button
                    type="button"
                    onClick={() => onNavigate('/dashboard/instances', r.id)}
                    className="ml-auto inline-flex items-center gap-1 text-[10.5px] text-zinc-400 hover:text-white"
                  >
                    Open <ArrowUpRight className="w-3 h-3" />
                  </button>
                )}
              </div>
              {Object.entries(r.access ?? {}).map(([k, v]) =>
                k === 'ssh' ? (
                  <CommandBlock key={k} cmd={v} label="SSH" />
                ) : (
                  <div key={k} className="flex items-center gap-2 min-w-0 text-[11px]">
                    <span className="text-zinc-500 font-mono uppercase text-[10px] flex-shrink-0">{k}</span>
                    <code className="font-mono text-zinc-200 break-all min-w-0 flex-1">{v}</code>
                    <CopyButton text={v} />
                  </div>
                )
              )}
            </li>
          ))}
        </ul>
      )}
      {report.warnings.length > 0 && (
        <ul className="space-y-0.5">
          {report.warnings.map((w, i) => (
            <li key={i} className="flex items-start gap-1.5 text-[11px] text-amber-200">
              <TriangleAlert className="w-3 h-3 mt-0.5 flex-shrink-0 text-amber-400" />
              <span className="break-words min-w-0">{w}</span>
            </li>
          ))}
        </ul>
      )}
      {report.next_actions.length > 0 && (
        <div>
          <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500 mb-0.5">Next steps</div>
          <ul className="list-disc pl-4 space-y-0.5 text-[11px] text-zinc-300">
            {report.next_actions.map((a, i) => (
              <li key={i} className="break-words">
                {a}
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  );
};

// ---------------------------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------------------------

export interface ExecutionCardProps {
  executionId: string;
  /** Summary from GET /ai/executions, or the full Execution returned by approve/execute. */
  initial?: ExecutionSummary | Execution | null;
  highlight?: boolean;
  onOpenPlan: (planId: string, opts?: { approve?: boolean; notice?: string }) => void;
  /** The execution became terminal (reload the plan / conversation). */
  onSettled: (execution: Execution) => void;
  onNavigate: (path: string, param?: string) => void;
}

/** Inline execution timeline: tasks, live events, cancel/retry/recovery, final report. */
export const ExecutionCard: React.FC<ExecutionCardProps> = ({ executionId, initial, highlight, onOpenPlan, onSettled, onNavigate }) => {
  const [exec, setExec] = useState<Execution | null>(() =>
    initial && Array.isArray((initial as Execution).tasks) && (initial as Execution).tasks.length > 0 ? (initial as Execution) : null
  );
  const [loadError, setLoadError] = useState<DescribedError | null>(null);
  const [events, setEvents] = useState<ExecutionEvent[]>([]);
  const [eventsState, setEventsState] = useState<'none' | 'loading' | 'loaded'>('none');
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set());
  const [showLog, setShowLog] = useState(false);
  const [restartKey, setRestartKey] = useState(0);
  const [busy, setBusy] = useState<string | null>(null);
  const [actionError, setActionError] = useState<DescribedError | null>(null);
  const [actionNotice, setActionNotice] = useState<string | null>(null);
  const [confirmCancel, setConfirmCancel] = useState(false);
  const [now, setNow] = useState(() => Date.now());

  const lastSeq = useRef(0);
  const execRef = useRef<Execution | null>(exec);
  execRef.current = exec;
  const refreshSeq = useRef(0);
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const unmounted = useRef(false);
  const prevStatus = useRef<string | null>(initial?.status ?? null);

  const status = exec?.status ?? initial?.status ?? 'queued';
  const active = isActiveExecution(status);
  // Live updates start while the execution is active and stop only when the event loop ends itself.
  const [liveWanted, setLiveWanted] = useState(active);
  useEffect(() => {
    if (active) setLiveWanted(true);
  }, [active]);

  const refresh = useCallback(async () => {
    const seq = ++refreshSeq.current;
    try {
      const next = await fetchExecution(executionId);
      if (unmounted.current || seq !== refreshSeq.current) return;
      setExec(next);
      setLoadError(null);
    } catch (err) {
      if (unmounted.current || seq !== refreshSeq.current || isAbortError(err)) return;
      setLoadError(describeError(err, 'Could not load the execution'));
    }
  }, [executionId]);

  /** An action's response is newer than any refresh already in flight: discard those. */
  const applyExec = useCallback((next: Execution) => {
    refreshSeq.current++;
    setExec(next);
  }, []);

  const scheduleRefresh = useCallback(
    (delay = 300) => {
      if (refreshTimer.current) return;
      refreshTimer.current = setTimeout(() => {
        refreshTimer.current = null;
        void refresh();
      }, delay);
    },
    [refresh]
  );

  useEffect(() => {
    unmounted.current = false;
    if (!execRef.current) void refresh();
    return () => {
      unmounted.current = true;
      if (refreshTimer.current) clearTimeout(refreshTimer.current);
    };
  }, [executionId, refresh]);

  // Tell the workspace when a run ends (active -> terminal), so it reloads the plan.
  useEffect(() => {
    if (!exec) return;
    const prev = prevStatus.current;
    prevStatus.current = exec.status;
    if (prev && isActiveExecution(prev) && !isActiveExecution(exec.status)) onSettled(exec);
  }, [exec, onSettled]);

  // Tick durations while running.
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [active]);

  const addEvent = useCallback(
    (ev: ExecutionEvent, live = true) => {
      if (ev.seq <= lastSeq.current) return; // replayed by a resume / overlap with polling
      lastSeq.current = ev.seq;
      setEvents((prev) => (prev.length >= MAX_EVENTS ? [...prev.slice(prev.length - MAX_EVENTS + 1), ev] : [...prev, ev]));
      if (live && REFRESH_EVENT_RE.test(ev.type)) scheduleRefresh(ev.type === 'execution.finished' ? 0 : 300);
    },
    [scheduleRefresh]
  );

  const liveMode = useExecutionLive({
    executionId,
    enabled: liveWanted,
    restartKey,
    terminal: !!exec && !isActiveExecution(exec.status),
    getAfter: () => lastSeq.current,
    onEvent: (ev) => addEvent(ev),
    advance: (nextAfter) => {
      if (nextAfter > lastSeq.current) lastSeq.current = nextAfter;
    },
    isTerminal: () => !!execRef.current && !isActiveExecution(execRef.current.status),
    onFinished: () => {
      setLiveWanted(false);
      setEventsState('loaded');
      void refresh();
    },
  });

  useEffect(() => {
    if (liveMode === 'live' || liveMode === 'polling') setEventsState('loaded');
  }, [liveMode]);

  /** Terminal executions load their event log on demand (task details / event log). */
  const ensureEvents = useCallback(async () => {
    if (eventsState !== 'none' || liveWanted) return;
    setEventsState('loading');
    try {
      for (;;) {
        const page = await fetchExecutionEvents(executionId, lastSeq.current);
        page.events.forEach((ev) => addEvent(ev, false));
        if (page.nextAfter > lastSeq.current) lastSeq.current = page.nextAfter;
        if (page.events.length < EVENTS_PAGE_LIMIT) break;
      }
      if (!unmounted.current) setEventsState('loaded');
    } catch (err) {
      if (!unmounted.current) {
        setEventsState('none');
        setActionError(describeError(err, 'Could not load the event log'));
      }
    }
  }, [eventsState, liveWanted, executionId, addEvent]);

  const toggleTask = (taskId: string) => {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(taskId)) next.delete(taskId);
      else next.add(taskId);
      return next;
    });
    void ensureEvents();
  };

  const act = async (key: string, fn: () => Promise<void>) => {
    setBusy(key);
    setActionError(null);
    setActionNotice(null);
    try {
      await fn();
    } catch (err) {
      setActionError(describeError(err));
    } finally {
      if (!unmounted.current) setBusy(null);
    }
  };

  const reactivate = (next: Execution | null) => {
    if (next) applyExec(next);
    else void refresh();
    setLiveWanted(true);
    setRestartKey((k) => k + 1);
    scheduleRefresh(800);
  };

  const doCancel = async () => {
    setBusy('cancel');
    setActionError(null);
    setActionNotice(null);
    try {
      const next = await cancelExecution(executionId);
      if (next) applyExec(next);
      scheduleRefresh(300);
    } catch (err) {
      setActionError(describeError(err));
    } finally {
      if (!unmounted.current) {
        setBusy(null);
        setConfirmCancel(false);
      }
    }
  };

  const doRetry = (task: ExecutionTask) =>
    act(`retry:${task.id}`, async () => {
      reactivate(await retryExecutionTask(executionId, task.id));
      setActionNotice(`Retrying "${task.title}".`);
    });

  const doRecovery = (action: string) =>
    act(`recovery:${action}`, async () => {
      const res = await recoverExecution(executionId, action);
      if (action === 'delete_created') {
        if (res.planId) {
          onOpenPlan(res.planId, {
            approve: true,
            notice: 'A plan that deletes the resources this execution created is ready. Review it and approve it to delete them.',
          });
          setActionNotice('A deletion plan was created. Review and approve it in the plan panel.');
        } else {
          setActionError({ code: 'BAD_RESPONSE', message: 'The server did not return the deletion plan.', details: [] });
        }
        void refresh();
        return;
      }
      if (action === 'retry_failed') {
        reactivate(res.execution);
        setActionNotice('Retrying the failed tasks.');
        return;
      }
      if (res.execution) applyExec(res.execution);
      else void refresh();
      if (action === 'keep') setActionNotice('Kept the partial result. Nothing was deleted.');
    });

  if (!exec) {
    return (
      <div className="rounded-lg border border-zinc-800 bg-zinc-950 p-3 text-xs" data-execution-id={executionId}>
        {loadError ? (
          <div className="space-y-2">
            <ErrorNote error={loadError} />
            <button type="button" onClick={() => void refresh()} className="text-[11px] text-zinc-300 hover:text-white">
              Retry
            </button>
          </div>
        ) : (
          <div className="flex items-center gap-2 text-zinc-400">
            <Spinner /> Loading execution {shortId(executionId)}…
          </div>
        )}
      </div>
    );
  }

  const total = exec.tasks.length;
  const succeeded = exec.tasks.filter((t) => t.status === 'succeeded').length;
  const failed = exec.tasks.filter((t) => t.status === 'failed').length;
  const pct = total ? Math.round((succeeded / total) * 100) : 0;
  const verification = overallVerification(exec.tasks);
  const live = LIVE_LABEL[liveMode];
  const duration = durationOf(exec.started_at ?? exec.created_at, exec.finished_at, now);
  const recovery = !active && (exec.status === 'failed' || exec.status === 'partially_completed') ? exec.recovery_options : [];

  return (
    <div
      className={`rounded-lg border bg-zinc-950 overflow-hidden transition-shadow ${
        highlight ? 'border-sky-500/40 shadow-[0_0_0_1px_rgba(14,165,233,0.25)]' : 'border-zinc-800'
      }`}
      data-execution-id={exec.id}
      data-execution-status={exec.status}
      data-live-mode={liveMode}
    >
      {/* Header */}
      <div className="flex flex-wrap items-center gap-2 px-3 py-2 border-b border-zinc-800 bg-zinc-900/40">
        <Workflow className="w-3.5 h-3.5 text-zinc-400" />
        <span className="text-xs font-semibold text-white">Execution</span>
        <span className="text-[10px] font-mono text-zinc-500">{shortId(exec.id)}</span>
        <ExecutionStatusBadge status={exec.status} />
        {exec.status === 'completed' && verification && verification !== 'verified' && <VerificationBadge status={verification} />}
        {live && (
          <span className="inline-flex items-center gap-1.5 text-[10px] font-mono text-zinc-400" data-testid="live-indicator">
            <span className={`w-1.5 h-1.5 rounded-full ${live.dot}`} />
            {live.label}
          </span>
        )}
        <div className="ml-auto flex items-center gap-1.5">
          <button
            type="button"
            onClick={() => onOpenPlan(exec.plan_id)}
            className="inline-flex items-center gap-1 text-[10.5px] text-zinc-400 hover:text-white"
            title="Open the plan"
          >
            plan v{exec.plan_version}
            <ArrowUpRight className="w-3 h-3" />
          </button>
          {active && exec.status !== 'cancelling' && (
            <button
              type="button"
              onClick={() => setConfirmCancel(true)}
              disabled={busy !== null}
              className="inline-flex items-center gap-1 px-2 py-0.5 rounded-md border border-rose-500/25 bg-rose-500/10 text-[10.5px] font-medium text-rose-300 hover:bg-rose-500/20 disabled:opacity-50"
              data-testid="execution-cancel"
            >
              <CircleStop className="w-3 h-3" />
              Cancel
            </button>
          )}
        </div>
      </div>

      <div className="p-3 space-y-3">
        {/* Progress */}
        <div className="space-y-1">
          <div className="flex flex-wrap items-center justify-between gap-2 text-[10.5px] font-mono text-zinc-500">
            <span>
              {succeeded}/{total} tasks succeeded{failed > 0 && <span className="text-rose-400"> · {failed} failed</span>}
            </span>
            <span>
              {formatClock(exec.started_at ?? exec.created_at)}
              {duration && ` · ${duration}`}
            </span>
          </div>
          <div className="h-1 rounded-full bg-zinc-800 overflow-hidden">
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                failed > 0 ? 'bg-rose-500' : exec.status === 'completed' ? 'bg-emerald-500' : 'bg-sky-500'
              }`}
              style={{ width: `${pct}%` }}
            />
          </div>
          {exec.cancel_requested && exec.status !== 'cancelled' && (
            <p className="text-[10.5px] text-amber-300">Cancel requested: running tasks finish their current step.</p>
          )}
        </div>

        {/* Tasks */}
        <ol className="space-y-1" data-testid="execution-tasks">
          {exec.tasks.map((task) => (
            <TaskRow
              key={task.id}
              task={task}
              events={events}
              expanded={expanded.has(task.id)}
              onToggle={() => toggleTask(task.id)}
              onRetry={task.status === 'failed' && !active ? () => void doRetry(task) : null}
              retrying={busy === `retry:${task.id}`}
              now={now}
              eventsLoading={eventsState === 'loading'}
            />
          ))}
        </ol>

        {actionNotice && (
          <Notice tone="info" onDismiss={() => setActionNotice(null)}>
            <span data-testid="execution-notice">{actionNotice}</span>
          </Notice>
        )}
        {actionError && <ErrorNote error={actionError} onDismiss={() => setActionError(null)} />}
        {loadError && <ErrorNote error={loadError} onDismiss={() => setLoadError(null)} />}

        {/* Recovery */}
        {recovery.length > 0 && (
          <div className="rounded-md border border-amber-500/25 bg-amber-500/5 p-2.5 space-y-2" data-testid="execution-recovery">
            <div className="flex items-center gap-1.5 text-[11px] font-semibold text-amber-200">
              <LifeBuoy className="w-3.5 h-3.5 text-amber-400" />
              Recovery options
            </div>
            <p className="text-[10.5px] text-zinc-400">Nothing is deleted automatically. Choose what to do with the partial result.</p>
            <div className="flex flex-wrap gap-1.5">
              {recovery.map((o) => (
                <button
                  key={o.action}
                  type="button"
                  onClick={() => void doRecovery(o.action)}
                  disabled={busy !== null}
                  data-recovery={o.action}
                  className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md border text-[11px] font-medium transition-colors disabled:opacity-50 ${
                    o.action === 'delete_created'
                      ? 'border-rose-500/25 bg-rose-500/10 text-rose-300 hover:bg-rose-500/20'
                      : 'border-zinc-700 bg-zinc-900 text-zinc-200 hover:bg-zinc-800'
                  }`}
                >
                  {busy === `recovery:${o.action}` && <Spinner className="w-3 h-3" />}
                  {o.label}
                  {o.requires_approval && <span className="text-[9.5px] font-mono opacity-70">needs approval</span>}
                </button>
              ))}
            </div>
          </div>
        )}

        <ReportCard execution={exec} verification={verification} onNavigate={onNavigate} />

        {/* Event log */}
        <div>
          <button
            type="button"
            onClick={() => {
              setShowLog((v) => !v);
              void ensureEvents();
            }}
            aria-expanded={showLog}
            className="inline-flex items-center gap-1 text-[10.5px] font-mono text-zinc-500 hover:text-zinc-300"
          >
            {showLog ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
            <ScrollText className="w-3 h-3" />
            Event log{events.length > 0 && ` (${events.length})`}
          </button>
          {showLog && (
            <div className="mt-1.5" data-testid="execution-log">
              <EventList events={events} showTask empty={eventsState === 'loading' ? 'Loading…' : 'No events yet.'} />
            </div>
          )}
        </div>
      </div>

      <ConfirmDialog
        open={confirmCancel}
        title="Cancel this execution?"
        description="Queued tasks are cancelled. Running tasks finish their current step first. Resources that were already created are kept."
        confirmLabel="Cancel execution"
        tone="danger"
        busy={busy === 'cancel'}
        onConfirm={() => void doCancel()}
        onClose={() => setConfirmCancel(false)}
      />
    </div>
  );
};

