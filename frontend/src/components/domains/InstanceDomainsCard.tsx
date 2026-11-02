import React, { useEffect, useState } from 'react';
import { Globe, PlusCircle, RefreshCw } from 'lucide-react';
import { Domain } from '../../types';
import { useSystemConfig } from '../../context/SystemConfigContext';
import { DomainLink, LiveStatusBadge } from './DomainBits';
import {
  describeDomainError,
  fetchDomains,
  isAbortError,
  isDomainsDisabledError,
  liveStatusInfo,
  sameId,
} from '../../lib/domains';

interface InstanceDomainsCardProps {
  instanceId: string;
  /** Re-fetch when the VM's status or IP changes, since a domain's live status follows them. */
  instanceStatus: string;
  instanceIp?: string | null;
  onNavigate: (path: string, param?: string) => void;
}

const InstanceDomainsCardContent: React.FC<InstanceDomainsCardProps & { root: string }> = ({
  instanceId,
  instanceStatus,
  instanceIp,
  onNavigate,
  root,
}) => {
  const [domains, setDomains] = useState<Domain[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [hidden, setHidden] = useState(false);
  const [loading, setLoading] = useState(true);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    fetchDomains(controller.signal)
      .then((list) => {
        if (controller.signal.aborted) return;
        setDomains(list.filter((d) => sameId(d.instance_id, instanceId)));
        setError(null);
      })
      .catch((err) => {
        if (controller.signal.aborted || isAbortError(err)) return;
        if (isDomainsDisabledError(err)) setHidden(true);
        else setError(describeDomainError(err).message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [instanceId, instanceStatus, instanceIp, reloadKey]);

  if (hidden) return null;

  const addDomain = () => onNavigate(`/dashboard/domains?instance=${encodeURIComponent(instanceId)}`);

  return (
    <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
      <div className="flex items-center justify-between gap-2 mb-3">
        <h3 className="text-xs font-semibold text-white flex items-center gap-2">
          <Globe className="w-3.5 h-3.5 text-zinc-400" />
          <span>Public domains</span>
          {domains && domains.length > 0 && (
            <span className="text-[10px] font-mono text-zinc-500">{domains.length}</span>
          )}
        </h3>
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => setReloadKey((k) => k + 1)}
            disabled={loading}
            title="Refresh domains"
            className="p-1 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-50"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            type="button"
            onClick={addDomain}
            className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-900 border border-zinc-800 text-[11px] font-medium text-zinc-300 hover:text-white hover:border-zinc-700 transition-colors"
          >
            <PlusCircle className="w-3 h-3" />
            <span>Add domain</span>
          </button>
        </div>
      </div>

      {error && !domains ? (
        <p className="text-[11px] text-rose-300 break-words">Could not load domains: {error}</p>
      ) : !domains ? (
        <p className="text-[11px] text-zinc-500 font-mono">Loading…</p>
      ) : domains.length === 0 ? (
        <p className="text-[11px] text-zinc-500 leading-relaxed">
          No public URL yet. Publish a port of this VM at{' '}
          <span className="font-mono text-zinc-300">https://name.{root}</span>.
        </p>
      ) : (
        <>
          <ul className="space-y-2">
            {domains.map((d) => {
              const info = liveStatusInfo(d);
              return (
                <li key={d.id} className="p-2.5 rounded-md bg-zinc-950/60 border border-zinc-800">
                  <div className="flex items-center justify-between gap-2 min-w-0">
                    <DomainLink domain={d} />
                    <LiveStatusBadge domain={d} />
                  </div>
                  <div className="mt-1 text-[11px] text-zinc-500 truncate" title={info.fix || info.explanation}>
                    <span className="font-mono">port {d.target_port}</span>
                    {d.live_status !== 'live' && (info.fix || info.explanation) && (
                      <span> · {info.fix || info.explanation}</span>
                    )}
                  </div>
                </li>
              );
            })}
          </ul>
          <button
            type="button"
            onClick={() => onNavigate('/dashboard/domains')}
            className="mt-3 text-[11px] font-medium text-zinc-400 hover:text-white hover:underline"
          >
            Manage domains
          </button>
        </>
      )}
    </div>
  );
};

/**
 * Compact "Public domains" block for a VM's overview. Renders nothing (and fetches nothing)
 * unless the backend config enables Domains. Memoized against InstanceDetails' 5 s polling.
 */
export const InstanceDomainsCard = React.memo<InstanceDomainsCardProps>((props) => {
  const { config, status } = useSystemConfig();
  if (status !== 'ready' || !config.domains.enabled) return null;
  return <InstanceDomainsCardContent {...props} root={config.domains.root} />;
});
InstanceDomainsCard.displayName = 'InstanceDomainsCard';
