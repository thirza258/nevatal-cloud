import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Bot,
  ClipboardList,
  Globe,
  History,
  Layers,
  MessageSquare,
  PanelRight,
  PanelRightClose,
  PlusCircle,
  RefreshCw,
  Send,
  Server,
  Sparkles,
  Trash2,
  Workflow,
  Wrench,
  X,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { useAuth } from '../../context/AuthContext';
import { useSystemConfig } from '../../context/SystemConfigContext';
import type {
  AIConversation,
  AIMessage,
  AIResourcePlan,
  ApprovePlanResult,
  Execution,
  ExecutionSummary,
  OrchestratorPlan,
  OrchestratorTool,
  UIComponentsPayload,
  WorkflowState,
} from '../../types';
import {
  AI_WORKSPACE_PATH,
  DescribedError,
  REAPPROVE_CODES,
  approvePlan,
  cancelPlan,
  conversationIdFromPath,
  conversationPath,
  describeError,
  errorCodeOf,
  executePlan,
  fetchConversationPlans,
  fetchExecution,
  fetchPlan,
  fetchTools,
  isActiveExecution,
  isUuid,
  listExecutions,
  parseTime,
  planDeepLink,
  readPlanParam,
  rejectPlan,
  sameId,
  writePlanParam,
} from '../../lib/orchestrator';
import { ConfirmDialog, Notice, NoticeTone, PlanStatusBadge, Spinner } from '../../components/orchestrator/OrchestratorBits';
import { PanelNotice, PlanPanel } from '../../components/orchestrator/PlanPanel';
import { ApprovalModal } from '../../components/orchestrator/ApprovalModal';
import { ExecutionCard } from '../../components/orchestrator/ExecutionCard';
import { ToolsCatalog } from '../../components/orchestrator/ToolsCatalog';
import {
  LegacyProgress,
  LegacyResult,
  PlanProposalCard,
  QuestionsCard,
  WorkflowStateBadge,
  parseUIComponents,
} from '../../components/orchestrator/ChatBits';

interface AICloudAgentProps {
  /** From `/dashboard/ai/conversations/<id>` (or the legacy in-memory route param). */
  conversationId?: string;
  onNavigate: (path: string, param?: string) => void;
}

type TimelineItem =
  | { kind: 'message'; msg: AIMessage; ui: UIComponentsPayload | null }
  | { kind: 'execution'; exec: ExecutionSummary };

const QUICK_BUILDERS = [
  { label: 'Deploy VPS', prompt: 'Create an Ubuntu 24.04 VPS with 4GB RAM', icon: Server },
  { label: 'Web App', prompt: 'Deploy a web application from GitHub repository', icon: Globe },
  { label: 'n8n Setup', prompt: 'Setup dedicated n8n workflow automation instance', icon: Workflow },
  { label: 'OpenClaw', prompt: 'Deploy OpenClaw autonomous AI agent container', icon: Bot },
];

const byCreated = (a: { created_at: string }, b: { created_at: string }) =>
  (parseTime(a.created_at) ?? 0) - (parseTime(b.created_at) ?? 0);

/**
 * AI Infrastructure Orchestrator workspace: conversation history | chat + inline execution
 * timelines | plan / approval panel. The planner proposes, the user approves a specific revision
 * here, the engine executes and verifies; chat text never approves anything.
 */
export const AICloudAgent: React.FC<AICloudAgentProps> = ({ conversationId, onNavigate }) => {
  const { user } = useAuth();
  const { config } = useSystemConfig();
  const isAdmin = user?.role === 'admin';

  // Deep link: /dashboard/ai/conversations/<conversationId>?plan=<planId> (both UUID-checked).
  const initialLink = useMemo(() => {
    const path = conversationIdFromPath(window.location.pathname);
    const plan = readPlanParam();
    return { conversationId: path.id, planId: plan.id, invalid: path.invalid || plan.invalid };
  }, []);

  const [linkInvalid, setLinkInvalid] = useState(initialLink.invalid);
  const [conversations, setConversations] = useState<AIConversation[]>([]);
  const [activeConvId, setActiveConvId] = useState<string | null>(
    conversationId && isUuid(conversationId) ? conversationId.toLowerCase() : initialLink.conversationId
  );
  const [activeConv, setActiveConv] = useState<AIConversation | null>(null);
  const [messages, setMessages] = useState<AIMessage[]>([]);
  const [convPlans, setConvPlans] = useState<AIResourcePlan[]>([]);
  const [executions, setExecutions] = useState<ExecutionSummary[]>([]);
  const [convLoading, setConvLoading] = useState(false);
  const [inputText, setInputText] = useState('');
  const [sending, setSending] = useState(false);
  const [notice, setNotice] = useState<{ tone: NoticeTone; text: string } | null>(null);

  const [historyOpen, setHistoryOpen] = useState(false);
  const [panelOpen, setPanelOpen] = useState(!!initialLink.planId);
  const [panelTab, setPanelTab] = useState<'plan' | 'tools'>('plan');

  const [explicitPlanId, setExplicitPlanId] = useState<string | null>(initialLink.planId);
  const [suppressDefaultPlan, setSuppressDefaultPlan] = useState(false);
  const [plan, setPlan] = useState<OrchestratorPlan | null>(null);
  const [planLoading, setPlanLoading] = useState(false);
  const [planError, setPlanError] = useState<DescribedError | null>(null);
  const [planNotice, setPlanNotice] = useState<PanelNotice | null>(null);
  const [approveOpen, setApproveOpen] = useState(false);

  const [tools, setTools] = useState<OrchestratorTool[] | null>(null);
  const [toolsError, setToolsError] = useState<DescribedError | null>(null);
  const [toolsLoading, setToolsLoading] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<AIConversation | null>(null);
  const [cancelPlanTarget, setCancelPlanTarget] = useState<string | null>(null);
  const [rejectOpen, setRejectOpen] = useState(false);
  const [rejectReason, setRejectReason] = useState('');
  const [dialogBusy, setDialogBusy] = useState(false);
  const [dialogError, setDialogError] = useState<DescribedError | null>(null);

  const [focusExecutionId, setFocusExecutionId] = useState<string | null>(null);

  const streamRef = useRef<HTMLDivElement>(null);
  const convSeq = useRef(0);
  const planSeq = useRef(0);
  const pendingApprove = useRef<string | null>(null);
  const activeConvRef = useRef(activeConvId);
  activeConvRef.current = activeConvId;
  const autoSelected = useRef(false);
  /** A conversation created on this page: nothing to load yet, and its starter prompt may be in flight. */
  const freshConvRef = useRef<string | null>(null);

  // ------------------------------------------------------------------ conversations

  const loadConversations = useCallback(async () => {
    try {
      const res = await apiRequest<AIConversation[]>('/ai/conversations');
      setConversations(Array.isArray(res) ? res : []);
    } catch (err) {
      setNotice({ tone: 'error', text: describeError(err, 'Could not load conversations').message });
    }
  }, []);

  useEffect(() => {
    void loadConversations();
  }, [loadConversations]);

  // No conversation in the URL and no plan link: open the most recent one (never overrides a deep link).
  useEffect(() => {
    if (autoSelected.current || activeConvId || explicitPlanId || conversations.length === 0) return;
    autoSelected.current = true;
    setActiveConvId(conversations[0].id);
  }, [conversations, activeConvId, explicitPlanId]);

  // Path changes from App (sidebar navigation, back/forward).
  useEffect(() => {
    if (conversationId && isUuid(conversationId) && !sameId(conversationId, activeConvRef.current)) {
      setActiveConvId(conversationId.toLowerCase());
    }
  }, [conversationId]);

  // `?plan=` changes (back/forward, or in-app navigation that only changed the query).
  useEffect(() => {
    const sync = () => {
      const p = readPlanParam();
      setExplicitPlanId(p.id);
      if (p.id) {
        setSuppressDefaultPlan(false);
        setPanelOpen(true);
        setPanelTab('plan');
      }
      if (p.invalid) setLinkInvalid(true);
    };
    window.addEventListener('popstate', sync);
    return () => window.removeEventListener('popstate', sync);
  }, []);

  const loadConversation = useCallback(async (id: string, quiet = false) => {
    const seq = ++convSeq.current;
    if (!quiet) setConvLoading(true);
    try {
      const [conv, msgs, plans, execs] = await Promise.all([
        apiRequest<AIConversation>(`/ai/conversations/${encodeURIComponent(id)}`),
        apiRequest<AIMessage[]>(`/ai/conversations/${encodeURIComponent(id)}/messages`),
        fetchConversationPlans(id).catch(() => null),
        listExecutions({ conversation_id: id }).catch(() => null),
      ]);
      if (seq !== convSeq.current) return;
      setActiveConv(conv);
      setMessages(Array.isArray(msgs) ? msgs : []);
      if (plans) setConvPlans(plans);
      if (execs) {
        setExecutions((prev) => {
          const fresh = new Map(execs.map((e) => [e.id, e]));
          // Keep executions the list does not know about yet (just started from this page).
          for (const e of prev) if (!fresh.has(e.id) && (!e.conversation_id || sameId(e.conversation_id, id))) fresh.set(e.id, e);
          return Array.from(fresh.values()).sort(byCreated);
        });
      }
    } catch (err) {
      if (seq !== convSeq.current) return;
      setNotice({ tone: 'error', text: describeError(err, 'Could not load the conversation').message });
    } finally {
      if (seq === convSeq.current) setConvLoading(false);
    }
  }, []);

  useEffect(() => {
    if (activeConvId && sameId(freshConvRef.current, activeConvId)) {
      freshConvRef.current = null; // state was already reset by handleNewConversation
      return;
    }
    setActiveConv(null);
    setMessages([]);
    setConvPlans([]);
    setExecutions([]);
    setFocusExecutionId(null);
    setSuppressDefaultPlan(false);
    if (activeConvId) void loadConversation(activeConvId);
  }, [activeConvId, loadConversation]);

  const refreshPlansList = useCallback(() => {
    const id = activeConvRef.current;
    if (!id) return;
    fetchConversationPlans(id)
      .then((plans) => {
        if (sameId(id, activeConvRef.current)) setConvPlans(plans);
      })
      .catch(() => {});
  }, []);

  const selectConversation = (id: string) => {
    setHistoryOpen(false);
    if (sameId(id, activeConvId)) return;
    setExplicitPlanId(null);
    setPlanNotice(null);
    setActiveConvId(id);
    onNavigate(conversationPath(id));
  };

  const handleNewConversation = async (starterPrompt?: string) => {
    if (sending) return;
    setSending(true);
    try {
      const res = await apiRequest<AIConversation>('/ai/conversations', {
        method: 'POST',
        body: JSON.stringify({ title: starterPrompt ? starterPrompt.slice(0, 35) : 'New Infrastructure Plan' }),
      });
      setConversations((prev) => [res, ...prev.filter((c) => c.id !== res.id)]);
      // Switch without a load: a new conversation has nothing on the server yet, and loading it
      // would race the starter prompt (and wipe the user's message until the reply arrives).
      convSeq.current++; // drop any in-flight load of the previous conversation
      freshConvRef.current = res.id;
      setActiveConv(res);
      setMessages([]);
      setConvPlans([]);
      setExecutions([]);
      setFocusExecutionId(null);
      setSuppressDefaultPlan(false);
      setSending(false);
      selectConversation(res.id);
      if (starterPrompt) await sendMessage(starterPrompt, res.id);
    } catch (err) {
      setNotice({ tone: 'error', text: describeError(err, 'Could not start a conversation').message });
    } finally {
      setSending(false);
    }
  };

  const confirmDeleteConversation = async () => {
    if (!deleteTarget) return;
    setDialogBusy(true);
    setDialogError(null);
    try {
      await apiRequest(`/ai/conversations/${encodeURIComponent(deleteTarget.id)}`, { method: 'DELETE' });
      const remaining = conversations.filter((c) => c.id !== deleteTarget.id);
      setConversations(remaining);
      if (sameId(deleteTarget.id, activeConvId)) {
        setExplicitPlanId(null);
        if (remaining.length > 0) selectConversation(remaining[0].id);
        else {
          setActiveConvId(null);
          onNavigate(AI_WORKSPACE_PATH);
        }
      }
      setDeleteTarget(null);
    } catch (err) {
      setDialogError(describeError(err, 'Could not delete the conversation'));
    } finally {
      setDialogBusy(false);
    }
  };

  // ------------------------------------------------------------------ messages

  async function sendMessage(text?: string, targetConvId?: string, answerKey?: string, answerVal?: string) {
    const body = (text ?? inputText).trim();
    const convId = targetConvId ?? activeConvId;
    if (!body && !answerVal) return;
    if (!convId) {
      await handleNewConversation(body);
      return;
    }
    if (sending) return;
    const temp: AIMessage = {
      id: `temp-${Date.now()}`,
      conversation_id: convId,
      user_id: '',
      role: 'user',
      content: body || `Selected: ${answerVal}`,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, temp]);
    if (!text) setInputText('');
    setSending(true);
    try {
      const resp = await apiRequest<{ message: AIMessage; state: WorkflowState }>(
        `/ai/conversations/${encodeURIComponent(convId)}/messages`,
        {
          method: 'POST',
          body: JSON.stringify({ message: body || answerVal, answer_key: answerKey, answer_val: answerVal }),
        }
      );
      if (sameId(convId, activeConvRef.current)) {
        setMessages((prev) => [...prev.filter((m) => m.id !== temp.id), temp, ...(resp?.message ? [resp.message] : [])]);
        if (resp?.state) setActiveConv((c) => (c ? { ...c, state: resp.state } : c));
        refreshPlansList();
      }
      void loadConversations();
    } catch (err) {
      setMessages((prev) => prev.filter((m) => m.id !== temp.id));
      if (!text) setInputText(body);
      setNotice({ tone: 'error', text: describeError(err, 'Could not send the message').message });
    } finally {
      setSending(false);
    }
  }

  // ------------------------------------------------------------------ plan panel

  const parsedMessages = useMemo(
    () => messages.map((msg) => ({ msg, ui: msg.role === 'user' ? null : parseUIComponents(msg.ui_components) })),
    [messages]
  );

  const defaultPlanId = useMemo(() => {
    for (let i = parsedMessages.length - 1; i >= 0; i--) {
      const id = parsedMessages[i].ui?.plan_proposal?.plan_id;
      if (id && isUuid(id)) return id.toLowerCase();
    }
    const fromState = activeConv?.state_obj?.active_plan_id;
    if (fromState && isUuid(fromState)) return fromState.toLowerCase();
    const latest = [...convPlans].sort(byCreated).pop();
    return latest && isUuid(latest.id) ? latest.id.toLowerCase() : null;
  }, [parsedMessages, activeConv, convPlans]);

  const activePlanId = explicitPlanId ?? (suppressDefaultPlan ? null : defaultPlanId);

  const loadPlan = useCallback(async (id: string, quiet = false) => {
    const seq = ++planSeq.current;
    if (!quiet) setPlanLoading(true);
    try {
      const p = await fetchPlan(id);
      if (seq !== planSeq.current) return;
      setPlan(p);
      setPlanError(null);
      if (pendingApprove.current && sameId(pendingApprove.current, p.id)) {
        pendingApprove.current = null;
        if (p.revision) setApproveOpen(true);
      }
      // A plan link (MCP) may point at another conversation, or at none: open the plan's conversation.
      if (p.conversation_id && isUuid(p.conversation_id) && !sameId(p.conversation_id, activeConvRef.current)) {
        setActiveConvId(p.conversation_id.toLowerCase());
        window.history.replaceState(window.history.state, '', planDeepLink(p.conversation_id.toLowerCase(), p.id));
      }
    } catch (err) {
      if (seq !== planSeq.current) return;
      pendingApprove.current = null;
      setPlanError(describeError(err, 'Could not load the plan'));
      if (!quiet) setPlan(null);
    } finally {
      if (seq === planSeq.current) setPlanLoading(false);
    }
  }, []);

  useEffect(() => {
    setPlan(null);
    setPlanError(null);
    setApproveOpen(false);
    if (activePlanId) void loadPlan(activePlanId);
    else planSeq.current++;
  }, [activePlanId, loadPlan]);

  const openPlan = useCallback(
    (planId: string, opts?: { approve?: boolean; notice?: string }) => {
      if (!isUuid(planId)) return;
      const id = planId.toLowerCase();
      setExplicitPlanId(id);
      setSuppressDefaultPlan(false);
      writePlanParam(id);
      setPanelOpen(true);
      setPanelTab('plan');
      setHistoryOpen(false);
      setPlanNotice(opts?.notice ? { tone: 'warning', text: opts.notice } : null);
      if (opts?.approve) {
        // Always approve against a freshly fetched revision.
        pendingApprove.current = id;
        if (sameId(id, activePlanId)) void loadPlan(id, true);
      }
    },
    [activePlanId, loadPlan]
  );

  const closePlan = () => {
    setExplicitPlanId(null);
    setSuppressDefaultPlan(true);
    writePlanParam(null);
    setPanelOpen(false);
    setPlanNotice(null);
  };

  const reloadPlan = useCallback(() => {
    if (activePlanId) void loadPlan(activePlanId, true);
  }, [activePlanId, loadPlan]);

  // ------------------------------------------------------------------ executions

  const upsertExecution = useCallback((exec: ExecutionSummary) => {
    setExecutions((prev) => {
      const without = prev.filter((e) => e.id !== exec.id);
      return [...without, exec].sort(byCreated);
    });
  }, []);

  const showExecution = useCallback(
    async (executionId: string) => {
      setPanelOpen(false);
      setHistoryOpen(false);
      if (!executions.some((e) => e.id === executionId)) {
        try {
          upsertExecution(await fetchExecution(executionId));
        } catch (err) {
          setNotice({ tone: 'error', text: describeError(err, 'Could not load the execution').message });
          return;
        }
      }
      setFocusExecutionId(executionId);
    },
    [executions, upsertExecution]
  );

  useEffect(() => {
    if (!focusExecutionId) return;
    const timer = setTimeout(() => {
      const container = streamRef.current;
      const el = container?.querySelector<HTMLElement>(`[data-execution-id="${CSS.escape(focusExecutionId)}"]`);
      if (container && el) container.scrollTo({ top: Math.max(0, el.offsetTop - 12), behavior: 'smooth' });
    }, 60);
    return () => clearTimeout(timer);
  }, [focusExecutionId, executions]);

  const onExecutionSettled = useCallback(
    (exec: Execution) => {
      upsertExecution(exec);
      if (plan && sameId(exec.plan_id, plan.id)) void loadPlan(plan.id, true);
      refreshPlansList();
      if (activeConvRef.current) void loadConversation(activeConvRef.current, true);
      void loadConversations();
    },
    [plan, loadPlan, refreshPlansList, loadConversation, loadConversations, upsertExecution]
  );

  const startedExecution = (exec: Execution | null, message: string) => {
    if (exec) {
      upsertExecution(exec);
      setFocusExecutionId(exec.id);
      setPanelOpen(false);
    }
    setPlanNotice({ tone: 'success', text: message });
    reloadPlan();
    refreshPlansList();
    void loadConversations();
  };

  const handleApproved = (result: ApprovePlanResult, executeNow: boolean) => {
    setApproveOpen(false);
    startedExecution(
      result.execution,
      executeNow
        ? result.execution
          ? 'Approved. The execution started; follow it in the timeline.'
          : result.message || 'Approved.'
        : 'Approved. Use Run when you are ready (the approval expires).'
    );
  };

  const handleRun = async (approvalId: string) => {
    if (!plan) return;
    try {
      const exec = await executePlan(plan.id, approvalId);
      startedExecution(exec, 'The execution started; follow it in the timeline.');
    } catch (err) {
      if (REAPPROVE_CODES.has(errorCodeOf(err))) reloadPlan();
      throw err;
    }
  };

  const handleQuickRun = async () => {
    const rev = plan?.revision;
    if (!plan || !rev) return;
    try {
      const result = await approvePlan(plan.id, {
        version: rev.version,
        spec_hash: rev.spec_hash,
        acknowledge_risks: false,
        execute: true,
      });
      startedExecution(result.execution, 'The plan is running; follow it in the timeline.');
    } catch (err) {
      if (errorCodeOf(err) === 'PLAN_VERSION_MISMATCH') {
        reloadPlan();
        setPlanNotice({ tone: 'warning', text: 'The plan changed while you were reviewing it. Review the new revision and run it again.' });
        return;
      }
      throw err;
    }
  };

  const confirmReject = async () => {
    if (!plan) return;
    setDialogBusy(true);
    setDialogError(null);
    try {
      const updated = await rejectPlan(plan.id, rejectReason);
      if (updated) setPlan(updated);
      else reloadPlan();
      setRejectOpen(false);
      setRejectReason('');
      setPlanNotice({ tone: 'info', text: 'Plan rejected. Nothing will run.' });
      refreshPlansList();
    } catch (err) {
      setDialogError(describeError(err, 'Could not reject the plan'));
    } finally {
      setDialogBusy(false);
    }
  };

  const confirmCancelPlan = async () => {
    if (!cancelPlanTarget) return;
    setDialogBusy(true);
    setDialogError(null);
    try {
      await cancelPlan(cancelPlanTarget);
      if (plan && sameId(plan.id, cancelPlanTarget)) reloadPlan();
      setCancelPlanTarget(null);
      if (activeConvRef.current) void loadConversation(activeConvRef.current, true);
      void loadConversations();
    } catch (err) {
      setDialogError(describeError(err, 'Could not cancel the plan'));
    } finally {
      setDialogBusy(false);
    }
  };

  // ------------------------------------------------------------------ tools

  const loadTools = useCallback(async () => {
    setToolsLoading(true);
    try {
      setTools(await fetchTools());
      setToolsError(null);
    } catch (err) {
      setToolsError(describeError(err, 'Could not load the tool registry'));
    } finally {
      setToolsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadTools();
  }, [loadTools]);

  // ------------------------------------------------------------------ chat stream

  const planStatusById = useMemo(() => new Map(convPlans.map((p) => [p.id.toLowerCase(), p])), [convPlans]);

  const timeline = useMemo<TimelineItem[]>(() => {
    const items: TimelineItem[] = [];
    const execs = [...executions].sort(byCreated);
    let j = 0;
    for (const p of parsedMessages) {
      const at = parseTime(p.msg.created_at) ?? 0;
      while (j < execs.length && (parseTime(execs[j].created_at) ?? 0) < at) items.push({ kind: 'execution', exec: execs[j++] });
      items.push({ kind: 'message', msg: p.msg, ui: p.ui });
    }
    while (j < execs.length) items.push({ kind: 'execution', exec: execs[j++] });
    return items;
  }, [parsedMessages, executions]);

  useEffect(() => {
    const el = streamRef.current;
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: 'smooth' });
  }, [messages.length, sending]);

  const activeExecutionForPlan = useMemo(
    () => (plan ? executions.find((e) => sameId(e.plan_id, plan.id) && isActiveExecution(e.status)) ?? null : null),
    [executions, plan]
  );

  const showBackdrop = historyOpen || panelOpen;

  // ------------------------------------------------------------------ render

  return (
    <div
      className="relative flex h-[calc(100dvh-5.5rem)] sm:h-[calc(100dvh-6.5rem)] lg:h-[calc(100dvh-7.5rem)] min-h-[30rem] rounded-lg border border-zinc-800 bg-zinc-950 overflow-hidden"
      data-testid="ai-workspace"
    >
      {/* Drawer backdrop (below the breakpoints where the side columns are static) */}
      {showBackdrop && (
        <div
          className={`absolute inset-0 z-20 bg-black/60 ${panelOpen ? 'min-[1360px]:hidden' : 'lg:hidden'}`}
          onClick={() => {
            setHistoryOpen(false);
            setPanelOpen(false);
          }}
          aria-hidden="true"
        />
      )}

      {/* ---------------- Left: conversation history ---------------- */}
      <aside
        className={`${historyOpen ? 'flex' : 'hidden'} lg:flex absolute lg:static inset-y-0 left-0 z-30 w-72 max-w-[85%] lg:w-60 flex-col border-r border-zinc-800 bg-zinc-950`}
        data-testid="history-sidebar"
      >
        <div className="p-3 border-b border-zinc-800 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0">
            <div className="w-6 h-6 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-300 flex-shrink-0">
              <Sparkles className="w-3.5 h-3.5" />
            </div>
            <div className="min-w-0">
              <div className="text-xs font-semibold text-white tracking-tight truncate">AI Cloud Agent</div>
              <div className="text-[10px] text-zinc-500 font-mono">Orchestrator</div>
            </div>
          </div>
          <div className="flex items-center gap-1">
            <button
              type="button"
              onClick={() => void handleNewConversation()}
              disabled={sending}
              className="flex items-center gap-1 px-2.5 py-1 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-medium transition-colors disabled:opacity-50"
              data-testid="new-conversation"
            >
              <PlusCircle className="w-3.5 h-3.5" />
              <span>New</span>
            </button>
            <button
              type="button"
              onClick={() => setHistoryOpen(false)}
              className="lg:hidden p-1 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800"
              title="Close history"
            >
              <X className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        <div className="p-2.5 border-b border-zinc-800">
          <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500 mb-1.5 px-1">Quick builders</div>
          <div className="grid grid-cols-2 gap-1">
            {QUICK_BUILDERS.map((b) => (
              <button
                key={b.label}
                type="button"
                onClick={() => void handleNewConversation(b.prompt)}
                disabled={sending}
                className="flex items-center gap-1.5 p-1.5 rounded-md bg-zinc-900 hover:bg-zinc-800 border border-zinc-800 text-left text-xs text-zinc-200 transition-colors disabled:opacity-50 min-w-0"
              >
                <b.icon className="w-3 h-3 text-zinc-400 flex-shrink-0" />
                <span className="truncate">{b.label}</span>
              </button>
            ))}
          </div>
        </div>

        <div className="flex-1 overflow-y-auto p-1.5 space-y-0.5" data-testid="conversation-list">
          <div className="px-2 py-1 text-[10px] font-mono uppercase tracking-wider text-zinc-500">Recent</div>
          {conversations.length === 0 ? (
            <div className="p-3 text-center text-xs text-zinc-500">No conversations yet.</div>
          ) : (
            conversations.map((c) => {
              const active = sameId(c.id, activeConvId);
              return (
                <div
                  key={c.id}
                  className={`group relative flex items-center rounded-md text-xs transition-colors ${
                    active ? 'bg-zinc-800 text-white font-medium' : 'hover:bg-zinc-900 text-zinc-400 hover:text-zinc-200'
                  }`}
                >
                  <button
                    type="button"
                    onClick={() => selectConversation(c.id)}
                    className="flex-1 min-w-0 flex items-center gap-2 p-2 text-left"
                    data-conversation-id={c.id}
                    aria-current={active ? 'page' : undefined}
                  >
                    <MessageSquare className={`w-3.5 h-3.5 flex-shrink-0 ${active ? 'text-zinc-200' : 'text-zinc-500'}`} />
                    <span className="truncate flex-1 min-w-0">{c.title}</span>
                    <WorkflowStateBadge state={c.state} compact />
                  </button>
                  <button
                    type="button"
                    onClick={() => {
                      setDialogError(null);
                      setDeleteTarget(c);
                    }}
                    title="Delete conversation"
                    aria-label={`Delete conversation ${c.title}`}
                    className="p-1 mr-1 text-zinc-500 hover:text-rose-400 rounded transition-opacity lg:opacity-0 lg:group-hover:opacity-100 focus:opacity-100"
                  >
                    <Trash2 className="w-3 h-3" />
                  </button>
                </div>
              );
            })
          )}
        </div>
      </aside>

      {/* ---------------- Center: chat + execution timelines ---------------- */}
      <section className="flex-1 min-w-0 flex flex-col">
        <div className="h-12 px-2 sm:px-4 border-b border-zinc-800 flex items-center justify-between gap-2">
          <div className="flex items-center gap-2 min-w-0">
            <button
              type="button"
              onClick={() => setHistoryOpen(true)}
              className="lg:hidden p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800"
              title="Conversations"
              data-testid="open-history"
            >
              <History className="w-4 h-4" />
            </button>
            <span className="font-semibold text-sm text-white truncate min-w-0" data-testid="conversation-title">
              {activeConv?.title || 'Nevatal AI Cloud Agent'}
            </span>
            <span className="hidden sm:inline-flex">
              <WorkflowStateBadge state={activeConv?.state} />
            </span>
          </div>
          <div className="flex items-center gap-1 flex-shrink-0">
            <button
              type="button"
              onClick={() => activeConvId && void loadConversation(activeConvId, true)}
              disabled={!activeConvId || convLoading}
              className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-40"
              title="Refresh conversation"
            >
              <RefreshCw className={`w-3.5 h-3.5 ${convLoading ? 'animate-spin' : ''}`} />
            </button>
            <button
              type="button"
              onClick={() => {
                setPanelTab('tools');
                setPanelOpen(true);
              }}
              className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
              title="Tool registry"
              data-testid="open-tools"
            >
              <Wrench className="w-3.5 h-3.5" />
            </button>
            <button
              type="button"
              onClick={() => {
                setPanelTab('plan');
                setPanelOpen(true);
              }}
              className="min-[1360px]:hidden inline-flex items-center gap-1 px-2 py-1 rounded-md text-xs text-zinc-300 hover:text-white bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 transition-colors"
              title="Plan and approval"
              data-testid="open-plan-panel"
            >
              <PanelRight className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">Plan</span>
            </button>
            <button
              type="button"
              onClick={() => onNavigate('/dashboard/resources')}
              className="hidden md:inline-flex items-center gap-1 text-xs text-zinc-300 hover:text-white px-2 py-1 rounded-md bg-zinc-900 border border-zinc-800 hover:bg-zinc-800 transition-colors"
            >
              <Layers className="w-3.5 h-3.5" />
              <span>Resources</span>
            </button>
          </div>
        </div>

        <div ref={streamRef} className="relative flex-1 overflow-y-auto overflow-x-hidden p-3 sm:p-5 space-y-5" data-testid="chat-stream">
          {linkInvalid && (
            <Notice tone="warning" onDismiss={() => setLinkInvalid(false)}>
              Ignored part of the link: it does not contain a valid conversation or plan ID.
            </Notice>
          )}
          {notice && (
            <Notice tone={notice.tone} onDismiss={() => setNotice(null)}>
              <span data-testid="page-notice">{notice.text}</span>
            </Notice>
          )}

          {!activeConvId && conversations.length === 0 && !convLoading && (
            <div className="max-w-xl mx-auto text-center py-12 space-y-2">
              <Sparkles className="w-6 h-6 text-zinc-600 mx-auto" />
              <div className="text-sm font-medium text-zinc-200">Describe the infrastructure you need</div>
              <p className="text-xs text-zinc-500 leading-relaxed">
                The agent writes a plan. You review it, approve the exact revision, and follow the execution here. Nothing runs
                without your approval.
              </p>
            </div>
          )}

          {convLoading && messages.length === 0 && (
            <div className="flex items-center gap-2 text-xs text-zinc-500">
              <Spinner /> Loading conversation…
            </div>
          )}

          {timeline.map((item) => {
            if (item.kind === 'execution') {
              return (
                <div key={`exec-${item.exec.id}`} className="max-w-3xl">
                  <ExecutionCard
                    executionId={item.exec.id}
                    initial={item.exec}
                    highlight={focusExecutionId === item.exec.id}
                    onOpenPlan={openPlan}
                    onSettled={onExecutionSettled}
                    onNavigate={onNavigate}
                  />
                </div>
              );
            }
            const { msg, ui } = item;
            const isUser = msg.role === 'user';
            const proposal = ui?.plan_proposal;
            const live = proposal ? planStatusById.get(proposal.plan_id?.toLowerCase?.() ?? '') : undefined;
            return (
              <div key={msg.id} className={`flex gap-2.5 sm:gap-3 max-w-3xl ${isUser ? 'ml-auto flex-row-reverse' : ''}`}>
                <div
                  className={`w-7 h-7 rounded-md flex items-center justify-center flex-shrink-0 text-xs font-mono border ${
                    isUser ? 'bg-zinc-800 text-zinc-200 border-zinc-700' : 'bg-zinc-900 text-white border-zinc-800'
                  }`}
                >
                  {isUser ? 'U' : <Sparkles className="w-3.5 h-3.5" />}
                </div>
                <div className={`space-y-2.5 flex-1 min-w-0 ${isUser ? 'flex flex-col items-end' : ''}`}>
                  <div
                    className={`p-3 rounded-lg text-xs sm:text-sm leading-relaxed border max-w-full ${
                      isUser ? 'bg-zinc-800 text-white border-zinc-700' : 'bg-zinc-900/60 border-zinc-800 text-zinc-200'
                    }`}
                  >
                    <div className="whitespace-pre-wrap break-words font-sans">{msg.content}</div>
                  </div>
                  {!isUser && ui && (
                    <div className="space-y-2.5">
                      {ui.questions && ui.questions.length > 0 && (
                        <QuestionsCard
                          questions={ui.questions}
                          disabled={sending}
                          onAnswer={(key, value) => void sendMessage(undefined, undefined, key, value)}
                        />
                      )}
                      {proposal && (
                        <PlanProposalCard
                          proposal={proposal}
                          liveStatus={live?.status}
                          liveVersion={live?.version}
                          selected={!!activePlanId && sameId(activePlanId, proposal.plan_id)}
                          onReview={() => openPlan(proposal.plan_id)}
                          onApprove={() => openPlan(proposal.plan_id, { approve: true })}
                          onModify={() => void sendMessage("I'd like to modify some configuration options")}
                          onCancel={() => {
                            setDialogError(null);
                            setCancelPlanTarget(proposal.plan_id);
                          }}
                        />
                      )}
                      {ui.progress_steps && ui.progress_steps.length > 0 && <LegacyProgress steps={ui.progress_steps} />}
                      {ui.success_data && (
                        <LegacyResult data={ui.success_data} onOpenResources={() => onNavigate('/dashboard/resources')} />
                      )}
                    </div>
                  )}
                </div>
              </div>
            );
          })}

          {sending && (
            <div className="flex gap-3 max-w-xl">
              <div className="w-7 h-7 rounded-md bg-zinc-900 border border-zinc-800 flex items-center justify-center text-white animate-pulse">
                <Sparkles className="w-3.5 h-3.5" />
              </div>
              <div className="p-2.5 rounded-lg bg-zinc-900 border border-zinc-800 text-zinc-400 text-xs flex items-center gap-2 font-mono">
                <Spinner className="w-3 h-3" />
                <span>Analyzing requirements &amp; capacity…</span>
              </div>
            </div>
          )}
        </div>

        <div className="p-2.5 sm:p-3 border-t border-zinc-800 bg-zinc-950">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              void sendMessage();
            }}
            className="flex items-center gap-2"
          >
            <input
              type="text"
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              placeholder="What would you like to build? e.g. 'Create an Ubuntu VPS with 4GB RAM'"
              className="flex-1 min-w-0 bg-zinc-900 border border-zinc-800 rounded-md px-3 py-2 text-xs text-zinc-100 placeholder-zinc-500 focus:outline-none focus:border-zinc-700 transition-colors"
              disabled={sending}
              data-testid="chat-input"
            />
            <button
              type="submit"
              disabled={!inputText.trim() || sending}
              className="p-2 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 transition-colors disabled:opacity-50"
              title="Send"
            >
              <Send className="w-3.5 h-3.5" />
            </button>
          </form>
          <div className="hidden sm:flex items-center justify-between gap-2 text-[10px] text-zinc-500 font-mono mt-1.5 px-1">
            <span className="truncate">Plans are approved per revision · chat messages never approve anything</span>
            <span className="flex-shrink-0">MCP · Proxmox VE</span>
          </div>
        </div>
      </section>

      {/* ---------------- Right: plan / approval / tools ---------------- */}
      <aside
        className={`${panelOpen ? 'flex' : 'hidden'} min-[1360px]:flex absolute min-[1360px]:static inset-y-0 right-0 z-30 w-full sm:w-[24rem] min-[1360px]:w-[22rem] flex-col border-l border-zinc-800 bg-zinc-950`}
        data-testid="right-panel"
      >
        <div className="flex items-center gap-1 px-2 py-1.5 border-b border-zinc-800">
          {(
            [
              ['plan', 'Plan', ClipboardList],
              ['tools', 'Tools', Wrench],
            ] as const
          ).map(([key, label, Icon]) => (
            <button
              key={key}
              type="button"
              onClick={() => setPanelTab(key)}
              aria-pressed={panelTab === key}
              className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-md text-[11px] font-medium transition-colors ${
                panelTab === key ? 'bg-zinc-800 text-white' : 'text-zinc-400 hover:text-zinc-200'
              }`}
              data-testid={`panel-tab-${key}`}
            >
              <Icon className="w-3 h-3" />
              {label}
            </button>
          ))}
          <button
            type="button"
            onClick={() => setPanelOpen(false)}
            className="ml-auto min-[1360px]:hidden p-1 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800"
            title="Hide panel"
            aria-label="Hide panel"
            data-testid="hide-panel"
          >
            <PanelRightClose className="w-3.5 h-3.5" />
          </button>
        </div>
        <div className="flex-1 min-h-0">
          {panelTab === 'tools' ? (
            <ToolsCatalog tools={tools} error={toolsError} loading={toolsLoading} onReload={() => void loadTools()} />
          ) : activePlanId ? (
            <PlanPanel
              planId={activePlanId}
              plan={plan && sameId(plan.id, activePlanId) ? plan : null}
              loading={planLoading}
              error={planError}
              notice={planNotice}
              onDismissNotice={() => setPlanNotice(null)}
              tools={tools ?? []}
              activeExecution={activeExecutionForPlan}
              onReload={reloadPlan}
              onApprove={() => openPlan(activePlanId, { approve: true })}
              onQuickRun={handleQuickRun}
              onRun={handleRun}
              onReject={() => {
                setDialogError(null);
                setRejectOpen(true);
              }}
              onPlanChanged={(updated, message) => {
                setPlan(updated);
                if (message) setPlanNotice({ tone: 'success', text: message });
                refreshPlansList();
              }}
              onShowExecution={(id) => void showExecution(id)}
              onClose={closePlan}
            />
          ) : (
            <div className="h-full overflow-y-auto p-3 space-y-2" data-testid="plans-list">
              <div className="text-[10px] font-mono uppercase tracking-wider text-zinc-500">Plans in this conversation</div>
              {convPlans.length === 0 ? (
                <p className="text-[11px] text-zinc-500 leading-relaxed">
                  No plans yet. Ask the agent for infrastructure; its plan appears here for review and approval.
                </p>
              ) : (
                [...convPlans]
                  .sort(byCreated)
                  .reverse()
                  .map((p) => (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => openPlan(p.id)}
                      className="w-full text-left rounded-md border border-zinc-800 bg-zinc-950/60 hover:bg-zinc-900 p-2 space-y-1"
                    >
                      <div className="text-xs font-medium text-zinc-100 break-words">{p.title}</div>
                      <div className="flex flex-wrap items-center gap-1.5">
                        <PlanStatusBadge status={p.status} />
                        <span className="text-[10px] font-mono text-zinc-500">v{p.version}</span>
                      </div>
                    </button>
                  ))
              )}
            </div>
          )}
        </div>
      </aside>

      {/* ---------------- Dialogs ---------------- */}
      {approveOpen && plan && plan.revision && (
        <ApprovalModal
          key={plan.id}
          plan={plan}
          tools={tools ?? []}
          isAdmin={isAdmin}
          mockHypervisor={config.proxmox_mock}
          onClose={() => setApproveOpen(false)}
          onApproved={handleApproved}
          onPlanReloaded={(latest) => {
            setPlan(latest);
            refreshPlansList();
          }}
        />
      )}

      <ConfirmDialog
        open={!!deleteTarget}
        title="Delete conversation?"
        description={deleteTarget ? `"${deleteTarget.title}" and its messages are removed. Resources it created are not deleted.` : undefined}
        confirmLabel="Delete"
        tone="danger"
        busy={dialogBusy}
        error={dialogError}
        onConfirm={() => void confirmDeleteConversation()}
        onClose={() => setDeleteTarget(null)}
      />

      <ConfirmDialog
        open={!!cancelPlanTarget}
        title="Cancel this plan?"
        description="The plan will not run. If it is already running, the active execution is cancelled too."
        confirmLabel="Cancel plan"
        tone="danger"
        busy={dialogBusy}
        error={dialogError}
        onConfirm={() => void confirmCancelPlan()}
        onClose={() => setCancelPlanTarget(null)}
      />

      <ConfirmDialog
        open={rejectOpen}
        title="Reject this plan?"
        description="A rejected plan cannot be approved or run. Ask the agent for a new plan instead."
        confirmLabel="Reject plan"
        tone="danger"
        busy={dialogBusy}
        error={dialogError}
        onConfirm={() => void confirmReject()}
        onClose={() => setRejectOpen(false)}
      >
        <label htmlFor="reject-reason" className="block text-[11px] text-zinc-400">
          Reason (optional)
        </label>
        <textarea
          id="reject-reason"
          value={rejectReason}
          onChange={(e) => setRejectReason(e.target.value)}
          rows={3}
          maxLength={500}
          className="w-full bg-zinc-900 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-zinc-100 focus:outline-none focus:border-zinc-600 resize-none"
        />
      </ConfirmDialog>
    </div>
  );
};
