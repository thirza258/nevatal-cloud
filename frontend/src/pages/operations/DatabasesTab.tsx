import React, { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  ArrowUpRight,
  Check,
  Copy,
  Database,
  Eye,
  HardDrive,
  KeyRound,
  Play,
  Plus,
  RefreshCw,
  RotateCcw,
  Square,
  Stethoscope,
  Trash2,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { Modal } from '../../components/ui/Modal';
import type {
  CreateDatabaseInput,
  CreateDatabaseResult,
  DatabaseBackup,
  DatabaseCredentials,
  DatabaseMetrics,
  DatabasePlan,
  ManagedDatabase,
} from '../../types';
import {
  errorMessage,
  formatBytes,
  formatDate,
  formatRelative,
  statusClasses,
} from './utils';

interface DatabasesTabProps {
  onNavigate: (path: string, param?: string) => void;
  onDiagnose: (resourceType: string, resourceID: string, name: string) => void;
}

const ENGINE_LABELS: Record<string, string> = {
  postgresql: 'PostgreSQL',
  mysql: 'MySQL',
  redis: 'Redis',
  mongodb: 'MongoDB',
};

const DEFAULT_FORM = {
  name: '',
  engine: 'postgresql',
  engine_version: '',
  plan_name: 'starter',
  database_name: '',
  auto_backup: true,
};

export const DatabasesTab: React.FC<DatabasesTabProps> = ({ onDiagnose }) => {
  const [databases, setDatabases] = useState<ManagedDatabase[]>([]);
  const [plans, setPlans] = useState<DatabasePlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const [showCreate, setShowCreate] = useState(false);
  const [form, setForm] = useState({ ...DEFAULT_FORM });
  const [creating, setCreating] = useState(false);
  const [created, setCreated] = useState<CreateDatabaseResult | null>(null);

  const [credentialsFor, setCredentialsFor] = useState<ManagedDatabase | null>(null);
  const [credentials, setCredentials] = useState<DatabaseCredentials | null>(null);
  const [credentialsLoading, setCredentialsLoading] = useState(false);

  const [backupsFor, setBackupsFor] = useState<ManagedDatabase | null>(null);
  const [backups, setBackups] = useState<DatabaseBackup[]>([]);
  const [backupsLoading, setBackupsLoading] = useState(false);
  const [backupName, setBackupName] = useState('');

  const [inspectFor, setInspectFor] = useState<ManagedDatabase | null>(null);
  const [metrics, setMetrics] = useState<DatabaseMetrics | null>(null);
  const [logs, setLogs] = useState<string[]>([]);
  const [inspectLoading, setInspectLoading] = useState(false);

  const [deleteFor, setDeleteFor] = useState<ManagedDatabase | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState('');
  const [copied, setCopied] = useState<string | null>(null);

  const load = async (quiet = false) => {
    if (!quiet) setLoading(true);
    setError(null);
    try {
      const [dbs, sizing] = await Promise.all([
        apiRequest<ManagedDatabase[]>('/databases'),
        apiRequest<DatabasePlan[]>('/databases/plans'),
      ]);
      setDatabases(dbs || []);
      setPlans(sizing || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const running = useMemo(() => databases.filter((db) => db.status === 'running').length, [databases]);
  const engines = useMemo(() => new Set(databases.map((db) => db.engine)).size, [databases]);

  const showNotice = (message: string) => {
    setNotice(message);
    window.setTimeout(() => setNotice(null), 4000);
  };

  const copy = async (label: string, value: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(label);
      window.setTimeout(() => setCopied(null), 1500);
    } catch {
      showNotice('Clipboard access was denied.');
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!form.name.trim()) return;
    setCreating(true);
    setError(null);
    try {
      const payload: CreateDatabaseInput = {
        name: form.name.trim().toLowerCase(),
        engine: form.engine,
        engine_version: form.engine_version.trim() || undefined,
        plan_name: form.plan_name,
        database_name: form.database_name.trim() || undefined,
        auto_backup: form.auto_backup,
      };
      const res = await apiRequest<CreateDatabaseResult>('/databases', {
        method: 'POST',
        body: JSON.stringify(payload),
      });
      setCreated(res);
      setShowCreate(false);
      setForm({ ...DEFAULT_FORM });
      await load(true);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setCreating(false);
    }
  };

  const power = async (db: ManagedDatabase, action: 'start' | 'stop' | 'restart') => {
    setBusy(`power:${db.id}`);
    setError(null);
    try {
      await apiRequest(`/databases/${db.id}/power/${action}`, { method: 'POST' });
      showNotice(`${db.name}: ${action} requested.`);
      await load(true);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const remove = async () => {
    if (!deleteFor) return;
    setBusy(`delete:${deleteFor.id}`);
    setError(null);
    try {
      await apiRequest(`/databases/${deleteFor.id}`, { method: 'DELETE' });
      showNotice(`${deleteFor.name} deleted.`);
      setDeleteFor(null);
      setDeleteConfirm('');
      await load(true);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const openCredentials = async (db: ManagedDatabase) => {
    setCredentialsFor(db);
    setCredentials(null);
    setCredentialsLoading(true);
    setError(null);
    try {
      const res = await apiRequest<DatabaseCredentials>(`/databases/${db.id}/credentials`);
      setCredentials(res);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setCredentialsLoading(false);
    }
  };

  const openBackups = async (db: ManagedDatabase) => {
    setBackupsFor(db);
    setBackups([]);
    setBackupsLoading(true);
    setError(null);
    try {
      const res = await apiRequest<DatabaseBackup[]>(`/databases/${db.id}/backups`);
      setBackups(res || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBackupsLoading(false);
    }
  };

  const createBackup = async () => {
    if (!backupsFor) return;
    setBackupsLoading(true);
    setError(null);
    try {
      await apiRequest(`/databases/${backupsFor.id}/backups`, {
        method: 'POST',
        body: JSON.stringify({ name: backupName.trim() }),
      });
      setBackupName('');
      const res = await apiRequest<DatabaseBackup[]>(`/databases/${backupsFor.id}/backups`);
      setBackups(res || []);
      showNotice('Backup completed.');
      await load(true);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBackupsLoading(false);
    }
  };

  const restoreBackup = async (backup: DatabaseBackup) => {
    if (!backupsFor) return;
    if (!confirm(`Restore backup "${backup.name}"? Current data will be replaced.`)) return;
    setBackupsLoading(true);
    setError(null);
    try {
      await apiRequest(`/databases/${backupsFor.id}/backups/${backup.id}/restore`, { method: 'POST' });
      showNotice('Backup restored.');
      await load(true);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBackupsLoading(false);
    }
  };

  const deleteBackup = async (backup: DatabaseBackup) => {
    if (!backupsFor) return;
    if (!confirm(`Delete backup "${backup.name}"?`)) return;
    setBackupsLoading(true);
    setError(null);
    try {
      await apiRequest(`/databases/${backupsFor.id}/backups/${backup.id}`, { method: 'DELETE' });
      const res = await apiRequest<DatabaseBackup[]>(`/databases/${backupsFor.id}/backups`);
      setBackups(res || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBackupsLoading(false);
    }
  };

  const openInspect = async (db: ManagedDatabase) => {
    setInspectFor(db);
    setMetrics(null);
    setLogs([]);
    setInspectLoading(true);
    setError(null);
    try {
      const [m, l] = await Promise.all([
        apiRequest<DatabaseMetrics>(`/databases/${db.id}/metrics`),
        apiRequest<{ logs: string[] }>(`/databases/${db.id}/logs?lines=120`),
      ]);
      setMetrics(m);
      setLogs(l.logs || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setInspectLoading(false);
    }
  };

  const usageBar = (label: string, value: number) => (
    <div>
      <div className="flex items-center justify-between text-[10px] font-mono text-zinc-400">
        <span>{label}</span>
        <span>{value.toFixed(1)}%</span>
      </div>
      <div className="mt-1 h-1 rounded-full bg-zinc-800 overflow-hidden">
        <div
          className={`h-full rounded-full ${value >= 90 ? 'bg-rose-500' : value >= 75 ? 'bg-amber-500' : 'bg-emerald-500'}`}
          style={{ width: `${Math.max(0, Math.min(100, value))}%` }}
        />
      </div>
    </div>
  );

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-white">Managed databases</h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Encrypted credentials, logical backups and health telemetry. Engine provisioning remains subject to plan approval.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={() => load()}
            disabled={loading}
            className="p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 transition"
            title="Refresh databases"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold transition"
          >
            <Plus className="w-3.5 h-3.5" />
            New database
          </button>
        </div>
      </div>

      <div className="grid grid-cols-3 gap-3">
        {[
          { label: 'Databases', value: databases.length, icon: Database },
          { label: 'Running', value: running, icon: Play },
          { label: 'Engines', value: engines, icon: HardDrive },
        ].map(({ label, value, icon: Icon }) => (
          <div key={label} className="p-3 rounded-lg bg-zinc-900/40 border border-zinc-800">
            <div className="flex items-center gap-1.5 text-[10px] uppercase font-mono text-zinc-400">
              <Icon className="w-3 h-3" />
              {label}
            </div>
            <div className="text-xl font-semibold text-white mt-1">{value}</div>
          </div>
        ))}
      </div>

      {error && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
          <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
      )}
      {notice && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
          <Check className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{notice}</span>
        </div>
      )}

      {loading ? (
        <div className="p-12 text-center text-zinc-500 flex flex-col items-center gap-2 text-xs">
          <RefreshCw className="w-5 h-5 animate-spin" />
          Loading managed databases…
        </div>
      ) : databases.length === 0 ? (
        <div className="text-center py-14 px-4 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20">
          <Database className="w-10 h-10 text-zinc-600 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-zinc-200">No managed databases yet</h3>
          <p className="text-xs text-zinc-400 max-w-md mx-auto mt-1 mb-5">
            Create a PostgreSQL, MySQL, Redis or MongoDB database. Credentials are generated once and stored encrypted.
          </p>
          <button
            onClick={() => setShowCreate(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold"
          >
            <Plus className="w-3.5 h-3.5" />
            Create database
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          {databases.map((db) => (
            <div key={db.id} className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800 space-y-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2">
                    <Database className="w-4 h-4 text-cyan-400 flex-shrink-0" />
                    <h3 className="text-sm font-semibold text-white truncate">{db.name}</h3>
                    <span className={`px-2 py-0.5 rounded-md border text-[10px] font-mono ${statusClasses(db.status)}`}>
                      {db.status}
                    </span>
                  </div>
                  <div className="text-[11px] text-zinc-400 font-mono mt-1">
                    {ENGINE_LABELS[db.engine] || db.engine} {db.engine_version} · {db.plan_name} · {db.vcpus} vCPU / {db.memory_mb} MB / {db.storage_gb} GB
                  </div>
                </div>
                <button
                  onClick={() => onDiagnose('database', db.id, db.name)}
                  className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition"
                  title="Run diagnostics"
                >
                  <Stethoscope className="w-3 h-3" />
                  Diagnose
                </button>
              </div>

              <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[11px] font-mono text-zinc-400">
                <div className="truncate">host <span className="text-zinc-200">{db.host}</span></div>
                <div>port <span className="text-zinc-200">{db.port}</span></div>
                <div className="truncate">db <span className="text-zinc-200">{db.database_name}</span></div>
                <div className="truncate">user <span className="text-zinc-200">{db.username}</span></div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
                {usageBar('CPU', db.cpu_percent || 0)}
                {usageBar('Memory', db.memory_usage_percent || 0)}
                {usageBar('Storage', db.storage_usage_percent || 0)}
              </div>

              <div className="flex items-center justify-between text-[10px] text-zinc-500 font-mono">
                <span>{db.connection_count} connections · created {formatDate(db.created_at)}</span>
                <span>last backup {formatRelative(db.last_backup_at)}</span>
              </div>

              <div className="pt-3 border-t border-zinc-850 flex flex-wrap items-center gap-1.5">
                <button
                  onClick={() => openInspect(db)}
                  className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition"
                >
                  <Eye className="w-3 h-3" /> Inspect
                </button>
                <button
                  onClick={() => openCredentials(db)}
                  className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition"
                >
                  <KeyRound className="w-3 h-3" /> Credentials
                </button>
                <button
                  onClick={() => openBackups(db)}
                  className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition"
                >
                  <HardDrive className="w-3 h-3" /> Backups
                </button>
                {db.status !== 'running' && (
                  <button
                    onClick={() => power(db, 'start')}
                    disabled={busy === `power:${db.id}`}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-emerald-500/10 hover:bg-emerald-500/20 text-emerald-300 border border-emerald-500/20 text-[10px] transition disabled:opacity-50"
                  >
                    <Play className="w-3 h-3" /> Start
                  </button>
                )}
                {db.status === 'running' && (
                  <button
                    onClick={() => power(db, 'stop')}
                    disabled={busy === `power:${db.id}`}
                    className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition disabled:opacity-50"
                  >
                    <Square className="w-3 h-3" /> Stop
                  </button>
                )}
                <button
                  onClick={() => power(db, 'restart')}
                  disabled={busy === `power:${db.id}`}
                  className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition disabled:opacity-50"
                >
                  <RotateCcw className="w-3 h-3" /> Restart
                </button>
                <button
                  onClick={() => {
                    setDeleteFor(db);
                    setDeleteConfirm('');
                  }}
                  className="ml-auto inline-flex items-center gap-1 px-2 py-1 rounded-md bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/20 text-[10px] transition"
                >
                  <Trash2 className="w-3 h-3" /> Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <Modal
        isOpen={showCreate}
        onClose={() => setShowCreate(false)}
        title="Create managed database"
        description="The password is shown once. Store it before closing the next dialog."
        maxWidth="lg"
      >
        <form onSubmit={handleCreate} className="space-y-3">
          <div>
            <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Name</label>
            <input
              value={form.name}
              onChange={(e) => setForm({ ...form, name: e.target.value })}
              placeholder="orders-db"
              required
              className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
            />
            <p className="text-[10px] text-zinc-500 mt-1">Lowercase letters, digits and hyphens; starts with a letter.</p>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Engine</label>
              <select
                value={form.engine}
                onChange={(e) => setForm({ ...form, engine: e.target.value })}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              >
                <option value="postgresql">PostgreSQL</option>
                <option value="mysql">MySQL</option>
                <option value="redis">Redis</option>
                <option value="mongodb">MongoDB</option>
              </select>
            </div>
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Plan</label>
              <select
                value={form.plan_name}
                onChange={(e) => setForm({ ...form, plan_name: e.target.value })}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              >
                {plans.map((plan) => (
                  <option key={plan.id} value={plan.id}>
                    {plan.name} — {plan.vcpus} vCPU / {plan.memory_mb} MB / {plan.storage_gb} GB
                  </option>
                ))}
              </select>
            </div>
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Engine version</label>
              <input
                value={form.engine_version}
                onChange={(e) => setForm({ ...form, engine_version: e.target.value })}
                placeholder="default"
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              />
            </div>
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Database name</label>
              <input
                value={form.database_name}
                onChange={(e) => setForm({ ...form, database_name: e.target.value })}
                placeholder="derived from name"
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              />
            </div>
          </div>
          <label className="flex items-center gap-2 text-xs text-zinc-300">
            <input
              type="checkbox"
              checked={form.auto_backup}
              onChange={(e) => setForm({ ...form, auto_backup: e.target.checked })}
              className="accent-white"
            />
            Enable automatic backups
          </label>
          <div className="pt-2 flex justify-end gap-2">
            <button
              type="button"
              onClick={() => setShowCreate(false)}
              className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={creating}
              className="px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold disabled:opacity-50"
            >
              {creating ? 'Creating…' : 'Create database'}
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        isOpen={!!created}
        onClose={() => setCreated(null)}
        title="Database created"
        description="This password is displayed exactly once."
        maxWidth="lg"
      >
        {created && (
          <div className="space-y-3">
            <div className="p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
              {created.database.name} is {created.database.status}.
            </div>
            <div className="space-y-2 text-xs font-mono">
              <div className="p-2.5 rounded-md bg-zinc-900 border border-zinc-800">
                <div className="text-[10px] text-zinc-500 uppercase">Password</div>
                <div className="flex items-center justify-between gap-2 mt-1">
                  <span className="text-zinc-100 break-all">{created.password}</span>
                  <button
                    onClick={() => copy('password', created.password)}
                    className="p-1 rounded bg-zinc-800 text-zinc-300 hover:text-white"
                    title="Copy password"
                  >
                    {copied === 'password' ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                  </button>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-2 text-zinc-400">
                <div>host <span className="text-zinc-200">{created.database.host}</span></div>
                <div>port <span className="text-zinc-200">{created.database.port}</span></div>
                <div>database <span className="text-zinc-200">{created.database.database_name}</span></div>
                <div>user <span className="text-zinc-200">{created.database.username}</span></div>
              </div>
            </div>
            <div className="flex justify-end">
              <button
                onClick={() => setCreated(null)}
                className="px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold"
              >
                I stored it
              </button>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        isOpen={!!credentialsFor}
        onClose={() => {
          setCredentialsFor(null);
          setCredentials(null);
        }}
        title={`Credentials · ${credentialsFor?.name || ''}`}
        description="Every credential access is audited. Treat the URI as a secret."
        maxWidth="lg"
      >
        {credentialsLoading ? (
          <div className="p-6 text-center text-xs text-zinc-400">Decrypting credential…</div>
        ) : credentials ? (
          <div className="space-y-2 text-xs font-mono">
            {[
              ['Host', credentials.host],
              ['Port', String(credentials.port)],
              ['Database', credentials.database_name],
              ['Username', credentials.username],
              ['Password', credentials.password],
              ['URI', credentials.uri],
            ].map(([label, value]) => (
              <div key={label} className="p-2.5 rounded-md bg-zinc-900 border border-zinc-800">
                <div className="text-[10px] text-zinc-500 uppercase">{label}</div>
                <div className="flex items-center justify-between gap-2 mt-1">
                  <span className="text-zinc-100 break-all">{value}</span>
                  <button
                    onClick={() => copy(label, value)}
                    className="p-1 rounded bg-zinc-800 text-zinc-300 hover:text-white flex-shrink-0"
                    title={`Copy ${label}`}
                  >
                    {copied === label ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                  </button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="p-6 text-center text-xs text-zinc-500">Credentials unavailable.</div>
        )}
      </Modal>

      <Modal
        isOpen={!!backupsFor}
        onClose={() => setBackupsFor(null)}
        title={`Backups · ${backupsFor?.name || ''}`}
        description="Backups are logical and can be restored into this database."
        maxWidth="lg"
      >
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <input
              value={backupName}
              onChange={(e) => setBackupName(e.target.value)}
              placeholder="Backup name (optional)"
              className="flex-1 bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
            />
            <button
              onClick={createBackup}
              disabled={backupsLoading}
              className="px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold disabled:opacity-50"
            >
              Create backup
            </button>
          </div>
          {backupsLoading ? (
            <div className="p-5 text-center text-xs text-zinc-400">Loading backups…</div>
          ) : backups.length === 0 ? (
            <div className="p-5 text-center text-xs text-zinc-500 border border-dashed border-zinc-800 rounded-lg">
              No backups yet.
            </div>
          ) : (
            <div className="divide-y divide-zinc-850 rounded-lg border border-zinc-800 overflow-hidden">
              {backups.map((backup) => (
                <div key={backup.id} className="p-2.5 flex items-center justify-between gap-2 bg-zinc-900/40">
                  <div className="min-w-0">
                    <div className="text-xs text-zinc-100 truncate">{backup.name}</div>
                    <div className="text-[10px] text-zinc-500 font-mono">
                      {formatDate(backup.created_at)} · {formatBytes(backup.size_bytes)} · {backup.status}
                    </div>
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      onClick={() => restoreBackup(backup)}
                      className="px-2 py-1 rounded bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px]"
                    >
                      Restore
                    </button>
                    <button
                      onClick={() => deleteBackup(backup)}
                      className="p-1 rounded bg-zinc-850 hover:bg-rose-500/20 text-zinc-400 hover:text-rose-300 border border-zinc-800"
                      title="Delete backup"
                    >
                      <Trash2 className="w-3 h-3" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </Modal>

      <Modal
        isOpen={!!inspectFor}
        onClose={() => setInspectFor(null)}
        title={`Inspect · ${inspectFor?.name || ''}`}
        description="Deterministic telemetry and recent log lines. Secrets are never included."
        maxWidth="2xl"
      >
        {inspectLoading ? (
          <div className="p-6 text-center text-xs text-zinc-400">Loading telemetry…</div>
        ) : (
          <div className="space-y-3">
            {metrics && (
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
                {[
                  ['CPU', `${metrics.cpu_percent.toFixed(1)}%`],
                  ['Memory', `${metrics.memory_usage_percent.toFixed(1)}%`],
                  ['Storage', `${metrics.storage_usage_percent.toFixed(1)}%`],
                  ['Connections', String(metrics.connection_count)],
                ].map(([label, value]) => (
                  <div key={label} className="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800">
                    <div className="text-[10px] uppercase font-mono text-zinc-500">{label}</div>
                    <div className="text-sm font-semibold text-white mt-0.5">{value}</div>
                  </div>
                ))}
              </div>
            )}
            <div className="rounded-lg border border-zinc-800 bg-zinc-950 overflow-hidden">
              <div className="px-3 py-2 border-b border-zinc-800 text-[10px] uppercase font-mono text-zinc-400 flex items-center justify-between">
                <span>Recent logs</span>
                <span>{logs.length} lines</span>
              </div>
              <pre className="p-3 text-[10px] leading-relaxed text-zinc-300 font-mono max-h-72 overflow-auto whitespace-pre-wrap">
                {logs.length ? logs.join('\n') : 'No log lines available.'}
              </pre>
            </div>
            <div className="flex justify-end">
              <button
                onClick={() => {
                  if (inspectFor) onDiagnose('database', inspectFor.id, inspectFor.name);
                  setInspectFor(null);
                }}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold"
              >
                <Stethoscope className="w-3.5 h-3.5" />
                Run diagnostics
                <ArrowUpRight className="w-3 h-3" />
              </button>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        isOpen={!!deleteFor}
        onClose={() => setDeleteFor(null)}
        title={`Delete ${deleteFor?.name || 'database'}?`}
        description="This is destructive. The database is soft-deleted and removed from the inventory and cost estimates."
      >
        <div className="space-y-3">
          <p className="text-xs text-zinc-400">
            Type <span className="font-mono text-zinc-100">{deleteFor?.name}</span> to confirm.
          </p>
          <input
            value={deleteConfirm}
            onChange={(e) => setDeleteConfirm(e.target.value)}
            className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-rose-500"
          />
          <div className="flex justify-end gap-2">
            <button
              onClick={() => setDeleteFor(null)}
              className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs"
            >
              Cancel
            </button>
            <button
              onClick={remove}
              disabled={!deleteFor || deleteConfirm !== deleteFor.name || busy === `delete:${deleteFor.id}`}
              className="px-3 py-1.5 rounded-md bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold disabled:opacity-40"
            >
              Delete database
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
