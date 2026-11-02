import React, { useMemo } from 'react';
import { ArrowRight, Minus, Plus, RefreshCw } from 'lucide-react';
import { DiffEntry, formatDiffValue } from '../../lib/orchestrator';

const KIND_STYLE: Record<DiffEntry['kind'], { icon: React.ComponentType<{ className?: string }>; badge: string; label: string }> = {
  added: { icon: Plus, badge: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20', label: 'added' },
  removed: { icon: Minus, badge: 'bg-rose-500/10 text-rose-400 border-rose-500/20', label: 'removed' },
  changed: { icon: RefreshCw, badge: 'bg-amber-500/10 text-amber-400 border-amber-500/20', label: 'changed' },
};

/** Shortens `specification.resources[web-01].config.memory_mb` to `resources[web-01].config.memory_mb`. */
const displayPath = (path: string) => path.replace(/^specification\./, '');

/** Field-level diff entries (from lib/orchestrator diffJson). */
export const DiffList: React.FC<{
  entries: DiffEntry[];
  fromLabel?: string;
  toLabel?: string;
  emptyText?: string;
  maxItems?: number;
}> = ({ entries, fromLabel = 'before', toLabel = 'after', emptyText = 'No differences.', maxItems = 60 }) => {
  const shown = useMemo(() => entries.slice(0, maxItems), [entries, maxItems]);
  if (entries.length === 0) {
    return <div className="text-[11px] text-zinc-500 italic px-1 py-2">{emptyText}</div>;
  }
  return (
    <div className="space-y-1.5" data-testid="diff-list">
      <div className="flex items-center gap-1.5 text-[10px] font-mono text-zinc-500 px-0.5">
        <span>{entries.length} change{entries.length === 1 ? '' : 's'}</span>
        <span className="text-zinc-700">·</span>
        <span className="truncate">
          {fromLabel} <ArrowRight className="inline w-2.5 h-2.5 -mt-px" /> {toLabel}
        </span>
      </div>
      <ul className="space-y-1">
        {shown.map((entry) => {
          const style = KIND_STYLE[entry.kind];
          const Icon = style.icon;
          return (
            <li
              key={`${entry.kind}:${entry.path}`}
              className="rounded-md border border-zinc-800 bg-zinc-950/60 px-2 py-1.5 space-y-1"
              data-diff-kind={entry.kind}
              data-diff-path={entry.path}
            >
              <div className="flex items-start gap-1.5 min-w-0">
                <span
                  className={`inline-flex items-center gap-1 px-1.5 py-px rounded border font-mono text-[10px] flex-shrink-0 ${style.badge}`}
                >
                  <Icon className="w-2.5 h-2.5" />
                  {style.label}
                </span>
                <code className="font-mono text-[11px] text-zinc-200 break-all min-w-0">{displayPath(entry.path)}</code>
              </div>
              <div className="font-mono text-[10.5px] leading-relaxed break-all">
                {entry.kind !== 'added' && (
                  <div className="text-rose-300/90">
                    <span className="select-none text-rose-500/70">- </span>
                    {formatDiffValue(entry.before)}
                  </div>
                )}
                {entry.kind !== 'removed' && (
                  <div className="text-emerald-300/90">
                    <span className="select-none text-emerald-500/70">+ </span>
                    {formatDiffValue(entry.after)}
                  </div>
                )}
              </div>
            </li>
          );
        })}
      </ul>
      {entries.length > shown.length && (
        <div className="text-[10px] text-zinc-500 px-0.5">…and {entries.length - shown.length} more</div>
      )}
    </div>
  );
};
