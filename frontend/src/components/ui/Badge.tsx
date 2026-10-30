import React from 'react';
import { InstanceStatus } from '../../types';

interface BadgeProps {
  status: InstanceStatus | string;
  size?: 'sm' | 'md';
}

export const StatusBadge: React.FC<BadgeProps> = ({ status, size = 'md' }) => {
  const normalized = status.toLowerCase();

  let bg = 'bg-slate-800 text-slate-300 border-slate-700';
  let dotColor = 'bg-slate-400';
  let pulse = false;

  switch (normalized) {
    case 'running':
    case 'online':
    case 'success':
    case 'completed':
    case 'active':
      bg = 'bg-emerald-950/60 text-emerald-300 border-emerald-800/80';
      dotColor = 'bg-emerald-400';
      pulse = true;
      break;
    case 'provisioning':
    case 'starting':
    case 'restarting':
    case 'in_progress':
      bg = 'bg-blue-950/60 text-blue-300 border-blue-800/80';
      dotColor = 'bg-blue-400';
      pulse = true;
      break;
    case 'queued':
      bg = 'bg-amber-950/60 text-amber-300 border-amber-800/80';
      dotColor = 'bg-amber-400';
      break;
    case 'stopping':
    case 'deleting':
      bg = 'bg-orange-950/60 text-orange-300 border-orange-800/80';
      dotColor = 'bg-orange-400';
      pulse = true;
      break;
    case 'stopped':
    case 'deleted':
      bg = 'bg-slate-900 text-slate-400 border-slate-800';
      dotColor = 'bg-slate-500';
      break;
    case 'failed':
    case 'suspended':
    case 'failure':
    case 'offline':
      bg = 'bg-rose-950/60 text-rose-300 border-rose-800/80';
      dotColor = 'bg-rose-400';
      break;
  }

  const sizeClasses = size === 'sm' ? 'px-2 py-0.5 text-xs' : 'px-2.5 py-1 text-xs tracking-wide';

  return (
    <span
      className={`inline-flex items-center gap-1.5 font-medium rounded-full border ${bg} ${sizeClasses}`}
    >
      <span className="relative flex h-2 w-2">
        {pulse && (
          <span
            className={`animate-ping absolute inline-flex h-full w-full rounded-full opacity-75 ${dotColor}`}
          />
        )}
        <span className={`relative inline-flex rounded-full h-2 w-2 ${dotColor}`} />
      </span>
      <span className="capitalize">{normalized.replace('_', ' ')}</span>
    </span>
  );
};
