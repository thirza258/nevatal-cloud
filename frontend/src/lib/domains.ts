import { ApiError, apiRequest } from './api';
import { Domain, DomainAvailability, DomainPatch, Instance } from '../types';

// ---------------------------------------------------------------------------------------------
// Public Domains: https://<subdomain>.<root> forwards to a port of the customer's VM.
// The server is authoritative for every rule below; these only give early, inline feedback.
// ---------------------------------------------------------------------------------------------

export const SUBDOMAIN_MIN_LENGTH = 3;
export const SUBDOMAIN_MAX_LENGTH = 63;
export const PORT_PRESETS = [80, 3000, 5000, 8000, 8080] as const;
export const AVAILABILITY_DEBOUNCE_MS = 400;
export const ADMIN_REASON_MAX_LENGTH = 255;

const SUBDOMAIN_RE = /^[a-z0-9](?:[a-z0-9-]*[a-z0-9])$/;
// Strict DNS name with at least two labels. Hostnames come from the API, so only a value that
// passes this check is ever turned into a link.
const HOSTNAME_RE = /^(?=.{1,253}$)(?:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?\.)+[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$/;
const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/** What the user typed, trimmed and lowercased. Nothing else is rewritten. */
export const normalizeSubdomainInput = (raw: string): string => raw.trim().toLowerCase();

/** Client copy of the server's subdomain rules. Returns why `value` is invalid, or null if it passes. */
export function subdomainProblem(value: string): string | null {
  if (value.length < SUBDOMAIN_MIN_LENGTH) return `Use at least ${SUBDOMAIN_MIN_LENGTH} characters.`;
  if (value.length > SUBDOMAIN_MAX_LENGTH) return `Use at most ${SUBDOMAIN_MAX_LENGTH} characters.`;
  if (!/^[a-z0-9-]+$/.test(value)) return 'Use only lowercase letters (a-z), digits and hyphens.';
  if (!SUBDOMAIN_RE.test(value)) return 'Start and end with a letter or a digit.';
  if (value.startsWith('xn--')) return 'Names starting with "xn--" are not allowed.';
  if (value.includes('nevatal')) return 'Names containing "nevatal" are reserved.';
  return null;
}

/** Port typed by the user, or null unless it is a whole number from 1 to 65535. */
export function parsePort(raw: string | number): number | null {
  const text = String(raw).trim();
  if (!/^\d{1,5}$/.test(text)) return null;
  const port = Number(text);
  return port >= 1 && port <= 65535 ? port : null;
}

/** `https://<hostname>` when the hostname is a well-formed DNS name, else null (render it as text). */
export function domainUrl(hostname: string): string | null {
  const host = (hostname || '').trim().toLowerCase();
  return HOSTNAME_RE.test(host) ? `https://${host}` : null;
}

export const isUuid = (value: string): boolean => UUID_RE.test(value);

export const sameId = (a: string | null | undefined, b: string | null | undefined): boolean =>
  !!a && !!b && a.toLowerCase() === b.toLowerCase();

const INSTANCE_RANK: Record<string, number> = {
  running: 0,
  starting: 1,
  restarting: 1,
  provisioning: 2,
  queued: 2,
  stopping: 3,
  stopped: 4,
};

/** Running VMs first, then VMs on their way up, then stopped ones; alphabetical within a group. */
export function sortInstancesForPicker(list: Instance[]): Instance[] {
  const rank = (i: Instance) => INSTANCE_RANK[i.status] ?? 5;
  return [...list].sort((a, b) => rank(a) - rank(b) || (a.name || '').localeCompare(b.name || ''));
}

/** Newest first, so rows don't jump around between refreshes. */
export function sortDomains(list: Domain[]): Domain[] {
  return [...list].sort(
    (a, b) => (b.created_at || '').localeCompare(a.created_at || '') || a.hostname.localeCompare(b.hostname)
  );
}

// ---- response normalization (Go encodes empty slices as null; never trust shapes blindly) ----

const asRecord = (value: unknown): Record<string, unknown> | null =>
  value && typeof value === 'object' && !Array.isArray(value) ? (value as Record<string, unknown>) : null;
const str = (value: unknown): string => (typeof value === 'string' ? value : '');
const strOrNull = (value: unknown): string | null => (typeof value === 'string' && value ? value : null);

export function normalizeDomain(raw: unknown): Domain | null {
  const r = asRecord(raw);
  if (!r) return null;
  const id = typeof r.id === 'number' ? String(r.id) : str(r.id);
  if (!id) return null;
  const owner = asRecord(r.owner);
  return {
    id,
    subdomain: str(r.subdomain),
    hostname: str(r.hostname).toLowerCase(),
    url: str(r.url),
    instance_id: strOrNull(r.instance_id),
    instance_name: str(r.instance_name),
    instance_status: str(r.instance_status),
    target_port: typeof r.target_port === 'number' && Number.isFinite(r.target_port) ? r.target_port : 0,
    target_ip: strOrNull(r.target_ip),
    enabled: r.enabled === true,
    admin_disabled: r.admin_disabled === true,
    disabled_reason: str(r.disabled_reason),
    live_status: str(r.live_status),
    created_at: str(r.created_at),
    updated_at: str(r.updated_at),
    owner: owner
      ? { id: str(owner.id), email: str(owner.email), display_name: str(owner.display_name) }
      : undefined,
  };
}

function normalizeDomainList(raw: unknown): Domain[] {
  if (!Array.isArray(raw)) return [];
  return sortDomains(raw.map(normalizeDomain).filter((d): d is Domain => d !== null));
}

/** Display name of a domain: its hostname, else the bare subdomain. */
export const domainLabel = (d: Pick<Domain, 'hostname' | 'subdomain'>): string => d.hostname || d.subdomain || '(unnamed)';

// ---- API ----------------------------------------------------------------------------------------

const domainPath = (id: string) => `/domains/${encodeURIComponent(id)}`;

export async function fetchDomains(signal?: AbortSignal): Promise<Domain[]> {
  return normalizeDomainList(await apiRequest<unknown>('/domains', { signal }));
}

export async function fetchAdminDomains(signal?: AbortSignal): Promise<Domain[]> {
  return normalizeDomainList(await apiRequest<unknown>('/admin/domains', { signal }));
}

export async function checkSubdomain(subdomain: string, signal?: AbortSignal): Promise<DomainAvailability> {
  const query = new URLSearchParams({ subdomain }).toString();
  const r = asRecord(await apiRequest<unknown>(`/domains/check?${query}`, { signal })) ?? {};
  return {
    subdomain: str(r.subdomain) || subdomain,
    hostname: str(r.hostname),
    available: r.available === true,
    reason_code: str(r.reason_code),
    reason: str(r.reason),
  };
}

/** Returns the created domain, or null if the response body was not a domain (caller refetches). */
export async function createDomain(input: {
  subdomain: string;
  instance_id: string;
  target_port: number;
}): Promise<Domain | null> {
  return normalizeDomain(
    await apiRequest<unknown>('/domains', { method: 'POST', body: JSON.stringify(input) })
  );
}

export async function updateDomain(id: string, patch: DomainPatch): Promise<Domain | null> {
  return normalizeDomain(
    await apiRequest<unknown>(domainPath(id), { method: 'PATCH', body: JSON.stringify(patch) })
  );
}

export async function releaseDomain(id: string): Promise<void> {
  await apiRequest<unknown>(domainPath(id), { method: 'DELETE' });
}

export async function setDomainAdminDisabled(
  id: string,
  adminDisabled: boolean,
  reason?: string
): Promise<Domain | null> {
  const body = adminDisabled ? { admin_disabled: true, disabled_reason: reason ?? '' } : { admin_disabled: false };
  return normalizeDomain(
    await apiRequest<unknown>(`/admin/domains/${encodeURIComponent(id)}`, {
      method: 'PATCH',
      body: JSON.stringify(body),
    })
  );
}

/** The signed-in user's VMs (customer endpoint). */
export async function fetchMyInstances(signal?: AbortSignal): Promise<Instance[]> {
  const raw = await apiRequest<unknown>('/instances', { signal });
  return Array.isArray(raw)
    ? (raw as Instance[]).filter((i) => !!i && typeof i.id === 'string' && i.status !== 'deleting')
    : [];
}

// ---- errors ------------------------------------------------------------------------------------

export type DomainErrorField = 'subdomain' | 'instance' | 'port' | 'form';

export interface DomainErrorInfo {
  code: string;
  field: DomainErrorField;
  message: string;
}

export const isAbortError = (err: unknown): boolean => (err as { name?: string } | null)?.name === 'AbortError';

export const isDomainsDisabledError = (err: unknown): boolean =>
  err instanceof ApiError && err.code === 'DOMAINS_DISABLED';

/** Which form field a VALIDATION_ERROR is about, from `details.field` or, failing that, the message. */
function validationField(err: ApiError): DomainErrorField {
  const detail = err.details && typeof err.details.field === 'string' ? err.details.field : '';
  const hint = `${detail} ${err.message}`.toLowerCase();
  if (hint.includes('port')) return 'port';
  if (hint.includes('instance')) return 'instance';
  if (hint.includes('subdomain') || hint.includes('name')) return 'subdomain';
  return 'form';
}

/** User-facing message for a failed Domains request, plus the field it belongs to. */
export function describeDomainError(err: unknown, maxPerUser = 0): DomainErrorInfo {
  if (!(err instanceof ApiError)) {
    const detail = err instanceof Error && err.message ? ` (${err.message})` : '';
    return {
      code: 'NETWORK_ERROR',
      field: 'form',
      message: `Could not reach the server${detail}. Check your connection and try again.`,
    };
  }
  const code = err.code;
  switch (code) {
    case 'SUBDOMAIN_RESERVED':
      return { code, field: 'subdomain', message: 'This name is reserved. Pick another one.' };
    case 'SUBDOMAIN_TAKEN':
      return { code, field: 'subdomain', message: 'This name is already in use. Pick another one.' };
    case 'SUBDOMAIN_QUARANTINED':
      return {
        code,
        field: 'subdomain',
        message: 'This name was released recently and is quarantined for a while. Pick another one.',
      };
    case 'DOMAIN_LIMIT_REACHED':
      return {
        code,
        field: 'form',
        message:
          maxPerUser > 0
            ? `You have reached your limit of ${maxPerUser} domain${maxPerUser === 1 ? '' : 's'}. Release one to add another.`
            : 'You have reached your domain limit. Release a domain to add another.',
      };
    case 'INSTANCE_NOT_FOUND':
      return { code, field: 'instance', message: 'That VM no longer exists. Pick another one.' };
    case 'DOMAIN_NOT_FOUND':
      return { code, field: 'form', message: 'This domain no longer exists. It may have been released elsewhere.' };
    case 'DOMAINS_DISABLED':
      return { code, field: 'form', message: 'Public domains are turned off on this deployment.' };
    case 'VALIDATION_ERROR':
      return { code, field: validationField(err), message: err.message || 'Some values are not valid.' };
    default:
      return { code, field: 'form', message: err.message || 'The request failed. Try again.' };
  }
}

// ---- live status ----------------------------------------------------------------------------

export type StatusTone = 'emerald' | 'amber' | 'rose' | 'zinc';

export interface LiveStatusInfo {
  label: string;
  tone: StatusTone;
  /** One line: what the status means for this domain. */
  explanation: string;
  /** What the customer can do about it (customer scope only). */
  fix?: string;
}

type LiveStatusFields = Pick<Domain, 'live_status' | 'admin_disabled' | 'disabled_reason' | 'target_port'>;

export function liveStatusInfo(d: LiveStatusFields, scope: 'customer' | 'admin' = 'customer'): LiveStatusInfo {
  const admin = scope === 'admin';
  const contactSupport = admin ? undefined : 'Contact support';
  switch (d.live_status) {
    case 'live':
      return {
        label: 'Live',
        tone: 'emerald',
        explanation: d.target_port
          ? `Forwarding HTTPS traffic to port ${d.target_port} on the VM.`
          : 'Forwarding HTTPS traffic to the VM.',
      };
    case 'disabled':
      if (d.admin_disabled) {
        const reason = d.disabled_reason.trim();
        return {
          label: 'Disabled by admin',
          tone: 'rose',
          explanation: `An administrator disabled this domain${reason ? `: ${reason}` : '.'}`,
          fix: contactSupport,
        };
      }
      return {
        label: 'Off',
        tone: 'zinc',
        explanation: admin
          ? 'The owner turned this domain off; nothing is forwarded.'
          : 'You turned this domain off, so nothing is forwarded.',
        fix: admin ? undefined : 'Turn it back on',
      };
    case 'instance_missing':
      return {
        label: 'VM missing',
        tone: 'rose',
        explanation: 'The VM this domain pointed to no longer exists.',
        fix: admin ? undefined : 'Edit the domain and point it at another VM',
      };
    case 'instance_stopped':
      return {
        label: 'VM not running',
        tone: 'amber',
        explanation: 'The VM is not running, so there is nothing to forward to.',
        fix: admin ? undefined : 'Start the VM',
      };
    case 'no_ip':
      return {
        label: 'No IP yet',
        tone: 'amber',
        explanation: 'The VM has not reported an IP address yet.',
        fix: admin ? undefined : 'Waiting for the guest agent to report an IP',
      };
    case 'ip_not_routable':
      return {
        label: 'IP not routable',
        tone: 'rose',
        explanation: "The VM's IP address is outside the range the proxy is allowed to reach.",
        fix: contactSupport,
      };
    case 'ip_conflict':
      return {
        label: 'IP conflict',
        tone: 'rose',
        explanation: 'Another VM reports the same IP address, so traffic is held back.',
        fix: contactSupport,
      };
    default:
      return {
        label: d.live_status ? d.live_status.replace(/_/g, ' ') : 'Unknown',
        tone: 'zinc',
        explanation: 'This dashboard does not recognise this status yet.',
      };
  }
}
