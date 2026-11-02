import React, { useCallback, useEffect, useState } from 'react';
import { Activity, Boxes, ChevronRight, Container, RefreshCw, ScrollText } from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { DockerContainer, DockerOverview } from '../../types';
import { Ago, DockerIssuePanel, ToggleChip } from './DockerBits';
import { DockerLogPanel } from './DockerLogPanel';
import {
  DEFAULT_TAIL,
  DockerScope,
  containerRefOf,
  describeDockerIssue,
  displayNameOf,
  dockerBasePath,
  errorCode,
  errorMessage,
  exitCodeOf,
  formatAgo,
  isAbortError,
  normalizeOverview,
  parseDockerDate,
  parsePercent,
} from './dockerUtils';

export interface DockerViewerProps {
  instanceId: string;
  /** 'customer' uses /instances/:id/docker, 'admin' uses /admin/instances/:id/docker. */
  scope: DockerScope;
}

const STATE_STYLES: Record<string, { badge: string; dot: string }> = {
  running: { badge: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20', dot: 'bg-emerald-500' },
  paused: { badge: 'bg-amber-500/10 text-amber-400 border-amber-500/20', dot: 'bg-amber-400' },
  restarting: { badge: 'bg-amber-500/10 text-amber-400 border-amber-500/20', dot: 'bg-amber-400 animate-pulse' },
  removing: { badge: 'bg-orange-500/10 text-orange-400 border-orange-500/20', dot: 'bg-orange-400' },
  dead: { badge: 'bg-rose-500/10 text-rose-400 border-rose-500/20', dot: 'bg-rose-400' },
  created: { badge: 'bg-zinc-800/60 text-zinc-300 border-zinc-700/60', dot: 'bg-zinc-400' },
  exited: { badge: 'bg-zinc-900 text-zinc-400 border-zinc-800', dot: 'bg-zinc-500' },
};

const ContainerStateBadge: React.FC<{ state: string; status: string }> = ({ state, status }) => {
  const normalized = (state || 'unknown').toLowerCase();
  const exitCode = normalized === 'exited' ? exitCodeOf(status) : null;
  const style =
    exitCode !== null && exitCode !== 0
      ? STATE_STYLES.dead
      : STATE_STYLES[normalized] ?? { badge: 'bg-zinc-800/60 text-zinc-300 border-zinc-700/60', dot: 'bg-zinc-400' };
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border font-mono text-[11px] whitespace-nowrap ${style.badge}`}
    >
      <span className={`inline-block w-1.5 h-1.5 rounded-full ${style.dot}`} />
      <span>{normalized}</span>
      {exitCode !== null && <span className="opacity-70">({exitCode})</span>}
    </span>
  );
};

const UsageCell: React.FC<{ value?: string; percent?: string; detail?: string }> = ({ value, percent, detail }) => {
  const pct = parsePercent(percent ?? value);
  if (!value && pct === null) return <span className="text-zinc-600">—</span>;
  return (
    <div className="min-w-[5.5rem]">
      <div className="text-zinc-200 whitespace-nowrap">{value}</div>
      {pct !== null && (
        <div className="mt-1 h-1 w-full rounded-full bg-zinc-800 overflow-hidden">
          <div
            className={`h-full rounded-full ${pct >= 90 ? 'bg-rose-500' : pct >= 70 ? 'bg-amber-500' : 'bg-emerald-500'}`}
            style={{ width: `${Math.min(100, Math.max(2, pct))}%` }}
          />
        </div>
      )}
      {detail && <div className="mt-0.5 text-[10px] text-zinc-500 whitespace-nowrap">{detail}</div>}
    </div>
  );
};

const CreatedCell: React.FC<{ value: string }> = ({ value }) => {
  const date = parseDockerDate(value);
  if (!date) return <span className="text-zinc-400">{value || '—'}</span>;
  return (
    <span className="text-zinc-400 whitespace-nowrap" title={date.toLocaleString()}>
      {formatAgo(date.getTime())}
    </span>
  );
};

const PortsCell: React.FC<{ ports: string }> = ({ ports }) => {
  const entries = (ports || '')
    .split(',')
    .map((p) => p.trim())
    .filter(Boolean);
  if (entries.length === 0) return <span className="text-zinc-600">—</span>;
  return (
    <div className="space-y-0.5">
      {entries.map((p) => (
        <div key={p} className="text-zinc-300 whitespace-nowrap">
          {p}
        </div>
      ))}
    </div>
  );
};

const DockerViewerImpl: React.FC<DockerViewerProps> = ({ instanceId, scope }) => {
  const base = dockerBasePath(scope, instanceId);

  // ---- overview -------------------------------------------------------------
  const [liveStats, setLiveStats] = useState(false);
  const [overview, setOverview] = useState<DockerOverview | null>(null);
  const [statsIncluded, setStatsIncluded] = useState(false);
  const [receivedAt, setReceivedAt] = useState<number | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<unknown>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    apiRequest<DockerOverview>(`${base}?stats=${liveStats ? 'true' : 'false'}`, { signal: controller.signal })
      .then((raw) => {
        if (controller.signal.aborted) return;
        setOverview(normalizeOverview(raw));
        setStatsIncluded(liveStats);
        setReceivedAt(Date.now());
        setError(null);
      })
      .catch((err) => {
        if (controller.signal.aborted || isAbortError(err)) return;
        setError(err);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [base, liveStats, reloadKey]);

  const refresh = useCallback(() => setReloadKey((k) => k + 1), []);

  // ---- log panel selection + settings (settings survive switching containers) ----
  const [selectedRef, setSelectedRef] = useState<string | null>(null);
  const [tail, setTail] = useState<number>(DEFAULT_TAIL);
  const [since, setSince] = useState('');
  const [showTimestamps, setShowTimestamps] = useState(true);
  const [follow, setFollow] = useState(false);

  const containers = overview?.available ? overview.containers : [];
  const selectedContainer: DockerContainer | undefined = selectedRef
    ? containers.find((c) => containerRefOf(c) === selectedRef)
    : undefined;

  const closeLogs = useCallback(() => {
    setSelectedRef(null);
    setFollow(false);
  }, []);

  const toggleContainer = (ref: string | null) => {
    if (!ref) return;
    if (ref === selectedRef) closeLogs();
    else setSelectedRef(ref);
  };

  const showStatsColumns = liveStats;
  const runningCount = containers.filter((c) => (c.state || '').toLowerCase() === 'running').length;
  const meta = overview
    ? [
        overview.hostname,
        overview.vmid ? `VMID ${overview.vmid}` : '',
        overview.node,
        overview.docker_version ? `Docker ${overview.docker_version}` : '',
      ].filter(Boolean)
    : [];

  // ---- body -------------------------------------------------------------------
  let body: React.ReactNode;
  if (!overview && loading) {
    body = (
      <div className="py-16 flex items-center justify-center gap-2 text-xs text-zinc-500 font-mono">
        <RefreshCw className="w-3.5 h-3.5 animate-spin" />
        <span>Querying Docker through the QEMU guest agent…</span>
      </div>
    );
  } else if (!overview && error) {
    body = (
      <div className="p-4">
        <DockerIssuePanel
          issue={describeDockerIssue(errorCode(error), {
            scope,
            context: 'overview',
            message: errorMessage(error),
          })}
          code={errorCode(error)}
          detail={errorMessage(error)}
          action={
            <button
              type="button"
              onClick={refresh}
              className="px-2.5 py-1 rounded-md bg-zinc-900 border border-zinc-800 text-[11px] font-medium text-zinc-200 hover:bg-zinc-800 transition-colors"
            >
              Retry
            </button>
          }
        />
      </div>
    );
  } else if (overview && !overview.available) {
    body = (
      <div className="p-4">
        <DockerIssuePanel
          issue={describeDockerIssue(overview.reason_code, {
            scope,
            context: 'overview',
            vmid: overview.vmid,
            message: overview.reason,
          })}
          code={overview.reason_code || undefined}
          detail={overview.reason || undefined}
        />
      </div>
    );
  } else if (overview && containers.length === 0) {
    body = (
      <div className="py-14 px-4 text-center space-y-2">
        <Boxes className="w-6 h-6 text-zinc-600 mx-auto" />
        <div className="text-xs font-medium text-zinc-300">No containers on this VM</div>
        <p className="text-[11px] text-zinc-500">
          Containers started with <code className="font-mono text-zinc-400">docker run</code> or{' '}
          <code className="font-mono text-zinc-400">docker compose up</code> appear here, including stopped ones.
        </p>
      </div>
    );
  } else {
    body = (
      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="bg-zinc-950/60 text-zinc-500 border-b border-zinc-800 uppercase tracking-wider text-[10px] font-mono">
            <tr>
              <th className="py-2.5 px-4 font-medium">Name</th>
              <th className="py-2.5 px-4 font-medium">Image</th>
              <th className="py-2.5 px-4 font-medium">State</th>
              <th className="py-2.5 px-4 font-medium">Status</th>
              <th className="py-2.5 px-4 font-medium">Ports</th>
              <th className="py-2.5 px-4 font-medium">Created</th>
              {showStatsColumns && <th className="py-2.5 px-4 font-medium">CPU</th>}
              {showStatsColumns && <th className="py-2.5 px-4 font-medium">Memory</th>}
              <th className="py-2.5 px-4 font-medium text-right">Logs</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-zinc-800/60 font-mono text-[11px]">
            {containers.map((c) => {
              const ref = containerRefOf(c);
              const selected = ref !== null && ref === selectedRef;
              return (
                <tr
                  key={c.id || c.name}
                  onClick={() => toggleContainer(ref)}
                  className={`transition-colors ${ref ? 'cursor-pointer' : ''} ${
                    selected ? 'bg-zinc-800/50' : 'hover:bg-zinc-900/60'
                  }`}
                >
                  <td className="py-2.5 px-4 font-sans">
                    <div className="flex items-center gap-1.5">
                      {selected && <ChevronRight className="w-3 h-3 text-white flex-shrink-0" />}
                      <span className="text-xs font-semibold text-white truncate max-w-[14rem]" title={c.name}>
                        {displayNameOf(c)}
                      </span>
                    </div>
                    <div className="text-[10px] font-mono text-zinc-500" title={c.id}>
                      {(c.id || '').slice(0, 12)}
                    </div>
                  </td>
                  <td className="py-2.5 px-4 text-zinc-300">
                    <div className="truncate max-w-[16rem]" title={c.command ? `${c.image}\n${c.command}` : c.image}>
                      {c.image}
                    </div>
                  </td>
                  <td className="py-2.5 px-4">
                    <ContainerStateBadge state={c.state} status={c.status} />
                  </td>
                  <td className="py-2.5 px-4 text-zinc-400 whitespace-nowrap">{c.status || '—'}</td>
                  <td className="py-2.5 px-4">
                    <PortsCell ports={c.ports} />
                  </td>
                  <td className="py-2.5 px-4">
                    <CreatedCell value={c.created_at} />
                  </td>
                  {showStatsColumns && (
                    <td className="py-2.5 px-4">
                      {statsIncluded ? <UsageCell value={c.cpu_percent} /> : <span className="text-zinc-600">…</span>}
                    </td>
                  )}
                  {showStatsColumns && (
                    <td className="py-2.5 px-4">
                      {statsIncluded ? (
                        <UsageCell value={c.mem_usage} percent={c.mem_percent} detail={c.mem_percent} />
                      ) : (
                        <span className="text-zinc-600">…</span>
                      )}
                    </td>
                  )}
                  <td className="py-2.5 px-4 text-right">
                    <button
                      type="button"
                      disabled={!ref}
                      onClick={(e) => {
                        e.stopPropagation();
                        toggleContainer(ref);
                      }}
                      title={ref ? 'View logs' : 'Container name is not a valid Docker identifier'}
                      className={`inline-flex items-center gap-1 px-2 py-1 rounded-md border font-sans text-[11px] font-medium transition-colors disabled:opacity-40 ${
                        selected
                          ? 'bg-zinc-800 border-zinc-700 text-white'
                          : 'bg-zinc-900 border-zinc-800 text-zinc-300 hover:text-white'
                      }`}
                    >
                      <ScrollText className="w-3 h-3" />
                      <span>Logs</span>
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
        {/* Header */}
        <div className="px-4 py-3 border-b border-zinc-800 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="w-7 h-7 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center flex-shrink-0">
              <Container className="w-3.5 h-3.5 text-zinc-300" />
            </div>
            <div className="min-w-0">
              <div className="flex items-center gap-2">
                <h3 className="text-xs font-semibold text-white">Docker Containers</h3>
                {overview?.available && (
                  <span className="text-[10px] font-mono text-zinc-500">
                    {runningCount}/{containers.length} running
                  </span>
                )}
              </div>
              <p className="text-[11px] text-zinc-400 font-mono truncate">
                {meta.length > 0 ? meta.join(' · ') : 'Read-only view via QEMU guest agent'}
              </p>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <ToggleChip
              active={liveStats}
              onClick={() => setLiveStats((v) => !v)}
              icon={Activity}
              label="Live stats"
              title="Include CPU and memory usage (docker stats, slower)"
            />
            <span
              className="text-[11px] font-mono text-zinc-500 whitespace-nowrap"
              title={overview?.fetched_at ? `Server time: ${overview.fetched_at}` : undefined}
            >
              fetched <Ago at={receivedAt} />
            </span>
            <button
              type="button"
              onClick={refresh}
              disabled={loading}
              title="Refresh containers"
              className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 border border-transparent hover:border-zinc-700 transition-colors disabled:opacity-50"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            </button>
          </div>
        </div>

        {/* Refresh failed but we still have the previous snapshot */}
        {overview && error ? (
          <div className="px-4 py-2 border-b border-zinc-800 bg-rose-500/5 text-[11px] text-rose-300 font-mono break-words">
            Refresh failed ({errorCode(error)}): {errorMessage(error)}. Showing the previous snapshot.
          </div>
        ) : null}

        {body}
      </div>

      {selectedRef && (
        <DockerLogPanel
          key={selectedRef}
          base={base}
          scope={scope}
          containerRef={selectedRef}
          containerLabel={selectedContainer ? displayNameOf(selectedContainer) : selectedRef}
          containerImage={selectedContainer?.image}
          hostname={overview?.hostname}
          vmid={overview?.vmid}
          tail={tail}
          onTailChange={setTail}
          since={since}
          onSinceChange={setSince}
          showTimestamps={showTimestamps}
          onShowTimestampsChange={setShowTimestamps}
          follow={follow}
          onFollowChange={setFollow}
          onClose={closeLogs}
          onContainerMissing={refresh}
        />
      )}
    </div>
  );
};

/**
 * Read-only Docker container list + log viewer for a VM, fetched through the backend's
 * QEMU-guest-agent bridge. Memoized so parent polling (e.g. InstanceDetails' 5 s refresh)
 * doesn't re-render the table or the log list.
 */
export const DockerViewer = React.memo(DockerViewerImpl);
