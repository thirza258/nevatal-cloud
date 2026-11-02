import React, {
  useCallback,
  useDeferredValue,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from 'react';
import {
  ArrowDownToLine,
  Clock,
  Download,
  RefreshCw,
  ScrollText,
  Search,
  TriangleAlert,
  X,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { downloadText } from '../../lib/clipboard';
import { DockerLogs } from '../../types';
import { Ago, CopyButton, DockerIssuePanel, ToggleChip } from './DockerBits';
import {
  DockerScope,
  LOG_POLL_INTERVAL_MS,
  MAX_RENDERED_LINE_CHARS,
  SINCE_OPTIONS,
  TAIL_OPTIONS,
  ViewLogLine,
  ViewLogs,
  describeDockerIssue,
  dockerLogsPath,
  errorCode,
  errorMessage,
  isAbortError,
  logsToText,
  normalizeLogs,
  safeFilePart,
  shouldStopFollowing,
} from './dockerUtils';

interface LogsState {
  data: ViewLogs | null;
  receivedAt: number | null;
  loading: boolean;
  error: unknown;
}

/**
 * Fetches container logs and, while `follow` is on, re-fetches every 5 s.
 * - one request at a time (a poll never starts while another is in flight)
 * - polling pauses while the document is hidden and resumes on visibilitychange
 * - every timer/request is torn down when the URL changes or the panel unmounts
 */
function useContainerLogs(url: string, follow: boolean, onError: (err: unknown) => void) {
  const [state, setState] = useState<LogsState>({ data: null, receivedAt: null, loading: true, error: null });
  const [reloadKey, setReloadKey] = useState(0);
  const followRef = useRef(follow);
  const onErrorRef = useRef(onError);
  const kickRef = useRef<() => void>(() => {});

  useEffect(() => {
    onErrorRef.current = onError;
  }, [onError]);

  useEffect(() => {
    let disposed = false;
    let controller: AbortController | null = null;
    let timer: ReturnType<typeof setTimeout> | null = null;

    const clearTimer = () => {
      if (timer !== null) {
        clearTimeout(timer);
        timer = null;
      }
    };

    const schedule = () => {
      clearTimer();
      if (disposed || !followRef.current) return;
      timer = setTimeout(() => {
        timer = null;
        if (disposed || !followRef.current || document.hidden) return; // resumed by onVisibility
        void load(true);
      }, LOG_POLL_INTERVAL_MS);
    };

    const load = async (background: boolean) => {
      if (disposed || controller) return;
      clearTimer();
      const ctrl = new AbortController();
      controller = ctrl;
      if (!background) setState((s) => ({ ...s, loading: true }));
      let keepPolling = true;
      try {
        const raw = await apiRequest<DockerLogs>(url, { signal: ctrl.signal });
        if (disposed || ctrl.signal.aborted) return;
        setState({ data: normalizeLogs(raw), receivedAt: Date.now(), loading: false, error: null });
      } catch (err) {
        if (disposed || isAbortError(err)) return;
        keepPolling = !shouldStopFollowing(err);
        setState((s) => ({ ...s, loading: false, error: err }));
        onErrorRef.current(err);
      } finally {
        if (controller === ctrl) controller = null;
        if (!disposed && keepPolling) schedule();
      }
    };

    const onVisibility = () => {
      if (document.hidden || disposed || !followRef.current) return;
      if (controller || timer !== null) return;
      void load(true);
    };

    kickRef.current = () => {
      if (!disposed && !controller) void load(true);
    };
    document.addEventListener('visibilitychange', onVisibility);
    void load(false);

    return () => {
      disposed = true;
      clearTimer();
      controller?.abort();
      controller = null;
      kickRef.current = () => {};
      document.removeEventListener('visibilitychange', onVisibility);
    };
  }, [url, reloadKey]);

  // Turning follow on fetches right away; turning it off lets the pending timer lapse.
  useEffect(() => {
    followRef.current = follow;
    if (follow) kickRef.current();
  }, [follow]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);
  return { ...state, reload };
}

const LogLines = React.memo(function LogLines({
  lines,
  showTimestamps,
}: {
  lines: ViewLogLine[];
  showTimestamps: boolean;
}) {
  return (
    <>
      {lines.map((line, i) => {
        const clipped = line.text.length > MAX_RENDERED_LINE_CHARS;
        return (
          <div
            key={i}
            className={`flex gap-3 pl-2.5 pr-3 border-l-2 hover:bg-zinc-900/70 ${
              line.stderr ? 'border-rose-500/50 bg-rose-500/[0.04] text-rose-300' : 'border-transparent text-zinc-300'
            }`}
          >
            {showTimestamps && line.tsLabel && (
              <span className="flex-shrink-0 text-zinc-500 tabular-nums">{line.tsLabel}</span>
            )}
            <span className="min-w-0 whitespace-pre-wrap [overflow-wrap:anywhere]">
              {clipped ? line.text.slice(0, MAX_RENDERED_LINE_CHARS) : line.text || ' '}
              {clipped && (
                <span className="text-zinc-500">
                  {` … (+${line.text.length - MAX_RENDERED_LINE_CHARS} chars, download for full line)`}
                </span>
              )}
            </span>
          </div>
        );
      })}
    </>
  );
});

const selectClass =
  'bg-zinc-900 border border-zinc-800 rounded-md px-2 py-1 text-[11px] font-mono text-zinc-200 focus:outline-none focus:border-zinc-700 cursor-pointer';

export interface DockerLogPanelProps {
  base: string;
  scope: DockerScope;
  containerRef: string;
  containerLabel: string;
  containerImage?: string;
  hostname?: string;
  vmid?: number;
  tail: number;
  onTailChange: (tail: number) => void;
  since: string;
  onSinceChange: (since: string) => void;
  showTimestamps: boolean;
  onShowTimestampsChange: (show: boolean) => void;
  follow: boolean;
  onFollowChange: (follow: boolean) => void;
  onClose: () => void;
  onContainerMissing: () => void;
}

export const DockerLogPanel: React.FC<DockerLogPanelProps> = ({
  base,
  scope,
  containerRef,
  containerLabel,
  containerImage,
  hostname,
  vmid,
  tail,
  onTailChange,
  since,
  onSinceChange,
  showTimestamps,
  onShowTimestampsChange,
  follow,
  onFollowChange,
  onClose,
  onContainerMissing,
}) => {
  const url = dockerLogsPath(base, containerRef, tail, since);

  const handleError = useCallback(
    (err: unknown) => {
      if (shouldStopFollowing(err)) onFollowChange(false);
      if (errorCode(err) === 'CONTAINER_NOT_FOUND') onContainerMissing();
    },
    [onFollowChange, onContainerMissing]
  );

  const { data, receivedAt, loading, error, reload } = useContainerLogs(url, follow, handleError);

  // ---- filtering ----------------------------------------------------------
  const [filter, setFilter] = useState('');
  const deferredFilter = useDeferredValue(filter.trim().toLowerCase());
  const allLines = data?.lines;
  const visibleLines = useMemo(() => {
    const lines = allLines ?? [];
    return deferredFilter ? lines.filter((l) => l.lower.includes(deferredFilter)) : lines;
  }, [allLines, deferredFilter]);
  const totalLines = allLines?.length ?? 0;

  // ---- stick-to-bottom auto-scroll ----------------------------------------
  const scrollRef = useRef<HTMLDivElement>(null);
  const stickRef = useRef(true);
  const [atBottom, setAtBottom] = useState(true);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const bottom = el.scrollHeight - el.scrollTop - el.clientHeight <= 24;
    stickRef.current = bottom;
    setAtBottom((prev) => (prev === bottom ? prev : bottom));
  };

  useLayoutEffect(() => {
    const el = scrollRef.current;
    if (el && stickRef.current) el.scrollTop = el.scrollHeight;
  }, [visibleLines, showTimestamps]);

  const jumpToLatest = () => {
    const el = scrollRef.current;
    if (!el) return;
    stickRef.current = true;
    setAtBottom(true);
    el.scrollTop = el.scrollHeight;
  };

  // ---- export (built on demand; copy/download use the visible lines) ---------
  const exportText = () => logsToText(visibleLines, showTimestamps);

  const handleDownload = () => {
    const stamp = new Date().toISOString().replace(/[:]/g, '-').replace(/\.\d+Z$/, 'Z');
    const host = safeFilePart(hostname || (vmid ? `vm${vmid}` : 'instance'));
    downloadText(`${host}-${safeFilePart(containerLabel)}-${stamp}.log`, exportText());
  };

  const errorIssue = error
    ? describeDockerIssue(errorCode(error), { scope, context: 'logs', vmid, message: errorMessage(error) })
    : null;

  return (
    <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
      {/* Header */}
      <div className="px-4 py-3 border-b border-zinc-800 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2.5 min-w-0">
          <ScrollText className="w-3.5 h-3.5 text-zinc-400 flex-shrink-0" />
          <div className="min-w-0">
            <div className="text-xs font-semibold text-white truncate">
              Logs · <span className="font-mono">{containerLabel}</span>
            </div>
            {containerImage && (
              <div className="text-[11px] font-mono text-zinc-500 truncate">{containerImage}</div>
            )}
          </div>
        </div>
        <div className="flex items-center gap-2 text-[11px] font-mono text-zinc-500">
          {follow && <span className="hidden sm:inline text-emerald-400/90">following · every 5s</span>}
          <span title={data?.fetchedAt ? `Server time: ${data.fetchedAt}` : undefined}>
            fetched <Ago at={receivedAt} />
          </span>
          <button
            type="button"
            onClick={reload}
            disabled={loading}
            title="Refresh logs"
            className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            type="button"
            onClick={onClose}
            title="Close logs"
            className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
          >
            <X className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Toolbar */}
      <div className="px-4 py-2.5 border-b border-zinc-800 flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-1.5 text-[11px] text-zinc-500">
          <span className="font-mono">tail</span>
          <select value={tail} onChange={(e) => onTailChange(Number(e.target.value))} className={selectClass}>
            {TAIL_OPTIONS.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>
        </label>
        <label className="flex items-center gap-1.5 text-[11px] text-zinc-500">
          <span className="font-mono">since</span>
          <select value={since} onChange={(e) => onSinceChange(e.target.value)} className={selectClass}>
            {SINCE_OPTIONS.map((o) => (
              <option key={o.value || 'all'} value={o.value}>
                {o.label}
              </option>
            ))}
          </select>
        </label>
        <ToggleChip
          active={showTimestamps}
          onClick={() => onShowTimestampsChange(!showTimestamps)}
          icon={Clock}
          label="Timestamps"
          title="Show or hide timestamps"
        />
        <ToggleChip
          active={follow}
          onClick={() => onFollowChange(!follow)}
          icon={ArrowDownToLine}
          indicator
          label="Follow"
          title="Re-fetch every 5 seconds (paused while this tab is hidden)"
        />

        <div className="flex items-center gap-2 ml-auto w-full sm:w-auto">
          <div className="relative flex-1 sm:w-56">
            <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter lines…"
              className="w-full bg-zinc-900 border border-zinc-800 rounded-md pl-8 pr-7 py-1 text-[11px] font-mono text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
            />
            {filter && (
              <button
                type="button"
                onClick={() => setFilter('')}
                title="Clear filter"
                className="absolute right-1.5 top-1/2 -translate-y-1/2 p-0.5 text-zinc-500 hover:text-white"
              >
                <X className="w-3 h-3" />
              </button>
            )}
          </div>
          <span className="text-[11px] font-mono text-zinc-500 whitespace-nowrap tabular-nums">
            {deferredFilter ? `${visibleLines.length} / ${totalLines} match` : `${totalLines} lines`}
          </span>
          <CopyButton text={exportText} disabled={visibleLines.length === 0} title="Copy visible lines" />
          <button
            type="button"
            onClick={handleDownload}
            disabled={visibleLines.length === 0}
            title="Download visible lines as .log"
            className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-40 disabled:pointer-events-none"
          >
            <Download className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Notices */}
      {(errorIssue || data?.truncated) && (
        <div className="px-4 pt-3 space-y-2">
          {errorIssue && (
            <DockerIssuePanel issue={errorIssue} code={errorCode(error)} detail={errorMessage(error)} compact />
          )}
          {data?.truncated && (
            <div className="flex items-start gap-2 p-2.5 rounded-md bg-amber-500/5 border border-amber-500/20 text-[11px] text-amber-300">
              <TriangleAlert className="w-3.5 h-3.5 mt-px flex-shrink-0 text-amber-400" />
              <span>
                Output was truncated by the server, so older lines may be missing. Lower the tail or narrow the time
                window to see a complete range.
              </span>
            </div>
          )}
        </div>
      )}

      {/* Log body */}
      <div className="relative p-3">
        <div
          ref={scrollRef}
          onScroll={handleScroll}
          className="h-[28rem] overflow-auto rounded-md bg-zinc-950 border border-zinc-800 py-2 font-mono text-[11px] leading-[1.55]"
        >
          {loading && !data ? (
            <div className="h-full flex items-center justify-center gap-2 text-zinc-500">
              <RefreshCw className="w-3.5 h-3.5 animate-spin" />
              <span>Reading logs through the guest agent…</span>
            </div>
          ) : visibleLines.length === 0 ? (
            <div className="h-full flex items-center justify-center text-zinc-500">
              {totalLines > 0 ? 'No lines match the filter.' : error ? 'No log lines loaded.' : 'No log output in this range.'}
            </div>
          ) : (
            <LogLines lines={visibleLines} showTimestamps={showTimestamps} />
          )}
        </div>

        {!atBottom && visibleLines.length > 0 && (
          <button
            type="button"
            onClick={jumpToLatest}
            className="absolute bottom-6 right-7 inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md bg-zinc-800 border border-zinc-700 text-[11px] font-medium text-white shadow-lg hover:bg-zinc-700 transition-colors"
          >
            <ArrowDownToLine className="w-3 h-3" />
            <span>Latest</span>
          </button>
        )}
      </div>
    </div>
  );
};
