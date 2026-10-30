import React, { useEffect, useState, useRef } from 'react';
import {
  FolderArchive,
  ArrowLeft,
  Upload,
  FolderPlus,
  Folder,
  File,
  FileText,
  FileImage,
  FileCode,
  FileAudio,
  FileVideo,
  Download,
  Share2,
  Trash2,
  ExternalLink,
  Lock,
  Globe,
  Copy,
  Check,
  X,
  Eye,
  AlertTriangle
} from 'lucide-react';
import { StorageBucket, StorageObject, StorageSharedLink } from '../../types';

interface FileManagerProps {
  bucketId: string;
  onNavigate: (path: string, param?: string) => void;
}

export const FileManager: React.FC<FileManagerProps> = ({ bucketId, onNavigate }) => {
  const [bucket, setBucket] = useState<StorageBucket | null>(null);
  const [objects, setObjects] = useState<StorageObject[]>([]);
  const [currentPrefix, setCurrentPrefix] = useState<string>('');
  const [loading, setLoading] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Folder modal
  const [showFolderModal, setShowFolderModal] = useState(false);
  const [newFolderName, setNewFolderName] = useState('');

  // Preview modal
  const [previewObj, setPreviewObj] = useState<StorageObject | null>(null);

  // Share modal
  const [shareObj, setShareObj] = useState<StorageObject | null>(null);
  const [sharePassword, setSharePassword] = useState('');
  const [shareExpiry, setShareExpiry] = useState('24h');
  const [createdShare, setCreatedShare] = useState<StorageSharedLink | null>(null);
  const [copiedShare, setCopiedShare] = useState(false);

  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const fetchBucketAndObjects = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const [bRes, oRes] = await Promise.all([
        fetch(`/api/v1/storage/buckets/${bucketId}`, { headers: { Authorization: `Bearer ${token}` } }),
        fetch(`/api/v1/storage/buckets/${bucketId}/objects?prefix=${encodeURIComponent(currentPrefix)}`, {
          headers: { Authorization: `Bearer ${token}` }
        })
      ]);

      const bData = await bRes.json();
      const oData = await oRes.json();

      if (!bRes.ok) throw new Error(bData.error?.message || 'Failed to load bucket');
      setBucket(bData.data);
      setObjects(oData.data || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchBucketAndObjects();
  }, [bucketId, currentPrefix]);

  const handleFileUpload = async (files: FileList | null) => {
    if (!files || files.length === 0) return;
    try {
      setUploading(true);
      const token = localStorage.getItem('token');

      for (let i = 0; i < files.length; i++) {
        const file = files[i];
        const formData = new FormData();
        formData.append('file', file);
        if (currentPrefix) {
          formData.append('folder_prefix', currentPrefix);
        }

        const res = await fetch(`/api/v1/storage/buckets/${bucketId}/objects`, {
          method: 'POST',
          headers: { Authorization: `Bearer ${token}` },
          body: formData
        });
        if (!res.ok) throw new Error(`Upload failed for ${file.name}`);
      }

      await fetchBucketAndObjects();
    } catch (err: any) {
      alert(err.message);
    } finally {
      setUploading(false);
    }
  };

  const handleCreateFolder = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newFolderName.trim()) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/storage/buckets/${bucketId}/folders`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          folder_prefix: currentPrefix,
          folder_name: newFolderName.trim()
        })
      });
      if (!res.ok) throw new Error('Failed to create folder');
      setShowFolderModal(false);
      setNewFolderName('');
      await fetchBucketAndObjects();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleDeleteObject = async (obj: StorageObject) => {
    if (!confirm(`Are you sure you want to delete ${obj.filename}?`)) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/storage/objects/${obj.id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to delete object');
      await fetchBucketAndObjects();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleCreateShareLink = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!shareObj) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/storage/shared-links', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          object_id: shareObj.id,
          access_type: 'download',
          password: sharePassword,
          expires_in: shareExpiry
        })
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to create shared link');
      setCreatedShare(data.data);
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleDownload = (obj: StorageObject) => {
    const token = localStorage.getItem('token');
    window.open(`/api/v1/storage/objects/${obj.id}/download?token=${token}`, '_blank');
  };

  const formatBytes = (bytes: number) => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
  };

  const getFileIcon = (obj: StorageObject) => {
    if (obj.is_folder) return <Folder className="w-4 h-4 text-amber-400" />;
    const ct = obj.content_type.toLowerCase();
    if (ct.startsWith('image/')) return <FileImage className="w-4 h-4 text-emerald-400" />;
    if (ct.startsWith('audio/')) return <FileAudio className="w-4 h-4 text-purple-400" />;
    if (ct.startsWith('video/')) return <FileVideo className="w-4 h-4 text-rose-400" />;
    if (ct.includes('javascript') || ct.includes('json') || ct.includes('html') || ct.includes('css')) {
      return <FileCode className="w-4 h-4 text-cyan-400" />;
    }
    if (ct.includes('text') || ct.includes('pdf')) return <FileText className="w-4 h-4 text-blue-400" />;
    return <File className="w-4 h-4 text-slate-400" />;
  };

  const breadcrumbs = currentPrefix.split('/').filter(Boolean);

  return (
    <div className="space-y-6">
      {/* Top Bar Navigation */}
      <div className="flex items-center gap-3 text-xs text-slate-400">
        <button
          onClick={() => onNavigate('/dashboard/storage')}
          className="hover:text-white flex items-center gap-1 cursor-pointer"
        >
          <ArrowLeft className="w-3.5 h-3.5" /> Storage Buckets
        </button>
        <span>/</span>
        <button
          onClick={() => setCurrentPrefix('')}
          className="hover:text-white cursor-pointer font-mono"
        >
          {bucket?.name}
        </button>
        {breadcrumbs.map((crumb, idx) => {
          const subPath = breadcrumbs.slice(0, idx + 1).join('/');
          return (
            <React.Fragment key={subPath}>
              <span>/</span>
              <button
                onClick={() => setCurrentPrefix(subPath)}
                className="hover:text-white cursor-pointer font-mono"
              >
                {crumb}
              </button>
            </React.Fragment>
          );
        })}
      </div>

      {/* Hero Overview */}
      <div className="bg-slate-900/80 border border-slate-800/80 rounded-xl p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div className="flex items-center gap-3">
          <div className="w-10 h-10 rounded-lg bg-cyan-600/10 border border-cyan-500/20 flex items-center justify-center text-cyan-400">
            <FolderArchive className="w-5 h-5" />
          </div>
          <div>
            <h1 className="text-lg font-bold text-white tracking-tight font-mono">{bucket?.name}</h1>
            <div className="flex items-center gap-2 text-xs text-slate-400">
              <span>{bucket?.region}</span>
              <span>•</span>
              <span className="capitalize">{bucket?.visibility} bucket</span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => setShowFolderModal(true)}
            className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-200 rounded-lg text-xs font-medium transition flex items-center gap-1.5 cursor-pointer"
          >
            <FolderPlus className="w-3.5 h-3.5 text-amber-400" />
            New Folder
          </button>

          <input
            ref={fileInputRef}
            type="file"
            multiple
            className="hidden"
            onChange={(e) => handleFileUpload(e.target.files)}
          />
          <button
            onClick={() => fileInputRef.current?.click()}
            disabled={uploading}
            className="px-3.5 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-xs font-medium transition flex items-center gap-1.5 cursor-pointer shadow-sm disabled:opacity-50"
          >
            <Upload className="w-3.5 h-3.5" />
            {uploading ? 'Uploading...' : 'Upload Files'}
          </button>
        </div>
      </div>

      {/* Drag & Drop Upload Zone */}
      <div
        onDragOver={(e) => e.preventDefault()}
        onDrop={(e) => {
          e.preventDefault();
          handleFileUpload(e.dataTransfer.files);
        }}
        className="border-2 border-dashed border-slate-800/80 hover:border-cyan-500/40 rounded-xl p-6 text-center bg-slate-900/20 transition cursor-pointer select-none"
        onClick={() => fileInputRef.current?.click()}
      >
        <Upload className="w-8 h-8 text-slate-600 mx-auto mb-2" />
        <p className="text-xs font-medium text-slate-300">
          Drag & drop files here to upload, or <span className="text-cyan-400">browse</span>
        </p>
        <p className="text-[11px] text-slate-500 mt-1">Directly uploaded to s3://{bucket?.name}/{currentPrefix ? `${currentPrefix}/` : ''}</p>
      </div>

      {/* Object Explorer Table */}
      <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl overflow-hidden">
        <table className="w-full text-left text-xs">
          <thead className="bg-slate-950/60 text-slate-400 uppercase tracking-wider border-b border-slate-800 font-mono">
            <tr>
              <th className="py-3 px-4">Name</th>
              <th className="py-3 px-4">Size</th>
              <th className="py-3 px-4">MIME Type</th>
              <th className="py-3 px-4">Last Modified</th>
              <th className="py-3 px-4 text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60 text-slate-300">
            {currentPrefix && (
              <tr
                onClick={() => {
                  const parts = currentPrefix.split('/').filter(Boolean);
                  parts.pop();
                  setCurrentPrefix(parts.join('/'));
                }}
                className="hover:bg-slate-800/30 transition cursor-pointer"
              >
                <td colSpan={5} className="py-2.5 px-4 text-cyan-400 font-mono flex items-center gap-2">
                  <Folder className="w-4 h-4 text-amber-400" /> .. (Up one level)
                </td>
              </tr>
            )}

            {objects.length === 0 ? (
              <tr>
                <td colSpan={5} className="py-8 text-center text-slate-500">
                  This folder is empty. Upload files or create subfolders above.
                </td>
              </tr>
            ) : (
              objects.map((obj) => (
                <tr
                  key={obj.id}
                  onClick={() => {
                    if (obj.is_folder) {
                      const cleanName = obj.filename.replace(/\/$/, '');
                      setCurrentPrefix(currentPrefix ? `${currentPrefix}/${cleanName}` : cleanName);
                    }
                  }}
                  className={`hover:bg-slate-800/30 transition ${obj.is_folder ? 'cursor-pointer' : ''}`}
                >
                  <td className="py-3 px-4 font-medium text-white flex items-center gap-2.5">
                    {getFileIcon(obj)}
                    <span className="font-mono">{obj.filename}</span>
                  </td>
                  <td className="py-3 px-4 font-mono text-slate-400">
                    {obj.is_folder ? '—' : formatBytes(obj.size_bytes)}
                  </td>
                  <td className="py-3 px-4 font-mono text-slate-500 text-[11px] truncate max-w-xs">
                    {obj.content_type}
                  </td>
                  <td className="py-3 px-4 text-slate-500">
                    {new Date(obj.updated_at || obj.created_at).toLocaleString()}
                  </td>
                  <td className="py-3 px-4 text-right">
                    <div className="flex items-center justify-end gap-1.5" onClick={(e) => e.stopPropagation()}>
                      {!obj.is_folder && (
                        <>
                          <button
                            onClick={() => setPreviewObj(obj)}
                            className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-800 transition cursor-pointer"
                            title="Preview file"
                          >
                            <Eye className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => handleDownload(obj)}
                            className="p-1 rounded text-slate-400 hover:text-white hover:bg-slate-800 transition cursor-pointer"
                            title="Download file"
                          >
                            <Download className="w-4 h-4" />
                          </button>
                          <button
                            onClick={() => {
                              setShareObj(obj);
                              setCreatedShare(null);
                              setSharePassword('');
                            }}
                            className="p-1 rounded text-slate-400 hover:text-cyan-400 hover:bg-slate-800 transition cursor-pointer"
                            title="Share link"
                          >
                            <Share2 className="w-4 h-4" />
                          </button>
                        </>
                      )}
                      <button
                        onClick={() => handleDeleteObject(obj)}
                        className="p-1 rounded text-slate-400 hover:text-red-400 hover:bg-slate-800 transition cursor-pointer"
                        title="Delete"
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

      {/* Create Folder Modal */}
      {showFolderModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 max-w-md w-full space-y-4">
            <h2 className="text-base font-bold text-white tracking-tight">Create New Folder</h2>
            <form onSubmit={handleCreateFolder} className="space-y-4">
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">Folder Name</label>
                <input
                  type="text"
                  required
                  placeholder="e.g. captures or thumbnails"
                  value={newFolderName}
                  onChange={(e) => setNewFolderName(e.target.value)}
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-cyan-500 font-mono"
                />
              </div>
              <div className="flex items-center justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setShowFolderModal(false)}
                  className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs font-medium transition cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-xs font-medium transition cursor-pointer"
                >
                  Create Folder
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Share Link Modal */}
      {shareObj && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 max-w-md w-full space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-base font-bold text-white tracking-tight flex items-center gap-2">
                <Share2 className="w-4 h-4 text-cyan-400" /> Share File
              </h2>
              <button
                onClick={() => setShareObj(null)}
                className="p-1 text-slate-400 hover:text-white cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <p className="text-xs text-slate-400 font-mono truncate">
              {shareObj.filename} ({formatBytes(shareObj.size_bytes)})
            </p>

            {createdShare ? (
              <div className="space-y-3 bg-slate-950 p-4 rounded-xl border border-slate-800">
                <span className="text-xs text-emerald-400 font-medium flex items-center gap-1.5">
                  <Check className="w-4 h-4" /> Share Link Generated!
                </span>
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    readOnly
                    value={`${window.location.origin}${createdShare.share_url}`}
                    className="flex-1 px-3 py-1.5 bg-slate-900 border border-slate-800 rounded-lg text-xs text-slate-300 font-mono"
                  />
                  <button
                    onClick={() => {
                      navigator.clipboard.writeText(`${window.location.origin}${createdShare.share_url}`);
                      setCopiedShare(true);
                      setTimeout(() => setCopiedShare(false), 2000);
                    }}
                    className="px-3 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-xs font-medium transition cursor-pointer"
                  >
                    {copiedShare ? 'Copied' : 'Copy'}
                  </button>
                </div>
                {createdShare.has_password && (
                  <p className="text-[11px] text-amber-400">
                    Protected with password: {sharePassword}
                  </p>
                )}
              </div>
            ) : (
              <form onSubmit={handleCreateShareLink} className="space-y-3">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">
                    Password Protection (Optional)
                  </label>
                  <input
                    type="password"
                    placeholder="Leave empty for public link"
                    value={sharePassword}
                    onChange={(e) => setSharePassword(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1">Expiration</label>
                  <select
                    value={shareExpiry}
                    onChange={(e) => setShareExpiry(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="1h">1 Hour</option>
                    <option value="24h">24 Hours (1 Day)</option>
                    <option value="7d">7 Days</option>
                    <option value="30d">30 Days</option>
                  </select>
                </div>

                <div className="flex items-center justify-end gap-3 pt-2">
                  <button
                    type="button"
                    onClick={() => setShareObj(null)}
                    className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-xs font-medium cursor-pointer"
                  >
                    Generate Share Link
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}

      {/* Preview Modal */}
      {previewObj && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 max-w-2xl w-full space-y-4">
            <div className="flex items-center justify-between">
              <h2 className="text-base font-bold text-white tracking-tight truncate font-mono">
                {previewObj.filename}
              </h2>
              <button
                onClick={() => setPreviewObj(null)}
                className="p-1 text-slate-400 hover:text-white cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="p-4 rounded-xl bg-slate-950 border border-slate-800 flex items-center justify-center min-h-[240px]">
              {previewObj.content_type.startsWith('image/') ? (
                <img
                  src={`/api/v1/storage/objects/${previewObj.id}/download?token=${localStorage.getItem('token')}`}
                  alt={previewObj.filename}
                  className="max-h-96 rounded object-contain"
                />
              ) : previewObj.content_type.startsWith('audio/') ? (
                <audio
                  controls
                  src={`/api/v1/storage/objects/${previewObj.id}/download?token=${localStorage.getItem('token')}`}
                  className="w-full"
                />
              ) : previewObj.content_type.startsWith('video/') ? (
                <video
                  controls
                  src={`/api/v1/storage/objects/${previewObj.id}/download?token=${localStorage.getItem('token')}`}
                  className="max-h-96 rounded w-full"
                />
              ) : (
                <div className="text-center text-slate-500">
                  <File className="w-12 h-12 mx-auto mb-2 text-slate-600" />
                  <p className="text-xs">Preview not available for MIME type: {previewObj.content_type}</p>
                </div>
              )}
            </div>

            <div className="flex items-center justify-between text-xs text-slate-400 pt-2">
              <span>Size: {formatBytes(previewObj.size_bytes)}</span>
              <button
                onClick={() => handleDownload(previewObj)}
                className="px-3 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-xs font-medium flex items-center gap-1.5 cursor-pointer"
              >
                <Download className="w-3.5 h-3.5" /> Download
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
