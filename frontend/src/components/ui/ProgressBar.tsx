import React from 'react';

interface ProgressBarProps {
  label: string;
  used: number;
  total: number;
  unit?: string;
  showPercent?: boolean;
}

export const ProgressBar: React.FC<ProgressBarProps> = ({
  label,
  used,
  total,
  unit = '',
  showPercent = true,
}) => {
  const percent = total > 0 ? Math.min(100, Math.round((used / total) * 100)) : 0;

  let color = 'bg-zinc-200';
  if (percent >= 90) {
    color = 'bg-rose-500';
  } else if (percent >= 75) {
    color = 'bg-amber-500';
  }

  const tbDisplay = unit === 'GB' && total >= 1000 ? (total >= 2400 ? '2.5 TB' : `${(total / 1000).toFixed(1)} TB`) : null;

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-xs">
        <span className="font-medium text-zinc-300">{label}</span>
        <span className="text-zinc-400 font-mono text-[11px]">
          <strong className="text-zinc-100 font-semibold">{used}</strong> / {total} {unit}
          {tbDisplay && <span className="text-zinc-500 ml-1">({tbDisplay})</span>}
          {showPercent && <span className="ml-1 text-zinc-500">({percent}%)</span>}
        </span>
      </div>
      <div className="h-1.5 w-full bg-zinc-800 rounded-full overflow-hidden">
        <div
          className={`h-full ${color} transition-all duration-300 rounded-full`}
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  );
};
