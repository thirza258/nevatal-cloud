import React, { useState, useEffect } from 'react';
import {
  Zap,
  Shield,
  Activity,
  CheckCircle2,
  AlertCircle,
  RefreshCw,
  Search,
  ExternalLink,
  Lock,
  Unlock,
  Key,
  Database,
  Sliders,
  Code,
  Check,
  ChevronRight,
  Filter,
  Layers,
  Terminal,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { MCPServer, MCPConnection, MCPTool, MCPAuditLog } from '../../types';

interface MCPRegistryProps {
  onNavigate: (path: string, param?: string) => void;
}

export const MCPRegistry: React.FC<MCPRegistryProps> = ({ onNavigate }) => {
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [connections, setConnections] = useState<MCPConnection[]>([]);
  const [auditLogs, setAuditLogs] = useState<MCPAuditLog[]>([]);
  const [activeTab, setActiveTab] = useState<'servers' | 'connections' | 'tools' | 'logs'>('servers');
  const [loading, setLoading] = useState(true);
  const [connectingServerId, setConnectingServerId] = useState<string | null>(null);

  // Tool Catalog Filter & Search
  const [toolSearch, setToolSearch] = useState('');
  const [riskFilter, setRiskFilter] = useState<'all' | 'read' | 'write' | 'destructive'>('all');
  const [selectedToolSchema, setSelectedToolSchema] = useState<MCPTool | null>(null);

  const loadData = async () => {
    try {
      setLoading(true);
      const [srvs, conns, logs] = await Promise.all([
        apiRequest<MCPServer[]>('/integrations/mcp/servers'),
        apiRequest<MCPConnection[]>('/integrations/mcp/connections'),
        apiRequest<MCPAuditLog[]>('/integrations/mcp/audit-logs'),
      ]);
      setServers(srvs || []);
      setConnections(conns || []);
      setAuditLogs(logs || []);
    } catch (err) {
      console.error('Failed to load MCP integration data:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Connect server
  const handleConnect = async (serverId: string) => {
    try {
      setConnectingServerId(serverId);
      await apiRequest('/integrations/mcp/connections', {
        method: 'POST',
        body: JSON.stringify({ server_id: serverId, config_json: '{}' }),
      });
      await loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to connect MCP server');
    } finally {
      setConnectingServerId(null);
    }
  };

  // Disconnect server
  const handleDisconnect = async (connId: string) => {
    if (!confirm('Disconnect this MCP integration? Tool permissions will be revoked.')) return;
    try {
      await apiRequest(`/integrations/mcp/connections/${connId}`, {
        method: 'DELETE',
      });
      await loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to disconnect integration');
    }
  };

  // Toggle tool permission
  const handleTogglePermission = async (connId: string, toolId: string, currentGranted: boolean) => {
    try {
      await apiRequest(`/integrations/mcp/connections/${connId}/permissions`, {
        method: 'PATCH',
        body: JSON.stringify({ tool_id: toolId, granted: !currentGranted }),
      });
      await loadData();
    } catch (err: any) {
      alert(err.message || 'Failed to update tool permission');
    }
  };

  // Extract all tools from servers
  const allTools: (MCPTool & { serverName: string })[] = [];
  servers.forEach((s) => {
    (s.tools || []).forEach((t) => {
      allTools.push({ ...t, serverName: s.display_name });
    });
  });

  const filteredTools = allTools.filter((t) => {
    const matchesSearch = t.name.toLowerCase().includes(toolSearch.toLowerCase()) ||
      t.description.toLowerCase().includes(toolSearch.toLowerCase()) ||
      t.serverName.toLowerCase().includes(toolSearch.toLowerCase());

    if (riskFilter === 'all') return matchesSearch;
    return matchesSearch && t.risk_level === riskFilter;
  });

  const isConnected = (serverId: string) => {
    return connections.some((c) => c.server_id === serverId && c.status === 'connected');
  };

  const getConnection = (serverId: string) => {
    return connections.find((c) => c.server_id === serverId);
  };

  const getRiskBadge = (risk: string) => {
    switch (risk) {
      case 'read':
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">Read-Only</span>;
      case 'write':
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-blue-500/10 text-blue-400 border border-blue-500/20">Resource Creation</span>;
      case 'destructive':
        return <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">Destructive</span>;
      default:
        return null;
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 bg-slate-900/60 p-6 rounded-2xl border border-slate-800 backdrop-blur">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">
              Model Context Protocol (MCP) Registry
            </h1>
            <span className="px-2 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20 text-xs font-semibold">
              v1.0 Standard
            </span>
          </div>
          <p className="text-xs sm:text-sm text-slate-400 mt-1">
            Standardized tool integration layer providing controlled, least-privilege cloud capabilities to the Nevatal AI Cloud Agent.
          </p>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={loadData}
            disabled={loading}
            className="p-2.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white border border-slate-700/60 transition"
            title="Refresh Registry"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => onNavigate('/dashboard/ai')}
            className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs shadow-lg shadow-blue-500/25 transition"
          >
            <Zap className="w-3.5 h-3.5" />
            <span>Launch AI Agent</span>
          </button>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="flex items-center gap-2 border-b border-slate-800 pb-2">
        <button
          onClick={() => setActiveTab('servers')}
          className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition ${
            activeTab === 'servers'
              ? 'bg-blue-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-white hover:bg-slate-800/60'
          }`}
        >
          <Layers className="w-4 h-4" />
          <span>Available Servers ({servers.length})</span>
        </button>

        <button
          onClick={() => setActiveTab('connections')}
          className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition ${
            activeTab === 'connections'
              ? 'bg-blue-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-white hover:bg-slate-800/60'
          }`}
        >
          <CheckCircle2 className="w-4 h-4" />
          <span>Connected Services ({connections.length})</span>
        </button>

        <button
          onClick={() => setActiveTab('tools')}
          className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition ${
            activeTab === 'tools'
              ? 'bg-blue-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-white hover:bg-slate-800/60'
          }`}
        >
          <Code className="w-4 h-4" />
          <span>Tool Catalog ({allTools.length})</span>
        </button>

        <button
          onClick={() => setActiveTab('logs')}
          className={`flex items-center gap-2 px-4 py-2 rounded-xl text-xs font-semibold transition ${
            activeTab === 'logs'
              ? 'bg-blue-600 text-white shadow-sm'
              : 'text-slate-400 hover:text-white hover:bg-slate-800/60'
          }`}
        >
          <Activity className="w-4 h-4" />
          <span>Audit Logs ({auditLogs.length})</span>
        </button>
      </div>

      {/* Tab 1: Available Servers */}
      {activeTab === 'servers' && (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
          {servers.map((s) => {
            const connected = isConnected(s.id);
            const conn = getConnection(s.id);

            return (
              <div
                key={s.id}
                className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800 hover:border-slate-700/80 transition flex flex-col justify-between space-y-4"
              >
                <div>
                  <div className="flex items-center justify-between mb-3">
                    <div className="flex items-center gap-2.5">
                      <div className="w-9 h-9 rounded-xl bg-blue-600/20 border border-blue-500/30 flex items-center justify-center text-blue-400">
                        <Zap className="w-4 h-4" />
                      </div>
                      <div>
                        <h3 className="font-bold text-white text-xs sm:text-sm">{s.display_name}</h3>
                        <span className="text-[10px] text-slate-500 font-mono">{s.name}</span>
                      </div>
                    </div>

                    {connected ? (
                      <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center gap-1">
                        <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
                        Connected
                      </span>
                    ) : (
                      <span className="px-2 py-0.5 rounded-full text-[10px] font-medium bg-slate-800 text-slate-400">
                        Available
                      </span>
                    )}
                  </div>

                  <p className="text-xs text-slate-400 leading-relaxed">{s.description}</p>

                  <div className="mt-4 pt-3 border-t border-slate-800/80 flex items-center justify-between text-[11px] text-slate-500">
                    <span>Transport: <strong className="text-slate-300 font-mono uppercase">{s.transport}</strong></span>
                    <span>Tools: <strong className="text-blue-400 font-mono">{s.tools?.length || 0}</strong></span>
                  </div>
                </div>

                <div className="pt-2">
                  {s.is_first_party ? (
                    <div className="p-2 rounded-xl bg-blue-500/10 border border-blue-500/20 text-center text-xs text-blue-300 font-medium">
                      Core Platform Service (Always Authorized)
                    </div>
                  ) : connected && conn ? (
                    <button
                      onClick={() => handleDisconnect(conn.id)}
                      className="w-full py-2 px-3 rounded-xl bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/20 text-xs font-semibold transition"
                    >
                      Disconnect Integration
                    </button>
                  ) : (
                    <button
                      onClick={() => handleConnect(s.id)}
                      disabled={connectingServerId === s.id}
                      className="w-full py-2 px-3 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow-md transition disabled:opacity-50 flex items-center justify-center gap-1.5"
                    >
                      {connectingServerId === s.id && <RefreshCw className="w-3.5 h-3.5 animate-spin" />}
                      <span>Connect Server</span>
                    </button>
                  )}
                </div>
              </div>
            );
          })}
        </div>
      )}

      {/* Tab 2: Connected Integrations */}
      {activeTab === 'connections' && (
        <div className="space-y-4">
          {connections.length === 0 ? (
            <div className="p-12 text-center text-slate-500 bg-slate-900/60 rounded-2xl border border-slate-800">
              No active MCP connections found. Connect an integration from the Available Servers tab.
            </div>
          ) : (
            connections.map((c) => (
              <div
                key={c.id}
                className="p-5 rounded-2xl bg-slate-900/60 border border-slate-800 flex flex-col md:flex-row md:items-center justify-between gap-4"
              >
                <div className="space-y-1">
                  <div className="flex items-center gap-2">
                    <h3 className="text-sm font-bold text-white">{c.server?.display_name || 'MCP Server'}</h3>
                    <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                      Active
                    </span>
                  </div>
                  <p className="text-xs text-slate-400">{c.server?.description}</p>
                  <div className="text-[11px] text-slate-500 font-mono pt-1">
                    Connected At: {c.created_at ? new Date(c.created_at).toLocaleString() : 'Recent'} • Granted Tools: {c.permissions?.filter(p => p.granted).length || 0}
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <button
                    onClick={() => setActiveTab('tools')}
                    className="px-3 py-1.5 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition"
                  >
                    Manage Tool Permissions
                  </button>
                  {!c.server?.is_first_party && (
                    <button
                      onClick={() => handleDisconnect(c.id)}
                      className="px-3 py-1.5 rounded-xl bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/20 text-xs font-semibold transition"
                    >
                      Disconnect
                    </button>
                  )}
                </div>
              </div>
            ))
          )}
        </div>
      )}

      {/* Tab 3: Tool Catalog & Permissions */}
      {activeTab === 'tools' && (
        <div className="space-y-4">
          {/* Search & Filter */}
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 bg-slate-900/60 p-4 rounded-2xl border border-slate-800">
            <div className="relative flex-1 max-w-md">
              <Search className="w-4 h-4 text-slate-500 absolute left-3 top-1/2 -translate-y-1/2" />
              <input
                type="text"
                value={toolSearch}
                onChange={(e) => setToolSearch(e.target.value)}
                placeholder="Search tools by name, action, or server..."
                className="w-full bg-slate-950 border border-slate-800 rounded-xl pl-9 pr-4 py-2 text-xs text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500"
              />
            </div>

            <div className="flex items-center gap-1.5">
              {(['all', 'read', 'write', 'destructive'] as const).map((r) => (
                <button
                  key={r}
                  onClick={() => setRiskFilter(r)}
                  className={`px-3 py-1.5 rounded-xl text-xs font-medium capitalize transition ${
                    riskFilter === r
                      ? 'bg-blue-600 text-white'
                      : 'bg-slate-800 text-slate-400 hover:text-white'
                  }`}
                >
                  {r === 'all' ? 'All Risks' : r}
                </button>
              ))}
            </div>
          </div>

          {/* Tools Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
            {filteredTools.map((t) => (
              <div
                key={t.id}
                className="p-4 rounded-2xl bg-slate-900/60 border border-slate-800 space-y-2.5 flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs font-bold text-white tracking-tight">{t.name}</span>
                    {getRiskBadge(t.risk_level)}
                  </div>
                  <p className="text-xs text-slate-400 mt-1">{t.description}</p>
                </div>

                <div className="pt-2 border-t border-slate-800/60 flex items-center justify-between text-[11px]">
                  <span className="text-slate-500">Provider: <strong className="text-slate-300 font-normal">{t.serverName}</strong></span>
                  <button
                    onClick={() => setSelectedToolSchema(t)}
                    className="text-blue-400 hover:text-blue-300 font-medium flex items-center gap-1"
                  >
                    <span>Inspect Schema</span>
                    <ChevronRight className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Tab 4: Audit Logs */}
      {activeTab === 'logs' && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-950/60 text-slate-400 uppercase text-[10px] tracking-wider border-b border-slate-800">
                <tr>
                  <th className="px-5 py-3.5">Timestamp</th>
                  <th className="px-4 py-3.5">Tool Name</th>
                  <th className="px-4 py-3.5">Risk Level</th>
                  <th className="px-4 py-3.5">User Confirmed</th>
                  <th className="px-4 py-3.5">Result</th>
                  <th className="px-4 py-3.5">Caller IP</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 text-slate-200">
                {auditLogs.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="px-5 py-12 text-center text-slate-500">
                      No tool executions recorded yet.
                    </td>
                  </tr>
                ) : (
                  auditLogs.map((log) => (
                    <tr key={log.id} className="hover:bg-slate-800/40">
                      <td className="px-5 py-3 font-mono text-[11px] text-slate-400">
                        {new Date(log.created_at).toLocaleString()}
                      </td>
                      <td className="px-4 py-3 font-mono font-medium text-white">{log.tool_name}</td>
                      <td className="px-4 py-3">{getRiskBadge(log.risk_level)}</td>
                      <td className="px-4 py-3">
                        {log.confirmed ? (
                          <span className="text-emerald-400 font-semibold flex items-center gap-1">
                            <Check className="w-3.5 h-3.5" /> Confirmed
                          </span>
                        ) : (
                          <span className="text-slate-500">Automated</span>
                        )}
                      </td>
                      <td className="px-4 py-3">
                        {log.result_status === 'success' ? (
                          <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                            Success
                          </span>
                        ) : (
                          <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-rose-500/10 text-rose-400 border border-rose-500/20">
                            {log.result_status}
                          </span>
                        )}
                      </td>
                      <td className="px-4 py-3 font-mono text-[11px] text-slate-400">{log.ip_address || '127.0.0.1'}</td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {/* JSON Schema Viewer Modal */}
      {selectedToolSchema && (
        <div
          className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4"
          onClick={() => setSelectedToolSchema(null)}
        >
          <div
            className="bg-slate-900 border border-slate-800 rounded-2xl max-w-lg w-full p-5 shadow-2xl space-y-4"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-center justify-between border-b border-slate-800 pb-3">
              <div>
                <h3 className="text-sm font-bold text-white font-mono">{selectedToolSchema.name}</h3>
                <span className="text-xs text-slate-400">{selectedToolSchema.description}</span>
              </div>
              <button
                onClick={() => setSelectedToolSchema(null)}
                className="text-slate-400 hover:text-white"
              >
                ✕
              </button>
            </div>

            <div>
              <div className="text-[10px] uppercase font-bold text-slate-500 mb-1">Input Schema (JSON Schema)</div>
              <pre className="p-3 rounded-xl bg-slate-950 border border-slate-800 text-[11px] font-mono text-slate-300 overflow-x-auto max-h-60">
                {JSON.stringify(JSON.parse(selectedToolSchema.input_schema || '{}'), null, 2)}
              </pre>
            </div>

            <div className="flex justify-end">
              <button
                onClick={() => setSelectedToolSchema(null)}
                className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-white text-xs font-semibold transition"
              >
                Close
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
