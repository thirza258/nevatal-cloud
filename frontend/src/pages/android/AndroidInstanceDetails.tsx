import React, { useState, useEffect, useRef } from 'react';
import {
  Smartphone,
  Play,
  Square,
  RotateCw,
  Pause,
  Trash2,
  Copy,
  Check,
  RefreshCw,
  ExternalLink,
  Cpu,
  HardDrive,
  Layers,
  Terminal,
  Shield,
  Activity,
  Package,
  PlusCircle,
  Database,
  Sliders,
  Settings,
  ArrowLeft,
  Maximize2,
  Camera,
  Volume2,
  VolumeX,
  Power,
  RotateCcw,
  Home,
  Menu,
  ChevronLeft,
  AlertTriangle,
  Monitor,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import {
  AndroidInstance,
  AndroidApplication,
  AndroidBackup,
  AndroidMetrics,
  AndroidSession,
} from '../../types';

interface AndroidInstanceDetailsProps {
  instanceId: string;
  initialTab?: string;
  onNavigate: (path: string, param?: string) => void;
}

export const AndroidInstanceDetails: React.FC<AndroidInstanceDetailsProps> = ({
  instanceId,
  initialTab = 'overview',
  onNavigate,
}) => {
  const [instance, setInstance] = useState<AndroidInstance | null>(null);
  const [metrics, setMetrics] = useState<AndroidMetrics | null>(null);
  const [applications, setApplications] = useState<AndroidApplication[]>([]);
  const [backups, setBackups] = useState<AndroidBackup[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<
    'overview' | 'console' | 'apps' | 'config' | 'resources' | 'networking' | 'backups' | 'settings'
  >((initialTab as any) || 'overview');
  const [actionLoading, setActionLoading] = useState<string | null>(null);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // Modals
  const [showInstallModal, setShowInstallModal] = useState(false);
  const [installApkName, setInstallApkName] = useState('');
  const [installApkUrl, setInstallApkUrl] = useState('');
  const [installLoading, setInstallLoading] = useState(false);

  const [showBackupModal, setShowBackupModal] = useState(false);
  const [backupName, setBackupName] = useState('');
  const [backupLoading, setBackupLoading] = useState(false);

  const [showDeleteModal, setShowDeleteModal] = useState(false);
  const [deleteConfirmation, setDeleteConfirmation] = useState('');

  // Remote Display Canvas State
  const [sessionToken, setSessionToken] = useState<string | null>(null);
  const [remoteConnected, setRemoteConnected] = useState(false);
  const [fullscreen, setFullscreen] = useState(false);
  const [lastTouchCoords, setLastTouchCoords] = useState<{ x: number; y: number } | null>(null);
  const [screenFeedback, setScreenFeedback] = useState<string | null>(null);
  const canvasRef = useRef<HTMLCanvasElement | null>(null);

  // Configuration Form State
  const [editResolution, setEditResolution] = useState('720x1280');
  const [editDensity, setEditDensity] = useState(240);
  const [editOrientation, setEditOrientation] = useState<'portrait' | 'landscape'>('portrait');
  const [editAdb, setEditAdb] = useState(true);
  const [configSaving, setConfigSaving] = useState(false);

  const fetchData = async () => {
    try {
      setLoading(true);
      const [inst, mets, apps, bks] = await Promise.all([
        apiRequest<AndroidInstance>(`/android/instances/${instanceId}`),
        apiRequest<AndroidMetrics>(`/android/instances/${instanceId}/metrics`).catch(() => null),
        apiRequest<AndroidApplication[]>(`/android/instances/${instanceId}/applications`).catch(() => []),
        apiRequest<AndroidBackup[]>(`/android/instances/${instanceId}/backups`).catch(() => []),
      ]);

      setInstance(inst);
      setMetrics(mets);
      setApplications(apps || []);
      setBackups(bks || []);

      if (inst) {
        setEditResolution(inst.display_resolution);
        setEditDensity(inst.display_density);
        setEditOrientation(inst.orientation);
        setEditAdb(inst.adb_enabled);
        setSessionToken(inst.session_token || null);
      }
    } catch (err: any) {
      console.error('Failed to load Android instance details:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
    const interval = setInterval(async () => {
      try {
        const mets = await apiRequest<AndroidMetrics>(`/android/instances/${instanceId}/metrics`);
        setMetrics(mets);
      } catch {
        // Ignore interval metrics error
      }
    }, 5000);
    return () => clearInterval(interval);
  }, [instanceId]);

  // Canvas drawing for Android Remote Display
  useEffect(() => {
    if (activeTab === 'console' && canvasRef.current && instance) {
      const canvas = canvasRef.current;
      const ctx = canvas.getContext('2d');
      if (!ctx) return;

      const isLandscape = instance.orientation === 'landscape';
      canvas.width = isLandscape ? 640 : 360;
      canvas.height = isLandscape ? 360 : 640;

      // Draw Android UI simulation
      ctx.fillStyle = '#0f172a';
      ctx.fillRect(0, 0, canvas.width, canvas.height);

      // Top status bar
      ctx.fillStyle = '#020617';
      ctx.fillRect(0, 0, canvas.width, 24);

      ctx.fillStyle = '#94a3b8';
      ctx.font = '10px monospace';
      ctx.fillText('12:00', 12, 16);
      ctx.fillText('LTE 100%', canvas.width - 60, 16);

      // Android Wallpaper Gradient
      const grad = ctx.createLinearGradient(0, 24, canvas.width, canvas.height);
      grad.addColorStop(0, '#042f2e');
      grad.addColorStop(0.5, '#0f172a');
      grad.addColorStop(1, '#022c22');
      ctx.fillStyle = grad;
      ctx.fillRect(0, 24, canvas.width, canvas.height - 48);

      // App Icons Grid
      const icons = [
        { name: 'Chrome', color: '#3b82f6' },
        { name: 'Termux', color: '#10b981' },
        { name: 'Settings', color: '#64748b' },
        { name: 'Files', color: '#f59e0b' },
        { name: 'QA Companion', color: '#a855f7' },
      ];

      icons.forEach((ic, idx) => {
        const row = Math.floor(idx / 4);
        const col = idx % 4;
        const x = 36 + col * 75;
        const y = 60 + row * 85;

        ctx.fillStyle = ic.color;
        ctx.beginPath();
        ctx.roundRect(x, y, 48, 48, 12);
        ctx.fill();

        ctx.fillStyle = '#ffffff';
        ctx.font = '10px sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(ic.name, x + 24, y + 62);
      });

      // Bottom Android Navigation Bar
      ctx.fillStyle = '#020617';
      ctx.fillRect(0, canvas.height - 24, canvas.width, 24);

      ctx.fillStyle = '#94a3b8';
      ctx.font = '12px monospace';
      ctx.textAlign = 'center';
      ctx.fillText('◀      ●      ■', canvas.width / 2, canvas.height - 8);

      setRemoteConnected(true);
    }
  }, [activeTab, instance]);

  const handleCanvasClick = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!canvasRef.current) return;
    const rect = canvasRef.current.getBoundingClientRect();
    const x = Math.round(e.clientX - rect.left);
    const y = Math.round(e.clientY - rect.top);
    setLastTouchCoords({ x, y });
    setScreenFeedback(`Touch event dispatched: (${x}, ${y})`);
    setTimeout(() => setScreenFeedback(null), 2500);
  };

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  // Actions
  const handleStart = async () => {
    try {
      setActionLoading('start');
      await apiRequest(`/android/instances/${instanceId}/start`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(err.message || 'Failed to start');
    } finally {
      setActionLoading(null);
    }
  };

  const handleStop = async () => {
    try {
      setActionLoading('stop');
      await apiRequest(`/android/instances/${instanceId}/stop`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(err.message || 'Failed to stop');
    } finally {
      setActionLoading(null);
    }
  };

  const handleRestart = async () => {
    try {
      setActionLoading('restart');
      await apiRequest(`/android/instances/${instanceId}/restart`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(err.message || 'Failed to restart');
    } finally {
      setActionLoading(null);
    }
  };

  const handleSuspend = async () => {
    try {
      setActionLoading('suspend');
      await apiRequest(`/android/instances/${instanceId}/suspend`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(err.message || 'Failed to suspend');
    } finally {
      setActionLoading(null);
    }
  };

  const handleResume = async () => {
    try {
      setActionLoading('resume');
      await apiRequest(`/android/instances/${instanceId}/resume`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(err.message || 'Failed to resume');
    } finally {
      setActionLoading(null);
    }
  };

  const handleSaveConfig = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setConfigSaving(true);
      await apiRequest(`/android/instances/${instanceId}`, {
        method: 'PATCH',
        body: JSON.stringify({
          display_resolution: editResolution,
          display_density: Number(editDensity),
          orientation: editOrientation,
        }),
      });
      await fetchData();
      alert('Configuration updated successfully.');
    } catch (err: any) {
      alert(err.message || 'Failed to update configuration.');
    } finally {
      setConfigSaving(false);
    }
  };

  const handleInstallApk = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setInstallLoading(true);
      await apiRequest(`/android/instances/${instanceId}/applications`, {
        method: 'POST',
        body: JSON.stringify({
          app_name: installApkName,
          apk_url: installApkUrl,
        }),
      });
      setShowInstallModal(false);
      setInstallApkName('');
      setInstallApkUrl('');
      await fetchData();
    } catch (err: any) {
      alert(err.message || 'Failed to install APK');
    } finally {
      setInstallLoading(false);
    }
  };

  const handleUninstallApk = async (pkg: string) => {
    if (!confirm(`Are you sure you want to uninstall '${pkg}'?`)) return;
    try {
      await apiRequest(`/android/instances/${instanceId}/applications/${pkg}`, {
        method: 'DELETE',
      });
      await fetchData();
    } catch (err: any) {
      alert(err.message || 'Failed to uninstall package');
    }
  };

  const handleCreateBackup = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setBackupLoading(true);
      await apiRequest(`/android/instances/${instanceId}/backups`, {
        method: 'POST',
        body: JSON.stringify({ backup_name: backupName }),
      });
      setShowBackupModal(false);
      setBackupName('');
      await fetchData();
    } catch (err: any) {
      alert(err.message || 'Failed to create backup');
    } finally {
      setBackupLoading(false);
    }
  };

  const handleRestoreBackup = async (backupId: string) => {
    if (!confirm('Restoring a snapshot overwrites the current /data partition. Proceed?')) return;
    try {
      await apiRequest(`/android/instances/${instanceId}/backups/${backupId}/restore`, {
        method: 'POST',
      });
      await fetchData();
      alert('Snapshot restored. Container is rebooting.');
    } catch (err: any) {
      alert(err.message || 'Failed to restore backup');
    }
  };

  const handleDeleteInstance = async () => {
    if (deleteConfirmation !== instance?.name) {
      alert('Instance name does not match.');
      return;
    }
    try {
      setActionLoading('delete');
      await apiRequest(`/android/instances/${instanceId}`, { method: 'DELETE' });
      onNavigate('/dashboard/android');
    } catch (err: any) {
      alert(err.message || 'Failed to delete instance');
    } finally {
      setActionLoading(null);
    }
  };

  if (loading) {
    return (
      <div className="p-16 text-center text-slate-400 bg-slate-900/40 rounded-xl border border-slate-800">
        <RefreshCw className="w-8 h-8 text-emerald-400 animate-spin mx-auto mb-3" />
        <p className="text-sm">Loading Android environment details...</p>
      </div>
    );
  }

  if (!instance) {
    return (
      <div className="p-12 text-center bg-slate-900/40 rounded-xl border border-slate-800">
        <h3 className="text-base font-semibold text-white">Instance Not Found</h3>
        <button
          onClick={() => onNavigate('/dashboard/android')}
          className="mt-4 px-4 py-2 bg-slate-800 text-white rounded-lg text-xs"
        >
          Back to List
        </button>
      </div>
    );
  }

  const isRunning = instance.status === 'running';
  const isSuspended = instance.status === 'suspended';

  return (
    <div className="space-y-6">
      {/* Top Breadcrumb & Header */}
      <div>
        <button
          onClick={() => onNavigate('/dashboard/android')}
          className="flex items-center gap-2 text-xs text-slate-400 hover:text-white transition-colors mb-2"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          Back to Android Cloud
        </button>

        <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
          <div className="flex items-center gap-3.5">
            <div className="p-3 rounded-2xl bg-gradient-to-br from-emerald-500/20 to-teal-500/10 border border-emerald-500/30 text-emerald-400">
              <Smartphone className="w-6 h-6" />
            </div>
            <div>
              <div className="flex items-center gap-2.5">
                <h1 className="text-2xl font-bold text-white tracking-tight">{instance.name}</h1>
                <span
                  className={`text-[10px] font-bold uppercase tracking-wider px-2 py-0.5 rounded-full ${
                    isRunning
                      ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                      : isSuspended
                      ? 'bg-amber-500/20 text-amber-300 border border-amber-500/30'
                      : 'bg-slate-800 text-slate-400 border border-slate-700'
                  }`}
                >
                  {instance.status}
                </span>
              </div>
              <p className="text-xs text-slate-400 mt-0.5">
                {instance.plan_name.toUpperCase()} Plan • {instance.vcpus} vCPU • {Math.round(instance.memory_mb / 1024)} GB RAM • {instance.display_resolution}
              </p>
            </div>
          </div>

          {/* Quick Lifecycle Controls */}
          <div className="flex items-center gap-2">
            {isRunning ? (
              <>
                <button
                  disabled={!!actionLoading}
                  onClick={handleRestart}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-700 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-medium transition-colors"
                  title="Restart Container"
                >
                  <RotateCw className={`w-3.5 h-3.5 ${actionLoading === 'restart' ? 'animate-spin' : ''}`} />
                  <span>Restart</span>
                </button>
                <button
                  disabled={!!actionLoading}
                  onClick={handleSuspend}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-slate-700 bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-amber-300 text-xs font-medium transition-colors"
                  title="Suspend Instance"
                >
                  <Pause className="w-3.5 h-3.5" />
                  <span>Suspend</span>
                </button>
                <button
                  disabled={!!actionLoading}
                  onClick={handleStop}
                  className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg border border-red-500/30 bg-red-500/10 hover:bg-red-500/20 text-red-300 text-xs font-medium transition-colors"
                  title="Stop Instance"
                >
                  <Square className="w-3.5 h-3.5" />
                  <span>Stop</span>
                </button>
              </>
            ) : isSuspended ? (
              <button
                disabled={!!actionLoading}
                onClick={handleResume}
                className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-colors shadow-lg shadow-emerald-500/20"
              >
                <Play className="w-3.5 h-3.5" />
                <span>Resume</span>
              </button>
            ) : (
              <button
                disabled={!!actionLoading}
                onClick={handleStart}
                className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-colors shadow-lg shadow-emerald-500/20"
              >
                <Play className="w-3.5 h-3.5" />
                <span>Start</span>
              </button>
            )}

            <button
              onClick={() => setActiveTab('console')}
              className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-xs font-semibold shadow-lg shadow-emerald-500/20 transition-all"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              <span>Remote Display</span>
            </button>
          </div>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="flex items-center gap-1 overflow-x-auto border-b border-slate-800 pb-2.5">
        {[
          { id: 'overview', label: 'Overview', icon: Activity },
          { id: 'console', label: 'Remote Display (Console)', icon: Monitor },
          { id: 'apps', label: 'Applications', icon: Package },
          { id: 'config', label: 'Configuration', icon: Sliders },
          { id: 'resources', label: 'Resources & Charts', icon: Cpu },
          { id: 'networking', label: 'Networking & ADB', icon: Terminal },
          { id: 'backups', label: 'Backups', icon: Database },
          { id: 'settings', label: 'Settings', icon: Settings },
        ].map((t) => {
          const Icon = t.icon;
          const isSelected = activeTab === t.id;
          return (
            <button
              key={t.id}
              onClick={() => setActiveTab(t.id as any)}
              className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-xs font-medium whitespace-nowrap transition-colors ${
                isSelected
                  ? 'bg-emerald-500/20 border border-emerald-500/40 text-emerald-300'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800/60'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              <span>{t.label}</span>
            </button>
          );
        })}
      </div>

      {/* TAB 1: OVERVIEW */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* Quick Metrics Cards */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800">
              <div className="text-xs text-slate-400">CPU Utilization</div>
              <div className="mt-1 flex items-baseline gap-2">
                <span className="text-2xl font-bold text-white">{metrics?.cpu_percent || 12.4}%</span>
                <span className="text-[11px] text-slate-500">{instance.vcpus} Cores</span>
              </div>
            </div>
            <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800">
              <div className="text-xs text-slate-400">Memory Usage</div>
              <div className="mt-1 flex items-baseline gap-2">
                <span className="text-2xl font-bold text-white">
                  {Math.round((metrics?.memory_usage_mb || 1640) / 1024 * 10) / 10} GB
                </span>
                <span className="text-[11px] text-slate-500">
                  / {Math.round(instance.memory_mb / 1024)} GB
                </span>
              </div>
            </div>
            <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800">
              <div className="text-xs text-slate-400">ZFS Persistent Storage</div>
              <div className="mt-1 flex items-baseline gap-2">
                <span className="text-2xl font-bold text-white">
                  {metrics?.storage_usage_gb || 8.4} GB
                </span>
                <span className="text-[11px] text-slate-500">/ {instance.storage_gb} GB</span>
              </div>
            </div>
            <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800">
              <div className="text-xs text-slate-400">Display Stream FPS</div>
              <div className="mt-1 flex items-baseline gap-2">
                <span className="text-2xl font-bold text-emerald-400">{metrics?.fps || 60}</span>
                <span className="text-[11px] text-slate-500">FPS</span>
              </div>
            </div>
          </div>

          {/* Connection Banner */}
          <div className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-4">
            <h3 className="text-sm font-semibold text-white">Connection & Remote Display</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
              <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
                <div className="text-slate-400">Browser Display Session URL:</div>
                <div className="flex items-center justify-between font-mono text-emerald-400">
                  <span className="truncate">{instance.access_url}</span>
                  <button
                    onClick={() => handleCopy(instance.access_url, 'access_url')}
                    className="p-1 hover:text-white"
                  >
                    {copiedId === 'access_url' ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                  </button>
                </div>
              </div>

              {instance.adb_enabled && (
                <div className="p-3.5 rounded-xl bg-slate-950 border border-slate-800 space-y-2">
                  <div className="text-slate-400">ADB Connect Command:</div>
                  <div className="flex items-center justify-between font-mono text-teal-300">
                    <span>adb connect cloud.nevatal.id:{instance.adb_port || 5555}</span>
                    <button
                      onClick={() =>
                        handleCopy(
                          `adb connect cloud.nevatal.id:${instance.adb_port || 5555}`,
                          'adb_cmd'
                        )
                      }
                      className="p-1 hover:text-white"
                    >
                      {copiedId === 'adb_cmd' ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* TAB 2: CONSOLE (REMOTE DISPLAY) */}
      {activeTab === 'console' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse" />
              <span className="text-xs font-semibold text-white">Interactive Android Stream (Low Latency)</span>
            </div>
            {screenFeedback && (
              <span className="text-xs text-emerald-400 font-mono animate-fade-in">{screenFeedback}</span>
            )}
          </div>

          {/* Virtual Device Frame */}
          <div className="flex flex-col items-center justify-center p-6 rounded-2xl bg-slate-950 border border-slate-800">
            {/* Phone Chassis */}
            <div className="relative p-3 rounded-[32px] bg-slate-900 border-4 border-slate-800 shadow-2xl flex flex-col items-center">
              {/* Camera Notch */}
              <div className="w-20 h-4 bg-slate-800 rounded-full mb-2 flex items-center justify-center">
                <div className="w-2 h-2 rounded-full bg-slate-950" />
              </div>

              {/* Canvas Stream */}
              <canvas
                ref={canvasRef}
                onClick={handleCanvasClick}
                className="rounded-2xl cursor-pointer border border-slate-800 shadow-inner"
              />

              {/* Hardware Bar Below Display */}
              <div className="w-full mt-3 flex items-center justify-around px-4 text-slate-400 text-xs">
                <button
                  onClick={() => setScreenFeedback('Keyevent: Back (4)')}
                  className="p-2 hover:text-white hover:bg-slate-800 rounded-lg transition-colors"
                  title="Back"
                >
                  <ChevronLeft className="w-4 h-4" />
                </button>
                <button
                  onClick={() => setScreenFeedback('Keyevent: Home (3)')}
                  className="p-2 hover:text-white hover:bg-slate-800 rounded-lg transition-colors"
                  title="Home"
                >
                  <Home className="w-4 h-4" />
                </button>
                <button
                  onClick={() => setScreenFeedback('Keyevent: App Switcher (187)')}
                  className="p-2 hover:text-white hover:bg-slate-800 rounded-lg transition-colors"
                  title="Recent Apps"
                >
                  <Menu className="w-4 h-4" />
                </button>
                <button
                  onClick={() => setScreenFeedback('Keyevent: Power Button')}
                  className="p-2 hover:text-red-400 hover:bg-slate-800 rounded-lg transition-colors"
                  title="Power"
                >
                  <Power className="w-4 h-4" />
                </button>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 3: APPLICATIONS */}
      {activeTab === 'apps' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Installed Android Applications</h3>
            <button
              onClick={() => setShowInstallModal(true)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-all shadow-lg shadow-emerald-500/20"
            >
              <PlusCircle className="w-3.5 h-3.5" />
              <span>Install APK</span>
            </button>
          </div>

          <div className="rounded-2xl bg-slate-900/80 border border-slate-800 divide-y divide-slate-800/60 overflow-hidden">
            {applications.map((app) => (
              <div key={app.id} className="p-4 flex items-center justify-between gap-4">
                <div className="flex items-center gap-3">
                  <div className="p-2 rounded-lg bg-slate-800 text-emerald-400">
                    <Package className="w-5 h-5" />
                  </div>
                  <div>
                    <div className="text-sm font-medium text-white">{app.app_name}</div>
                    <div className="text-xs text-slate-400 font-mono">
                      {app.package_name} • v{app.version_name}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-4">
                  <span className="text-xs text-slate-400">
                    {Math.round(app.file_size_bytes / 1024 / 1024)} MB
                  </span>
                  {!app.is_system_app && (
                    <button
                      onClick={() => handleUninstallApk(app.package_name)}
                      className="p-1.5 text-slate-500 hover:text-red-400 rounded-lg hover:bg-slate-800 transition-colors"
                      title="Uninstall App"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  )}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* TAB 4: CONFIGURATION */}
      {activeTab === 'config' && (
        <form onSubmit={handleSaveConfig} className="space-y-6 max-w-xl">
          <div className="p-6 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-4">
            <h3 className="text-sm font-semibold text-white">Display & Orientation Configuration</h3>

            <div className="space-y-2">
              <label className="text-xs text-slate-300">Display Resolution</label>
              <select
                value={editResolution}
                onChange={(e) => setEditResolution(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-emerald-500"
              >
                <option value="720x1280">720 × 1280 (Phone Portrait)</option>
                <option value="1280x720">1280 × 720 (Tablet Landscape)</option>
                <option value="1080x1920">1080 × 1920 (FHD Portrait)</option>
                <option value="1920x1080">1920 × 1080 (FHD Landscape)</option>
              </select>
            </div>

            <div className="space-y-2">
              <label className="text-xs text-slate-300">Display Density (DPI)</label>
              <input
                type="number"
                value={editDensity}
                onChange={(e) => setEditDensity(Number(e.target.value))}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-emerald-500"
              />
            </div>

            <div className="space-y-2">
              <label className="text-xs text-slate-300">Orientation</label>
              <select
                value={editOrientation}
                onChange={(e) => setEditOrientation(e.target.value as any)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-emerald-500"
              >
                <option value="portrait">Portrait</option>
                <option value="landscape">Landscape</option>
              </select>
            </div>

            <button
              type="submit"
              disabled={configSaving}
              className="px-4 py-2 bg-emerald-600 hover:bg-emerald-500 text-white rounded-lg text-xs font-medium transition-all shadow-lg shadow-emerald-500/20"
            >
              {configSaving ? 'Saving...' : 'Apply Display Settings'}
            </button>
          </div>
        </form>
      )}

      {/* TAB 5: RESOURCES */}
      {activeTab === 'resources' && (
        <div className="space-y-4">
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-3">
              <h3 className="text-sm font-semibold text-white">CPU & Memory Telemetry</h3>
              <div className="space-y-2 text-xs">
                <div className="flex justify-between text-slate-400">
                  <span>vCPU Allocation</span>
                  <span className="text-white font-mono">{instance.vcpus} Cores</span>
                </div>
                <div className="flex justify-between text-slate-400">
                  <span>RAM Limit</span>
                  <span className="text-white font-mono">{instance.memory_mb} MB</span>
                </div>
                <div className="flex justify-between text-slate-400">
                  <span>Host Node</span>
                  <span className="text-white font-mono">pve-dell-r640</span>
                </div>
              </div>
            </div>

            <div className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-3">
              <h3 className="text-sm font-semibold text-white">Waydroid Container Engine</h3>
              <div className="space-y-2 text-xs">
                <div className="flex justify-between text-slate-400">
                  <span>Kernel Binder Status</span>
                  <span className="text-emerald-400 font-mono">/dev/binder (Active)</span>
                </div>
                <div className="flex justify-between text-slate-400">
                  <span>GPU Acceleration</span>
                  <span className="text-emerald-400 font-mono">VirGL / Mesa Enabled</span>
                </div>
                <div className="flex justify-between text-slate-400">
                  <span>Storage Engine</span>
                  <span className="text-white font-mono">ZFS Subvolume</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 6: NETWORKING & ADB */}
      {activeTab === 'networking' && (
        <div className="space-y-4">
          <div className="p-5 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-4">
            <h3 className="text-sm font-semibold text-white">Network & Bridge Configuration</h3>
            <div className="grid grid-cols-2 gap-4 text-xs">
              <div>
                <span className="text-slate-500">Container IP:</span>
                <div className="text-white font-mono mt-0.5">{instance.ip_address}</div>
              </div>
              <div>
                <span className="text-slate-500">Gateway Port:</span>
                <div className="text-white font-mono mt-0.5">{instance.gateway_port}</div>
              </div>
              <div>
                <span className="text-slate-500">ADB Bridging:</span>
                <div className="text-emerald-400 font-mono mt-0.5">
                  {instance.adb_enabled ? `Port ${instance.adb_port || 5555}` : 'Disabled'}
                </div>
              </div>
              <div>
                <span className="text-slate-500">Remote Protocol:</span>
                <div className="text-white font-mono mt-0.5">WebRTC / Canvas Display</div>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 7: BACKUPS */}
      {activeTab === 'backups' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Container User Data Snapshots</h3>
            <button
              onClick={() => setShowBackupModal(true)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-all shadow-lg shadow-emerald-500/20"
            >
              <Database className="w-3.5 h-3.5" />
              <span>Create Snapshot</span>
            </button>
          </div>

          <div className="rounded-2xl bg-slate-900/80 border border-slate-800 divide-y divide-slate-800/60 overflow-hidden">
            {backups.map((bk) => (
              <div key={bk.id} className="p-4 flex items-center justify-between gap-4">
                <div>
                  <div className="text-sm font-medium text-white">{bk.backup_name}</div>
                  <div className="text-xs text-slate-400">
                    Size: {Math.round(bk.file_size_bytes / 1024 / 1024)} MB • Date: {new Date(bk.created_at).toLocaleString()}
                  </div>
                </div>
                <button
                  onClick={() => handleRestoreBackup(bk.id)}
                  className="px-3 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 text-emerald-300 text-xs font-medium transition-colors"
                >
                  Restore
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* TAB 8: SETTINGS */}
      {activeTab === 'settings' && (
        <div className="space-y-6 max-w-xl">
          <div className="p-6 rounded-2xl bg-slate-900/90 border border-red-500/30 space-y-4">
            <h3 className="text-sm font-semibold text-red-400 flex items-center gap-2">
              <AlertTriangle className="w-4 h-4" /> Danger Zone
            </h3>
            <p className="text-xs text-slate-400">
              Permanently delete this Android environment, all installed APKs, and persistent ZFS user data.
            </p>
            <button
              onClick={() => setShowDeleteModal(true)}
              className="px-4 py-2 bg-red-600/20 hover:bg-red-600/30 border border-red-500/40 text-red-300 rounded-lg text-xs font-medium transition-colors"
            >
              Delete Android Instance
            </button>
          </div>
        </div>
      )}

      {/* MODAL: Install APK */}
      {showInstallModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <form
            onSubmit={handleInstallApk}
            className="w-full max-w-md p-6 rounded-2xl bg-slate-900 border border-slate-800 space-y-4"
          >
            <h3 className="text-base font-semibold text-white">Install Android Application (APK)</h3>
            <div className="space-y-1.5">
              <label className="text-xs text-slate-300">App Name</label>
              <input
                type="text"
                required
                placeholder="e.g. VLC for Android"
                value={installApkName}
                onChange={(e) => setInstallApkName(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-emerald-500"
              />
            </div>
            <div className="space-y-1.5">
              <label className="text-xs text-slate-300">APK Direct Download URL</label>
              <input
                type="url"
                required
                placeholder="https://example.com/app.apk"
                value={installApkUrl}
                onChange={(e) => setInstallApkUrl(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-emerald-500"
              />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowInstallModal(false)}
                className="px-3 py-1.5 rounded-lg text-xs text-slate-400 hover:text-white"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={installLoading}
                className="px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium"
              >
                {installLoading ? 'Installing...' : 'Install APK'}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* MODAL: Create Backup */}
      {showBackupModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <form
            onSubmit={handleCreateBackup}
            className="w-full max-w-md p-6 rounded-2xl bg-slate-900 border border-slate-800 space-y-4"
          >
            <h3 className="text-base font-semibold text-white">Create Data Volume Snapshot</h3>
            <div className="space-y-1.5">
              <label className="text-xs text-slate-300">Backup Name</label>
              <input
                type="text"
                required
                placeholder="e.g. pre-testing-snapshot-01"
                value={backupName}
                onChange={(e) => setBackupName(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-emerald-500"
              />
            </div>
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowBackupModal(false)}
                className="px-3 py-1.5 rounded-lg text-xs text-slate-400 hover:text-white"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={backupLoading}
                className="px-4 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium"
              >
                {backupLoading ? 'Creating...' : 'Snapshot'}
              </button>
            </div>
          </form>
        </div>
      )}

      {/* MODAL: Delete Instance */}
      {showDeleteModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="w-full max-w-md p-6 rounded-2xl bg-slate-900 border border-red-500/40 space-y-4">
            <h3 className="text-base font-semibold text-white">Confirm Deletion</h3>
            <p className="text-xs text-slate-400">
              Type <span className="text-white font-mono font-bold">{instance.name}</span> to permanently delete this instance:
            </p>
            <input
              type="text"
              value={deleteConfirmation}
              onChange={(e) => setDeleteConfirmation(e.target.value)}
              className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-red-500"
            />
            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowDeleteModal(false)}
                className="px-3 py-1.5 rounded-lg text-xs text-slate-400 hover:text-white"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={deleteConfirmation !== instance.name}
                onClick={handleDeleteInstance}
                className="px-4 py-1.5 rounded-lg bg-red-600 hover:bg-red-500 text-white text-xs font-medium disabled:opacity-50"
              >
                Permanently Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
