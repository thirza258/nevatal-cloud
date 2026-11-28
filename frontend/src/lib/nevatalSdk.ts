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
      this.request<{ job_id: string }>(`/instances/${id}/shutdown`, { method: 'POST' }),
    reboot: (id: string): Promise<{ job_id: string }> =>
      this.request<{ job_id: string }>(`/instances/${id}/restart`, { method: 'POST' }),
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
    delete: (id: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/networks/${id}`, { method: 'DELETE' }),
    attach: (networkId: string, data: { instance_id: string; ip_address?: string }): Promise<NetworkAttachment> =>
      this.request<NetworkAttachment>(`/networks/${networkId}/attachments`, { method: 'POST', body: JSON.stringify(data) }),
    detach: (networkId: string, attachmentId: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/networks/${networkId}/attachments/${attachmentId}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Cloud Firewalls & Security Rules
  // ==========================================
  public firewalls = {
    list: (): Promise<CloudFirewall[]> => this.request<CloudFirewall[]>('/firewalls'),
    create: (data: { name: string; description?: string; project_id?: string }): Promise<CloudFirewall> =>
      this.request<CloudFirewall>('/firewalls', { method: 'POST', body: JSON.stringify(data) }),
    get: (id: string): Promise<CloudFirewall> => this.request<CloudFirewall>(`/firewalls/${id}`),
    delete: (id: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/firewalls/${id}`, { method: 'DELETE' }),
    addRule: (
      firewallId: string,
      data: {
        direction: 'inbound' | 'outbound';
        action: 'allow' | 'deny';
        protocol: 'tcp' | 'udp' | 'icmp' | 'any';
        port_min?: number;
        port_max?: number;
        source_cidr?: string;
        destination_cidr?: string;
        priority?: number;
      }
    ): Promise<FirewallRule> =>
      this.request<FirewallRule>(`/firewalls/${firewallId}/rules`, { method: 'POST', body: JSON.stringify(data) }),
    deleteRule: (firewallId: string, ruleId: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/firewalls/${firewallId}/rules/${ruleId}`, { method: 'DELETE' }),
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
    delete: (id: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/load-balancers/${id}`, { method: 'DELETE' }),
    createListener: (
      loadBalancerId: string,
      data: { protocol: 'tcp' | 'http' | 'https'; frontend_port: number; certificate_id?: string; default_pool_id?: string }
    ): Promise<LoadBalancerListener> =>
      this.request<LoadBalancerListener>(`/load-balancers/${loadBalancerId}/listeners`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    deleteListener: (loadBalancerId: string, listenerId: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/load-balancers/${loadBalancerId}/listeners/${listenerId}`, { method: 'DELETE' }),
    createPool: (
      loadBalancerId: string,
      data: {
        name: string;
        protocol: 'tcp' | 'http' | 'https';
        port?: number;
        algorithm?: 'round_robin' | 'least_conn' | 'source_ip' | 'weighted_round_robin';
        health_check_config?: string;
      }
    ): Promise<LoadBalancerPool> =>
      this.request<LoadBalancerPool>(`/load-balancers/${loadBalancerId}/pools`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    deletePool: (loadBalancerId: string, poolId: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/load-balancers/${loadBalancerId}/pools/${poolId}`, { method: 'DELETE' }),
    addMember: (
      loadBalancerId: string,
      poolId: string,
      data: { instance_id: string; address?: string; port: number; weight?: number }
    ): Promise<LoadBalancerMember> =>
      this.request<LoadBalancerMember>(`/load-balancers/${loadBalancerId}/pools/${poolId}/members`, {
        method: 'POST',
        body: JSON.stringify(data),
      }),
    deleteMember: (loadBalancerId: string, poolId: string, memberId: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/load-balancers/${loadBalancerId}/pools/${poolId}/members/${memberId}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Floating IPs
  // ==========================================
  public floatingIPs = {
    list: (): Promise<FloatingIP[]> => this.request<FloatingIP[]>('/floating-ips'),
    create: (data: { project_id?: string } = {}): Promise<FloatingIP> =>
      this.request<FloatingIP>('/floating-ips', { method: 'POST', body: JSON.stringify(data) }),
    assign: (id: string, instanceId: string): Promise<FloatingIP> =>
      this.request<FloatingIP>(`/floating-ips/${id}/assign`, {
        method: 'POST',
        body: JSON.stringify({ instance_id: instanceId }),
      }),
    release: (id: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/floating-ips/${id}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Authoritative DNS Zones & Records
  // ==========================================
  public dns = {
    listZones: (): Promise<DNSZone[]> => this.request<DNSZone[]>('/dns/zones'),
    createZone: (data: { domain: string; project_id?: string }): Promise<DNSZone> =>
      this.request<DNSZone>('/dns/zones', { method: 'POST', body: JSON.stringify(data) }),
    getZone: async (id: string): Promise<DNSZone | undefined> => {
      const zones = await this.dns.listZones();
      return zones.find((z) => z.id === id);
    },
    deleteZone: (id: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/dns/zones/${id}`, { method: 'DELETE' }),
    listRecords: (zoneId: string): Promise<DNSRecord[]> =>
      this.request<DNSRecord[]>(`/dns/zones/${zoneId}/records`),
    createRecord: (
      zoneId: string,
      data: { name: string; record_type: string; value: string; ttl?: number }
    ): Promise<DNSRecord> =>
      this.request<DNSRecord>(`/dns/zones/${zoneId}/records`, { method: 'POST', body: JSON.stringify(data) }),
    deleteRecord: (zoneId: string, recordId: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/dns/zones/${zoneId}/records/${recordId}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Scoped API Keys (SHA-256 Hashed At Rest)
  // ==========================================
  public apiKeys = {
    list: (): Promise<APIKey[]> => this.request<APIKey[]>('/api-keys'),
    create: async (data: {
      name: string;
      scopes?: string[];
      expiry_days?: number;
      project_id?: string;
    }): Promise<APIKey> => {
      const res = await this.request<{ key: APIKey; raw_key: string } | APIKey>('/api-keys', {
        method: 'POST',
        body: JSON.stringify(data),
      });
      if (res && typeof res === 'object' && 'key' in res && res.key) {
        return { ...res.key, raw_key: res.raw_key };
      }
      return res as APIKey;
    },
    revoke: (id: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/api-keys/${id}`, { method: 'DELETE' }),
  };

  // ==========================================
  // Webhooks & Event Subscriptions
  // ==========================================
  public webhooks = {
    list: (): Promise<WebhookEndpoint[]> => this.request<WebhookEndpoint[]>('/webhooks'),
    create: (data: {
      name: string;
      url: string;
      events: string[];
      project_id?: string;
    }): Promise<WebhookEndpoint> =>
      this.request<WebhookEndpoint>('/webhooks', { method: 'POST', body: JSON.stringify(data) }),
    delete: (id: string): Promise<{ status: string }> =>
      this.request<{ status: string }>(`/webhooks/${id}`, { method: 'DELETE' }),
    rotateSecret: (id: string): Promise<{ secret: string }> =>
      this.request<{ secret: string }>(`/webhooks/${id}/rotate-secret`, { method: 'POST' }),
    ping: (id: string): Promise<WebhookDelivery> =>
      this.request<WebhookDelivery>(`/webhooks/${id}/ping`, { method: 'POST' }),
    listDeliveries: (endpointId: string): Promise<WebhookDelivery[]> =>
      this.request<WebhookDelivery[]>(`/webhooks/deliveries?endpoint_id=${encodeURIComponent(endpointId)}`),
    retryDelivery: (deliveryId: string): Promise<WebhookDelivery> =>
      this.request<WebhookDelivery>(`/webhooks/deliveries/${deliveryId}/retry`, { method: 'POST' }),
  };
}

export function createNevatalClient(config?: NevatalClientConfig): NevatalClient {
  return new NevatalClient(config);
}
