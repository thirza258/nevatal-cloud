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
  return apiRequest<PrivateNetwork[]>('/networks');
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

export async function detachNetwork(attachmentId: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/networks/attachments/${attachmentId}`, {
    method: 'DELETE',
  });
}

// ==========================================
// 2. Cloud Firewalls
// ==========================================

export async function listFirewalls(): Promise<CloudFirewall[]> {
  return apiRequest<CloudFirewall[]>('/firewalls');
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
  protocol: 'tcp' | 'udp' | 'icmp' | 'all';
  port_range?: string;
  cidr: string;
  description?: string;
  priority?: number;
}): Promise<FirewallRule> {
  return apiRequest<FirewallRule>(`/firewalls/${firewallId}/rules`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteFirewallRule(ruleId: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/firewalls/rules/${ruleId}`, {
    method: 'DELETE',
  });
}

export async function attachFirewall(firewallId: string, instanceId: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/firewalls/${firewallId}/attachments`, {
    method: 'POST',
    body: JSON.stringify({ instance_id: instanceId }),
  });
}

export async function detachFirewall(attachmentId: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/firewalls/attachments/${attachmentId}`, {
    method: 'DELETE',
  });
}

// ==========================================
// 3. Floating IPs
// ==========================================

export async function listFloatingIPs(): Promise<FloatingIP[]> {
  return apiRequest<FloatingIP[]>('/floating-ips');
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
  return apiRequest<DNSZone[]>('/dns/zones');
}

export async function createDNSZone(data: { name: string; description?: string; project_id?: string }): Promise<DNSZone> {
  return apiRequest<DNSZone>('/dns/zones', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function getDNSZone(id: string): Promise<DNSZone> {
  return apiRequest<DNSZone>(`/dns/zones/${id}`);
}

export async function deleteDNSZone(id: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/dns/zones/${id}`, {
    method: 'DELETE',
  });
}

export async function listDNSRecords(zoneId: string): Promise<DNSRecord[]> {
  return apiRequest<DNSRecord[]>(`/dns/zones/${zoneId}/records`);
}

export async function createDNSRecord(zoneId: string, data: {
  name: string;
  type: string;
  content: string;
  ttl?: number;
  priority?: number;
}): Promise<DNSRecord> {
  return apiRequest<DNSRecord>(`/dns/zones/${zoneId}/records`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteDNSRecord(recordId: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/dns/records/${recordId}`, {
    method: 'DELETE',
  });
}

// ==========================================
// 5. Load Balancers
// ==========================================

export async function listLoadBalancers(): Promise<LoadBalancer[]> {
  return apiRequest<LoadBalancer[]>('/load-balancers');
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
  name: string;
  protocol: 'tcp' | 'http' | 'https';
  port: number;
  ssl_enabled?: boolean;
  pool_id?: string;
}): Promise<LoadBalancerListener> {
  return apiRequest<LoadBalancerListener>(`/load-balancers/${loadBalancerId}/listeners`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteListener(listenerId: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/load-balancers/listeners/${listenerId}`, {
    method: 'DELETE',
  });
}

export async function createPool(loadBalancerId: string, data: {
  name: string;
  protocol: 'tcp' | 'http' | 'https';
  algorithm?: 'round_robin' | 'least_conn' | 'source_ip' | 'weighted_round_robin';
  health_check_type?: 'tcp' | 'http';
  health_check_path?: string;
  health_check_interval?: number;
  health_check_timeout?: number;
}): Promise<LoadBalancerPool> {
  return apiRequest<LoadBalancerPool>(`/load-balancers/${loadBalancerId}/pools`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deletePool(poolId: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/load-balancers/pools/${poolId}`, {
    method: 'DELETE',
  });
}

export async function createMember(poolId: string, data: {
  instance_id?: string;
  ip_address: string;
  port: number;
  weight?: number;
}): Promise<LoadBalancerMember> {
  return apiRequest<LoadBalancerMember>(`/load-balancers/pools/${poolId}/members`, {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteMember(memberId: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/load-balancers/members/${memberId}`, {
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
  return apiRequest<APIKey[]>('/api-keys');
}

export async function createAPIKey(data: {
  name: string;
  scopes?: string[];
  expires_in_days?: number;
  project_id?: string;
}): Promise<APIKey> {
  return apiRequest<APIKey>('/api-keys', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function revokeAPIKey(id: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/api-keys/${id}`, {
    method: 'DELETE',
  });
}

export async function listWebhooks(): Promise<WebhookEndpoint[]> {
  return apiRequest<WebhookEndpoint[]>('/webhooks');
}

export async function createWebhook(data: {
  url: string;
  description?: string;
  events: string[];
  project_id?: string;
}): Promise<WebhookEndpoint> {
  return apiRequest<WebhookEndpoint>('/webhooks', {
    method: 'POST',
    body: JSON.stringify(data),
  });
}

export async function deleteWebhook(id: string): Promise<{ success: boolean }> {
  return apiRequest<{ success: boolean }>(`/webhooks/${id}`, {
    method: 'DELETE',
  });
}

export async function rotateWebhookSecret(id: string): Promise<{ secret: string }> {
  return apiRequest<{ secret: string }>(`/webhooks/${id}/rotate-secret`, {
    method: 'POST',
  });
}

export async function pingWebhook(id: string): Promise<{ message: string; response_code: number; status: string }> {
  return apiRequest(`/webhooks/${id}/ping`, {
    method: 'POST',
  });
}

export async function listWebhookDeliveries(endpointId: string): Promise<WebhookDelivery[]> {
  return apiRequest<WebhookDelivery[]>(`/webhooks/${endpointId}/deliveries`);
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
