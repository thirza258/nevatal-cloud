import React, { useState, useEffect } from 'react';
import { OSTemplate } from '../../types';
import { apiRequest } from '../../lib/api';
import { Modal } from '../../components/ui/Modal';
import { Layers, Plus, Search, Edit3, Trash2, CheckCircle2, XCircle, HardDrive, Cpu, Terminal, Disc, Upload } from 'lucide-react';

interface AdminTemplatesProps {
  onNavigate?: (path: string) => void;
}

export const AdminTemplates: React.FC<AdminTemplatesProps> = ({ onNavigate }) => {
  const [templates, setTemplates] = useState<OSTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');

  // Create / Edit modal state
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<OSTemplate | null>(null);
  const [saving, setSaving] = useState(false);

  // Form fields
  const [name, setName] = useState('');
  const [distribution, setDistribution] = useState('ubuntu');
  const [version, setVersion] = useState('');
  const [description, setDescription] = useState('');
  const [proxmoxVMID, setProxmoxVMID] = useState(9000);
  const [architecture, setArchitecture] = useState('x86_64');
  const [cloudInitSupported, setCloudInitSupported] = useState(true);
  const [defaultUsername, setDefaultUsername] = useState('ubuntu');
  const [enabled, setEnabled] = useState(true);

  const fetchTemplates = async () => {
    try {
      const data = await apiRequest<OSTemplate[]>('/admin/templates');
      setTemplates(data || []);
    } catch (err) {
      console.error('Failed to load templates:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTemplates();
  }, []);

  const openCreateModal = () => {
    setEditingTemplate(null);
    setName('');
    setDistribution('ubuntu');
    setVersion('');
    setDescription('');
    setProxmoxVMID(9003);
    setArchitecture('x86_64');
    setCloudInitSupported(true);
    setDefaultUsername('ubuntu');
    setEnabled(true);
    setIsModalOpen(true);
  };

  const openEditModal = (tmpl: OSTemplate) => {
    setEditingTemplate(tmpl);
    setName(tmpl.name);
    setDistribution(tmpl.distribution);
    setVersion(tmpl.version);
    setDescription(tmpl.description || '');
    setProxmoxVMID(tmpl.proxmox_template_vmid);
    setArchitecture(tmpl.architecture || 'x86_64');
    setCloudInitSupported(tmpl.cloud_init_supported);
    setDefaultUsername(tmpl.default_username || 'ubuntu');
    setEnabled(tmpl.enabled);
    setIsModalOpen(true);
  };

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    try {
      const payload = {
        name,
        distribution,
        version,
        description,
        proxmox_template_vmid: Number(proxmoxVMID),
        architecture,
        cloud_init_supported: cloudInitSupported,
        default_username: defaultUsername,
        enabled,
      };

      if (editingTemplate) {
        await apiRequest(`/admin/templates/${editingTemplate.id}`, {
          method: 'PUT',
          body: JSON.stringify(payload),
        });
      } else {
        await apiRequest('/admin/templates', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
      }

      setIsModalOpen(false);
      await fetchTemplates();
    } catch (err: any) {
      alert(err.message || 'Failed to save OS template');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (tmpl: OSTemplate) => {
    if (!confirm(`Are you sure you want to remove template "${tmpl.name}"?`)) {
      return;
    }

    try {
      await apiRequest(`/admin/templates/${tmpl.id}`, { method: 'DELETE' });
      await fetchTemplates();
    } catch (err: any) {
      alert(err.message || 'Failed to delete template');
    }
  };

  const filtered = templates.filter(
    (t) =>
      t.name.toLowerCase().includes(search.toLowerCase()) ||
      t.distribution.toLowerCase().includes(search.toLowerCase()) ||
      t.version.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-2xl font-bold text-white tracking-tight flex items-center gap-2.5">
            <Layers className="w-6 h-6 text-amber-400" />
            <span>Operating System Templates</span>
          </h1>
          <p className="text-sm text-slate-400 mt-1">
            Manage approved Proxmox VM base templates and cloud-init guest configurations.
          </p>
        </div>

        <div className="flex items-center gap-2.5 self-start sm:self-auto">
          {onNavigate && (
            <button
              onClick={() => onNavigate('/admin/isos')}
              className="flex items-center gap-2 px-3.5 py-2.5 rounded-xl bg-slate-900 hover:bg-slate-800 border border-slate-700/80 text-slate-200 text-xs font-semibold shadow-sm transition"
              title="Upload and manage raw bootable ISO images (.iso / .img)"
            >
              <Disc className="w-4 h-4 text-indigo-400" />
              <span>Upload ISO Media</span>
            </button>
          )}
          <button
            onClick={openCreateModal}
            className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-amber-600 hover:bg-amber-500 text-white text-xs font-semibold shadow-md shadow-amber-600/20 transition"
          >
            <Plus className="w-4 h-4" />
            <span>Add OS Template</span>
          </button>
        </div>
      </div>

      {/* ISO Media Helper Banner */}
      <div className="flex flex-col sm:flex-row items-start sm:items-center justify-between gap-3 bg-indigo-950/30 border border-indigo-800/40 rounded-xl px-4 py-3">
        <div className="flex items-center gap-3">
          <div className="p-2 bg-indigo-900/40 border border-indigo-700/40 rounded-lg text-indigo-400">
            <Disc className="w-5 h-5" />
          </div>
          <div>
            <div className="text-xs sm:text-sm font-semibold text-white">Looking to upload raw bootable ISO files (.iso)?</div>
            <div className="text-[11px] sm:text-xs text-indigo-300/80 mt-0.5">
              The templates below represent pre-baked Proxmox VM clones with cloud-init (<code className="text-indigo-200">#9000+</code>). To upload installable CD-ROM ISO media, use <strong>OS ISOs &amp; Media</strong>.
            </div>
          </div>
        </div>
        {onNavigate && (
          <button
            onClick={() => onNavigate('/admin/isos')}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-indigo-600 hover:bg-indigo-500 text-white text-xs font-semibold shrink-0 transition"
          >
            <Upload className="w-3.5 h-3.5" />
            <span>Go to OS ISOs</span>
          </button>
        )}
      </div>

      {/* Search Bar */}
      <div className="flex items-center gap-3 bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-300 w-full sm:w-80">
        <Search className="w-4 h-4 text-slate-500" />
        <input
          type="text"
          placeholder="Filter templates..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="bg-transparent border-none outline-none text-white w-full placeholder:text-slate-500 text-xs"
        />
      </div>

      {/* Templates Table */}
      <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-950/60 text-slate-400 font-semibold border-b border-slate-800 uppercase tracking-wider text-[11px]">
              <tr>
                <th className="py-3.5 px-4">Template Name</th>
                <th className="py-3.5 px-4">Distro / Version</th>
                <th className="py-3.5 px-4">Proxmox VMID</th>
                <th className="py-3.5 px-4">Cloud-Init</th>
                <th className="py-3.5 px-4">Default User</th>
                <th className="py-3.5 px-4">Status</th>
                <th className="py-3.5 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60 text-slate-300">
              {loading ? (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-slate-500">
                    Loading templates...
                  </td>
                </tr>
              ) : filtered.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-slate-500">
                    No OS templates found matching your search.
                  </td>
                </tr>
              ) : (
                filtered.map((tmpl) => (
                  <tr key={tmpl.id} className="hover:bg-slate-800/40 transition">
                    <td className="py-3.5 px-4">
                      <div className="font-semibold text-white">{tmpl.name}</div>
                      <div className="text-[11px] text-slate-400 line-clamp-1">
                        {tmpl.description || 'No description provided'}
                      </div>
                    </td>
                    <td className="py-3.5 px-4">
                      <span className="capitalize font-medium text-slate-200">{tmpl.distribution}</span>
                      <span className="text-slate-500 ml-1">({tmpl.version})</span>
                    </td>
                    <td className="py-3.5 px-4 font-mono font-medium text-amber-400">
                      #{tmpl.proxmox_template_vmid}
                    </td>
                    <td className="py-3.5 px-4">
                      {tmpl.cloud_init_supported ? (
                        <span className="inline-flex items-center gap-1 text-[11px] font-medium text-emerald-400 bg-emerald-950/60 border border-emerald-800/60 px-2 py-0.5 rounded-md">
                          <CheckCircle2 className="w-3 h-3" /> Enabled
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-[11px] font-medium text-slate-400 bg-slate-800/60 px-2 py-0.5 rounded-md">
                          <XCircle className="w-3 h-3" /> Disabled
                        </span>
                      )}
                    </td>
                    <td className="py-3.5 px-4 font-mono text-slate-400">
                      {tmpl.default_username}
                    </td>
                    <td className="py-3.5 px-4">
                      {tmpl.enabled ? (
                        <span className="inline-block text-[11px] font-semibold text-blue-400 bg-blue-950/60 border border-blue-800/60 px-2 py-0.5 rounded-full">
                          Active
                        </span>
                      ) : (
                        <span className="inline-block text-[11px] font-semibold text-slate-400 bg-slate-800/60 px-2 py-0.5 rounded-full">
                          Disabled
                        </span>
                      )}
                    </td>
                    <td className="py-3.5 px-4 text-right">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() => openEditModal(tmpl)}
                          className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 hover:text-white transition"
                          title="Edit Template"
                        >
                          <Edit3 className="w-3.5 h-3.5" />
                        </button>
                        <button
                          onClick={() => handleDelete(tmpl)}
                          className="p-1.5 rounded-lg bg-rose-950/40 hover:bg-rose-900/60 text-rose-400 hover:text-rose-200 transition"
                          title="Delete Template"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Create / Edit Modal */}
      <Modal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        title={editingTemplate ? 'Edit OS Template' : 'Add New OS Template'}
      >
        <form onSubmit={handleSave} className="space-y-4">
          <div>
            <label className="text-xs font-semibold text-slate-300 block mb-1">
              Template Display Name
            </label>
            <input
              type="text"
              required
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="e.g. Ubuntu Server 24.04 LTS"
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white outline-none focus:border-amber-500 transition"
            />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1">
                Distribution
              </label>
              <select
                value={distribution}
                onChange={(e) => setDistribution(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white outline-none focus:border-amber-500 transition"
              >
                <option value="ubuntu">Ubuntu</option>
                <option value="debian">Debian</option>
                <option value="alpine">Alpine</option>
                <option value="fedora">Fedora</option>
                <option value="rocky">Rocky Linux</option>
                <option value="arch">Arch Linux</option>
              </select>
            </div>
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1">
                Version Tag
              </label>
              <input
                type="text"
                required
                value={version}
                onChange={(e) => setVersion(e.target.value)}
                placeholder="e.g. 24.04 LTS"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white outline-none focus:border-amber-500 transition"
              />
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1">
                Proxmox Template VMID
              </label>
              <input
                type="number"
                required
                min={100}
                value={proxmoxVMID}
                onChange={(e) => setProxmoxVMID(Number(e.target.value))}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white font-mono outline-none focus:border-amber-500 transition"
              />
            </div>
            <div>
              <label className="text-xs font-semibold text-slate-300 block mb-1">
                Default SSH Username
              </label>
              <input
                type="text"
                required
                value={defaultUsername}
                onChange={(e) => setDefaultUsername(e.target.value)}
                placeholder="e.g. ubuntu"
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white font-mono outline-none focus:border-amber-500 transition"
              />
            </div>
          </div>

          <div>
            <label className="text-xs font-semibold text-slate-300 block mb-1">
              Description
            </label>
            <textarea
              rows={2}
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder="Brief description for customers selecting this OS template"
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3.5 py-2 text-xs text-white outline-none focus:border-amber-500 transition"
            />
          </div>

          <div className="flex items-center gap-6 pt-2">
            <label className="flex items-center gap-2 cursor-pointer text-xs text-slate-300">
              <input
                type="checkbox"
                checked={cloudInitSupported}
                onChange={(e) => setCloudInitSupported(e.target.checked)}
                className="rounded border-slate-700 bg-slate-900 text-amber-500 focus:ring-0"
              />
              <span>Cloud-Init Supported</span>
            </label>

            <label className="flex items-center gap-2 cursor-pointer text-xs text-slate-300">
              <input
                type="checkbox"
                checked={enabled}
                onChange={(e) => setEnabled(e.target.checked)}
                className="rounded border-slate-700 bg-slate-900 text-amber-500 focus:ring-0"
              />
              <span>Publicly Available</span>
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
              {saving ? 'Saving...' : editingTemplate ? 'Update Template' : 'Create Template'}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
};
