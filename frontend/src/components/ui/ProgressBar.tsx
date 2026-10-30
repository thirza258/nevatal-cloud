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

  let color = 'bg-blue-500';
  if (percent >= 90) {
    color = 'bg-rose-500';
  } else if (percent >= 75) {
    color = 'bg-amber-500';
  }

  return (
    <div className="space-y-1.5">
      <div className="flex items-center justify-between text-xs">
        <span className="font-medium text-slate-300">{label}</span>
        <span className="text-slate-400">
          <strong className="text-white font-semibold">{used}</strong> / {total} {unit}
          {showPercent && <span className="ml-1 text-slate-500">({percent}%)</span>}
        </span>
      </div>
      <div className="h-2 w-full bg-slate-800 rounded-full overflow-hidden">
        <div
          className={`h-full ${color} transition-all duration-500 rounded-full`}
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  );
};
