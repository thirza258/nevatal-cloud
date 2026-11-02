import React from 'react';
import {
  ArrowLeftRight,
  Clock,
  ExternalLink,
  Globe,
  LifeBuoy,
  Lock,
  Network,
  Timer,
  Upload,
  Wrench,
} from 'lucide-react';
import { Domain } from '../../types';
import { useSystemConfig } from '../../context/SystemConfigContext';
import { PageFallback } from '../layout/RouteBoundary';
import { CommandBlock } from '../docker/DockerBits';
import { LiveStatusInfo, StatusTone, domainLabel, domainUrl, liveStatusInfo } from '../../lib/domains';

export interface DomainsFeature {
  root: string;
  maxPerUser: number;
}

const TONES: Record<StatusTone, { badge: string; dot: string; text: string }> = {
  emerald: { badge: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20', dot: 'bg-emerald-500', text: 'text-emerald-300' },
  amber: { badge: 'bg-amber-500/10 text-amber-400 border-amber-500/20', dot: 'bg-amber-400', text: 'text-amber-300' },
  rose: { badge: 'bg-rose-500/10 text-rose-400 border-rose-500/20', dot: 'bg-rose-400', text: 'text-rose-300' },
  zinc: { badge: 'bg-zinc-800/60 text-zinc-300 border-zinc-700/60', dot: 'bg-zinc-500', text: 'text-zinc-300' },
};

/** Neutral panel shown instead of any Domains UI when the feature is off. */
export const DomainsUnavailable: React.FC<{ turnedOff?: boolean }> = ({ turnedOff = false }) => (
  <div className="bg-zinc-900/40 border border-dashed border-zinc-800 rounded-lg py-16 px-6 text-center space-y-2">
    <Globe className="w-6 h-6 text-zinc-600 mx-auto" />
    <div className="text-xs font-medium text-zinc-300">Public domains are not available</div>
    <p className="text-[11px] text-zinc-500">
      {turnedOff
        ? 'Public domains were turned off on this deployment. Existing domains are not reachable right now.'
        : 'This deployment does not offer public domains.'}
    </p>
  </div>
);

/**
 * Renders `children` only when the backend config enables Domains. Nothing below the gate
 * mounts (so nothing calls /api/v1/domains*) while the config is loading, failed or disabled.
 */
export const DomainsGate: React.FC<{ children: (feature: DomainsFeature) => React.ReactNode }> = ({ children }) => {
  const { config, status } = useSystemConfig();
  if (status === 'loading') return <PageFallback />;
  if (!config.domains.enabled) return <DomainsUnavailable />;
  return <>{children({ root: config.domains.root, maxPerUser: config.domains.max_per_user })}</>;
};

type StatusFields = Pick<Domain, 'live_status' | 'admin_disabled' | 'disabled_reason' | 'target_port'>;

export const LiveStatusBadge: React.FC<{ domain: StatusFields; scope?: 'customer' | 'admin' }> = ({
  domain,
  scope = 'customer',
}) => {
  const info = liveStatusInfo(domain, scope);
  const tone = TONES[info.tone];
  return (
    <span
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border font-mono text-[11px] whitespace-nowrap ${tone.badge}`}
    >
      <span className={`inline-block w-1.5 h-1.5 rounded-full ${tone.dot} ${info.tone === 'emerald' ? 'animate-pulse' : ''}`} />
      <span>{info.label}</span>
    </span>
  );
};

const fixIconFor = (domain: StatusFields, info: LiveStatusInfo) => {
  if (domain.live_status === 'no_ip') return Clock;
  if (info.fix === 'Contact support') return LifeBuoy;
  return Wrench;
};

/** One-line explanation of a domain's live status plus the fix hint (a button when `onFix` is set). */
export const LiveStatusNote: React.FC<{
  domain: StatusFields;
  scope?: 'customer' | 'admin';
  onFix?: () => void;
  className?: string;
}> = ({ domain, scope = 'customer', onFix, className = '' }) => {
  const info = liveStatusInfo(domain, scope);
  const tone = TONES[info.tone];
  const FixIcon = fixIconFor(domain, info);
  return (
    <div className={`text-[11px] leading-snug space-y-0.5 ${className}`}>
      <p className="text-zinc-400 break-words">{info.explanation}</p>
      {info.fix &&
        (onFix ? (
          <button
            type="button"
            onClick={onFix}
            className={`flex items-start gap-1 text-left font-medium hover:underline ${tone.text}`}
          >
            <FixIcon className="w-3 h-3 mt-px flex-shrink-0" />
            <span>{info.fix}</span>
          </button>
        ) : (
          <p className={`flex items-start gap-1 font-medium ${tone.text}`}>
            <FixIcon className="w-3 h-3 mt-px flex-shrink-0" />
            <span>{info.fix}</span>
          </p>
        ))}
    </div>
  );
};

/** Hostname as an external https link (plain text if the hostname is not a well-formed DNS name). */
export const DomainLink: React.FC<{ domain: Pick<Domain, 'hostname' | 'subdomain'>; className?: string }> = ({
  domain,
  className = '',
}) => {
  const label = domainLabel(domain);
  const url = domainUrl(domain.hostname);
  if (!url) {
    return <span className={`font-mono text-xs text-zinc-200 truncate ${className}`}>{label}</span>;
  }
  return (
    <a
      href={url}
      target="_blank"
      rel="noopener noreferrer"
      title={`Open ${url} in a new tab`}
      className={`group inline-flex items-center gap-1 min-w-0 font-mono text-xs font-semibold text-white hover:underline ${className}`}
    >
      <span className="truncate">{label}</span>
      <ExternalLink className="w-3 h-3 flex-shrink-0 text-zinc-500 group-hover:text-zinc-300" />
    </a>
  );
};

/** Accessible on/off switch. */
export const EnabledSwitch: React.FC<{
  checked: boolean;
  onChange: (next: boolean) => void;
  label: string;
  disabled?: boolean;
  pending?: boolean;
  title?: string;
}> = ({ checked, onChange, label, disabled = false, pending = false, title }) => (
  <button
    type="button"
    role="switch"
    aria-checked={checked}
    aria-label={label}
    aria-busy={pending || undefined}
    title={title}
    disabled={disabled || pending}
    onClick={() => onChange(!checked)}
    className={`relative inline-flex h-[18px] w-8 flex-shrink-0 items-center rounded-full border transition-colors focus:outline-none focus-visible:ring-2 focus-visible:ring-zinc-500 disabled:cursor-not-allowed ${
      checked ? 'bg-emerald-500/80 border-emerald-500' : 'bg-zinc-800 border-zinc-700'
    } ${pending ? 'opacity-60 cursor-wait' : disabled ? 'opacity-40' : ''}`}
  >
    <span
      className={`absolute left-0.5 h-3.5 w-3.5 rounded-full bg-white shadow transition-transform ${
        checked ? 'translate-x-3.5' : 'translate-x-0'
      }`}
    />
  </button>
);

/** What the VM-side app needs to get right, shown next to the domain list and in the create form. */
export const DomainsHelp: React.FC<{ port?: number | null }> = ({ port }) => {
  const shownPort = port ? String(port) : '<port>';
  const items: Array<{ icon: React.ComponentType<{ className?: string }>; title: string; body: React.ReactNode }> = [
    {
      icon: Network,
      title: `Listen on 0.0.0.0:${shownPort}`,
      body: (
        <>
          Bind the app to <code className="font-mono text-zinc-300">0.0.0.0</code> inside the VM, not{' '}
          <code className="font-mono text-zinc-300">127.0.0.1</code>, or the proxy cannot reach it. Most dev servers take a
          flag such as <code className="font-mono text-zinc-300">--host 0.0.0.0</code>.
        </>
      ),
    },
    {
      icon: Lock,
      title: 'HTTPS is handled for you',
      body: (
        <>
          Cloudflare terminates TLS. Your app receives plain HTTP with{' '}
          <code className="font-mono text-zinc-300">X-Forwarded-Proto: https</code>, so don&apos;t redirect it to HTTPS
          again.
        </>
      ),
    },
    { icon: ArrowLeftRight, title: 'WebSockets work', body: 'Upgrade requests are passed through to your app.' },
    { icon: Upload, title: 'Uploads up to 100 MB', body: 'Cloudflare rejects request bodies above 100 MB.' },
    { icon: Timer, title: 'Live in seconds', body: 'A new or re-pointed domain can take a few seconds to go live.' },
  ];

  return (
    <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-4 space-y-3">
      <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-400">Before you publish</div>
      <ul className="space-y-3">
        {items.map(({ icon: Icon, title, body }) => (
          <li key={title} className="flex items-start gap-2.5">
            <Icon className="w-3.5 h-3.5 mt-0.5 flex-shrink-0 text-zinc-500" />
            <div className="min-w-0">
              <div className="text-xs font-medium text-zinc-200 font-mono">{title}</div>
              <p className="text-[11px] text-zinc-400 leading-relaxed mt-0.5">{body}</p>
            </div>
          </li>
        ))}
      </ul>
      <div className="pt-3 border-t border-zinc-800 space-y-1.5">
        <CommandBlock
          label="Check what is listening (inside the VM)"
          cmd={`sudo ss -tlnp | grep ':${port || 3000} '`}
        />
        <p className="text-[11px] text-zinc-500 leading-relaxed">
          <span className="font-mono text-zinc-400">0.0.0.0:{port || 3000}</span> or{' '}
          <span className="font-mono text-zinc-400">*:{port || 3000}</span> is reachable;{' '}
          <span className="font-mono text-zinc-400">127.0.0.1:{port || 3000}</span> is local-only.
        </p>
      </div>
    </div>
  );
};
