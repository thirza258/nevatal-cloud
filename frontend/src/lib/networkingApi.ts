import { apiRequest } from './api';
import type {
  PrivateNetwork,
  NetworkAttachment,
  CloudFirewall,
  FirewallRule,
  FloatingIP,
  DNSZone,
  DNSRecord,
  LoadBalancer,
  LoadBalancerListener,
  LoadBalancerPool,
  LoadBalancerMember,
  APIKey,
  WebhookEndpoint,
  WebhookDelivery,
} from '../types';

// ==========================================
// 1. Networking & IPAM
// ==========================================

export async function listNetworks(): Promise<PrivateNetwork[]> {
  const res = await apiRequest<any>('/networks');
  return Array.isArray(res) ? res : (res?.networks || []);
}

export async function createNetwork(data: { name: string; cidr: string; description?: string; project_id?: string }): Promise<PrivateNetwork> {
  return apiRequest<PrivateNetwork>('/networks', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getNetwork(id: string): Promise<PrivateNetwork> {
  return apiRequest<PrivateNetwork>(`/networks/${id}`);
}

export async function deleteNetwork(id: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/networks/${id}`, {
    method: 'DELETE',
  });
}

export async function attachNetwork(networkId: string, data: { instance_id: string; ip_address?: string }): Promise<NetworkAttachment> {
  return apiRequest<NetworkAttachment>(`/networks/${networkId}/attachments`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function detachNetwork(
  networkId: string,
  attachmentId: string
): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/networks/${networkId}/attachments/${attachmentId}`, {
    method: 'DELETE',
  });
}

// ==========================================
// 2. Cloud Firewalls
// ==========================================

export async function listFirewalls(): Promise<CloudFirewall[]> {
  const res = await apiRequest<any>('/firewalls');
  return Array.isArray(res) ? res : (res?.firewalls || []);
}

export async function createFirewall(data: { name: string; description?: string; project_id?: string }): Promise<CloudFirewall> {
  return apiRequest<CloudFirewall>('/firewalls', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getFirewall(id: string): Promise<CloudFirewall> {
  return apiRequest<CloudFirewall>(`/firewalls/${id}`);
}

export async function deleteFirewall(id: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/firewalls/${id}`, {
    method: 'DELETE',
  });
}

export async function createFirewallRule(firewallId: string, data: {
  direction: 'inbound' | 'outbound';
  action: 'allow' | 'deny';
  protocol: 'tcp' | 'udp' | 'icmp' | 'any';
  port_min?: number;
  port_max?: number;
  source_cidr?: string;
  destination_cidr?: string;
  priority?: number;
}): Promise<FirewallRule> {
  return apiRequest<FirewallRule>(`/firewalls/${firewallId}/rules`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteFirewallRule(firewallId: string, ruleId: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/firewalls/${firewallId}/rules/${ruleId}`, {
    method: 'DELETE',
  });
}

export async function attachFirewall(
  firewallId: string,
  resourceId: string,
  resourceType: 'vm' | 'network' = 'vm'
): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/firewalls/${firewallId}/attachments`, {
    method: 'POST',
    body: JSON.stringify({ resource_id: resourceId, resource_type: resourceType }),
  });
}

export async function detachFirewall(firewallId: string, attachmentId: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/firewalls/${firewallId}/attachments/${attachmentId}`, {
    method: 'DELETE',
  });
}

// ==========================================
// 3. Floating IPs
// ==========================================

export async function listFloatingIPs(): Promise<FloatingIP[]> {
  const res = await apiRequest<any>('/floating-ips');
  return Array.isArray(res) ? res : (res?.floating_ips || []);
}

export async function createFloatingIP(data: { description?: string; project_id?: string }): Promise<FloatingIP> {
  return apiRequest<FloatingIP>('/floating-ips', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function assignFloatingIP(id: string, instanceId: string): Promise<FloatingIP> {
  return apiRequest<FloatingIP>(`/floating-ips/${id}/assign`, {
    method: 'POST',
    body: JSON.stringify({ instance_id: instanceId }),
  });
}

export async function releaseFloatingIP(id: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/floating-ips/${id}`, {
    method: 'DELETE',
  });
}

// ==========================================
// 4. DNS Zones & Records
// ==========================================

export async function listDNSZones(): Promise<DNSZone[]> {
  const res = await apiRequest<any>('/dns/zones');
  return Array.isArray(res) ? res : (res?.zones || []);
}

export async function createDNSZone(data: { domain: string; project_id?: string }): Promise<DNSZone> {
  return apiRequest<DNSZone>('/dns/zones', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getDNSZone(id: string): Promise<DNSZone | undefined> {
  const zones = await listDNSZones();
  return zones.find((z) => z.id === id);
}

export async function deleteDNSZone(id: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/dns/zones/${id}`, {
    method: 'DELETE',
  });
}

export async function listDNSRecords(zoneId: string): Promise<DNSRecord[]> {
  const res = await apiRequest<any>(`/dns/zones/${zoneId}/records`);
  return Array.isArray(res) ? res : (res?.records || []);
}

export async function createDNSRecord(zoneId: string, data: {
  name: string;
  record_type: string;
  value: string;
  ttl?: number;
}): Promise<DNSRecord> {
  return apiRequest<DNSRecord>(`/dns/zones/${zoneId}/records`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteDNSRecord(zoneId: string, recordId: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/dns/zones/${zoneId}/records/${recordId}`, {
    method: 'DELETE',
  });
}

// ==========================================
// 5. Load Balancers
// ==========================================

export async function listLoadBalancers(): Promise<LoadBalancer[]> {
  const res = await apiRequest<any>('/load-balancers');
  return Array.isArray(res) ? res : (res?.load_balancers || []);
}

export async function createLoadBalancer(data: {
  name: string;
  network_id?: string;
  algorithm?: string;
  project_id?: string;
}): Promise<LoadBalancer> {
  return apiRequest<LoadBalancer>('/load-balancers', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getLoadBalancer(id: string): Promise<LoadBalancer> {
  return apiRequest<LoadBalancer>(`/load-balancers/${id}`);
}

export async function deleteLoadBalancer(id: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/load-balancers/${id}`, {
    method: 'DELETE',
  });
}

export async function createListener(loadBalancerId: string, data: {
  protocol: 'tcp' | 'http' | 'https';
  frontend_port: number;
  certificate_id?: string;
  default_pool_id?: string;
}): Promise<LoadBalancerListener> {
  return apiRequest<LoadBalancerListener>(`/load-balancers/${loadBalancerId}/listeners`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteListener(loadBalancerId: string, listenerId: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/load-balancers/${loadBalancerId}/listeners/${listenerId}`, {
    method: 'DELETE',
  });
}

export async function createPool(loadBalancerId: string, data: {
  name: string;
  protocol: 'tcp' | 'http' | 'https';
  port?: number;
  algorithm?: 'round_robin' | 'least_conn' | 'source_ip' | 'weighted_round_robin';
  health_check_config?: string;
}): Promise<LoadBalancerPool> {
  return apiRequest<LoadBalancerPool>(`/load-balancers/${loadBalancerId}/pools`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deletePool(loadBalancerId: string, poolId: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/load-balancers/${loadBalancerId}/pools/${poolId}`, {
    method: 'DELETE',
  });
}

export async function createMember(loadBalancerId: string, poolId: string, data: {
  instance_id: string;
  address?: string;
  port: number;
  weight?: number;
}): Promise<LoadBalancerMember> {
  return apiRequest<LoadBalancerMember>(`/load-balancers/${loadBalancerId}/pools/${poolId}/members`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteMember(loadBalancerId: string, poolId: string, memberId: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/load-balancers/${loadBalancerId}/pools/${poolId}/members/${memberId}`, {
    method: 'DELETE',
  });
}

export async function getLoadBalancerMetrics(loadBalancerId: string): Promise<{
  active_connections: number;
  requests_per_second: number;
  healthy_backends: number;
  unhealthy_backends: number;
}> {
  return apiRequest(`/load-balancers/${loadBalancerId}/metrics`);
}

// ==========================================
// 6. Developer Platform (API Keys & Webhooks)
// ==========================================

export async function listAPIKeys(): Promise<APIKey[]> {
  const res = await apiRequest<any>('/api-keys');
  return Array.isArray(res) ? res : (res?.api_keys || []);
}

export function parseAPIKeyScopes(scopes: string[] | string | undefined): string[] {
  if (Array.isArray(scopes)) return scopes;
  if (typeof scopes === 'string' && scopes.trim() !== '') {
    try {
      const parsed = JSON.parse(scopes);
      if (Array.isArray(parsed)) return parsed.map(String);
    } catch {
      return scopes.split(',').map((s) => s.trim()).filter(Boolean);
    }
  }
  return [];
}

export async function createAPIKey(data: {
  name: string;
  scopes?: string[];
  expiry_days?: number;
  project_id?: string;
}): Promise<APIKey> {
  const res = await apiRequest<{ key: APIKey; raw_key: string } | APIKey>('/api-keys', {
    method: 'POST',
    body: JSON.stringify(data),
  });
  if (res && typeof res === 'object' && 'key' in res && res.key) {
    return {
      ...res.key,
      scopes: parseAPIKeyScopes(res.key.scopes),
      raw_key: res.raw_key,
    };
  }
  return res as APIKey;
}

export async function revokeAPIKey(id: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/api-keys/${id}`, {
    method: 'DELETE',
  });
}

export async function listWebhooks(): Promise<WebhookEndpoint[]> {
  const res = await apiRequest<any>('/webhooks');
  return Array.isArray(res) ? res : (res?.webhooks || []);
}

export async function createWebhook(data: {
  name: string;
  url: string;
  events: string[];
  project_id?: string;
}): Promise<WebhookEndpoint> {
  return apiRequest<WebhookEndpoint>('/webhooks', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteWebhook(id: string): Promise<{ status: string }> {
  return apiRequest<{ status: string }>(`/webhooks/${id}`, {
    method: 'DELETE',
  });
}

export async function rotateWebhookSecret(id: string): Promise<{ secret: string }> {
  return apiRequest<{ secret: string }>(`/webhooks/${id}/rotate-secret`, {
    method: 'POST',
  });
}

export async function pingWebhook(id: string): Promise<WebhookDelivery> {
  return apiRequest(`/webhooks/${id}/ping`, {
    method: 'POST',
  });
}

export async function listWebhookDeliveries(endpointId: string): Promise<WebhookDelivery[]> {
  const res = await apiRequest<any>(`/webhooks/deliveries?endpoint_id=${encodeURIComponent(endpointId)}`);
  return Array.isArray(res) ? res : (res?.deliveries || []);
}

export async function retryWebhookDelivery(deliveryId: string): Promise<{ message: string }> {
  return apiRequest(`/webhooks/deliveries/${deliveryId}/retry`, {
    method: 'POST',
  });
}

export async function getOpenAPISpec(): Promise<Record<string, unknown>> {
  const resp = await fetch('/api/v1/openapi.json');
  return resp.json();
}
