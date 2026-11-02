import React, { useCallback, useEffect, useMemo, useState } from 'react';
import { AlertTriangle, Ban, CheckCircle2, Globe, RefreshCw, Search, Server, ShieldAlert } from 'lucide-react';
import { Domain } from '../../types';
import { useAuth } from '../../context/AuthContext';
import { Modal } from '../../components/ui/Modal';
import { StatusBadge } from '../../components/ui/Badge';
import { formatAgo } from '../../components/docker/dockerUtils';
import {
  DomainLink,
  DomainsGate,
  DomainsUnavailable,
  LiveStatusBadge,
  LiveStatusNote,
} from '../../components/domains/DomainBits';
import {
  ADMIN_REASON_MAX_LENGTH,
  describeDomainError,
  domainLabel,
  fetchAdminDomains,
  isAbortError,
  isDomainsDisabledError,
  setDomainAdminDisabled,
  sortDomains,
} from '../../lib/domains';

interface AdminDomainsProps {
  onNavigate?: (path: string, param?: string) => void;
}

type Filter = 'all' | 'live' | 'attention' | 'admin_disabled';

const matchesFilter = (d: Domain, filter: Filter): boolean => {
  switch (filter) {
    case 'live':
      return d.live_status === 'live';
    case 'attention':
      return d.live_status !== 'live' && d.live_status !== 'disabled';
    case 'admin_disabled':
      return d.admin_disabled;
    default:
      return true;
  }
};

const FILTERS: Array<{ value: Filter; label: string }> = [
  { value: 'all', label: 'All' },
  { value: 'live', label: 'Live' },
  { value: 'attention', label: 'Needs attention' },
  { value: 'admin_disabled', label: 'Disabled by admin' },
];

const ROW_GRID =
  'lg:grid lg:grid-cols-[minmax(0,1.3fr)_minmax(0,1.1fr)_minmax(0,1fr)_minmax(0,1.6fr)_auto] lg:gap-x-4 lg:items-start';

const MobileLabel: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className="lg:hidden text-[10px] font-mono uppercase tracking-wider text-zinc-500 mb-0.5">{children}</div>
);

type Pending = { domain: Domain; action: 'disable' | 'enable' };

const AdminDomainsContent: React.FC<AdminDomainsProps> = ({ onNavigate }) => {
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [turnedOff, setTurnedOff] = useState(false);
  const [reloadKey, setReloadKey] = useState(0);
  const [search, setSearch] = useState('');
  const [filter, setFilter] = useState<Filter>('all');

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    fetchAdminDomains(controller.signal)
      .then((list) => {
        if (controller.signal.aborted) return;
        setDomains(list);
        setLoaded(true);
        setLoadError(null);
      })
      .catch((err) => {
        if (controller.signal.aborted || isAbortError(err)) return;
        if (isDomainsDisabledError(err)) setTurnedOff(true);
        else setLoadError(describeDomainError(err).message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [reloadKey]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);

  const counts = useMemo(() => {
    const result: Record<Filter, number> = { all: domains.length, live: 0, attention: 0, admin_disabled: 0 };
    for (const d of domains) {
      if (matchesFilter(d, 'live')) result.live++;
      if (matchesFilter(d, 'attention')) result.attention++;
      if (matchesFilter(d, 'admin_disabled')) result.admin_disabled++;
    }
    return result;
  }, [domains]);

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    return domains.filter(
      (d) =>
        matchesFilter(d, filter) &&
        (!q ||
          [
            d.hostname,
            d.subdomain,
            d.owner?.email,
            d.owner?.display_name,
            d.instance_name,
            d.target_ip,
            d.instance_id,
            d.id,
          ].some((v) => v && v.toLowerCase().includes(q)))
    );
  }, [domains, search, filter]);

  // ---- disable / enable modal ------------------------------------------------------------------
  const [pending, setPending] = useState<Pending | null>(null);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const openAction = (domain: Domain, action: Pending['action']) => {
    setPending({ domain, action });
    setReason('');
    setActionError(null);
  };
  const closeAction = () => {
    if (!busy) setPending(null);
  };

  const trimmedReason = reason.trim();
  const canConfirm = !!pending && !busy && (pending.action === 'enable' || trimmedReason.length > 0);

  const confirmAction = async () => {
    if (!pending || !canConfirm) return;
    const { domain, action } = pending;
    setBusy(true);
    setActionError(null);
    try {
      const updated = await setDomainAdminDisabled(domain.id, action === 'disable', trimmedReason);
      if (updated) setDomains((prev) => sortDomains([...prev.filter((x) => x.id !== updated.id), { ...updated, owner: updated.owner ?? domain.owner }]));
      setPending(null);
      reload();
    } catch (err) {
      if (isDomainsDisabledError(err)) {
        setPending(null);
        setTurnedOff(true);
        return;
      }
      const info = describeDomainError(err);
      if (info.code === 'DOMAIN_NOT_FOUND') {
        setPending(null);
        reload();
        return;
      }
      setActionError(info.message);
    } finally {
      setBusy(false);
    }
  };

  if (turnedOff) return <DomainsUnavailable turnedOff />;

  let body: React.ReactNode;
  if (!loaded && loadError) {
    body = (
      <div className="p-4 flex flex-wrap items-center justify-between gap-3 text-[11px] text-rose-300">
        <span className="flex items-start gap-2 min-w-0">
          <AlertTriangle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-rose-400" />
          <span className="break-words">Could not load domains: {loadError}</span>
        </span>
        <button
          type="button"
          onClick={reload}
          className="px-2.5 py-1 rounded-md bg-zinc-900 border border-zinc-800 text-[11px] font-medium text-zinc-200 hover:bg-zinc-800 transition-colors"
        >
          Retry
        </button>
      </div>
    );
  } else if (!loaded) {
    body = (
      <div className="py-16 flex items-center justify-center gap-2 text-xs text-zinc-500 font-mono">
        <RefreshCw className="w-3.5 h-3.5 animate-spin" />
        <span>Loading domains…</span>
      </div>
    );
  } else if (filtered.length === 0) {
    body = (
      <div className="py-14 text-center text-xs text-zinc-500">
        {domains.length === 0 ? 'No customer has published a domain yet.' : 'No domains match.'}
      </div>
    );
  } else {
    body = (
      <>
        <div
          className={`hidden ${ROW_GRID} px-4 py-2 border-b border-zinc-800 bg-zinc-950/60 text-[10px] font-mono uppercase tracking-wider text-zinc-500`}
        >
          <span>Domain</span>
          <span>Owner</span>
          <span>Target</span>
          <span>Status</span>
          <span className="text-right">Actions</span>
        </div>
        <div className="divide-y divide-zinc-800/60">
          {filtered.map((d) => {
            const createdMs = Date.parse(d.created_at);
            const instanceId = d.instance_id;
            return (
              <div key={d.id} className={`px-4 py-3 space-y-3 lg:space-y-0 ${ROW_GRID}`}>
                <div className="min-w-0">
                  <DomainLink domain={d} />
                  <div className="text-[10px] font-mono text-zinc-500">
                    {Number.isFinite(createdMs) ? (
                      <span title={new Date(createdMs).toLocaleString()}>added {formatAgo(createdMs)}</span>
                    ) : null}
                    {!d.enabled && !d.admin_disabled && <span> · off by owner</span>}
                  </div>
                </div>

                <div className="min-w-0">
                  <MobileLabel>Owner</MobileLabel>
                  <div className="text-xs text-zinc-200 truncate">{d.owner?.display_name || '—'}</div>
                  <div className="text-[11px] font-mono text-zinc-400 truncate">{d.owner?.email || d.owner?.id || ''}</div>
                </div>

                <div className="min-w-0">
                  <MobileLabel>Target</MobileLabel>
                  <div className="flex items-center gap-1.5 min-w-0">
                    <Server className="w-3 h-3 flex-shrink-0 text-zinc-500" />
                    {instanceId && onNavigate ? (
                      <button
                        type="button"
                        onClick={() => onNavigate(`/admin/docker?instance=${encodeURIComponent(instanceId)}`)}
                        title="Inspect this VM in the Docker viewer"
                        className="text-xs text-zinc-200 truncate hover:text-white hover:underline"
                      >
                        {d.instance_name || instanceId}
                      </button>
                    ) : (
                      <span className={`text-xs truncate ${instanceId ? 'text-zinc-200' : 'text-rose-300'}`}>
                        {instanceId ? d.instance_name || instanceId : 'VM deleted'}
                      </span>
                    )}
                    {d.instance_status && <StatusBadge status={d.instance_status} size="sm" />}
                  </div>
                  <div className="mt-0.5 text-[11px] font-mono text-zinc-400 truncate">
                    {d.target_ip ? `${d.target_ip}:${d.target_port}` : `no IP · port ${d.target_port}`}
                  </div>
                </div>

                <div className="min-w-0 space-y-1">
                  <MobileLabel>Status</MobileLabel>
                  <LiveStatusBadge domain={d} scope="admin" />
                  <LiveStatusNote domain={d} scope="admin" />
                </div>

                <div className="flex items-center lg:justify-end">
                  {d.admin_disabled ? (
                    <button
                      type="button"
                      onClick={() => openAction(d, 'enable')}
                      className="inline-flex items-center gap-1 px-2 py-1 rounded-md border border-emerald-500/30 bg-emerald-500/5 text-[11px] font-medium text-emerald-300 hover:bg-emerald-500/10 transition-colors"
                    >
                      <CheckCircle2 className="w-3 h-3" />
                      <span>Enable</span>
                    </button>
                  ) : (
                    <button
                      type="button"
                      onClick={() => openAction(d, 'disable')}
                      className="inline-flex items-center gap-1 px-2 py-1 rounded-md border border-zinc-800 bg-zinc-900 text-[11px] font-medium text-zinc-300 hover:text-rose-300 hover:border-rose-500/30 transition-colors"
                    >
                      <Ban className="w-3 h-3" />
                      <span>Disable</span>
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      </>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight flex items-center gap-2">
            <Globe className="w-5 h-5 text-zinc-400" />
            <span>Domains</span>
          </h1>
          <p className="text-xs sm:text-sm text-zinc-400 mt-0.5">
            Every customer domain, who owns it, where it points and whether it is live
          </p>
        </div>
        <div className="flex items-center gap-2 w-full sm:w-auto">
          <div className="relative flex-1 sm:w-72">
            <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
            <input
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Hostname, owner, VM, IP…"
              aria-label="Search domains"
              className="w-full bg-zinc-900 border border-zinc-800 rounded-md pl-8 pr-3 py-1.5 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
            />
          </div>
          <button
            type="button"
            onClick={reload}
            disabled={loading}
            title="Reload domains"
            className="p-2 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-900 border border-zinc-800 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label="Filter by status">
        {FILTERS.map((f) => {
          const active = filter === f.value;
          return (
            <button
              key={f.value}
              type="button"
              aria-pressed={active}
              onClick={() => setFilter(f.value)}
              className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-md border text-[11px] font-medium transition-colors ${
                active
                  ? 'bg-zinc-800 border-zinc-700 text-white'
                  : 'bg-zinc-900 border-zinc-800 text-zinc-400 hover:text-zinc-200'
              }`}
            >
              <span>{f.label}</span>
              <span className="font-mono text-zinc-500">{counts[f.value]}</span>
            </button>
          );
        })}
      </div>

      <section className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
        {loaded && loadError && (
          <div className="px-4 py-2 border-b border-zinc-800 bg-rose-500/5 text-[11px] text-rose-300 font-mono break-words">
            Reload failed: {loadError} Showing the last loaded list.
          </div>
        )}
        {body}
      </section>

      <Modal
        isOpen={pending !== null}
        onClose={closeAction}
        title={pending?.action === 'enable' ? 'Re-enable domain' : 'Disable domain'}
        description={pending ? domainLabel(pending.domain) : undefined}
      >
        {pending && (
          <div className="space-y-4">
            {pending.action === 'disable' ? (
              <>
                <div className="flex items-start gap-2 p-2.5 rounded-md bg-rose-500/5 border border-rose-500/20 text-[11px] text-rose-200 leading-relaxed">
                  <ShieldAlert className="w-3.5 h-3.5 mt-px flex-shrink-0 text-rose-400" />
                  <span>
                    Traffic stops being forwarded right away. The owner sees the reason and cannot switch the domain
                    back on until an admin re-enables it.
                  </span>
                </div>
                <div>
                  <div className="flex items-baseline justify-between mb-1.5">
                    <label htmlFor="admin-domain-reason" className="text-xs font-medium text-zinc-200">
                      Reason <span className="text-zinc-500 font-normal">(shown to the owner)</span>
                    </label>
                    <span className="text-[10px] font-mono text-zinc-500">
                      {reason.length}/{ADMIN_REASON_MAX_LENGTH}
                    </span>
                  </div>
                  <textarea
                    id="admin-domain-reason"
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                    maxLength={ADMIN_REASON_MAX_LENGTH}
                    rows={3}
                    autoFocus
                    placeholder="e.g. Phishing report #1234; contact support to appeal."
                    className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-3 py-2 text-xs text-white placeholder-zinc-600 focus:outline-none focus:border-zinc-600 resize-y"
                  />
                </div>
              </>
            ) : (
              <div className="space-y-2 text-xs text-zinc-300 leading-relaxed">
                <p>
                  Forwarding resumes if the owner has the domain switched on and the VM is reachable.
                </p>
                {pending.domain.disabled_reason && (
                  <p className="text-[11px] text-zinc-400">
                    Disabled because: <span className="text-zinc-200">{pending.domain.disabled_reason}</span>
                  </p>
                )}
              </div>
            )}

            {actionError && (
              <p role="alert" className="flex items-start gap-1.5 text-[11px] text-rose-300">
                <AlertTriangle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-rose-400" />
                <span className="break-words">{actionError}</span>
              </p>
            )}

            <div className="flex items-center justify-end gap-2 pt-3 border-t border-zinc-800">
              <button
                type="button"
                onClick={closeAction}
                disabled={busy}
                className="px-3 py-1.5 rounded-md bg-zinc-900 border border-zinc-800 text-xs font-medium text-zinc-300 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={confirmAction}
                disabled={!canConfirm}
                className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md text-white text-xs font-medium transition-colors disabled:opacity-40 ${
                  pending.action === 'disable' ? 'bg-rose-600 hover:bg-rose-500' : 'bg-emerald-600 hover:bg-emerald-500'
                }`}
              >
                {busy && <RefreshCw className="w-3.5 h-3.5 animate-spin" />}
                <span>
                  {pending.action === 'disable'
                    ? busy
                      ? 'Disabling…'
                      : 'Disable domain'
                    : busy
                      ? 'Enabling…'
                      : 'Re-enable domain'}
                </span>
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

/** Admin page: /admin/domains. Hidden (and never fetched) unless Domains is enabled. */
export const AdminDomains: React.FC<AdminDomainsProps> = ({ onNavigate }) => {
  const { user } = useAuth();
  return (
    <DomainsGate>
      {() =>
        user?.role === 'admin' ? (
          <AdminDomainsContent onNavigate={onNavigate} />
        ) : (
          <div className="bg-zinc-900/40 border border-dashed border-zinc-800 rounded-lg py-16 px-6 text-center space-y-2">
            <ShieldAlert className="w-6 h-6 text-zinc-600 mx-auto" />
            <div className="text-xs font-medium text-zinc-300">Administrators only</div>
            <p className="text-[11px] text-zinc-500">This page lists every customer&apos;s domains.</p>
          </div>
        )
      }
    </DomainsGate>
  );
};
