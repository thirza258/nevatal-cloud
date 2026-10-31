import React, { useState, useEffect } from 'react';
import {
  Smartphone,
  PlusCircle,
  Search,
  RefreshCw,
  Copy,
  Check,
  Play,
  Square,
  RotateCw,
  HardDrive,
  Cpu,
  Layers,
  Sparkles,
  ArrowRight,
  Database,
  ExternalLink,
  Shield,
  Activity,
  Terminal,
  Settings,
  Trash2,
  Package,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import {
  AndroidInstance,
  AndroidImage,
  AndroidBackup,
  AndroidApplication,
} from '../../types';

interface AndroidInstancesListProps {
  onNavigate: (path: string, param?: string) => void;
}

export const AndroidInstancesList: React.FC<AndroidInstancesListProps> = ({ onNavigate }) => {
  const [instances, setInstances] = useState<AndroidInstance[]>([]);
  const [images, setImages] = useState<AndroidImage[]>([]);
  const [backups, setBackups] = useState<AndroidBackup[]>([]);
  const [loading, setLoading] = useState(true);
  const [activeTab, setActiveTab] = useState<'all' | 'images' | 'apps' | 'backups'>('all');
  const [searchTerm, setSearchTerm] = useState('');
  const [copiedId, setCopiedId] = useState<string | null>(null);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  const fetchData = async () => {
    try {
      setLoading(true);
      const [instRes, imgRes] = await Promise.all([
        apiRequest<AndroidInstance[]>('/android/instances'),
        apiRequest<AndroidImage[]>('/android/images'),
      ]);
      setInstances(instRes || []);
      setImages(imgRes || []);

      // Gather backups across instances
      const allBackups: AndroidBackup[] = [];
      if (instRes && instRes.length > 0) {
        for (const inst of instRes) {
          try {
            const bks = await apiRequest<AndroidBackup[]>(`/android/instances/${inst.id}/backups`);
            if (bks) allBackups.push(...bks);
          } catch {
            // Ignore
          }
        }
      }
      setBackups(allBackups);
    } catch (err) {
      console.error('Failed to load Android cloud data:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedId(id);
    setTimeout(() => setCopiedId(null), 2000);
  };

  const handleStart = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      setActionLoading(`start-${id}`);
      await apiRequest(`/android/instances/${id}/start`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(`Failed to start Android instance: ${err.message || 'Error'}`);
    } finally {
      setActionLoading(null);
    }
  };

  const handleStop = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      setActionLoading(`stop-${id}`);
      await apiRequest(`/android/instances/${id}/stop`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(`Failed to stop Android instance: ${err.message || 'Error'}`);
    } finally {
      setActionLoading(null);
    }
  };

  const handleRestart = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      setActionLoading(`restart-${id}`);
      await apiRequest(`/android/instances/${id}/restart`, { method: 'POST' });
      await fetchData();
    } catch (err: any) {
      alert(`Failed to restart Android instance: ${err.message || 'Error'}`);
    } finally {
      setActionLoading(null);
    }
  };

  const filteredInstances = instances.filter((inst) => {
    const matchesSearch =
      inst.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      inst.plan_name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      inst.ip_address.toLowerCase().includes(searchTerm.toLowerCase());
    return matchesSearch;
  });

  // Calculate quick metrics
  const totalInstances = instances.length;
  const runningInstances = instances.filter((i) => i.status === 'running').length;
  const totalVCPUs = instances.reduce((acc, i) => acc + (i.vcpus || 0), 0);
  const totalMemoryGB = Math.round(instances.reduce((acc, i) => acc + (i.memory_mb || 0), 0) / 1024);
  const totalStorageGB = instances.reduce((acc, i) => acc + (i.storage_gb || 0), 0);

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-gradient-to-br from-emerald-500/20 to-teal-500/10 border border-emerald-500/30 text-emerald-400">
              <Smartphone className="w-6 h-6" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-white tracking-tight">Android Cloud (Waydroid)</h1>
              <p className="text-sm text-slate-400">
                Managed containerized Android environments with low-latency remote display streaming & ADB bridging
              </p>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={fetchData}
            disabled={loading}
            className="p-2 rounded-lg border border-slate-700 bg-slate-800/80 hover:bg-slate-700/80 text-slate-300 hover:text-white transition-colors"
            title="Refresh list"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>

          <button
            onClick={() => onNavigate('/dashboard/android/create')}
            className="flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white rounded-lg font-medium shadow-lg shadow-emerald-500/20 transition-all text-sm"
          >
            <PlusCircle className="w-4 h-4" />
            Create Android Instance
          </button>
        </div>
      </div>

      {/* AI Assistant Banner */}
      <div className="p-4 rounded-xl bg-gradient-to-r from-emerald-950/40 via-teal-950/20 to-slate-900 border border-emerald-500/30 flex flex-col md:flex-row items-start md:items-center justify-between gap-4">
        <div className="flex items-center gap-3.5">
          <div className="p-2 rounded-lg bg-emerald-500/20 border border-emerald-500/30 text-emerald-400 shrink-0">
            <Sparkles className="w-5 h-5" />
          </div>
          <div>
            <div className="text-sm font-semibold text-white">Need an automated Android QA or testing device?</div>
            <div className="text-xs text-slate-400">
              Prompt our AI Cloud Agent: <span className="text-emerald-400 font-mono">"Launch an Android 13 Waydroid instance with root and 4GB RAM"</span>
            </div>
          </div>
        </div>
        <button
          onClick={() => onNavigate('/dashboard/ai')}
          className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-emerald-600/20 hover:bg-emerald-600/30 border border-emerald-500/30 text-emerald-300 text-xs font-medium transition-colors shrink-0"
        >
          <span>Ask AI Agent</span>
          <ArrowRight className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* Resource Metrics Strip */}
      <div className="grid grid-cols-2 sm:grid-cols-5 gap-3">
        <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800/80">
          <div className="text-xs text-slate-400 font-medium">Total Instances</div>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white">{totalInstances}</span>
            <span className="text-xs text-emerald-400">{runningInstances} running</span>
          </div>
        </div>
        <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800/80">
          <div className="text-xs text-slate-400 font-medium">Allocated CPU</div>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white">{totalVCPUs}</span>
            <span className="text-xs text-slate-400">vCPUs</span>
          </div>
        </div>
        <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800/80">
          <div className="text-xs text-slate-400 font-medium">Allocated RAM</div>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white">{totalMemoryGB}</span>
            <span className="text-xs text-slate-400">GB</span>
          </div>
        </div>
        <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800/80">
          <div className="text-xs text-slate-400 font-medium">Persistent Storage</div>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white">{totalStorageGB}</span>
            <span className="text-xs text-slate-400">GB ZFS</span>
          </div>
        </div>
        <div className="p-3.5 rounded-xl bg-slate-900/60 border border-slate-800/80 col-span-2 sm:col-span-1">
          <div className="text-xs text-slate-400 font-medium">Backups Stored</div>
          <div className="mt-1 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white">{backups.length}</span>
            <span className="text-xs text-emerald-400">snapshots</span>
          </div>
        </div>
      </div>

      {/* Tabs and Search Bar */}
      <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3 border-b border-slate-800 pb-3">
        <div className="flex items-center gap-1 overflow-x-auto">
          <button
            onClick={() => setActiveTab('all')}
            className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors whitespace-nowrap ${
              activeTab === 'all'
                ? 'bg-emerald-500/20 border border-emerald-500/40 text-emerald-300'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            All Instances ({instances.length})
          </button>
          <button
            onClick={() => setActiveTab('images')}
            className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors whitespace-nowrap ${
              activeTab === 'images'
                ? 'bg-emerald-500/20 border border-emerald-500/40 text-emerald-300'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            Android Images ({images.length})
          </button>
          <button
            onClick={() => setActiveTab('apps')}
            className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors whitespace-nowrap ${
              activeTab === 'apps'
                ? 'bg-emerald-500/20 border border-emerald-500/40 text-emerald-300'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            Applications
          </button>
          <button
            onClick={() => setActiveTab('backups')}
            className={`px-3 py-1.5 rounded-lg text-xs font-medium transition-colors whitespace-nowrap ${
              activeTab === 'backups'
                ? 'bg-emerald-500/20 border border-emerald-500/40 text-emerald-300'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            Backups ({backups.length})
          </button>
        </div>

        <div className="relative min-w-[220px]">
          <Search className="w-4 h-4 text-slate-400 absolute left-3 top-1/2 -translate-y-1/2" />
          <input
            type="text"
            placeholder="Search instances, IP, plans..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full pl-9 pr-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-emerald-500"
          />
        </div>
      </div>

      {/* Main Tab Content */}
      {loading ? (
        <div className="p-12 text-center text-slate-400 bg-slate-900/40 rounded-xl border border-slate-800 flex flex-col items-center justify-center gap-3">
          <RefreshCw className="w-8 h-8 text-emerald-500 animate-spin" />
          <p className="text-sm">Loading Android instances and catalogs...</p>
        </div>
      ) : activeTab === 'all' ? (
        filteredInstances.length === 0 ? (
          <div className="p-12 text-center bg-slate-900/30 rounded-2xl border border-slate-800/80">
            <div className="w-16 h-16 mx-auto rounded-2xl bg-emerald-500/10 border border-emerald-500/20 flex items-center justify-center text-emerald-400 mb-4">
              <Smartphone className="w-8 h-8" />
            </div>
            <h3 className="text-base font-semibold text-white">No Android Instances Yet</h3>
            <p className="text-sm text-slate-400 max-w-md mx-auto mt-1 mb-6">
              Create your first cloud Android container to test APKs, run automation, or interact via browser display streaming.
            </p>
            <button
              onClick={() => onNavigate('/dashboard/android/create')}
              className="inline-flex items-center gap-2 px-4 py-2 bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white rounded-lg text-sm font-medium transition-all shadow-lg shadow-emerald-500/20"
            >
              <PlusCircle className="w-4 h-4" />
              Create Android Instance
            </button>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {filteredInstances.map((inst) => {
              const isRunning = inst.status === 'running';
              const isBusy = actionLoading?.includes(inst.id);

              return (
                <div
                  key={inst.id}
                  onClick={() => onNavigate(`/dashboard/android/${inst.id}`)}
                  className="group relative rounded-2xl bg-slate-900/80 hover:bg-slate-900 border border-slate-800/80 hover:border-emerald-500/50 p-5 transition-all cursor-pointer flex flex-col justify-between shadow-lg hover:shadow-emerald-500/5"
                >
                  <div>
                    {/* Header: Title & Status */}
                    <div className="flex items-start justify-between gap-3">
                      <div className="flex items-center gap-3">
                        <div className="p-2.5 rounded-xl bg-slate-800/90 border border-slate-700/80 text-emerald-400 group-hover:border-emerald-500/40 transition-colors">
                          <Smartphone className="w-5 h-5" />
                        </div>
                        <div>
                          <h3 className="text-base font-semibold text-white group-hover:text-emerald-300 transition-colors line-clamp-1">
                            {inst.name}
                          </h3>
                          <div className="text-xs text-slate-400 capitalize">
                            {inst.plan_name} plan • {inst.display_resolution}
                          </div>
                        </div>
                      </div>

                      <div className="flex items-center gap-1.5 shrink-0">
                        <span
                          className={`w-2 h-2 rounded-full ${
                            isRunning ? 'bg-emerald-400 animate-pulse' : 'bg-slate-500'
                          }`}
                        />
                        <span
                          className={`text-xs font-medium uppercase tracking-wider ${
                            isRunning ? 'text-emerald-400' : 'text-slate-400'
                          }`}
                        >
                          {inst.status}
                        </span>
                      </div>
                    </div>

                    {/* Hardware specs pills */}
                    <div className="mt-4 grid grid-cols-3 gap-2">
                      <div className="p-2 rounded-lg bg-slate-950/60 border border-slate-800 text-center">
                        <div className="text-[10px] text-slate-500 font-medium uppercase tracking-wider">CPU</div>
                        <div className="text-xs font-bold text-white mt-0.5">{inst.vcpus} vCPU</div>
                      </div>
                      <div className="p-2 rounded-lg bg-slate-950/60 border border-slate-800 text-center">
                        <div className="text-[10px] text-slate-500 font-medium uppercase tracking-wider">RAM</div>
                        <div className="text-xs font-bold text-white mt-0.5">
                          {Math.round((inst.memory_mb || 0) / 1024)} GB
                        </div>
                      </div>
                      <div className="p-2 rounded-lg bg-slate-950/60 border border-slate-800 text-center">
                        <div className="text-[10px] text-slate-500 font-medium uppercase tracking-wider">Disk</div>
                        <div className="text-xs font-bold text-white mt-0.5">{inst.storage_gb} GB</div>
                      </div>
                    </div>

                    {/* Connection info */}
                    <div className="mt-4 p-2.5 rounded-xl bg-slate-950/80 border border-slate-800/90 space-y-1.5 text-xs">
                      <div className="flex items-center justify-between text-slate-300">
                        <span className="text-slate-500">IP Endpoint:</span>
                        <div className="flex items-center gap-1.5 font-mono text-emerald-400">
                          <span>{inst.ip_address}</span>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleCopy(inst.ip_address, `ip-${inst.id}`);
                            }}
                            className="p-1 hover:text-white"
                          >
                            {copiedId === `ip-${inst.id}` ? (
                              <Check className="w-3 h-3 text-emerald-400" />
                            ) : (
                              <Copy className="w-3 h-3" />
                            )}
                          </button>
                        </div>
                      </div>

                      {inst.adb_enabled && (
                        <div className="flex items-center justify-between text-slate-300">
                          <span className="text-slate-500">ADB Port:</span>
                          <div className="flex items-center gap-1.5 font-mono text-teal-300">
                            <span>adb connect cloud.nevatal.id:{inst.adb_port || 5555}</span>
                            <button
                              type="button"
                              onClick={(e) => {
                                e.stopPropagation();
                                handleCopy(
                                  `adb connect cloud.nevatal.id:${inst.adb_port || 5555}`,
                                  `adb-${inst.id}`
                                );
                              }}
                              className="p-1 hover:text-white"
                            >
                              {copiedId === `adb-${inst.id}` ? (
                                <Check className="w-3 h-3 text-emerald-400" />
                              ) : (
                                <Copy className="w-3 h-3" />
                              )}
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Footer Actions */}
                  <div className="mt-5 pt-3 border-t border-slate-800/80 flex items-center justify-between gap-2">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        onNavigate(`/dashboard/android/${inst.id}`, 'console');
                      }}
                      className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-500/20 hover:bg-emerald-500/30 border border-emerald-500/30 text-emerald-300 text-xs font-medium transition-colors"
                    >
                      <ExternalLink className="w-3.5 h-3.5" />
                      <span>Remote Display</span>
                    </button>

                    <div className="flex items-center gap-1" onClick={(e) => e.stopPropagation()}>
                      {isRunning ? (
                        <>
                          <button
                            disabled={isBusy}
                            onClick={(e) => handleRestart(inst.id, e)}
                            className="p-1.5 rounded-lg text-slate-400 hover:text-amber-400 hover:bg-slate-800 transition-colors"
                            title="Restart Container"
                          >
                            <RotateCw className={`w-3.5 h-3.5 ${isBusy ? 'animate-spin' : ''}`} />
                          </button>
                          <button
                            disabled={isBusy}
                            onClick={(e) => handleStop(inst.id, e)}
                            className="p-1.5 rounded-lg text-slate-400 hover:text-red-400 hover:bg-slate-800 transition-colors"
                            title="Stop Instance"
                          >
                            <Square className="w-3.5 h-3.5" />
                          </button>
                        </>
                      ) : (
                        <button
                          disabled={isBusy}
                          onClick={(e) => handleStart(inst.id, e)}
                          className="p-1.5 rounded-lg text-slate-400 hover:text-emerald-400 hover:bg-slate-800 transition-colors"
                          title="Start Instance"
                        >
                          <Play className="w-3.5 h-3.5" />
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )
      ) : activeTab === 'images' ? (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {images.map((img) => (
            <div
              key={img.id}
              className="rounded-2xl bg-slate-900/80 border border-slate-800/80 p-5 flex flex-col justify-between"
            >
              <div>
                <div className="flex items-start justify-between gap-3">
                  <div className="flex items-center gap-3">
                    <div className="p-2.5 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-emerald-400">
                      <Layers className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="text-base font-semibold text-white">{img.name}</h3>
                      <div className="text-xs text-slate-400">
                        Android {img.android_version} • {img.architecture}
                      </div>
                    </div>
                  </div>
                  <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-300 border border-emerald-500/30 uppercase">
                    {img.image_type}
                  </span>
                </div>

                <p className="mt-3 text-xs text-slate-400 leading-relaxed">{img.description}</p>

                <div className="mt-4 pt-3 border-t border-slate-800/80 space-y-1.5 text-xs text-slate-400">
                  <div className="flex justify-between">
                    <span className="text-slate-500">Min Resources:</span>
                    <span className="text-white">
                      {img.min_cpu} vCPU / {Math.round(img.min_ram_mb / 1024)} GB RAM
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Image Source:</span>
                    <span className="text-slate-300 capitalize">{img.image_source}</span>
                  </div>
                </div>
              </div>

              <div className="mt-5 pt-3 border-t border-slate-800/80 flex items-center justify-between">
                <span className="text-[11px] text-emerald-400 font-medium flex items-center gap-1">
                  <Check className="w-3 h-3" /> Verified & Published
                </span>
                <button
                  onClick={() => onNavigate(`/dashboard/android/create?image=${img.id}`)}
                  className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-white text-xs font-medium transition-colors"
                >
                  Deploy Image
                </button>
              </div>
            </div>
          ))}
        </div>
      ) : activeTab === 'apps' ? (
        <div className="rounded-2xl bg-slate-900/80 border border-slate-800/80 overflow-hidden">
          <div className="p-4 border-b border-slate-800 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Installed Android Packages & APKs</h3>
            <span className="text-xs text-slate-400">Across running instances</span>
          </div>
          <div className="divide-y divide-slate-800/60">
            <div className="p-4 flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-blue-500/20 text-blue-400">
                  <Package className="w-5 h-5" />
                </div>
                <div>
                  <div className="text-sm font-medium text-white">Google Chrome</div>
                  <div className="text-xs text-slate-400 font-mono">com.android.chrome • v128.0.6613.88</div>
                </div>
              </div>
              <div className="text-right">
                <div className="text-xs text-white">85 MB</div>
                <div className="text-[10px] text-emerald-400">User Application</div>
              </div>
            </div>

            <div className="p-4 flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-emerald-500/20 text-emerald-400">
                  <Terminal className="w-5 h-5" />
                </div>
                <div>
                  <div className="text-sm font-medium text-white">Termux Terminal</div>
                  <div className="text-xs text-slate-400 font-mono">com.termux • v0.118.0</div>
                </div>
              </div>
              <div className="text-right">
                <div className="text-xs text-white">35 MB</div>
                <div className="text-[10px] text-emerald-400">User Application</div>
              </div>
            </div>

            <div className="p-4 flex items-center justify-between gap-4">
              <div className="flex items-center gap-3">
                <div className="p-2 rounded-lg bg-purple-500/20 text-purple-400">
                  <Activity className="w-5 h-5" />
                </div>
                <div>
                  <div className="text-sm font-medium text-white">Nevatal Mobile QA Companion</div>
                  <div className="text-xs text-slate-400 font-mono">com.nevatal.cloudagent • v1.0.4</div>
                </div>
              </div>
              <div className="text-right">
                <div className="text-xs text-white">12 MB</div>
                <div className="text-[10px] text-purple-400">QA Diagnostic Agent</div>
              </div>
            </div>
          </div>
        </div>
      ) : (
        /* Backups Tab */
        <div className="rounded-2xl bg-slate-900/80 border border-slate-800/80 overflow-hidden">
          <div className="p-4 border-b border-slate-800 flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Android Container Data Snapshots</h3>
            <span className="text-xs text-slate-400">{backups.length} snapshots available</span>
          </div>

          {backups.length === 0 ? (
            <div className="p-8 text-center text-slate-400 text-xs">
              No backups created yet. You can snapshot user data and apps from an instance's details page.
            </div>
          ) : (
            <div className="divide-y divide-slate-800/60">
              {backups.map((bk) => (
                <div key={bk.id} className="p-4 flex items-center justify-between gap-4">
                  <div className="flex items-center gap-3">
                    <div className="p-2 rounded-lg bg-slate-800 text-emerald-400">
                      <Database className="w-4 h-4" />
                    </div>
                    <div>
                      <div className="text-sm font-medium text-white">{bk.backup_name}</div>
                      <div className="text-xs text-slate-400">
                        Type: {bk.backup_type} • Created: {new Date(bk.created_at).toLocaleString()}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-4">
                    <div className="text-right">
                      <div className="text-xs text-white">
                        {Math.round(bk.file_size_bytes / 1024 / 1024)} MB
                      </div>
                      <div className="text-[10px] text-emerald-400 uppercase tracking-wider">
                        {bk.status}
                      </div>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
};
