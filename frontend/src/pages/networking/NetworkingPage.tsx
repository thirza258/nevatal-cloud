import React, { useState, useEffect } from 'react';
import {
  Network,
  Shield,
  Globe,
  Radio,
  Plus,
  Trash2,
  ExternalLink,
  Server,
  RefreshCw,
  CheckCircle2,
  AlertCircle,
  Link,
  Unlink,
  Layers,
  ArrowRightLeft,
} from 'lucide-react';
import {
  listNetworks,
  createNetwork,
  deleteNetwork,
  attachNetwork,
  detachNetwork,
  listFirewalls,
  createFirewall,
  deleteFirewall,
  createFirewallRule,
  deleteFirewallRule,
  listFloatingIPs,
  createFloatingIP,
  assignFloatingIP,
  releaseFloatingIP,
  listDNSZones,
  createDNSZone,
  deleteDNSZone,
  listDNSRecords,
  createDNSRecord,
  deleteDNSRecord,
} from '../../lib/networkingApi';
import { apiRequest } from '../../lib/api';
import type {
  PrivateNetwork,
  CloudFirewall,
  FloatingIP,
  DNSZone,
  DNSRecord,
  Instance,
} from '../../types';

interface NetworkingPageProps {
  onNavigate?: (path: string) => void;
}

type TabType = 'networks' | 'firewalls' | 'floating_ips' | 'dns';

export const NetworkingPage: React.FC<NetworkingPageProps> = ({ onNavigate }) => {
  const [activeTab, setActiveTab] = useState<TabType>('networks');
  const [loading, setLoading] = useState(true);
  const [instances, setInstances] = useState<Instance[]>([]);

  // Networks State
  const [networks, setNetworks] = useState<PrivateNetwork[]>([]);
  const [showCreateNetwork, setShowCreateNetwork] = useState(false);
  const [newNetName, setNewNetName] = useState('');
  const [newNetCIDR, setNewNetCIDR] = useState('10.100.0.0/24');
  const [newNetDesc, setNewNetDesc] = useState('');
  const [selectedNetwork, setSelectedNetwork] = useState<PrivateNetwork | null>(null);
  const [showAttachVM, setShowAttachVM] = useState(false);
  const [attachVMId, setAttachVMId] = useState('');
  const [attachVMIp, setAttachVMIp] = useState('');

  // Firewalls State
  const [firewalls, setFirewalls] = useState<CloudFirewall[]>([]);
  const [showCreateFirewall, setShowCreateFirewall] = useState(false);
  const [newFwName, setNewFwName] = useState('');
  const [newFwDesc, setNewFwDesc] = useState('');
  const [selectedFirewall, setSelectedFirewall] = useState<CloudFirewall | null>(null);
  const [showAddRule, setShowAddRule] = useState(false);
  const [ruleDir, setRuleDir] = useState<'inbound' | 'outbound'>('inbound');
  const [ruleAction, setRuleAction] = useState<'allow' | 'deny'>('allow');
  const [ruleProto, setRuleProto] = useState<'tcp' | 'udp' | 'icmp' | 'all'>('tcp');
  const [rulePort, setRulePort] = useState('80,443');
  const [ruleCIDR, setRuleCIDR] = useState('0.0.0.0/0');
  const [ruleDesc, setRuleDesc] = useState('');

  // Floating IPs State
  const [floatingIPs, setFloatingIPs] = useState<FloatingIP[]>([]);
  const [showCreateFIP, setShowCreateFIP] = useState(false);
  const [fipDesc, setFipDesc] = useState('');
  const [assignFIPModal, setAssignFIPModal] = useState<FloatingIP | null>(null);
  const [assignVMId, setAssignVMId] = useState('');

  // DNS State
  const [dnsZones, setDnsZones] = useState<DNSZone[]>([]);
  const [showCreateZone, setShowCreateZone] = useState(false);
  const [newZoneName, setNewZoneName] = useState('');
  const [newZoneDesc, setNewZoneDesc] = useState('');
  const [selectedZone, setSelectedZone] = useState<DNSZone | null>(null);
  const [zoneRecords, setZoneRecords] = useState<DNSRecord[]>([]);
  const [showAddRecord, setShowAddRecord] = useState(false);
  const [recName, setRecName] = useState('@');
  const [recType, setRecType] = useState('A');
  const [recContent, setRecContent] = useState('');
  const [recTTL, setRecTTL] = useState(300);

  const [notification, setNotification] = useState<{ type: 'success' | 'error'; message: string } | null>(null);

  const notify = (type: 'success' | 'error', message: string) => {
    setNotification({ type, message });
    setTimeout(() => setNotification(null), 4000);
  };

  const loadAll = async () => {
    setLoading(true);
    try {
      const [nets, fws, fips, zones, insts] = await Promise.all([
        listNetworks().catch(() => []),
        listFirewalls().catch(() => []),
        listFloatingIPs().catch(() => []),
        listDNSZones().catch(() => []),
        apiRequest<Instance[]>('/instances').catch(() => []),
      ]);
      setNetworks(nets);
      setFirewalls(fws);
      setFloatingIPs(fips);
      setDnsZones(zones);
      setInstances(insts);
    } catch (err: any) {
      notify('error', err.message || 'Failed to load networking data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadAll();
  }, []);

  // --- Network Actions ---
  const handleCreateNetwork = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await createNetwork({ name: newNetName, cidr: newNetCIDR, description: newNetDesc });
      setShowCreateNetwork(false);
      setNewNetName('');
      setNewNetDesc('');
      notify('success', 'Private network created successfully');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error creating network');
    }
  };

  const handleDeleteNetwork = async (id: string) => {
    if (!confirm('Are you sure you want to delete this network?')) return;
    try {
      await deleteNetwork(id);
      notify('success', 'Private network deleted');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting network');
    }
  };

  const handleAttachVM = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedNetwork || !attachVMId) return;
    try {
      await attachNetwork(selectedNetwork.id, {
        instance_id: attachVMId,
        ip_address: attachVMIp || undefined,
      });
      setShowAttachVM(false);
      setAttachVMId('');
      setAttachVMIp('');
      notify('success', 'Virtual machine attached to network');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error attaching VM');
    }
  };

  const handleDetachVM = async (attId: string) => {
    if (!confirm('Detach this virtual machine from the network?')) return;
    try {
      await detachNetwork(attId);
      notify('success', 'Attachment detached');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error detaching');
    }
  };

  // --- Firewall Actions ---
  const handleCreateFirewall = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await createFirewall({ name: newFwName, description: newFwDesc });
      setShowCreateFirewall(false);
      setNewFwName('');
      setNewFwDesc('');
      notify('success', 'Firewall created');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error creating firewall');
    }
  };

  const handleDeleteFirewall = async (id: string) => {
    if (!confirm('Delete this firewall?')) return;
    try {
      await deleteFirewall(id);
      notify('success', 'Firewall deleted');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting firewall');
    }
  };

  const handleAddRule = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedFirewall) return;
    try {
      await createFirewallRule(selectedFirewall.id, {
        direction: ruleDir,
        action: ruleAction,
        protocol: ruleProto,
        port_range: rulePort,
        cidr: ruleCIDR,
        description: ruleDesc,
      });
      setShowAddRule(false);
      setRuleDesc('');
      notify('success', 'Firewall rule applied');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error adding rule');
    }
  };

  const handleDeleteRule = async (ruleId: string) => {
    try {
      await deleteFirewallRule(ruleId);
      notify('success', 'Rule deleted');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting rule');
    }
  };

  // --- Floating IP Actions ---
  const handleCreateFloatingIP = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await createFloatingIP({ description: fipDesc });
      setShowCreateFIP(false);
      setFipDesc('');
      notify('success', 'Floating IP allocated');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error allocating Floating IP');
    }
  };

  const handleAssignFloatingIP = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!assignFIPModal || !assignVMId) return;
    try {
      await assignFloatingIP(assignFIPModal.id, assignVMId);
      setAssignFIPModal(null);
      setAssignVMId('');
      notify('success', 'Floating IP assigned to instance');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error assigning IP');
    }
  };

  const handleReleaseFloatingIP = async (id: string) => {
    if (!confirm('Release this Floating IP back to the pool?')) return;
    try {
      await releaseFloatingIP(id);
      notify('success', 'Floating IP released');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error releasing IP');
    }
  };

  // --- DNS Actions ---
  const handleCreateZone = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      await createDNSZone({ name: newZoneName, description: newZoneDesc });
      setShowCreateZone(false);
      setNewZoneName('');
      setNewZoneDesc('');
      notify('success', 'DNS zone created');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error creating zone');
    }
  };

  const handleDeleteZone = async (id: string) => {
    if (!confirm('Delete this DNS zone and all its records?')) return;
    try {
      await deleteDNSZone(id);
      if (selectedZone?.id === id) setSelectedZone(null);
      notify('success', 'DNS zone deleted');
      loadAll();
    } catch (err: any) {
      notify('error', err.message || 'Error deleting zone');
    }
  };

  const openZoneRecords = async (zone: DNSZone) => {
    setSelectedZone(zone);
    try {
      const recs = await listDNSRecords(zone.id);
      setZoneRecords(recs);
    } catch (err: any) {
      notify('error', err.message || 'Error loading records');
    }
  };

  const handleAddRecord = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!selectedZone) return;
    try {
      await createDNSRecord(selectedZone.id, {
        name: recName,
        type: recType,
        content: recContent,
        ttl: recTTL,
      });
      setShowAddRecord(false);
      setRecName('@');
      setRecContent('');
      notify('success', 'DNS record added');
      openZoneRecords(selectedZone);
    } catch (err: any) {
      notify('error', err.message || 'Error adding record');
    }
  };

  const handleDeleteRecord = async (recordId: string) => {
    if (!confirm('Delete this DNS record?')) return;
    try {
      await deleteDNSRecord(recordId);
      notify('success', 'Record deleted');
      if (selectedZone) openZoneRecords(selectedZone);
    } catch (err: any) {
      notify('error', err.message || 'Error deleting record');
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight flex items-center gap-2">
            <Network className="w-6 h-6 text-indigo-400" />
            Cloud Networking
          </h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-1">
            Manage private VPC networks, cloud firewalls, floating IPs, and managed DNS zones.
          </p>
        </div>
        <button
          onClick={loadAll}
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
          onClick={() => setActiveTab('networks')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'networks'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Layers className="w-4 h-4" />
          Private Networks ({(networks || []).length})
        </button>
        <button
          onClick={() => setActiveTab('firewalls')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'firewalls'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Shield className="w-4 h-4" />
          Cloud Firewalls ({(firewalls || []).length})
        </button>
        <button
          onClick={() => setActiveTab('floating_ips')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'floating_ips'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Radio className="w-4 h-4" />
          Floating IPs ({(floatingIPs || []).length})
        </button>
        <button
          onClick={() => setActiveTab('dns')}
          className={`pb-3 px-4 text-xs font-medium flex items-center gap-2 border-b-2 transition-colors ${
            activeTab === 'dns'
              ? 'border-indigo-500 text-indigo-400'
              : 'border-transparent text-slate-400 hover:text-slate-200'
          }`}
        >
          <Globe className="w-4 h-4" />
          DNS Zones ({(dnsZones || []).length})
        </button>
      </div>

      {/* 1. NETWORKS TAB */}
      {activeTab === 'networks' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <p className="text-xs text-slate-400">
              Isolated Layer-2/3 private subnets for high-speed, secure inter-VM communication.
            </p>
            <button
              onClick={() => setShowCreateNetwork(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg transition-colors shadow-sm"
            >
              <Plus className="w-3.5 h-3.5" />
              Create Private Network
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {networks.map((net) => (
              <div
                key={net.id}
                className="bg-slate-900 border border-slate-800 rounded-xl p-5 hover:border-slate-700 transition-all flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-start justify-between">
                    <div>
                      <h3 className="text-sm font-semibold text-white">{net.name}</h3>
                      <p className="text-xs text-slate-400 mt-0.5">{net.description || 'No description'}</p>
                    </div>
                    <span className="px-2 py-0.5 text-[10px] font-mono rounded bg-indigo-950 text-indigo-300 border border-indigo-800/60">
                      {net.cidr}
                    </span>
                  </div>

                  <div className="mt-4 pt-3 border-t border-slate-800/80">
                    <div className="flex items-center justify-between text-xs text-slate-400 mb-2">
                      <span>Attached Virtual Machines ({net.attachments?.length || 0})</span>
                      <button
                        onClick={() => {
                          setSelectedNetwork(net);
                          setShowAttachVM(true);
                        }}
                        className="text-[11px] text-indigo-400 hover:text-indigo-300 flex items-center gap-1"
                      >
                        <Link className="w-3 h-3" />
                        Attach VM
                      </button>
                    </div>

                    {net.attachments && net.attachments.length > 0 ? (
                      <div className="space-y-1.5">
                        {net.attachments.map((att) => (
                          <div
                            key={att.id}
                            className="flex items-center justify-between bg-slate-950/60 px-2.5 py-1.5 rounded border border-slate-800 text-xs"
                          >
                            <div className="flex items-center gap-2">
                              <Server className="w-3.5 h-3.5 text-slate-400" />
                              <span className="text-slate-200 font-medium">
                                {att.instance?.name || att.instance_id.slice(0, 8)}
                              </span>
                              <span className="text-indigo-400 font-mono text-[11px]">({att.ip_address})</span>
                            </div>
                            <button
                              onClick={() => handleDetachVM(att.id)}
                              className="text-slate-500 hover:text-rose-400 transition-colors"
                              title="Detach"
                            >
                              <Unlink className="w-3 h-3" />
                            </button>
                          </div>
                        ))}
                      </div>
                    ) : (
                      <p className="text-xs text-slate-600 italic">No instances currently attached.</p>
                    )}
                  </div>
                </div>

                <div className="mt-5 pt-3 border-t border-slate-800 flex justify-between items-center text-xs text-slate-500">
                  <span>Gateway: {net.gateway || 'None'}</span>
                  <button
                    onClick={() => handleDeleteNetwork(net.id)}
                    className="text-rose-400 hover:text-rose-300 transition-colors"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            ))}

            {(networks || []).length === 0 && !loading && (
              <div className="col-span-2 p-8 text-center bg-slate-900 border border-slate-800 rounded-xl">
                <p className="text-xs text-slate-400">No private networks configured. Create your first VPC network.</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* 2. FIREWALLS TAB */}
      {activeTab === 'firewalls' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <p className="text-xs text-slate-400">
              Stateful Cloud Firewalls applied to VM network interfaces to control inbound and outbound traffic.
            </p>
            <button
              onClick={() => setShowCreateFirewall(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg transition-colors shadow-sm"
            >
              <Plus className="w-3.5 h-3.5" />
              Create Cloud Firewall
            </button>
          </div>

          <div className="grid grid-cols-1 gap-4">
            {firewalls.map((fw) => (
              <div key={fw.id} className="bg-slate-900 border border-slate-800 rounded-xl p-5 space-y-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <Shield className="w-5 h-5 text-indigo-400" />
                    <div>
                      <h3 className="text-sm font-semibold text-white">{fw.name}</h3>
                      <p className="text-xs text-slate-400">{fw.description || 'Cloud security policy'}</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => {
                        setSelectedFirewall(fw);
                        setShowAddRule(true);
                      }}
                      className="inline-flex items-center gap-1 px-2.5 py-1 bg-slate-800 hover:bg-slate-700 text-slate-200 text-xs rounded border border-slate-700"
                    >
                      <Plus className="w-3 h-3" />
                      Add Rule
                    </button>
                    <button
                      onClick={() => handleDeleteFirewall(fw.id)}
                      className="p-1 text-slate-500 hover:text-rose-400 transition-colors"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>

                {/* Rules Table */}
                <div className="overflow-x-auto">
                  <table className="w-full text-left text-xs">
                    <thead>
                      <tr className="border-b border-slate-800 text-slate-400 font-mono uppercase text-[10px]">
                        <th className="py-2 px-3">Direction</th>
                        <th className="py-2 px-3">Action</th>
                        <th className="py-2 px-3">Protocol</th>
                        <th className="py-2 px-3">Port Range</th>
                        <th className="py-2 px-3">CIDR Block</th>
                        <th className="py-2 px-3">Description</th>
                        <th className="py-2 px-3 text-right">Action</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800/60 font-mono">
                      {fw.rules && fw.rules.length > 0 ? (
                        fw.rules.map((rule) => (
                          <tr key={rule.id} className="hover:bg-slate-800/30">
                            <td className="py-2 px-3 uppercase text-[11px] font-semibold text-slate-300">
                              {rule.direction}
                            </td>
                            <td className="py-2 px-3">
                              <span
                                className={`px-2 py-0.5 rounded text-[10px] ${
                                  rule.action === 'allow'
                                    ? 'bg-emerald-950 text-emerald-300 border border-emerald-800'
                                    : 'bg-rose-950 text-rose-300 border border-rose-800'
                                }`}
                              >
                                {rule.action}
                              </span>
                            </td>
                            <td className="py-2 px-3 uppercase text-slate-300">{rule.protocol}</td>
                            <td className="py-2 px-3 text-indigo-300">{rule.port_range || 'ALL'}</td>
                            <td className="py-2 px-3 text-slate-300">{rule.cidr}</td>
                            <td className="py-2 px-3 text-slate-400 font-sans text-xs">{rule.description || '-'}</td>
                            <td className="py-2 px-3 text-right">
                              <button
                                onClick={() => handleDeleteRule(rule.id)}
                                className="text-slate-500 hover:text-rose-400 transition-colors"
                              >
                                <Trash2 className="w-3.5 h-3.5 inline" />
                              </button>
                            </td>
                          </tr>
                        ))
                      ) : (
                        <tr>
                          <td colSpan={7} className="py-4 text-center text-slate-500 italic">
                            No rules defined. Default policy applies.
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              </div>
            ))}

            {(firewalls || []).length === 0 && !loading && (
              <div className="p-8 text-center bg-slate-900 border border-slate-800 rounded-xl">
                <p className="text-xs text-slate-400">No cloud firewalls created yet.</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* 3. FLOATING IPS TAB */}
      {activeTab === 'floating_ips' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <p className="text-xs text-slate-400">
              Static public IPv4 addresses that can be instantly reassigned between virtual machines for zero-downtime failover.
            </p>
            <button
              onClick={() => setShowCreateFIP(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg transition-colors shadow-sm"
            >
              <Plus className="w-3.5 h-3.5" />
              Allocate Floating IP
            </button>
          </div>

          <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-left text-xs">
                <thead>
                  <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 uppercase text-[10px] font-mono">
                    <th className="py-3 px-4">Floating IP</th>
                    <th className="py-3 px-4">Status</th>
                    <th className="py-3 px-4">Assigned VM</th>
                    <th className="py-3 px-4">Description</th>
                    <th className="py-3 px-4 text-right">Actions</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-800 font-mono">
                  {floatingIPs.map((fip) => (
                    <tr key={fip.id} className="hover:bg-slate-800/30">
                      <td className="py-3 px-4 font-semibold text-white">{fip.ip_address}</td>
                      <td className="py-3 px-4">
                        <span
                          className={`px-2 py-0.5 rounded text-[10px] ${
                            fip.status === 'assigned'
                              ? 'bg-emerald-950 text-emerald-300 border border-emerald-800'
                              : 'bg-amber-950 text-amber-300 border border-amber-800'
                          }`}
                        >
                          {fip.status}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-slate-300">
                        {fip.instance ? fip.instance.name : fip.instance_id ? fip.instance_id.slice(0, 8) : 'Unassigned'}
                      </td>
                      <td className="py-3 px-4 text-slate-400 font-sans">{fip.description || '-'}</td>
                      <td className="py-3 px-4 text-right space-x-2">
                        <button
                          onClick={() => {
                            setAssignFIPModal(fip);
                            setAssignVMId(fip.instance_id || '');
                          }}
                          className="px-2 py-1 bg-slate-800 hover:bg-slate-700 text-indigo-300 text-xs rounded border border-slate-700"
                        >
                          Reassign
                        </button>
                        <button
                          onClick={() => handleReleaseFloatingIP(fip.id)}
                          className="p-1 text-slate-500 hover:text-rose-400 transition-colors"
                          title="Release IP"
                        >
                          <Trash2 className="w-3.5 h-3.5 inline" />
                        </button>
                      </td>
                    </tr>
                  ))}
                  {(floatingIPs || []).length === 0 && !loading && (
                    <tr>
                      <td colSpan={5} className="py-8 text-center text-slate-500 italic">
                        No floating IPs allocated.
                      </td>
                    </tr>
                  )}
                </tbody>
              </table>
            </div>
          </div>
        </div>
      )}

      {/* 4. DNS TAB */}
      {activeTab === 'dns' && (
        <div className="space-y-4">
          <div className="flex justify-between items-center">
            <p className="text-xs text-slate-400">
              Authoritative DNS zones and resource records managed through the Nevatal Cloud API and dashboard.
            </p>
            <button
              onClick={() => setShowCreateZone(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-medium rounded-lg transition-colors shadow-sm"
            >
              <Plus className="w-3.5 h-3.5" />
              Add DNS Zone
            </button>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            {/* Zones List */}
            <div className="md:col-span-1 space-y-2">
              <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider">DNS Zones</h2>
              {dnsZones.map((z) => (
                <div
                  key={z.id}
                  onClick={() => openZoneRecords(z)}
                  className={`p-3.5 rounded-xl border cursor-pointer transition-all flex items-center justify-between ${
                    selectedZone?.id === z.id
                      ? 'bg-indigo-950/40 border-indigo-500/80 text-white'
                      : 'bg-slate-900 border-slate-800 hover:border-slate-700 text-slate-300'
                  }`}
                >
                  <div>
                    <p className="text-sm font-semibold">{z.name}</p>
                    <p className="text-xs text-slate-400">{z.description || 'Authoritative Zone'}</p>
                  </div>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      handleDeleteZone(z.id);
                    }}
                    className="p-1 text-slate-500 hover:text-rose-400"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              ))}
              {(dnsZones || []).length === 0 && !loading && (
                <div className="p-4 bg-slate-900 border border-slate-800 rounded-xl text-center text-xs text-slate-500">
                  No DNS zones found.
                </div>
              )}
            </div>

            {/* Records Table */}
            <div className="md:col-span-2 space-y-2">
              <div className="flex justify-between items-center">
                <h2 className="text-xs font-semibold text-slate-400 uppercase tracking-wider">
                  {selectedZone ? `Records for ${selectedZone.name}` : 'Select a Zone'}
                </h2>
                {selectedZone && (
                  <button
                    onClick={() => setShowAddRecord(true)}
                    className="inline-flex items-center gap-1 px-2.5 py-1 bg-indigo-600 hover:bg-indigo-500 text-white text-xs rounded transition-colors"
                  >
                    <Plus className="w-3 h-3" />
                    Add Record
                  </button>
                )}
              </div>

              {selectedZone ? (
                <div className="bg-slate-900 border border-slate-800 rounded-xl overflow-hidden">
                  <table className="w-full text-left text-xs">
                    <thead>
                      <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 uppercase text-[10px] font-mono">
                        <th className="py-2.5 px-3">Name</th>
                        <th className="py-2.5 px-3">Type</th>
                        <th className="py-2.5 px-3">Content</th>
                        <th className="py-2.5 px-3">TTL</th>
                        <th className="py-2.5 px-3 text-right">Action</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-800 font-mono">
                      {zoneRecords.map((rec) => (
                        <tr key={rec.id} className="hover:bg-slate-800/30">
                          <td className="py-2 px-3 font-semibold text-white">{rec.name}</td>
                          <td className="py-2 px-3 text-indigo-400 font-bold">{rec.type}</td>
                          <td className="py-2 px-3 text-slate-300 break-all">{rec.content}</td>
                          <td className="py-2 px-3 text-slate-500">{rec.ttl}s</td>
                          <td className="py-2 px-3 text-right">
                            <button
                              onClick={() => handleDeleteRecord(rec.id)}
                              className="text-slate-500 hover:text-rose-400 transition-colors"
                            >
                              <Trash2 className="w-3.5 h-3.5 inline" />
                            </button>
                          </td>
                        </tr>
                      ))}
                      {(zoneRecords || []).length === 0 && (
                        <tr>
                          <td colSpan={5} className="py-6 text-center text-slate-500 italic">
                            No DNS records yet.
                          </td>
                        </tr>
                      )}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="p-12 text-center bg-slate-900 border border-slate-800 rounded-xl text-slate-500 text-xs">
                  Select a DNS zone on the left to view and configure DNS records.
                </div>
              )}
            </div>
          </div>
        </div>
      )}

      {/* MODAL: Create Network */}
      {showCreateNetwork && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Create Private Network</h3>
            <form onSubmit={handleCreateNetwork} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Network Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. backend-vpc"
                  value={newNetName}
                  onChange={(e) => setNewNetName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">IPv4 CIDR Block</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. 10.100.0.0/24"
                  value={newNetCIDR}
                  onChange={(e) => setNewNetCIDR(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-indigo-500"
                />
                <p className="text-[11px] text-slate-500 mt-1">RFC 1918 block: 10.0.0.0/8, 172.16.0.0/12, or 192.168.0.0/16</p>
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Description (Optional)</label>
                <input
                  type="text"
                  placeholder="Production microservices interconnect"
                  value={newNetDesc}
                  onChange={(e) => setNewNetDesc(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateNetwork(false)}
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

      {/* MODAL: Attach VM to Network */}
      {showAttachVM && selectedNetwork && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-2">Attach VM to {selectedNetwork.name}</h3>
            <p className="text-xs text-slate-400 mb-4">Select an instance to connect to {selectedNetwork.cidr}</p>
            <form onSubmit={handleAttachVM} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Virtual Machine</label>
                <select
                  required
                  value={attachVMId}
                  onChange={(e) => setAttachVMId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-indigo-500"
                >
                  <option value="">Select Virtual Machine...</option>
                  {instances.map((i) => (
                    <option key={i.id} value={i.id}>
                      {i.name} ({i.status})
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Specific Static IP (Optional)</label>
                <input
                  type="text"
                  placeholder="Auto-allocate next available"
                  value={attachVMIp}
                  onChange={(e) => setAttachVMIp(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white font-mono focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAttachVM(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Attach
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: Create Firewall */}
      {showCreateFirewall && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Create Cloud Firewall</h3>
            <form onSubmit={handleCreateFirewall} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Firewall Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. web-tier-security"
                  value={newFwName}
                  onChange={(e) => setNewFwName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Description</label>
                <input
                  type="text"
                  placeholder="Inbound rules for web server workloads"
                  value={newFwDesc}
                  onChange={(e) => setNewFwDesc(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateFirewall(false)}
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

      {/* MODAL: Add Firewall Rule */}
      {showAddRule && selectedFirewall && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Add Rule to {selectedFirewall.name}</h3>
            <form onSubmit={handleAddRule} className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Direction</label>
                  <select
                    value={ruleDir}
                    onChange={(e: any) => setRuleDir(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                  >
                    <option value="inbound">Inbound</option>
                    <option value="outbound">Outbound</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Action</label>
                  <select
                    value={ruleAction}
                    onChange={(e: any) => setRuleAction(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                  >
                    <option value="allow">Allow</option>
                    <option value="deny">Deny</option>
                  </select>
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Protocol</label>
                  <select
                    value={ruleProto}
                    onChange={(e: any) => setRuleProto(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                  >
                    <option value="tcp">TCP</option>
                    <option value="udp">UDP</option>
                    <option value="icmp">ICMP</option>
                    <option value="all">ALL</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Port Range</label>
                  <input
                    type="text"
                    placeholder="80, 443, or 8000-9000"
                    value={rulePort}
                    onChange={(e) => setRulePort(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Source / Destination CIDR</label>
                <input
                  type="text"
                  required
                  placeholder="0.0.0.0/0"
                  value={ruleCIDR}
                  onChange={(e) => setRuleCIDR(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Description</label>
                <input
                  type="text"
                  placeholder="Allow HTTPS traffic"
                  value={ruleDesc}
                  onChange={(e) => setRuleDesc(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddRule(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Add Rule
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: Allocate Floating IP */}
      {showCreateFIP && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Allocate Floating IP</h3>
            <form onSubmit={handleCreateFloatingIP} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Description (Optional)</label>
                <input
                  type="text"
                  placeholder="Production API ingress"
                  value={fipDesc}
                  onChange={(e) => setFipDesc(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateFIP(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Allocate
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: Reassign Floating IP */}
      {assignFIPModal && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-2">Assign Floating IP</h3>
            <p className="text-xs text-indigo-400 font-mono mb-4">{assignFIPModal.ip_address}</p>
            <form onSubmit={handleAssignFloatingIP} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Target Virtual Machine</label>
                <select
                  required
                  value={assignVMId}
                  onChange={(e) => setAssignVMId(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                >
                  <option value="">Select Virtual Machine...</option>
                  {instances.map((i) => (
                    <option key={i.id} value={i.id}>
                      {i.name} ({i.private_ip || i.public_ip || 'No IP'})
                    </option>
                  ))}
                </select>
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setAssignFIPModal(null)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Confirm Assignment
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: Create DNS Zone */}
      {showCreateZone && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Add DNS Zone</h3>
            <form onSubmit={handleCreateZone} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Zone Domain Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. example.com"
                  value={newZoneName}
                  onChange={(e) => setNewZoneName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Description (Optional)</label>
                <input
                  type="text"
                  placeholder="Primary corporate domain"
                  value={newZoneDesc}
                  onChange={(e) => setNewZoneDesc(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-3 py-2 text-xs text-white"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowCreateZone(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Create Zone
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: Add DNS Record */}
      {showAddRecord && selectedZone && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4 z-50">
          <div className="bg-slate-900 border border-slate-800 rounded-2xl w-full max-w-md p-6 shadow-2xl">
            <h3 className="text-base font-semibold text-white mb-3">Add Record to {selectedZone.name}</h3>
            <form onSubmit={handleAddRecord} className="space-y-3">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Record Type</label>
                  <select
                    value={recType}
                    onChange={(e) => setRecType(e.target.value)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white"
                  >
                    <option value="A">A (IPv4)</option>
                    <option value="AAAA">AAAA (IPv6)</option>
                    <option value="CNAME">CNAME (Alias)</option>
                    <option value="TXT">TXT (Text)</option>
                    <option value="MX">MX (Mail)</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">TTL (Seconds)</label>
                  <input
                    type="number"
                    value={recTTL}
                    onChange={(e) => setRecTTL(parseInt(e.target.value) || 300)}
                    className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                  />
                </div>
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Record Name</label>
                <input
                  type="text"
                  required
                  placeholder="@ or subdomain (e.g. api)"
                  value={recName}
                  onChange={(e) => setRecName(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                />
              </div>
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Value / Content</label>
                <input
                  type="text"
                  required
                  placeholder="192.0.2.1 or lb.example.com"
                  value={recContent}
                  onChange={(e) => setRecContent(e.target.value)}
                  className="w-full bg-slate-950 border border-slate-800 rounded-lg px-2.5 py-1.5 text-xs text-white font-mono"
                />
              </div>
              <div className="flex justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setShowAddRecord(false)}
                  className="px-3 py-1.5 bg-slate-800 text-slate-300 hover:bg-slate-700 text-xs rounded-lg"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-1.5 bg-indigo-600 text-white hover:bg-indigo-500 text-xs font-medium rounded-lg"
                >
                  Save Record
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
