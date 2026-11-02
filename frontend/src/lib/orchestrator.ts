/**
 * AI Infrastructure Orchestrator (Phase 1) client helpers.
 *
 * Contract: docs/ORCHESTRATOR_API.md (frozen). Every response is normalized defensively
 * (Go nil slices arrive as `null`, optional objects may be missing) so the workspace never
 * crashes on a partially filled object.
 */
import { ApiError, apiRequest } from './api';
import { AI_CONVERSATIONS_PATH, isUuid } from './aiRoutes';
import type {
  AIResourcePlan,
  ApprovePlanResult,
  ConfirmationRequirement,
  Execution,
  ExecutionEvent,
  ExecutionReport,
  ExecutionSummary,
  ExecutionTask,
  OSTemplate,
  OrchestratorPlan,
  OrchestratorTool,
  PlanApproval,
  PlanRevision,
  PlanRevisionSummary,
  PlanSpecResource,
  PlanSpecification,
  PlanTask,
  PlanValidation,
  PlanValidationIssue,
  RecoveryAction,
  RecoveryOption,
  ResourcePlan,
  RiskLevel,
  TaskVerification,
} from '../types';

const API_BASE = '/api/v1';
const enc = encodeURIComponent;

// ---------------------------------------------------------------------------------------------
// Coercion helpers
// ---------------------------------------------------------------------------------------------

const asRecord = (v: unknown): Record<string, unknown> =>
  v && typeof v === 'object' && !Array.isArray(v) ? (v as Record<string, unknown>) : {};
const asArray = (v: unknown): unknown[] => (Array.isArray(v) ? v : []);
const str = (v: unknown, fallback = ''): string => (typeof v === 'string' ? v : fallback);
const strOrNull = (v: unknown): string | null => (typeof v === 'string' && v !== '' ? v : null);
const num = (v: unknown, fallback = 0): number => (typeof v === 'number' && Number.isFinite(v) ? v : fallback);
const strings = (v: unknown): string[] => asArray(v).filter((x): x is string => typeof x === 'string');
const isObject = (v: unknown): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v);

export const isAbortError = (err: unknown): boolean => (err as { name?: string } | null)?.name === 'AbortError';

// ---------------------------------------------------------------------------------------------
// IDs, deep links (/dashboard/ai/conversations/<conversationId>?plan=<planId>)
// ---------------------------------------------------------------------------------------------

export { AI_CONVERSATIONS_PATH, AI_WORKSPACE_PATH, conversationIdFromPath, isAIWorkspacePath, isUuid } from './aiRoutes';

export const conversationPath = (conversationId: string): string => `${AI_CONVERSATIONS_PATH}/${enc(conversationId)}`;

/** Deep link to a plan inside its conversation (the link MCP-created plans use). */
export const planDeepLink = (conversationId: string, planId: string): string =>
  `${conversationPath(conversationId)}?plan=${enc(planId)}`;

/** Reads `?plan=<uuid>`; App routes on the pathname only, so the workspace owns its query string. */
export function readPlanParam(): { id: string | null; invalid: boolean } {
  const raw = new URLSearchParams(window.location.search).get('plan');
  if (!raw) return { id: null, invalid: false };
  return isUuid(raw) ? { id: raw.toLowerCase(), invalid: false } : { id: null, invalid: true };
}

export function writePlanParam(id: string | null): void {
  const params = new URLSearchParams(window.location.search);
  if (id) params.set('plan', id);
  else params.delete('plan');
  const query = params.toString();
  const next = `${window.location.pathname}${query ? `?${query}` : ''}`;
  if (next !== window.location.pathname + window.location.search) {
    window.history.replaceState(window.history.state, '', next);
  }
}

export const sameId = (a: string | null | undefined, b: string | null | undefined): boolean =>
  !!a && !!b && a.toLowerCase() === b.toLowerCase();

// ---------------------------------------------------------------------------------------------
// Risk levels
// ---------------------------------------------------------------------------------------------

export const RISK_LEVELS: RiskLevel[] = ['read', 'low', 'medium', 'high', 'destructive'];
export const riskRank = (risk: RiskLevel): number => RISK_LEVELS.indexOf(risk);

export function normalizeRisk(value: unknown, fallback: RiskLevel = 'medium'): RiskLevel {
  const s = str(value).toLowerCase();
  return (RISK_LEVELS as string[]).includes(s) ? (s as RiskLevel) : fallback;
}

export const maxRisk = (risks: RiskLevel[]): RiskLevel =>
  risks.reduce<RiskLevel>((acc, r) => (riskRank(r) > riskRank(acc) ? r : acc), 'read');

export interface RiskMeta {
  label: string;
  badge: string;
  text: string;
  dot: string;
  panel: string;
  /** What approving a plan at this level requires (contract §1). */
  approval: string;
  /** Approval lifetime in minutes, 0 when no approval is needed. */
  expiryMinutes: number;
}

export const RISK_META: Record<RiskLevel, RiskMeta> = {
  read: {
    label: 'Read',
    badge: 'bg-zinc-800/60 text-zinc-300 border-zinc-700/60',
    text: 'text-zinc-300',
    dot: 'bg-zinc-400',
    panel: 'bg-zinc-900/60 border-zinc-800',
    approval: 'No approval needed',
    expiryMinutes: 0,
  },
  low: {
    label: 'Low',
    badge: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
    text: 'text-emerald-300',
    dot: 'bg-emerald-400',
    panel: 'bg-emerald-500/5 border-emerald-500/20',
    approval: 'No approval needed',
    expiryMinutes: 0,
  },
  medium: {
    label: 'Medium',
    badge: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
    text: 'text-amber-300',
    dot: 'bg-amber-400',
    panel: 'bg-amber-500/5 border-amber-500/20',
    approval: 'Approval bound to this revision',
    expiryMinutes: 60,
  },
  high: {
    label: 'High',
    badge: 'bg-orange-500/10 text-orange-400 border-orange-500/30',
    text: 'text-orange-300',
    dot: 'bg-orange-400',
    panel: 'bg-orange-500/5 border-orange-500/25',
    approval: 'Approval + risk acknowledgement',
    expiryMinutes: 60,
  },
  destructive: {
    label: 'Destructive',
    badge: 'bg-rose-500/10 text-rose-400 border-rose-500/30',
    text: 'text-rose-300',
    dot: 'bg-rose-500',
    panel: 'bg-rose-500/5 border-rose-500/30',
    approval: 'Approval + risk acknowledgement + typed confirmation',
    expiryMinutes: 15,
  },
};

const TOOL_CONSEQUENCES: Record<string, string> = {
  'vps.create': 'Creates a VPS. It counts against your vCPU, memory, disk and instance quota until you delete it.',
  'vps.start': 'Starts the VPS. Services on it become reachable again.',
  'vps.restart': 'Restarts the VPS. Running services are interrupted for a short time.',
  'vps.snapshot.create': 'Creates a snapshot of the VPS disk. The snapshot uses storage.',
  'bucket.create': 'Creates an object storage bucket.',
  'app.create': 'Deploys an application on new infrastructure.',
  'n8n.create': 'Creates an n8n automation instance.',
  'openclaw.create': 'Creates an OpenClaw agent instance.',
  'game_server.create': 'Creates a game server.',
  'android.create': 'Creates an Android cloud instance.',
  'vps.stop': 'Stops the VPS. Everything running on it is unreachable until it is started again.',
  'vps.resize': 'Resizes the VPS. It can restart, and running services are interrupted.',
  'vps.delete': 'Permanently deletes the VPS and its disks. This cannot be undone.',
  'vps.snapshot.restore': 'Replaces the VPS disk with the snapshot. Changes made after the snapshot are lost.',
  'bucket.delete': 'Permanently deletes the bucket and every object in it. This cannot be undone.',
};

const RISK_CONSEQUENCE: Record<RiskLevel, string> = {
  read: 'Reads data only. Nothing is changed.',
  low: 'Inspects or plans only. Nothing is changed.',
  medium: 'Creates or starts resources.',
  high: 'Changes running resources. Services can be interrupted.',
  destructive: 'Deletes or overwrites data. This cannot be undone.',
};

/** One consequence line per distinct tool at medium risk or above (falls back to the risk text). */
export function consequencesFor(tasks: PlanTask[]): { tool: string; risk: RiskLevel; text: string }[] {
  const seen = new Set<string>();
  const out: { tool: string; risk: RiskLevel; text: string }[] = [];
  for (const t of tasks) {
    if (riskRank(t.risk) < riskRank('medium') || seen.has(t.tool)) continue;
    seen.add(t.tool);
    out.push({ tool: t.tool, risk: t.risk, text: TOOL_CONSEQUENCES[t.tool] ?? RISK_CONSEQUENCE[t.risk] });
  }
  return out.sort((a, b) => riskRank(b.risk) - riskRank(a.risk));
}

export const RECOVERY_NOTES: Record<RiskLevel, string> = {
  read: 'Nothing is changed, so there is nothing to recover.',
  low: 'Nothing is changed, so there is nothing to recover.',
  medium:
    'Nothing is rolled back automatically. If a task fails you can retry the failed tasks, keep what was created, or create a separate plan that deletes it (that plan needs its own approval).',
  high:
    'Stopped or resized resources are not changed back automatically. You can retry a failed task; reverting the change needs a new plan.',
  destructive:
    'Deleted data cannot be recovered by Nevatal unless a snapshot or backup exists. Make sure you have one before you approve.',
};

// ---------------------------------------------------------------------------------------------
// Status metadata
// ---------------------------------------------------------------------------------------------

export type Tone = 'emerald' | 'amber' | 'rose' | 'sky' | 'orange' | 'zinc';

export const TONE_BADGE: Record<Tone, string> = {
  emerald: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
  amber: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  rose: 'bg-rose-500/10 text-rose-400 border-rose-500/20',
  sky: 'bg-sky-500/10 text-sky-400 border-sky-500/20',
  orange: 'bg-orange-500/10 text-orange-400 border-orange-500/20',
  zinc: 'bg-zinc-800/60 text-zinc-300 border-zinc-700/60',
};

export const TONE_DOT: Record<Tone, string> = {
  emerald: 'bg-emerald-500',
  amber: 'bg-amber-400',
  rose: 'bg-rose-400',
  sky: 'bg-sky-400',
  orange: 'bg-orange-400',
  zinc: 'bg-zinc-500',
};

export const isAwaitingApproval = (status: string): boolean =>
  status === 'awaiting_approval' || status === 'awaiting_confirmation';

/**
 * PATCH is accepted for `draft`, `awaiting_approval` and `approved` (not yet executing) plans; saving
 * an approved plan supersedes its approval (contract §3/§7). PLAN_NOT_EDITABLE covers the rest.
 */
export const isEditablePlan = (status: string): boolean =>
  status === 'draft' || status === 'approved' || isAwaitingApproval(status);

export function planStatusTone(status: string): Tone {
  switch (status) {
    case 'completed':
      return 'emerald';
    case 'approved':
    case 'executing':
    case 'verifying':
    case 'validating':
      return 'sky';
    case 'awaiting_approval':
    case 'awaiting_confirmation':
    case 'partially_completed':
      return 'amber';
    case 'failed':
    case 'rejected':
      return 'rose';
    default:
      return 'zinc';
  }
}

export const planStatusLabel = (status: string): string =>
  isAwaitingApproval(status) ? 'awaiting approval' : (status || 'unknown').replace(/_/g, ' ');

const ACTIVE_EXECUTION = new Set(['queued', 'running', 'verifying', 'cancelling']);
export const isActiveExecution = (status: string): boolean => ACTIVE_EXECUTION.has(status);

export function executionStatusTone(status: string): Tone {
  switch (status) {
    case 'completed':
      return 'emerald';
    case 'running':
    case 'verifying':
      return 'sky';
    case 'queued':
    case 'cancelling':
    case 'partially_completed':
      return 'amber';
    case 'failed':
      return 'rose';
    default:
      return 'zinc';
  }
}

export const TASK_STATUS_LABEL: Record<string, string> = {
  queued: 'Queued',
  blocked: 'Waiting on dependencies',
  running: 'Running',
  retrying: 'Retrying',
  succeeded: 'Succeeded',
  failed: 'Failed',
  cancelled: 'Cancelled',
  skipped: 'Skipped (a dependency failed)',
  compensating: 'Compensating',
};

export function approvalStatusTone(status: string): Tone {
  switch (status) {
    case 'approved':
      return 'emerald';
    case 'consumed':
      return 'sky';
    case 'superseded':
    case 'expired':
      return 'amber';
    case 'rejected':
      return 'rose';
    default:
      return 'zinc';
  }
}

// ---------------------------------------------------------------------------------------------
// Normalizers
// ---------------------------------------------------------------------------------------------

function normalizeIssue(raw: unknown): PlanValidationIssue | null {
  if (typeof raw === 'string') return { code: '', message: raw };
  const r = asRecord(raw);
  const message = str(r.message) || str(r.code);
  if (!message) return null;
  return { code: str(r.code), message, task_key: strOrNull(r.task_key) ?? undefined };
}

const issues = (v: unknown): PlanValidationIssue[] =>
  asArray(v).map(normalizeIssue).filter((x): x is PlanValidationIssue => x !== null);

export function normalizeValidation(raw: unknown): PlanValidation | null {
  if (!isObject(raw)) return null;
  const errors = issues(raw.errors);
  const warnings = issues(raw.warnings);
  const fallback = errors.length ? 'failed' : warnings.length ? 'warnings' : 'passed';
  return { status: str(raw.status, fallback) || fallback, checked_at: str(raw.checked_at) || undefined, errors, warnings };
}

function normalizePlanTask(raw: unknown): PlanTask | null {
  const r = asRecord(raw);
  const key = str(r.key);
  if (!key) return null;
  return {
    key,
    tool: str(r.tool),
    title: str(r.title) || key,
    depends_on: strings(r.depends_on),
    risk: normalizeRisk(r.risk),
    input: isObject(r.input) ? r.input : undefined,
  };
}

function normalizeConfirmation(raw: unknown): ConfirmationRequirement | null {
  if (!isObject(raw)) return null;
  const expected = strings(raw.expected);
  if (expected.length === 0) return null;
  return { type: str(raw.type, 'typed_name'), expected };
}

export function normalizeRevision(raw: unknown): PlanRevision | null {
  if (!isObject(raw)) return null;
  const tasks = asArray(raw.tasks).map(normalizePlanTask).filter((t): t is PlanTask => t !== null);
  const risk = raw.risk_level ? normalizeRisk(raw.risk_level) : maxRisk(tasks.map((t) => t.risk));
  return {
    version: num(raw.version, 1),
    spec_hash: str(raw.spec_hash),
    created_at: str(raw.created_at),
    created_by: str(raw.created_by, 'planner'),
    summary: str(raw.summary),
    // Kept as sent (unknown fields included) so an edit round-trips everything it did not touch.
    specification: (isObject(raw.specification) ? raw.specification : {}) as PlanSpecification,
    tasks,
    estimated_resources: asRecord(raw.estimated_resources),
    risk_level: risk,
    requires_approval:
      typeof raw.requires_approval === 'boolean' ? raw.requires_approval : riskRank(risk) >= riskRank('medium'),
    confirmation_required: normalizeConfirmation(raw.confirmation_required),
    validation: normalizeValidation(raw.validation),
  };
}

function normalizeApproval(raw: unknown): PlanApproval | null {
  if (!isObject(raw) || !str(raw.id)) return null;
  return {
    id: str(raw.id),
    version: num(raw.version),
    spec_hash: str(raw.spec_hash),
    status: str(raw.status, 'approved'),
    risk_level: normalizeRisk(raw.risk_level),
    expires_at: str(raw.expires_at),
    created_at: str(raw.created_at),
  };
}

export function normalizePlan(raw: unknown): OrchestratorPlan | null {
  const p = asRecord(raw);
  const id = str(p.id);
  if (!id) return null;
  const revision = normalizeRevision(p.revision);
  return {
    id,
    conversation_id: str(p.conversation_id),
    user_id: str(p.user_id) || undefined,
    version: typeof p.version === 'number' ? p.version : undefined,
    intent: str(p.intent) || undefined,
    status: str(p.status, 'draft'),
    title: str(p.title) || revision?.summary || 'Infrastructure plan',
    description: str(p.description) || undefined,
    created_at: str(p.created_at) || undefined,
    updated_at: str(p.updated_at) || undefined,
    current_version: num(p.current_version, revision?.version ?? num(p.version, 1)),
    revision,
    approval: normalizeApproval(p.approval),
    latest_execution_id: strOrNull(p.latest_execution_id),
  };
}

function normalizeRevisionSummary(raw: unknown): PlanRevisionSummary | null {
  const r = asRecord(raw);
  if (typeof r.version !== 'number') return null;
  return {
    version: r.version,
    spec_hash: str(r.spec_hash),
    created_at: str(r.created_at),
    created_by: str(r.created_by, 'planner'),
    summary: str(r.summary),
    risk_level: normalizeRisk(r.risk_level),
  };
}

/** Resources of a specification, normalized for display (the raw object is kept for editing). */
export function specResources(spec: PlanSpecification | null | undefined): PlanSpecResource[] {
  return asArray(spec?.resources)
    .map((raw, i): PlanSpecResource | null => {
      if (!isObject(raw)) return null;
      const config = asRecord(raw.config);
      return {
        key: str(raw.key) || str(config.name) || `resource-${i + 1}`,
        type: str(raw.type, 'resource'),
        action: str(raw.action),
        config,
      };
    })
    .filter((r): r is PlanSpecResource => r !== null);
}

function normalizeVerification(raw: unknown): TaskVerification | null {
  if (!isObject(raw)) return null;
  return {
    status: str(raw.status, 'unverified'),
    checks: asArray(raw.checks).map((c) => {
      const r = asRecord(c);
      return { name: str(r.name, 'check'), status: str(r.status, 'skip'), detail: str(r.detail) || undefined };
    }),
  };
}

function normalizeExecTask(raw: unknown, index: number): ExecutionTask | null {
  const r = asRecord(raw);
  const key = str(r.key);
  if (!key && !str(r.id)) return null;
  const err = isObject(r.error) ? { code: str(r.error.code), message: str(r.error.message) || str(r.error.code) } : null;
  return {
    id: str(r.id) || key || `task-${index}`,
    key: key || str(r.id),
    tool: str(r.tool),
    title: str(r.title) || key,
    risk: normalizeRisk(r.risk),
    depends_on: strings(r.depends_on),
    status: str(r.status, 'queued'),
    attempt: num(r.attempt),
    max_attempts: num(r.max_attempts),
    next_attempt_at: strOrNull(r.next_attempt_at),
    started_at: strOrNull(r.started_at),
    finished_at: strOrNull(r.finished_at),
    output: isObject(r.output) ? r.output : null,
    error: err && (err.code || err.message) ? err : null,
    verification: normalizeVerification(r.verification),
  };
}

function normalizeReport(raw: unknown): ExecutionReport | null {
  if (!isObject(raw)) return null;
  return {
    summary: str(raw.summary),
    resources: asArray(raw.resources).map((x) => {
      const r = asRecord(x);
      const access: Record<string, string> = {};
      for (const [k, v] of Object.entries(asRecord(r.access))) {
        if (typeof v === 'string' && v) access[k] = v;
        else if (typeof v === 'number') access[k] = String(v);
      }
      return { type: str(r.type), id: str(r.id), name: str(r.name) || str(r.id), status: str(r.status), access };
    }),
    warnings: asArray(raw.warnings)
      .map((w) => (typeof w === 'string' ? w : str(asRecord(w).message)))
      .filter(Boolean),
    next_actions: asArray(raw.next_actions)
      .map((w) => (typeof w === 'string' ? w : str(asRecord(w).label) || str(asRecord(w).message)))
      .filter(Boolean),
  };
}

function normalizeRecoveryOption(raw: unknown): RecoveryOption | null {
  const r = asRecord(raw);
  const action = str(r.action);
  if (!action) return null;
  return { action, label: str(r.label) || action.replace(/_/g, ' '), requires_approval: r.requires_approval === true };
}

export function normalizeExecution(raw: unknown): Execution | null {
  const r = asRecord(raw);
  const id = str(r.id);
  if (!id) return null;
  const counts: Partial<Record<string, number>> = {};
  for (const [k, v] of Object.entries(asRecord(r.task_counts))) if (typeof v === 'number') counts[k] = v;
  return {
    id,
    plan_id: str(r.plan_id),
    plan_version: num(r.plan_version),
    conversation_id: strOrNull(r.conversation_id),
    status: str(r.status, 'queued'),
    cancel_requested: r.cancel_requested === true,
    created_at: str(r.created_at),
    started_at: strOrNull(r.started_at),
    finished_at: strOrNull(r.finished_at),
    tasks: asArray(r.tasks)
      .map(normalizeExecTask)
      .filter((t): t is ExecutionTask => t !== null),
    report: normalizeReport(r.report),
    recovery_options: asArray(r.recovery_options)
      .map(normalizeRecoveryOption)
      .filter((o): o is RecoveryOption => o !== null),
    task_counts: Object.keys(counts).length ? counts : undefined,
  };
}

export function normalizeEvent(raw: unknown, frameId?: string | null, frameType?: string): ExecutionEvent | null {
  const r = asRecord(raw);
  const seq = typeof r.seq === 'number' ? r.seq : Number(frameId);
  if (!Number.isFinite(seq)) return null;
  return {
    seq,
    id: str(r.id) || String(seq),
    type: str(r.type) || frameType || 'message',
    severity: str(r.severity, 'info'),
    message: str(r.message),
    task_key: strOrNull(r.task_key),
    metadata: asRecord(r.metadata),
    created_at: str(r.created_at),
  };
}

function normalizeTool(raw: unknown): OrchestratorTool | null {
  const r = asRecord(raw);
  const id = str(r.id);
  if (!id) return null;
  return {
    id,
    title: str(r.title) || id,
    description: str(r.description),
    risk: normalizeRisk(r.risk, 'read'),
    resource_types: strings(r.resource_types),
    idempotent: r.idempotent === true,
    timeout_seconds: num(r.timeout_seconds),
    requires_admin: r.requires_admin === true,
    input_schema: r.input_schema ?? null,
  };
}

// ---------------------------------------------------------------------------------------------
// API
// ---------------------------------------------------------------------------------------------

const badResponse = (what: string) => new ApiError(`The server returned an unreadable ${what}.`, 'BAD_RESPONSE');

export async function fetchPlan(id: string, signal?: AbortSignal): Promise<OrchestratorPlan> {
  const plan = normalizePlan(await apiRequest<unknown>(`/ai/plans/${enc(id)}`, { signal }));
  if (!plan) throw badResponse('plan');
  return plan;
}

export async function fetchPlanRevisions(id: string, signal?: AbortSignal): Promise<PlanRevisionSummary[]> {
  const raw = await apiRequest<unknown>(`/ai/plans/${enc(id)}/revisions`, { signal });
  return asArray(raw)
    .map(normalizeRevisionSummary)
    .filter((r): r is PlanRevisionSummary => r !== null)
    .sort((a, b) => b.version - a.version);
}

export async function fetchPlanRevision(id: string, version: number, signal?: AbortSignal): Promise<PlanRevision> {
  const rev = normalizeRevision(await apiRequest<unknown>(`/ai/plans/${enc(id)}/revisions/${version}`, { signal }));
  if (!rev) throw badResponse('revision');
  return rev;
}

export async function patchPlan(id: string, baseVersion: number, specification: PlanSpecification): Promise<OrchestratorPlan> {
  const plan = normalizePlan(
    await apiRequest<unknown>(`/ai/plans/${enc(id)}`, {
      method: 'PATCH',
      body: JSON.stringify({ base_version: baseVersion, specification }),
    })
  );
  if (!plan) throw badResponse('plan');
  return plan;
}

export async function validatePlan(id: string): Promise<PlanValidation | null> {
  const raw = asRecord(await apiRequest<unknown>(`/ai/plans/${enc(id)}/validate`, { method: 'POST' }));
  return normalizeValidation(raw.validation ?? raw);
}

export interface ApproveRequest {
  version: number;
  spec_hash: string;
  acknowledge_risks: boolean;
  confirmation?: string;
  execute: boolean;
}

export async function approvePlan(id: string, body: ApproveRequest): Promise<ApprovePlanResult> {
  const r = asRecord(
    await apiRequest<unknown>(`/ai/plans/${enc(id)}/approve`, { method: 'POST', body: JSON.stringify(body) })
  );
  return {
    approval: normalizeApproval(r.approval),
    execution: normalizeExecution(r.execution),
    plan_id: str(r.plan_id, id),
    status: str(r.status),
    message: str(r.message),
  };
}

export async function rejectPlan(id: string, reason: string): Promise<OrchestratorPlan | null> {
  const body = reason.trim() ? { reason: reason.trim() } : {};
  return normalizePlan(
    await apiRequest<unknown>(`/ai/plans/${enc(id)}/reject`, { method: 'POST', body: JSON.stringify(body) })
  );
}

export async function cancelPlan(id: string): Promise<void> {
  await apiRequest<unknown>(`/ai/plans/${enc(id)}/cancel`, { method: 'POST' });
}

export async function executePlan(id: string, approvalId: string): Promise<Execution | null> {
  return normalizeExecution(
    await apiRequest<unknown>(`/ai/plans/${enc(id)}/execute`, {
      method: 'POST',
      body: JSON.stringify({ approval_id: approvalId }),
    })
  );
}

export async function listExecutions(
  params: { conversation_id?: string; plan_id?: string; status?: string },
  signal?: AbortSignal
): Promise<ExecutionSummary[]> {
  const q = new URLSearchParams();
  for (const [k, v] of Object.entries(params)) if (v) q.set(k, v);
  const raw = await apiRequest<unknown>(`/ai/executions?${q.toString()}`, { signal });
  return asArray(raw)
    .map(normalizeExecution)
    .filter((e): e is Execution => e !== null);
}

export async function fetchExecution(id: string, signal?: AbortSignal): Promise<Execution> {
  const exec = normalizeExecution(await apiRequest<unknown>(`/ai/executions/${enc(id)}`, { signal }));
  if (!exec) throw badResponse('execution');
  return exec;
}

export const EVENTS_PAGE_LIMIT = 200;

function authHeaders(extra: Record<string, string>): Record<string, string> {
  const token = localStorage.getItem('nevatal_token');
  return token ? { ...extra, Authorization: `Bearer ${token}` } : extra;
}

/**
 * JSON form of the event log. Uses fetch directly (not apiRequest) to pin `Accept:
 * application/json` on the URL that also serves SSE, and to accept the page with or without
 * the `{data}` envelope.
 */
export async function fetchExecutionEvents(
  id: string,
  after: number,
  signal?: AbortSignal,
  limit = EVENTS_PAGE_LIMIT
): Promise<{ events: ExecutionEvent[]; nextAfter: number }> {
  const res = await fetch(`${API_BASE}/ai/executions/${enc(id)}/events?after=${after}&limit=${limit}`, {
    headers: authHeaders({ Accept: 'application/json' }),
    signal,
    cache: 'no-store',
  });
  const body = await res.json().catch(() => null);
  if (!res.ok) {
    const info = asRecord(asRecord(body).error);
    throw new ApiError(
      str(info.message) || `Request failed with status ${res.status}`,
      str(info.code) || 'API_ERROR',
      isObject(info.details) ? info.details : undefined,
      res.status
    );
  }
  const page = asRecord(isObject(body) && 'data' in body ? body.data : body);
  const events = asArray(page.events)
    .map((e) => normalizeEvent(e))
    .filter((e): e is ExecutionEvent => e !== null)
    .sort((a, b) => a.seq - b.seq);
  const last = events.length ? events[events.length - 1].seq : after;
  return { events, nextAfter: typeof page.next_after === 'number' ? Math.max(page.next_after, after) : last };
}

export async function cancelExecution(id: string): Promise<Execution | null> {
  return normalizeExecution(await apiRequest<unknown>(`/ai/executions/${enc(id)}/cancel`, { method: 'POST' }));
}

/** Retry uses the task UUID (`task.id`), never the key (keys contain `:`). */
export async function retryExecutionTask(id: string, taskId: string): Promise<Execution | null> {
  return normalizeExecution(
    await apiRequest<unknown>(`/ai/executions/${enc(id)}/tasks/${enc(taskId)}/retry`, { method: 'POST' })
  );
}

/** `delete_created` answers `{plan_id}` (a new destructive plan); the others may answer an Execution. */
export async function recoverExecution(
  id: string,
  action: RecoveryAction | string
): Promise<{ execution: Execution | null; planId: string | null }> {
  const raw = await apiRequest<unknown>(`/ai/executions/${enc(id)}/recovery`, {
    method: 'POST',
    body: JSON.stringify({ action }),
  });
  const r = asRecord(raw);
  return { execution: normalizeExecution(r.execution ?? raw), planId: strOrNull(r.plan_id) };
}

export async function fetchTools(signal?: AbortSignal): Promise<OrchestratorTool[]> {
  const raw = await apiRequest<unknown>('/ai/tools', { signal });
  return asArray(raw)
    .map(normalizeTool)
    .filter((t): t is OrchestratorTool => t !== null);
}

export async function fetchConversationPlans(conversationId: string, signal?: AbortSignal): Promise<AIResourcePlan[]> {
  const raw = await apiRequest<unknown>(`/ai/conversations/${enc(conversationId)}/plans`, { signal });
  return asArray(raw).filter((p): p is AIResourcePlan => isObject(p) && typeof p.id === 'string');
}

export async function fetchTemplates(signal?: AbortSignal): Promise<OSTemplate[]> {
  const raw = await apiRequest<unknown>('/templates', { signal });
  return asArray(raw).filter((t): t is OSTemplate => isObject(t) && typeof t.id === 'string');
}

export async function fetchResourcePlans(signal?: AbortSignal): Promise<ResourcePlan[]> {
  const raw = await apiRequest<unknown>('/plans', { signal });
  return asArray(raw).filter((p): p is ResourcePlan => isObject(p) && typeof p.id === 'string');
}

export interface Catalog {
  templates: OSTemplate[];
  plans: ResourcePlan[];
  error: boolean;
}

let catalogPromise: Promise<Catalog> | null = null;

/** Template + size catalog (GET /templates, GET /plans), fetched once per page load; retried after a failure. */
export function loadCatalog(): Promise<Catalog> {
  if (!catalogPromise) {
    catalogPromise = Promise.all([
      fetchTemplates().then((v) => ({ v, ok: true }), () => ({ v: [] as OSTemplate[], ok: false })),
      fetchResourcePlans().then((v) => ({ v, ok: true }), () => ({ v: [] as ResourcePlan[], ok: false })),
    ]).then(([t, p]) => {
      const catalog = { templates: t.v, plans: p.v, error: !t.ok || !p.ok };
      if (catalog.error) catalogPromise = null;
      return catalog;
    });
  }
  return catalogPromise;
}

// ---------------------------------------------------------------------------------------------
// Errors (contract §7)
// ---------------------------------------------------------------------------------------------

const ERROR_HINTS: Record<string, string> = {
  VALIDATION_ERROR: 'The request was rejected as malformed.',
  ADMIN_REQUIRED: 'This plan uses admin-only tools. Only an admin can approve or run it.',
  PLAN_NOT_FOUND: 'The plan does not exist or is not yours.',
  EXECUTION_NOT_FOUND: 'The execution does not exist or is not yours.',
  TASK_NOT_FOUND: 'The task does not exist.',
  PLAN_VERSION_CONFLICT: 'A newer revision of this plan was saved while you were editing.',
  PLAN_VERSION_MISMATCH: 'The plan changed while you were reviewing it.',
  PLAN_NOT_EDITABLE: 'This plan can no longer be edited: it is executing or finished.',
  APPROVAL_REQUIRED: 'Approve the plan before running it.',
  APPROVAL_EXPIRED: 'The approval expired. Approve the plan again.',
  APPROVAL_SUPERSEDED: 'The plan was edited after it was approved. Review and approve the new revision.',
  APPROVAL_CONSUMED: 'This approval was already used by an execution. Approve again to run it again.',
  EXECUTION_ACTIVE: 'This plan already has a running execution.',
  TASK_NOT_RETRYABLE: 'Only failed tasks can be retried.',
  VALIDATION_FAILED: 'The policy check failed.',
  RISK_ACK_REQUIRED: 'Confirm that you understand the risks.',
  CONFIRMATION_REQUIRED: 'Type the resource name(s) to confirm.',
  CONFIRMATION_MISMATCH: 'The confirmation does not match the resource name(s).',
  TOO_MANY_EXECUTIONS: 'You already have 3 executions running. Wait for one to finish.',
};

/** Codes that mean "approve again" when they come back from /execute. */
export const REAPPROVE_CODES = new Set(['APPROVAL_REQUIRED', 'APPROVAL_EXPIRED', 'APPROVAL_SUPERSEDED', 'APPROVAL_CONSUMED']);

export interface DescribedError {
  code: string;
  status?: number;
  message: string;
  hint?: string;
  details: string[];
}

export function describeError(err: unknown, fallback = 'Request failed'): DescribedError {
  if (err instanceof ApiError) {
    const details: string[] = [];
    const d = err.details ?? {};
    for (const issue of issues((d as Record<string, unknown>).errors)) {
      details.push(issue.task_key ? `${issue.task_key}: ${issue.message}` : issue.message);
    }
    const field = (d as Record<string, unknown>).field;
    if (typeof field === 'string' && field) details.push(`Field: ${field}`);
    const hint = ERROR_HINTS[err.code];
    return {
      code: err.code,
      status: err.status,
      message: err.message || hint || fallback,
      hint: hint && hint !== err.message ? hint : undefined,
      details,
    };
  }
  const message = err instanceof Error ? err.message : fallback;
  return { code: 'NETWORK_ERROR', message: message || fallback, details: [] };
}

export const errorCodeOf = (err: unknown): string => (err instanceof ApiError ? err.code : '');

// ---------------------------------------------------------------------------------------------
// JSON diff (field level; arrays of objects with a `key` are matched by key, not index)
// ---------------------------------------------------------------------------------------------

export interface DiffEntry {
  path: string;
  kind: 'added' | 'removed' | 'changed';
  before?: unknown;
  after?: unknown;
}

/** JSON with object keys sorted, so equal values always serialize identically. */
export function canonicalJson(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonicalJson).join(',')}]`;
  if (isObject(value)) {
    return `{${Object.keys(value)
      .filter((k) => value[k] !== undefined)
      .sort()
      .map((k) => `${JSON.stringify(k)}:${canonicalJson(value[k])}`)
      .join(',')}}`;
  }
  return JSON.stringify(value ?? null);
}

const isPrimitive = (v: unknown) => v === null || (typeof v !== 'object' && typeof v !== 'function');

function keyOf(v: unknown): string | null {
  return isObject(v) && typeof v.key === 'string' && v.key !== '' ? v.key : null;
}

function isKeyedArray(arr: unknown[]): boolean {
  if (arr.length === 0) return true;
  const keys = arr.map(keyOf);
  return keys.every((k) => k !== null) && new Set(keys).size === keys.length;
}

export function diffJson(a: unknown, b: unknown, path = '', out: DiffEntry[] = []): DiffEntry[] {
  if (canonicalJson(a) === canonicalJson(b)) return out;
  const at = (seg: string) => (path ? `${path}.${seg}` : seg);

  if (isObject(a) && isObject(b)) {
    const keys = Array.from(new Set([...Object.keys(a), ...Object.keys(b)])).sort();
    for (const k of keys) {
      const inA = a[k] !== undefined;
      const inB = b[k] !== undefined;
      if (inA && !inB) out.push({ path: at(k), kind: 'removed', before: a[k] });
      else if (!inA && inB) out.push({ path: at(k), kind: 'added', after: b[k] });
      else diffJson(a[k], b[k], at(k), out);
    }
    return out;
  }

  if (Array.isArray(a) && Array.isArray(b)) {
    const nonEmpty = a.length > 0 || b.length > 0;
    if (nonEmpty && isKeyedArray(a) && isKeyedArray(b) && (a.length > 0 ? keyOf(a[0]) : keyOf(b[0])) !== null) {
      const mapA = new Map(a.map((x) => [keyOf(x) as string, x]));
      const mapB = new Map(b.map((x) => [keyOf(x) as string, x]));
      const order = [...mapA.keys(), ...[...mapB.keys()].filter((k) => !mapA.has(k))];
      for (const k of order) {
        const p = `${path}[${k}]`;
        if (!mapB.has(k)) out.push({ path: p, kind: 'removed', before: mapA.get(k) });
        else if (!mapA.has(k)) out.push({ path: p, kind: 'added', after: mapB.get(k) });
        else diffJson(mapA.get(k), mapB.get(k), p, out);
      }
      return out;
    }
    if (a.every(isPrimitive) && b.every(isPrimitive)) {
      out.push({ path: path || '(root)', kind: 'changed', before: a, after: b });
      return out;
    }
    const n = Math.max(a.length, b.length);
    for (let i = 0; i < n; i++) {
      const p = `${path}[${i}]`;
      if (i >= b.length) out.push({ path: p, kind: 'removed', before: a[i] });
      else if (i >= a.length) out.push({ path: p, kind: 'added', after: b[i] });
      else diffJson(a[i], b[i], p, out);
    }
    return out;
  }

  out.push({ path: path || '(root)', kind: 'changed', before: a, after: b });
  return out;
}

/** Field-level diff of what a revision hashes: `{specification, tasks}`. */
export function diffRevisions(
  from: Pick<PlanRevision, 'specification' | 'tasks'>,
  to: Pick<PlanRevision, 'specification' | 'tasks'>
): DiffEntry[] {
  return diffJson({ specification: from.specification, tasks: from.tasks }, { specification: to.specification, tasks: to.tasks });
}

export function formatDiffValue(value: unknown, max = 160): string {
  if (value === undefined) return '—';
  const text = typeof value === 'string' ? JSON.stringify(value) : canonicalJson(value);
  return text.length > max ? `${text.slice(0, max - 1)}…` : text;
}

// ---------------------------------------------------------------------------------------------
// SSE over fetch (Bearer header; EventSource cannot send one)
// ---------------------------------------------------------------------------------------------

export interface SSEFrame {
  id: string | null;
  event: string;
  data: string;
}

/** Incremental text/event-stream parser (WHATWG rules: CRLF/LF/CR, `:` comments, multi-line data). */
export class SSEParser {
  private buf = '';
  private data: string[] = [];
  private event = '';
  private lastId: string | null = null;

  push(chunk: string): SSEFrame[] {
    this.buf += chunk;
    const frames: SSEFrame[] = [];
    for (;;) {
      const m = /\r\n|\r|\n/.exec(this.buf);
      if (!m) break;
      // A lone CR at the end may be the first half of CRLF split across chunks: wait for more.
      if (m[0] === '\r' && m.index === this.buf.length - 1) break;
      const line = this.buf.slice(0, m.index);
      this.buf = this.buf.slice(m.index + m[0].length);
      const frame = this.line(line);
      if (frame) frames.push(frame);
    }
    return frames;
  }

  /** End of stream: a trailing frame without its blank line is still delivered (lenient). */
  end(): SSEFrame[] {
    const frames = this.buf ? this.push('\n') : [];
    const last = this.line('');
    if (last) frames.push(last);
    return frames;
  }

  private line(line: string): SSEFrame | null {
    if (line === '') {
      if (this.data.length === 0) {
        this.event = '';
        return null;
      }
      const frame = { id: this.lastId, event: this.event || 'message', data: this.data.join('\n') };
      this.data = [];
      this.event = '';
      return frame;
    }
    if (line.startsWith(':')) return null; // comment / heartbeat (": ping")
    const idx = line.indexOf(':');
    const field = idx === -1 ? line : line.slice(0, idx);
    let value = idx === -1 ? '' : line.slice(idx + 1);
    if (value.startsWith(' ')) value = value.slice(1);
    if (field === 'data') this.data.push(value);
    else if (field === 'event') this.event = value;
    else if (field === 'id' && !value.includes('\0')) this.lastId = value;
    return null;
  }
}

/** The endpoint answered, but not with an event stream: switch to polling instead of retrying. */
export class StreamUnsupportedError extends Error {
  constructor(message: string) {
    super(message);
    this.name = 'StreamUnsupportedError';
  }
}

export function frameToEvent(frame: SSEFrame): ExecutionEvent | null {
  let payload: unknown = null;
  try {
    payload = JSON.parse(frame.data);
  } catch {
    return null;
  }
  return normalizeEvent(payload, frame.id, frame.event);
}

/** No bytes for this long (the server sends `: ping` every 20 s) means the stream stalled. */
export const SSE_IDLE_TIMEOUT_MS = 45_000;

/**
 * One SSE connection. Resolves 'finished' after `execution.finished`, 'eof' when the server closed
 * the stream early (restart/shutdown), or 'stalled' when nothing arrived for `idleTimeoutMs` (a proxy
 * or tunnel kept a dead stream open). The caller reconnects with Last-Event-ID in both cases.
 */
export async function streamExecutionEvents(
  executionId: string,
  opts: {
    after: number;
    signal: AbortSignal;
    onOpen?: () => void;
    onEvent: (event: ExecutionEvent) => void;
    idleTimeoutMs?: number;
  }
): Promise<'finished' | 'eof' | 'stalled'> {
  const headers = authHeaders({ Accept: 'text/event-stream', 'Cache-Control': 'no-cache' });
  if (opts.after > 0) headers['Last-Event-ID'] = String(opts.after);
  const idle = opts.idleTimeoutMs ?? SSE_IDLE_TIMEOUT_MS;
  const conn = new AbortController();
  const abortConn = () => conn.abort();
  opts.signal.addEventListener('abort', abortConn, { once: true });
  let lastByteAt = Date.now();
  let stalled = false;
  const watchdog = setInterval(() => {
    if (Date.now() - lastByteAt > idle) {
      stalled = true;
      conn.abort();
    }
  }, Math.max(250, Math.min(5000, Math.floor(idle / 3))));

  const deliver = (frames: SSEFrame[]): boolean => {
    for (const frame of frames) {
      const ev = frameToEvent(frame);
      if (!ev) continue;
      opts.onEvent(ev);
      if (ev.type === 'execution.finished') return true;
    }
    return false;
  };

  try {
    const res = await fetch(`${API_BASE}/ai/executions/${enc(executionId)}/events?after=${opts.after}`, {
      headers,
      signal: conn.signal,
      cache: 'no-store',
    });
    lastByteAt = Date.now();
    if (!res.ok) {
      await res.body?.cancel().catch(() => {});
      if (res.status === 404 || res.status === 406 || res.status === 501) {
        throw new StreamUnsupportedError(`Event stream unavailable (HTTP ${res.status})`);
      }
      throw new Error(`Event stream failed (HTTP ${res.status})`);
    }
    const type = res.headers.get('content-type') || '';
    if (!type.includes('text/event-stream') || !res.body) {
      await res.body?.cancel().catch(() => {});
      throw new StreamUnsupportedError('The server did not answer with an event stream');
    }
    opts.onOpen?.();
    const reader = res.body.getReader();
    const decoder = new TextDecoder();
    const parser = new SSEParser();
    try {
      for (;;) {
        const { value, done } = await reader.read();
        lastByteAt = Date.now();
        if (done) break;
        if (deliver(parser.push(decoder.decode(value, { stream: true })))) return 'finished';
      }
      return deliver([...parser.push(decoder.decode()), ...parser.end()]) ? 'finished' : 'eof';
    } finally {
      reader.cancel().catch(() => {});
    }
  } catch (err) {
    if (stalled && !opts.signal.aborted) return 'stalled';
    throw err;
  } finally {
    clearInterval(watchdog);
    opts.signal.removeEventListener('abort', abortConn);
  }
}

// ---------------------------------------------------------------------------------------------
// Formatting
// ---------------------------------------------------------------------------------------------

export const parseTime = (iso: string | null | undefined): number | null => {
  if (!iso) return null;
  const t = Date.parse(iso);
  return Number.isFinite(t) ? t : null;
};

export function formatDuration(ms: number): string {
  if (!Number.isFinite(ms) || ms < 0) return '—';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

export function durationOf(start: string | null, end: string | null, now = Date.now()): string | null {
  const a = parseTime(start);
  if (a === null) return null;
  const b = parseTime(end) ?? now;
  return formatDuration(b - a);
}

export function formatClock(iso: string | null | undefined): string {
  const t = parseTime(iso);
  return t === null ? '—' : new Date(t).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}

export function formatDateTime(iso: string | null | undefined): string {
  const t = parseTime(iso);
  return t === null
    ? '—'
    : new Date(t).toLocaleString([], { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
}

/** "in 42 min" / "3 min ago". */
export function formatRelative(iso: string | null | undefined, now = Date.now()): string {
  const t = parseTime(iso);
  if (t === null) return '—';
  const diff = t - now;
  const mins = Math.round(Math.abs(diff) / 60000);
  const label = mins < 1 ? 'less than a minute' : mins < 120 ? `${mins} min` : `${Math.round(mins / 60)} h`;
  return diff >= 0 ? `in ${label}` : `${label} ago`;
}

export function formatMemoryMb(value: unknown): string {
  const mb = typeof value === 'number' ? value : Number(value);
  if (!Number.isFinite(mb)) return String(value ?? '—');
  return mb >= 1024 && mb % 1024 === 0 ? `${mb / 1024} GB` : mb >= 1024 ? `${(mb / 1024).toFixed(1)} GB` : `${mb} MB`;
}

export const shortHash = (hash: string): string => {
  const [algo, hex] = hash.includes(':') ? hash.split(':', 2) : ['', hash];
  return hex ? `${algo ? `${algo}:` : ''}${hex.slice(0, 10)}${hex.length > 10 ? '…' : ''}` : '—';
};

export const shortId = (id: string): string => (id.length > 8 ? id.slice(0, 8) : id);

const RESOURCE_LABELS: Record<string, string> = {
  vcpus: 'vCPU',
  memory_mb: 'Memory',
  disk_gb: 'Disk',
  instances: 'Instances',
  buckets: 'Buckets',
  storage_gb: 'Storage',
};

/** Label + display value for an `estimated_resources` entry or a config field. */
export function describeQuantity(key: string, value: unknown): { label: string; value: string } {
  const label = RESOURCE_LABELS[key] ?? key.replace(/_/g, ' ');
  if (key === 'memory_mb') return { label, value: formatMemoryMb(value) };
  if (key.endsWith('_gb') && typeof value === 'number') return { label, value: `${value} GB` };
  if (typeof value === 'object' && value !== null) return { label, value: canonicalJson(value) };
  return { label, value: String(value ?? '—') };
}

const SECRET_KEY_RE = /pass(word|phrase)?|secret|token|private[_-]?key|api[_-]?key|credential|authorization/i;

/** Defense in depth: outputs are sanitized server-side; mask anything that still looks secret. */
export function redactSecrets(value: unknown, depth = 0): unknown {
  if (depth > 8) return '…';
  if (Array.isArray(value)) return value.map((v) => redactSecrets(v, depth + 1));
  if (isObject(value)) {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value)) {
      out[k] = SECRET_KEY_RE.test(k) && v !== null && v !== '' ? '••••••' : redactSecrets(v, depth + 1);
    }
    return out;
  }
  return value;
}

/** The exact text a destructive approval needs: the expected names joined by ", ". */
export function expectedConfirmation(revision: PlanRevision | null | undefined): string | null {
  const expected = revision?.confirmation_required?.expected ?? [];
  return expected.length ? expected.join(', ') : null;
}

/** Depth of each task in the dependency DAG (0 = no dependencies); cycles are cut. */
export function taskDepths(tasks: { key: string; depends_on: string[] }[]): Map<string, number> {
  const byKey = new Map(tasks.map((t) => [t.key, t]));
  const depth = new Map<string, number>();
  const visit = (key: string, stack: Set<string>): number => {
    const known = depth.get(key);
    if (known !== undefined) return known;
    const task = byKey.get(key);
    if (!task || stack.has(key)) return 0;
    stack.add(key);
    const d = task.depends_on.reduce(
      (acc, dep) => (byKey.has(dep) && !stack.has(dep) ? Math.max(acc, visit(dep, stack) + 1) : acc),
      0
    );
    stack.delete(key);
    depth.set(key, d);
    return d;
  };
  for (const t of tasks) visit(t.key, new Set());
  return depth;
}

/** Tasks in dependency order (stable for independent tasks). */
export function orderTasks<T extends { key: string; depends_on: string[] }>(tasks: T[]): T[] {
  const depths = taskDepths(tasks);
  return tasks
    .map((t, i) => ({ t, i }))
    .sort((a, b) => (depths.get(a.t.key) ?? 0) - (depths.get(b.t.key) ?? 0) || a.i - b.i)
    .map((x) => x.t);
}
