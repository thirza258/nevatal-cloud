import { apiRequest } from './api';
import {
  AdminObservabilityOverview,
  UserObservabilityOverview,
  InfrastructureHealth,
  APIObservabilitySummary,
  JobObservabilitySummary,
  AIObservabilitySummary,
  TenantObservabilityDetail,
  CapacityForecast,
  AIIncidentInvestigationResult,
  VMDetailObservability,
  ObservabilityMetric,
  ObservabilityEvent,
  ObservabilityLog,
  ObservabilityAlertRule,
  ObservabilityAlert,
  ObservabilityIncident,
  CreateAlertRuleInput,
  UpdateAlertRuleInput,
  CreateIncidentInput,
  UpdateIncidentInput,
  AIInvestigateInput,
} from '../types';

// ==========================================
// Customer Observability Endpoints
// ==========================================

export async function getUserObservabilityOverview(): Promise<UserObservabilityOverview> {
  return apiRequest<UserObservabilityOverview>('/observability/overview');
}

export async function getUserMetrics(params?: {
  metric_name?: string;
  resource_id?: string;
  since?: string;
}): Promise<ObservabilityMetric[]> {
  const query = new URLSearchParams();
  if (params?.metric_name) query.set('metric_name', params.metric_name);
  if (params?.resource_id) query.set('resource_id', params.resource_id);
  if (params?.since) query.set('since', params.since);
  const qStr = query.toString();
  return apiRequest<ObservabilityMetric[]>(`/observability/metrics${qStr ? `?${qStr}` : ''}`);
}

export async function getUserEvents(params?: {
  severity?: string;
  event_type?: string;
  limit?: number;
}): Promise<ObservabilityEvent[]> {
  const query = new URLSearchParams();
  if (params?.severity) query.set('severity', params.severity);
  if (params?.event_type) query.set('event_type', params.event_type);
  if (params?.limit) query.set('limit', String(params.limit));
  const qStr = query.toString();
  return apiRequest<ObservabilityEvent[]>(`/observability/events${qStr ? `?${qStr}` : ''}`);
}

export async function getUserAlerts(): Promise<{
  active_alerts: ObservabilityAlert[];
  rules: ObservabilityAlertRule[];
}> {
  return apiRequest<{
    active_alerts: ObservabilityAlert[];
    rules: ObservabilityAlertRule[];
  }>('/observability/alerts');
}

export async function createUserAlertRule(
  data: CreateAlertRuleInput
): Promise<ObservabilityAlertRule> {
  return apiRequest<ObservabilityAlertRule>('/observability/alerts', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function updateUserAlertRule(
  id: string,
  data: UpdateAlertRuleInput
): Promise<ObservabilityAlertRule> {
  return apiRequest<ObservabilityAlertRule>(`/observability/alerts/${id}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export async function deleteUserAlertRule(id: string): Promise<void> {
  return apiRequest<void>(`/observability/alerts/${id}`, {
    method: 'DELETE',
  });
}

export async function getUserInstanceObservability(
  instanceId: string
): Promise<VMDetailObservability> {
  return apiRequest<VMDetailObservability>(`/observability/instances/${instanceId}`);
}

export async function getUserInstanceMetrics(
  instanceId: string,
  sinceMinutes: number = 60
): Promise<ObservabilityMetric[]> {
  return apiRequest<ObservabilityMetric[]>(
    `/observability/instances/${instanceId}/metrics?since_minutes=${sinceMinutes}`
  );
}

export async function getUserInstanceEvents(
  instanceId: string
): Promise<ObservabilityEvent[]> {
  return apiRequest<ObservabilityEvent[]>(`/observability/instances/${instanceId}/events`);
}

// ==========================================
// Admin Observability Endpoints
// ==========================================

export async function getAdminObservabilityOverview(): Promise<AdminObservabilityOverview> {
  return apiRequest<AdminObservabilityOverview>('/admin/observability/overview');
}

export async function getAdminInfrastructure(): Promise<InfrastructureHealth> {
  return apiRequest<InfrastructureHealth>('/admin/observability/infrastructure');
}

export async function getAdminNodes(): Promise<any[]> {
  return apiRequest<any[]>('/admin/observability/nodes');
}

export async function getAdminNodeDetail(nodeId: string): Promise<any> {
  return apiRequest<any>(`/admin/observability/nodes/${nodeId}`);
}

export async function getAdminStorage(): Promise<any> {
  return apiRequest<any>('/admin/observability/storage');
}

export async function getAdminVMs(): Promise<VMDetailObservability[]> {
  return apiRequest<VMDetailObservability[]>('/admin/observability/vms');
}

export async function getAdminVMDetail(vmId: string): Promise<VMDetailObservability> {
  return apiRequest<VMDetailObservability>(`/admin/observability/vms/${vmId}`);
}

export async function getAdminAPI(): Promise<APIObservabilitySummary> {
  return apiRequest<APIObservabilitySummary>('/admin/observability/api');
}

export async function getAdminJobs(): Promise<JobObservabilitySummary> {
  return apiRequest<JobObservabilitySummary>('/admin/observability/jobs');
}

export async function getAdminTenants(): Promise<TenantObservabilityDetail[]> {
  return apiRequest<TenantObservabilityDetail[]>('/admin/observability/tenants');
}

export async function getAdminEvents(params?: {
  severity?: string;
  event_type?: string;
  tenant_id?: string;
  correlation_id?: string;
  limit?: number;
}): Promise<ObservabilityEvent[]> {
  const query = new URLSearchParams();
  if (params?.severity) query.set('severity', params.severity);
  if (params?.event_type) query.set('event_type', params.event_type);
  if (params?.tenant_id) query.set('tenant_id', params.tenant_id);
  if (params?.correlation_id) query.set('correlation_id', params.correlation_id);
  if (params?.limit) query.set('limit', String(params.limit));
  const qStr = query.toString();
  return apiRequest<ObservabilityEvent[]>(`/admin/observability/events${qStr ? `?${qStr}` : ''}`);
}

export async function getAdminLogs(params?: {
  level?: string;
  service?: string;
  correlation_id?: string;
  job_id?: string;
  limit?: number;
}): Promise<ObservabilityLog[]> {
  const query = new URLSearchParams();
  if (params?.level) query.set('level', params.level);
  if (params?.service) query.set('service', params.service);
  if (params?.correlation_id) query.set('correlation_id', params.correlation_id);
  if (params?.job_id) query.set('job_id', params.job_id);
  if (params?.limit) query.set('limit', String(params.limit));
  const qStr = query.toString();
  return apiRequest<ObservabilityLog[]>(`/admin/observability/logs${qStr ? `?${qStr}` : ''}`);
}

export async function getAdminAlerts(): Promise<{
  active_alerts: ObservabilityAlert[];
  all_alerts: ObservabilityAlert[];
  rules: ObservabilityAlertRule[];
}> {
  return apiRequest<{
    active_alerts: ObservabilityAlert[];
    all_alerts: ObservabilityAlert[];
    rules: ObservabilityAlertRule[];
  }>('/admin/observability/alerts');
}

export async function acknowledgeAlert(alertId: string): Promise<ObservabilityAlert> {
  return apiRequest<ObservabilityAlert>(`/admin/observability/alerts/${alertId}/acknowledge`, {
    method: 'POST',
  });
}

export async function resolveAlert(
  alertId: string,
  resolution: string
): Promise<ObservabilityAlert> {
  return apiRequest<ObservabilityAlert>(`/admin/observability/alerts/${alertId}/resolve`, {
    method: 'POST',
    body: JSON.stringify({ resolution }),
  });
}

export async function getAdminIncidents(): Promise<ObservabilityIncident[]> {
  return apiRequest<ObservabilityIncident[]>('/admin/observability/incidents');
}

export async function createAdminIncident(
  data: CreateIncidentInput
): Promise<ObservabilityIncident> {
  return apiRequest<ObservabilityIncident>('/admin/observability/incidents', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getAdminIncidentDetail(
  incidentId: string
): Promise<ObservabilityIncident> {
  return apiRequest<ObservabilityIncident>(`/admin/observability/incidents/${incidentId}`);
}

export async function updateAdminIncident(
  incidentId: string,
  data: UpdateIncidentInput
): Promise<ObservabilityIncident> {
  return apiRequest<ObservabilityIncident>(`/admin/observability/incidents/${incidentId}`, {
    method: 'PATCH',
    body: JSON.stringify(data),
  });
}

export async function getAdminCapacity(): Promise<CapacityForecast> {
  return apiRequest<CapacityForecast>('/admin/observability/capacity');
}

export async function getAdminAI(): Promise<AIObservabilitySummary> {
  return apiRequest<AIObservabilitySummary>('/admin/observability/ai');
}

export async function adminAIInvestigate(
  data: AIInvestigateInput
): Promise<AIIncidentInvestigationResult> {
  return apiRequest<AIIncidentInvestigationResult>('/admin/observability/ai/investigate', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}
