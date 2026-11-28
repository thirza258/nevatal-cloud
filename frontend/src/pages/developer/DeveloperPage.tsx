import React, { useState, useEffect } from 'react';
import {
  Code,
  Key,
  Webhook,
  BookOpen,
  Terminal,
  Plus,
  Trash2,
  Copy,
  Check,
  RefreshCw,
  AlertTriangle,
  CheckCircle2,
  AlertCircle,
  ExternalLink,
  ShieldCheck,
  Send,
  Eye,
  Lock,
  Bot,
  Sparkles,
  Layers,
} from 'lucide-react';
import {
  listAPIKeys,
  createAPIKey,
  revokeAPIKey,
  parseAPIKeyScopes,
  listWebhooks,
  createWebhook,
  deleteWebhook,
  rotateWebhookSecret,
  pingWebhook,
  listWebhookDeliveries,
  retryWebhookDelivery,
} from '../../lib/networkingApi';
import type {
  APIKey,
  WebhookEndpoint,
  WebhookDelivery,
} from '../../types';

interface DeveloperPageProps {
  onNavigate?: (path: string) => void;
}

type TabType = 'api_keys' | 'webhooks' | 'sdk' | 'ai_agents' | 'docs' | 'cli';

const AVAILABLE_SCOPES = [
  'instances:read',
  'instances:write',
  'networks:read',
  'networks:write',
  'firewalls:read',
  'firewalls:write',
  'load_balancers:read',
  'load_balancers:write',
  'dns:read',
  'dns:write',
  'webhooks:read',
  'webhooks:write',
];

export const DeveloperPage: React.FC<DeveloperPageProps> = ({ onNavigate }) => {
  const [activeTab, setActiveTab] = useState<TabType>('api_keys');
  const [loading, setLoading] = useState(true);

  // API Keys State
  const [apiKeys, setApiKeys] = useState<APIKey[]>([]);
  const [showCreateKey, setShowCreateKey] = useState(false);
  const [keyName, setKeyName] = useState('');
  const [selectedScopes, setSelectedScopes] = useState<string[]>([
    'instances:read',
    'networks:read',
  ]);
  const [newlyCreatedKey, setNewlyCreatedKey] = useState<APIKey | null>(null);
  const [copiedKey, setCopiedKey] = useState(false);

  // Webhooks State
  const [webhooks, setWebhooks] = useState<WebhookEndpoint[]>([]);
  const [showCreateWebhook, setShowCreateWebhook] = useState(false);
  const [webhookURL, setWebhookURL] = useState('');
  const [webhookDesc, setWebhookDesc] = useState('');
  const [webhookEvents, setWebhookEvents] = useState<string[]>(['*']);
  const [newlyCreatedSecret, setNewlyCreatedSecret] = useState<string | null>(null);
  const [selectedEndpoint, setSelectedEndpoint] = useState<WebhookEndpoint | null>(null);
  const [deliveries, setDeliveries] = useState<WebhookDelivery[]>([]);

  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  const notify = (type: 'success' | 'error', message: string) => {
    setNotification({ type, message });
    setTimeout(() => setNotification(null), 4000);
  };

  const loadData = async () => {
    setLoading(true);
    try {
      const [keys, whs] = await Promise.all([
        listAPIKeys().catch(() => []),
        listWebhooks().catch(() => []),
      ]);
      setApiKeys(keys);
      setWebhooks(whs);
      if (selectedEndpoint) {
        const dels = await listWebhookDeliveries(selectedEndpoint.id).catch(() => []);
        setDeliveries(dels);
      }
    } catch (err: any) {
      notify('error', err.message || 'Failed to load developer settings');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleCreateAPIKey = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const key = await createAPIKey({
        name: keyName,
        scopes: selectedScopes,
      });
      setShowCreateKey(false);
      setKeyName('');
      setNewlyCreatedKey(key);
      notify('success', 'API key generated successfully');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error generating API key');
    }
  };

  const handleRevokeAPIKey = async (id: string) => {
    if (!confirm('Are you sure you want to revoke this API key immediately? This action is permanent.')) return;
    try {
      await revokeAPIKey(id);
      notify('success', 'API key revoked');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error revoking API key');
    }
  };

  const handleCreateWebhook = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const endpoint = await createWebhook({
        name: webhookDesc.trim() || 'Webhook Endpoint',
        url: webhookURL,
        events: webhookEvents,
      });
      setShowCreateWebhook(false);
      setWebhookURL('');
      setWebhookDesc('');
      if (endpoint.secret) setNewlyCreatedSecret(endpoint.secret);
      notify('success', 'Webhook registered successfully');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error registering webhook');
    }
  };

  const handleDeleteWebhook = async (id: string) => {
    if (!confirm('Delete this webhook endpoint and its delivery history?')) return;
    try {
      await deleteWebhook(id);
      if (selectedEndpoint?.id === id) {
        setSelectedEndpoint(null);
        setDeliveries([]);
      }
      notify('success', 'Webhook endpoint deleted');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting webhook');
    }
  };

  const handlePingWebhook = async (id: string) => {
    try {
      const res = await pingWebhook(id);
      notify('success', `Ping queued: ${res.status}${res.last_http_status ? ` (HTTP ${res.last_http_status})` : ''}`);
      if (selectedEndpoint?.id === id) {
        const dels = await listWebhookDeliveries(id);
        setDeliveries(dels);
      }
    } catch (err: any) {
      notify('error', err.message || 'Ping delivery failed');
    }
  };

  const handleSelectEndpoint = async (ep: WebhookEndpoint) => {
    setSelectedEndpoint(ep);
    try {
      const dels = await listWebhookDeliveries(ep.id);
      setDeliveries(dels);
    } catch (err: any) {
      notify('error', err.message || 'Error loading deliveries');
    }
  };

  const handleRetryDelivery = async (delId: string) => {
    try {
      await retryWebhookDelivery(delId);
      notify('success', 'Delivery retry queued');
      if (selectedEndpoint) {
        const dels = await listWebhookDeliveries(selectedEndpoint.id);
        setDeliveries(dels);
      }
    } catch (err: any) {
      notify('error', err.message || 'Error retrying delivery');
    }
  };

  const copyToClipboard = (text: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(true);
    setTimeout(() => setCopiedKey(false), 2500);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight flex items-center gap-2">
            <Terminal className="w-6 h-6 text-indigo-400" />
            Developer Platform
          </h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-1">
            Programmatic access tokens, signed webhooks, OpenAPI specifications, and the official Nevatal CLI.
          </p>
        </div>
        <button
          onClick={loadData}
          className="inline-flex items-center gap-2 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg transition-colors border border-slate-700"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          Refresh
        </button>
      </div>

      {/* Notifications */}
      {notification && (
        <div
          className={`p-3 rounded-xl border flex items-center gap-3 text-xs ${
            notification.type === 'success'
              ? 'bg-emerald-950/40 border-emerald-800/80 text-emerald-300'
              : 'bg-rose-950/40 border-rose-800/80 text-rose-300'
          }`}
        >
          {notification.type === 'success' ? (
            <CheckCircle2 className="w-4 h-4 shrink-0 text-emerald-400" />
          ) : (
            <AlertCircle className="w-4 h-4 shrink-0 text-rose-400" />
          )}
          <span>{notification.message}</span>
        </div>
      )}

      {/* Tabs */}
      <div className="flex border-b border-slate-800 gap-2">
        <button
          onClick={() => setActiveTab('api_keys')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'api_keys'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Key className="w-4 h-4" />
          API Keys ({(apiKeys || []).length})
        </button>
        <button
          onClick={() => setActiveTab('webhooks')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'webhooks'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Webhook className="w-4 h-4" />
          Webhooks ({(webhooks || []).length})
        </button>
        <button
          onClick={() => setActiveTab('sdk')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'sdk'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Code className="w-4 h-4" />
          TypeScript SDK
        </button>
        <button
          onClick={() => setActiveTab('ai_agents')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'ai_agents'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Bot className="w-4 h-4 text-purple-400" />
          AI Agents &amp; MCP
        </button>
        <button
          onClick={() => setActiveTab('docs')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'docs'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <BookOpen className="w-4 h-4" />
          API Reference
        </button>
        <button
          onClick={() => setActiveTab('cli')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'cli'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Code className="w-4 h-4" />
          CLI Tool
        </button>
      </div>

      {/* 1. API KEYS TAB */}
      {activeTab === 'api_keys' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <p className="text-xs text-slate-400">
              Scoped API keys for authenticating scripts, CI/CD pipelines, and the Nevatal CLI with SHA-256 secret hashing.
            </p>
            <button
              onClick={() => setShowCreateKey(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg transition-colors shadow-sm"
            >
              <Plus className="w-3.5 h-3.5" />
              Generate API Key
            </button>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs font-mono">
                <thead>
                  <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 uppercase text-[10px]">
                    <th className="py-3 px-4 font-sans">Name</th>
                    <th className="py-3 px-4">Key Prefix</th>
                    <th className="py-3 px-4 font-sans">Scopes</th>
                    <th className="py-3 px-4 font-sans">Last Used</th>
                    <th className="py-3 px-4 font-sans">Created</th>
                    <th className="py-3 px-4 text-right font-sans">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800">
                  {apiKeys.map((key) => (
                    <tr key={key.id} className="hover:bg-slate-800/30">
                      <td className="py-3 px-4 font-sans font-semibold text-white">{key.name}</td>
                      <td className="py-3 px-4 text-indigo-300">
                        {key.key_prefix}••••••••
                      </td>
                      <td className="py-3 px-4 font-sans">
                        <div className="flex flex-wrap gap-1 max-w-xs">
                          {parseAPIKeyScopes(key.scopes).length > 0 ? (
                            parseAPIKeyScopes(key.scopes).map((s) => (
                              <span
                                key={s}
                                className="px-1.5 py-0.5 rounded bg-slate-800 text-slate-300 text-[10px]"
                              >
                                {s}
                              </span>
                            ))
                          ) : (
                            <span className="text-slate-500 text-xs">All permissions</span>
                          )}
                        </div>
                      </td>
                      <td className="py-3 px-4 font-sans text-slate-400">
                        {key.last_used_at ? new Date(key.last_used_at).toLocaleDateString() : 'Never'}
                      </td>
                      <td className="py-3 px-4 font-sans text-slate-400">
                        {new Date(key.created_at).toLocaleDateString()}
                      </td>
                      <td className="py-3 px-4 text-right font-sans">
                        {key.revoked_at ? (
                          <span className="text-rose-400 text-xs">Revoked</span>
                        ) : (
                          <button
                            onClick={() => handleRevokeAPIKey(key.id)}
                            className="px-2.5 py-1 bg-rose-950/40 hover:bg-rose-900/60 text-rose-300 border border-rose-800/60 rounded text-xs transition-colors"
                          >
                            Revoke
                          </button>
                        )}
                      </td>
                    </tr>
                  ))}
                  {(apiKeys || []).length === 0 && !loading && (
                    <tr>
                      <td colSpan={6} className="py-8 text-center text-slate-500 italic font-sans text-xs">
                        No API keys generated yet.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* 2. WEBHOOKS TAB */}
      {activeTab === 'webhooks' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <p className="text-xs text-slate-400">
              Receive real-time signed HTTP POST payloads whenever cloud infrastructure events occur.
            </p>
            <button
              onClick={() => setShowCreateWebhook(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg transition-colors shadow-sm"
            >
              <Plus className="w-3.5 h-3.5" />
              Register Webhook
            </button>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {/* Endpoints List */}
            <div className="lg:col-span-1 space-y-3">
              <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Webhook Endpoints</h2>
              {(webhooks || []).map((ep) => (
                <div
                  key={ep.id}
                  onClick={() => handleSelectEndpoint(ep)}
                  className={`p-4 rounded-xl border cursor-pointer transition-all space-y-2 ${
                    selectedEndpoint?.id === ep.id
                      ? 'bg-indigo-950/40 border-indigo-500/80 text-white shadow-lg'
                      : 'bg-slate-900 border-slate-800 hover:border-slate-700 text-slate-300'
                  }`}
                >
                  <div className="flex items-start justify-between">
                    <span className="text-xs font-mono font-medium text-indigo-400 truncate max-w-[200px]">
                      {ep.url}
                    </span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleDeleteWebhook(ep.id);
                      }}
                      className="p-1 text-slate-500 hover:text-rose-400"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                  <p className="text-xs text-slate-400">{ep.name || 'No name'}</p>
                  <div className="pt-2 border-t border-slate-800/80 flex items-center justify-between text-[11px] text-slate-500">
                    <span>Secret: {ep.secret_preview}••••</span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handlePingWebhook(ep.id);
                      }}
                      className="text-indigo-400 hover:text-indigo-300 flex items-center gap-1"
                    >
                      <Send className="w-3 h-3" />
                      Ping Test
                    </button>
                  </div>
                </div>
              ))}
              {(webhooks || []).length === 0 && !loading && (
                <div className="p-6 bg-slate-900 border border-slate-800 rounded-xl text-center text-xs text-slate-500">
                  No webhook endpoints registered.
                </div>
              )}
            </div>

            {/* Deliveries Stream */}
            <div className="lg:col-span-2 space-y-3">
              <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider">
                {selectedEndpoint ? `Delivery History (${(deliveries || []).length})` : 'Select an Endpoint'}
              </h2>

              {selectedEndpoint ? (
                <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                  <div className="overflow-x-auto">
                    <table className="w-full text-left text-xs font-mono">
                      <thead>
                        <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 uppercase text-[10px]">
                          <th className="py-2.5 px-3">Status</th>
                          <th className="py-2.5 px-3">Event</th>
                          <th className="py-2.5 px-3">HTTP Code</th>
                          <th className="py-2.5 px-3">Attempts</th>
                          <th className="py-2.5 px-3">Delivered At</th>
                          <th className="py-2.5 px-3 text-right">Action</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-slate-800">
                        {(deliveries || []).map((del) => (
                          <tr key={del.id} className="hover:bg-slate-800/30">
                            <td className="py-2 px-3">
                              <span
                                className={`px-2 py-0.5 rounded text-[10px] ${
                                  del.status === 'delivered'
                                    ? 'bg-emerald-950 text-emerald-300 border border-emerald-800'
                                    : 'bg-rose-950 text-rose-300 border border-rose-800'
                                }`}
                              >
                                {del.status}
                              </span>
                            </td>
                            <td className="py-2 px-3 text-slate-200">{del.event_type}</td>
                            <td className="py-2 px-3 text-indigo-300">{del.last_http_status || '-'}</td>
                            <td className="py-2 px-3 text-slate-400">{del.attempt_count}</td>
                            <td className="py-2 px-3 text-slate-400 font-sans">
                              {new Date(del.created_at).toLocaleTimeString()}
                            </td>
                            <td className="py-2 px-3 text-right">
                              {del.status !== 'delivered' && (
                                <button
                                  onClick={() => handleRetryDelivery(del.id)}
                                  className="text-[11px] text-indigo-400 hover:text-indigo-300 font-sans underline"
                                >
                                  Retry
                                </button>
                              )}
                            </td>
                          </tr>
                        ))}
                        {(deliveries || []).length === 0 && (
                          <tr>
                            <td colSpan={6} className="py-6 text-center text-slate-500 italic font-sans text-xs">
                              No deliveries recorded for this endpoint yet.
                            </td>
                          </tr>
                        )}
                      </tbody>
                    </table>
                  </div>
                </div>
              ) : (
                <div className="p-12 text-center bg-slate-900 border border-slate-800 rounded-xl text-slate-500 text-xs">
                  Select a webhook endpoint on the left to inspect its delivery attempts and response codes.
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* 3. TYPESCRIPT SDK TAB */}
      {activeTab === 'sdk' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <div>
              <h2 className="text-sm font-semibold text-white">Nevatal Cloud TypeScript / JavaScript SDK</h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Full-featured, type-safe client library with complete autocomplete for VMs, VPC Networks, Cloud Firewalls, and Load Balancers.
              </p>
            </div>
            <button
              onClick={() => copyToClipboard(`import { NevatalClient } from './nevatalSdk';\n\nconst client = new NevatalClient({\n  apiKey: 'nv_live_xxxxxxxxxxxxxxxxxxxx',\n  baseUrl: 'https://cloud.nevatal.id/api/v1'\n});`)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg transition-colors shadow-sm"
            >
              <Copy className="w-3.5 h-3.5" />
              Copy Starter Snippet
            </button>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-5">
            <div>
              <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">1. Installation</h3>
              <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-emerald-400 font-mono overflow-x-auto">
{`# Available for Node.js, Bun, and Deno
npm install @nevatal/sdk
# or import local module:
import { NevatalClient } from '@/lib/nevatalSdk';`}
              </pre>
            </div>

            <div>
              <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">2. Client Initialization</h3>
              <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-indigo-300 font-mono overflow-x-auto">
{`import { NevatalClient } from '@nevatal/sdk';

// Initialize with your scoped API key (nv_live_...)
const nevatal = new NevatalClient({
  apiKey: process.env.NEVATAL_API_KEY, // 'nv_live_xxxxxxxxxxxxxxxxxxxx'
  baseUrl: 'https://cloud.nevatal.id/api/v1',
  timeoutMs: 15000,
});`}
              </pre>
            </div>

            <div>
              <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">3. Example Infrastructure Operations</h3>
              <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-slate-300 font-mono overflow-x-auto">
{`// 1. List Virtual Machines
const instances = await nevatal.instances.list();
console.log('Active instances:', instances.map(i => ({ name: i.name, ip: i.private_ip })));

// 2. Provision a Private VPC Network
const network = await nevatal.networks.create({
  name: 'prod-backend-vpc',
  cidr: '10.200.0.0/24',
  description: 'Private backend database subnet'
});

// 3. Attach a Virtual Machine to the Private Network
const attachment = await nevatal.networks.attach(network.id, {
  instance_id: instances[0].id,
  ip_address: '10.200.0.10'
});

// 4. Create a Stateful Cloud Firewall & Security Rules
const firewall = await nevatal.firewalls.create({
  name: 'web-ingress-policy'
});

await nevatal.firewalls.addRule(firewall.id, {
  direction: 'inbound',
  action: 'allow',
  protocol: 'tcp',
  port_range: '443',
  cidr: '0.0.0.0/0',
  description: 'Public HTTPS traffic'
});

// 5. Deploy a Managed HAProxy Load Balancer
const lb = await nevatal.loadBalancers.create({
  name: 'api-gateway-lb',
  algorithm: 'round_robin',
  network_id: network.id
});

// Create Backend Pool with Health Check
const pool = await nevatal.loadBalancers.createPool(lb.id, {
  name: 'api-pool',
  protocol: 'http',
  health_check_type: 'http',
  health_check_path: '/health'
});

// Add Backend VM Member
await nevatal.loadBalancers.addMember(pool.id, {
  instance_id: instances[0].id,
  ip_address: '10.200.0.10',
  port: 8080,
  weight: 100
});`}
              </pre>
            </div>
          </div>
        </div>
      )}

      {/* 4. AI AGENTS & MCP INTEGRATION TAB */}
      {activeTab === 'ai_agents' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <div>
              <h2 className="text-sm font-semibold text-white">AI Agent &amp; MCP Integration</h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Equip autonomous agents (OpenClaw, Claude Code, Cursor, LangChain) with tools to control Nevatal Cloud.
              </p>
            </div>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-5">
            <div>
              <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                1. Model Context Protocol (MCP) Configuration
              </h3>
              <p className="text-xs text-slate-400 mb-2">
                Configure your local Claude Desktop, Cursor, or OpenClaw agent to access Nevatal Cloud tools via CLI or HTTP:
              </p>
              <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-purple-300 font-mono overflow-x-auto">
{`// claude_desktop_config.json or .cursor/mcp.json
{
  "mcpServers": {
    "nevatal-cloud": {
      "command": "nevatal",
      "args": ["--api-url", "https://cloud.nevatal.id", "--output", "json"],
      "env": {
        "NEVATAL_API_KEY": "nv_live_xxxxxxxxxxxxxxxxxxxxxxxx"
      }
    }
  }
}`}
              </pre>
            </div>

            <div>
              <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                2. Direct Agentic Tool Execution API
              </h3>
              <p className="text-xs text-slate-400 mb-2">
                AI agents can query the registry and call tools programmatically with Scoped API Keys:
              </p>
              <div className="space-y-2">
                <div className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs font-mono">
                  <span className="text-emerald-400">GET</span> <span className="text-slate-300">/api/v1/ai/tools</span>
                  <p className="text-slate-500 font-sans mt-1">Lists all registered tools with JSONSchema input specifications.</p>
                </div>
                <div className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs font-mono">
                  <span className="text-indigo-400">POST</span> <span className="text-slate-300">/api/v1/ai/tools/call</span>
                  <p className="text-slate-500 font-sans mt-1">Executes or plans a tool invocation (e.g. create network, check quota).</p>
                </div>
              </div>
            </div>

            <div>
              <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider mb-2">
                3. Registered Tools Available for AI Agents
              </h3>
              <div className="overflow-x-auto">
                <table className="w-full text-left text-xs font-mono">
                  <thead>
                    <tr className="border-b border-slate-800 text-slate-500 uppercase text-[10px]">
                      <th className="py-2 px-3">Tool ID</th>
                      <th className="py-2 px-3">Risk Level</th>
                      <th className="py-2 px-3 font-sans">Description</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-800/60">
                    <tr className="hover:bg-slate-800/30">
                      <td className="py-2 px-3 text-indigo-400">vps.list</td>
                      <td className="py-2 px-3 text-emerald-400">Read</td>
                      <td className="py-2 px-3 font-sans text-slate-300">List user&apos;s virtual machines and statuses.</td>
                    </tr>
                    <tr className="hover:bg-slate-800/30">
                      <td className="py-2 px-3 text-indigo-400">network.list</td>
                      <td className="py-2 px-3 text-emerald-400">Read</td>
                      <td className="py-2 px-3 font-sans text-slate-300">List caller&apos;s VPC private networks.</td>
                    </tr>
                    <tr className="hover:bg-slate-800/30">
                      <td className="py-2 px-3 text-indigo-400">network.create</td>
                      <td className="py-2 px-3 text-amber-400">Medium</td>
                      <td className="py-2 px-3 font-sans text-slate-300">Create private network with CIDR block.</td>
                    </tr>
                    <tr className="hover:bg-slate-800/30">
                      <td className="py-2 px-3 text-indigo-400">firewall.rule.add</td>
                      <td className="py-2 px-3 text-amber-400">Medium</td>
                      <td className="py-2 px-3 font-sans text-slate-300">Add allow/deny security policy rule.</td>
                    </tr>
                    <tr className="hover:bg-slate-800/30">
                      <td className="py-2 px-3 text-indigo-400">loadbalancer.create</td>
                      <td className="py-2 px-3 text-amber-400">Medium</td>
                      <td className="py-2 px-3 font-sans text-slate-300">Deploy HAProxy managed load balancer.</td>
                    </tr>
                    <tr className="hover:bg-slate-800/30">
                      <td className="py-2 px-3 text-indigo-400">developer.apikey.list</td>
                      <td className="py-2 px-3 text-emerald-400">Read</td>
                      <td className="py-2 px-3 font-sans text-slate-300">List active API keys and scopes.</td>
                    </tr>
                  </tbody>
                </table>
              </div>
            </div>
          </div>
        </div>
      )}

      {/* 5. API REFERENCE TAB */}
      {activeTab === 'docs' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <p className="text-xs text-slate-400">
              OpenAPI 3.1 Specification for Nevatal Cloud Public API.
            </p>
            <a
              href="/api/v1/openapi.json"
              target="_blank"
              rel="noreferrer"
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg transition-colors border border-slate-700"
            >
              <ExternalLink className="w-3.5 h-3.5" />
              Download openapi.json
            </a>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-4">
            <h3 className="text-sm font-semibold text-white">Authentication</h3>
            <p className="text-xs text-slate-300 leading-relaxed">
              Nevatal Cloud supports Bearer tokens or the <code className="bg-slate-950 px-1 py-0.5 rounded text-indigo-300 font-mono">X-API-Key</code> header for API requests:
            </p>
            <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-indigo-300 font-mono overflow-x-auto">
{`curl -X GET "https://cp.cloud.nevatal.com/api/v1/instances" \\
  -H "X-API-Key: nv_live_xxxxxxxxxxxxxxxxxxxxxxxx"`}
            </pre>

            <h3 className="text-sm font-semibold text-white pt-4">Signature Verification for Webhooks</h3>
            <p className="text-xs text-slate-300 leading-relaxed">
              Every webhook request includes an <code className="bg-slate-950 px-1 py-0.5 rounded text-indigo-300 font-mono">X-Nevatal-Signature</code> header calculated as HMAC-SHA256:
            </p>
            <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-indigo-300 font-mono overflow-x-auto">
{`// Signature header format:
X-Nevatal-Signature: t=1700000000,v1=5d41402abc4b2a76b9719d911017c592...

// Compute signature:
HMAC_SHA256(webhook_secret, timestamp + "." + payload)`}
            </pre>
          </div>
        </div>
      )}

      {/* 4. CLI GUIDE TAB */}
      {activeTab === 'cli' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <p className="text-xs text-slate-400">
              Official command-line interface for managing Nevatal Cloud from your terminal.
            </p>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 space-y-5">
            <div>
              <h3 className="text-sm font-semibold text-white mb-2">1. Installation</h3>
              <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-emerald-400 font-mono overflow-x-auto">
{`# Build standalone CLI binary
git clone https://github.com/thirza258/nevatal-cloud.git
cd nevatal-cloud/backend
go build -o /usr/local/bin/nevatal ./cmd/cli`}
              </pre>
            </div>

            <div>
              <h3 className="text-sm font-semibold text-white mb-2">2. Quick Authentication</h3>
              <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-indigo-300 font-mono overflow-x-auto">
{`# Authenticate using your generated API key
nevatal login --api-key nv_live_xxxxxxxxxxxx --api-url https://cp.cloud.nevatal.com

# Confirm identity
nevatal whoami`}
              </pre>
            </div>

            <div>
              <h3 className="text-sm font-semibold text-white mb-2">3. Common Automation Commands</h3>
              <pre className="bg-slate-950 border border-slate-800 rounded-lg p-3 text-xs text-indigo-300 font-mono overflow-x-auto">
{`# Manage Virtual Machines
nevatal vm list
nevatal vm start <vm-id>
nevatal vm stop <vm-id>

# Manage Private Networks
nevatal network list
nevatal network create --name app-tier --cidr 10.100.0.0/24
nevatal network attach <net-id> --vm-id <vm-id>

# Cloud Firewalls
nevatal firewall list
nevatal firewall rule add <fw-id> --direction inbound --action allow --protocol tcp --port 80,443

# Load Balancers
nevatal lb list
nevatal lb create --name edge-lb --algo round_robin

# Output Formats
nevatal vm list -o json
nevatal network list -o yaml`}
              </pre>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: Generate API Key */}
      {showCreateKey && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Generate API Key</h3>
            <form onSubmit={handleCreateAPIKey} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Key Description / Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. CI/CD Deployment Token"
                  value={keyName}
                  onChange={(e) => setKeyName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-2">Permissions &amp; Scopes</label>
                <div className="grid grid-cols-2 gap-2 max-h-48 overflow-y-auto p-2 bg-slate-950 border border-slate-800 rounded-lg">
                  {AVAILABLE_SCOPES.map((sc) => (
                    <label key={sc} className="flex items-center gap-2 text-xs text-slate-300 cursor-pointer">
                      <input
                        type="checkbox"
                        checked={selectedScopes.includes(sc)}
                        onChange={(e) => {
                          if (e.target.checked) setSelectedScopes([...selectedScopes, sc]);
                          else setSelectedScopes(selectedScopes.filter((s) => s !== sc));
                        }}
                        className="rounded border-slate-700 text-indigo-600 focus:ring-0"
                      />
                      <span className="font-mono text-[11px]">{sc}</span>
                    </label>
                  ))}
                </div>
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateKey(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Generate Key
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: One-time API Key Reveal */}
      {newlyCreatedKey && (
        <div className="fixed inset-0 bg-black/80 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-indigo-500/50 rounded-2xl w-full max-w-lg p-6 shadow-2xl space-y-4">
            <div className="flex items-center gap-3 text-indigo-400">
              <ShieldCheck className="w-6 h-6" />
              <h3 className="text-base font-bold text-white">Your New API Key</h3>
            </div>

            <div className="p-3 bg-rose-950/40 border border-rose-800/80 rounded-xl text-xs text-rose-300 flex items-start gap-2">
              <AlertTriangle className="w-4 h-4 shrink-0 text-rose-400 mt-0.5" />
              <span>
                Please save this secret key now. For your security, this key will <strong>never be displayed again</strong>.
              </span>
            </div>

            <div className="flex items-center justify-between bg-slate-950 border border-slate-800 rounded-lg p-3">
              <span className="font-mono text-xs text-indigo-300 break-all select-all">
                {newlyCreatedKey.raw_key}
              </span>
              <button
                onClick={() => copyToClipboard(newlyCreatedKey.raw_key || '')}
                className="ml-2 p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded transition-colors"
                title="Copy Key"
              >
                {copiedKey ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
              </button>
            </div>

            <div className="flex justify-end pt-2">
              <button
                onClick={() => setNewlyCreatedKey(null)}
                className="px-4 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg"
              >
                I have saved this key safely
              </button>
            </div>
          </div>
        </div>
      )}

      {/* MODAL: Register Webhook */}
      {showCreateWebhook && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Register Webhook Endpoint</h3>
            <form onSubmit={handleCreateWebhook} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Payload URL</label>
                <input
                  type="url"
                  required
                  placeholder="https://api.yourdomain.com/webhooks"
                  value={webhookURL}
                  onChange={(e) => setWebhookURL(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white font-mono"
                />
                <p className="text-[11px] text-slate-500 mt-1">
                  Target must be an external HTTPS endpoint (internal/loopback addresses blocked).
                </p>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Description</label>
                <input
                  type="text"
                  placeholder="Slack / Discord notification relay"
                  value={webhookDesc}
                  onChange={(e) => setWebhookDesc(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Event Subscriptions</label>
                <select
                  value={webhookEvents[0]}
                  onChange={(e) => setWebhookEvents([e.target.value])}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                >
                  <option value="*">All Events (*)</option>
                  <option value="instance.created">instance.created</option>
                  <option value="instance.started">instance.started</option>
                  <option value="instance.stopped">instance.stopped</option>
                  <option value="instance.deleted">instance.deleted</option>
                  <option value="network.attached">network.attached</option>
                </select>
              </div>

              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateWebhook(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Register Endpoint
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
