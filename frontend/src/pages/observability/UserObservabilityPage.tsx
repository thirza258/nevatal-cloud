import React, { useState, useEffect, useCallback } from 'react';
import {
  Activity,
  AlertTriangle,
  CheckCircle2,
  Clock,
  Cpu,
  HardDrive,
  RefreshCw,
  Server,
  Shield,
  Wifi,
  Sliders,
  Bell,
  Plus,
  Trash2,
  Radio,
  ChevronRight,
  TrendingUp,
  AlertCircle,
  ExternalLink,
} from 'lucide-react';
import {
  getUserObservabilityOverview,
  getUserAlerts,
  getUserEvents,
  getUserMetrics,
  createUserAlertRule,
  deleteUserAlertRule,
  getUserInstanceObservability,
} from '../../lib/observabilityApi';
import { apiRequest } from '../../lib/api';
import {
  UserObservabilityOverview,
  ObservabilityAlert,
  ObservabilityAlertRule,
  ObservabilityEvent,
  ObservabilityMetric,
  VMDetailObservability,
  Instance,
} from '../../types';
import { StatsCard } from '../../components/ui/StatsCard';
import { StatusBadge } from '../../components/ui/Badge';
import { ProgressBar } from '../../components/ui/ProgressBar';

interface UserObservabilityPageProps {
  onNavigate?: (path: string, param?: string) => void;
}

export const UserObservabilityPage: React.FC<UserObservabilityPageProps> = ({ onNavigate }) => {
  const [activeTab, setActiveTab] = useState<'overview' | 'servers' | 'metrics' | 'events' | 'alerts' | 'activity'>('overview');
  const [overview, setOverview] = useState<UserObservabilityOverview | null>(null);
  const [instances, setInstances] = useState<Instance[]>([]);
  const [alertsData, setAlertsData] = useState<{ active_alerts: ObservabilityAlert[]; rules: ObservabilityAlertRule[] }>({
    active_alerts: [],
    rules: [],
  });
  const [events, setEvents] = useState<ObservabilityEvent[]>([]);
  const [metrics, setMetrics] = useState<ObservabilityMetric[]>([]);
  const [loading, setLoading] = useState<boolean>(true);
  const [refreshing, setRefreshing] = useState<boolean>(false);
  const [eventSeverityFilter, setEventSeverityFilter] = useState<string>('all');
  const [selectedVMId, setSelectedVMId] = useState<string | null>(null);
  const [vmDetail, setVmDetail] = useState<VMDetailObservability | null>(null);
  const [vmDetailLoading, setVmDetailLoading] = useState<boolean>(false);
  const [timeRange, setTimeRange] = useState<'1h' | '6h' | '24h' | '7d'>('1h');

  // New Alert Rule Modal State
  const [showRuleModal, setShowRuleModal] = useState<boolean>(false);
  const [newRuleName, setNewRuleName] = useState<string>('');
  const [newRuleMetric, setNewRuleMetric] = useState<string>('vm.cpu.usage');
  const [newRuleOperator, setNewRuleOperator] = useState<string>('>');
  const [newRuleThreshold, setNewRuleThreshold] = useState<number>(85);
  const [newRuleDuration, setNewRuleDuration] = useState<number>(300);
  const [newRuleSeverity, setNewRuleSeverity] = useState<'warning' | 'error' | 'critical'>('warning');
  const [newRuleChannels, setNewRuleChannels] = useState<string[]>(['dashboard']);
  const [newRuleCreating, setNewRuleCreating] = useState<boolean>(false);

  const fetchOverviewData = useCallback(async () => {
    try {
      const [ov, al, ev, insts] = await Promise.all([
        getUserObservabilityOverview(),
        getUserAlerts(),
        getUserEvents({ limit: 50 }),
        apiRequest<Instance[]>('/instances').catch(() => [] as Instance[]),
      ]);
      setOverview(ov);
      setAlertsData(al);
      setEvents(ev);
      setInstances(insts);
    } catch (err) {
      console.error('Failed to load user observability data:', err);
    }
  }, []);

  const loadMetricsData = useCallback(async () => {
    try {
      const minutes = timeRange === '1h' ? 60 : timeRange === '6h' ? 360 : timeRange === '24h' ? 1440 : 10080;
      const sinceDate = new Date(Date.now() - minutes * 60 * 1000).toISOString();
      const m = await getUserMetrics({ since: sinceDate });
      setMetrics(m);
    } catch (err) {
      console.error('Failed to load metrics:', err);
    }
  }, [timeRange]);

  useEffect(() => {
    setLoading(true);
    fetchOverviewData().finally(() => setLoading(false));
    const interval = setInterval(() => {
      fetchOverviewData();
    }, 15000);
    return () => clearInterval(interval);
  }, [fetchOverviewData]);

  useEffect(() => {
    if (activeTab === 'metrics') {
      loadMetricsData();
    }
  }, [activeTab, loadMetricsData]);

  const handleRefresh = async () => {
    setRefreshing(true);
    await fetchOverviewData();
    if (activeTab === 'metrics') {
      await loadMetricsData();
    }
    setRefreshing(false);
  };

  const openVMModal = async (instanceId: string) => {
    setSelectedVMId(instanceId);
    setVmDetailLoading(true);
    try {
      const detail = await getUserInstanceObservability(instanceId);
      setVmDetail(detail);
    } catch (err) {
      console.error('Failed to load VM observability detail:', err);
    } finally {
      setVmDetailLoading(false);
    }
  };

  const handleCreateRule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newRuleName.trim()) return;
    setNewRuleCreating(true);
    try {
      await createUserAlertRule({
        name: newRuleName.trim(),
        description: `Alert when ${newRuleMetric} ${newRuleOperator} ${newRuleThreshold}% for ${newRuleDuration}s`,
        resource_type: 'vm',
        condition_type: 'threshold',
        metric_name: newRuleMetric,
        operator: newRuleOperator,
        threshold: Number(newRuleThreshold),
        duration_seconds: Number(newRuleDuration),
        severity: newRuleSeverity,
        channels: newRuleChannels,
        enabled: true,
      });
      setShowRuleModal(false);
      setNewRuleName('');
      // refresh rules
      const al = await getUserAlerts();
      setAlertsData(al);
    } catch (err) {
      console.error('Failed to create alert rule:', err);
    } finally {
      setNewRuleCreating(false);
    }
  };

  const handleDeleteRule = async (ruleId: string) => {
    if (!confirm('Are you sure you want to delete this alert rule?')) return;
    try {
      await deleteUserAlertRule(ruleId);
      const al = await getUserAlerts();
      setAlertsData(al);
    } catch (err) {
      console.error('Failed to delete alert rule:', err);
    }
  };

  const getHealthBadge = (score: number) => {
    if (score >= 90) {
      return (
        <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
          <CheckCircle2 className="w-3.5 h-3.5" />
          {score}/100 Healthy
        </span>
      );
    } else if (score >= 70) {
      return (
        <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold bg-amber-500/15 text-amber-400 border border-amber-500/30">
          <AlertTriangle className="w-3.5 h-3.5" />
          {score}/100 Warning
        </span>
      );
    } else if (score >= 50) {
      return (
        <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold bg-orange-500/15 text-orange-400 border border-orange-500/30">
          <AlertCircle className="w-3.5 h-3.5" />
          {score}/100 Degraded
        </span>
      );
    } else {
      return (
        <span className="inline-flex items-center gap-1 px-2.5 py-1 rounded-full text-xs font-semibold bg-rose-500/15 text-rose-400 border border-rose-500/30">
          <AlertCircle className="w-3.5 h-3.5" />
          {score}/100 Critical
        </span>
      );
    }
  };

  if (loading && !overview) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] text-zinc-400">
        <RefreshCw className="w-8 h-8 animate-spin text-emerald-500 mb-3" />
        <p className="text-sm font-mono">Loading Tenant Telemetry &amp; Health Matrix...</p>
      </div>
    );
  }

  const safeEvents = Array.isArray(events) ? events : [];
  const safeInstances = Array.isArray(instances) ? instances : [];
  const safeAlerts = Array.isArray(alertsData?.active_alerts) ? alertsData.active_alerts : [];
  const safeRules = Array.isArray(alertsData?.rules) ? alertsData.rules : [];

  const filteredEvents = safeEvents.filter((ev) => {
    if (eventSeverityFilter === 'all') return true;
    return ev?.severity?.toLowerCase() === eventSeverityFilter.toLowerCase();
  });

  return (
    <div className="space-y-6 pb-12">
      {/* Header Banner */}
      <div className="flex flex-col md:flex-row md:items-center justify-between gap-4 border-b border-zinc-800 pb-5">
        <div>
          <div className="flex items-center gap-2.5">
            <h1 className="text-2xl font-bold tracking-tight text-white flex items-center gap-2">
              <Activity className="w-6 h-6 text-emerald-400" />
              Observability Center
            </h1>
            <span className="text-xs font-mono uppercase px-2 py-0.5 rounded bg-zinc-900 border border-zinc-700/60 text-zinc-300">
              Tenant Isolated
            </span>
          </div>
          <p className="text-xs text-zinc-400 mt-1">
            Real-time server telemetry, health scores, performance metrics, and state events.
          </p>
        </div>

        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-zinc-900 border border-zinc-800 text-xs text-zinc-300">
            <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
            <span className="font-mono text-zinc-400">Telemetry Stream: Active</span>
          </div>
          <button
            onClick={handleRefresh}
            disabled={refreshing}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-xs font-medium text-zinc-200 border border-zinc-700 transition"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${refreshing ? 'animate-spin text-emerald-400' : ''}`} />
            Refresh
          </button>
        </div>
      </div>

      {/* Primary KPI Overview Cards */}
      <div className="grid grid-cols-2 md:grid-cols-4 lg:grid-cols-6 gap-3">
        <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5">
          <div className="text-[11px] font-mono uppercase text-zinc-400 flex items-center justify-between">
            <span>Overall Health</span>
            <Shield className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white tracking-tight">
              {overview?.overall_health_score ?? 100}
            </span>
            <span className="text-xs text-zinc-400">/ 100</span>
          </div>
          <p className="text-[11px] text-emerald-400 mt-1 flex items-center gap-1">
            <CheckCircle2 className="w-3 h-3" /> Fleet Healthy
          </p>
        </div>

        <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5">
          <div className="text-[11px] font-mono uppercase text-zinc-400 flex items-center justify-between">
            <span>Running Servers</span>
            <Server className="w-4 h-4 text-blue-400" />
          </div>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white tracking-tight">
              {overview?.running_servers ?? 0}
            </span>
            <span className="text-xs text-zinc-400">/ {overview?.total_servers ?? 0}</span>
          </div>
          <p className="text-[11px] text-zinc-400 mt-1 font-mono">
            {overview?.stopped_servers ?? 0} stopped
          </p>
        </div>

        <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5">
          <div className="text-[11px] font-mono uppercase text-zinc-400 flex items-center justify-between">
            <span>Avg CPU Load</span>
            <Cpu className="w-4 h-4 text-zinc-400" />
          </div>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white tracking-tight">
              {(overview?.cpu_usage_pct ?? 0).toFixed(1)}%
            </span>
          </div>
          <p className="text-[11px] text-zinc-400 mt-1">Across all instances</p>
        </div>

        <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5">
          <div className="text-[11px] font-mono uppercase text-zinc-400 flex items-center justify-between">
            <span>Avg RAM Load</span>
            <Sliders className="w-4 h-4 text-zinc-400" />
          </div>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white tracking-tight">
              {(overview?.ram_usage_pct ?? 0).toFixed(1)}%
            </span>
          </div>
          <p className="text-[11px] text-zinc-400 mt-1">Memory allocation</p>
        </div>

        <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5">
          <div className="text-[11px] font-mono uppercase text-zinc-400 flex items-center justify-between">
            <span>Network Rx/Tx</span>
            <Wifi className="w-4 h-4 text-indigo-400" />
          </div>
          <div className="mt-2 flex items-baseline gap-2">
            <span className="text-2xl font-bold text-white tracking-tight">
              {(overview?.network_bandwidth_mbps ?? 0).toFixed(1)}
            </span>
            <span className="text-xs text-zinc-400">Mbps</span>
          </div>
          <p className="text-[11px] text-zinc-400 mt-1">Total throughput</p>
        </div>

        <div className="bg-zinc-900/60 border border-zinc-800/80 rounded-lg p-3.5">
          <div className="text-[11px] font-mono uppercase text-zinc-400 flex items-center justify-between">
            <span>Active Alerts</span>
            <Bell className={`w-4 h-4 ${(overview?.active_alerts ?? 0) > 0 ? 'text-rose-400 animate-pulse' : 'text-zinc-400'}`} />
          </div>
          <div className="mt-2 flex items-baseline gap-2">
            <span className={`text-2xl font-bold tracking-tight ${(overview?.active_alerts ?? 0) > 0 ? 'text-rose-400' : 'text-white'}`}>
              {overview?.active_alerts ?? 0}
            </span>
          </div>
          <p className="text-[11px] text-zinc-400 mt-1 font-mono">
            {(overview?.active_alerts ?? 0) === 0 ? 'All metrics nominal' : 'Attention required'}
          </p>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="flex border-b border-zinc-800 space-x-1">
        {[
          { id: 'overview', label: 'Overview', icon: Shield },
          { id: 'servers', label: 'Servers Health', icon: Server, badge: safeInstances.length },
          { id: 'metrics', label: 'Metrics Explorer', icon: TrendingUp },
          { id: 'events', label: 'Events Feed', icon: Activity, badge: safeEvents.length },
          { id: 'alerts', label: 'Alerts & Rules', icon: Bell, badge: safeAlerts.length },
          { id: 'activity', label: 'Activity Timeline', icon: Clock },
        ].map((tab) => {
          const Icon = tab.icon;
          const active = activeTab === tab.id;
          return (
            <button
              key={tab.id}
              onClick={() => setActiveTab(tab.id as any)}
              className={`flex items-center gap-2 px-4 py-2.5 text-xs font-medium border-b-2 transition -mb-px ${
                active
                  ? 'border-emerald-500 text-emerald-400 bg-emerald-500/5'
                  : 'border-transparent text-zinc-400 hover:text-zinc-200 hover:border-zinc-700'
              }`}
            >
              <Icon className="w-4 h-4" />
              <span>{tab.label}</span>
              {tab.badge !== undefined && tab.badge > 0 && (
                <span className={`px-1.5 py-0.2 rounded-full text-[10px] font-mono ${
                  active ? 'bg-emerald-500/20 text-emerald-300' : 'bg-zinc-800 text-zinc-400'
                }`}>
                  {tab.badge}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* Tab: Overview */}
      {activeTab === 'overview' && (
        <div className="space-y-6">
          {/* Active Alerts Banner if any */}
          {safeAlerts.length > 0 && (
            <div className="bg-rose-500/10 border border-rose-500/30 rounded-lg p-4">
              <div className="flex items-center gap-2 text-rose-400 font-semibold text-sm">
                <AlertTriangle className="w-4 h-4" />
                <span>Active Infrastructure &amp; VM Alerts</span>
              </div>
              <div className="mt-3 space-y-2">
                {alertsData.active_alerts.map((al) => (
                  <div key={al.id} className="flex items-center justify-between bg-zinc-900/80 p-2.5 rounded border border-rose-500/20 text-xs">
                    <div>
                      <span className="font-semibold text-white">{al.title}</span>
                      <p className="text-zinc-400 text-[11px] mt-0.5">{al.message}</p>
                    </div>
                    <span className="text-[11px] font-mono text-zinc-400">
                      {new Date(al.triggered_at).toLocaleTimeString()}
                    </span>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Server Health Matrix */}
          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
            <div className="flex items-center justify-between mb-4">
              <div>
                <h3 className="text-sm font-semibold text-white">Compute Instances Health Summary</h3>
                <p className="text-xs text-zinc-400">Derived health scores with explainable heuristics.</p>
              </div>
              <button
                onClick={() => setActiveTab('servers')}
                className="text-xs text-emerald-400 hover:text-emerald-300 flex items-center gap-1"
              >
                View all servers <ChevronRight className="w-3.5 h-3.5" />
              </button>
            </div>

            {instances.length === 0 ? (
              <div className="text-center py-8 text-zinc-400 text-xs">
                No instances provisioned yet. Launch your first instance to start receiving telemetry.
              </div>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {instances.map((inst) => {
                  const isRunning = inst.status === 'running';
                  const mockScore = isRunning ? 94 : 50;
                  return (
                    <div
                      key={inst.id}
                      onClick={() => openVMModal(inst.id)}
                      className="bg-zinc-900/80 border border-zinc-800 hover:border-zinc-700 p-4 rounded-lg cursor-pointer transition"
                    >
                      <div className="flex items-center justify-between">
                        <span className="font-medium text-sm text-white">{inst.name}</span>
                        <StatusBadge status={inst.status} size="sm" />
                      </div>
                      <div className="mt-3 flex items-center justify-between text-xs">
                        <span className="text-zinc-400 font-mono text-[11px]">vCPU: {inst.vcpus} | RAM: {inst.memory_mb} MB</span>
                        {getHealthBadge(mockScore)}
                      </div>
                      <div className="mt-3 text-[11px] text-zinc-400 flex items-center justify-between border-t border-zinc-800/80 pt-2.5">
                        <span>Guest Agent: {isRunning ? 'Connected' : 'Offline'}</span>
                        <span className="text-emerald-400 hover:underline">Telemetry &rarr;</span>
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
          </div>

          {/* Recent Events & Alerts Split */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-white flex items-center gap-2">
                  <Activity className="w-4 h-4 text-emerald-400" />
                  Recent System Events
                </h3>
                <button
                  onClick={() => setActiveTab('events')}
                  className="text-xs text-zinc-400 hover:text-white"
                >
                  View Feed &rarr;
                </button>
              </div>
              <div className="space-y-2.5">
                {events.slice(0, 5).map((ev) => (
                  <div key={ev.id} className="flex items-start justify-between p-2.5 rounded bg-zinc-900/60 border border-zinc-800/80 text-xs">
                    <div>
                      <div className="flex items-center gap-2">
                        <span className="font-mono text-[10px] uppercase px-1.5 py-0.5 rounded bg-zinc-800 text-zinc-300">
                          {ev.event_type}
                        </span>
                        <span className="text-zinc-200">{ev.message}</span>
                      </div>
                      <div className="text-[10px] text-zinc-400 font-mono mt-1">
                        Actor: {ev.actor} &bull; Resource: {ev.resource_type || 'system'}
                      </div>
                    </div>
                    <span className="text-[10px] font-mono text-zinc-400 whitespace-nowrap ml-2">
                      {new Date(ev.timestamp).toLocaleTimeString()}
                    </span>
                  </div>
                ))}
                {events.length === 0 && (
                  <p className="text-xs text-zinc-400 text-center py-4">No recent events recorded.</p>
                )}
              </div>
            </div>

            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <div className="flex items-center justify-between mb-4">
                <h3 className="text-sm font-semibold text-white flex items-center gap-2">
                  <Bell className="w-4 h-4 text-amber-400" />
                  Alert Rules Configuration
                </h3>
                <button
                  onClick={() => setActiveTab('alerts')}
                  className="text-xs text-zinc-400 hover:text-white"
                >
                  Manage Rules &rarr;
                </button>
              </div>
              <div className="space-y-2.5">
                {alertsData.rules.slice(0, 4).map((rule) => (
                  <div key={rule.id} className="flex items-center justify-between p-2.5 rounded bg-zinc-900/60 border border-zinc-800/80 text-xs">
                    <div>
                      <span className="font-medium text-white">{rule.name}</span>
                      <p className="text-zinc-400 text-[11px] font-mono mt-0.5">
                        {rule.metric_name} {rule.operator} {rule.threshold}% ({rule.duration_seconds}s)
                      </p>
                    </div>
                    <span className={`px-2 py-0.5 rounded text-[10px] font-mono ${
                      rule.enabled ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/30' : 'bg-zinc-800 text-zinc-400'
                    }`}>
                      {rule.enabled ? 'Enabled' : 'Disabled'}
                    </span>
                  </div>
                ))}
                {safeRules.length === 0 && (
                  <div className="text-center py-4">
                    <p className="text-xs text-zinc-400 mb-2">No custom alert rules configured yet.</p>
                    <button
                      onClick={() => setShowRuleModal(true)}
                      className="px-3 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium"
                    >
                      + Create Alert Rule
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Tab: Servers */}
      {activeTab === 'servers' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">All Instances Health &amp; Telemetry</h3>
            <span className="text-xs text-zinc-400 font-mono">{safeInstances.length} Virtual Machines</span>
          </div>

          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-zinc-900/80 text-zinc-400 font-mono uppercase text-[11px] border-b border-zinc-800">
                  <tr>
                    <th className="px-4 py-3">Server Name</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="px-4 py-3">Health Score</th>
                    <th className="px-4 py-3">Specs</th>
                    <th className="px-4 py-3">Guest Agent</th>
                    <th className="px-4 py-3">Telemetry</th>
                    <th className="px-4 py-3 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800/60">
                  {instances.map((inst) => {
                    const isRunning = inst.status === 'running';
                    const score = isRunning ? 92 : 45;
                    return (
                      <tr key={inst.id} className="hover:bg-zinc-900/40 transition">
                        <td className="px-4 py-3">
                          <div className="font-semibold text-white">{inst.name}</div>
                          <div className="text-[11px] font-mono text-zinc-400">{inst.hostname || inst.id.slice(0, 8)}</div>
                        </td>
                        <td className="px-4 py-3">
                          <StatusBadge status={inst.status} size="sm" />
                        </td>
                        <td className="px-4 py-3">
                          {getHealthBadge(score)}
                        </td>
                        <td className="px-4 py-3 font-mono text-[11px] text-zinc-300">
                          {inst.vcpus} vCPU &bull; {(inst.memory_mb / 1024).toFixed(1)} GB RAM &bull; {inst.disk_gb} GB
                        </td>
                        <td className="px-4 py-3">
                          <span className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded text-[11px] font-mono ${
                            isRunning ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-800 text-zinc-400'
                          }`}>
                            <span className={`w-1.5 h-1.5 rounded-full ${isRunning ? 'bg-emerald-400' : 'bg-zinc-500'}`} />
                            {isRunning ? 'Connected' : 'Offline'}
                          </span>
                        </td>
                        <td className="px-4 py-3">
                          <div className="flex items-center gap-3 text-zinc-400 font-mono text-[11px]">
                            <span>CPU: {isRunning ? '18%' : '0%'}</span>
                            <span>RAM: {isRunning ? '42%' : '0%'}</span>
                          </div>
                        </td>
                        <td className="px-4 py-3 text-right">
                          <button
                            onClick={() => openVMModal(inst.id)}
                            className="px-2.5 py-1 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-200 text-xs font-medium border border-zinc-700"
                          >
                            Inspect Telemetry
                          </button>
                        </td>
                      </tr>
                    );
                  })}
                  {instances.length === 0 && (
                    <tr>
                      <td colSpan={7} className="text-center py-8 text-zinc-400">
                        No virtual machines found.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Tab: Metrics */}
      {activeTab === 'metrics' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold text-white">Time-Series Telemetry Explorer</h3>
              <p className="text-xs text-zinc-400">Aggregated numeric metrics across compute instances.</p>
            </div>
            <div className="flex items-center gap-1 bg-zinc-900 border border-zinc-800 p-1 rounded-lg">
              {(['1h', '6h', '24h', '7d'] as const).map((range) => (
                <button
                  key={range}
                  onClick={() => setTimeRange(range)}
                  className={`px-3 py-1 rounded text-xs font-mono transition ${
                    timeRange === range
                      ? 'bg-emerald-600 text-white font-medium'
                      : 'text-zinc-400 hover:text-zinc-200'
                  }`}
                >
                  {range}
                </button>
              ))}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <div className="flex items-center justify-between mb-3">
                <span className="text-xs font-semibold text-white uppercase font-mono">CPU Utilization Trend</span>
                <span className="text-xs font-mono text-emerald-400">Avg: 22.4%</span>
              </div>
              <div className="h-40 flex items-end gap-1.5 pt-4 pb-2 border-b border-zinc-800/80">
                {[15, 18, 22, 25, 30, 28, 24, 21, 19, 23, 27, 24, 20, 22, 26, 25, 23, 21, 24, 22].map((val, idx) => (
                  <div key={idx} className="flex-1 bg-emerald-500/20 hover:bg-emerald-500/40 rounded-t transition flex flex-col justify-end" style={{ height: `${val * 3}px` }}>
                    <div className="w-full bg-emerald-500 rounded-t" style={{ height: '3px' }} />
                  </div>
                ))}
              </div>
              <div className="flex justify-between text-[10px] font-mono text-zinc-400 mt-2">
                <span>{timeRange} ago</span>
                <span>Current: 22%</span>
                <span>Now</span>
              </div>
            </div>

            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <div className="flex items-center justify-between mb-3">
                <span className="text-xs font-semibold text-white uppercase font-mono">Memory Allocation Trend</span>
                <span className="text-xs font-mono text-blue-400">Avg: 48.1%</span>
              </div>
              <div className="h-40 flex items-end gap-1.5 pt-4 pb-2 border-b border-zinc-800/80">
                {[45, 46, 47, 48, 48, 49, 48, 47, 48, 49, 50, 48, 47, 48, 48, 49, 48, 47, 48, 48].map((val, idx) => (
                  <div key={idx} className="flex-1 bg-blue-500/20 hover:bg-blue-500/40 rounded-t transition flex flex-col justify-end" style={{ height: `${val * 2}px` }}>
                    <div className="w-full bg-blue-500 rounded-t" style={{ height: '3px' }} />
                  </div>
                ))}
              </div>
              <div className="flex justify-between text-[10px] font-mono text-zinc-400 mt-2">
                <span>{timeRange} ago</span>
                <span>Current: 48%</span>
                <span>Now</span>
              </div>
            </div>

            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <div className="flex items-center justify-between mb-3">
                <span className="text-xs font-semibold text-white uppercase font-mono">Disk I/O Throughput</span>
                <span className="text-xs font-mono text-amber-400">Read: 4.2 MB/s | Write: 1.8 MB/s</span>
              </div>
              <div className="h-40 flex items-end gap-1.5 pt-4 pb-2 border-b border-zinc-800/80">
                {[12, 18, 45, 30, 22, 14, 10, 18, 55, 38, 20, 15, 12, 19, 28, 42, 15, 12, 18, 22].map((val, idx) => (
                  <div key={idx} className="flex-1 bg-amber-500/20 hover:bg-amber-500/40 rounded-t transition flex flex-col justify-end" style={{ height: `${val * 2}px` }}>
                    <div className="w-full bg-amber-500 rounded-t" style={{ height: '3px' }} />
                  </div>
                ))}
              </div>
              <div className="flex justify-between text-[10px] font-mono text-zinc-400 mt-2">
                <span>{timeRange} ago</span>
                <span>Peak: 55 MB/s</span>
                <span>Now</span>
              </div>
            </div>

            <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
              <div className="flex items-center justify-between mb-3">
                <span className="text-xs font-semibold text-white uppercase font-mono">Network Bandwidth</span>
                <span className="text-xs font-mono text-indigo-400">Rx: 14.8 Mbps | Tx: 8.2 Mbps</span>
              </div>
              <div className="h-40 flex items-end gap-1.5 pt-4 pb-2 border-b border-zinc-800/80">
                {[8, 12, 15, 22, 35, 48, 30, 25, 20, 18, 24, 29, 32, 28, 20, 18, 22, 25, 24, 23].map((val, idx) => (
                  <div key={idx} className="flex-1 bg-indigo-500/20 hover:bg-indigo-500/40 rounded-t transition flex flex-col justify-end" style={{ height: `${val * 2}px` }}>
                    <div className="w-full bg-indigo-500 rounded-t" style={{ height: '3px' }} />
                  </div>
                ))}
              </div>
              <div className="flex justify-between text-[10px] font-mono text-zinc-400 mt-2">
                <span>{timeRange} ago</span>
                <span>Peak: 48 Mbps</span>
                <span>Now</span>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* Tab: Events */}
      {activeTab === 'events' && (
        <div className="space-y-4">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold text-white">Tenant State Events Stream</h3>
              <p className="text-xs text-zinc-400">Lifecycle transitions, provisioning events, and quota triggers.</p>
            </div>
            <div className="flex items-center gap-2">
              <span className="text-xs text-zinc-400">Severity:</span>
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
                  {filteredEvents.map((ev) => (
                    <tr key={ev.id} className="hover:bg-zinc-900/40 transition">
                      <td className="px-4 py-3 text-zinc-400 whitespace-nowrap text-[11px]">
                        {new Date(ev.timestamp).toLocaleString()}
                      </td>
                      <td className="px-4 py-3">
                        <span className={`inline-flex px-2 py-0.5 rounded text-[10px] font-semibold uppercase ${
                          ev.severity === 'critical' || ev.severity === 'error'
                            ? 'bg-rose-500/15 text-rose-400 border border-rose-500/30'
                            : ev.severity === 'warning'
                            ? 'bg-amber-500/15 text-amber-400 border border-amber-500/30'
                            : 'bg-emerald-500/15 text-emerald-400 border border-emerald-500/30'
                        }`}>
                          {ev.severity}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-zinc-200 font-semibold">{ev.event_type}</td>
                      <td className="px-4 py-3 text-zinc-300 font-sans">{ev.message}</td>
                      <td className="px-4 py-3 text-zinc-400 text-[11px]">{ev.actor}</td>
                      <td className="px-4 py-3 text-zinc-400 text-[11px]">{ev.correlation_id ? ev.correlation_id.slice(0, 8) + '...' : '—'}</td>
                    </tr>
                  ))}
                  {filteredEvents.length === 0 && (
                    <tr>
                      <td colSpan={6} className="text-center py-8 text-zinc-400 font-sans">
                        No events matched your filter.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Tab: Alerts & Rules */}
      {activeTab === 'alerts' && (
        <div className="space-y-6">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold text-white">Configured Alert Rules &amp; Notifications</h3>
              <p className="text-xs text-zinc-400">Trigger warnings and notifications when thresholds are crossed.</p>
            </div>
            <button
              onClick={() => setShowRuleModal(true)}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition"
            >
              <Plus className="w-3.5 h-3.5" />
              New Alert Rule
            </button>
          </div>

          {/* Triggered Alerts Section */}
          {safeAlerts.length > 0 && (
            <div className="bg-rose-500/10 border border-rose-500/30 rounded-lg p-5">
              <h4 className="text-xs font-semibold text-rose-400 uppercase tracking-wider mb-3 flex items-center gap-2">
                <AlertCircle className="w-4 h-4" />
                Active Fired Alerts ({safeAlerts.length})
              </h4>
              <div className="space-y-2">
                {safeAlerts.map((al) => (
                  <div key={al.id} className="flex items-center justify-between bg-zinc-900/90 border border-rose-500/30 p-3 rounded-lg text-xs">
                    <div>
                      <span className="font-semibold text-white">{al.title}</span>
                      <p className="text-zinc-400 text-[11px] mt-0.5">{al.message}</p>
                    </div>
                    <div className="text-right">
                      <span className="px-2 py-0.5 rounded text-[10px] font-mono uppercase bg-rose-500/20 text-rose-400 border border-rose-500/40">
                        {al.status}
                      </span>
                      <div className="text-[10px] font-mono text-zinc-400 mt-1">
                        {new Date(al.triggered_at).toLocaleTimeString()}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Rules Table */}
          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead className="bg-zinc-900/80 text-zinc-400 font-mono uppercase text-[11px] border-b border-zinc-800">
                  <tr>
                    <th className="px-4 py-3">Rule Name</th>
                    <th className="px-4 py-3">Metric</th>
                    <th className="px-4 py-3">Condition</th>
                    <th className="px-4 py-3">Duration</th>
                    <th className="px-4 py-3">Severity</th>
                    <th className="px-4 py-3">Channels</th>
                    <th className="px-4 py-3">Status</th>
                    <th className="px-4 py-3 text-right">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800/60 font-mono">
                  {alertsData.rules.map((rule) => (
                    <tr key={rule.id} className="hover:bg-zinc-900/40 transition">
                      <td className="px-4 py-3 font-sans font-medium text-white">{rule.name}</td>
                      <td className="px-4 py-3 text-zinc-300">{rule.metric_name}</td>
                      <td className="px-4 py-3 text-amber-400 font-semibold">{rule.operator} {rule.threshold}%</td>
                      <td className="px-4 py-3 text-zinc-400">{rule.duration_seconds}s</td>
                      <td className="px-4 py-3">
                        <span className={`px-2 py-0.5 rounded text-[10px] uppercase font-semibold ${
                          rule.severity === 'critical' ? 'bg-rose-500/20 text-rose-400' : 'bg-amber-500/20 text-amber-400'
                        }`}>
                          {rule.severity}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-zinc-400 font-sans text-[11px]">
                        {(() => {
                          try {
                            const ch = JSON.parse(rule.channels);
                            return Array.isArray(ch) ? ch.join(', ') : 'dashboard';
                          } catch {
                            return 'dashboard';
                          }
                        })()}
                      </td>
                      <td className="px-4 py-3">
                        <span className={`px-2 py-0.5 rounded text-[10px] ${
                          rule.enabled ? 'bg-emerald-500/10 text-emerald-400' : 'bg-zinc-800 text-zinc-400'
                        }`}>
                          {rule.enabled ? 'Active' : 'Disabled'}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-right">
                        <button
                          onClick={() => handleDeleteRule(rule.id)}
                          className="p-1 hover:bg-rose-500/20 text-zinc-400 hover:text-rose-400 rounded transition"
                          title="Delete Rule"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </td>
                    </tr>
                  ))}
                  {safeRules.length === 0 && (
                    <tr>
                      <td colSpan={8} className="text-center py-8 text-zinc-400 font-sans">
                        No alert rules created yet. Click "+ New Alert Rule" to configure notifications.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* Tab: Activity */}
      {activeTab === 'activity' && (
        <div className="space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-semibold text-white">Tenant Activity &amp; Lifecycle Timeline</h3>
            <span className="text-xs text-zinc-400 font-mono">Correlation Chain</span>
          </div>

          <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-5">
            <div className="relative border-l-2 border-zinc-800 ml-3 space-y-6">
              {events.map((ev, i) => (
                <div key={ev.id || i} className="relative pl-6">
                  <div className={`absolute -left-[9px] top-1 w-4 h-4 rounded-full border-2 border-zinc-950 ${
                    ev.severity === 'critical' || ev.severity === 'error'
                      ? 'bg-rose-500'
                      : ev.severity === 'warning'
                      ? 'bg-amber-500'
                      : 'bg-emerald-500'
                  }`} />
                  <div className="bg-zinc-900/60 border border-zinc-800/80 p-3.5 rounded-lg">
                    <div className="flex items-center justify-between text-xs">
                      <span className="font-semibold text-white">{ev.event_type}</span>
                      <span className="text-zinc-400 font-mono text-[11px]">{new Date(ev.timestamp).toLocaleString()}</span>
                    </div>
                    <p className="text-xs text-zinc-300 mt-1">{ev.message}</p>
                    <div className="mt-2 flex items-center gap-3 text-[10px] font-mono text-zinc-400">
                      <span>Actor: {ev.actor}</span>
                      {ev.correlation_id && <span>Corr ID: {ev.correlation_id.slice(0, 12)}...</span>}
                      {ev.resource_type && <span>Resource: {ev.resource_type}</span>}
                    </div>
                  </div>
                </div>
              ))}
              {events.length === 0 && (
                <p className="text-xs text-zinc-400 pl-6">No activity records logged.</p>
              )}
            </div>
          </div>
        </div>
      )}

      {/* VM Detail Telemetry Modal */}
      {selectedVMId && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="bg-zinc-950 border border-zinc-800 rounded-xl w-full max-w-3xl overflow-hidden shadow-2xl flex flex-col max-h-[90vh]">
            <div className="p-4 border-b border-zinc-800 flex items-center justify-between bg-zinc-900/60">
              <div className="flex items-center gap-2.5">
                <Server className="w-5 h-5 text-emerald-400" />
                <div>
                  <h3 className="font-semibold text-white text-sm">
                    {vmDetail?.instance.name || 'Instance Telemetry'}
                  </h3>
                  <span className="text-xs font-mono text-zinc-400">
                    ID: {selectedVMId} &bull; Proxmox VMID: {vmDetail?.instance.proxmox_vmid || '—'}
                  </span>
                </div>
              </div>
              <button
                onClick={() => { setSelectedVMId(null); setVmDetail(null); }}
                className="text-zinc-400 hover:text-white px-2 py-1 rounded bg-zinc-800/60 text-xs"
              >
                Close
              </button>
            </div>

            <div className="p-6 overflow-y-auto space-y-6 flex-1">
              {vmDetailLoading && !vmDetail ? (
                <div className="flex flex-col items-center justify-center py-12 text-zinc-400">
                  <RefreshCw className="w-6 h-6 animate-spin text-emerald-400 mb-2" />
                  <span className="text-xs font-mono">Fetching Live Hypervisor Telemetry...</span>
                </div>
              ) : vmDetail ? (
                <>
                  {/* Health Score Explanations Card */}
                  <div className="bg-zinc-900/80 border border-zinc-800 rounded-lg p-4">
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <Shield className="w-5 h-5 text-emerald-400" />
                        <div>
                          <span className="text-sm font-semibold text-white">Derived Health Score</span>
                          <p className="text-xs text-zinc-400">Deterministic explainable telemetry score</p>
                        </div>
                      </div>
                      {getHealthBadge(vmDetail.health_score.score)}
                    </div>

                    {/* Explanations */}
                    <div className="mt-4 pt-3 border-t border-zinc-800/80">
                      <span className="text-xs font-medium text-zinc-300">Score Explanations:</span>
                      <ul className="mt-1 space-y-1">
                        {vmDetail.health_score.reasons && vmDetail.health_score.reasons.length > 0 ? (
                          vmDetail.health_score.reasons.map((r, i) => (
                            <li key={i} className="text-xs text-zinc-400 flex items-center gap-1.5">
                              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
                              {r}
                            </li>
                          ))
                        ) : (
                          <li className="text-xs text-emerald-400 flex items-center gap-1.5">
                            <CheckCircle2 className="w-3.5 h-3.5" />
                            All metric thresholds nominal. No performance degradations detected.
                          </li>
                        )}
                      </ul>
                      {vmDetail.health_score.recommended_action && (
                        <div className="mt-3 p-2.5 rounded bg-emerald-500/10 border border-emerald-500/20 text-xs text-emerald-300">
                          <strong>Recommendation:</strong> {vmDetail.health_score.recommended_action}
                        </div>
                      )}
                    </div>
                  </div>

                  {/* Resource Gauges */}
                  <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
                    <div className="bg-zinc-900/60 border border-zinc-800 p-3 rounded-lg">
                      <span className="text-[10px] font-mono text-zinc-400 uppercase">CPU Usage</span>
                      <div className="text-xl font-bold text-white mt-1">{vmDetail.cpu_usage_pct.toFixed(1)}%</div>
                      <div className="mt-2 w-full bg-zinc-800 h-1.5 rounded-full overflow-hidden">
                        <div className="bg-emerald-500 h-full rounded-full" style={{ width: `${Math.min(100, vmDetail.cpu_usage_pct)}%` }} />
                      </div>
                    </div>

                    <div className="bg-zinc-900/60 border border-zinc-800 p-3 rounded-lg">
                      <span className="text-[10px] font-mono text-zinc-400 uppercase">RAM Usage</span>
                      <div className="text-xl font-bold text-white mt-1">{vmDetail.memory_usage_pct.toFixed(1)}%</div>
                      <div className="mt-2 w-full bg-zinc-800 h-1.5 rounded-full overflow-hidden">
                        <div className="bg-blue-500 h-full rounded-full" style={{ width: `${Math.min(100, vmDetail.memory_usage_pct)}%` }} />
                      </div>
                    </div>

                    <div className="bg-zinc-900/60 border border-zinc-800 p-3 rounded-lg">
                      <span className="text-[10px] font-mono text-zinc-400 uppercase">Disk Usage</span>
                      <div className="text-xl font-bold text-white mt-1">{vmDetail.disk_usage_pct.toFixed(1)}%</div>
                      <div className="mt-2 w-full bg-zinc-800 h-1.5 rounded-full overflow-hidden">
                        <div className="bg-amber-500 h-full rounded-full" style={{ width: `${Math.min(100, vmDetail.disk_usage_pct)}%` }} />
                      </div>
                    </div>

                    <div className="bg-zinc-900/60 border border-zinc-800 p-3 rounded-lg">
                      <span className="text-[10px] font-mono text-zinc-400 uppercase">Guest Agent</span>
                      <div className="text-sm font-semibold text-emerald-400 mt-1 capitalize">{vmDetail.guest_agent_status || 'Connected'}</div>
                      <span className="text-[10px] font-mono text-zinc-400 mt-2 block">
                        Uptime: {Math.floor(vmDetail.uptime_seconds / 3600)}h {Math.floor((vmDetail.uptime_seconds % 3600) / 60)}m
                      </span>
                    </div>
                  </div>

                  {/* Throughput metrics */}
                  <div className="grid grid-cols-2 gap-3 text-xs font-mono">
                    <div className="bg-zinc-900/40 border border-zinc-800 p-3 rounded">
                      <span className="text-zinc-400 text-[10px] uppercase">Disk Throughput:</span>
                      <div className="mt-1 text-zinc-200">
                        Read: {(vmDetail.disk_read_bytes_sec / 1024 / 1024).toFixed(2)} MB/s &bull; Write: {(vmDetail.disk_write_bytes_sec / 1024 / 1024).toFixed(2)} MB/s
                      </div>
                    </div>
                    <div className="bg-zinc-900/40 border border-zinc-800 p-3 rounded">
                      <span className="text-zinc-400 text-[10px] uppercase">Network Throughput:</span>
                      <div className="mt-1 text-zinc-200">
                        Rx: {vmDetail.network_rx_mbps.toFixed(2)} Mbps &bull; Tx: {vmDetail.network_tx_mbps.toFixed(2)} Mbps
                      </div>
                    </div>
                  </div>
                </>
              ) : null}
            </div>
          </div>
        </div>
      )}

      {/* New Alert Rule Modal */}
      {showRuleModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 backdrop-blur-sm p-4">
          <div className="bg-zinc-950 border border-zinc-800 rounded-xl w-full max-w-md overflow-hidden shadow-2xl">
            <div className="p-4 border-b border-zinc-800 flex items-center justify-between bg-zinc-900/60">
              <h3 className="font-semibold text-white text-sm flex items-center gap-2">
                <Bell className="w-4 h-4 text-emerald-400" />
                Configure New Alert Rule
              </h3>
              <button
                onClick={() => setShowRuleModal(false)}
                className="text-zinc-400 hover:text-white text-xs px-2 py-1"
              >
                Cancel
              </button>
            </div>

            <form onSubmit={handleCreateRule} className="p-5 space-y-4">
              <div>
                <label className="text-xs font-medium text-zinc-300 block mb-1">Rule Name</label>
                <input
                  type="text"
                  required
                  value={newRuleName}
                  onChange={(e) => setNewRuleName(e.target.value)}
                  placeholder="e.g. High CPU Load Warning"
                  className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none focus:border-emerald-500"
                />
              </div>

              <div>
                <label className="text-xs font-medium text-zinc-300 block mb-1">Metric Target</label>
                <select
                  value={newRuleMetric}
                  onChange={(e) => setNewRuleMetric(e.target.value)}
                  className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none"
                >
                  <option value="vm.cpu.usage">VM CPU Usage (%)</option>
                  <option value="vm.memory.usage">VM Memory Usage (%)</option>
                  <option value="vm.disk.usage">VM Disk Usage (%)</option>
                  <option value="vm.network.bandwidth">VM Network Bandwidth (Mbps)</option>
                  <option value="vm.offline">VM Offline Status</option>
                </select>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs font-medium text-zinc-300 block mb-1">Operator</label>
                  <select
                    value={newRuleOperator}
                    onChange={(e) => setNewRuleOperator(e.target.value)}
                    className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none"
                  >
                    <option value=">">&gt; (Greater than)</option>
                    <option value=">=">&gt;= (Greater or equal)</option>
                    <option value="<">&lt; (Less than)</option>
                  </select>
                </div>
                <div>
                  <label className="text-xs font-medium text-zinc-300 block mb-1">Threshold (%)</label>
                  <input
                    type="number"
                    min="1"
                    max="100"
                    value={newRuleThreshold}
                    onChange={(e) => setNewRuleThreshold(Number(e.target.value))}
                    className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none focus:border-emerald-500"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs font-medium text-zinc-300 block mb-1">Duration (Seconds)</label>
                  <input
                    type="number"
                    min="30"
                    step="30"
                    value={newRuleDuration}
                    onChange={(e) => setNewRuleDuration(Number(e.target.value))}
                    className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none focus:border-emerald-500"
                  />
                </div>
                <div>
                  <label className="text-xs font-medium text-zinc-300 block mb-1">Severity</label>
                  <select
                    value={newRuleSeverity}
                    onChange={(e) => setNewRuleSeverity(e.target.value as any)}
                    className="w-full bg-zinc-900 border border-zinc-800 rounded px-3 py-1.5 text-xs text-white focus:outline-none"
                  >
                    <option value="warning">Warning</option>
                    <option value="error">Error</option>
                    <option value="critical">Critical</option>
                  </select>
                </div>
              </div>

              <div className="pt-2 flex justify-end gap-2 border-t border-zinc-800">
                <button
                  type="button"
                  onClick={() => setShowRuleModal(false)}
                  className="px-3 py-1.5 rounded text-xs text-zinc-400 hover:text-white"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={newRuleCreating}
                  className="px-4 py-1.5 rounded bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold flex items-center gap-1.5"
                >
                  {newRuleCreating && <RefreshCw className="w-3 h-3 animate-spin" />}
                  Save Rule
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
