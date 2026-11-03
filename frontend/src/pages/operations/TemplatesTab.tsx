import React, { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  ArrowUpRight,
  Check,
  Copy,
  FileCode2,
  Layers,
  Pencil,
  Plus,
  RefreshCw,
  Rocket,
  Search,
  Trash2,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { Modal } from '../../components/ui/Modal';
import { useAuth } from '../../context/AuthContext';
import type {
  DeployTemplateResult,
  InfraTemplate,
  PlanSpecification,
  SaveTemplateInput,
  TemplateParameter,
} from '../../types';
import { errorMessage, formatDate, statusClasses } from './utils';

interface TemplatesTabProps {
  onNavigate: (path: string, param?: string) => void;
}

const SAMPLE_SPEC = `{
  "intent": "template",
  "resources": [
    {
      "key": "app",
      "type": "app",
      "action": "create",
      "config": {
        "name": "{{app_name}}",
        "repository_url": "https://github.com/example/app"
      }
    }
  ]
}`;

const SAMPLE_PARAMS = `[
  {
    "key": "app_name",
    "label": "Application name",
    "type": "string",
    "required": true,
    "default": "demo-app"
  }
]`;

const emptyEditor = {
  name: '',
  description: '',
  isPublic: false,
  specText: SAMPLE_SPEC,
  paramsText: SAMPLE_PARAMS,
};

export const TemplatesTab: React.FC<TemplatesTabProps> = ({ onNavigate }) => {
  const { user } = useAuth();
  const [templates, setTemplates] = useState<InfraTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [search, setSearch] = useState('');

  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<InfraTemplate | null>(null);
  const [editor, setEditor] = useState({ ...emptyEditor });
  const [saving, setSaving] = useState(false);

  const [deploying, setDeploying] = useState<InfraTemplate | null>(null);
  const [deployValues, setDeployValues] = useState<Record<string, unknown>>({});
  const [deployLoading, setDeployLoading] = useState(false);

  const [deleting, setDeleting] = useState<InfraTemplate | null>(null);
  const [copied, setCopied] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiRequest<InfraTemplate[]>('/ai/templates');
      setTemplates(res || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const filtered = useMemo(() => {
    const term = search.trim().toLowerCase();
    if (!term) return templates;
    return templates.filter(
      (template) =>
        template.name.toLowerCase().includes(term) ||
        template.description.toLowerCase().includes(term) ||
        template.intent.toLowerCase().includes(term)
    );
  }, [templates, search]);

  const showNotice = (message: string) => {
    setNotice(message);
    window.setTimeout(() => setNotice(null), 4000);
  };

  const openCreate = () => {
    setEditing(null);
    setEditor({ ...emptyEditor });
    setEditorOpen(true);
  };

  const openEdit = (template: InfraTemplate) => {
    setEditing(template);
    setEditor({
      name: template.name,
      description: template.description,
      isPublic: template.is_public,
      specText: JSON.stringify(template.specification, null, 2),
      paramsText: JSON.stringify(template.parameters, null, 2),
    });
    setEditorOpen(true);
  };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      let specification: PlanSpecification;
      let parameters: TemplateParameter[];
      try {
        specification = JSON.parse(editor.specText);
      } catch {
        throw new Error('The specification is not valid JSON.');
      }
      try {
        parameters = JSON.parse(editor.paramsText);
      } catch {
        throw new Error('The parameters are not valid JSON.');
      }
      if (!specification || typeof specification !== 'object' || Array.isArray(specification)) {
        throw new Error('The specification must be a JSON object.');
      }
      if (!Array.isArray(parameters)) {
        throw new Error('Parameters must be a JSON array.');
      }
      const payload: SaveTemplateInput = {
        name: editor.name.trim(),
        description: editor.description.trim(),
        specification,
        parameters,
        is_public: editor.isPublic,
      };
      if (editing) {
        await apiRequest(`/ai/templates/${editing.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        });
        showNotice('Template updated. Its version was incremented.');
      } else {
        await apiRequest('/ai/templates', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        showNotice('Template created.');
      }
      setEditorOpen(false);
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const remove = async () => {
    if (!deleting) return;
    setError(null);
    try {
      await apiRequest(`/ai/templates/${deleting.id}`, { method: 'DELETE' });
      setDeleting(null);
      showNotice('Template deleted.');
      await load();
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const openDeploy = (template: InfraTemplate) => {
    const values: Record<string, unknown> = {};
    for (const parameter of template.parameters || []) {
      if (parameter.default !== undefined) values[parameter.key] = parameter.default;
      else if (parameter.type === 'boolean') values[parameter.key] = false;
      else values[parameter.key] = '';
    }
    setDeployValues(values);
    setDeploying(template);
  };

  const deploy = async () => {
    if (!deploying) return;
    setDeployLoading(true);
    setError(null);
    try {
      const parameters: Record<string, unknown> = {};
      for (const parameter of deploying.parameters || []) {
        const raw = deployValues[parameter.key];
        if (parameter.required && (raw === '' || raw === undefined || raw === null)) {
          throw new Error(`${parameter.label || parameter.key} is required.`);
        }
        if (raw === '' || raw === undefined || raw === null) continue;
        if (parameter.type === 'integer') {
          parameters[parameter.key] = Number(raw);
        } else if (parameter.type === 'boolean') {
          parameters[parameter.key] = Boolean(raw);
        } else {
          parameters[parameter.key] = raw;
        }
      }
      const res = await apiRequest<DeployTemplateResult>(`/ai/templates/${deploying.id}/deploy`, {
        method: 'POST',
        body: JSON.stringify({ parameters }),
      });
      setDeploying(null);
      onNavigate(
        `/dashboard/ai/conversations/${res.conversation_id}?plan=${res.plan_id}`,
        res.plan_id
      );
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setDeployLoading(false);
    }
  };

  const copySpec = async (template: InfraTemplate) => {
    try {
      await navigator.clipboard.writeText(JSON.stringify(template.specification, null, 2));
      setCopied(true);
      window.setTimeout(() => setCopied(false), 1500);
    } catch {
      setError('Clipboard access was denied.');
    }
  };

  const isOwner = (template: InfraTemplate) => user?.id === template.owner_id;

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-white">Infrastructure templates</h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Parameterized, secret-free specifications. Deploying a template creates a plan; it never executes infrastructure changes directly.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search className="w-3.5 h-3.5 text-zinc-500 absolute left-2.5 top-1/2 -translate-y-1/2" />
            <input
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Search templates…"
              className="bg-zinc-950 border border-zinc-800 rounded-md pl-8 pr-2.5 py-1.5 text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-zinc-600 w-48"
            />
          </div>
          <button
            onClick={load}
            disabled={loading}
            className="p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 transition"
            title="Refresh templates"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={openCreate}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold transition"
          >
            <Plus className="w-3.5 h-3.5" />
            New template
          </button>
        </div>
      </div>

      {error && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs">
          <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{error}</span>
        </div>
      )}
      {notice && (
        <div className="flex items-start gap-2 p-3 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-300 text-xs">
          <Check className="w-4 h-4 flex-shrink-0 mt-0.5" />
          <span>{notice}</span>
        </div>
      )}

      {loading ? (
        <div className="p-12 text-center text-zinc-500 flex flex-col items-center gap-2 text-xs">
          <RefreshCw className="w-5 h-5 animate-spin" />
          Loading templates…
        </div>
      ) : filtered.length === 0 ? (
        <div className="text-center py-14 px-4 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20">
          <FileCode2 className="w-10 h-10 text-zinc-600 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-zinc-200">No templates found</h3>
          <p className="text-xs text-zinc-400 max-w-md mx-auto mt-1 mb-5">
            Templates turn a proven resource graph into a reusable form. Public templates are readable by every account.
          </p>
          <button
            onClick={openCreate}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold"
          >
            <Plus className="w-3.5 h-3.5" />
            Create template
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
          {filtered.map((template) => (
            <div key={template.id} className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800 space-y-3">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <Layers className="w-4 h-4 text-violet-400 flex-shrink-0" />
                    <h3 className="text-sm font-semibold text-white truncate">{template.name}</h3>
                    <span className="text-[10px] font-mono text-zinc-500">v{template.version}</span>
                    <span className={`px-2 py-0.5 rounded-md border text-[10px] font-mono ${statusClasses(template.validation_status === 'valid' ? 'ok' : 'warning')}`}>
                      {template.validation_status}
                    </span>
                    {template.is_public && (
                      <span className="px-2 py-0.5 rounded-md bg-blue-500/10 text-blue-300 border border-blue-500/20 text-[10px] font-mono">
                        public
                      </span>
                    )}
                  </div>
                  <p className="text-xs text-zinc-400 mt-1 line-clamp-2">
                    {template.description || 'No description.'}
                  </p>
                </div>
                <button
                  onClick={() => copySpec(template)}
                  className="p-1.5 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-400 hover:text-white border border-zinc-800 flex-shrink-0"
                  title="Copy specification"
                >
                  {copied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                </button>
              </div>

              <div className="flex flex-wrap gap-1.5">
                {(template.specification?.resources || []).map((resource) => (
                  <span
                    key={resource.key}
                    className="px-2 py-0.5 rounded-md bg-zinc-950 border border-zinc-800 text-[10px] font-mono text-zinc-300"
                  >
                    {resource.key}
                    <span className="text-zinc-500"> · </span>
                    {resource.type}/{resource.action}
                  </span>
                ))}
                {(!template.specification?.resources || template.specification.resources.length === 0) && (
                  <span className="text-[10px] text-zinc-500">No resources in specification.</span>
                )}
              </div>

              <div className="flex items-center justify-between text-[10px] font-mono text-zinc-500">
                <span>
                  {(template.parameters || []).length} parameters · used {template.usage_count} times · intent {template.intent}
                </span>
                <span>{formatDate(template.updated_at)}</span>
              </div>

              <div className="pt-3 border-t border-zinc-850 flex items-center gap-1.5">
                <button
                  onClick={() => openDeploy(template)}
                  className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bg-violet-500/10 hover:bg-violet-500/20 text-violet-300 border border-violet-500/20 text-[10px] font-semibold transition"
                >
                  <Rocket className="w-3 h-3" /> Deploy
                </button>
                {isOwner(template) && (
                  <>
                    <button
                      onClick={() => openEdit(template)}
                      className="inline-flex items-center gap-1 px-2.5 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px] transition"
                    >
                      <Pencil className="w-3 h-3" /> Edit
                    </button>
                    <button
                      onClick={() => setDeleting(template)}
                      className="ml-auto inline-flex items-center gap-1 px-2.5 py-1 rounded-md bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/20 text-[10px] transition"
                    >
                      <Trash2 className="w-3 h-3" /> Delete
                    </button>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      <Modal
        isOpen={editorOpen}
        onClose={() => setEditorOpen(false)}
        title={editing ? `Edit template · ${editing.name}` : 'New infrastructure template'}
        description="Specifications are canonical JSON and may reference parameters as {{key}}. Secrets are rejected."
        maxWidth="3xl"
      >
        <form onSubmit={save} className="space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Name</label>
              <input
                value={editor.name}
                onChange={(e) => setEditor({ ...editor, name: e.target.value })}
                required
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              />
            </div>
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Description</label>
              <input
                value={editor.description}
                onChange={(e) => setEditor({ ...editor, description: e.target.value })}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              />
            </div>
          </div>
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Specification (JSON)</label>
              <textarea
                value={editor.specText}
                onChange={(e) => setEditor({ ...editor, specText: e.target.value })}
                rows={15}
                spellCheck={false}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-2 text-[11px] font-mono text-zinc-100 focus:outline-none focus:border-zinc-600"
              />
            </div>
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Parameters (JSON array)</label>
              <textarea
                value={editor.paramsText}
                onChange={(e) => setEditor({ ...editor, paramsText: e.target.value })}
                rows={15}
                spellCheck={false}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-2 text-[11px] font-mono text-zinc-100 focus:outline-none focus:border-zinc-600"
              />
            </div>
          </div>
          <label className="flex items-center gap-2 text-xs text-zinc-300">
            <input
              type="checkbox"
              checked={editor.isPublic}
              onChange={(e) => setEditor({ ...editor, isPublic: e.target.checked })}
              className="accent-white"
            />
            Publish this template to every account
          </label>
          <div className="flex justify-end gap-2 pt-1">
            <button
              type="button"
              onClick={() => setEditorOpen(false)}
              className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={saving}
              className="px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold disabled:opacity-50"
            >
              {saving ? 'Saving…' : editing ? 'Save template' : 'Create template'}
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        isOpen={!!deploying}
        onClose={() => setDeploying(null)}
        title={`Deploy ${deploying?.name || 'template'}`}
        description="The deployment becomes a plan in your AI workspace. Review and approve it there; nothing runs before approval."
        maxWidth="lg"
      >
        {deploying && (
          <div className="space-y-3">
            {(deploying.parameters || []).length === 0 ? (
              <p className="text-xs text-zinc-400">This template has no parameters.</p>
            ) : (
              (deploying.parameters || []).map((parameter) => (
                <div key={parameter.key}>
                  <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">
                    {parameter.label || parameter.key}
                    {parameter.required && <span className="text-rose-400"> *</span>}
                  </label>
                  {parameter.type === 'boolean' ? (
                    <label className="flex items-center gap-2 text-xs text-zinc-300">
                      <input
                        type="checkbox"
                        checked={Boolean(deployValues[parameter.key])}
                        onChange={(e) =>
                          setDeployValues({ ...deployValues, [parameter.key]: e.target.checked })
                        }
                        className="accent-white"
                      />
                      Enable
                    </label>
                  ) : parameter.type === 'enum' ? (
                    <select
                      value={String(deployValues[parameter.key] ?? '')}
                      onChange={(e) =>
                        setDeployValues({ ...deployValues, [parameter.key]: e.target.value })
                      }
                      className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
                    >
                      <option value="">Select…</option>
                      {(parameter.enum || []).map((value) => (
                        <option key={value} value={value}>
                          {value}
                        </option>
                      ))}
                    </select>
                  ) : (
                    <input
                      type={parameter.type === 'integer' ? 'number' : 'text'}
                      min={parameter.min}
                      max={parameter.max}
                      value={String(deployValues[parameter.key] ?? '')}
                      onChange={(e) =>
                        setDeployValues({ ...deployValues, [parameter.key]: e.target.value })
                      }
                      className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
                    />
                  )}
                  {parameter.description && (
                    <p className="text-[10px] text-zinc-500 mt-1">{parameter.description}</p>
                  )}
                </div>
              ))
            )}
            <div className="flex justify-end gap-2 pt-1">
              <button
                onClick={() => setDeploying(null)}
                className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs"
              >
                Cancel
              </button>
              <button
                onClick={deploy}
                disabled={deployLoading}
                className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold disabled:opacity-50"
              >
                <Rocket className="w-3.5 h-3.5" />
                {deployLoading ? 'Creating plan…' : 'Create plan'}
                <ArrowUpRight className="w-3 h-3" />
              </button>
            </div>
          </div>
        )}
      </Modal>

      <Modal
        isOpen={!!deleting}
        onClose={() => setDeleting(null)}
        title={`Delete ${deleting?.name || 'template'}?`}
        description="Plans already created from this template keep their own snapshot."
      >
        <div className="flex justify-end gap-2">
          <button
            onClick={() => setDeleting(null)}
            className="px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs"
          >
            Cancel
          </button>
          <button
            onClick={remove}
            className="px-3 py-1.5 rounded-md bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold"
          >
            Delete template
          </button>
        </div>
      </Modal>
    </div>
  );
};
