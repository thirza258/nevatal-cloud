import React, { useState, useEffect } from 'react';
import { Instance, InstanceStatus } from '../types';
import { apiRequest } from '../lib/api';
import { StatusBadge } from '../components/ui/Badge';
import { Modal } from '../components/ui/Modal';
import {
  Server,
  Search,
  PlusCircle,
  Play,
  Square,
  RotateCcw,
  Trash2,
  ExternalLink,
  RefreshCw,
  AlertTriangle,
} from 'lucide-react';

interface InstancesListProps {
  onNavigate: (path: string, param?: string) => void;
}

export const InstancesList: React.FC<InstancesListProps> = ({ onNavigate }) => {
  const [instances, setInstances] = useState<Instance[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchTerm, setSearchTerm] = useState('');
  const [statusFilter, setStatusFilter] = useState<string>('all');
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  // Deletion Modal state
  const [deleteTarget, setDeleteTarget] = useState<Instance | null>(null);
  const [confirmName, setConfirmName] = useState('');
  const [deleting, setDeleting] = useState(false);

  const fetchInstances = async () => {
    try {
      const data = await apiRequest<Instance[]>('/instances');
      setInstances(data || []);
    } catch (err) {
      console.error('Failed to fetch instances:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchInstances();
    const interval = setInterval(fetchInstances, 6000);
    return () => clearInterval(interval);
  }, []);

  const handlePowerAction = async (e: React.MouseEvent, id: string, action: string) => {
    e.stopPropagation();
    setActionLoading(id);
    try {
      await apiRequest(`/instances/${id}/${action}`, { method: 'POST' });
      await fetchInstances();
    } catch (err: any) {
      alert(err.message || `Failed to ${action} instance`);
    } finally {
      setActionLoading(null);
    }
  };

  const handleDelete = async () => {
    if (!deleteTarget) return;
    setDeleting(true);
    try {
      await apiRequest(`/instances/${deleteTarget.id}`, { method: 'DELETE' });
      setDeleteTarget(null);
      setConfirmName('');
      await fetchInstances();
    } catch (err: any) {
      alert(err.message || 'Failed to delete instance');
    } finally {
      setDeleting(false);
    }
  };

  const filteredInstances = instances.filter((inst) => {
    const matchesSearch =
      inst.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
      (inst.private_ip && inst.private_ip.includes(searchTerm));
    if (!matchesSearch) return false;

    if (statusFilter === 'all') return true;
    if (statusFilter === 'running') return inst.status === 'running';
    if (statusFilter === 'stopped') return inst.status === 'stopped';
    if (statusFilter === 'provisioning')
      return (
        inst.status === 'provisioning' ||
        inst.status === 'queued' ||
        inst.status === 'starting' ||
        inst.status === 'restarting'
      );
    return true;
  });

  return (
    <div className="space-y-6">
      {/* Header bar */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-zinc-800 pb-5">
        <div>
          <h1 className="text-xl sm:text-2xl font-semibold text-white tracking-tight">Instances</h1>
          <p className="text-xs text-zinc-400 mt-0.5 font-mono">
            Proxmox VE virtual private servers · Dell PowerEdge R640
          </p>
        </div>
        <button
          onClick={() => onNavigate('/dashboard/instances/new')}
          className="flex items-center justify-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 font-medium text-xs transition"
        >
          <PlusCircle className="w-3.5 h-3.5" />
          <span>Create Instance</span>
        </button>
      </div>

      {/* Filter and Search Bar */}
      <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg p-2.5 flex flex-col md:flex-row items-center justify-between gap-3">
        {/* Status filter tabs */}
        <div className="flex items-center gap-1 w-full md:w-auto overflow-x-auto pb-1 md:pb-0">
          {['all', 'running', 'stopped', 'provisioning'].map((tab) => (
            <button
              key={tab}
              onClick={() => setStatusFilter(tab)}
              className={`px-2.5 py-1 rounded-md text-xs font-medium capitalize whitespace-nowrap transition ${
                statusFilter === tab
                  ? 'bg-zinc-800 text-white font-semibold'
                  : 'text-zinc-400 hover:text-white hover:bg-zinc-850'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>

        {/* Search Input */}
        <div className="relative w-full md:w-64">
          <Search className="w-3.5 h-3.5 absolute left-2.5 top-1/2 -translate-y-1/2 text-zinc-500" />
          <input
            type="text"
            placeholder="Search name or IP..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full bg-zinc-950 border border-zinc-800 rounded-md pl-8 pr-2.5 py-1 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-750"
          />
        </div>
      </div>

      {/* Instances Table */}
      <div className="bg-zinc-900/40 border border-zinc-800 rounded-lg overflow-hidden">
        {loading ? (
          <div className="flex items-center justify-center py-16 text-zinc-500 font-mono text-xs">
            <RefreshCw className="w-4 h-4 animate-spin mr-2" />
            <span>Loading instances...</span>
          </div>
        ) : filteredInstances.length === 0 ? (
          <div className="p-10 text-center text-zinc-500 space-y-2.5">
            <Server className="w-8 h-8 text-zinc-600 mx-auto" />
            <h3 className="text-sm font-medium text-white">No instances found</h3>
            <p className="text-xs text-zinc-400 max-w-sm mx-auto">
              {searchTerm || statusFilter !== 'all'
                ? 'No instances match your search or filter criteria.'
                : 'No virtual machines deployed yet. Launch a Linux VM in seconds.'}
            </p>
            <button
              onClick={() => onNavigate('/dashboard/instances/new')}
              className="mt-2 inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white text-xs font-medium text-zinc-950 hover:bg-zinc-200"
            >
              <PlusCircle className="w-3.5 h-3.5" />
              <span>Create Instance</span>
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-zinc-800 bg-zinc-950 text-zinc-400 font-mono text-[10px] uppercase tracking-wider">
                  <th className="py-2.5 px-3.5">Instance</th>
                  <th className="py-2.5 px-3.5">Status</th>
                  <th className="py-2.5 px-3.5">Hardware Specs</th>
                  <th className="py-2.5 px-3.5">IP Address</th>
                  <th className="py-2.5 px-3.5">Node &amp; VMID</th>
                  <th className="py-2.5 px-3.5 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-850">
                {filteredInstances.map((inst) => (
                  <tr
                    key={inst.id}
                    onClick={() => onNavigate('/dashboard/instances', inst.id)}
                    className="hover:bg-zinc-900/60 cursor-pointer transition-colors"
                  >
                    <td className="py-3 px-3.5">
                      <div className="font-medium text-white">{inst.name}</div>
                      <div className="text-[10px] text-zinc-400 font-mono mt-0.5">
                        {inst.os_template?.distribution} {inst.os_template?.version}
                      </div>
                    </td>
                    <td className="py-3 px-3.5">
                      <StatusBadge status={inst.status} />
                    </td>
                    <td className="py-3 px-3.5 font-mono text-zinc-300 text-[11px]">
                      <div>
                        {inst.vcpus} vCPU • {Math.round(inst.memory_mb / 1024)} GB RAM
                      </div>
                      <div className="text-[10px] text-zinc-400">{inst.disk_gb} GB SSD</div>
                    </td>
                    <td className="py-3 px-3.5 font-mono text-zinc-300 text-[11px]">
                      {inst.private_ip ? (
                        <span className="px-1.5 py-0.5 rounded bg-zinc-900 border border-zinc-800 font-mono text-[11px] text-zinc-200">
                          {inst.private_ip}
                        </span>
                      ) : (
                        <span className="text-zinc-600 italic">Pending DHCP...</span>
                      )}
                    </td>
                    <td className="py-3 px-3.5 font-mono text-zinc-400 text-[11px]">
                      <div>{inst.node?.proxmox_node_name || 'pve-dell-r640'}</div>
                      <div className="text-zinc-400">VMID: #{inst.proxmox_vmid || '—'}</div>
                    </td>
                    <td className="py-3 px-3.5 text-right">
                      <div
                        className="inline-flex items-center gap-1"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {inst.status === 'stopped' && (
                          <button
                            onClick={(e) => handlePowerAction(e, inst.id, 'start')}
                            disabled={actionLoading === inst.id}
                            className="p-1.5 rounded bg-zinc-850 hover:bg-zinc-800 text-emerald-400 border border-zinc-750 transition disabled:opacity-50"
                            title="Start"
                          >
                            <Play className="w-3.5 h-3.5" />
                          </button>
                        )}

                        {inst.status === 'running' && (
                          <>
                            <button
                              onClick={(e) => handlePowerAction(e, inst.id, 'shutdown')}
                              disabled={actionLoading === inst.id}
                              className="p-1.5 rounded bg-zinc-850 text-zinc-300 hover:text-white hover:bg-zinc-800 border border-zinc-750 transition disabled:opacity-50"
                              title="Shutdown"
                            >
                              <Square className="w-3.5 h-3.5" />
                            </button>
                            <button
                              onClick={(e) => handlePowerAction(e, inst.id, 'restart')}
                              disabled={actionLoading === inst.id}
                              className="p-1.5 rounded bg-zinc-850 text-zinc-300 hover:text-white hover:bg-zinc-800 border border-zinc-750 transition disabled:opacity-50"
                              title="Restart"
                            >
                              <RotateCcw className="w-3.5 h-3.5" />
                            </button>
                          </>
                        )}

                        <button
                          onClick={() => {
                            setDeleteTarget(inst);
                            setConfirmName('');
                          }}
                          className="p-1.5 rounded bg-zinc-850 text-zinc-400 hover:text-rose-400 hover:bg-rose-950/30 border border-zinc-750 hover:border-rose-900/60 transition"
                          title="Delete Instance"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Confirmation Modal for Permanent VM Deletion */}
      <Modal
        isOpen={deleteTarget !== null}
        onClose={() => setDeleteTarget(null)}
        title="Destroy Virtual Machine"
        description="This action is destructive and irreversible. The virtual machine and all associated virtual disk storage on Proxmox will be permanently deleted."
      >
        <div className="space-y-4">
          <div className="p-3 rounded-md bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs flex items-start gap-2.5">
            <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5 text-rose-400" />
            <div>
              <p className="font-semibold">Confirm Deletion</p>
              <p className="mt-0.5 text-rose-300/80">
                Type the instance name <strong className="text-white font-mono">{deleteTarget?.name}</strong>{' '}
                to confirm.
              </p>
            </div>
          </div>

          <div>
            <label className="block text-xs font-mono text-zinc-400 mb-1.5">Instance Name</label>
            <input
              type="text"
              placeholder={deleteTarget?.name}
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-3 py-1.5 text-xs text-white placeholder-zinc-600 focus:outline-none focus:border-rose-500 font-mono"
            />
          </div>

          <div className="flex items-center justify-end gap-2 pt-3 border-t border-zinc-850">
            <button
              onClick={() => setDeleteTarget(null)}
              className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-800 text-zinc-300 text-xs font-medium border border-zinc-800"
            >
              Cancel
            </button>
            <button
              onClick={handleDelete}
              disabled={confirmName !== deleteTarget?.name || deleting}
              className="px-3 py-1.5 rounded-md bg-rose-600 hover:bg-rose-500 text-white text-xs font-medium disabled:opacity-50 transition"
            >
              {deleting ? 'Destroying VM...' : 'Confirm & Destroy'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
