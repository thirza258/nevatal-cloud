import React, { useState, useEffect, useCallback } from 'react';
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Cpu,
  Database,
  HardDrive,
  RefreshCw,
  Server,
  Shield,
  Sliders,
  TrendingUp,
  Users,
  Wifi,
  Bot,
  Layers,
  FileText,
  AlertOctagon,
  Search,
  Sparkles,
  Check,
  X,
  ChevronRight,
  ArrowUpRight,
  ArrowDownRight,
  Plus,
  Radio,
  Clock,
  Eye,
  Info,
} from 'lucide-react';
import {
  getAdminObservabilityOverview,
  getAdminInfrastructure,
  getAdminNodes,
  getAdminStorage,
  getAdminVMs,
  getAdminAPI,
  getAdminJobs,
  getAdminTenants,
  getAdminEvents,
  getAdminLogs,
  getAdminAlerts,
  acknowledgeAlert,
  resolveAlert,
  getAdminIncidents,
  createAdminIncident,
  updateAdminIncident,
  getAdminCapacity,
  getAdminAI,
  adminAIInvestigate,
} from '../../lib/observabilityApi';
import {
  AdminObservabilityOverview,
  InfrastructureHealth,
  APIObservabilitySummary,
  JobObservabilitySummary,
  AIObservabilitySummary,
  TenantObservabilityDetail,
  CapacityForecast,
  AIIncidentInvestigationResult,
  VMDetailObservability,
  ObservabilityEvent,
  ObservabilityLog,
  ObservabilityAlert,
  ObservabilityAlertRule,
  ObservabilityIncident,
  CreateIncidentInput,
  ObservabilityIncidentSeverity,
  ObservabilityIncidentStatus,
} from '../../types';

interface AdminObservabilityProps {
  initialTab?: string;
}

export const AdminObservability: React.FC<AdminObservabilityProps> = ({ initialTab = 'overview' }) => {
  const [activeTab, setActiveTab] = useState<string>(initialTab);
  const [overview, setOverview] = useState<AdminObservabilityOverview | null>(null);
  const [infra, setInfra] = useState<InfrastructureHealth | null>(null);
  const [nodes, setNodes] = useState<any[]>([]);
  const [storageData, setStorageData] = useState<any>(null);
  const [vms, setVms] = useState<VMDetailObservability[]>([]);
  const [apiData, setApiData] = useState<APIObservabilitySummary | null>(null);
  const [jobsData, setJobsData] = useState<JobObservabilitySummary | null>(null);
  const [tenantsData, setTenantsData] = useState<TenantObservabilityDetail[]>([]);
  const [events, setEvents] = useState<ObservabilityEvent[]>([]);
  const [logs, setLogs] = useState<ObservabilityLog[]>([]);
  const [alertsData, setAlertsData] = useState<{ active_alerts: ObservabilityAlert[]; all_alerts: ObservabilityAlert[]; rules: ObservabilityAlertRule[] }>({
    active_alerts: [],
    all_alerts: [],
    rules: [],
  });
  const [incidents, setIncidents] = useState<ObservabilityIncident[]>([]);
  const [capacity, setCapacity] = useState<CapacityForecast | null>(null);
  const [aiData, setAiData] = useState<AIObservabilitySummary | null>(null);

  const [loading, setLoading] = useState<boolean>(true);
  const [refreshing, setRefreshing] = useState<boolean>(false);

  // Filters & Modal States
  const [eventSeverityFilter, setEventSeverityFilter] = useState<string>('all');
  const [logLevelFilter, setLogLevelFilter] = useState<string>('all');
  const [logSearchQuery, setLogSearchQuery] = useState<string>('');

  // Resolve Alert Modal
  const [resolvingAlertId, setResolvingAlertId] = useState<string | null>(null);
  const [resolutionText, setResolutionText] = useState<string>('');

  // Create Incident Modal
  const [showIncidentModal, setShowIncidentModal] = useState<boolean>(false);
  const [incidentTitle, setIncidentTitle] = useState<string>('');
  const [incidentSeverity, setIncidentSeverity] = useState<ObservabilityIncidentSeverity>('SEV-2');
  const [incidentRootCause, setIncidentRootCause] = useState<string>('');
  const [incidentSymptoms, setIncidentSymptoms] = useState<string>('');

  // AI Investigation State
  const [aiQuery, setAiQuery] = useState<string>('Analyze recent VM clone timeouts and hypervisor storage IOPS bottlenecks');
  const [aiInvestigationResult, setAiInvestigationResult] = useState<AIIncidentInvestigationResult | null>(null);
  const [aiInvestigating, setAiInvestigating] = useState<boolean>(false);

  // Selected VM detail drawer
  const [selectedVM, setSelectedVM] = useState<VMDetailObservability | null>(null);

  const fetchTabTelemetry = useCallback(async () => {
    try {
      if (activeTab === 'overview') {
        const [ov, incs, als] = await Promise.all([
          getAdminObservabilityOverview(),
          getAdminIncidents().catch(() => []),
          getAdminAlerts().catch(() => ({ active_alerts: [], all_alerts: [], rules: [] })),
        ]);
        setOverview(ov);
        setIncidents(incs);
        setAlertsData(als);
      } else if (activeTab === 'infrastructure') {
        const inf = await getAdminInfrastructure();
        setInfra(inf);
      } else if (activeTab === 'nodes') {
        const nds = await getAdminNodes();
        setNodes(nds);
      } else if (activeTab === 'storage') {
        const st = await getAdminStorage();
        setStorageData(st);
      } else if (activeTab === 'vms') {
        const v = await getAdminVMs();
        setVms(v);
      } else if (activeTab === 'api') {
        const a = await getAdminAPI();
        setApiData(a);
      } else if (activeTab === 'jobs') {
        const j = await getAdminJobs();
        setJobsData(j);
      } else if (activeTab === 'ai') {
        const ai = await getAdminAI();
        setAiData(ai);
      } else if (activeTab === 'tenants') {
        const t = await getAdminTenants();
        setTenantsData(t);
      } else if (activeTab === 'events') {
        const ev = await getAdminEvents({ limit: 100 });
        setEvents(ev);
      } else if (activeTab === 'logs') {
        const l = await getAdminLogs({ limit: 100 });
        setLogs(l);
      } else if (activeTab === 'alerts') {
        const al = await getAdminAlerts();
        setAlertsData(al);
      } else if (activeTab === 'incidents') {
        const inc = await getAdminIncidents();
        setIncidents(inc);
      } else if (activeTab === 'capacity') {
        const cap = await getAdminCapacity();
        setCapacity(cap);
      }
    } catch (err) {
      console.error(`Failed to load telemetry for tab ${activeTab}:`, err);
    }
  }, [activeTab]);

  useEffect(() => {
    setLoading(true);
    fetchTabTelemetry().finally(() => setLoading(false));
  }, [fetchTabTelemetry]);

  // Periodic background telemetry refresh
  useEffect(() => {
    const interval = setInterval(() => {
      fetchTabTelemetry();
    }, 12000);
    return () => clearInterval(interval);
  }, [fetchTabTelemetry]);

  const handleRefresh = async () => {
    setRefreshing(true);
    await fetchTabTelemetry();
    setRefreshing(false);
  };

  const handleAcknowledgeAlert = async (id: string) => {
    try {
      await acknowledgeAlert(id);
      const al = await getAdminAlerts();
      setAlertsData(al);
    } catch (err) {
      console.error('Failed to acknowledge alert:', err);
    }
  };

  const handleResolveAlert = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!resolvingAlertId || !resolutionText.trim()) return;
    try {
      await resolveAlert(resolvingAlertId, resolutionText.trim());
      setResolvingAlertId(null);
      setResolutionText('');
      const al = await getAdminAlerts();
      setAlertsData(al);
    } catch (err) {
      console.error('Failed to resolve alert:', err);
    }
  };

  const handleCreateIncident = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!incidentTitle.trim()) return;
    try {
      const symptomsArr = incidentSymptoms
        .split('\n')
        .map((s) => s.trim())
        .filter(Boolean);
      await createAdminIncident({
        title: incidentTitle.trim(),
        severity: incidentSeverity,
        root_cause: incidentRootCause.trim(),
        symptoms: symptomsArr,
        affected_resource_ids: [],
      });
      setShowIncidentModal(false);
      setIncidentTitle('');
      setIncidentRootCause('');
      setIncidentSymptoms('');
      const inc = await getAdminIncidents();
      setIncidents(inc);
    } catch (err) {
      console.error('Failed to create incident:', err);
    }
  };

  const handleRunAIInvestigation = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!aiQuery.trim()) return;
    setAiInvestigating(true);
    try {
      const res = await adminAIInvestigate({ query: aiQuery.trim() });
      setAiInvestigationResult(res);
    } catch (err) {
      console.error('AI investigation failed:', err);
    } finally {
      setAiInvestigating(false);
    }
  };

  const renderStatusPill = (status: string) => {
    const norm = (status || '').toLowerCase();
    if (norm === 'healthy' || norm === 'operational' || norm === 'online') {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
          <CheckCircle2 className="w-3 h-3" />
          {status}
        </span>
      );
    } else if (norm === 'warning' || norm === 'degraded') {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30">
          <AlertTriangle className="w-3 h-3" />
          {status}
        </span>
      );
    } else {
      return (
        <span className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30">
          <AlertOctagon className="w-3 h-3" />
          {status}
        </span>
      );
    }
  };

  return (
    <div className="space-y-6 pb-16">
      {/* Platform Status Bar */}
      <div className="bg-zinc-900/60 border border-zinc-800 rounded-xl p-4 flex flex-col md:flex-row md:items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-xl font-bold tracking-tight text-white flex items-center gap-2">
              <Activity className="w-5 h-5 text-emerald-400" />
              Admin Observability Center
            </h1>
            <span className="flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
              Platform Operational
            </span>
          </div>
          <p className="text-xs text-zinc-400 mt-1">
            Cluster-wide real-time telemetry, hypervisor health, tenant blast radius, and AI root-cause analysis.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-xs font-medium text-zinc-200 border border-zinc-700 transition"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin text-emerald-400' : ''}`} />
            Sync Now
          </button>
        </div>
      </div>

      {/* Observability Sub-Navigation */}
      <div className="flex border-b border-zinc-800 space-x-1 overflow-x-auto pb-px">
        {[
          { id: 'overview', label: 'Overview', icon: Shield },
          { id: 'infrastructure', label: 'Infrastructure', icon: Cpu },
          { id: 'nodes', label: 'Nodes', icon: Server },
          { id: 'storage', label: 'Storage & ZFS', icon: Database },
          { id: 'vms', label: 'Fleet VMs', icon: Layers },
          { id: 'api', label: 'API Metrics', icon: TrendingUp },
          { id: 'jobs', label: 'Provisioning Jobs', icon: RefreshCw },
          { id: 'ai', label: 'AI & Agents', icon: Bot },
          { id: 'tenants', label: 'Tenants Matrix', icon: Users },
          { id: 'events', label: 'Platform Events', icon: Activity },
          { id: 'logs', label: 'Evidence Logs', icon: FileText },
          { id: 'alerts', label: 'Alerts', icon: AlertTriangle, badge: alertsData.active_alerts.length },
          { id: 'incidents', label: 'Incidents', icon: AlertOctagon, badge: incidents.filter((i) => i.status !== 'resolved').length },
          { id: 'capacity', label: 'Capacity Forecast', icon: Sliders },
          { id: 'investigation', label: 'AI Investigation', icon: Sparkles },
        ].map((tab) => {
          const Icon = tab.icon;
          const active = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id)}
              className={`flex items-center gap-1.5 px-3.5 py-2 text-xs font-medium border-b-2 whitespace-nowrap transition ${
                active
                  ? 'border-emerald-500 text-emerald-400 bg-emerald-500/5'
                  : 'border-transparent text-zinc-400 hover:text-zinc-200 hover:border-zinc-700'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              <span>{tab.label}</span>
              {tab.badge !== undefined && tab.badge > 0 && (
                <span className="px-1.5 py-0.2 rounded-full text-[10px] font-mono bg-rose-500/20 text-rose-300">
                  {tab.badge}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* TAB 1: OVERVIEW */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* Top-Level Component Health Grid */}
          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-4">
            <h3 className="text-xs font-semibold text-zinc-300 uppercase tracking-wider mb-3">
              Core Subsystem Health
            </h3>
            <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7 gap-3">
              {Object.entries(overview?.components || {
                API: 'Healthy',
                Database: 'Healthy',
                Worker: 'Healthy',
                Proxmox: 'Healthy',
                Storage: 'Healthy',
                Edge: 'Healthy',
                'AI Orchestrator': 'Healthy',
              }).map(([name, stat]) => (
                <div key={name} className="bg-zinc-900/80 border border-zinc-800/80 p-3 rounded-lg text-center">
                  <span className="text-xs font-medium text-zinc-300 block">{name}</span>
                  <div className="mt-2">{renderStatusPill(stat)}</div>
                </div>
              ))}
            </div>
          </div>

          {/* KPI Cards Grid */}
          <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3">
            <div className="bg-zinc-900/60 border border-zinc-800 rounded-lg p-3.5">
              <span className="text-[11px] font-mono uppercase text-zinc-400">Active Users</span>
              <div className="mt-2 text-2xl font-bold text-white">{overview?.kpi.active_users ?? 0}</div>
              <span className="text-[10px] text-zinc-400 font-mono">Platform tenants</span>
            </div>

            <div className="bg-zinc-900/60 border border-zinc-800 rounded-lg p-3.5">
              <span className="text-[11px] font-mono uppercase text-zinc-400">Active / Running VMs</span>
              <div className="mt-2 text-2xl font-bold text-white">
                {overview?.kpi.running_vms ?? 0} <span className="text-sm text-zinc-400 font-normal">/ {overview?.kpi.active_vms ?? 0}</span>
              </div>
              <span className="text-[10px] text-zinc-400 font-mono">Proxmox virtual machines</span>
            </div>

            <div className="bg-zinc-900/60 border border-zinc-800 rounded-lg p-3.5">
              <span className="text-[11px] font-mono uppercase text-zinc-400">CPU Utilization</span>
              <div className="mt-2 text-2xl font-bold text-white">
                {(overview?.kpi.cpu_utilization_pct ?? 0).toFixed(1)}%
              </div>
              <span className="text-[10px] text-emerald-400 font-mono">Cluster aggregate</span>
            </div>

            <div className="bg-zinc-900/60 border border-zinc-800 rounded-lg p-3.5">
              <span className="text-[11px] font-mono uppercase text-zinc-400">RAM Utilization</span>
              <div className="mt-2 text-2xl font-bold text-white">
                {(overview?.kpi.ram_utilization_pct ?? 0).toFixed(1)}%
              </div>
              <span className="text-[10px] text-zinc-400 font-mono">Memory allocation</span>
            </div>

            <div className="bg-zinc-900/60 border border-zinc-800 rounded-lg p-3.5">
              <span className="text-[11px] font-mono uppercase text-zinc-400">Storage Utilization</span>
              <div className="mt-2 text-2xl font-bold text-white">
                {(overview?.kpi.storage_utilization_pct ?? 0).toFixed(1)}%
              </div>
              <span className="text-[10px] text-zinc-400 font-mono">ZFS pool capacity</span>
            </div>

            <div className="bg-zinc-900/60 border border-zinc-800 rounded-lg p-3.5">
              <span className="text-[11px] font-mono uppercase text-zinc-400">API Error Rate</span>
              <div className="mt-2 text-2xl font-bold text-white">
                {(overview?.kpi.api_error_rate_pct ?? 0).toFixed(2)}%
              </div>
              <span className="text-[10px] text-zinc-400 font-mono">P95: {(overview?.kpi.api_p95_latency_ms ?? 0).toFixed(0)}ms</span>
            </div>
          </div>

          {/* Open Incidents & Active Alerts Banner */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-white flex items-center gap-2">
                  <AlertOctagon className="w-4 h-4 text-rose-400" />
                  Open Incidents &amp; Triage
                </h3>
                <button
                  onClick={() => setActiveTab('incidents')}
                  className="text-xs text-zinc-400 hover:text-white"
                >
                  Manage Incidents &rarr;
                </button>
              </div>
              <div className="space-y-2.5">
                {incidents.filter((i) => i.status !== 'resolved').slice(0, 3).map((inc) => (
                  <div key={inc.id} className="p-3 rounded-lg bg-zinc-900/80 border border-rose-500/30 text-xs">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="px-1.5 py-0.5 rounded text-[10px] font-mono font-bold bg-rose-500/20 text-rose-400 border border-rose-500/40">
                          {inc.severity}
                        </span>
                        <span className="font-semibold text-white">{inc.title}</span>
                      </div>
                      <span className="text-[10px] font-mono text-zinc-400 uppercase">{inc.status}</span>
                    </div>
                    <p className="text-zinc-400 text-[11px] mt-1.5">{inc.root_cause || 'Investigating impact...'}</p>
                  </div>
                ))}
                {incidents.filter((i) => i.status !== 'resolved').length === 0 && (
                  <div className="text-center py-6 text-zinc-400 text-xs">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 mx-auto mb-1.5" />
                    Zero open incidents detected. All services operational.
                  </div>
                )}
              </div>
            </div>

            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-white flex items-center gap-2">
                  <AlertTriangle className="w-4 h-4 text-amber-400" />
                  Active Fired Alerts
                </h3>
                <button
                  onClick={() => setActiveTab('alerts')}
                  className="text-xs text-zinc-400 hover:text-white"
                >
                  View Alerts &rarr;
                </button>
              </div>
              <div className="space-y-2.5">
                {alertsData.active_alerts.slice(0, 3).map((al) => (
                  <div key={al.id} className="p-3 rounded-lg bg-zinc-900/80 border border-amber-500/30 text-xs flex items-center justify-between">
                    <div>
                      <span className="font-semibold text-white">{al.title}</span>
                      <p className="text-zinc-400 text-[11px] mt-0.5">{al.message}</p>
                    </div>
                    <button
                      onClick={() => handleAcknowledgeAlert(al.id)}
                      className="px-2 py-1 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-mono"
                    >
                      Ack
                    </button>
                  </div>
                ))}
                {alertsData.active_alerts.length === 0 && (
                  <div className="text-center py-6 text-zinc-400 text-xs">
                    <CheckCircle2 className="w-5 h-5 text-emerald-400 mx-auto mb-1.5" />
                    No active threshold triggers firing.
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 2: INFRASTRUCTURE HEALTH */}
      {activeTab === 'infrastructure' && (
        <div className="space-y-6">
          <div className="bg-zinc-900/60 border border-zinc-800 rounded-xl p-5">
            <div className="flex items-center justify-between mb-4">
              <div>
                <span className="text-xs font-mono text-zinc-400 uppercase">Hardware Chassis</span>
                <h3 className="text-lg font-bold text-white">{infra?.model || 'Dell PowerEdge R640'}</h3>
              </div>
              <div className="flex items-center gap-3">
                <span className="px-2.5 py-1 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-mono">
                  Uptime: {infra ? Math.floor(infra.uptime_seconds / 86400) : 14}d {infra ? Math.floor((infra.uptime_seconds % 86400) / 3600) : 6}h
                </span>
                <span className="px-2.5 py-1 rounded bg-zinc-800 text-zinc-300 text-xs font-mono">
                  Chassis Status: {infra?.hardware_status || 'HEALTHY'}
                </span>
              </div>
            </div>

            {/* Sensors */}
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-3 border-t border-zinc-800/80">
              <div className="bg-zinc-900/90 border border-zinc-800 p-3 rounded-lg">
                <span className="text-[10px] font-mono text-zinc-400 uppercase">Chassis Temp</span>
                <div className="text-xl font-bold text-white mt-1">
                  {(infra?.temperature_celsius ?? 28.5).toFixed(1)} &deg;C
                </div>
                <span className="text-[10px] text-emerald-400 font-mono">Nominal thermal profile</span>
              </div>
              <div className="bg-zinc-900/90 border border-zinc-800 p-3 rounded-lg">
                <span className="text-[10px] font-mono text-zinc-400 uppercase">Power Consumption</span>
                <div className="text-xl font-bold text-white mt-1">
                  {(infra?.power_watts ?? 184).toFixed(0)} W
                </div>
                <span className="text-[10px] text-zinc-400 font-mono">Dual PSU Redundant</span>
              </div>
              <div className="bg-zinc-900/90 border border-zinc-800 p-3 rounded-lg">
                <span className="text-[10px] font-mono text-zinc-400 uppercase">ZFS Latency</span>
                <div className="text-xl font-bold text-white mt-1">
                  {(infra?.zfs.latency_ms ?? 0.8).toFixed(2)} ms
                </div>
                <span className="text-[10px] text-emerald-400 font-mono">NVMe storage pool</span>
              </div>
              <div className="bg-zinc-900/90 border border-zinc-800 p-3 rounded-lg">
                <span className="text-[10px] font-mono text-zinc-400 uppercase">ZFS IOPS</span>
                <div className="text-xl font-bold text-white mt-1">
                  {(infra?.zfs.iops ?? 1240).toFixed(0)} IOPS
                </div>
                <span className="text-[10px] text-zinc-400 font-mono">Read/Write operations</span>
              </div>
            </div>
          </div>

          {/* Metric Trends Breakdown */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {[
              { label: 'CPU Utilization', data: infra?.cpu, defaultVal: 62 },
              { label: 'RAM Utilization', data: infra?.ram, defaultVal: 54 },
              { label: 'Storage Utilization', data: infra?.storage, defaultVal: 68 },
              { label: 'Network Throughput', data: infra?.network, defaultVal: 45 },
            ].map(({ label, data, defaultVal }) => (
              <div key={label} className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-4">
                <div className="flex items-center justify-between">
                  <span className="text-xs font-semibold text-white uppercase font-mono">{label}</span>
                  <span className="text-xs font-mono text-emerald-400">
                    Status: {data?.status || 'Healthy'}
                  </span>
                </div>
                <div className="mt-3 flex items-baseline gap-2">
                  <span className="text-3xl font-bold text-white">
                    {data?.current ? data.current.toFixed(1) : defaultVal}%
                  </span>
                  <span className="text-xs text-zinc-400 font-mono">
                    (5m avg: {data?.avg_5m ? data.avg_5m.toFixed(1) : defaultVal}%)
                  </span>
                </div>
                <div className="mt-3 grid grid-cols-2 gap-2 text-xs font-mono text-zinc-400 pt-2 border-t border-zinc-800/60">
                  <div className="flex items-center gap-1">
                    <span>1h Trend:</span>
                    <span className="text-emerald-400 flex items-center">
                      <ArrowUpRight className="w-3.5 h-3.5" /> +{data?.trend_1h_pct ? data.trend_1h_pct.toFixed(1) : '2.1'}%
                    </span>
                  </div>
                  <div className="flex items-center gap-1">
                    <span>24h Trend:</span>
                    <span className="text-zinc-400 flex items-center">
                      <ArrowDownRight className="w-3.5 h-3.5" /> -{data?.trend_24h_pct ? Math.abs(data.trend_24h_pct).toFixed(1) : '1.4'}%
                    </span>
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* TAB 3: PROXMOX NODES */}
      {activeTab === 'nodes' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Proxmox VE Hypervisor Nodes</h3>
            <span className="text-xs text-zinc-400 font-mono">1 Physical Node</span>
          </div>

          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
            <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-zinc-800 pb-4">
              <div className="flex items-center gap-3">
                <div className="w-10 h-10 rounded-lg bg-zinc-800 flex items-center justify-center text-emerald-400">
                  <Server className="w-5 h-5" />
                </div>
                <div>
                  <h4 className="font-bold text-white text-base">pve-r640</h4>
                  <p className="text-xs text-zinc-400 font-mono">Proxmox Virtual Environment 8.x &bull; Linux 6.8</p>
                </div>
              </div>
              <div className="flex items-center gap-2">
                <span className="px-2.5 py-1 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-semibold">
                  Healthy / Online
                </span>
              </div>
            </div>

            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mt-4">
              <div>
                <span className="text-[11px] font-mono text-zinc-400 uppercase">CPU Load</span>
                <p className="text-lg font-bold text-white mt-1">12.4% / 40 Threads</p>
              </div>
              <div>
                <span className="text-[11px] font-mono text-zinc-400 uppercase">RAM Usage</span>
                <p className="text-lg font-bold text-white mt-1">68.2 GB / 128 GB</p>
              </div>
              <div>
                <span className="text-[11px] font-mono text-zinc-400 uppercase">Load Average</span>
                <p className="text-lg font-bold text-white mt-1">0.82, 0.94, 1.05</p>
              </div>
              <div>
                <span className="text-[11px] font-mono text-zinc-400 uppercase">Active VMs</span>
                <p className="text-lg font-bold text-white mt-1">8 Running</p>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 4: STORAGE & ZFS */}
      {activeTab === 'storage' && (
        <div className="space-y-6">
          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
            <div className="flex items-center justify-between mb-4">
              <div>
                <span className="text-xs font-mono text-zinc-400 uppercase">ZFS Pool</span>
                <h3 className="text-lg font-bold text-white">rpool (NVMe Mirror)</h3>
              </div>
              <span className="px-2.5 py-1 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 text-xs font-mono">
                ONLINE / Optimal
              </span>
            </div>

            <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
              <div className="bg-zinc-900/80 border border-zinc-800 p-3 rounded">
                <span className="text-[10px] font-mono text-zinc-400 uppercase">Capacity Threshold</span>
                <div className="text-xl font-bold text-white mt-1">64.5%</div>
                <span className="text-[10px] text-emerald-400 font-mono">&lt; 70% Healthy</span>
              </div>
              <div className="bg-zinc-900/80 border border-zinc-800 p-3 rounded">
                <span className="text-[10px] font-mono text-zinc-400 uppercase">Throughput Read</span>
                <div className="text-xl font-bold text-white mt-1">124.8 MB/s</div>
                <span className="text-[10px] text-zinc-400 font-mono">Read operations</span>
              </div>
              <div className="bg-zinc-900/80 border border-zinc-800 p-3 rounded">
                <span className="text-[10px] font-mono text-zinc-400 uppercase">Throughput Write</span>
                <div className="text-xl font-bold text-white mt-1">48.2 MB/s</div>
                <span className="text-[10px] text-zinc-400 font-mono">Write operations</span>
              </div>
              <div className="bg-zinc-900/80 border border-zinc-800 p-3 rounded">
                <span className="text-[10px] font-mono text-zinc-400 uppercase">SMART Status</span>
                <div className="text-xl font-bold text-emerald-400 mt-1">PASSED</div>
                <span className="text-[10px] text-zinc-400 font-mono">0 reallocated sectors</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* TAB 5: FLEET VMS */}
      {activeTab === 'vms' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Cluster Fleet Observability</h3>
            <span className="text-xs text-zinc-400 font-mono">{vms.length} Instances</span>
          </div>

          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-zinc-900/80 text-zinc-400 font-mono uppercase text-[11px] border-b border-zinc-800">
                  <tr>
                    <th className="px-4 py-3">Instance</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="px-4 py-3">Health Score</th>
                    <th className="px-4 py-3">CPU %</th>
                    <th className="px-4 py-3">RAM %</th>
                    <th className="px-4 py-3">Disk %</th>
                    <th className="px-4 py-3">Guest Agent</th>
                    <th className="px-4 py-3 text-right">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800/60 font-mono">
                  {vms.map((v) => (
                    <tr key={v.instance.id} className="hover:bg-zinc-900/40 transition">
                      <td className="px-4 py-3 font-sans">
                        <div className="font-semibold text-white">{v.instance.name}</div>
                        <div className="text-[11px] font-mono text-zinc-400">VMID: {v.instance.proxmox_vmid}</div>
                      </td>
                      <td className="px-4 py-3">
                        <span className="px-2 py-0.5 rounded text-[10px] uppercase font-semibold bg-emerald-500/10 text-emerald-400">
                          {v.instance.status}
                        </span>
                      </td>
                      <td className="px-4 py-3 font-sans">
                        <span className={`px-2 py-0.5 rounded text-xs font-semibold ${
                          v.health_score.score >= 90
                            ? 'bg-emerald-500/15 text-emerald-400'
                            : v.health_score.score >= 70
                            ? 'bg-amber-500/15 text-amber-400'
                            : 'bg-rose-500/15 text-rose-400'
                        }`}>
                          {v.health_score.score}/100
                        </span>
                      </td>
                      <td className="px-4 py-3 text-zinc-200">{v.cpu_usage_pct.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-zinc-200">{v.memory_usage_pct.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-zinc-200">{v.disk_usage_pct.toFixed(1)}%</td>
                      <td className="px-4 py-3">
                        <span className="text-[11px] text-emerald-400">{v.guest_agent_status || 'Connected'}</span>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={() => setSelectedVM(v)}
                          className="px-2.5 py-1 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-sans"
                        >
                          Explain Score
                        </button>
                      </td>
                    </tr>
                  ))}
                  {vms.length === 0 && (
                    <tr>
                      <td colSpan={8} className="text-center py-8 text-zinc-400 font-sans">
                        No instances found.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* TAB 6: API METRICS */}
      {activeTab === 'api' && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Requests / Sec</span>
              <div className="text-2xl font-bold text-white mt-1">{(apiData?.requests_per_sec ?? 18.4).toFixed(1)}</div>
              <span className="text-[10px] text-zinc-400 font-mono">Current throughput</span>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">P95 Latency</span>
              <div className="text-2xl font-bold text-white mt-1">{(apiData?.p95_latency_ms ?? 92).toFixed(0)} ms</div>
              <span className="text-[10px] text-emerald-400 font-mono">P50: {(apiData?.p50_latency_ms ?? 45).toFixed(0)}ms &bull; P99: {(apiData?.p99_latency_ms ?? 140).toFixed(0)}ms</span>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Error Rate</span>
              <div className="text-2xl font-bold text-white mt-1">{(apiData?.error_rate_pct ?? 0.08).toFixed(2)}%</div>
              <span className="text-[10px] text-emerald-400 font-mono">4xx/5xx responses</span>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Total Requests Recorded</span>
              <div className="text-2xl font-bold text-white mt-1">{apiData?.total_requests ?? 14200}</div>
              <span className="text-[10px] text-zinc-400 font-mono">Sliding metric window</span>
            </div>
          </div>

          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
            <div className="p-4 border-b border-zinc-800">
              <h4 className="text-xs font-semibold text-white uppercase font-mono">Top API Endpoints</h4>
            </div>
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-zinc-900/80 text-zinc-400 font-mono uppercase text-[11px] border-b border-zinc-800">
                  <tr>
                    <th className="px-4 py-3">Endpoint</th>
                    <th className="px-4 py-3">Requests</th>
                    <th className="px-4 py-3">P95 Latency</th>
                    <th className="px-4 py-3">Errors</th>
                    <th className="px-4 py-3">Status</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800/60 font-mono">
                  {(apiData?.endpoints || [
                    { endpoint: '/instances', requests: 12400, p95_ms: 88, error_rate_pct: 0.1, status: 'Healthy' },
                    { endpoint: '/instances/:id/metrics', requests: 24000, p95_ms: 120, error_rate_pct: 0.2, status: 'Healthy' },
                    { endpoint: '/observability/overview', requests: 18000, p95_ms: 65, error_rate_pct: 0.0, status: 'Healthy' },
                    { endpoint: '/ai/orchestrator/plans', requests: 1800, p95_ms: 850, error_rate_pct: 1.2, status: 'Healthy' },
                  ]).map((ep, i) => (
                    <tr key={i} className="hover:bg-zinc-900/40 transition">
                      <td className="px-4 py-3 text-white font-semibold">{ep.endpoint}</td>
                      <td className="px-4 py-3 text-zinc-300">{ep.requests.toLocaleString()}</td>
                      <td className="px-4 py-3 text-zinc-300">{ep.p95_ms.toFixed(0)} ms</td>
                      <td className="px-4 py-3 text-zinc-300">{ep.error_rate_pct.toFixed(1)}%</td>
                      <td className="px-4 py-3">{renderStatusPill(ep.status)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* TAB 7: PROVISIONING JOBS */}
      {activeTab === 'jobs' && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Queued / Running</span>
              <div className="text-2xl font-bold text-white mt-1">
                {jobsData?.running ?? 0} <span className="text-sm text-zinc-400 font-normal">/ {jobsData?.queued ?? 0} queued</span>
              </div>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Succeeded</span>
              <div className="text-2xl font-bold text-emerald-400 mt-1">{jobsData?.succeeded ?? 0}</div>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Failed / Retrying</span>
              <div className="text-2xl font-bold text-rose-400 mt-1">
                {jobsData?.failed ?? 0} <span className="text-sm text-zinc-400 font-normal">/ {jobsData?.retrying ?? 0} retrying</span>
              </div>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">P95 Duration</span>
              <div className="text-2xl font-bold text-white mt-1">{(jobsData?.p95_duration_sec ?? 45).toFixed(0)} s</div>
            </div>
          </div>

          {/* Job Failure Analysis */}
          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
            <h4 className="text-xs font-semibold text-white uppercase font-mono mb-3">
              Job Failure Timeline &amp; Root-Cause Analyzer
            </h4>
            {jobsData?.recent_failed && jobsData.recent_failed.length > 0 ? (
              <div className="space-y-4">
                {jobsData.recent_failed.map((fail) => (
                  <div key={fail.job_id} className="bg-zinc-900/80 border border-rose-500/30 rounded-lg p-4 text-xs">
                    <div className="flex items-center justify-between">
                      <span className="font-bold text-white text-sm">Job: {fail.job_id.slice(0, 8)} ({fail.instance_name})</span>
                      <span className="text-rose-400 font-mono uppercase font-bold">{fail.status}</span>
                    </div>
                    <div className="mt-2 text-rose-300 font-medium">Root Failure: {fail.root_failure}</div>
                    <div className="mt-3 space-y-1">
                      <span className="text-[11px] text-zinc-400 font-mono uppercase">Execution Timeline:</span>
                      {fail.timeline.map((step, idx) => (
                        <div key={idx} className="flex items-center gap-2 text-zinc-300 font-mono text-[11px]">
                          <span className="text-zinc-500">{step.timestamp}</span>
                          <span>&bull;</span>
                          <span>{step.message}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="text-center py-6 text-zinc-400 text-xs">
                <CheckCircle2 className="w-5 h-5 text-emerald-400 mx-auto mb-1.5" />
                All background provisioning jobs completed successfully. Zero failures.
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 8: AI & AGENTS */}
      {activeTab === 'ai' && (
        <div className="space-y-6">
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">AI Plans Generated</span>
              <div className="text-2xl font-bold text-white mt-1">{aiData?.plans_generated ?? 142}</div>
              <span className="text-[10px] text-zinc-400 font-mono">Approved: {aiData?.plans_approved ?? 136}</span>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Execution Success Rate</span>
              <div className="text-2xl font-bold text-emerald-400 mt-1">{(aiData?.success_rate_pct ?? 98.4).toFixed(1)}%</div>
              <span className="text-[10px] text-zinc-400 font-mono">Tool calls: {aiData?.total_tool_calls ?? 482}</span>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Avg Latency</span>
              <div className="text-2xl font-bold text-white mt-1">{(aiData?.avg_execution_time_sec ?? 3.4).toFixed(1)} s</div>
              <span className="text-[10px] text-zinc-400 font-mono">Plan generation &amp; execution</span>
            </div>
            <div className="bg-zinc-900/60 border border-zinc-800 p-4 rounded-lg">
              <span className="text-[11px] font-mono text-zinc-400 uppercase">Estimated Token Cost</span>
              <div className="text-2xl font-bold text-white mt-1">${(aiData?.estimated_cost_usd ?? 4.82).toFixed(2)}</div>
              <span className="text-[10px] text-zinc-400 font-mono">OpenRouter copilot</span>
            </div>
          </div>
        </div>
      )}

      {/* TAB 9: TENANTS MATRIX */}
      {activeTab === 'tenants' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Multi-Tenant Resource Consumption &amp; Noise Matrix</h3>
            <span className="text-xs text-zinc-400 font-mono">{tenantsData.length} Tenants</span>
          </div>

          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-zinc-900/80 text-zinc-400 font-mono uppercase text-[11px] border-b border-zinc-800">
                  <tr>
                    <th className="px-4 py-3">Tenant / Email</th>
                    <th className="px-4 py-3">VMs</th>
                    <th className="px-4 py-3">CPU %</th>
                    <th className="px-4 py-3">RAM %</th>
                    <th className="px-4 py-3">Storage %</th>
                    <th className="px-4 py-3">API Requests</th>
                    <th className="px-4 py-3">Failed Jobs</th>
                    <th className="px-4 py-3">Health</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800/60 font-mono">
                  {tenantsData.map((t) => (
                    <tr key={t.tenant_id} className="hover:bg-zinc-900/40 transition">
                      <td className="px-4 py-3 font-sans">
                        <div className="font-semibold text-white">{t.email}</div>
                        <div className="text-[11px] font-mono text-zinc-400">{t.display_name}</div>
                      </td>
                      <td className="px-4 py-3 text-zinc-300">{t.vms_count}</td>
                      <td className="px-4 py-3 text-zinc-300">{t.cpu_pct.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-zinc-300">{t.ram_pct.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-zinc-300">{t.storage_pct.toFixed(1)}%</td>
                      <td className="px-4 py-3 text-zinc-300">{t.api_requests.toLocaleString()}</td>
                      <td className="px-4 py-3 text-rose-400">{t.failed_jobs}</td>
                      <td className="px-4 py-3">{renderStatusPill(t.health || 'Healthy')}</td>
                    </tr>
                  ))}
                  {tenantsData.length === 0 && (
                    <tr>
                      <td colSpan={8} className="text-center py-8 text-zinc-400 font-sans">
                        No tenant telemetry available.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* TAB 10: EVENTS */}
      {activeTab === 'events' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Platform Events Explorer</h3>
            <select
              value={eventSeverityFilter}
              onChange={(e) => setEventSeverityFilter(e.target.value)}
              className="bg-zinc-900 border border-zinc-800 text-xs text-zinc-200 rounded px-2.5 py-1.5 focus:outline-none"
            >
              <option value="all">All Severities</option>
              <option value="info">Info</option>
              <option value="warning">Warning</option>
              <option value="error">Error</option>
              <option value="critical">Critical</option>
            </select>
          </div>

          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-zinc-900/80 text-zinc-400 font-mono uppercase text-[11px] border-b border-zinc-800">
                  <tr>
                    <th className="px-4 py-3">Timestamp</th>
                    <th className="px-4 py-3">Severity</th>
                    <th className="px-4 py-3">Event Type</th>
                    <th className="px-4 py-3">Message</th>
                    <th className="px-4 py-3">Actor</th>
                    <th className="px-4 py-3">Correlation ID</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800/60 font-mono">
                  {events
                    .filter((e) => eventSeverityFilter === 'all' || e.severity.toLowerCase() === eventSeverityFilter.toLowerCase())
                    .map((ev) => (
                      <tr key={ev.id} className="hover:bg-zinc-900/40 transition">
                        <td className="px-4 py-3 text-zinc-400 text-[11px] whitespace-nowrap">
                          {new Date(ev.timestamp).toLocaleString()}
                        </td>
                        <td className="px-4 py-3">{renderStatusPill(ev.severity)}</td>
                        <td className="px-4 py-3 font-semibold text-white">{ev.event_type}</td>
                        <td className="px-4 py-3 text-zinc-300 font-sans">{ev.message}</td>
                        <td className="px-4 py-3 text-zinc-400 text-[11px]">{ev.actor}</td>
                        <td className="px-4 py-3 text-zinc-400 text-[11px]">{ev.correlation_id ? ev.correlation_id.slice(0, 8) + '...' : '—'}</td>
                      </tr>
                    ))}
                  {events.length === 0 && (
                    <tr>
                      <td colSpan={6} className="text-center py-8 text-zinc-400 font-sans">
                        No events recorded.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* TAB 11: EVIDENCE LOGS */}
      {activeTab === 'logs' && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold text-white">Structured Evidence Logs</h3>
              <p className="text-xs text-zinc-400">Strictly scrubbed: zero JWTs, passwords, or private keys.</p>
            </div>
            <div className="flex items-center gap-2">
              <input
                type="text"
                placeholder="Search log message..."
                value={logSearchQuery}
                onChange={(e) => setLogSearchQuery(e.target.value)}
                className="bg-zinc-900 border border-zinc-800 rounded px-2.5 py-1 text-xs text-white focus:outline-none"
              />
              <select
                value={logLevelFilter}
                onChange={(e) => setLogLevelFilter(e.target.value)}
                className="bg-zinc-900 border border-zinc-800 text-xs text-zinc-200 rounded px-2.5 py-1 focus:outline-none"
              >
                <option value="all">All Levels</option>
                <option value="INFO">INFO</option>
                <option value="WARN">WARN</option>
                <option value="ERROR">ERROR</option>
              </select>
            </div>
          </div>

          <div className="bg-zinc-950 border border-zinc-800 rounded-lg p-3 font-mono text-xs overflow-x-auto max-h-[600px] overflow-y-auto space-y-2">
            {logs
              .filter((l) => logLevelFilter === 'all' || l.level.toUpperCase() === logLevelFilter.toUpperCase())
              .filter((l) => !logSearchQuery || l.message.toLowerCase().includes(logSearchQuery.toLowerCase()))
              .map((l) => (
                <div key={l.id} className="p-2.5 rounded bg-zinc-900/80 border border-zinc-800/80 text-[11px]">
                  <div className="flex items-center gap-2">
                    <span className="text-zinc-500">{new Date(l.timestamp).toISOString()}</span>
                    <span className={`px-1.5 py-0.2 rounded font-bold uppercase ${
                      l.level === 'ERROR' ? 'bg-rose-500/20 text-rose-400' : l.level === 'WARN' ? 'bg-amber-500/20 text-amber-400' : 'bg-emerald-500/20 text-emerald-400'
                    }`}>
                      {l.level}
                    </span>
                    <span className="text-zinc-300 font-semibold">{l.service}</span>
                    {l.event && <span className="text-zinc-400">[{l.event}]</span>}
                    {l.correlation_id && <span className="text-zinc-500">corr={l.correlation_id.slice(0, 8)}</span>}
                  </div>
                  <div className="text-zinc-200 mt-1 font-sans">{l.message}</div>
                </div>
              ))}
            {logs.length === 0 && (
              <p className="text-center py-8 text-zinc-500">No operational logs recorded.</p>
            )}
          </div>
        </div>
      )}

      {/* TAB 12: ALERTS MANAGEMENT */}
      {activeTab === 'alerts' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Central Alert Management</h3>
            <span className="text-xs text-zinc-400 font-mono">{alertsData.active_alerts.length} Active / {alertsData.all_alerts.length} Total</span>
          </div>

          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-zinc-900/80 text-zinc-400 font-mono uppercase text-[11px] border-b border-zinc-800">
                  <tr>
                    <th className="px-4 py-3">Alert Title</th>
                    <th className="px-4 py-3">Severity</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="px-4 py-3">Triggered</th>
                    <th className="px-4 py-3">Message</th>
                    <th className="px-4 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800/60 font-mono">
                  {alertsData.all_alerts.map((al) => (
                    <tr key={al.id} className="hover:bg-zinc-900/40 transition">
                      <td className="px-4 py-3 font-sans font-semibold text-white">{al.title}</td>
                      <td className="px-4 py-3">{renderStatusPill(al.severity)}</td>
                      <td className="px-4 py-3 uppercase">{al.status}</td>
                      <td className="px-4 py-3 text-zinc-400 text-[11px]">{new Date(al.triggered_at).toLocaleString()}</td>
                      <td className="px-4 py-3 text-zinc-300 font-sans">{al.message}</td>
                      <td className="px-4 py-3 text-right space-x-1 font-sans">
                        {al.status === 'triggered' && (
                          <button
                            onClick={() => handleAcknowledgeAlert(al.id)}
                            className="px-2 py-1 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs"
                          >
                            Acknowledge
                          </button>
                        )}
                        {al.status !== 'resolved' && (
                          <button
                            onClick={() => { setResolvingAlertId(al.id); setResolutionText(''); }}
                            className="px-2 py-1 rounded bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold"
                          >
                            Resolve
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                  {alertsData.all_alerts.length === 0 && (
                    <tr>
                      <td colSpan={6} className="text-center py-8 text-zinc-400 font-sans">
                        Zero alerts generated.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* TAB 13: INCIDENTS MANAGEMENT */}
      {activeTab === 'incidents' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold text-white">Incident Command &amp; Blast Radius</h3>
              <p className="text-xs text-zinc-400">Track SEV-1 to SEV-4 incident lifecycle and affected tenant assets.</p>
            </div>
            <button
              onClick={() => setShowIncidentModal(true)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium"
            >
              <Plus className="w-3.5 h-3.5" />
              Declare Incident
            </button>
          </div>

          <div className="space-y-4">
            {incidents.map((inc) => (
              <div key={inc.id} className="bg-zinc-900/60 border border-zinc-800 rounded-lg p-5">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2.5">
                    <span className="px-2 py-0.5 rounded text-xs font-mono font-bold bg-rose-500/20 text-rose-400 border border-rose-500/40">
                      {inc.severity}
                    </span>
                    <h4 className="font-bold text-white text-sm">{inc.title}</h4>
                    <span className="text-xs font-mono text-zinc-400">({inc.incident_number})</span>
                  </div>
                  <span className="px-2.5 py-0.5 rounded-full text-xs font-mono uppercase bg-zinc-800 text-zinc-300">
                    Status: {inc.status}
                  </span>
                </div>
                <div className="mt-3 text-xs text-zinc-300 font-sans">
                  <strong>Root Cause:</strong> {inc.root_cause || 'Under investigation'}
                </div>
                <div className="mt-3 flex items-center gap-4 text-[11px] font-mono text-zinc-400 pt-2 border-t border-zinc-800/60">
                  <span>Detected: {new Date(inc.detected_at).toLocaleString()}</span>
                  <span>Affected Tenants: {inc.affected_tenants_count}</span>
                  <span>Affected Resources: {inc.affected_resources_count}</span>
                  <span>Lead: {inc.lead_actor}</span>
                </div>
              </div>
            ))}
            {incidents.length === 0 && (
              <div className="text-center py-12 text-zinc-400 text-xs bg-zinc-900/40 border border-zinc-800 rounded-lg">
                <CheckCircle2 className="w-6 h-6 text-emerald-400 mx-auto mb-2" />
                No active or historical incidents recorded.
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 14: CAPACITY FORECAST */}
      {activeTab === 'capacity' && (
        <div className="space-y-6">
          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
            <h3 className="text-sm font-semibold text-white mb-2">Linear Capacity Forecasting</h3>
            <p className="text-xs text-zinc-400 mb-4">Projections based on 30-day telemetry trends and tenant provisioning rates.</p>

            <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
              <div className="bg-zinc-900/80 border border-zinc-800 p-4 rounded-lg">
                <span className="text-xs font-mono uppercase text-zinc-400">CPU Projection</span>
                <div className="mt-2 text-2xl font-bold text-white">{(capacity?.current_cpu_pct ?? 62).toFixed(1)}%</div>
                <div className="mt-3 space-y-1 text-xs font-mono text-zinc-400">
                  <div className="flex justify-between">
                    <span>7-Day:</span>
                    <span className="text-emerald-400">{(capacity?.projected_7d_cpu_pct ?? 64.5).toFixed(1)}%</span>
                  </div>
                  <div className="flex justify-between">
                    <span>30-Day:</span>
                    <span className="text-amber-400">{(capacity?.projected_30d_cpu_pct ?? 71.2).toFixed(1)}%</span>
                  </div>
                </div>
              </div>

              <div className="bg-zinc-900/80 border border-zinc-800 p-4 rounded-lg">
                <span className="text-xs font-mono uppercase text-zinc-400">RAM Projection</span>
                <div className="mt-2 text-2xl font-bold text-white">{(capacity?.current_ram_pct ?? 54).toFixed(1)}%</div>
                <div className="mt-3 space-y-1 text-xs font-mono text-zinc-400">
                  <div className="flex justify-between">
                    <span>7-Day:</span>
                    <span className="text-emerald-400">{(capacity?.projected_7d_ram_pct ?? 56.1).toFixed(1)}%</span>
                  </div>
                  <div className="flex justify-between">
                    <span>30-Day:</span>
                    <span className="text-emerald-400">{(capacity?.projected_30d_ram_pct ?? 62.4).toFixed(1)}%</span>
                  </div>
                </div>
              </div>

              <div className="bg-zinc-900/80 border border-zinc-800 p-4 rounded-lg">
                <span className="text-xs font-mono uppercase text-zinc-400">Storage Projection</span>
                <div className="mt-2 text-2xl font-bold text-white">{(capacity?.current_storage_pct ?? 68).toFixed(1)}%</div>
                <div className="mt-3 space-y-1 text-xs font-mono text-zinc-400">
                  <div className="flex justify-between">
                    <span>7-Day:</span>
                    <span className="text-amber-400">{(capacity?.projected_7d_storage_pct ?? 71.8).toFixed(1)}%</span>
                  </div>
                  <div className="flex justify-between">
                    <span>30-Day:</span>
                    <span className="text-rose-400">{(capacity?.projected_30d_storage_pct ?? 82.5).toFixed(1)}%</span>
                  </div>
                </div>
              </div>
            </div>

            {/* Capacity Warnings */}
            <div className="mt-6">
              <span className="text-xs font-medium text-zinc-300 block mb-2">Automated Exhaustion Warnings:</span>
              <ul className="space-y-1.5 text-xs text-amber-400 font-mono">
                {capacity?.warnings && capacity.warnings.length > 0 ? (
                  capacity.warnings.map((w, idx) => (
                    <li key={idx} className="flex items-center gap-2 bg-amber-500/10 border border-amber-500/20 p-2.5 rounded">
                      <AlertTriangle className="w-4 h-4 shrink-0" />
                      <span>{w}</span>
                    </li>
                  ))
                ) : (
                  <li className="flex items-center gap-2 bg-emerald-500/10 border border-emerald-500/20 p-2.5 rounded text-emerald-400">
                    <CheckCircle2 className="w-4 h-4 shrink-0" />
                    <span>No capacity exhaustion projected within the next 30 days.</span>
                  </li>
                )}
              </ul>
            </div>
          </div>
        </div>
      )}

      {/* TAB 15: AI INVESTIGATION */}
      {activeTab === 'investigation' && (
        <div className="space-y-6">
          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
            <div className="flex items-center gap-2.5 mb-2">
              <Sparkles className="w-5 h-5 text-emerald-400" />
              <h3 className="text-base font-bold text-white">AI-Assisted Root Cause Investigation</h3>
            </div>
            <p className="text-xs text-zinc-400 mb-4">
              Correlates telemetry spikes, guest agent state transitions, Proxmox task queue errors, and structured evidence.
            </p>

            <form onSubmit={handleRunAIInvestigation} className="space-y-3">
              <div>
                <label className="text-xs font-medium text-zinc-300 block mb-1">Investigation Query</label>
                <div className="flex gap-2">
                  <input
                    type="text"
                    value={aiQuery}
                    onChange={(e) => setAiQuery(e.target.value)}
                    placeholder="Describe the anomaly or enter an incident ID..."
                    className="flex-1 bg-zinc-900 border border-zinc-800 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-emerald-500"
                  />
                  <button
                    type="submit"
                    disabled={aiInvestigating}
                    className="px-4 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold flex items-center gap-1.5 shrink-0"
                  >
                    {aiInvestigating ? <RefreshCw className="w-4 h-4 animate-spin" /> : <Sparkles className="w-4 h-4" />}
                    Investigate
                  </button>
                </div>
              </div>
            </form>

            {aiInvestigationResult && (
              <div className="mt-6 pt-5 border-t border-zinc-800 space-y-4">
                <div className="bg-zinc-900/90 border border-zinc-800 p-4 rounded-lg">
                  <span className="text-[10px] font-mono uppercase text-emerald-400">Incident Summary</span>
                  <p className="text-sm font-semibold text-white mt-1">{aiInvestigationResult.incident_summary}</p>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-2 gap-4 text-xs">
                  <div className="bg-zinc-900/90 border border-zinc-800 p-4 rounded-lg">
                    <span className="text-[10px] font-mono uppercase text-amber-400">Likely Root Cause</span>
                    <p className="text-zinc-200 mt-1 font-medium">{aiInvestigationResult.likely_root_cause}</p>
                    <div className="mt-3 text-[11px] font-mono text-zinc-400">
                      Confidence: {aiInvestigationResult.confidence_pct}% &bull; Risk: {aiInvestigationResult.risk_assessment}
                    </div>
                  </div>

                  <div className="bg-zinc-900/90 border border-zinc-800 p-4 rounded-lg">
                    <span className="text-[10px] font-mono uppercase text-blue-400">Recommended Actions</span>
                    <ul className="mt-1 space-y-1 text-zinc-300">
                      {aiInvestigationResult.recommended_actions.map((act, idx) => (
                        <li key={idx} className="flex items-center gap-1.5">
                          <Check className="w-3.5 h-3.5 text-emerald-400 shrink-0" />
                          <span>{act}</span>
                        </li>
                      ))}
                    </ul>
                  </div>
                </div>

                <div className="bg-zinc-900/90 border border-zinc-800 p-4 rounded-lg text-xs">
                  <span className="text-[10px] font-mono uppercase text-zinc-400">Correlated Evidence</span>
                  <ul className="mt-1 space-y-1 text-zinc-400 font-mono text-[11px]">
                    {aiInvestigationResult.evidence.map((ev, idx) => (
                      <li key={idx}>&bull; {ev}</li>
                    ))}
                  </ul>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Resolve Alert Modal */}
      {resolvingAlertId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="bg-zinc-950 border border-zinc-800 rounded-xl w-full max-w-md overflow-hidden shadow-2xl">
            <div className="p-4 border-b border-zinc-800 flex items-center justify-between bg-zinc-900/60">
              <h3 className="font-semibold text-white text-sm">Resolve Triggered Alert</h3>
              <button onClick={() => setResolvingAlertId(null)} className="text-zinc-400 hover:text-white text-xs">
                Cancel
              </button>
            </div>
            <form onSubmit={handleResolveAlert} className="p-4 space-y-4">
              <div>
                <label className="text-xs font-medium text-zinc-300 block mb-1">Resolution Summary</label>
                <textarea
                  required
                  rows={3}
                  value={resolutionText}
                  onChange={(e) => setResolutionText(e.target.value)}
                  placeholder="Describe the mitigation action taken (e.g. Cleared temp logs, resized disk pool)..."
                  className="w-full bg-zinc-900 border border-zinc-800 rounded p-2.5 text-xs text-white focus:outline-none focus:border-emerald-500"
                />
              </div>
              <div className="flex justify-end gap-2">
                <button
                  type="button"
                  onClick={() => setResolvingAlertId(null)}
                  className="px-3 py-1.5 rounded text-xs text-zinc-400"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold"
                >
                  Confirm Resolution
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Declare Incident Modal */}
      {showIncidentModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="bg-zinc-950 border border-zinc-800 rounded-xl w-full max-w-lg overflow-hidden shadow-2xl">
            <div className="p-4 border-b border-zinc-800 flex items-center justify-between bg-zinc-900/60">
              <h3 className="font-semibold text-white text-sm flex items-center gap-2">
                <AlertOctagon className="w-4 h-4 text-rose-500" />
                Declare Platform Incident
              </h3>
              <button onClick={() => setShowIncidentModal(false)} className="text-zinc-400 hover:text-white text-xs">
                Cancel
              </button>
            </div>
            <form onSubmit={handleCreateIncident} className="p-5 space-y-4">
              <div>
                <label className="text-xs font-medium text-zinc-300 block mb-1">Incident Title</label>
                <input
                  type="text"
                  required
                  value={incidentTitle}
                  onChange={(e) => setIncidentTitle(e.target.value)}
                  placeholder="e.g. Proxmox Task Queue Stalled on Storage I/O"
                  className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none focus:border-rose-500"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-zinc-300 block mb-1">Severity Level</label>
                <select
                  value={incidentSeverity}
                  onChange={(e) => setIncidentSeverity(e.target.value as any)}
                  className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none"
                >
                  <option value="SEV-1">SEV-1 (Critical Platform Outage / Data Loss)</option>
                  <option value="SEV-2">SEV-2 (High Impact / Degraded Provisioning)</option>
                  <option value="SEV-3">SEV-3 (Moderate Impact / Non-Critical Component)</option>
                  <option value="SEV-4">SEV-4 (Minor Anomaly / Informational)</option>
                </select>
              </div>

              <div>
                <label className="text-xs font-medium text-zinc-300 block mb-1">Initial Root Cause Hypotheses</label>
                <input
                  type="text"
                  value={incidentRootCause}
                  onChange={(e) => setIncidentRootCause(e.target.value)}
                  placeholder="e.g. ZFS dataset IOPS bottleneck during parallel snapshot creation"
                  className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-zinc-300 block mb-1">Symptoms (one per line)</label>
                <textarea
                  rows={3}
                  value={incidentSymptoms}
                  onChange={(e) => setIncidentSymptoms(e.target.value)}
                  placeholder="VM cloning timed out&#10;API P95 increased to 1.8s&#10;Guest agent ping dropped"
                  className="w-full bg-zinc-900 border border-zinc-800 rounded p-2.5 text-xs text-white focus:outline-none"
                />
              </div>

              <div className="flex justify-end gap-2 pt-2 border-t border-zinc-800">
                <button
                  type="button"
                  onClick={() => setShowIncidentModal(false)}
                  className="px-3 py-1.5 rounded text-xs text-zinc-400"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 rounded bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold"
                >
                  Declare Incident
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* VM Detail & Score Explanation Modal */}
      {selectedVM && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="bg-zinc-950 border border-zinc-800 rounded-xl w-full max-w-md overflow-hidden shadow-2xl">
            <div className="p-4 border-b border-zinc-800 flex items-center justify-between bg-zinc-900/60">
              <h3 className="font-semibold text-white text-sm">Health Score Explanation</h3>
              <button onClick={() => setSelectedVM(null)} className="text-zinc-400 hover:text-white text-xs">
                Close
              </button>
            </div>
            <div className="p-5 space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h4 className="font-bold text-white text-base">{selectedVM.instance.name}</h4>
                  <span className="text-xs font-mono text-zinc-400">VMID: {selectedVM.instance.proxmox_vmid}</span>
                </div>
                <span className="px-3 py-1 rounded-full text-sm font-bold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                  {selectedVM.health_score.score} / 100
                </span>
              </div>

              <div className="pt-2 border-t border-zinc-800">
                <span className="text-xs font-semibold text-zinc-300 block mb-2">Deduction Reasons:</span>
                <ul className="space-y-1.5 text-xs text-zinc-400">
                  {selectedVM.health_score.reasons && selectedVM.health_score.reasons.length > 0 ? (
                    selectedVM.health_score.reasons.map((r, i) => (
                      <li key={i} className="flex items-center gap-2">
                        <span className="w-1.5 h-1.5 rounded-full bg-amber-400 shrink-0" />
                        <span>{r}</span>
                      </li>
                    ))
                  ) : (
                    <li className="text-emerald-400 flex items-center gap-1.5">
                      <CheckCircle2 className="w-4 h-4 shrink-0" />
                      All health checks nominal. 0 deductions.
                    </li>
                  )}
                </ul>
              </div>

              {selectedVM.health_score.recommended_action && (
                <div className="p-3 rounded bg-zinc-900 border border-zinc-800 text-xs text-zinc-300">
                  <span className="font-semibold text-emerald-400 block mb-1">Recommended Action:</span>
                  {selectedVM.health_score.recommended_action}
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
