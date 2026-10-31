import React from 'react';
import { InstanceStatus } from '../../types';

interface BadgeProps {
  status: InstanceStatus | string;
  size?: 'sm' | 'md';
}

export const StatusBadge: React.FC<BadgeProps> = ({ status, size = 'md' }) => {
  const normalized = status.toLowerCase();

  let bg = 'bg-zinc-800/60 text-zinc-300 border-zinc-700/60';
  let dotColor = 'bg-zinc-400';

  switch (normalized) {
    case 'running':
    case 'online':
    case 'success':
    case 'completed':
    case 'active':
      bg = 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
      dotColor = 'bg-emerald-500';
      break;
    case 'provisioning':
    case 'starting':
    case 'restarting':
    case 'in_progress':
      bg = 'bg-blue-500/10 text-blue-400 border-blue-500/20';
      dotColor = 'bg-blue-400';
      break;
    case 'queued':
      bg = 'bg-amber-500/10 text-amber-400 border-amber-500/20';
      dotColor = 'bg-amber-400';
      break;
    case 'stopping':
    case 'deleting':
      bg = 'bg-orange-500/10 text-orange-400 border-orange-500/20';
      dotColor = 'bg-orange-400';
      break;
    case 'stopped':
    case 'deleted':
      bg = 'bg-zinc-900 text-zinc-400 border-zinc-800';
      dotColor = 'bg-zinc-500';
      break;
    case 'failed':
    case 'suspended':
    case 'failure':
    case 'offline':
      bg = 'bg-rose-500/10 text-rose-400 border-rose-500/20';
      dotColor = 'bg-rose-400';
      break;
  }

  const sizeClasses = size === 'sm' ? 'px-2 py-0.5 text-[11px]' : 'px-2.5 py-0.5 text-xs';

  return (
    <span
      className={`inline-flex items-center gap-1.5 font-mono rounded-md border ${bg} ${sizeClasses}`}
    >
      <span className={`inline-block rounded-full w-1.5 h-1.5 ${dotColor}`} />
      <span className="capitalize">{normalized.replace('_', ' ')}</span>
    </span>
  );
};
