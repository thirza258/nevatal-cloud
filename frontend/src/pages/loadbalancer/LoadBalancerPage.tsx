import React, { useState, useEffect } from 'react';
import {
  Cpu,
  Layers,
  Server,
  Activity,
  Plus,
  Trash2,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  FileCode,
  ShieldCheck,
  ChevronDown,
  ChevronRight,
  ArrowDownUp,
  Settings,
  HeartPulse,
} from 'lucide-react';
import {
  listLoadBalancers,
  createLoadBalancer,
  getLoadBalancer,
  deleteLoadBalancer,
  createListener,
  deleteListener,
  createPool,
  deletePool,
  createMember,
  deleteMember,
  listNetworks,
} from '../../lib/networkingApi';
import { apiRequest } from '../../lib/api';
import type {
  LoadBalancer,
  LoadBalancerListener,
  LoadBalancerPool,
  LoadBalancerMember,
  PrivateNetwork,
  Instance,
} from '../../types';

interface LoadBalancerPageProps {
  onNavigate?: (path: string) => void;
}

export const LoadBalancerPage: React.FC<LoadBalancerPageProps> = ({ onNavigate }) => {
  const [loadBalancers, setLoadBalancers] = useState<LoadBalancer[]>([]);
  const [networks, setNetworks] = useState<PrivateNetwork[]>([]);
  const [instances, setInstances] = useState<Instance[]>([]);
  const [loading, setLoading] = useState(true);
  const [selectedLB, setSelectedLB] = useState<LoadBalancer | null>(null);

  // Modals
  const [showCreateLB, setShowCreateLB] = useState(false);
  const [lbName, setLbName] = useState('');
  const [lbNetworkId, setLbNetworkId] = useState('');
  const [lbAlgorithm, setLbAlgorithm] = useState('round_robin');

  const [showAddListener, setShowAddListener] = useState(false);
  const [listenerName, setListenerName] = useState('http-80');
  const [listenerProto, setListenerProto] = useState<'http' | 'tcp' | 'https'>('http');
  const [listenerPort, setListenerPort] = useState(80);
  const [listenerPoolId, setListenerPoolId] = useState('');

  const [showAddPool, setShowAddPool] = useState(false);
  const [poolName, setPoolName] = useState('web-pool');
  const [poolProto, setPoolProto] = useState<'http' | 'tcp' | 'https'>('http');
  const [poolAlgo, setPoolAlgo] = useState<'round_robin' | 'least_conn' | 'source_ip' | 'weighted_round_robin'>('round_robin');
  const [poolHCType, setPoolHCType] = useState<'http' | 'tcp'>('http');
  const [poolHCPath, setPoolHCPath] = useState('/health');

  const [showAddMember, setShowAddMember] = useState(false);
  const [memberPoolId, setMemberPoolId] = useState('');
  const [memberInstanceId, setMemberInstanceId] = useState('');
  const [memberIP, setMemberIP] = useState('');
  const [memberPort, setMemberPort] = useState(80);
  const [memberWeight, setMemberWeight] = useState(1);

  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  const notify = (type: 'success' | 'error', message: string) => {
    setNotification({ type, message });
    setTimeout(() => setNotification(null), 4000);
  };

  const loadData = async () => {
    setLoading(true);
    try {
      const [lbs, nets, insts] = await Promise.all([
        listLoadBalancers().catch(() => []),
        listNetworks().catch(() => []),
        apiRequest<Instance[]>('/instances').catch(() => []),
      ]);
      setLoadBalancers(lbs);
      setNetworks(nets);
      setInstances(insts);
      if (selectedLB) {
        const updated = lbs.find((item) => item.id === selectedLB.id);
        if (updated) setSelectedLB(updated);
      }
    } catch (err: any) {
      notify('error', err.message || 'Failed to load load balancers');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  const handleCreateLB = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await createLoadBalancer({
        name: lbName,
        network_id: lbNetworkId || undefined,
        algorithm: lbAlgorithm,
      });
      setShowCreateLB(false);
      setLbName('');
      notify('success', 'Load Balancer created');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error creating Load Balancer');
    }
  };

  const handleDeleteLB = async (id: string) => {
    if (!confirm('Are you sure you want to delete this Load Balancer?')) return;
    try {
      await deleteLoadBalancer(id);
      if (selectedLB?.id === id) setSelectedLB(null);
      notify('success', 'Load Balancer deleted');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting Load Balancer');
    }
  };

  const handleAddListener = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedLB) return;
    try {
      await createListener(selectedLB.id, {
        name: listenerName,
        protocol: listenerProto,
        port: listenerPort,
        pool_id: listenerPoolId || undefined,
      });
      setShowAddListener(false);
      notify('success', 'Frontend listener added');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error adding listener');
    }
  };

  const handleDeleteListener = async (id: string) => {
    try {
      await deleteListener(id);
      notify('success', 'Listener removed');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting listener');
    }
  };

  const handleAddPool = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedLB) return;
    try {
      await createPool(selectedLB.id, {
        name: poolName,
        protocol: poolProto,
        algorithm: poolAlgo,
        health_check_type: poolHCType,
        health_check_path: poolHCType === 'http' ? poolHCPath : undefined,
      });
      setShowAddPool(false);
      notify('success', 'Backend pool added');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error adding pool');
    }
  };

  const handleDeletePool = async (id: string) => {
    try {
      await deletePool(id);
      notify('success', 'Backend pool deleted');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting pool');
    }
  };

  const handleAddMember = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!memberPoolId) return;
    try {
      await createMember(memberPoolId, {
        instance_id: memberInstanceId || undefined,
        ip_address: memberIP,
        port: memberPort,
        weight: memberWeight,
      });
      setShowAddMember(false);
      setMemberIP('');
      setMemberInstanceId('');
      notify('success', 'Backend member added');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error adding member');
    }
  };

  const handleDeleteMember = async (id: string) => {
    try {
      await deleteMember(id);
      notify('success', 'Backend member removed');
      loadData();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting member');
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight flex items-center gap-2">
            <Cpu className="w-6 h-6 text-indigo-400" />
            Managed Load Balancing
          </h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-1">
            HAProxy-powered layer 4 and layer 7 load balancers with active health checking, TLS termination, and auto-failover.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={loadData}
            className="inline-flex items-center gap-2 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs font-medium rounded-lg transition-colors border border-slate-700"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button
            onClick={() => setShowCreateLB(true)}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg transition-colors shadow-sm"
          >
            <Plus className="w-3.5 h-3.5" />
            Create Load Balancer
          </button>
        </div>
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

      {/* Content Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
        {/* Left: Load Balancers List */}
        <div className="lg:col-span-1 space-y-3">
          <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Load Balancers</h2>
          {(loadBalancers || []).map((lb) => (
            <div
              key={lb.id}
              onClick={() => setSelectedLB(lb)}
              className={`p-4 rounded-xl border cursor-pointer transition-all flex flex-col justify-between ${
                selectedLB?.id === lb.id
                  ? 'bg-indigo-950/40 border-indigo-500/80 text-white shadow-lg'
                  : 'bg-slate-900 border-slate-800 hover:border-slate-700 text-slate-300'
              }`}
            >
              <div className="flex items-start justify-between">
                <div>
                  <h3 className="text-sm font-semibold">{lb.name}</h3>
                  <p className="text-xs font-mono text-indigo-400 mt-0.5">VIP: {lb.vip_address || 'Allocating...'}</p>
                </div>
                <span
                  className={`px-2 py-0.5 text-[10px] rounded font-medium ${
                    lb.status === 'active'
                      ? 'bg-emerald-950 text-emerald-300 border border-emerald-800'
                      : 'bg-amber-950 text-amber-300 border border-amber-800'
                  }`}
                >
                  {lb.status}
                </span>
              </div>

              <div className="mt-3 pt-2.5 border-t border-slate-800/80 flex items-center justify-between text-xs text-slate-400">
                <span>Algo: {lb.algorithm}</span>
                <span>{lb.listeners?.length || 0} Listeners / {lb.pools?.length || 0} Pools</span>
              </div>
            </div>
          ))}

          {(loadBalancers || []).length === 0 && !loading && (
            <div className="p-8 bg-slate-900 border border-slate-800 rounded-xl text-center text-xs text-slate-400">
              No load balancers configured. Click &quot;Create Load Balancer&quot; to begin.
            </div>
          )}
        </div>

        {/* Right: Selected LB Details */}
        <div className="lg:col-span-2 space-y-4">
          {selectedLB ? (
            <>
              {/* Overview Card */}
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
                <div className="flex items-center justify-between">
                  <div>
                    <h2 className="text-base font-bold text-white flex items-center gap-2">
                      {selectedLB.name}
                      <span className="text-xs px-2 py-0.5 rounded bg-slate-800 text-slate-300 font-mono">
                        {selectedLB.id.slice(0, 8)}
                      </span>
                    </h2>
                    <p className="text-xs text-slate-400 mt-0.5">
                      VIP Address: <span className="font-mono text-indigo-400">{selectedLB.vip_address || 'None'}</span> | Network: {selectedLB.network ? selectedLB.network.name : 'Public Edge'}
                    </p>
                  </div>
                  <button
                    onClick={() => handleDeleteLB(selectedLB.id)}
                    className="p-1.5 text-slate-500 hover:text-rose-400 transition-colors"
                    title="Delete Load Balancer"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                </div>

                {/* Live Stats */}
                <div className="grid grid-cols-3 gap-3 pt-2">
                  <div className="bg-slate-950/60 p-3 rounded-lg border border-slate-800 text-center">
                    <p className="text-[10px] text-slate-400 uppercase font-mono">Algorithm</p>
                    <p className="text-sm font-semibold text-white mt-1 capitalize">{selectedLB.algorithm.replace('_', ' ')}</p>
                  </div>
                  <div className="bg-slate-950/60 p-3 rounded-lg border border-slate-800 text-center">
                    <p className="text-[10px] text-slate-400 uppercase font-mono">Frontend Listeners</p>
                    <p className="text-sm font-semibold text-indigo-400 mt-1">{selectedLB.listeners?.length || 0}</p>
                  </div>
                  <div className="bg-slate-950/60 p-3 rounded-lg border border-slate-800 text-center">
                    <p className="text-[10px] text-slate-400 uppercase font-mono">Backend Pools</p>
                    <p className="text-sm font-semibold text-emerald-400 mt-1">{selectedLB.pools?.length || 0}</p>
                  </div>
                </div>
              </div>

              {/* Frontend Listeners Section */}
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-3">
                <div className="flex items-center justify-between">
                  <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider flex items-center gap-2">
                    <ArrowDownUp className="w-4 h-4 text-indigo-400" />
                    Frontend Listeners
                  </h3>
                  <button
                    onClick={() => setShowAddListener(true)}
                    className="inline-flex items-center gap-1 px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs rounded border border-slate-700 transition-colors"
                  >
                    <Plus className="w-3 h-3" />
                    Add Listener
                  </button>
                </div>

                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs font-mono">
                    <thead>
                      <tr className="border-b border-slate-800 text-slate-400 uppercase text-[10px]">
                        <th className="py-2 px-3">Name</th>
                        <th className="py-2 px-3">Protocol</th>
                        <th className="py-2 px-3">Port</th>
                        <th className="py-2 px-3">Default Pool</th>
                        <th className="py-2 px-3 text-right">Action</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60">
                      {selectedLB.listeners && selectedLB.listeners.length > 0 ? (
                        selectedLB.listeners.map((lis) => (
                          <tr key={lis.id} className="hover:bg-slate-800/30">
                            <td className="py-2.5 px-3 font-semibold text-white">{lis.name}</td>
                            <td className="py-2.5 px-3 uppercase text-indigo-400">{lis.protocol}</td>
                            <td className="py-2.5 px-3 text-slate-300">{lis.port}</td>
                            <td className="py-2.5 px-3 text-slate-400 font-sans">
                              {lis.pool ? lis.pool.name : lis.pool_id ? lis.pool_id.slice(0, 8) : 'Round Robin'}
                            </td>
                            <td className="py-2.5 px-3 text-right">
                              <button
                                onClick={() => handleDeleteListener(lis.id)}
                                className="text-slate-500 hover:text-rose-400"
                              >
                                <Trash2 className="w-3.5 h-3.5 inline" />
                              </button>
                            </td>
                          </tr>
                        ))
                      ) : (
                        <tr>
                          <td colSpan={5} className="py-4 text-center text-slate-500 italic font-sans">
                            No frontend listeners defined.
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Backend Pools & Members Section */}
              <div className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
                <div className="flex items-center justify-between">
                  <h3 className="text-xs font-semibold text-slate-300 uppercase tracking-wider flex items-center gap-2">
                    <Layers className="w-4 h-4 text-emerald-400" />
                    Backend Pools &amp; Members
                  </h3>
                  <button
                    onClick={() => setShowAddPool(true)}
                    className="inline-flex items-center gap-1 px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs rounded border border-slate-700 transition-colors"
                  >
                    <Plus className="w-3 h-3" />
                    Add Pool
                  </button>
                </div>

                {selectedLB.pools && selectedLB.pools.length > 0 ? (
                  <div className="space-y-4">
                    {selectedLB.pools.map((pool) => (
                      <div key={pool.id} className="bg-slate-950/60 border border-slate-800 rounded-lg p-4 space-y-3">
                        <div className="flex items-center justify-between">
                          <div>
                            <span className="text-sm font-semibold text-white">{pool.name}</span>
                            <span className="ml-2 px-2 py-0.5 rounded text-[10px] bg-slate-800 text-indigo-300 font-mono">
                              {pool.protocol.toUpperCase()} • {pool.algorithm}
                            </span>
                          </div>
                          <div className="flex items-center gap-2">
                            <button
                              onClick={() => {
                                setMemberPoolId(pool.id);
                                setShowAddMember(true);
                              }}
                              className="inline-flex items-center gap-1 px-2 py-0.5 bg-slate-800 hover:bg-slate-700 text-slate-300 text-[11px] rounded border border-slate-700"
                            >
                              <Plus className="w-3 h-3" />
                              Add Member
                            </button>
                            <button
                              onClick={() => handleDeletePool(pool.id)}
                              className="text-slate-500 hover:text-rose-400 p-1"
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        </div>

                        {/* Members table */}
                        <div className="overflow-x-auto">
                          <table className="w-full text-left text-xs font-mono">
                            <thead>
                              <tr className="border-b border-slate-800 text-slate-500 uppercase text-[10px]">
                                <th className="py-1.5 px-2">Backend IP</th>
                                <th className="py-1.5 px-2">Port</th>
                                <th className="py-1.5 px-2">Weight</th>
                                <th className="py-1.5 px-2">Health</th>
                                <th className="py-1.5 px-2 text-right">Action</th>
                              </tr>
                            </thead>
                            <tbody className="divide-y divide-slate-900">
                              {pool.members && pool.members.length > 0 ? (
                                pool.members.map((m) => (
                                  <tr key={m.id} className="hover:bg-slate-800/20">
                                    <td className="py-2 px-2 text-slate-300">
                                      {m.ip_address} {m.instance ? `(${m.instance.name})` : ''}
                                    </td>
                                    <td className="py-2 px-2 text-indigo-300">{m.port}</td>
                                    <td className="py-2 px-2 text-slate-400">{m.weight}</td>
                                    <td className="py-2 px-2">
                                      <span
                                        className={`px-1.5 py-0.5 rounded text-[10px] ${
                                          m.status === 'healthy'
                                            ? 'bg-emerald-950 text-emerald-300 border border-emerald-800'
                                            : 'bg-rose-950 text-rose-300 border border-rose-800'
                                        }`}
                                      >
                                        {m.status}
                                      </span>
                                    </td>
                                    <td className="py-2 px-2 text-right">
                                      <button
                                        onClick={() => handleDeleteMember(m.id)}
                                        className="text-slate-600 hover:text-rose-400"
                                      >
                                        <Trash2 className="w-3 h-3 inline" />
                                      </button>
                                    </td>
                                  </tr>
                                ))
                              ) : (
                                <tr>
                                  <td colSpan={5} className="py-3 text-center text-slate-600 italic font-sans text-xs">
                                    No backend members in this pool yet.
                                  </td>
                                </tr>
                              )}
                            </tbody>
                          </table>
                        </div>
                      </div>
                    ))}
                  </div>
                ) : (
                  <p className="text-xs text-slate-500 italic">No backend pools defined for this load balancer.</p>
                )}
              </div>
            </>
          ) : (
            <div className="p-16 text-center bg-slate-900 border border-slate-800 rounded-xl text-slate-500 text-xs">
              Select a load balancer from the list to inspect configuration, listeners, and backend members.
            </div>
          )}
        </div>
      </div>

      {/* MODAL: Create Load Balancer */}
      {showCreateLB && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Create Load Balancer</h3>
            <form onSubmit={handleCreateLB} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Load Balancer Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. production-api-lb"
                  value={lbName}
                  onChange={(e) => setLbName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">VPC Private Network</label>
                <select
                  value={lbNetworkId}
                  onChange={(e) => setLbNetworkId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                >
                  <option value="">Public Edge (Direct Ingress)</option>
                  {networks.map((net) => (
                    <option key={net.id} value={net.id}>
                      {net.name} ({net.cidr})
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Balancing Algorithm</label>
                <select
                  value={lbAlgorithm}
                  onChange={(e) => setLbAlgorithm(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                >
                  <option value="round_robin">Round Robin</option>
                  <option value="least_conn">Least Connections</option>
                  <option value="source_ip">Source IP Hash (Sticky)</option>
                  <option value="weighted_round_robin">Weighted Round Robin</option>
                </select>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateLB(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Create
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: Add Listener */}
      {showAddListener && selectedLB && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Add Frontend Listener</h3>
            <form onSubmit={handleAddListener} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Listener Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. http-inbound"
                  value={listenerName}
                  onChange={(e) => setListenerName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Protocol</label>
                  <select
                    value={listenerProto}
                    onChange={(e: any) => setListenerProto(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                  >
                    <option value="http">HTTP</option>
                    <option value="https">HTTPS</option>
                    <option value="tcp">TCP</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Port</label>
                  <input
                    type="number"
                    required
                    value={listenerPort}
                    onChange={(e) => setListenerPort(parseInt(e.target.value) || 80)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Default Target Pool (Optional)</label>
                <select
                  value={listenerPoolId}
                  onChange={(e) => setListenerPoolId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                >
                  <option value="">None (Select pool later)</option>
                  {selectedLB.pools?.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name} ({p.protocol})
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddListener(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Add Listener
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: Add Pool */}
      {showAddPool && selectedLB && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Add Backend Pool</h3>
            <form onSubmit={handleAddPool} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Pool Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. api-servers"
                  value={poolName}
                  onChange={(e) => setPoolName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Protocol</label>
                  <select
                    value={poolProto}
                    onChange={(e: any) => setPoolProto(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                  >
                    <option value="http">HTTP</option>
                    <option value="tcp">TCP</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Algorithm</label>
                  <select
                    value={poolAlgo}
                    onChange={(e: any) => setPoolAlgo(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                  >
                    <option value="round_robin">Round Robin</option>
                    <option value="least_conn">Least Connections</option>
                    <option value="source_ip">Source IP</option>
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Health Check</label>
                  <select
                    value={poolHCType}
                    onChange={(e: any) => setPoolHCType(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                  >
                    <option value="http">HTTP Healthcheck</option>
                    <option value="tcp">TCP Connect</option>
                  </select>
                </div>
                {poolHCType === 'http' && (
                  <div>
                    <label className="block text-xs font-medium text-slate-300 mb-1">Check Path</label>
                    <input
                      type="text"
                      value={poolHCPath}
                      onChange={(e) => setPoolHCPath(e.target.value)}
                      placeholder="/health"
                      className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                    />
                  </div>
                )}
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddPool(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Add Pool
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: Add Member */}
      {showAddMember && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Add Member to Pool</h3>
            <form onSubmit={handleAddMember} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Select VM or Enter IP</label>
                <select
                  value={memberInstanceId}
                  onChange={(e) => {
                    const instId = e.target.value;
                    setMemberInstanceId(instId);
                    const inst = instances.find((i) => i.id === instId);
                    if (inst) {
                      const ip = inst.private_ip || inst.public_ip;
                      if (ip) setMemberIP(ip);
                    }
                  }}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white mb-2"
                >
                  <option value="">Custom Private IP Address</option>
                  {instances.map((i) => (
                    <option key={i.id} value={i.id}>
                      {i.name} ({i.private_ip || i.public_ip || 'No IP'})
                    </option>
                  ))}
                </select>
                <input
                  type="text"
                  required
                  placeholder="Backend IP address (e.g. 10.100.0.5)"
                  value={memberIP}
                  onChange={(e) => setMemberIP(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white font-mono"
                />
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Port</label>
                  <input
                    type="number"
                    required
                    value={memberPort}
                    onChange={(e) => setMemberPort(parseInt(e.target.value) || 80)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Weight</label>
                  <input
                    type="number"
                    min="1"
                    max="256"
                    value={memberWeight}
                    onChange={(e) => setMemberWeight(parseInt(e.target.value) || 1)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                  />
                </div>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddMember(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Add Member
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
