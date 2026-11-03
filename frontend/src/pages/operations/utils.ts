import { ApiError } from '../../lib/api';
import type { InfraCheckType, RiskLevel } from '../../types';

export const formatDate = (value?: string | null): string => {
  if (!value) return '—';
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
};

export const formatRelative = (value?: string | null): string => {
  if (!value) return '—';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  const seconds = Math.round((Date.now() - date.getTime()) / 1000);
  if (seconds < 60) return 'just now';
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ago`;
  if (seconds < 86400) return `${Math.floor(seconds / 3600)}h ago`;
  return `${Math.floor(seconds / 86400)}d ago`;
};

export const formatBytes = (bytes: number): string => {
  if (!bytes) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const exponent = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  return `${parseFloat((bytes / 1024 ** exponent).toFixed(2))} ${units[exponent]}`;
};

export const formatMoney = (amount: number, currency = 'USD'): string => {
  try {
    return new Intl.NumberFormat(undefined, { style: 'currency', currency: currency || 'USD' }).format(amount || 0);
  } catch {
    return `${currency || 'USD'} ${(amount || 0).toFixed(2)}`;
  }
};

export const errorMessage = (err: unknown): string => {
  if (err instanceof ApiError) return err.message;
  if (err instanceof Error) return err.message;
  return String(err || 'Request failed');
};

export const statusClasses = (status?: string | null): string => {
  switch ((status || '').toLowerCase()) {
    case 'running':
    case 'active':
    case 'ok':
    case 'healthy':
    case 'completed':
    case 'succeeded':
    case 'verified':
    case 'pass':
      return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
    case 'provisioning':
    case 'starting':
    case 'restarting':
    case 'in_progress':
    case 'queued':
      return 'bg-blue-500/10 text-blue-400 border-blue-500/20';
    case 'warning':
    case 'degraded':
    case 'suspended':
      return 'bg-amber-500/10 text-amber-400 border-amber-500/20';
    case 'critical':
    case 'failed':
    case 'error':
    case 'deleting':
      return 'bg-rose-500/10 text-rose-400 border-rose-500/20';
    case 'stopped':
    case 'unknown':
    case 'not observable':
      return 'bg-zinc-900 text-zinc-400 border-zinc-800';
    default:
      return 'bg-zinc-900 text-zinc-300 border-zinc-800';
  }
};

export const riskClasses = (risk?: string | null): string => {
  switch ((risk || '').toLowerCase() as RiskLevel) {
    case 'read':
      return 'bg-zinc-800 text-zinc-300 border-zinc-700';
    case 'low':
      return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20';
    case 'medium':
      return 'bg-amber-500/10 text-amber-400 border-amber-500/20';
    case 'high':
      return 'bg-orange-500/10 text-orange-400 border-orange-500/20';
    case 'destructive':
      return 'bg-rose-500/10 text-rose-400 border-rose-500/20';
    default:
      return 'bg-zinc-800 text-zinc-300 border-zinc-700';
  }
};

export const resourceTypeLabel = (type: string): string => {
  const labels: Record<string, string> = {
    vps: 'Compute VPS',
    app: 'Web application',
    n8n: 'n8n automation',
    openclaw: 'OpenClaw agent',
    game_server: 'Game server',
    android: 'Android instance',
    database: 'Managed database',
    bucket: 'Storage bucket',
    domain: 'Domain',
    account: 'Account',
  };
  return labels[type] || type;
};

/** Maps a unified-inventory type to the diagnostics/check resource type. */
export const unifiedToResourceType = (type: string): string => {
  switch (type) {
    case 'automation':
      return 'n8n';
    case 'ai_agent':
      return 'openclaw';
    case 'android_instance':
      return 'android';
    default:
      return type;
  }
};

export const riskRank = (risk: string): number => {
  const order = ['read', 'low', 'medium', 'high', 'destructive'];
  const index = order.indexOf(risk);
  return index === -1 ? 1 : index;
};

export const highestRisk = (items: { risk: string }[]): RiskLevel => {
  return items.reduce<RiskLevel>((highest, item) => (riskRank(item.risk) > riskRank(highest) ? (item.risk as RiskLevel) : highest), 'read');
};

export interface CheckTypeMeta {
  value: InfraCheckType;
  label: string;
  description: string;
  resourceTypes: string[];
}

export const CHECK_TYPES: CheckTypeMeta[] = [
  {
    value: 'instance.health',
    label: 'VPS health',
    description: 'Verifies the VM is running and reports its metrics.',
    resourceTypes: ['vps'],
  },
  {
    value: 'app.health',
    label: 'Application health',
    description: 'Requests the application health path.',
    resourceTypes: ['app'],
  },
  {
    value: 'game_server.health',
    label: 'Game server health',
    description: 'Checks the game server status and telemetry.',
    resourceTypes: ['game_server'],
  },
  {
    value: 'android.health',
    label: 'Android instance health',
    description: 'Checks the Waydroid session and telemetry.',
    resourceTypes: ['android'],
  },
  {
    value: 'database.health',
    label: 'Database health',
    description: 'Checks the managed database status and telemetry.',
    resourceTypes: ['database'],
  },
  {
    value: 'backup.freshness',
    label: 'Backup freshness',
    description: 'Warns when the last backup is missing or stale.',
    resourceTypes: ['game_server', 'android', 'database'],
  },
  {
    value: 'quota.usage',
    label: 'Quota usage',
    description: 'Warns when the account approaches a resource quota.',
    resourceTypes: ['account'],
  },
];

export const checkTypeMeta = (value: string): CheckTypeMeta | undefined =>
  CHECK_TYPES.find((item) => item.value === value);
