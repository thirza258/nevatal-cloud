import React, { useCallback, useEffect, useRef, useState } from 'react';
import { AlertTriangle, Globe, Pencil, PlusCircle, RefreshCw, Server, Trash2 } from 'lucide-react';
import { Domain } from '../../types';
import { Modal } from '../../components/ui/Modal';
import { CopyButton } from '../../components/docker/DockerBits';
import { formatAgo } from '../../components/docker/dockerUtils';
import {
  DomainLink,
  DomainsFeature,
  DomainsGate,
  DomainsHelp,
  DomainsUnavailable,
  EnabledSwitch,
  LiveStatusBadge,
  LiveStatusNote,
} from '../../components/domains/DomainBits';
import { CreateDomainForm, EditDomainForm } from '../../components/domains/DomainForms';
import {
  describeDomainError,
  domainLabel,
  domainUrl,
  fetchDomains,
  isAbortError,
  isDomainsDisabledError,
  isUuid,
  releaseDomain,
  sortDomains,
  updateDomain,
} from '../../lib/domains';

type Navigate = (path: string, param?: string) => void;

const POLL_INTERVAL_MS = 15000;
/** Re-read the list a moment after creating, when the proxy has usually picked the domain up. */
const POST_CREATE_REFRESH_MS = 3000;

/**
 * Reads `?instance=<uuid>` (set by "Add domain" on a VM's page). App routes on the pathname
 * only, so this page owns its query string. Non-UUID values are ignored.
 */
function readInstanceParam(): { id: string | null; invalid: boolean } {
  const raw = new URLSearchParams(window.location.search).get('instance');
  if (!raw) return { id: null, invalid: false };
  return isUuid(raw) ? { id: raw.toLowerCase(), invalid: false } : { id: null, invalid: true };
}

function clearInstanceParam() {
  const params = new URLSearchParams(window.location.search);
  if (!params.has('instance')) return;
  params.delete('instance');
  const query = params.toString();
  window.history.replaceState(
    window.history.state,
    '',
    `${window.location.pathname}${query ? `?${query}` : ''}${window.location.hash}`
  );
}

const omitKey = <T,>(record: Record<string, T>, key: string): Record<string, T> => {
  if (!(key in record)) return record;
  const next = { ...record };
  delete next[key];
  return next;
};

const ROW_GRID =
  'md:grid md:grid-cols-[minmax(0,1.3fr)_minmax(0,1fr)_minmax(0,1.7fr)_4.5rem_auto] md:gap-x-4 md:items-start';

const MobileLabel: React.FC<{ children: React.ReactNode }> = ({ children }) => (
  <div className="md:hidden text-[10px] font-mono uppercase tracking-wider text-zinc-500 mb-0.5">{children}</div>
);

const UsageMeter: React.FC<{ used: number; max: number }> = ({ used, max }) => {
  const atLimit = max > 0 && used >= max;
  const pct = max > 0 ? Math.min(100, (used / max) * 100) : 0;
  return (
    <div
      className="px-2.5 py-1.5 rounded-md bg-zinc-900/60 border border-zinc-800 min-w-[8.5rem]"
      title={max > 0 ? `${used} of ${max} domains used` : `${used} domains`}
    >
      <div className="flex items-center justify-between gap-3 text-[11px] font-mono">
        <span className="text-zinc-500">domains</span>
        <span className={atLimit ? 'text-amber-300' : 'text-zinc-200'}>
          {used}
          {max > 0 && <span className="text-zinc-500"> / {max}</span>}
        </span>
      </div>
      {max > 0 && (
        <div className="mt-1 h-1 rounded-full bg-zinc-800 overflow-hidden">
          <div
            className={`h-full rounded-full ${atLimit ? 'bg-amber-500' : 'bg-emerald-500'}`}
            style={{ width: `${Math.max(used > 0 ? 4 : 0, pct)}%` }}
          />
        </div>
      )}
    </div>
  );
};

const DomainRow: React.FC<{
  domain: Domain;
  pending: boolean;
  error?: string;
  onToggle: (next: boolean) => void;
  onEdit: () => void;
  onRelease: () => void;
  onOpenInstance: (instanceId: string) => void;
}> = ({ domain: d, pending, error, onToggle, onEdit, onRelease, onOpenInstance }) => {
  const url = domainUrl(d.hostname);
  const createdMs = Date.parse(d.created_at);
  const instanceId = d.instance_id;

  let onFix: (() => void) | undefined;
  if (d.live_status === 'instance_stopped' && instanceId) onFix = () => onOpenInstance(instanceId);
  else if (d.live_status === 'instance_missing') onFix = onEdit;
  else if (d.live_status === 'disabled' && !d.admin_disabled && !pending) onFix = () => onToggle(true);

  return (
    <div className={`px-4 py-3 space-y-3 md:space-y-0 ${ROW_GRID}`}>
      {/* Domain */}
      <div className="min-w-0">
        <div className="flex items-center gap-0.5 min-w-0">
          <DomainLink domain={d} />
          {url && <CopyButton text={url} title="Copy URL" className="flex-shrink-0" />}
        </div>
        {Number.isFinite(createdMs) && (
          <div className="text-[10px] font-mono text-zinc-500" title={new Date(createdMs).toLocaleString()}>
            added {formatAgo(createdMs)}
          </div>
        )}
      </div>

      {/* Target */}
      <div className="min-w-0">
        <MobileLabel>Target</MobileLabel>
        {instanceId ? (
          <button
            type="button"
            onClick={() => onOpenInstance(instanceId)}
            title="Open this VM"
            className="inline-flex items-center gap-1.5 max-w-full text-xs font-medium text-zinc-200 hover:text-white hover:underline"
          >
            <Server className="w-3 h-3 flex-shrink-0 text-zinc-500" />
            <span className="truncate">{d.instance_name || 'VM'}</span>
          </button>
        ) : (
          <span className="inline-flex items-center gap-1.5 text-xs text-rose-300">
            <Server className="w-3 h-3 flex-shrink-0 text-rose-400" />
            <span>VM deleted</span>
          </span>
        )}
        <div className="mt-0.5 text-[11px] font-mono text-zinc-400 truncate">
          {d.target_ip ? `${d.target_ip}:${d.target_port}` : `port ${d.target_port}`}
        </div>
      </div>

      {/* Status */}
      <div className="min-w-0 space-y-1">
        <MobileLabel>Status</MobileLabel>
        <LiveStatusBadge domain={d} />
        <LiveStatusNote domain={d} onFix={onFix} />
      </div>

      {/* Enabled */}
      <div className="flex items-center gap-2 md:pt-0.5">
        <EnabledSwitch
          checked={d.enabled}
          onChange={onToggle}
          pending={pending}
          disabled={d.admin_disabled}
          label={`Enable ${domainLabel(d)}`}
          title={
            d.admin_disabled
              ? 'Disabled by an administrator'
              : d.enabled
                ? 'Enabled: click to stop forwarding'
                : 'Off: click to start forwarding'
          }
        />
        <span className="md:hidden text-[11px] text-zinc-400">{d.enabled ? 'Enabled' : 'Off'}</span>
      </div>

      {/* Actions */}
      <div className="flex items-center gap-1 md:justify-end">
        <button
          type="button"
          onClick={onEdit}
          title="Point this domain at another VM or port"
          className="inline-flex items-center gap-1 px-2 py-1 rounded-md border border-zinc-800 bg-zinc-900 text-[11px] font-medium text-zinc-300 hover:text-white hover:border-zinc-700 transition-colors"
        >
          <Pencil className="w-3 h-3" />
          <span>Edit</span>
        </button>
        <button
          type="button"
          onClick={onRelease}
          title="Release this domain"
          className="inline-flex items-center gap-1 px-2 py-1 rounded-md border border-zinc-800 bg-zinc-900 text-[11px] font-medium text-zinc-400 hover:text-rose-300 hover:border-rose-500/30 transition-colors"
        >
          <Trash2 className="w-3 h-3" />
          <span>Release</span>
        </button>
      </div>

      {error && (
        <p role="alert" className="md:col-span-5 md:mt-2 flex items-start gap-1.5 text-[11px] text-rose-300">
          <AlertTriangle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-rose-400" />
          <span className="break-words">{error}</span>
        </p>
      )}
    </div>
  );
};

const DomainsPageContent: React.FC<{ feature: DomainsFeature; onNavigate: Navigate }> = ({ feature, onNavigate }) => {
  const { root, maxPerUser } = feature;

  // ---- list -----------------------------------------------------------------------------------
  const [domains, setDomains] = useState<Domain[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [refreshing, setRefreshing] = useState(false);
  const [turnedOff, setTurnedOff] = useState(false);
  const inflight = useRef<AbortController | null>(null);
  const refreshTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);

  /** Fetches the list; a newer call aborts an older one, so the latest request always wins. */
  const load = useCallback(async (manual = false) => {
    inflight.current?.abort();
    const controller = new AbortController();
    inflight.current = controller;
    if (manual) setRefreshing(true);
    try {
      const list = await fetchDomains(controller.signal);
      if (controller.signal.aborted) return;
      setDomains(list);
      setLoadError(null);
      setLoaded(true);
    } catch (err) {
      if (controller.signal.aborted || isAbortError(err)) return;
      if (isDomainsDisabledError(err)) {
        setTurnedOff(true);
        return;
      }
      setLoadError(describeDomainError(err).message);
    } finally {
      if (inflight.current === controller) {
        inflight.current = null;
        setRefreshing(false);
      }
    }
  }, []);

  useEffect(() => {
    if (turnedOff) return;
    load();
    const timer = setInterval(() => {
      if (document.visibilityState === 'visible' && !inflight.current) load();
    }, POLL_INTERVAL_MS);
    return () => {
      clearInterval(timer);
      inflight.current?.abort();
    };
  }, [load, turnedOff]);

  useEffect(() => () => clearTimeout(refreshTimer.current), []);

  /** Show a mutation's result right away, then re-read the list with a request that starts after it. */
  const applyDomain = useCallback(
    (domain: Domain | null) => {
      if (domain) setDomains((prev) => sortDomains([...prev.filter((x) => x.id !== domain.id), domain]));
      load();
    },
    [load]
  );

  const used = domains.length;
  const atLimit = maxPerUser > 0 && used >= maxPerUser;

  // ---- create (button, or ?instance=<uuid> from a VM's page) -----------------------------------
  const [createOpen, setCreateOpen] = useState(false);
  const [createFor, setCreateFor] = useState<string | null>(null);
  const [createKey, setCreateKey] = useState(0);
  const [createBusy, setCreateBusy] = useState(false);
  const [invalidParam, setInvalidParam] = useState(false);

  useEffect(() => {
    // Initial URL, back/forward, and in-app navigations that only change the query string.
    const sync = () => {
      const param = readInstanceParam();
      setInvalidParam(param.invalid);
      if (param.id) {
        setCreateFor(param.id);
        setCreateKey((k) => k + 1);
        setCreateOpen(true);
      }
    };
    sync();
    window.addEventListener('popstate', sync);
    return () => window.removeEventListener('popstate', sync);
  }, []);

  const openCreate = () => {
    clearInstanceParam();
    setInvalidParam(false);
    setCreateFor(null);
    setCreateKey((k) => k + 1);
    setCreateOpen(true);
  };

  const closeCreate = () => {
    if (createBusy) return;
    setCreateOpen(false);
    clearInstanceParam();
  };

  const handleCreated = (domain: Domain | null) => {
    clearInstanceParam();
    applyDomain(domain);
    clearTimeout(refreshTimer.current);
    refreshTimer.current = setTimeout(() => load(), POST_CREATE_REFRESH_MS);
  };

  // ---- edit -------------------------------------------------------------------------------------
  const [editing, setEditing] = useState<Domain | null>(null);
  const [editBusy, setEditBusy] = useState(false);
  const closeEdit = () => {
    if (!editBusy) setEditing(null);
  };

  // ---- enable / disable ------------------------------------------------------------------------
  const [pending, setPending] = useState<Record<string, boolean>>({});
  const [rowErrors, setRowErrors] = useState<Record<string, string>>({});

  const toggleEnabled = async (domain: Domain, next: boolean) => {
    if (pending[domain.id]) return;
    setPending((p) => ({ ...p, [domain.id]: true }));
    setRowErrors((e) => omitKey(e, domain.id));
    try {
      applyDomain(await updateDomain(domain.id, { enabled: next }));
    } catch (err) {
      if (isDomainsDisabledError(err)) {
        handleTurnedOff();
        return;
      }
      const info = describeDomainError(err);
      setRowErrors((e) => ({ ...e, [domain.id]: info.message }));
      if (info.code === 'DOMAIN_NOT_FOUND') load();
    } finally {
      setPending((p) => omitKey(p, domain.id));
    }
  };

  // ---- release ----------------------------------------------------------------------------------
  const [releasing, setReleasing] = useState<Domain | null>(null);
  const [releaseBusy, setReleaseBusy] = useState(false);
  const [releaseError, setReleaseError] = useState<string | null>(null);

  /** The server says Domains were switched off after this page loaded (config is cached up to 60 s). */
  const handleTurnedOff = () => {
    setTurnedOff(true);
    setCreateOpen(false);
    setEditing(null);
    setReleasing(null);
  };

  const openRelease = (domain: Domain) => {
    setReleaseError(null);
    setReleasing(domain);
  };
  const closeRelease = () => {
    if (!releaseBusy) setReleasing(null);
  };

  const confirmRelease = async () => {
    const target = releasing;
    if (!target || releaseBusy) return;
    setReleaseBusy(true);
    setReleaseError(null);
    const removeLocally = () => {
      setDomains((prev) => prev.filter((x) => x.id !== target.id));
      setRowErrors((e) => omitKey(e, target.id));
      setReleasing(null);
    };
    let reload = true;
    try {
      await releaseDomain(target.id);
      removeLocally();
    } catch (err) {
      if (isDomainsDisabledError(err)) {
        reload = false;
        handleTurnedOff();
        return;
      }
      const info = describeDomainError(err);
      if (info.code === 'DOMAIN_NOT_FOUND') removeLocally();
      else setReleaseError(info.message);
    } finally {
      setReleaseBusy(false);
      if (reload) load();
    }
  };

  const openInstance = (instanceId: string) => onNavigate('/dashboard/instances', instanceId);

  if (turnedOff) return <DomainsUnavailable turnedOff />;

  // ---- render -------------------------------------------------------------------------------------
  let list: React.ReactNode;
  if (!loaded && loadError) {
    list = (
      <div className="p-4">
        <div className="flex flex-wrap items-center justify-between gap-3 p-3 rounded-md border border-rose-500/20 bg-rose-500/5">
          <div className="flex items-start gap-2 min-w-0 text-[11px] text-rose-300">
            <AlertTriangle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-rose-400" />
            <span className="break-words">Could not load your domains: {loadError}</span>
          </div>
          <button
            type="button"
            onClick={() => load(true)}
            className="px-2.5 py-1 rounded-md bg-zinc-900 border border-zinc-800 text-[11px] font-medium text-zinc-200 hover:bg-zinc-800 transition-colors"
          >
            Retry
          </button>
        </div>
      </div>
    );
  } else if (!loaded) {
    list = (
      <div className="py-16 flex items-center justify-center gap-2 text-xs text-zinc-500 font-mono">
        <RefreshCw className="w-3.5 h-3.5 animate-spin" />
        <span>Loading domains…</span>
      </div>
    );
  } else if (domains.length === 0) {
    list = (
      <div className="py-14 px-6 text-center space-y-3">
        <Globe className="w-6 h-6 text-zinc-600 mx-auto" />
        <div className="text-xs font-medium text-zinc-300">No domains yet</div>
        <p className="text-[11px] text-zinc-500 max-w-sm mx-auto leading-relaxed">
          Pick a name, one of your VMs and the port your app listens on. You get{' '}
          <span className="font-mono text-zinc-300">https://name.{root}</span> with HTTPS handled for you.
        </p>
        <button
          type="button"
          onClick={openCreate}
          disabled={atLimit}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white text-zinc-950 text-xs font-medium hover:bg-zinc-200 transition-colors disabled:opacity-40"
        >
          <PlusCircle className="w-3.5 h-3.5" />
          <span>Publish your first domain</span>
        </button>
      </div>
    );
  } else {
    list = (
      <>
        <div
          className={`hidden ${ROW_GRID} px-4 py-2 border-b border-zinc-800 bg-zinc-950/60 text-[10px] font-mono uppercase tracking-wider text-zinc-500`}
        >
          <span>Domain</span>
          <span>Target</span>
          <span>Status</span>
          <span>Enabled</span>
          <span className="text-right">Actions</span>
        </div>
        <div className="divide-y divide-zinc-800/60">
          {domains.map((d) => (
            <DomainRow
              key={d.id}
              domain={d}
              pending={!!pending[d.id]}
              error={rowErrors[d.id]}
              onToggle={(next) => toggleEnabled(d, next)}
              onEdit={() => setEditing(d)}
              onRelease={() => openRelease(d)}
              onOpenInstance={openInstance}
            />
          ))}
        </div>
      </>
    );
  }

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="min-w-0">
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight flex items-center gap-2">
            <Globe className="w-5 h-5 text-zinc-400" />
            <span>Domains</span>
          </h1>
          <p className="text-xs sm:text-sm text-zinc-400 mt-0.5">
            Publish a port of your VM at <span className="font-mono text-zinc-200">https://name.{root}</span>
          </p>
        </div>
        <div className="flex items-center gap-2">
          <UsageMeter used={used} max={maxPerUser} />
          <button
            type="button"
            onClick={() => load(true)}
            disabled={refreshing}
            title="Refresh"
            className="p-2 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-900 border border-zinc-800 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin' : ''}`} />
          </button>
          <button
            type="button"
            onClick={openCreate}
            disabled={atLimit}
            title={atLimit ? `Limit reached (${used}/${maxPerUser}). Release a domain to add another.` : undefined}
            className="inline-flex items-center gap-1.5 px-3 py-2 rounded-md bg-white text-zinc-950 text-xs font-medium hover:bg-zinc-200 transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <PlusCircle className="w-3.5 h-3.5" />
            <span>New domain</span>
          </button>
        </div>
      </div>

      {invalidParam && (
        <div className="flex items-center gap-2 p-2.5 rounded-md bg-amber-500/5 border border-amber-500/20 text-[11px] text-amber-300">
          <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
          <span>Ignored the VM in the link: it is not a valid instance ID.</span>
        </div>
      )}

      {atLimit && loaded && (
        <div className="flex items-center gap-2 p-2.5 rounded-md bg-amber-500/5 border border-amber-500/20 text-[11px] text-amber-300">
          <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
          <span>
            You are using all {maxPerUser} of your domains. Release one to publish another.
          </span>
        </div>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-[minmax(0,1fr)_20rem] gap-4 items-start">
        <section className="min-w-0 bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
          {loaded && loadError && (
            <div className="px-4 py-2 border-b border-zinc-800 bg-rose-500/5 text-[11px] text-rose-300 font-mono break-words">
              Refresh failed: {loadError} Showing the last loaded list.
            </div>
          )}
          {list}
        </section>
        <aside className="min-w-0">
          <DomainsHelp />
        </aside>
      </div>

      {/* Create */}
      <Modal
        isOpen={createOpen}
        onClose={closeCreate}
        title="Publish a domain"
        description={`Forward https://<name>.${root} to a port of one of your VMs.`}
        maxWidth="lg"
      >
        <CreateDomainForm
          key={createKey}
          root={root}
          maxPerUser={maxPerUser}
          usedCount={used}
          initialInstanceId={createFor}
          onBusyChange={setCreateBusy}
          onCreated={handleCreated}
          onDomainsDisabled={handleTurnedOff}
          onClose={closeCreate}
          onNavigate={onNavigate}
        />
      </Modal>

      {/* Edit */}
      <Modal
        isOpen={editing !== null}
        onClose={closeEdit}
        title="Edit domain"
        description={editing ? `Point ${domainLabel(editing)} at another VM or port.` : undefined}
        maxWidth="lg"
      >
        {editing && (
          <EditDomainForm
            key={editing.id}
            domain={editing}
            onBusyChange={setEditBusy}
            onSaved={(updated) => {
              setEditing(null);
              applyDomain(updated);
            }}
            onDomainsDisabled={handleTurnedOff}
            onClose={closeEdit}
            onNavigate={onNavigate}
          />
        )}
      </Modal>

      {/* Release */}
      <Modal
        isOpen={releasing !== null}
        onClose={closeRelease}
        title="Release domain"
        description={releasing ? domainLabel(releasing) : undefined}
      >
        {releasing && (
          <div className="space-y-4">
            <p className="text-xs text-zinc-300 leading-relaxed">
              <span className="font-mono text-white">https://{domainLabel(releasing)}</span> stops working right away
              and visitors get an error instead of your app.
            </p>
            <div className="flex items-start gap-2 p-2.5 rounded-md bg-amber-500/5 border border-amber-500/20 text-[11px] text-amber-200 leading-relaxed">
              <AlertTriangle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-amber-400" />
              <span>
                The name is then quarantined for other users for a while, so nobody else can claim it straight away
                and old links cannot land on someone else&apos;s app.
              </span>
            </div>
            {releaseError && (
              <p role="alert" className="flex items-start gap-1.5 text-[11px] text-rose-300">
                <AlertTriangle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-rose-400" />
                <span className="break-words">{releaseError}</span>
              </p>
            )}
            <div className="flex items-center justify-end gap-2 pt-3 border-t border-zinc-800">
              <button
                type="button"
                onClick={closeRelease}
                disabled={releaseBusy}
                className="px-3 py-1.5 rounded-md bg-zinc-900 border border-zinc-800 text-xs font-medium text-zinc-300 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-50"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={confirmRelease}
                disabled={releaseBusy}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium transition-colors disabled:opacity-50"
              >
                {releaseBusy ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <Trash2 className="w-3.5 h-3.5" />}
                <span>{releaseBusy ? 'Releasing…' : 'Release domain'}</span>
              </button>
            </div>
          </div>
        )}
      </Modal>
    </div>
  );
};

/** Customer page: /dashboard/domains (optionally `?instance=<uuid>` to start creating one for that VM). */
export const DomainsPage: React.FC<{ onNavigate: Navigate }> = ({ onNavigate }) => (
  <DomainsGate>{(feature) => <DomainsPageContent feature={feature} onNavigate={onNavigate} />}</DomainsGate>
);
