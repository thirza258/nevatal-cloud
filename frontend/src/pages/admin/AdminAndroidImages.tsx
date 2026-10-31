import React, { useState, useEffect } from 'react';
import {
  Smartphone,
  PlusCircle,
  RefreshCw,
  Layers,
  Check,
  X,
  Trash2,
  Shield,
  HardDrive,
  Eye,
  EyeOff,
  AlertCircle,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { AndroidImage } from '../../types';

export const AdminAndroidImages: React.FC = () => {
  const [images, setImages] = useState<AndroidImage[]>([]);
  const [loading, setLoading] = useState(true);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [actionLoading, setActionLoading] = useState<string | null>(null);

  // Form State
  const [name, setName] = useState('');
  const [slug, setSlug] = useState('');
  const [androidVersion, setAndroidVersion] = useState('13');
  const [architecture, setArchitecture] = useState('x86_64');
  const [imageType, setImageType] = useState('standard');
  const [systemImageUrl, setSystemImageUrl] = useState('');
  const [vendorImageUrl, setVendorImageUrl] = useState('');
  const [imageSource, setImageSource] = useState('official');
  const [checksum, setChecksum] = useState('');
  const [minCpu, setMinCpu] = useState(2);
  const [minRamMb, setMinRamMb] = useState(4096);
  const [minStorageGb, setMinStorageGb] = useState(32);
  const [description, setDescription] = useState('');

  const fetchImages = async () => {
    try {
      setLoading(true);
      const res = await apiRequest<AndroidImage[]>('/admin/android/images');
      setImages(res || []);
    } catch (err: any) {
      console.error('Failed to fetch admin Android images:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchImages();
  }, []);

  const handleTogglePublish = async (img: AndroidImage) => {
    try {
      setActionLoading(`pub-${img.id}`);
      const endpoint = img.is_published
        ? `/admin/android/images/${img.id}/unpublish`
        : `/admin/android/images/${img.id}/publish`;
      await apiRequest(endpoint, { method: 'PATCH' });
      await fetchImages();
    } catch (err: any) {
      alert(err.message || 'Failed to toggle publication status');
    } finally {
      setActionLoading(null);
    }
  };

  const handleVerify = async (id: string) => {
    try {
      setActionLoading(`verify-${id}`);
      await apiRequest(`/admin/android/images/${id}/verify`, { method: 'POST' });
      await fetchImages();
      alert('Checksum verification passed!');
    } catch (err: any) {
      alert(err.message || 'Verification failed');
    } finally {
      setActionLoading(null);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this Android image package?')) return;
    try {
      setActionLoading(`del-${id}`);
      await apiRequest(`/admin/android/images/${id}`, { method: 'DELETE' });
      await fetchImages();
    } catch (err: any) {
      alert(err.message || 'Failed to delete image');
    } finally {
      setActionLoading(null);
    }
  };

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      setActionLoading('create');
      await apiRequest('/admin/android/images', {
        method: 'POST',
        body: JSON.stringify({
          name,
          slug: slug || name.toLowerCase().replace(/[^a-z0-9]+/g, '-'),
          android_version: androidVersion,
          architecture,
          image_type: imageType,
          system_image_url: systemImageUrl,
          vendor_image_url: vendorImageUrl,
          image_source: imageSource,
          checksum_sha256: checksum,
          min_cpu: Number(minCpu),
          min_ram_mb: Number(minRamMb),
          min_storage_gb: Number(minStorageGb),
          description,
        }),
      });
      setShowCreateModal(false);
      resetForm();
      await fetchImages();
    } catch (err: any) {
      alert(err.message || 'Failed to create image');
    } finally {
      setActionLoading(null);
    }
  };

  const resetForm = () => {
    setName('');
    setSlug('');
    setAndroidVersion('13');
    setArchitecture('x86_64');
    setImageType('standard');
    setSystemImageUrl('');
    setVendorImageUrl('');
    setImageSource('official');
    setChecksum('');
    setMinCpu(2);
    setMinRamMb(4096);
    setMinStorageGb(32);
    setDescription('');
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4">
        <div>
          <div className="flex items-center gap-3">
            <div className="p-2.5 rounded-xl bg-purple-500/10 border border-purple-500/20 text-purple-400">
              <Smartphone className="w-6 h-6" />
            </div>
            <div>
              <h1 className="text-2xl font-bold text-white tracking-tight">Waydroid Image Registry</h1>
              <p className="text-sm text-slate-400">
                Manage system and vendor images, architecture compatibility, and tenant visibility
              </p>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={fetchImages}
            disabled={loading}
            className="p-2 rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-white transition-colors"
          >
            <RefreshCw className={`w-4 h-4 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={() => setShowCreateModal(true)}
            className="flex items-center gap-2 px-4 py-2 bg-purple-600 hover:bg-purple-500 text-white rounded-lg text-sm font-medium transition-all shadow-lg shadow-purple-500/20"
          >
            <PlusCircle className="w-4 h-4" />
            <span>Register Android Image</span>
          </button>
        </div>
      </div>

      {/* Table */}
      <div className="rounded-2xl bg-slate-900/80 border border-slate-800 overflow-hidden">
        <table className="w-full text-left text-xs">
          <thead className="bg-slate-950/80 text-slate-400 border-b border-slate-800 uppercase tracking-wider font-semibold">
            <tr>
              <th className="p-4">Image Name</th>
              <th className="p-4">Android Release</th>
              <th className="p-4">Arch & Type</th>
              <th className="p-4">Min Specs</th>
              <th className="p-4">Status & Checksum</th>
              <th className="p-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60 text-slate-300">
            {images.map((img) => (
              <tr key={img.id} className="hover:bg-slate-800/30 transition-colors">
                <td className="p-4">
                  <div className="font-semibold text-white">{img.name}</div>
                  <div className="text-[11px] text-slate-500 font-mono">{img.slug}</div>
                </td>
                <td className="p-4 font-mono text-emerald-400">Android {img.android_version}</td>
                <td className="p-4">
                  <div>{img.architecture}</div>
                  <div className="text-[10px] text-purple-400 uppercase tracking-wider">
                    {img.image_type}
                  </div>
                </td>
                <td className="p-4">
                  {img.min_cpu} vCPU • {Math.round(img.min_ram_mb / 1024)} GB RAM • {img.min_storage_gb} GB
                </td>
                <td className="p-4">
                  <div className="flex items-center gap-1.5">
                    {img.is_published ? (
                      <span className="px-2 py-0.5 rounded-full bg-emerald-500/20 text-emerald-300 text-[10px] font-medium border border-emerald-500/30">
                        Published
                      </span>
                    ) : (
                      <span className="px-2 py-0.5 rounded-full bg-slate-800 text-slate-400 text-[10px] font-medium border border-slate-700">
                        Unpublished
                      </span>
                    )}
                    {img.is_verified && (
                      <span className="px-2 py-0.5 rounded-full bg-blue-500/20 text-blue-300 text-[10px] font-medium border border-blue-500/30">
                        SHA256 Verified
                      </span>
                    )}
                  </div>
                </td>
                <td className="p-4 text-right">
                  <div className="flex items-center justify-end gap-2">
                    <button
                      onClick={() => handleTogglePublish(img)}
                      className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition-colors"
                      title={img.is_published ? 'Unpublish' : 'Publish'}
                    >
                      {img.is_published ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={() => handleVerify(img.id)}
                      className="p-1.5 rounded-lg text-slate-400 hover:text-blue-400 hover:bg-slate-800 transition-colors"
                      title="Verify Checksum"
                    >
                      <Shield className="w-4 h-4" />
                    </button>
                    <button
                      onClick={() => handleDelete(img.id)}
                      className="p-1.5 rounded-lg text-slate-400 hover:text-red-400 hover:bg-slate-800 transition-colors"
                      title="Delete Image"
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

      {/* MODAL: Register Android Image */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 bg-black/70 backdrop-blur-sm flex items-center justify-center p-4">
          <form
            onSubmit={handleCreate}
            className="w-full max-w-lg p-6 rounded-2xl bg-slate-900 border border-slate-800 space-y-4 max-h-[90vh] overflow-y-auto"
          >
            <h3 className="text-base font-semibold text-white">Register Waydroid System Image</h3>

            <div className="space-y-1.5">
              <label className="text-xs text-slate-300">Image Name</label>
              <input
                type="text"
                required
                placeholder="e.g. LineageOS 20 (Android 13)"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-purple-500"
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <label className="text-xs text-slate-300">Android Version</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. 13"
                  value={androidVersion}
                  onChange={(e) => setAndroidVersion(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-purple-500"
                />
              </div>

              <div className="space-y-1.5">
                <label className="text-xs text-slate-300">Architecture</label>
                <select
                  value={architecture}
                  onChange={(e) => setArchitecture(e.target.value)}
                  className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-purple-500"
                >
                  <option value="x86_64">x86_64 (Intel / AMD)</option>
                  <option value="arm64">arm64 (AArch64)</option>
                </select>
              </div>
            </div>

            <div className="space-y-1.5">
              <label className="text-xs text-slate-300">System Image URL / Path</label>
              <input
                type="text"
                required
                placeholder="https://images.nevatal.id/waydroid/system.img"
                value={systemImageUrl}
                onChange={(e) => setSystemImageUrl(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-purple-500"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs text-slate-300">Vendor Image URL / Path</label>
              <input
                type="text"
                required
                placeholder="https://images.nevatal.id/waydroid/vendor.img"
                value={vendorImageUrl}
                onChange={(e) => setVendorImageUrl(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-purple-500"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs text-slate-300">SHA256 Checksum</label>
              <input
                type="text"
                required
                placeholder="64-character hex hash"
                value={checksum}
                onChange={(e) => setChecksum(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs font-mono focus:outline-none focus:border-purple-500"
              />
            </div>

            <div className="space-y-1.5">
              <label className="text-xs text-slate-300">Description</label>
              <textarea
                rows={2}
                placeholder="Short description of this build and compatibility notes..."
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                className="w-full px-3 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-xs focus:outline-none focus:border-purple-500"
              />
            </div>

            <div className="flex justify-end gap-2 pt-2">
              <button
                type="button"
                onClick={() => setShowCreateModal(false)}
                className="px-3 py-1.5 rounded-lg text-xs text-slate-400 hover:text-white"
              >
                Cancel
              </button>
              <button
                type="submit"
                disabled={actionLoading === 'create'}
                className="px-4 py-1.5 rounded-lg bg-purple-600 hover:bg-purple-500 text-white text-xs font-medium"
              >
                {actionLoading === 'create' ? 'Registering...' : 'Register Image'}
              </button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
};
