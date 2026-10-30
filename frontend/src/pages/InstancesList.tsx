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
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">Instances</h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-0.5">
            Manage your virtual private servers deployed on Proxmox VE
          </p>
        </div>
        <button
          onClick={() => onNavigate('/dashboard/instances/new')}
          className="flex items-center justify-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm shadow-md transition"
        >
          <PlusCircle className="w-4 h-4" />
          <span>Create Instance</span>
        </button>
      </div>

      {/* Filter and Search Bar */}
      <div className="bg-slate-900 border border-slate-800 rounded-2xl p-4 flex flex-col md:flex-row items-center justify-between gap-4">
        {/* Status filter tabs */}
        <div className="flex items-center gap-1.5 w-full md:w-auto overflow-x-auto pb-2 md:pb-0">
          {['all', 'running', 'stopped', 'provisioning'].map((tab) => (
            <button
              key={tab}
              onClick={() => setStatusFilter(tab)}
              className={`px-3 py-1.5 rounded-xl text-xs font-medium capitalize whitespace-nowrap transition ${
                statusFilter === tab
                  ? 'bg-blue-600 text-white shadow-sm'
                  : 'text-slate-400 hover:text-white hover:bg-slate-800'
              }`}
            >
              {tab}
            </button>
          ))}
        </div>

        {/* Search Input */}
        <div className="relative w-full md:w-72">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
          <input
            type="text"
            placeholder="Search by name or IP..."
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
            className="w-full bg-slate-950 border border-slate-800 rounded-xl pl-9 pr-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>

      {/* Instances Table */}
      <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
        {loading ? (
          <div className="flex items-center justify-center py-20 text-slate-400">
            <RefreshCw className="w-6 h-6 animate-spin text-blue-500 mr-2" />
            <span>Loading instances...</span>
          </div>
        ) : filteredInstances.length === 0 ? (
          <div className="p-12 text-center text-slate-400 space-y-3">
            <Server className="w-12 h-12 text-slate-600 mx-auto" />
            <h3 className="text-base font-semibold text-white">No instances found</h3>
            <p className="text-xs text-slate-500 max-w-sm mx-auto">
              {searchTerm || statusFilter !== 'all'
                ? 'No instances match your search or filter criteria.'
                : 'You have not created any virtual machines yet. Deploy a Linux instance in a few clicks.'}
            </p>
            <button
              onClick={() => onNavigate('/dashboard/instances/new')}
              className="mt-2 inline-flex items-center gap-1.5 px-3 py-2 rounded-xl bg-blue-600 text-xs font-semibold text-white hover:bg-blue-500"
            >
              <PlusCircle className="w-3.5 h-3.5" />
              <span>Create New Instance</span>
            </button>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 font-medium uppercase tracking-wider">
                  <th className="py-3.5 px-4">Instance</th>
                  <th className="py-3.5 px-4">Status</th>
                  <th className="py-3.5 px-4">Hardware Specs</th>
                  <th className="py-3.5 px-4">IP Address</th>
                  <th className="py-3.5 px-4">Node & VMID</th>
                  <th className="py-3.5 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {filteredInstances.map((inst) => (
                  <tr
                    key={inst.id}
                    onClick={() => onNavigate('/dashboard/instances', inst.id)}
                    className="hover:bg-slate-800/40 cursor-pointer transition-colors"
                  >
                    <td className="py-4 px-4">
                      <div className="font-semibold text-white text-sm">{inst.name}</div>
                      <div className="text-[11px] text-slate-500 font-mono mt-0.5">
                        {inst.os_template?.distribution} {inst.os_template?.version}
                      </div>
                    </td>
                    <td className="py-4 px-4">
                      <StatusBadge status={inst.status} />
                    </td>
                    <td className="py-4 px-4 font-mono text-slate-300">
                      <div>
                        {inst.vcpus} vCPU • {Math.round(inst.memory_mb / 1024)} GB RAM
                      </div>
                      <div className="text-[11px] text-slate-500">{inst.disk_gb} GB SSD</div>
                    </td>
                    <td className="py-4 px-4 font-mono text-slate-300">
                      {inst.private_ip ? (
                        <span className="px-2 py-1 rounded bg-slate-800/80 border border-slate-700/60 text-emerald-400 font-mono text-[11px]">
                          {inst.private_ip}
                        </span>
                      ) : (
                        <span className="text-slate-500 italic">Pending DHCP...</span>
                      )}
                    </td>
                    <td className="py-4 px-4 font-mono text-slate-400 text-[11px]">
                      <div>{inst.node?.proxmox_node_name || 'pve-dell-r640'}</div>
                      <div className="text-slate-500">VMID: #{inst.proxmox_vmid || '—'}</div>
                    </td>
                    <td className="py-4 px-4 text-right">
                      <div
                        className="inline-flex items-center gap-1.5"
                        onClick={(e) => e.stopPropagation()}
                      >
                        {inst.status === 'stopped' && (
                          <button
                            onClick={(e) => handlePowerAction(e, inst.id, 'start')}
                            disabled={actionLoading === inst.id}
                            className="p-1.5 rounded-lg bg-emerald-950/60 text-emerald-400 hover:bg-emerald-900 border border-emerald-800/60 transition disabled:opacity-50"
                            title="Start"
                          >
                            <Play className="w-4 h-4" />
                          </button>
                        )}

                        {inst.status === 'running' && (
                          <>
                            <button
                              onClick={(e) => handlePowerAction(e, inst.id, 'shutdown')}
                              disabled={actionLoading === inst.id}
                              className="p-1.5 rounded-lg bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 border border-slate-700/60 transition disabled:opacity-50"
                              title="Shutdown"
                            >
                              <Square className="w-4 h-4" />
                            </button>
                            <button
                              onClick={(e) => handlePowerAction(e, inst.id, 'restart')}
                              disabled={actionLoading === inst.id}
                              className="p-1.5 rounded-lg bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 border border-slate-700/60 transition disabled:opacity-50"
                              title="Restart"
                            >
                              <RotateCcw className="w-4 h-4" />
                            </button>
                          </>
                        )}

                        <button
                          onClick={() => {
                            setDeleteTarget(inst);
                            setConfirmName('');
                          }}
                          className="p-1.5 rounded-lg bg-slate-800/80 text-slate-400 hover:text-rose-400 hover:bg-rose-950/40 border border-slate-700/60 hover:border-rose-800 transition"
                          title="Delete Instance"
                        >
                          <Trash2 className="w-4 h-4" />
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
          <div className="p-3 rounded-xl bg-rose-950/50 border border-rose-800 text-rose-300 text-xs flex items-start gap-2.5">
            <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5 text-rose-400" />
            <div>
              <p className="font-semibold">Confirm Deletion</p>
              <p className="mt-0.5 text-rose-300/80">
                Type the instance name <strong className="text-white">{deleteTarget?.name}</strong>{' '}
                to confirm.
              </p>
            </div>
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1.5">Instance Name</label>
            <input
              type="text"
              placeholder={deleteTarget?.name}
              value={confirmName}
              onChange={(e) => setConfirmName(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-sm text-white placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-rose-500 font-mono"
            />
          </div>

          <div className="flex items-center justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              onClick={() => setDeleteTarget(null)}
              className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold"
            >
              Cancel
            </button>
            <button
              onClick={handleDelete}
              disabled={confirmName !== deleteTarget?.name || deleting}
              className="px-4 py-2 rounded-xl bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold disabled:opacity-50 transition shadow-md shadow-rose-600/20"
            >
              {deleting ? 'Destroying VM...' : 'Confirm & Destroy'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
