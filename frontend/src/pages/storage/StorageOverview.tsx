import React, { useEffect, useState } from 'react';
import {
  Database,
  Plus,
  Folder,
  Key,
  HardDrive,
  Globe,
  Lock,
  ExternalLink,
  Trash2,
  CheckCircle2,
  AlertTriangle,
  FolderArchive,
  ArrowUpRight,
  Server
} from 'lucide-react';
import { StorageBucket, StorageOverview } from '../../types';

interface StorageOverviewProps {
  onNavigate: (path: string, param?: string) => void;
}

export const StorageOverviewPage: React.FC<StorageOverviewProps> = ({ onNavigate }) => {
  const [buckets, setBuckets] = useState<StorageBucket[]>([]);
  const [overview, setOverview] = useState<StorageOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Create bucket modal
  const [showModal, setShowModal] = useState(false);
  const [bucketName, setBucketName] = useState('');
  const [description, setDescription] = useState('');
  const [visibility, setVisibility] = useState<'private' | 'public'>('private');
  const [createLoading, setCreateLoading] = useState(false);

  const fetchData = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const [bRes, oRes] = await Promise.all([
        fetch('/api/v1/storage/buckets', { headers: { Authorization: `Bearer ${token}` } }),
        fetch('/api/v1/storage/overview', { headers: { Authorization: `Bearer ${token}` } })
      ]);

      const bData = await bRes.json();
      const oData = await oRes.json();

      if (!bRes.ok) throw new Error(bData.error?.message || 'Failed to load storage buckets');
      setBuckets(bData.data || []);
      if (oRes.ok) setOverview(oData.data);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleCreateBucket = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!bucketName.trim()) return;

    try {
      setCreateLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/storage/buckets', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          name: bucketName.trim().toLowerCase(),
          description,
          visibility,
          region: 'id-central-1'
        })
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to create bucket');

      setShowModal(false);
      setBucketName('');
      setDescription('');
      await fetchData();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setCreateLoading(false);
    }
  };

  const handleDeleteBucket = async (e: React.MouseEvent, bucket: StorageBucket) => {
    e.stopPropagation();
    if (!confirm(`Are you sure you want to delete bucket '${bucket.name}'?`)) return;

    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/storage/buckets/${bucket.id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to delete bucket');
      await fetchData();
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
            <h1 className="text-2xl font-bold tracking-tight text-white">S3 Object Storage</h1>
            <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
              MinIO S3 Compatible
            </span>
          </div>
          <p className="text-sm text-slate-400 mt-1">
            High-speed S3 compatible storage buckets for application media, datasets, and static creator assets.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => onNavigate('/dashboard/storage/access-keys')}
            className="inline-flex items-center gap-2 px-3.5 py-2 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg text-sm font-medium transition cursor-pointer"
          >
            <Key className="w-4 h-4 text-cyan-400" />
            S3 Access Keys
          </button>
          <button
            onClick={() => setShowModal(true)}
            className="inline-flex items-center gap-2 px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-cyan-500/20 cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            Create Bucket
          </button>
        </div>
      </div>

      {/* Stats Cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80">
          <div className="text-xs text-slate-500 flex items-center gap-1.5 mb-1">
            <FolderArchive className="w-3.5 h-3.5 text-cyan-400" />
            Total Buckets
          </div>
          <div className="text-2xl font-bold text-white">{overview?.total_buckets || buckets.length}</div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80">
          <div className="text-xs text-slate-500 flex items-center gap-1.5 mb-1">
            <Database className="w-3.5 h-3.5 text-blue-400" />
            Total Objects
          </div>
          <div className="text-2xl font-bold text-white">{overview?.total_objects || 0}</div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80">
          <div className="text-xs text-slate-500 flex items-center gap-1.5 mb-1">
            <HardDrive className="w-3.5 h-3.5 text-emerald-400" />
            Storage Used
          </div>
          <div className="text-2xl font-bold text-white">{formatBytes(overview?.total_size_bytes || 0)}</div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80">
          <div className="text-xs text-slate-500 flex items-center gap-1.5 mb-1">
            <Key className="w-3.5 h-3.5 text-purple-400" />
            API Access Keys
          </div>
          <div className="text-2xl font-bold text-white">{overview?.total_access_keys || 0}</div>
        </div>
      </div>

      {loading ? (
        <div className="p-12 text-center text-slate-500 flex flex-col items-center justify-center gap-3">
          <div className="w-8 h-8 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin" />
          <span className="text-sm">Loading storage buckets...</span>
        </div>
      ) : buckets.length === 0 ? (
        <div className="text-center py-16 px-4 rounded-xl border border-dashed border-slate-800 bg-slate-900/30">
          <Database className="w-12 h-12 text-slate-600 mx-auto mb-4" />
          <h3 className="text-lg font-semibold text-slate-200">No Storage Buckets Created</h3>
          <p className="text-sm text-slate-400 max-w-md mx-auto mt-1 mb-6">
            Create an S3 bucket to store media, user uploads, logs, and static files with AWS S3 SDK compatibility.
          </p>
          <button
            onClick={() => setShowModal(true)}
            className="inline-flex items-center gap-2 px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-sm font-medium transition cursor-pointer"
          >
            <Plus className="w-4 h-4" />
            Create Your First Bucket
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
          {buckets.map((b) => (
            <div
              key={b.id}
              onClick={() => onNavigate('/dashboard/storage/files', b.id)}
              className="bg-slate-900/80 border border-slate-800/80 hover:border-slate-700/80 rounded-xl p-5 transition duration-150 cursor-pointer flex flex-col justify-between group shadow-sm hover:shadow-md"
            >
              <div>
                <div className="flex items-start justify-between gap-3 mb-3">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-lg bg-cyan-500/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400 group-hover:bg-cyan-600 group-hover:text-white transition">
                      <FolderArchive className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="text-base font-semibold text-white group-hover:text-cyan-400 transition font-mono">
                        {b.name}
                      </h3>
                      <span className="text-xs text-slate-400">{b.region}</span>
                    </div>
                  </div>

                  <span
                    className={`text-xs px-2 py-0.5 rounded-full font-medium border flex items-center gap-1 ${
                      b.visibility === 'public'
                        ? 'bg-amber-500/10 text-amber-400 border-amber-500/20'
                        : 'bg-slate-800 text-slate-400 border-slate-700'
                    }`}
                  >
                    {b.visibility === 'public' ? <Globe className="w-3 h-3" /> : <Lock className="w-3 h-3" />}
                    <span className="capitalize">{b.visibility}</span>
                  </span>
                </div>

                <p className="text-xs text-slate-400 line-clamp-2 mt-2">
                  {b.description || 'General S3 object storage container.'}
                </p>

                {/* S3 URI */}
                <div className="mt-4 p-2.5 rounded-lg bg-slate-950/60 border border-slate-800/80 text-xs font-mono text-slate-400 flex items-center justify-between">
                  <span className="truncate">s3://{b.name}</span>
                  <ExternalLink className="w-3 h-3 text-slate-500 flex-shrink-0 ml-1" />
                </div>
              </div>

              <div className="mt-5 pt-4 border-t border-slate-800/60 flex items-center justify-between text-xs text-slate-500">
                <span>{b.total_objects} Objects • {formatBytes(b.total_size_bytes)}</span>
                <div className="flex items-center gap-2">
                  <button
                    onClick={(e) => handleDeleteBucket(e, b)}
                    className="p-1 rounded text-slate-500 hover:text-red-400 transition cursor-pointer"
                    title="Delete bucket"
                  >
                    <Trash2 className="w-4 h-4" />
                  </button>
                  <span className="text-slate-400 group-hover:text-cyan-400 flex items-center gap-1 font-medium">
                    Explore <ArrowUpRight className="w-3 h-3" />
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* S3 Endpoint Connection Card */}
      <div className="bg-slate-900/40 border border-slate-800 rounded-xl p-5 space-y-3">
        <h3 className="text-sm font-semibold text-white flex items-center gap-2">
          <Server className="w-4 h-4 text-cyan-400" />
          S3 API Endpoint Connection Info
        </h3>
        <p className="text-xs text-slate-400 leading-relaxed">
          Nevatal S3 Storage is 100% compatible with AWS CLI, AWS SDKs, Cyberduck, Rclone, and Boto3. Configure your clients using these settings:
        </p>
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 text-xs font-mono">
          <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800">
            <span className="text-slate-500 block mb-0.5">S3 Endpoint URL:</span>
            <span className="text-cyan-400">https://s3.nevatal.id</span>
          </div>
          <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800">
            <span className="text-slate-500 block mb-0.5">Region:</span>
            <span className="text-slate-300">id-central-1</span>
          </div>
          <div className="p-2.5 rounded-lg bg-slate-950 border border-slate-800">
            <span className="text-slate-500 block mb-0.5">Addressing Style:</span>
            <span className="text-slate-300">Path-Style (Supported)</span>
          </div>
        </div>
      </div>

      {/* Create Bucket Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 max-w-md w-full space-y-4 shadow-xl">
            <h2 className="text-base font-bold text-white tracking-tight">Create S3 Storage Bucket</h2>

            <form onSubmit={handleCreateBucket} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">Bucket Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. assets-production"
                  value={bucketName}
                  onChange={(e) => setBucketName(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, ''))}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-cyan-500 font-mono"
                />
                <span className="text-[11px] text-slate-500 mt-1 block">
                  Must be unique, lowercase, 3-63 characters, numbers & hyphens only.
                </span>
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">Description (Optional)</label>
                <input
                  type="text"
                  placeholder="e.g. Creator video uploads and raw captures"
                  value={description}
                  onChange={(e) => setDescription(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-cyan-500"
                />
              </div>

              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">Bucket Visibility</label>
                <div className="grid grid-cols-2 gap-3">
                  <button
                    type="button"
                    onClick={() => setVisibility('private')}
                    className={`p-3 rounded-lg border text-left transition cursor-pointer ${
                      visibility === 'private'
                        ? 'bg-cyan-600/10 border-cyan-500 text-white'
                        : 'bg-slate-950/40 border-slate-800 text-slate-400'
                    }`}
                  >
                    <div className="flex items-center gap-1.5 font-semibold text-xs">
                      <Lock className="w-3.5 h-3.5 text-cyan-400" /> Private
                    </div>
                    <div className="text-[10px] text-slate-500 mt-0.5">S3 Auth required</div>
                  </button>

                  <button
                    type="button"
                    onClick={() => setVisibility('public')}
                    className={`p-3 rounded-lg border text-left transition cursor-pointer ${
                      visibility === 'public'
                        ? 'bg-amber-600/10 border-amber-500 text-white'
                        : 'bg-slate-950/40 border-slate-800 text-slate-400'
                    }`}
                  >
                    <div className="flex items-center gap-1.5 font-semibold text-xs">
                      <Globe className="w-3.5 h-3.5 text-amber-400" /> Public
                    </div>
                    <div className="text-[10px] text-slate-500 mt-0.5">Direct HTTPS CDN</div>
                  </button>
                </div>
              </div>

              <div className="flex items-center justify-end gap-3 pt-3">
                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs font-medium transition cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createLoading}
                  className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-xs font-medium transition cursor-pointer disabled:opacity-50"
                >
                  {createLoading ? 'Creating...' : 'Create Bucket'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
