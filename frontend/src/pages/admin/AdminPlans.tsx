import React, { useState, useEffect } from 'react';
import { ResourcePlan } from '../../types';
import { apiRequest } from '../../lib/api';
import { Modal } from '../../components/ui/Modal';
import { HardDrive, Plus, Search, Edit3, Trash2, Cpu, Server, Check } from 'lucide-react';

export const AdminPlans: React.FC = () => {
  const [plans, setPlans] = useState<ResourcePlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  // Modal state
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingPlan, setEditingPlan] = useState<ResourcePlan | null>(null);
  const [saving, setSaving] = useState(false);

  // Form fields
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [vcpus, setVCPUs] = useState(2);
  const [memoryMB, setMemoryMB] = useState(2048);
  const [diskGB, setDiskGB] = useState(40);
  const [enabled, setEnabled] = useState(true);

  const fetchPlans = async () => {
    try {
      const data = await apiRequest<ResourcePlan[]>('/admin/plans');
      setPlans(data || []);
    } catch (err) {
      console.error('Failed to load plans:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchPlans();
  }, []);

  const openCreateModal = () => {
    setEditingPlan(null);
    setName('');
    setDescription('');
    setVCPUs(2);
    setMemoryMB(4096);
    setDiskGB(50);
    setEnabled(true);
    setIsModalOpen(true);
  };

  const openEditModal = (plan: ResourcePlan) => {
    setEditingPlan(plan);
    setName(plan.name);
    setDescription(plan.description || '');
    setVCPUs(plan.vcpus);
    setMemoryMB(plan.memory_mb);
    setDiskGB(plan.disk_gb);
    setEnabled(plan.enabled);
    setIsModalOpen(true);
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const payload = {
        name,
        description,
        vcpus: Number(vcpus),
        memory_mb: Number(memoryMB),
        disk_gb: Number(diskGB),
        enabled,
      };

      if (editingPlan) {
        await apiRequest(`/admin/plans/${editingPlan.id}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
      } else {
        await apiRequest('/admin/plans', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
      }

      setIsModalOpen(false);
      await fetchPlans();
    } catch (err: any) {
      alert(err.message || 'Failed to save resource plan');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (plan: ResourcePlan) => {
    if (!confirm(`Are you sure you want to remove plan "${plan.name}"?`)) {
      return;
    }

    try {
      await apiRequest(`/admin/plans/${plan.id}`, { method: 'DELETE' });
      await fetchPlans();
    } catch (err: any) {
      alert(err.message || 'Failed to delete plan');
    }
  };

  const filtered = plans.filter(
    (p) =>
      p.name.toLowerCase().includes(search.toLowerCase()) ||
      (p.description && p.description.toLowerCase().includes(search.toLowerCase()))
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white tracking-tight flex items-center gap-2.5">
            <HardDrive className="w-6 h-6 text-amber-400" />
            <span>Resource Plans</span>
          </h1>
          <p className="text-sm text-slate-400 mt-1">
            Configure preset vCPU, RAM, and Disk hardware tiers available in the instance wizard.
          </p>
        </div>

        <button
          onClick={openCreateModal}
          className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-md shadow-amber-600/20 transition self-start sm:self-auto"
        >
          <Plus className="w-4 h-4" />
          <span>Add Plan</span>
        </button>
      </div>

      {/* Search Bar */}
      <div className="flex items-center gap-3 bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-300 w-full sm:w-80">
        <Search className="w-4 h-4 text-slate-500" />
        <input
          type="text"
          placeholder="Filter plans..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="bg-transparent border-none outline-none text-white w-full placeholder:text-slate-500 text-xs"
        />
      </div>

      {/* Grid of Plans */}
      <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
        {loading ? (
          <div className="col-span-full py-12 text-center text-slate-500 text-xs">
            Loading resource plans...
          </div>
        ) : filtered.length === 0 ? (
          <div className="col-span-full py-12 text-center text-slate-500 text-xs">
            No plans found matching your search.
          </div>
        ) : (
          filtered.map((plan) => (
            <div
              key={plan.id}
              className="bg-slate-900 border border-slate-800 hover:border-slate-700/80 rounded-2xl p-5 flex flex-col justify-between transition group shadow-sm"
            >
              <div>
                <div className="flex items-center justify-between gap-2 mb-2">
                  <h3 className="font-bold text-white text-base">{plan.name}</h3>
                  {plan.enabled ? (
                    <span className="text-[10px] font-semibold text-emerald-400 bg-emerald-950/60 border border-emerald-800/60 px-2 py-0.5 rounded-full">
                      Active
                    </span>
                  ) : (
                    <span className="text-[10px] font-semibold text-slate-400 bg-slate-800/60 px-2 py-0.5 rounded-full">
                      Disabled
                    </span>
                  )}
                </div>

                <p className="text-xs text-slate-400 min-h-[36px] line-clamp-2 mb-4">
                  {plan.description || 'Custom computing configuration'}
                </p>

                <div className="space-y-2 bg-slate-950/60 border border-slate-800/80 rounded-xl p-3 text-xs mb-4">
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500 flex items-center gap-1.5">
                      <Cpu className="w-3.5 h-3.5 text-blue-400" /> vCPU
                    </span>
                    <span className="font-semibold text-white font-mono">{plan.vcpus} Cores</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500 flex items-center gap-1.5">
                      <Server className="w-3.5 h-3.5 text-purple-400" /> Memory
                    </span>
                    <span className="font-semibold text-white font-mono">
                      {Math.round(plan.memory_mb / 1024)} GB ({plan.memory_mb} MB)
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-slate-500 flex items-center gap-1.5">
                      <HardDrive className="w-3.5 h-3.5 text-emerald-400" /> Storage
                    </span>
                    <span className="font-semibold text-white font-mono">{plan.disk_gb} GB SSD</span>
                  </div>
                </div>
              </div>

              <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-800/80">
                <button
                  onClick={() => openEditModal(plan)}
                  className="flex items-center gap-1 px-2.5 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white text-xs font-semibold transition"
                >
                  <Edit3 className="w-3.5 h-3.5" />
                  <span>Edit</span>
                </button>
                <button
                  onClick={() => handleDelete(plan)}
                  className="p-1.5 rounded-lg bg-rose-950/40 hover:bg-rose-900/60 text-rose-400 hover:text-rose-200 transition"
                  title="Delete Plan"
                >
                  <Trash2 className="w-3.5 h-3.5" />
                </button>
              </div>
            </div>
          ))
        )}
      </div>

      {/* Create / Edit Modal */}
      <Modal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        title={editingPlan ? 'Edit Resource Plan' : 'Add New Resource Plan'}
      >
        <form onSubmit={handleSave} className="space-y-4">
          <div>
            <label className="text-xs font-semibold text-slate-300 block mb-1">Plan Name</label>
            <input
              type="text"
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Standard 2x"
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white outline-none focus:border-amber-500 transition"
            />
          </div>

          <div>
            <label className="text-xs font-semibold text-slate-300 block mb-1">Description</label>
            <textarea
              rows={2}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Recommended use case or target workload"
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white outline-none focus:border-amber-500 transition"
            />
          </div>

          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1">vCPUs</label>
              <input
                type="number"
                required
                min={1}
                max={64}
                value={vcpus}
                onChange={(e) => setVCPUs(Number(e.target.value))}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white font-mono outline-none focus:border-amber-500 transition"
              />
            </div>
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1">Memory (MB)</label>
              <input
                type="number"
                required
                min={512}
                step={512}
                value={memoryMB}
                onChange={(e) => setMemoryMB(Number(e.target.value))}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white font-mono outline-none focus:border-amber-500 transition"
              />
            </div>
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1">Disk (GB)</label>
              <input
                type="number"
                required
                min={10}
                value={diskGB}
                onChange={(e) => setDiskGB(Number(e.target.value))}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white font-mono outline-none focus:border-amber-500 transition"
              />
            </div>
          </div>

          <div className="pt-2">
            <label className="flex items-center gap-2 cursor-pointer text-xs text-slate-300">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="rounded border-slate-700 bg-slate-900 text-amber-500 focus:ring-0"
              />
              <span>Enabled for instance creation</span>
            </label>
          </div>

          <div className="flex items-center justify-end gap-3 pt-4 border-t border-slate-800">
            <button
              type="button"
              onClick={() => setIsModalOpen(false)}
              className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold transition"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="px-4 py-2 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-md shadow-amber-600/20 transition disabled:opacity-50"
            >
              {saving ? 'Saving...' : editingPlan ? 'Update Plan' : 'Create Plan'}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
};
