import React, { useState, useEffect } from 'react';
import { Instance } from '../../types';
import { apiRequest } from '../../lib/api';
import { StatusBadge } from '../../components/ui/Badge';
import { Server, Search, RefreshCw } from 'lucide-react';

export const AdminInstances: React.FC = () => {
  const [instances, setInstances] = useState<Instance[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  const fetchAllInstances = async () => {
    try {
      const data = await apiRequest<Instance[]>('/admin/instances');
      setInstances(data || []);
    } catch (err) {
      console.error('Failed to load all instances:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAllInstances();
    const interval = setInterval(fetchAllInstances, 8000);
    return () => clearInterval(interval);
  }, []);

  const filtered = instances.filter(
    (i) =>
      i.name.toLowerCase().includes(search.toLowerCase()) ||
      (i.owner?.email && i.owner.email.toLowerCase().includes(search.toLowerCase())) ||
      (i.private_ip && i.private_ip.includes(search))
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">All Tenant Instances</h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-0.5">
            Global inventory of virtual machines provisioned across the Dell PowerEdge R640
          </p>
        </div>
        <div className="relative w-full sm:w-64">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
          <input
            type="text"
            placeholder="Search VM, owner, or IP..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-slate-900 border border-slate-800 rounded-xl pl-9 pr-3 py-2 text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>

      <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
        {loading ? (
          <div className="p-12 text-center text-slate-400 text-xs">Loading all instances...</div>
        ) : filtered.length === 0 ? (
          <div className="p-12 text-center text-slate-400 text-xs">No instances found.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 font-medium uppercase tracking-wider">
                  <th className="py-3.5 px-4">Instance</th>
                  <th className="py-3.5 px-4">Owner</th>
                  <th className="py-3.5 px-4">Status</th>
                  <th className="py-3.5 px-4">Allocation</th>
                  <th className="py-3.5 px-4">Private IP</th>
                  <th className="py-3.5 px-4">Proxmox Target</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 font-mono">
                {filtered.map((inst) => (
                  <tr key={inst.id} className="hover:bg-slate-800/30 transition">
                    <td className="py-3.5 px-4 font-sans">
                      <div className="font-semibold text-white">{inst.name}</div>
                      <div className="text-[11px] text-slate-500 font-mono">
                        {inst.os_template?.name || 'Linux'}
                      </div>
                    </td>
                    <td className="py-3.5 px-4 font-sans">
                      <div className="text-slate-200">{inst.owner?.display_name || 'User'}</div>
                      <div className="text-[11px] text-slate-400 font-mono">{inst.owner?.email}</div>
                    </td>
                    <td className="py-3.5 px-4 font-sans">
                      <StatusBadge status={inst.status} size="sm" />
                    </td>
                    <td className="py-3.5 px-4 text-slate-300">
                      {inst.vcpus} vCPU • {Math.round(inst.memory_mb / 1024)}GB • {inst.disk_gb}GB
                    </td>
                    <td className="py-3.5 px-4 text-emerald-400">
                      {inst.private_ip || '—'}
                    </td>
                    <td className="py-3.5 px-4 text-slate-400 text-[11px]">
                      {inst.node?.proxmox_node_name || 'pve-dell-r640'} (VMID: #{inst.proxmox_vmid || '—'})
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};
