import React, { useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  Check,
  Pause,
  Play,
  Plus,
  RefreshCw,
  ShieldCheck,
  Trash2,
  Workflow,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { Modal } from '../../components/ui/Modal';
import type {
  AutomationPolicy,
  InfraTemplate,
  PolicyAction,
  PolicyTrigger,
  SavePolicyInput,
} from '../../types';
import { errorMessage, formatDate, formatRelative, statusClasses } from './utils';

interface PoliciesTabProps {
  onNavigate: (path: string, param?: string) => void;
}

const TRIGGERS: { value: PolicyTrigger; label: string; description: string }[] = [
  { value: 'execution.failed', label: 'Execution failed', description: 'An orchestrator execution failed or partially completed.' },
  { value: 'execution.completed', label: 'Execution completed', description: 'An orchestrator execution completed successfully.' },
  { value: 'check.failed', label: 'Check failed', description: 'A scheduled check reported warning or critical.' },
  { value: 'advisor.recommendation', label: 'Advisor recommendation', description: 'The advisor produced a new recommendation.' },
];

const ACTIONS: { value: PolicyAction; label: string; description: string }[] = [
  { value: 'notify', label: 'Notify me', description: 'Create an in-app notification. No infrastructure change.' },
  { value: 'create_plan', label: 'Propose a plan', description: 'Create an approval-bound plan from a template. Nothing executes automatically.' },
];

const CONDITION_KEYS = [
  { key: 'severity', label: 'Severity', placeholder: 'warning,critical' },
  { key: 'resource_type', label: 'Resource type', placeholder: 'vps,database' },
  { key: 'check_type', label: 'Check type', placeholder: 'database.health' },
  { key: 'category', label: 'Advisor category', placeholder: 'waste,capacity' },
  { key: 'action', label: 'Event action', placeholder: 'failed' },
];

interface EditorState {
  name: string;
  trigger: PolicyTrigger;
  action: PolicyAction;
  enabled: boolean;
  conditions: Record<string, string>;
  notifyTitle: string;
  notifyMessage: string;
  templateId: string;
  parametersText: string;
}

const emptyEditor: EditorState = {
  name: '',
  trigger: 'execution.failed',
  action: 'notify',
  enabled: true,
  conditions: {},
  notifyTitle: '',
  notifyMessage: '',
  templateId: '',
  parametersText: '{}',
};

export const PoliciesTab: React.FC<PoliciesTabProps> = ({ onNavigate }) => {
  const [policies, setPolicies] = useState<AutomationPolicy[]>([]);
  const [templates, setTemplates] = useState<InfraTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const [editorOpen, setEditorOpen] = useState(false);
  const [editing, setEditing] = useState<AutomationPolicy | null>(null);
  const [editor, setEditor] = useState<EditorState>({ ...emptyEditor });
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<AutomationPolicy | null>(null);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const [list, templateList] = await Promise.all([
        apiRequest<AutomationPolicy[]>('/ai/policies'),
        apiRequest<InfraTemplate[]>('/ai/templates').catch(() => []),
      ]);
      setPolicies(list || []);
      setTemplates(templateList || []);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  const showNotice = (message: string) => {
    setNotice(message);
    window.setTimeout(() => setNotice(null), 4000);
  };

  const activeCount = useMemo(() => policies.filter((policy) => policy.enabled).length, [policies]);

  const openCreate = () => {
    setEditing(null);
    setEditor({ ...emptyEditor, templateId: templates[0]?.id || '' });
    setEditorOpen(true);
  };

  const openEdit = (policy: AutomationPolicy) => {
    const config = policy.action_config || {};
    setEditing(policy);
    setEditor({
      name: policy.name,
      trigger: policy.trigger,
      action: policy.action,
      enabled: policy.enabled,
      conditions: { ...(policy.conditions || {}) },
      notifyTitle: typeof config.title === 'string' ? config.title : '',
      notifyMessage: typeof config.message === 'string' ? config.message : '',
      templateId: typeof config.template_id === 'string' ? config.template_id : templates[0]?.id || '',
      parametersText: JSON.stringify(config.parameters || {}, null, 2),
    });
    setEditorOpen(true);
  };

  const save = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const conditions: Record<string, string> = {};
      for (const [key, value] of Object.entries(editor.conditions)) {
        if (value.trim()) conditions[key] = value.trim();
      }
      let actionConfig: Record<string, unknown> = {};
      if (editor.action === 'notify') {
        if (editor.notifyTitle.trim()) actionConfig.title = editor.notifyTitle.trim();
        if (editor.notifyMessage.trim()) actionConfig.message = editor.notifyMessage.trim();
      } else {
        if (!editor.templateId) throw new Error('Choose a template for the plan proposal.');
        let parameters: Record<string, unknown> = {};
        try {
          parameters = JSON.parse(editor.parametersText || '{}');
        } catch {
          throw new Error('Plan parameters are not valid JSON.');
        }
        actionConfig = { template_id: editor.templateId, parameters };
      }
      const payload: SavePolicyInput = {
        name: editor.name.trim(),
        trigger: editor.trigger,
        action: editor.action,
        conditions,
        action_config: actionConfig,
        enabled: editor.enabled,
      };
      if (editing) {
        await apiRequest(`/ai/policies/${editing.id}`, {
          method: 'PATCH',
          body: JSON.stringify(payload),
        });
        showNotice('Policy updated.');
      } else {
        await apiRequest('/ai/policies', {
          method: 'POST',
          body: JSON.stringify(payload),
        });
        showNotice('Policy created.');
      }
      setEditorOpen(false);
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  const toggle = async (policy: AutomationPolicy) => {
    setBusy(`toggle:${policy.id}`);
    setError(null);
    try {
      const payload: SavePolicyInput = {
        name: policy.name,
        trigger: policy.trigger,
        action: policy.action,
        conditions: policy.conditions || {},
        action_config: policy.action_config || {},
        enabled: !policy.enabled,
      };
      await apiRequest(`/ai/policies/${policy.id}`, {
        method: 'PATCH',
        body: JSON.stringify(payload),
      });
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const remove = async () => {
    if (!deleting) return;
    setBusy(`delete:${deleting.id}`);
    setError(null);
    try {
      await apiRequest(`/ai/policies/${deleting.id}`, { method: 'DELETE' });
      setDeleting(null);
      showNotice('Policy deleted.');
      await load();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(null);
    }
  };

  const triggerLabel = (value: string) => TRIGGERS.find((item) => item.value === value)?.label || value;
  const actionLabel = (value: string) => ACTIONS.find((item) => item.value === value)?.label || value;

  return (
    <div className="space-y-4">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div>
          <h2 className="text-sm font-semibold text-white">Automation policies</h2>
          <p className="text-xs text-zinc-400 mt-0.5">
            Deterministic rules evaluated on platform events. Policies may notify or propose a plan; they can never execute infrastructure changes.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-mono text-zinc-500">
            {activeCount}/{policies.length} active
          </span>
          <button
            onClick={load}
            disabled={loading}
            className="p-2 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 transition"
            title="Refresh policies"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin' : ''}`} />
          </button>
          <button
            onClick={openCreate}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold transition"
          >
            <Plus className="w-3.5 h-3.5" />
            New policy
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
          Loading policies…
        </div>
      ) : policies.length === 0 ? (
        <div className="text-center py-14 px-4 rounded-xl border border-dashed border-zinc-800 bg-zinc-900/20">
          <ShieldCheck className="w-10 h-10 text-zinc-600 mx-auto mb-3" />
          <h3 className="text-sm font-semibold text-zinc-200">No automation policies</h3>
          <p className="text-xs text-zinc-400 max-w-md mx-auto mt-1 mb-5">
            Create a rule that notifies you on failures, or proposes a template plan when a check needs attention.
          </p>
          <button
            onClick={openCreate}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold"
          >
            <Plus className="w-3.5 h-3.5" />
            Create policy
          </button>
        </div>
      ) : (
        <div className="grid grid-cols-1 xl:grid-cols-2 gap-3">
          {policies.map((policy) => (
            <div key={policy.id} className="p-4 rounded-xl bg-zinc-900/40 border border-zinc-800 space-y-2.5">
              <div className="flex items-start justify-between gap-2">
                <div className="min-w-0">
                  <div className="flex items-center gap-2 flex-wrap">
                    <Workflow className="w-4 h-4 text-emerald-400 flex-shrink-0" />
                    <h3 className="text-sm font-semibold text-white truncate">{policy.name}</h3>
                    <span className={`px-2 py-0.5 rounded-md border text-[10px] font-mono ${statusClasses(policy.enabled ? 'ok' : 'stopped')}`}>
                      {policy.enabled ? 'active' : 'paused'}
                    </span>
                  </div>
                  <div className="text-[10px] text-zinc-500 font-mono mt-1">
                    {triggerLabel(policy.trigger)} → {actionLabel(policy.action)}
                  </div>
                </div>
                <button
                  onClick={() => toggle(policy)}
                  disabled={busy === `toggle:${policy.id}`}
                  className="p-1.5 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-400 hover:text-white border border-zinc-800 disabled:opacity-50"
                  title={policy.enabled ? 'Pause policy' : 'Resume policy'}
                >
                  {policy.enabled ? <Pause className="w-3.5 h-3.5" /> : <Play className="w-3.5 h-3.5" />}
                </button>
              </div>

              <div className="flex flex-wrap gap-1.5">
                {Object.entries(policy.conditions || {}).length === 0 ? (
                  <span className="text-[10px] text-zinc-500">No conditions (matches every event).</span>
                ) : (
                  Object.entries(policy.conditions || {}).map(([key, value]) => (
                    <span key={key} className="px-2 py-0.5 rounded-md bg-zinc-950 border border-zinc-800 text-[10px] font-mono text-zinc-300">
                      {key}={value}
                    </span>
                  ))
                )}
              </div>

              {policy.action === 'notify' && (
                <div className="text-[11px] text-zinc-400">
                  <span className="font-mono text-[10px] uppercase text-zinc-500">Notification: </span>
                  {typeof policy.action_config?.title === 'string' && policy.action_config.title
                    ? policy.action_config.title
                    : 'Policy name'}
                  {typeof policy.action_config?.message === 'string' && policy.action_config.message
                    ? ` — ${policy.action_config.message}`
                    : ''}
                </div>
              )}
              {policy.action === 'create_plan' && (
                <div className="text-[11px] text-zinc-400">
                  <span className="font-mono text-[10px] uppercase text-zinc-500">Template: </span>
                  {templates.find((template) => template.id === policy.action_config?.template_id)?.name ||
                    String(policy.action_config?.template_id || 'unknown')}
                </div>
              )}

              <div className="flex items-center justify-between text-[10px] font-mono text-zinc-500">
                <span>
                  triggered {policy.trigger_count} times · last {formatRelative(policy.last_triggered_at)}
                </span>
                <span>{formatDate(policy.updated_at)}</span>
              </div>

              <div className="pt-2.5 border-t border-zinc-850 flex items-center gap-1.5">
                <button
                  onClick={() => openEdit(policy)}
                  className="px-2.5 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px]"
                >
                  Edit
                </button>
                {policy.action === 'create_plan' && (
                  <button
                    onClick={() => onNavigate('/dashboard/operations?tab=advisor')}
                    className="px-2.5 py-1 rounded-md bg-zinc-850 hover:bg-zinc-800 text-zinc-300 border border-zinc-800 text-[10px]"
                  >
                    Inspect advisor
                  </button>
                )}
                <button
                  onClick={() => setDeleting(policy)}
                  className="ml-auto inline-flex items-center gap-1 px-2.5 py-1 rounded-md bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/20 text-[10px]"
                >
                  <Trash2 className="w-3 h-3" /> Delete
                </button>
              </div>
            </div>
          ))}
        </div>
      )}

      <Modal
        isOpen={editorOpen}
        onClose={() => setEditorOpen(false)}
        title={editing ? `Edit policy · ${editing.name}` : 'New automation policy'}
        description="Policies are evaluated after the triggering operation. They can notify or propose a plan; execution always requires approval."
        maxWidth="2xl"
      >
        <form onSubmit={save} className="space-y-3">
          <div>
            <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Name</label>
            <input
              value={editor.name}
              onChange={(e) => setEditor({ ...editor, name: e.target.value })}
              required
              className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
            />
          </div>
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Trigger</label>
              <select
                value={editor.trigger}
                onChange={(e) => setEditor({ ...editor, trigger: e.target.value })}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              >
                {TRIGGERS.map((trigger) => (
                  <option key={trigger.value} value={trigger.value}>
                    {trigger.label}
                  </option>
                ))}
              </select>
              <p className="text-[10px] text-zinc-500 mt-1">
                {TRIGGERS.find((trigger) => trigger.value === editor.trigger)?.description}
              </p>
            </div>
            <div>
              <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Action</label>
              <select
                value={editor.action}
                onChange={(e) => setEditor({ ...editor, action: e.target.value })}
                className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
              >
                {ACTIONS.map((action) => (
                  <option key={action.value} value={action.value}>
                    {action.label}
                  </option>
                ))}
              </select>
              <p className="text-[10px] text-zinc-500 mt-1">
                {ACTIONS.find((action) => action.value === editor.action)?.description}
              </p>
            </div>
          </div>

          <div>
            <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Conditions</label>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
              {CONDITION_KEYS.map((condition) => (
                <div key={condition.key}>
                  <div className="text-[10px] font-mono text-zinc-500 mb-0.5">{condition.label}</div>
                  <input
                    value={editor.conditions[condition.key] || ''}
                    onChange={(e) =>
                      setEditor({
                        ...editor,
                        conditions: { ...editor.conditions, [condition.key]: e.target.value },
                      })
                    }
                    placeholder={condition.placeholder}
                    className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 font-mono focus:outline-none focus:border-zinc-600"
                  />
                </div>
              ))}
            </div>
            <p className="text-[10px] text-zinc-500 mt-1">
              Comma-separated values match any. Leave a condition blank to ignore it.
            </p>
          </div>

          {editor.action === 'notify' ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Notification title</label>
                <input
                  value={editor.notifyTitle}
                  onChange={(e) => setEditor({ ...editor, notifyTitle: e.target.value })}
                  placeholder="Defaults to the policy name"
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
                />
              </div>
              <div>
                <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Notification message</label>
                <input
                  value={editor.notifyMessage}
                  onChange={(e) => setEditor({ ...editor, notifyMessage: e.target.value })}
                  placeholder="Defaults to the event message"
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
                />
              </div>
            </div>
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Template</label>
                <select
                  value={editor.templateId}
                  onChange={(e) => setEditor({ ...editor, templateId: e.target.value })}
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600"
                >
                  <option value="">Select a template…</option>
                  {templates.map((template) => (
                    <option key={template.id} value={template.id}>
                      {template.name} (v{template.version})
                    </option>
                  ))}
                </select>
                {templates.length === 0 && (
                  <p className="text-[10px] text-amber-400 mt-1">
                    Create a template first so the policy can propose a plan.
                  </p>
                )}
              </div>
              <div>
                <label className="block text-[11px] font-mono uppercase text-zinc-400 mb-1">Template parameters (JSON)</label>
                <textarea
                  value={editor.parametersText}
                  onChange={(e) => setEditor({ ...editor, parametersText: e.target.value })}
                  rows={5}
                  spellCheck={false}
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-md px-2.5 py-2 text-[11px] font-mono text-zinc-100 focus:outline-none focus:border-zinc-600"
                />
              </div>
            </div>
          )}

          <label className="flex items-center gap-2 text-xs text-zinc-300">
            <input
              type="checkbox"
              checked={editor.enabled}
              onChange={(e) => setEditor({ ...editor, enabled: e.target.checked })}
              className="accent-white"
            />
            Policy enabled
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
              {saving ? 'Saving…' : editing ? 'Save policy' : 'Create policy'}
            </button>
          </div>
        </form>
      </Modal>

      <Modal
        isOpen={!!deleting}
        onClose={() => setDeleting(null)}
        title={`Delete ${deleting?.name || 'policy'}?`}
        description="Existing notifications and plans created by the policy are kept."
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
            disabled={busy === `delete:${deleting?.id}`}
            className="px-3 py-1.5 rounded-md bg-rose-600 hover:bg-rose-500 text-white text-xs font-semibold disabled:opacity-50"
          >
            Delete policy
          </button>
        </div>
      </Modal>
    </div>
  );
};
