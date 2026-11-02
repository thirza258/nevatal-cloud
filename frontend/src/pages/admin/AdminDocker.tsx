import React, { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, Container, RefreshCw, Search, Server, X } from 'lucide-react';
import { Instance } from '../../types';
import { apiRequest } from '../../lib/api';
import { StatusBadge } from '../../components/ui/Badge';
import { DockerViewer } from '../../components/docker/DockerViewer';

interface AdminDockerProps {
  onNavigate?: (path: string, param?: string) => void;
}

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

/**
 * Reads `?instance=<uuid>` from the current URL. App routes on `location.pathname` only, so
 * this page owns its query string. Non-UUID values are rejected because the id becomes an
 * API path segment (blocks crafted links such as `?instance=../../users`).
 */
function readInstanceParam(): { id: string | null; invalid: boolean } {
  const raw = new URLSearchParams(window.location.search).get('instance');
  if (!raw) return { id: null, invalid: false };
  return UUID_RE.test(raw) ? { id: raw.toLowerCase(), invalid: false } : { id: null, invalid: true };
}

function writeInstanceParam(id: string | null) {
  const params = new URLSearchParams(window.location.search);
  if (id) params.set('instance', id);
  else params.delete('instance');
  const query = params.toString();
  window.history.replaceState(window.history.state, '', `${window.location.pathname}${query ? `?${query}` : ''}`);
}

const statusRank = (status: string) => (status === 'running' ? 0 : status === 'stopped' ? 2 : 1);

export const AdminDocker: React.FC<AdminDockerProps> = () => {
  const [instances, setInstances] = useState<Instance[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);
  const [search, setSearch] = useState('');
  const [selectedId, setSelectedId] = useState<string | null>(() => readInstanceParam().id);
  const [invalidParam, setInvalidParam] = useState<boolean>(() => readInstanceParam().invalid);

  // Back/forward, and in-app navigations that only change the query string, re-sync from the URL.
  useEffect(() => {
    const sync = () => {
      const param = readInstanceParam();
      setSelectedId(param.id);
      setInvalidParam(param.invalid);
    };
    window.addEventListener('popstate', sync);
    return () => window.removeEventListener('popstate', sync);
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    apiRequest<Instance[]>('/admin/instances', { signal: controller.signal })
      .then((data) => {
        if (controller.signal.aborted) return;
        setInstances(Array.isArray(data) ? data : []);
        setLoadError(null);
      })
      .catch((err) => {
        if (controller.signal.aborted || (err as { name?: string })?.name === 'AbortError') return;
        setLoadError(err instanceof Error ? err.message : 'Failed to load instances');
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [reloadKey]);

  const sorted = useMemo(
    () =>
      [...instances].sort(
        (a, b) =>
          statusRank(a.status) - statusRank(b.status) ||
          (a.hostname || a.name || '').localeCompare(b.hostname || b.name || '')
      ),
    [instances]
  );

  const filtered = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return sorted;
    return sorted.filter((i) =>
      [
        i.hostname,
        i.name,
        i.id,
        i.owner?.email,
        i.owner?.display_name,
        i.private_ip,
        i.public_ip,
        i.proxmox_vmid ? String(i.proxmox_vmid) : '',
        i.node?.proxmox_node_name,
      ].some((v) => v && v.toLowerCase().includes(q))
    );
  }, [sorted, search]);

  const selected = selectedId ? instances.find((i) => i.id.toLowerCase() === selectedId) : undefined;

  const selectInstance = (id: string | null) => {
    const normalized = id ? id.toLowerCase() : null;
    setSelectedId(normalized);
    setInvalidParam(false);
    writeInstanceParam(normalized);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight flex items-center gap-2">
            <Container className="w-5 h-5 text-zinc-400" />
            <span>Docker Viewer</span>
          </h1>
          <p className="text-xs sm:text-sm text-zinc-400 mt-0.5">
            Read-only containers and logs inside any tenant VM, via the QEMU guest agent
          </p>
        </div>
      </div>

      {invalidParam && (
        <div className="flex items-center gap-2 p-2.5 rounded-md bg-amber-500/5 border border-amber-500/20 text-[11px] text-amber-300">
          <AlertTriangle className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
          <span>Ignored the instance in the URL: it is not a valid instance ID.</span>
        </div>
      )}

      <div className="grid grid-cols-1 xl:grid-cols-[18rem_minmax(0,1fr)] gap-4 items-start">
        {/* VM selector */}
        <aside className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden xl:sticky xl:top-0">
          <div className="p-3 border-b border-zinc-800 space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-[10px] font-mono uppercase tracking-wider text-zinc-400">
                Virtual machines {instances.length > 0 && <span className="text-zinc-500">({instances.length})</span>}
              </span>
              <button
                type="button"
                onClick={() => setReloadKey((k) => k + 1)}
                disabled={loading}
                title="Reload instances"
                className="p-1 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-50"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
              </button>
            </div>
            <div className="relative">
              <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
              <input
                type="text"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder="Hostname, owner, VMID, IP…"
                className="w-full bg-zinc-900 border border-zinc-800 rounded-md pl-8 pr-3 py-1.5 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
              />
            </div>
          </div>

          <div className="max-h-72 xl:max-h-[calc(100vh-16rem)] overflow-y-auto">
            {loading && instances.length === 0 ? (
              <div className="p-6 text-center text-xs text-zinc-500">Loading instances…</div>
            ) : loadError && instances.length === 0 ? (
              <div className="p-4 text-xs text-rose-400">{loadError}</div>
            ) : filtered.length === 0 ? (
              <div className="p-6 text-center text-xs text-zinc-500">No instances match.</div>
            ) : (
              filtered.map((inst) => {
                const active = inst.id.toLowerCase() === selectedId;
                return (
                  <button
                    key={inst.id}
                    type="button"
                    onClick={() => selectInstance(inst.id)}
                    className={`w-full text-left px-3 py-2.5 border-b border-zinc-800/60 last:border-b-0 transition-colors ${
                      active ? 'bg-zinc-800/60' : 'hover:bg-zinc-900'
                    }`}
                  >
                    <div className="flex items-center justify-between gap-2">
                      <span className={`text-xs truncate ${active ? 'font-semibold text-white' : 'font-medium text-zinc-200'}`}>
                        {inst.hostname || inst.name}
                      </span>
                      <StatusBadge status={inst.status} size="sm" />
                    </div>
                    <div className="mt-0.5 text-[11px] text-zinc-400 truncate">
                      {inst.owner?.email || inst.owner?.display_name || inst.owner_id}
                    </div>
                    <div className="mt-0.5 text-[10px] font-mono text-zinc-500 truncate">
                      VMID {inst.proxmox_vmid || '—'} · {inst.private_ip || inst.public_ip || 'no IP'}
                    </div>
                  </button>
                );
              })
            )}
          </div>
        </aside>

        {/* Viewer */}
        <section className="min-w-0 space-y-3">
          {selectedId ? (
            <>
              <div className="flex flex-wrap items-center justify-between gap-2 px-3 py-2 rounded-lg border border-zinc-800 bg-zinc-900/40">
                <div className="flex items-center gap-2 min-w-0 text-xs">
                  <Server className="w-3.5 h-3.5 text-zinc-400 flex-shrink-0" />
                  {selected ? (
                    <>
                      <span className="font-semibold text-white truncate">{selected.hostname || selected.name}</span>
                      <StatusBadge status={selected.status} size="sm" />
                      <span className="text-[11px] text-zinc-400 truncate">
                        {selected.owner?.email || selected.owner_id}
                      </span>
                    </>
                  ) : (
                    <span className="text-zinc-400 font-mono truncate">
                      {selectedId}
                      {!loading && <span className="text-amber-400"> · not in the instance inventory</span>}
                    </span>
                  )}
                </div>
                <button
                  type="button"
                  onClick={() => selectInstance(null)}
                  title="Clear selection"
                  className="p-1 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
                >
                  <X className="w-3.5 h-3.5" />
                </button>
              </div>
              <DockerViewer key={selectedId} scope="admin" instanceId={selectedId} />
            </>
          ) : (
            <div className="bg-zinc-900/40 border border-dashed border-zinc-800 rounded-lg py-20 px-6 text-center space-y-2">
              <Container className="w-6 h-6 text-zinc-600 mx-auto" />
              <div className="text-xs font-medium text-zinc-300">Select a virtual machine</div>
              <p className="text-[11px] text-zinc-500">
                Running instances are listed first. Docker data is read through the guest agent on demand.
              </p>
            </div>
          )}
        </section>
      </div>
    </div>
  );
};
