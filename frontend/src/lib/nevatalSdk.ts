/**
 * Nevatal Cloud TypeScript SDK (@nevatal/sdk)
 * 
 * Official type-safe client library for automating Nevatal Cloud infrastructure.
 * Compatible with Node.js 18+, Bun, Deno, and modern browser environments.
 */

import type {
  Instance,
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

export interface NevatalClientConfig {
  /** API Key starting with nv_live_ */
  apiKey?: string;
  /** JWT Bearer token */
  token?: string;
  /** Nevatal Cloud API base URL (defaults to https://cp.cloud.nevatal.com or /api/v1) */
  baseUrl?: string;
  /** Timeout in milliseconds (default: 30000ms) */
  timeoutMs?: number;
}

export class NevatalAPIError extends Error {
  public status: number;
  public code: string;
  public details?: Record<string, unknown>;

  constructor(message: string, status: number, code: string, details?: Record<string, unknown>) {
    super(message);
    this.name = 'NevatalAPIError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export class NevatalClient {
  private baseUrl: string;
  private apiKey?: string;
  private token?: string;
  private timeoutMs: number;

  constructor(config: NevatalClientConfig = {}) {
    this.baseUrl = (config.baseUrl || '/api/v1').replace(/\/+$/, '');
    this.apiKey = config.apiKey;
    this.token = config.token;
    this.timeoutMs = config.timeoutMs || 30000;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeoutMs);

    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      Accept: 'application/json',
      ...(options.headers as Record<string, string>),
    };

    if (this.apiKey) {
      headers['X-API-Key'] = this.apiKey;
    } else if (this.token) {
      headers['Authorization'] = `Bearer ${this.token}`;
    }

    try {
      const url = `${this.baseUrl}${endpoint.startsWith('/') ? '' : '/'}${endpoint}`;
      const res = await fetch(url, {
        ...options,
        headers,
        signal: controller.signal,
      });

      const body = await res.json().catch(() => null);

      if (!res.ok) {
        const errorInfo = body?.error;
        throw new NevatalAPIError(
          errorInfo?.message || `HTTP ${res.status} request failed`,
          res.status,
          errorInfo?.code || 'API_ERROR',
          errorInfo?.details
        );
      }

      return (body?.data ?? body) as T;
    } finally {
      clearTimeout(timeoutId);
    }
  }

  // ==========================================
  // Compute Instances (VMs)
  // ==========================================
  public instances = {
    list: (): Promise<Instance[]> => this.request<Instance[]>('/instances'),
    get: (id: string): Promise<Instance> => this.request<Instance>(`/instances/${id}`),
    start: (id: string): Promise<{ job_id: string }> =>
      this.request<{ job_id: string }>(`/instances/${id}/start`, { method: 'POST' }),
    stop: (id: string): Promise<{ job_id: string }> =>
      this.request<{ job_id: string }>(`/instances/${id}/stop`, { method: 'POST' }),
    reboot: (id: string): Promise<{ job_id: string }> =>
      this.request<{ job_id: string }>(`/instances/${id}/reboot`, { method: 'POST' }),
    delete: (id: string): Promise<{ job_id: string }> =>
      this.request<{ job_id: string }>(`/instances/${id}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Private VPC Networks & IPAM
  // ==========================================
  public networks = {
    list: (): Promise<PrivateNetwork[]> => this.request<PrivateNetwork[]>('/networks'),
    create: (data: { name: string; cidr: string; description?: string; project_id?: string }): Promise<PrivateNetwork> =>
      this.request<PrivateNetwork>('/networks', { method: 'POST', body: JSON.stringify(data) }),
    get: (id: string): Promise<PrivateNetwork> => this.request<PrivateNetwork>(`/networks/${id}`),
    delete: (id: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/networks/${id}`, { method: 'DELETE' }),
    attach: (networkId: string, data: { instance_id: string; ip_address?: string }): Promise<NetworkAttachment> =>
      this.request<NetworkAttachment>(`/networks/${networkId}/attachments`, { method: 'POST', body: JSON.stringify(data) }),
    detach: (attachmentId: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/networks/attachments/${attachmentId}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Cloud Firewalls & Security Rules
  // ==========================================
  public firewalls = {
    list: (): Promise<CloudFirewall[]> => this.request<CloudFirewall[]>('/firewalls'),
    create: (data: { name: string; description?: string; project_id?: string }): Promise<CloudFirewall> =>
      this.request<CloudFirewall>('/firewalls', { method: 'POST', body: JSON.stringify(data) }),
    get: (id: string): Promise<CloudFirewall> => this.request<CloudFirewall>(`/firewalls/${id}`),
    delete: (id: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/firewalls/${id}`, { method: 'DELETE' }),
    addRule: (
      firewallId: string,
      data: {
        direction: 'inbound' | 'outbound';
        action: 'allow' | 'deny';
        protocol: 'tcp' | 'udp' | 'icmp' | 'all';
        port_range?: string;
        cidr: string;
        description?: string;
        priority?: number;
      }
    ): Promise<FirewallRule> =>
      this.request<FirewallRule>(`/firewalls/${firewallId}/rules`, { method: 'POST', body: JSON.stringify(data) }),
    deleteRule: (ruleId: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/firewalls/rules/${ruleId}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Managed Load Balancers (HAProxy L4/L7)
  // ==========================================
  public loadBalancers = {
    list: (): Promise<LoadBalancer[]> => this.request<LoadBalancer[]>('/load-balancers'),
    create: (data: {
      name: string;
      network_id?: string;
      algorithm?: 'round_robin' | 'least_conn' | 'source_ip' | 'weighted_round_robin';
      project_id?: string;
    }): Promise<LoadBalancer> =>
      this.request<LoadBalancer>('/load-balancers', { method: 'POST', body: JSON.stringify(data) }),
    get: (id: string): Promise<LoadBalancer> => this.request<LoadBalancer>(`/load-balancers/${id}`),
    delete: (id: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/load-balancers/${id}`, { method: 'DELETE' }),
    createListener: (
      loadBalancerId: string,
      data: { name: string; protocol: 'tcp' | 'http' | 'https'; port: number; pool_id?: string }
    ): Promise<LoadBalancerListener> =>
      this.request<LoadBalancerListener>(`/load-balancers/${loadBalancerId}/listeners`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    deleteListener: (listenerId: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/load-balancers/listeners/${listenerId}`, { method: 'DELETE' }),
    createPool: (
      loadBalancerId: string,
      data: {
        name: string;
        protocol: 'tcp' | 'http' | 'https';
        algorithm?: 'round_robin' | 'least_conn' | 'source_ip' | 'weighted_round_robin';
        health_check_type?: 'tcp' | 'http';
        health_check_path?: string;
      }
    ): Promise<LoadBalancerPool> =>
      this.request<LoadBalancerPool>(`/load-balancers/${loadBalancerId}/pools`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    deletePool: (poolId: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/load-balancers/pools/${poolId}`, { method: 'DELETE' }),
    addMember: (
      poolId: string,
      data: { instance_id?: string; ip_address: string; port: number; weight?: number }
    ): Promise<LoadBalancerMember> =>
      this.request<LoadBalancerMember>(`/load-balancers/pools/${poolId}/members`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    deleteMember: (memberId: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/load-balancers/members/${memberId}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Floating IPs
  // ==========================================
  public floatingIPs = {
    list: (): Promise<FloatingIP[]> => this.request<FloatingIP[]>('/floating-ips'),
    create: (data: { description?: string; project_id?: string }): Promise<FloatingIP> =>
      this.request<FloatingIP>('/floating-ips', { method: 'POST', body: JSON.stringify(data) }),
    assign: (id: string, instanceId: string): Promise<FloatingIP> =>
      this.request<FloatingIP>(`/floating-ips/${id}/assign`, {
        method: 'POST',
        body: JSON.stringify({ instance_id: instanceId }),
      }),
    release: (id: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/floating-ips/${id}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Authoritative DNS Zones & Records
  // ==========================================
  public dns = {
    listZones: (): Promise<DNSZone[]> => this.request<DNSZone[]>('/dns/zones'),
    createZone: (data: { name: string; description?: string; project_id?: string }): Promise<DNSZone> =>
      this.request<DNSZone>('/dns/zones', { method: 'POST', body: JSON.stringify(data) }),
    getZone: (id: string): Promise<DNSZone> => this.request<DNSZone>(`/dns/zones/${id}`),
    deleteZone: (id: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/dns/zones/${id}`, { method: 'DELETE' }),
    listRecords: (zoneId: string): Promise<DNSRecord[]> =>
      this.request<DNSRecord[]>(`/dns/zones/${zoneId}/records`),
    createRecord: (
      zoneId: string,
      data: { name: string; type: string; content: string; ttl?: number; priority?: number }
    ): Promise<DNSRecord> =>
      this.request<DNSRecord>(`/dns/zones/${zoneId}/records`, { method: 'POST', body: JSON.stringify(data) }),
    deleteRecord: (recordId: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/dns/records/${recordId}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Scoped API Keys (SHA-256 Hashed At Rest)
  // ==========================================
  public apiKeys = {
    list: (): Promise<APIKey[]> => this.request<APIKey[]>('/api-keys'),
    create: (data: {
      name: string;
      scopes?: string[];
      expires_in_days?: number;
      project_id?: string;
    }): Promise<APIKey> =>
      this.request<APIKey>('/api-keys', { method: 'POST', body: JSON.stringify(data) }),
    revoke: (id: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/api-keys/${id}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Webhooks & Event Subscriptions
  // ==========================================
  public webhooks = {
    list: (): Promise<WebhookEndpoint[]> => this.request<WebhookEndpoint[]>('/webhooks'),
    create: (data: {
      url: string;
      description?: string;
      events: string[];
      project_id?: string;
    }): Promise<WebhookEndpoint> =>
      this.request<WebhookEndpoint>('/webhooks', { method: 'POST', body: JSON.stringify(data) }),
    delete: (id: string): Promise<{ success: boolean }> =>
      this.request<{ success: boolean }>(`/webhooks/${id}`, { method: 'DELETE' }),
    rotateSecret: (id: string): Promise<{ secret: string }> =>
      this.request<{ secret: string }>(`/webhooks/${id}/rotate-secret`, { method: 'POST' }),
    ping: (id: string): Promise<{ message: string; response_code: number; status: string }> =>
      this.request(`/webhooks/${id}/ping`, { method: 'POST' }),
    listDeliveries: (endpointId: string): Promise<WebhookDelivery[]> =>
      this.request<WebhookDelivery[]>(`/webhooks/${endpointId}/deliveries`),
    retryDelivery: (deliveryId: string): Promise<{ message: string }> =>
      this.request(`/webhooks/deliveries/${deliveryId}/retry`, { method: 'POST' }),
  };
}

export function createNevatalClient(config?: NevatalClientConfig): NevatalClient {
  return new NevatalClient(config);
}
