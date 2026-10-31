import React from 'react';

interface StatsCardProps {
  title: string;
  value: string | number;
  subtext?: string;
  icon: React.ReactNode;
  trend?: string;
  trendPositive?: boolean;
}

export const StatsCard: React.FC<StatsCardProps> = ({
  title,
  value,
  subtext,
  icon,
  trend,
  trendPositive,
}) => {
  return (
    <div className="bg-zinc-900/40 border border-zinc-800/80 rounded-lg p-4 hover:border-zinc-700/70 transition-colors">
      <div className="flex items-center justify-between">
        <span className="text-[11px] font-mono uppercase tracking-wider text-zinc-400">
          {title}
        </span>
        <div className="text-zinc-400">{icon}</div>
      </div>
      <div className="mt-2.5 flex items-baseline gap-2">
        <span className="text-2xl font-semibold text-white tracking-tight">{value}</span>
        {trend && (
          <span
            className={`text-[11px] font-mono ${
              trendPositive ? 'text-emerald-400' : 'text-zinc-400'
            }`}
          >
            {trend}
          </span>
        )}
      </div>
      {subtext && <p className="text-xs text-zinc-400 mt-1">{subtext}</p>}
    </div>
  );
};
