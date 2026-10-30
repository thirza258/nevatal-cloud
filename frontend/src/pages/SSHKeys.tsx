import React, { useState, useEffect } from 'react';
import { SSHKey } from '../types';
import { apiRequest } from '../lib/api';
import { Modal } from '../components/ui/Modal';
import { Key, Plus, Trash2, Edit2, ShieldAlert, Check, Copy } from 'lucide-react';

export const SSHKeys: React.FC = () => {
  const [keys, setKeys] = useState<SSHKey[]>([]);
  const [loading, setLoading] = useState(true);

  // Add modal
  const [showAddModal, setShowAddModal] = useState(false);
  const [name, setName] = useState('');
  const [publicKey, setPublicKey] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  // Rename modal
  const [renameTarget, setRenameTarget] = useState<SSHKey | null>(null);
  const [newName, setNewName] = useState('');

  const fetchKeys = async () => {
    try {
      const data = await apiRequest<SSHKey[]>('/ssh-keys');
      setKeys(data || []);
    } catch (err) {
      console.error('Failed to fetch SSH keys:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchKeys();
  }, []);

  const handleCreate = async () => {
    setError(null);
    setSaving(true);
    try {
      await apiRequest('/ssh-keys', {
        method: 'POST',
        body: JSON.stringify({ name, public_key: publicKey }),
      });
      setShowAddModal(false);
      setName('');
      setPublicKey('');
      await fetchKeys();
    } catch (err: any) {
      setError(err.message || 'Failed to save key');
    } finally {
      setSaving(false);
    }
  };

  const handleRename = async () => {
    if (!renameTarget || !newName.trim()) return;
    try {
      await apiRequest(`/ssh-keys/${renameTarget.id}`, {
        method: 'PATCH',
        body: JSON.stringify({ name: newName.trim() }),
      });
      setRenameTarget(null);
      await fetchKeys();
    } catch (err: any) {
      alert(err.message || 'Failed to rename key');
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Are you sure you want to delete this SSH key from your account? This will not affect existing running instances.')) {
      return;
    }
    try {
      await apiRequest(`/ssh-keys/${id}`, { method: 'DELETE' });
      await fetchKeys();
    } catch (err: any) {
      alert(err.message || 'Failed to delete key');
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">SSH Public Keys</h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-0.5">
            Manage public keys injected via Cloud-Init into your virtual servers
          </p>
        </div>
        <button
          onClick={() => setShowAddModal(true)}
          className="flex items-center gap-2 px-4 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-sm shadow transition"
        >
          <Plus className="w-4 h-4" />
          <span>Add SSH Key</span>
        </button>
      </div>

      <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
        {loading ? (
          <div className="p-12 text-center text-slate-400 text-xs">Loading keys...</div>
        ) : keys.length === 0 ? (
          <div className="p-12 text-center text-slate-400 space-y-3">
            <Key className="w-10 h-10 text-slate-600 mx-auto" />
            <h3 className="text-sm font-semibold text-white">No SSH Keys Registered</h3>
            <p className="text-xs text-slate-500 max-w-sm mx-auto">
              Add an SSH public key so you can securely log into Linux VMs without passwords.
            </p>
            <button
              onClick={() => setShowAddModal(true)}
              className="px-3 py-1.5 rounded-xl bg-blue-600 text-white text-xs font-semibold hover:bg-blue-500"
            >
              Add First Key
            </button>
          </div>
        ) : (
          <div className="divide-y divide-slate-800/80">
            {keys.map((k) => (
              <div
                key={k.id}
                className="p-5 flex flex-col sm:flex-row sm:items-center justify-between gap-4 hover:bg-slate-800/30 transition"
              >
                <div className="flex items-start gap-3.5">
                  <div className="w-9 h-9 rounded-xl bg-slate-800 flex items-center justify-center text-blue-400 mt-0.5">
                    <Key className="w-4 h-4" />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <span className="font-semibold text-white text-sm">{k.name}</span>
                    </div>
                    <div className="text-xs font-mono text-slate-400 mt-1 select-all">
                      {k.fingerprint}
                    </div>
                    <div className="text-[11px] text-slate-500 mt-0.5">
                      Added on {new Date(k.created_at).toLocaleDateString()}
                    </div>
                  </div>
                </div>

                <div className="flex items-center gap-2 self-end sm:self-center">
                  <button
                    onClick={() => {
                      setRenameTarget(k);
                      setNewName(k.name);
                    }}
                    className="p-2 rounded-lg bg-slate-800 text-slate-400 hover:text-white transition"
                    title="Rename"
                  >
                    <Edit2 className="w-3.5 h-3.5" />
                  </button>
                  <button
                    onClick={() => handleDelete(k.id)}
                    className="p-2 rounded-lg bg-slate-800 text-slate-400 hover:text-rose-400 transition"
                    title="Delete"
                  >
                    <Trash2 className="w-3.5 h-3.5" />
                  </button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Add Key Modal */}
      <Modal
        isOpen={showAddModal}
        onClose={() => setShowAddModal(false)}
        title="Add SSH Public Key"
        description="Paste your OpenSSH public key. Private keys are rejected."
      >
        <div className="space-y-4">
          {error && (
            <div className="p-3 rounded-xl bg-rose-950/60 border border-rose-800 text-rose-300 text-xs">
              {error}
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">Key Name</label>
            <input
              type="text"
              placeholder="e.g. macbook-air"
              value={name}
              onChange={(e) => setName(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Public Key Content (.pub)
            </label>
            <textarea
              rows={4}
              placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA..."
              value={publicKey}
              onChange={(e) => setPublicKey(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-600 font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              onClick={() => setShowAddModal(false)}
              className="px-4 py-2 rounded-xl bg-slate-800 text-slate-300 text-xs font-semibold hover:bg-slate-700"
            >
              Cancel
            </button>
            <button
              onClick={handleCreate}
              disabled={saving || !name.trim() || !publicKey.trim()}
              className="px-4 py-2 rounded-xl bg-blue-600 text-white text-xs font-semibold hover:bg-blue-500 disabled:opacity-50"
            >
              {saving ? 'Validating...' : 'Add Key'}
            </button>
          </div>
        </div>
      </Modal>

      {/* Rename Modal */}
      <Modal
        isOpen={renameTarget !== null}
        onClose={() => setRenameTarget(null)}
        title="Rename SSH Key"
      >
        <div className="space-y-4">
          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">New Key Name</label>
            <input
              type="text"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white font-mono"
            />
          </div>
          <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              onClick={() => setRenameTarget(null)}
              className="px-4 py-2 rounded-xl bg-slate-800 text-slate-300 text-xs font-semibold"
            >
              Cancel
            </button>
            <button
              onClick={handleRename}
              className="px-4 py-2 rounded-xl bg-blue-600 text-white text-xs font-semibold hover:bg-blue-500"
            >
              Save Name
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
