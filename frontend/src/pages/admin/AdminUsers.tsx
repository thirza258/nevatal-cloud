import React, { useState, useEffect } from 'react';
import { User, QuotaUsage } from '../../types';
import { apiRequest } from '../../lib/api';
import { Modal } from '../../components/ui/Modal';
import { Users, Search, Edit3, ShieldAlert, ShieldCheck, Check } from 'lucide-react';

interface UserWithUsage extends User {
  usage?: QuotaUsage;
}

export const AdminUsers: React.FC = () => {
  const [users, setUsers] = useState<UserWithUsage[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  // Edit Quotas modal
  const [editingUser, setEditingUser] = useState<UserWithUsage | null>(null);
  const [quotaVCPUs, setQuotaVCPUs] = useState(4);
  const [quotaMemoryMB, setQuotaMemoryMB] = useState(8192);
  const [quotaStorageGB, setQuotaStorageGB] = useState(100);
  const [quotaInstances, setQuotaInstances] = useState(3);
  const [savingQuota, setSavingQuota] = useState(false);

  const fetchUsers = async () => {
    try {
      const data = await apiRequest<UserWithUsage[]>('/admin/users');
      setUsers(data || []);
    } catch (err) {
      console.error('Failed to load users:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchUsers();
  }, []);

  const handleToggleStatus = async (user: User) => {
    const newStatus = user.status === 'active' ? 'suspended' : 'active';
    const actionLabel = newStatus === 'suspended' ? 'suspend' : 'activate';

    if (!confirm(`Are you sure you want to ${actionLabel} account ${user.email}?`)) {
      return;
    }

    try {
      await apiRequest(`/admin/users/${user.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ status: newStatus }),
      });
      await fetchUsers();
    } catch (err: any) {
      alert(err.message || 'Failed to update user status');
    }
  };

  const openQuotaModal = (user: UserWithUsage) => {
    setEditingUser(user);
    if (user.usage) {
      setQuotaVCPUs(user.usage.max_vcpus);
      setQuotaMemoryMB(user.usage.max_memory_mb);
      setQuotaStorageGB(user.usage.max_storage_gb);
      setQuotaInstances(user.usage.max_instances);
    } else {
      setQuotaVCPUs(4);
      setQuotaMemoryMB(8192);
      setQuotaStorageGB(100);
      setQuotaInstances(3);
    }
  };

  const handleSaveQuotas = async () => {
    if (!editingUser) return;
    setSavingQuota(true);
    try {
      await apiRequest(`/admin/users/${editingUser.id}/quotas`, {
        method: 'PUT',
        body: JSON.stringify({
          max_vcpus: quotaVCPUs,
          max_memory_mb: quotaMemoryMB,
          max_storage_gb: quotaStorageGB,
          max_instances: quotaInstances,
        }),
      });
      setEditingUser(null);
      await fetchUsers();
    } catch (err: any) {
      alert(err.message || 'Failed to update quotas');
    } finally {
      setSavingQuota(false);
    }
  };

  const filtered = users.filter(
    (u) =>
      u.email.toLowerCase().includes(search.toLowerCase()) ||
      u.display_name.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">User Directory</h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-0.5">
            Customer account control, resource limit governance, and access suspension
          </p>
        </div>
        <div className="relative w-full sm:w-64">
          <Search className="w-4 h-4 absolute left-3 top-1/2 -translate-y-1/2 text-slate-500" />
          <input
            type="text"
            placeholder="Search users by name or email..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="w-full bg-slate-900 border border-slate-800 rounded-xl pl-9 pr-3 py-2 text-xs text-white placeholder-slate-500 focus:outline-none focus:ring-2 focus:ring-blue-500"
          />
        </div>
      </div>

      <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
        {loading ? (
          <div className="p-12 text-center text-slate-400 text-xs">Loading accounts...</div>
        ) : filtered.length === 0 ? (
          <div className="p-12 text-center text-slate-400 text-xs">No users found.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 font-medium uppercase tracking-wider">
                  <th className="py-3.5 px-4">User</th>
                  <th className="py-3.5 px-4">Role</th>
                  <th className="py-3.5 px-4">Account Status</th>
                  <th className="py-3.5 px-4">vCPU Quota</th>
                  <th className="py-3.5 px-4">RAM Quota</th>
                  <th className="py-3.5 px-4">Storage Quota</th>
                  <th className="py-3.5 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 font-mono">
                {filtered.map((u) => (
                  <tr key={u.id} className="hover:bg-slate-800/30 transition">
                    <td className="py-3.5 px-4 font-sans">
                      <div className="font-semibold text-white">{u.display_name}</div>
                      <div className="text-[11px] text-slate-400 font-mono">{u.email}</div>
                    </td>
                    <td className="py-3.5 px-4">
                      <span
                        className={`px-2 py-0.5 rounded text-[10px] font-bold uppercase ${
                          u.role === 'admin'
                            ? 'bg-amber-950/80 text-amber-300 border border-amber-800/80'
                            : 'bg-blue-950/80 text-blue-300 border border-blue-800/80'
                        }`}
                      >
                        {u.role}
                      </span>
                    </td>
                    <td className="py-3.5 px-4">
                      <span
                        className={`px-2 py-0.5 rounded text-[10px] font-bold capitalize ${
                          u.status === 'active'
                            ? 'bg-emerald-950/80 text-emerald-300 border border-emerald-800/80'
                            : 'bg-rose-950/80 text-rose-300 border border-rose-800/80'
                        }`}
                      >
                        {u.status}
                      </span>
                    </td>
                    <td className="py-3.5 px-4 text-slate-300">
                      {u.usage?.used_vcpus || 0} / {u.usage?.max_vcpus || 4} Cores
                    </td>
                    <td className="py-3.5 px-4 text-slate-300">
                      {Math.round((u.usage?.used_memory_mb || 0) / 1024)} /{' '}
                      {Math.round((u.usage?.max_memory_mb || 8192) / 1024)} GB
                    </td>
                    <td className="py-3.5 px-4 text-slate-300">
                      {u.usage?.used_storage_gb || 0} / {u.usage?.max_storage_gb || 100} GB
                    </td>
                    <td className="py-3.5 px-4 text-right">
                      <div className="inline-flex items-center gap-2">
                        <button
                          onClick={() => openQuotaModal(u)}
                          className="px-2.5 py-1 rounded-lg bg-slate-800 text-slate-300 hover:text-white hover:bg-slate-700 text-xs font-sans transition"
                        >
                          Edit Quotas
                        </button>
                        {u.role !== 'admin' && (
                          <button
                            onClick={() => handleToggleStatus(u)}
                            className={`px-2.5 py-1 rounded-lg text-xs font-sans transition ${
                              u.status === 'active'
                                ? 'bg-rose-950/50 text-rose-300 hover:bg-rose-900 border border-rose-800/60'
                                : 'bg-emerald-950/50 text-emerald-300 hover:bg-emerald-900 border border-emerald-800/60'
                            }`}
                          >
                            {u.status === 'active' ? 'Suspend' : 'Activate'}
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Edit Quotas Modal */}
      <Modal
        isOpen={editingUser !== null}
        onClose={() => setEditingUser(null)}
        title="Modify Account Quotas"
        description={`Configure resource limits for ${editingUser?.email}`}
      >
        <div className="space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">Max vCPUs</label>
              <input
                type="number"
                min="1"
                max="64"
                value={quotaVCPUs}
                onChange={(e) => setQuotaVCPUs(parseInt(e.target.value) || 1)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white font-mono"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">Max Memory (MB)</label>
              <input
                type="number"
                min="512"
                step="512"
                value={quotaMemoryMB}
                onChange={(e) => setQuotaMemoryMB(parseInt(e.target.value) || 512)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white font-mono"
              />
              <span className="text-[10px] text-slate-500 mt-1 block">
                ≈ {Math.round(quotaMemoryMB / 1024)} GB
              </span>
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">Max Storage (GB)</label>
              <input
                type="number"
                min="10"
                step="10"
                value={quotaStorageGB}
                onChange={(e) => setQuotaStorageGB(parseInt(e.target.value) || 10)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white font-mono"
              />
            </div>
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1">Max Instances</label>
              <input
                type="number"
                min="1"
                max="50"
                value={quotaInstances}
                onChange={(e) => setQuotaInstances(parseInt(e.target.value) || 1)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white font-mono"
              />
            </div>
          </div>

          <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              onClick={() => setEditingUser(null)}
              className="px-4 py-2 rounded-xl bg-slate-800 text-slate-300 text-xs font-semibold"
            >
              Cancel
            </button>
            <button
              onClick={handleSaveQuotas}
              disabled={savingQuota}
              className="px-4 py-2 rounded-xl bg-blue-600 text-white text-xs font-semibold hover:bg-blue-500 disabled:opacity-50"
            >
              {savingQuota ? 'Saving...' : 'Update Quotas'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
