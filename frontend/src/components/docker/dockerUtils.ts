import { ApiError } from '../../lib/api';
import { DockerContainer, DockerLogLine, DockerLogs, DockerOverview } from '../../types';

export type DockerScope = 'customer' | 'admin';

export const LOG_POLL_INTERVAL_MS = 5000;
export const MAX_LOG_LINES = 5000;
/** Lines longer than this are clipped on screen (copy/download keep the full text). */
export const MAX_RENDERED_LINE_CHARS = 8000;

export const TAIL_OPTIONS = [100, 200, 500, 1000, 5000] as const;
export const DEFAULT_TAIL = 200;

export const SINCE_OPTIONS: ReadonlyArray<{ value: string; label: string }> = [
  { value: '', label: 'All time' },
  { value: '5m', label: 'Last 5m' },
  { value: '15m', label: 'Last 15m' },
  { value: '1h', label: 'Last 1h' },
  { value: '6h', label: 'Last 6h' },
  { value: '24h', label: 'Last 24h' },
];

const SINCE_RE = /^[0-9]{1,4}[smh]$/;

export function dockerBasePath(scope: DockerScope, instanceId: string): string {
  const prefix = scope === 'admin' ? '/admin/instances' : '/instances';
  return `${prefix}/${encodeURIComponent(instanceId)}/docker`;
}

export function dockerLogsPath(base: string, containerRef: string, tail: number, since: string): string {
  const safeTail = Math.min(MAX_LOG_LINES, Math.max(1, Math.round(tail) || DEFAULT_TAIL));
  const params = new URLSearchParams({ tail: String(safeTail) });
  if (since && SINCE_RE.test(since)) params.set('since', since);
  return `${base}/containers/${encodeURIComponent(containerRef)}/logs?${params.toString()}`;
}

// Container metadata comes from inside a tenant-controlled VM, so never trust it as a URL path
// segment without validating it against Docker's own naming rules (blocks "..", "/", etc.).
// Mirrors the backend's containerRefPattern.
const CONTAINER_NAME_RE = /^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$/;
const CONTAINER_ID_RE = /^[a-f0-9]{12,64}$/i;

/** Identifier used in the logs URL: the container name, falling back to its ID. */
export function containerRefOf(container: DockerContainer): string | null {
  const name = (container.name || '').split(',')[0].trim().replace(/^\//, '');
  if (CONTAINER_NAME_RE.test(name)) return name;
  const id = (container.id || '').trim();
  if (CONTAINER_ID_RE.test(id)) return id;
  return null;
}

export function displayNameOf(container: DockerContainer): string {
  return (container.name || '').split(',')[0].trim().replace(/^\//, '') || (container.id || '').slice(0, 12) || '(unnamed)';
}

export function isAbortError(err: unknown): boolean {
  return (err as { name?: string } | null)?.name === 'AbortError';
}

export function errorCode(err: unknown): string {
  return err instanceof ApiError ? err.code : 'NETWORK_ERROR';
}

export function errorMessage(err: unknown): string {
  if (err instanceof Error && err.message) return err.message;
  return 'Request failed';
}

/**
 * Whether a failed logs poll should stop "follow" mode. 4xx responses (bad query, missing
 * container/instance, VM not running, agent/daemon unavailable, auth) need user action;
 * 5xx/504 and network failures are treated as transient and keep polling.
 */
export function shouldStopFollowing(err: unknown): boolean {
  if (!(err instanceof ApiError)) return false;
  switch (err.code) {
    case 'GUEST_AGENT_TIMEOUT':
    case 'GUEST_AGENT_ERROR':
      return false;
    case 'VALIDATION_ERROR':
    case 'CONTAINER_NOT_FOUND':
    case 'INSTANCE_NOT_FOUND':
    case 'INSTANCE_NOT_RUNNING':
    case 'GUEST_AGENT_UNAVAILABLE':
    case 'DOCKER_UNAVAILABLE':
      return true;
    default:
      return err.status !== undefined && err.status >= 400 && err.status < 500;
  }
}

export function normalizeOverview(raw: DockerOverview | null | undefined): DockerOverview {
  return {
    instance_id: raw?.instance_id ?? '',
    hostname: raw?.hostname ?? '',
    vmid: typeof raw?.vmid === 'number' ? raw.vmid : 0,
    node: raw?.node ?? '',
    available: raw?.available === true,
    reason_code: raw?.reason_code ?? '',
    reason: raw?.reason ?? '',
    docker_version: raw?.docker_version ?? '',
    containers: Array.isArray(raw?.containers) ? raw.containers : [],
    fetched_at: raw?.fetched_at ?? '',
  };
}

export interface ViewLogLine {
  ts: string;
  tsLabel: string;
  stderr: boolean;
  text: string;
  lower: string;
}

export interface ViewLogs {
  container: string;
  tail: number;
  since: string;
  truncated: boolean;
  fetchedAt: string;
  lines: ViewLogLine[];
}

// CSI (colors, cursor moves) and OSC (window titles, hyperlinks) escape sequences.
const ANSI_RE = /\u001b\[[0-?]*[ -/]*[@-~]|\u001b\][^\u0007\u001b]*(?:\u0007|\u001b\\)/g;

export function stripAnsi(text: string): string {
  return text.indexOf('\u001b') === -1 ? text : text.replace(ANSI_RE, '');
}

const LOG_TS_RE = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:?\d{2})?$/;

/** "2026-11-02T15:04:05.123456789Z" -> "2026-11-02 15:04:05.123Z" (raw value kept for export). */
export function formatLogTimestamp(ts: string): string {
  const m = LOG_TS_RE.exec(ts);
  if (!m) return ts;
  const millis = (m[3] ?? '').padEnd(3, '0').slice(0, 3);
  return `${m[1]} ${m[2]}.${millis}${m[4] ?? ''}`;
}

function toViewLine(line: DockerLogLine): ViewLogLine {
  const text = stripAnsi(String(line?.message ?? '')).replace(/\r+$/, '');
  const ts = typeof line?.ts === 'string' ? line.ts : '';
  return { ts, tsLabel: formatLogTimestamp(ts), stderr: line?.stream === 'stderr', text, lower: text.toLowerCase() };
}

export function normalizeLogs(raw: DockerLogs | null | undefined): ViewLogs {
  const lines = Array.isArray(raw?.lines) ? raw.lines.slice(-MAX_LOG_LINES) : [];
  return {
    container: raw?.container ?? '',
    tail: typeof raw?.tail === 'number' ? raw.tail : 0,
    since: raw?.since ?? '',
    truncated: raw?.truncated === true,
    fetchedAt: raw?.fetched_at ?? '',
    lines: lines.map(toViewLine),
  };
}

export function logsToText(lines: ViewLogLine[], withTimestamps: boolean): string {
  return lines.map((l) => (withTimestamps && l.ts ? `${l.ts} ${l.text}` : l.text)).join('\n');
}

export function safeFilePart(value: string): string {
  return value.replace(/[^a-zA-Z0-9_.-]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 64) || 'container';
}

// docker ps "CreatedAt" looks like "2026-10-24 09:12:44 +0000 UTC"; the backend may also send RFC 3339.
const DOCKER_CREATED_RE = /^(\d{4}-\d{2}-\d{2})[ T](\d{2}:\d{2}:\d{2})(?:\.\d+)?\s*([+-]\d{2}):?(\d{2})/;

export function parseDockerDate(value: string): Date | null {
  if (!value) return null;
  const m = DOCKER_CREATED_RE.exec(value);
  const parsed = m ? Date.parse(`${m[1]}T${m[2]}${m[3]}:${m[4]}`) : Date.parse(value);
  return Number.isNaN(parsed) ? null : new Date(parsed);
}

export function formatAgo(fromMs: number, nowMs: number = Date.now()): string {
  const s = Math.max(0, Math.round((nowMs - fromMs) / 1000));
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 48) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

/** Parse "12.5%" -> 12.5; returns null when not a number. */
export function parsePercent(value?: string): number | null {
  if (!value) return null;
  const n = parseFloat(value.replace('%', ''));
  return Number.isFinite(n) ? n : null;
}

/** Exit code from a docker status string such as "Exited (137) 2 hours ago". */
export function exitCodeOf(status: string): number | null {
  const m = /Exited \((-?\d+)\)/i.exec(status || '');
  return m ? Number(m[1]) : null;
}

// ---------------------------------------------------------------------------
// Unavailable / error remediation
// ---------------------------------------------------------------------------

export interface DockerIssue {
  tone: 'amber' | 'rose';
  title: string;
  description: string;
  commands?: Array<{ label?: string; cmd: string }>;
  notes?: string[];
}

export const GUEST_AGENT_INSTALL_CMD =
  'sudo apt install -y qemu-guest-agent && sudo systemctl enable --now qemu-guest-agent';
export const DOCKER_INSTALL_CMD = 'curl -fsSL https://get.docker.com | sudo sh';
export const DOCKER_START_CMD = 'sudo systemctl start docker';

function guestAgentNote(scope: DockerScope, vmid?: number): string {
  if (scope === 'admin') {
    return `The VM option "QEMU Guest Agent" must also be enabled in Proxmox (VM${
      vmid ? ` ${vmid}` : ''
    } → Options → QEMU Guest Agent). Changing it requires a full stop/start of the VM; a reboot from inside the guest is not enough.`;
  }
  return 'The VM option "QEMU Guest Agent" must also be enabled on the hypervisor, which requires a full stop/start of the instance. Contact an administrator if Docker is still unavailable after installing the agent.';
}

// The backend reports a Proxmox API token that may not run guest-agent commands as
// GUEST_AGENT_UNAVAILABLE too; installing the agent in the VM would not help there.
const AGENT_FORBIDDEN_RE = /not allowed to run guest-agent|VM\.Monitor|VM\.GuestAgent/i;
const NOT_INSTALLED_RE = /not installed/i;

/**
 * Maps both vocabularies to one remediation: overview `reason_code`s (VM_NOT_RUNNING,
 * DOCKER_DAEMON_UNAVAILABLE, …) and logs error codes (INSTANCE_NOT_RUNNING, DOCKER_UNAVAILABLE, …).
 * `message` (the backend's reason/error text) refines codes that cover several causes.
 */
export function describeDockerIssue(
  code: string,
  opts: { scope: DockerScope; context: 'overview' | 'logs'; vmid?: number; message?: string }
): DockerIssue {
  const { scope, context, vmid, message = '' } = opts;
  if (code === 'GUEST_AGENT_UNAVAILABLE' && AGENT_FORBIDDEN_RE.test(message)) {
    return {
      tone: 'rose',
      title: 'Platform is not allowed to use the guest agent',
      description:
        scope === 'admin'
          ? "The Proxmox API token used by Nevatal Cloud lacks permission to run guest-agent commands. Grant its role VM.Monitor (Proxmox VE 8) or VM.GuestAgent.Unrestricted (Proxmox VE 9). Nothing needs to change inside the VM."
          : 'The platform is not currently permitted to run guest-agent commands on this VM. This is not caused by your instance. Contact the platform administrator.',
    };
  }
  if ((code === 'DOCKER_UNAVAILABLE' || code === 'DOCKER_DAEMON_UNAVAILABLE') && NOT_INSTALLED_RE.test(message)) {
    code = 'DOCKER_NOT_INSTALLED';
  }
  switch (code) {
    case 'VM_NOT_RUNNING':
    case 'INSTANCE_NOT_RUNNING':
      return {
        tone: 'amber',
        title: 'Instance is not running',
        description:
          scope === 'admin'
            ? 'Docker runs inside the virtual machine. Start the instance first (or ask its owner to), then refresh.'
            : 'Docker runs inside the virtual machine. Start the instance first, then refresh.',
      };
    case 'GUEST_AGENT_UNAVAILABLE':
      return {
        tone: 'amber',
        title: 'QEMU guest agent is not responding',
        description:
          'The Docker viewer reads containers through the QEMU guest agent. Install and start it inside the VM:',
        commands: [{ label: 'Debian / Ubuntu', cmd: GUEST_AGENT_INSTALL_CMD }],
        notes: [guestAgentNote(scope, vmid)],
      };
    case 'GUEST_AGENT_TIMEOUT':
      if (context === 'logs') {
        return {
          tone: 'amber',
          title: 'Guest agent timed out while reading logs',
          description:
            'The VM may be under heavy load or the log volume is large. Try a smaller tail or a shorter time window. Follow mode keeps retrying.',
        };
      }
      return {
        tone: 'amber',
        title: 'QEMU guest agent timed out',
        description:
          'The guest agent did not answer in time. It may be missing or stuck, or the VM may be under heavy load. Make sure it is installed and running:',
        commands: [{ label: 'Debian / Ubuntu', cmd: GUEST_AGENT_INSTALL_CMD }],
        notes: [guestAgentNote(scope, vmid)],
      };
    case 'GUEST_AGENT_ERROR':
      return {
        tone: 'rose',
        title: 'Guest agent returned an error',
        description:
          context === 'logs'
            ? 'The command failed inside the VM. Follow mode keeps retrying.'
            : 'The command failed inside the VM. Retry in a moment.',
      };
    case 'DOCKER_NOT_INSTALLED':
      return {
        tone: 'amber',
        title: 'Docker is not installed in this VM',
        description: 'No docker binary was found inside the guest. Install Docker Engine with the official script:',
        commands: [{ cmd: DOCKER_INSTALL_CMD }],
      };
    case 'DOCKER_DAEMON_UNAVAILABLE':
    case 'DOCKER_UNAVAILABLE':
      return {
        tone: 'amber',
        title: 'Docker daemon is not running',
        description: 'The Docker CLI is installed but the daemon is not reachable. Start it inside the VM:',
        commands: [{ cmd: DOCKER_START_CMD }],
      };
    case 'DOCKER_PERMISSION_DENIED':
      return {
        tone: 'rose',
        title: 'Permission denied talking to Docker',
        description:
          'The guest agent runs docker as root, but the Docker daemon refused access. Common causes: SELinux/AppArmor confining the guest agent, unusual permissions on /var/run/docker.sock, or a rootless-only Docker install (not supported). Check inside the VM:',
        commands: [
          { label: 'Socket owner and mode', cmd: 'ls -l /var/run/docker.sock' },
          { label: 'Daemon reachable as root', cmd: 'sudo docker info' },
        ],
      };
    case 'CONTAINER_NOT_FOUND':
      return {
        tone: 'amber',
        title: 'Container not found',
        description: 'It may have been removed or recreated. The container list was refreshed.',
      };
    case 'INSTANCE_NOT_FOUND':
      return {
        tone: 'rose',
        title: 'Instance not found',
        description: 'This instance no longer exists or you do not have access to it.',
      };
    case 'VALIDATION_ERROR':
      return {
        tone: 'rose',
        title: 'Invalid log query',
        description: 'The server rejected the tail/since parameters.',
      };
    case 'NETWORK_ERROR':
      return {
        tone: 'rose',
        title: 'Network error',
        description: 'Could not reach the Nevatal API.',
      };
    default:
      return {
        tone: 'rose',
        title: context === 'overview' ? 'Docker is unavailable' : 'Could not load logs',
        description: 'The request failed.',
      };
  }
}
