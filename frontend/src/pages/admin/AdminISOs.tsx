import React, { useEffect, useState, useRef } from 'react';
import {
  Disc,
  Plus,
  Upload,
  CheckCircle2,
  AlertTriangle,
  HardDrive,
  Copy,
  Check,
  Eye,
  EyeOff,
  Trash2,
  ShieldCheck,
  Cpu
} from 'lucide-react';
import { OSImage } from '../../types';

export const AdminISOs: React.FC = () => {
  const [images, setImages] = useState<OSImage[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Upload modal
  const [showModal, setShowModal] = useState(false);
  const [name, setName] = useState('');
  const [version, setVersion] = useState('');
  const [arch, setArch] = useState('x86_64');
  const [category, setCategory] = useState('Linux');
  const [description, setDescription] = useState('');
  const [proxmoxStorage, setProxmoxStorage] = useState('local');
  const [visibility, setVisibility] = useState<'published' | 'unpublished'>('published');
  const [file, setFile] = useState<File | null>(null);
  const [uploading, setUploading] = useState(false);
  const [copiedSha, setCopiedSha] = useState<string | null>(null);

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const fetchISOs = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/admin/isos?all=true', {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to load ISO images');
      setImages(data.data || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchISOs();
  }, []);

  const handleUpload = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!file) {
      alert('Please select an ISO file to upload');
      return;
    }

    try {
      setUploading(true);
      const token = localStorage.getItem('token');
      const formData = new FormData();
      formData.append('file', file);
      formData.append('name', name.trim());
      formData.append('version', version.trim());
      formData.append('architecture', arch);
      formData.append('category', category);
      formData.append('description', description);
      formData.append('proxmox_storage', proxmoxStorage);
      formData.append('visibility', visibility);

      const res = await fetch('/api/v1/admin/isos/upload', {
        method: 'POST',
        headers: { Authorization: `Bearer ${token}` },
        body: formData
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Upload failed');

      setShowModal(false);
      setName('');
      setVersion('');
      setDescription('');
      setFile(null);
      await fetchISOs();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setUploading(false);
    }
  };

  const handleTogglePublish = async (img: OSImage) => {
    const newVis = img.visibility === 'published' ? 'unpublished' : 'published';
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/admin/isos/${img.id}/publish`, {
        method: 'PATCH',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({ visibility: newVis })
      });
      if (!res.ok) throw new Error('Failed to update visibility');
      await fetchISOs();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to remove this ISO image?')) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/admin/isos/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to delete ISO');
      await fetchISOs();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-800 pb-5">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-white">OS ISOs & Media Library</h1>
            <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-blue-500/10 text-blue-400 border border-blue-500/20">
              Proxmox Storage
            </span>
          </div>
          <p className="text-sm text-slate-400 mt-1">
            Upload verified operating system ISO images and control customer availability for bare-metal VM installs.
          </p>
        </div>

        <button
          onClick={() => setShowModal(true)}
          className="inline-flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-blue-500/20 cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          Upload New ISO
        </button>
      </div>

      {/* ISO Table */}
      <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl overflow-hidden">
        <table className="w-full text-left text-xs">
          <thead className="bg-slate-950/60 text-slate-400 uppercase tracking-wider border-b border-slate-800 font-mono">
            <tr>
              <th className="py-3 px-4">Distribution & Name</th>
              <th className="py-3 px-4">Version / Arch</th>
              <th className="py-3 px-4">Size</th>
              <th className="py-3 px-4">SHA-256 Checksum</th>
              <th className="py-3 px-4">Proxmox Storage</th>
              <th className="py-3 px-4">Visibility</th>
              <th className="py-3 px-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60 text-slate-300">
            {images.length === 0 ? (
              <tr>
                <td colSpan={7} className="py-8 text-center text-slate-500">
                  No ISO images cataloged. Upload an ISO to enable customer custom installations.
                </td>
              </tr>
            ) : (
              images.map((img) => (
                <tr key={img.id} className="hover:bg-slate-800/30 transition">
                  <td className="py-3 px-4 font-medium text-white flex items-center gap-2">
                    <Disc className="w-4 h-4 text-blue-400 flex-shrink-0" />
                    <div>
                      <div className="font-semibold">{img.name}</div>
                      <div className="text-[11px] font-mono text-slate-500">{img.filename}</div>
                    </div>
                  </td>
                  <td className="py-3 px-4">
                    <span className="font-mono text-slate-200">{img.version}</span>
                    <span className="ml-1 text-[10px] text-slate-500 font-mono">({img.architecture})</span>
                  </td>
                  <td className="py-3 px-4 font-mono">{formatBytes(img.file_size_bytes)}</td>
                  <td className="py-3 px-4">
                    <div className="flex items-center gap-1.5 font-mono text-[11px] text-slate-400">
                      <span className="truncate max-w-[120px]">{img.sha256}</span>
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(img.sha256);
                          setCopiedSha(img.id);
                          setTimeout(() => setCopiedSha(null), 2000);
                        }}
                        className="text-slate-500 hover:text-white"
                        title="Copy SHA-256"
                      >
                        {copiedSha === img.id ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                      </button>
                    </div>
                  </td>
                  <td className="py-3 px-4 font-mono text-slate-400">{img.proxmox_volume}</td>
                  <td className="py-3 px-4">
                    <span
                      className={`px-2 py-0.5 rounded text-[10px] font-medium uppercase border ${
                        img.visibility === 'published'
                          ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                          : 'bg-slate-800 text-slate-400 border-slate-700'
                      }`}
                    >
                      {img.visibility}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-right">
                    <div className="flex items-center justify-end gap-1.5">
                      <button
                        onClick={() => handleTogglePublish(img)}
                        className="p-1 rounded text-slate-400 hover:text-blue-400 hover:bg-slate-800 transition cursor-pointer"
                        title={img.visibility === 'published' ? 'Unpublish from customer catalog' : 'Publish to customer catalog'}
                      >
                        {img.visibility === 'published' ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                      </button>
                      <button
                        onClick={() => handleDelete(img.id)}
                        className="p-1 rounded text-slate-400 hover:text-red-400 hover:bg-slate-800 transition cursor-pointer"
                        title="Delete ISO"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Upload Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 max-w-lg w-full space-y-4">
            <h2 className="text-base font-bold text-white tracking-tight flex items-center gap-2">
              <Upload className="w-4 h-4 text-blue-400" />
              Upload Operating System ISO
            </h2>

            <form onSubmit={handleUpload} className="space-y-4">
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Name</label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. AlmaLinux"
                    value={name}
                    onChange={(e) => setName(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Version</label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. 9.5"
                    value={version}
                    onChange={(e) => setVersion(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>
              </div>

              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Category</label>
                  <select
                    value={category}
                    onChange={(e) => setCategory(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="Linux">Linux</option>
                    <option value="Windows">Windows</option>
                    <option value="BSD">BSD</option>
                    <option value="Other">Other</option>
                  </select>
                </div>
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Proxmox Storage</label>
                  <select
                    value={proxmoxStorage}
                    onChange={(e) => setProxmoxStorage(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="local">local (Default ISO storage)</option>
                    <option value="local-zfs">local-zfs</option>
                  </select>
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Description</label>
                <input
                  type="text"
                  placeholder="e.g. Enterprise grade Linux distribution"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1">Select ISO File</label>
                <input
                  type="file"
                  accept=".iso,.img"
                  required
                  onChange={(e) => setFile(e.target.files ? e.target.files[0] : null)}
                  className="w-full text-xs text-slate-400 file:mr-3 file:py-1.5 file:px-3 file:rounded-lg file:border-0 file:text-xs file:font-medium file:bg-blue-600 file:text-white hover:file:bg-blue-500 cursor-pointer"
                />
              </div>

              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={uploading}
                  className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg text-xs font-medium cursor-pointer disabled:opacity-50"
                >
                  {uploading ? 'Streaming & Verifying SHA-256...' : 'Upload & Catalog'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
