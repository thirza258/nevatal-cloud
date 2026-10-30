import React, { useEffect, useState } from 'react';
import {
  Key,
  ArrowLeft,
  Plus,
  Copy,
  Check,
  Trash2,
  ShieldAlert,
  Lock,
  Clock,
  AlertTriangle
} from 'lucide-react';
import { StorageAccessKey } from '../../types';

interface AccessKeysProps {
  onNavigate: (path: string, param?: string) => void;
}

export const AccessKeys: React.FC<AccessKeysProps> = ({ onNavigate }) => {
  const [keys, setKeys] = useState<StorageAccessKey[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // New key modal
  const [showModal, setShowModal] = useState(false);
  const [description, setDescription] = useState('');
  const [permissions, setPermissions] = useState<'read_write' | 'read_only' | 'admin'>('read_write');
  const [scope, setScope] = useState('*');
  const [createdKey, setCreatedKey] = useState<StorageAccessKey | null>(null);
  const [copiedId, setCopiedId] = useState(false);
  const [copiedSecret, setCopiedSecret] = useState(false);

  const fetchKeys = async () => {
    try {
      setLoading(true);
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/storage/access-keys', {
        headers: { Authorization: `Bearer ${token}` }
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to load access keys');
      setKeys(data.data || []);
    } catch (err: any) {
      setError(err.message);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchKeys();
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    try {
      const token = localStorage.getItem('token');
      const res = await fetch('/api/v1/storage/access-keys', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          description,
          permissions,
          bucket_scope: scope
        })
      });
      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to create access key');
      setCreatedKey(data.data);
      await fetchKeys();
    } catch (err: any) {
      alert(err.message);
    }
  };

  const handleRevoke = async (id: string) => {
    if (!confirm('Are you sure you want to revoke this S3 access key? Immediate effect.')) return;
    try {
      const token = localStorage.getItem('token');
      const res = await fetch(`/api/v1/storage/access-keys/${id}`, {
        method: 'DELETE',
        headers: { Authorization: `Bearer ${token}` }
      });
      if (!res.ok) throw new Error('Failed to revoke access key');
      await fetchKeys();
    } catch (err: any) {
      alert(err.message);
    }
  };

  return (
    <div className="space-y-6">
      {/* Top Bar Navigation */}
      <div className="flex items-center gap-3 text-xs text-slate-400">
        <button
          onClick={() => onNavigate('/dashboard/storage')}
          className="hover:text-white flex items-center gap-1 cursor-pointer"
        >
          <ArrowLeft className="w-3.5 h-3.5" /> S3 Storage
        </button>
        <span>/</span>
        <span className="text-slate-200 font-medium">S3 API Access Keys</span>
      </div>

      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-800 pb-5">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl font-bold tracking-tight text-white">S3 API Access Keys</h1>
            <span className="text-xs font-semibold px-2.5 py-0.5 rounded-full bg-cyan-500/10 text-cyan-400 border border-cyan-500/20">
              AWS Signature v4
            </span>
          </div>
          <p className="text-sm text-slate-400 mt-1">
            Generate and manage access keys for automated tools, backup pipelines, and S3 SDK clients.
          </p>
        </div>

        <button
          onClick={() => {
            setCreatedKey(null);
            setDescription('');
            setShowModal(true);
          }}
          className="inline-flex items-center gap-2 px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-cyan-500/20 cursor-pointer"
        >
          <Plus className="w-4 h-4" />
          Create Access Key
        </button>
      </div>

      {/* Keys Table */}
      <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl overflow-hidden">
        <table className="w-full text-left text-xs">
          <thead className="bg-slate-950/60 text-slate-400 uppercase tracking-wider border-b border-slate-800 font-mono">
            <tr>
              <th className="py-3 px-4">Access Key ID</th>
              <th className="py-3 px-4">Description</th>
              <th className="py-3 px-4">Permissions</th>
              <th className="py-3 px-4">Scope</th>
              <th className="py-3 px-4">Status</th>
              <th className="py-3 px-4">Created At</th>
              <th className="py-3 px-4 text-right">Action</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-800/60 text-slate-300">
            {keys.length === 0 ? (
              <tr>
                <td colSpan={7} className="py-8 text-center text-slate-500">
                  No access keys found. Generate a key above to access your buckets via S3 API.
                </td>
              </tr>
            ) : (
              keys.map((k) => (
                <tr key={k.id} className="hover:bg-slate-800/30 transition">
                  <td className="py-3 px-4 font-mono font-bold text-cyan-400">{k.access_key_id}</td>
                  <td className="py-3 px-4 text-slate-200">{k.description || '—'}</td>
                  <td className="py-3 px-4 capitalize font-mono text-[11px]">{k.permissions}</td>
                  <td className="py-3 px-4 font-mono text-slate-400">{k.bucket_scope}</td>
                  <td className="py-3 px-4">
                    <span
                      className={`px-2 py-0.5 rounded text-[10px] font-medium uppercase border ${
                        k.status === 'active'
                          ? 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20'
                          : 'bg-red-500/10 text-red-400 border-red-500/20'
                      }`}
                    >
                      {k.status}
                    </span>
                  </td>
                  <td className="py-3 px-4 text-slate-500">{new Date(k.created_at).toLocaleDateString()}</td>
                  <td className="py-3 px-4 text-right">
                    {k.status === 'active' && (
                      <button
                        onClick={() => handleRevoke(k.id)}
                        className="p-1 rounded text-slate-400 hover:text-red-400 hover:bg-slate-800 transition cursor-pointer"
                        title="Revoke Key"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    )}
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Modal */}
      {showModal && (
        <div className="fixed inset-0 z-50 bg-slate-950/80 backdrop-blur-sm flex items-center justify-center p-4">
          <div className="bg-slate-900 border border-slate-800 rounded-xl p-6 max-w-md w-full space-y-4">
            <h2 className="text-base font-bold text-white tracking-tight flex items-center gap-2">
              <Key className="w-4 h-4 text-cyan-400" />
              {createdKey ? 'Save S3 Access Credentials' : 'Create S3 Access Key'}
            </h2>

            {createdKey ? (
              <div className="space-y-4">
                <div className="p-3 rounded-lg bg-amber-500/10 border border-amber-500/20 text-xs text-amber-300 flex items-start gap-2">
                  <ShieldAlert className="w-4 h-4 text-amber-400 flex-shrink-0 mt-0.5" />
                  <span>
                    Your <strong>Secret Access Key</strong> will only be displayed once. Copy and store it securely now.
                  </span>
                </div>

                <div>
                  <label className="block text-xs text-slate-400 mb-1">Access Key ID</label>
                  <div className="flex items-center gap-2 p-2 rounded-lg bg-slate-950 border border-slate-800 font-mono text-xs">
                    <span className="flex-1 text-white">{createdKey.access_key_id}</span>
                    <button
                      onClick={() => {
                        navigator.clipboard.writeText(createdKey.access_key_id);
                        setCopiedId(true);
                        setTimeout(() => setCopiedId(false), 2000);
                      }}
                      className="text-slate-400 hover:text-white"
                    >
                      {copiedId ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>

                <div>
                  <label className="block text-xs text-slate-400 mb-1">Secret Access Key</label>
                  <div className="flex items-center gap-2 p-2 rounded-lg bg-slate-950 border border-slate-800 font-mono text-xs">
                    <span className="flex-1 text-emerald-400 truncate">{createdKey.secret_access_key}</span>
                    <button
                      onClick={() => {
                        if (createdKey.secret_access_key) {
                          navigator.clipboard.writeText(createdKey.secret_access_key);
                          setCopiedSecret(true);
                          setTimeout(() => setCopiedSecret(false), 2000);
                        }
                      }}
                      className="text-slate-400 hover:text-white"
                    >
                      {copiedSecret ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                    </button>
                  </div>
                </div>

                <button
                  type="button"
                  onClick={() => setShowModal(false)}
                  className="w-full py-2 bg-slate-800 hover:bg-slate-700 text-white rounded-lg text-xs font-medium cursor-pointer"
                >
                  I have saved my secret key
                </button>
              </div>
            ) : (
              <form onSubmit={handleCreate} className="space-y-4">
                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1.5">Description</label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. Production Next.js Uploads or Backup Script"
                    value={description}
                    onChange={(e) => setDescription(e.target.value)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white focus:outline-none focus:border-cyan-500"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-slate-300 mb-1.5">Permissions</label>
                  <select
                    value={permissions}
                    onChange={(e) => setPermissions(e.target.value as any)}
                    className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white"
                  >
                    <option value="read_write">Read & Write (Upload & Download)</option>
                    <option value="read_only">Read Only (Download & List only)</option>
                    <option value="admin">Admin Full Control</option>
                  </select>
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
                    className="px-4 py-2 bg-cyan-600 hover:bg-cyan-500 text-white rounded-lg text-xs font-medium cursor-pointer"
                  >
                    Generate Credentials
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
