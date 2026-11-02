import React, { useEffect, useState } from 'react';
import { AlertTriangle, Check, CircleAlert, Copy } from 'lucide-react';
import { copyText } from '../../lib/clipboard';
import { DockerIssue, formatAgo } from './dockerUtils';

/** Icon button that copies `text` (or the result of calling it) and briefly confirms. */
export const CopyButton: React.FC<{
  text: string | (() => string);
  title?: string;
  label?: string;
  disabled?: boolean;
  className?: string;
}> = ({ text, title = 'Copy to clipboard', label, disabled, className = '' }) => {
  const [state, setState] = useState<'idle' | 'copied' | 'failed'>('idle');

  useEffect(() => {
    if (state === 'idle') return;
    const timer = setTimeout(() => setState('idle'), 1600);
    return () => clearTimeout(timer);
  }, [state]);

  const handleClick = async () => {
    const value = typeof text === 'function' ? text() : text;
    setState((await copyText(value)) ? 'copied' : 'failed');
  };

  return (
    <button
      type="button"
      onClick={handleClick}
      disabled={disabled}
      title={state === 'failed' ? 'Copy failed: select the text manually' : title}
      className={`inline-flex items-center gap-1.5 p-1.5 rounded-md text-[11px] font-medium text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-40 disabled:pointer-events-none ${className}`}
    >
      {state === 'copied' ? (
        <Check className="w-3.5 h-3.5 text-emerald-400" />
      ) : state === 'failed' ? (
        <CircleAlert className="w-3.5 h-3.5 text-rose-400" />
      ) : (
        <Copy className="w-3.5 h-3.5" />
      )}
      {label && <span>{state === 'copied' ? 'Copied' : state === 'failed' ? 'Failed' : label}</span>}
    </button>
  );
};

/** Shell command with a copy button. */
export const CommandBlock: React.FC<{ cmd: string; label?: string }> = ({ cmd, label }) => (
  <div>
    {label && <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500 mb-1">{label}</div>}
    <div className="flex items-start gap-2 bg-zinc-950 border border-zinc-800 rounded-md pl-3 pr-1 py-1">
      <span className="text-zinc-600 font-mono text-[11px] select-none pt-1">$</span>
      <code className="flex-1 min-w-0 py-1 font-mono text-[11px] text-zinc-200 break-all">{cmd}</code>
      <CopyButton text={cmd} title="Copy command" />
    </div>
  </div>
);

const TONE_STYLES: Record<DockerIssue['tone'], { box: string; icon: string; title: string }> = {
  amber: {
    box: 'bg-amber-500/5 border-amber-500/20',
    icon: 'text-amber-400',
    title: 'text-amber-300',
  },
  rose: {
    box: 'bg-rose-500/5 border-rose-500/20',
    icon: 'text-rose-400',
    title: 'text-rose-300',
  },
};

/** Explains why Docker data is unavailable and how to fix it. */
export const DockerIssuePanel: React.FC<{
  issue: DockerIssue;
  detail?: string;
  code?: string;
  action?: React.ReactNode;
  compact?: boolean;
}> = ({ issue, detail, code, action, compact = false }) => {
  const tone = TONE_STYLES[issue.tone];
  return (
    <div className={`border rounded-lg ${tone.box} ${compact ? 'p-3' : 'p-4'}`}>
      <div className="flex items-start gap-3">
        <AlertTriangle className={`w-4 h-4 mt-0.5 flex-shrink-0 ${tone.icon}`} />
        <div className="flex-1 min-w-0 space-y-2.5">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div className="min-w-0">
              <div className={`text-xs font-semibold ${tone.title}`}>{issue.title}</div>
              <p className="text-xs text-zinc-400 mt-0.5 leading-relaxed">{issue.description}</p>
            </div>
            {action}
          </div>

          {issue.commands && issue.commands.length > 0 && (
            <div className="space-y-2">
              {issue.commands.map((c) => (
                <CommandBlock key={c.cmd} cmd={c.cmd} label={c.label} />
              ))}
            </div>
          )}

          {issue.notes?.map((note) => (
            <p key={note} className="text-[11px] text-zinc-400 leading-relaxed">
              {note}
            </p>
          ))}

          {(detail || code) && (
            <p className="text-[11px] font-mono text-zinc-500 break-words">
              {code && <span className="text-zinc-400">{code}</span>}
              {code && detail && <span> · </span>}
              {detail}
            </p>
          )}
        </div>
      </div>
    </div>
  );
};

/** "12s ago" label that re-renders itself once per second (and nothing else). */
export const Ago: React.FC<{ at: number | null; title?: string }> = ({ at, title }) => {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (at === null) return;
    setNow(Date.now());
    const timer = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(timer);
  }, [at]);

  if (at === null) return <span>never</span>;
  return <span title={title}>{formatAgo(at, now)}</span>;
};

/** Small pill toggle used in Docker toolbars. */
export const ToggleChip: React.FC<{
  active: boolean;
  onClick: () => void;
  icon?: React.ComponentType<{ className?: string }>;
  label: string;
  title?: string;
  indicator?: boolean;
}> = ({ active, onClick, icon: Icon, label, title, indicator }) => (
  <button
    type="button"
    onClick={onClick}
    aria-pressed={active}
    title={title}
    className={`inline-flex items-center gap-1.5 px-2 py-1 rounded-md border text-[11px] font-medium transition-colors ${
      active
        ? 'bg-zinc-800 border-zinc-700 text-white'
        : 'bg-zinc-900 border-zinc-800 text-zinc-400 hover:text-zinc-200'
    }`}
  >
    {indicator && active ? (
      <span className="w-1.5 h-1.5 rounded-full bg-emerald-500 animate-pulse" />
    ) : (
      Icon && <Icon className="w-3 h-3" />
    )}
    <span>{label}</span>
  </button>
);
