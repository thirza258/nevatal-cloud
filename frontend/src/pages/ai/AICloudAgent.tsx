import React, { useState, useEffect, useRef } from 'react';
import {
  Sparkles,
  Send,
  PlusCircle,
  RefreshCw,
  Server,
  Globe,
  Workflow,
  Bot,
  Database,
  CheckCircle2,
  AlertCircle,
  Clock,
  ArrowRight,
  Trash2,
  Copy,
  Check,
  Terminal,
  ExternalLink,
  ShieldAlert,
  ChevronRight,
  Layers,
  Cpu,
  HardDrive,
  MessageSquare,
  Sliders,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import {
  AIConversation,
  AIMessage,
  WorkflowState,
  StructuredQuestion,
  StructuredPlanView,
  UIProgressStep,
  UIComponentsPayload,
} from '../../types';

interface AICloudAgentProps {
  conversationId?: string;
  onNavigate: (path: string, param?: string) => void;
}

export const AICloudAgent: React.FC<AICloudAgentProps> = ({ conversationId, onNavigate }) => {
  const [conversations, setConversations] = useState<AIConversation[]>([]);
  const [activeConvId, setActiveConvId] = useState<string | null>(conversationId || null);
  const [activeConv, setActiveConv] = useState<AIConversation | null>(null);
  const [messages, setMessages] = useState<AIMessage[]>([]);
  const [inputText, setInputText] = useState('');
  const [loading, setLoading] = useState(false);
  const [approvingPlanId, setApprovingPlanId] = useState<string | null>(null);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [selectedAnswers, setSelectedAnswers] = useState<Record<string, string>>({});

  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages, loading]);

  // Load conversation list
  const loadConversations = async () => {
    try {
      const res = await apiRequest<AIConversation[]>('/ai/conversations');
      setConversations(res);
      if (!activeConvId && res.length > 0) {
        setActiveConvId(res[0].id);
      }
    } catch (err) {
      console.error('Failed to load conversations:', err);
    }
  };

  useEffect(() => {
    loadConversations();
  }, []);

  // When activeConvId changes, load details
  useEffect(() => {
    if (conversationId && conversationId !== activeConvId) {
      setActiveConvId(conversationId);
    }
  }, [conversationId]);

  const loadConversationDetails = async (id: string) => {
    try {
      const [conv, msgs] = await Promise.all([
        apiRequest<AIConversation>(`/ai/conversations/${id}`),
        apiRequest<AIMessage[]>(`/ai/conversations/${id}/messages`),
      ]);
      setActiveConv(conv);
      setMessages(msgs);
    } catch (err) {
      console.error('Failed to load conversation details:', err);
    }
  };

  useEffect(() => {
    if (activeConvId) {
      loadConversationDetails(activeConvId);
    } else {
      setActiveConv(null);
      setMessages([]);
    }
  }, [activeConvId]);

  // Create new conversation
  const handleNewConversation = async (starterPrompt?: string) => {
    try {
      setLoading(true);
      const res = await apiRequest<AIConversation>('/ai/conversations', {
        method: 'POST',
        body: JSON.stringify({ title: starterPrompt ? starterPrompt.slice(0, 35) : 'New Infrastructure Plan' }),
      });
      setConversations((prev) => [res, ...prev]);
      setActiveConvId(res.id);
      onNavigate('/dashboard/ai/conversations', res.id);

      if (starterPrompt) {
        // Send initial message automatically
        setTimeout(() => {
          handleSendMessage(starterPrompt, res.id);
        }, 300);
      }
    } catch (err: any) {
      alert(err.message || 'Failed to start conversation');
    } finally {
      setLoading(false);
    }
  };

  // Delete conversation
  const handleDeleteConversation = async (e: React.MouseEvent, id: string) => {
    e.stopPropagation();
    if (!confirm('Are you sure you want to delete this conversation?')) return;
    try {
      await apiRequest(`/ai/conversations/${id}`, { method: 'DELETE' });
      setConversations((prev) => prev.filter((c) => c.id !== id));
      if (activeConvId === id) {
        const remaining = conversations.filter((c) => c.id !== id);
        setActiveConvId(remaining.length > 0 ? remaining[0].id : null);
      }
    } catch (err: any) {
      alert(err.message || 'Failed to delete conversation');
    }
  };

  // Send message
  const handleSendMessage = async (textToSend?: string, targetConvId?: string, answerKey?: string, answerVal?: string) => {
    const text = (textToSend || inputText).trim();
    const convId = targetConvId || activeConvId;

    if (!text && !answerVal) return;
    if (!convId) {
      // Create first then send
      await handleNewConversation(text);
      return;
    }

    if (loading) return;

    // Optimistically add user message
    const tempUserMsg: AIMessage = {
      id: 'temp-' + Date.now(),
      conversation_id: convId,
      user_id: '',
      role: 'user',
      content: text || `Selected: ${answerVal}`,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, tempUserMsg]);
    setInputText('');
    setLoading(true);

    try {
      const resp = await apiRequest<{
        message: AIMessage;
        state: WorkflowState;
        plan_proposal?: StructuredPlanView;
      }>(`/ai/conversations/${convId}/messages`, {
        method: 'POST',
        body: JSON.stringify({
          message: text || answerVal,
          answer_key: answerKey,
          answer_val: answerVal,
        }),
      });

      // Update messages with server response
      setMessages((prev) => [...prev.filter((m) => m.id !== tempUserMsg.id), tempUserMsg, resp.message]);

      if (activeConv) {
        setActiveConv({
          ...activeConv,
          state: resp.state,
        });
      }
      loadConversations();
    } catch (err: any) {
      alert(err.message || 'Failed to send message');
    } finally {
      setLoading(false);
    }
  };

  // Handle plan approval
  const handleApprovePlan = async (planId: string) => {
    if (!activeConvId) return;
    setApprovingPlanId(planId);
    try {
      const resp = await apiRequest<{
        status: string;
        progress_steps: UIProgressStep[];
        message: string;
      }>(`/ai/plans/${planId}/approve`, {
        method: 'POST',
      });

      // Reload conversation messages to display provisioning progress & report
      await loadConversationDetails(activeConvId);
      loadConversations();
    } catch (err: any) {
      alert(err.message || 'Failed to approve plan');
    } finally {
      setApprovingPlanId(null);
    }
  };

  // Handle plan cancellation
  const handleCancelPlan = async (planId: string) => {
    if (!activeConvId) return;
    if (!confirm('Cancel this infrastructure plan?')) return;
    try {
      await apiRequest(`/ai/plans/${planId}/cancel`, {
        method: 'POST',
      });
      await loadConversationDetails(activeConvId);
      loadConversations();
    } catch (err: any) {
      alert(err.message || 'Failed to cancel plan');
    }
  };

  const copyToClipboard = (text: string, key: string) => {
    navigator.clipboard.writeText(text);
    setCopiedKey(key);
    setTimeout(() => setCopiedKey(null), 2000);
  };

  const getStateBadge = (state: WorkflowState) => {
    switch (state) {
      case 'UnderstandingRequest':
        return <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-blue-500/10 text-blue-400 border border-blue-500/20">Understanding</span>;
      case 'ClarifyingRequirements':
        return <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-amber-500/10 text-amber-400 border border-amber-500/20">Clarifying</span>;
      case 'Planning':
        return <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-purple-500/10 text-purple-400 border border-purple-500/20">Planning</span>;
      case 'AwaitingConfirmation':
        return <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-amber-400/20 text-amber-300 border border-amber-400/40 animate-pulse">Awaiting Approval</span>;
      case 'Provisioning':
        return <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-blue-500/20 text-blue-300 border border-blue-500/40 animate-pulse">Provisioning</span>;
      case 'Completed':
        return <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">Completed</span>;
      case 'Cancelled':
        return <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-slate-500/10 text-slate-400 border border-slate-500/20">Cancelled</span>;
      case 'Failed':
        return <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-rose-500/10 text-rose-400 border border-rose-500/20">Failed</span>;
      default:
        return null;
    }
  };

  return (
    <div className="flex h-[calc(100vh-8.5rem)] rounded-2xl border border-slate-800 bg-slate-900/60 overflow-hidden shadow-2xl backdrop-blur">
      {/* Sidebar: Conversations & Presets */}
      <div className="w-80 border-r border-slate-800 flex flex-col bg-slate-950/40">
        <div className="p-4 border-b border-slate-800 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-7 h-7 rounded-lg bg-blue-600 flex items-center justify-center text-white shadow-md shadow-blue-500/20">
              <Sparkles className="w-4 h-4" />
            </div>
            <div>
              <div className="text-xs font-bold text-white tracking-tight">AI Cloud Agent</div>
              <div className="text-[10px] text-slate-400">Conversational Console</div>
            </div>
          </div>
          <button
            onClick={() => handleNewConversation()}
            className="flex items-center gap-1.5 px-2.5 py-1.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-medium transition shadow-sm"
          >
            <PlusCircle className="w-3.5 h-3.5" />
            <span>New Chat</span>
          </button>
        </div>

        {/* Quick Presets */}
        <div className="p-3 border-b border-slate-800/80 bg-slate-900/30">
          <div className="text-[10px] font-semibold uppercase tracking-wider text-slate-500 mb-2 px-1">
            Quick Builders
          </div>
          <div className="grid grid-cols-2 gap-1.5">
            <button
              onClick={() => handleNewConversation('Create an Ubuntu 24.04 VPS with 4GB RAM')}
              className="flex items-center gap-1.5 p-2 rounded-xl bg-slate-800/70 hover:bg-slate-700/80 border border-slate-700/40 text-left text-xs text-slate-200 transition"
            >
              <Server className="w-3.5 h-3.5 text-blue-400 flex-shrink-0" />
              <span className="truncate">Deploy VPS</span>
            </button>
            <button
              onClick={() => handleNewConversation('Deploy a web application from GitHub repository')}
              className="flex items-center gap-1.5 p-2 rounded-xl bg-slate-800/70 hover:bg-slate-700/80 border border-slate-700/40 text-left text-xs text-slate-200 transition"
            >
              <Globe className="w-3.5 h-3.5 text-emerald-400 flex-shrink-0" />
              <span className="truncate">Web App</span>
            </button>
            <button
              onClick={() => handleNewConversation('Setup dedicated n8n workflow automation instance')}
              className="flex items-center gap-1.5 p-2 rounded-xl bg-slate-800/70 hover:bg-slate-700/80 border border-slate-700/40 text-left text-xs text-slate-200 transition"
            >
              <Workflow className="w-3.5 h-3.5 text-amber-400 flex-shrink-0" />
              <span className="truncate">n8n Setup</span>
            </button>
            <button
              onClick={() => handleNewConversation('Deploy OpenClaw autonomous AI agent container')}
              className="flex items-center gap-1.5 p-2 rounded-xl bg-slate-800/70 hover:bg-slate-700/80 border border-slate-700/40 text-left text-xs text-slate-200 transition"
            >
              <Bot className="w-3.5 h-3.5 text-purple-400 flex-shrink-0" />
              <span className="truncate">OpenClaw</span>
            </button>
          </div>
        </div>

        {/* Conversation List */}
        <div className="flex-1 overflow-y-auto p-2 space-y-1">
          <div className="px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
            Recent Conversations
          </div>
          {conversations.length === 0 ? (
            <div className="p-4 text-center text-xs text-slate-500">
              No conversations yet. Start by describing what you want to build!
            </div>
          ) : (
            conversations.map((c) => {
              const active = c.id === activeConvId;
              return (
                <div
                  key={c.id}
                  onClick={() => {
                    setActiveConvId(c.id);
                    onNavigate('/dashboard/ai/conversations', c.id);
                  }}
                  className={`group relative flex items-center justify-between p-2.5 rounded-xl cursor-pointer text-xs transition ${
                    active
                      ? 'bg-blue-600/15 border border-blue-500/30 text-white font-medium shadow-sm'
                      : 'hover:bg-slate-800/60 text-slate-300 border border-transparent'
                  }`}
                >
                  <div className="flex items-center gap-2 overflow-hidden pr-6">
                    <MessageSquare className={`w-3.5 h-3.5 flex-shrink-0 ${active ? 'text-blue-400' : 'text-slate-500'}`} />
                    <span className="truncate">{c.title}</span>
                  </div>

                  <div className="flex items-center gap-1.5 flex-shrink-0">
                    {getStateBadge(c.state)}
                    <button
                      onClick={(e) => handleDeleteConversation(e, c.id)}
                      title="Delete Conversation"
                      className="opacity-0 group-hover:opacity-100 p-1 text-slate-500 hover:text-rose-400 rounded transition"
                    >
                      <Trash2 className="w-3.5 h-3.5" />
                    </button>
                  </div>
                </div>
              );
            })
          )}
        </div>
      </div>

      {/* Main Conversation Stream */}
      <div className="flex-1 flex flex-col min-w-0 bg-slate-900/40">
        {/* Active Conversation Top Bar */}
        <div className="h-14 px-6 border-b border-slate-800 flex items-center justify-between bg-slate-950/20">
          <div className="flex items-center gap-3">
            <span className="font-bold text-sm text-white truncate max-w-md">
              {activeConv?.title || 'Nevatal AI Cloud Agent'}
            </span>
            {activeConv && getStateBadge(activeConv.state)}
          </div>

          <div className="flex items-center gap-2">
            <button
              onClick={() => activeConvId && loadConversationDetails(activeConvId)}
              className="p-1.5 rounded-lg text-slate-400 hover:text-white hover:bg-slate-800 transition"
              title="Refresh conversation state"
            >
              <RefreshCw className="w-4 h-4" />
            </button>
            <button
              onClick={() => onNavigate('/dashboard/resources')}
              className="flex items-center gap-1 text-xs text-blue-400 hover:text-blue-300 px-2 py-1 rounded-lg bg-blue-500/10 border border-blue-500/20 transition"
            >
              <Layers className="w-3.5 h-3.5" />
              <span>View Resources</span>
            </button>
          </div>
        </div>

        {/* Message Stream */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {messages.map((msg) => {
            const isUser = msg.role === 'user';
            let uiPayload: UIComponentsPayload | null = null;
            if (msg.ui_components) {
              try {
                uiPayload = JSON.parse(msg.ui_components);
              } catch (e) {
                // ignore
              }
            }

            return (
              <div
                key={msg.id}
                className={`flex gap-3.5 max-w-3xl ${isUser ? 'ml-auto flex-row-reverse' : ''}`}
              >
                {/* Avatar */}
                <div
                  className={`w-8 h-8 rounded-xl flex items-center justify-center flex-shrink-0 text-white font-bold text-xs shadow-md ${
                    isUser
                      ? 'bg-blue-600 shadow-blue-600/20'
                      : 'bg-gradient-to-br from-indigo-500 to-purple-600 shadow-indigo-500/20'
                  }`}
                >
                  {isUser ? 'U' : <Sparkles className="w-4 h-4" />}
                </div>

                {/* Message Body */}
                <div className={`space-y-3.5 flex-1 ${isUser ? 'items-end text-right' : ''}`}>
                  <div
                    className={`p-4 rounded-2xl text-xs sm:text-sm leading-relaxed border shadow-sm ${
                      isUser
                        ? 'bg-blue-600 text-white border-blue-500/50 rounded-tr-none'
                        : 'bg-slate-900 border-slate-800 text-slate-200 rounded-tl-none'
                    }`}
                  >
                    <div className="prose prose-invert prose-xs max-w-none whitespace-pre-wrap font-sans">
                      {msg.content}
                    </div>
                  </div>

                  {/* Render Structured UI Components if attached */}
                  {!isUser && uiPayload && (
                    <div className="space-y-4">
                      {/* 1. Dynamic Requirement Gathering Questions */}
                      {uiPayload.questions && uiPayload.questions.length > 0 && (
                        <div className="space-y-3 p-4 rounded-2xl bg-slate-950/60 border border-slate-800">
                          <div className="text-xs font-bold text-white tracking-tight flex items-center gap-1.5">
                            <Sliders className="w-3.5 h-3.5 text-blue-400" />
                            <span>Select Configuration Options</span>
                          </div>

                          {uiPayload.questions.map((q) => (
                            <div key={q.id} className="space-y-2">
                              <div className="text-xs font-semibold text-slate-200">{q.title}</div>
                              {q.description && (
                                <div className="text-[11px] text-slate-400">{q.description}</div>
                              )}
                              <div className="flex flex-wrap gap-2 pt-1">
                                {q.options?.map((opt) => (
                                  <button
                                    key={opt.id}
                                    onClick={() => handleSendMessage(undefined, undefined, q.key, opt.value)}
                                    className={`px-3 py-1.5 rounded-xl text-xs font-medium transition border flex items-center gap-1.5 ${
                                      opt.is_default
                                        ? 'bg-blue-600/20 text-blue-300 border-blue-500/40 hover:bg-blue-600/30'
                                        : 'bg-slate-800/80 text-slate-300 border-slate-700/60 hover:bg-slate-700'
                                    }`}
                                  >
                                    <span>{opt.label}</span>
                                    {opt.is_default && (
                                      <span className="text-[9px] px-1 py-0.2 rounded bg-blue-500/30 text-blue-300 font-mono">
                                        Default
                                      </span>
                                    )}
                                  </button>
                                ))}
                              </div>
                            </div>
                          ))}
                        </div>
                      )}

                      {/* 2. Structured Plan Proposal Review Card */}
                      {uiPayload.plan_proposal && (
                        <div className="p-5 rounded-2xl bg-gradient-to-b from-slate-900 to-slate-950 border border-blue-500/30 shadow-xl space-y-4">
                          <div className="flex items-center justify-between border-b border-slate-800 pb-3">
                            <div className="flex items-center gap-2">
                              <div className="w-6 h-6 rounded-lg bg-blue-600/20 flex items-center justify-center text-blue-400 border border-blue-500/30">
                                <Cpu className="w-3.5 h-3.5" />
                              </div>
                              <div>
                                <h4 className="text-xs font-bold text-white tracking-tight">
                                  {uiPayload.plan_proposal.title}
                                </h4>
                                <span className="text-[11px] text-slate-400">
                                  Plan Version {uiPayload.plan_proposal.version} • {uiPayload.plan_proposal.summary}
                                </span>
                              </div>
                            </div>
                            <span className="text-[11px] px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-300 border border-amber-500/20 font-medium">
                              Confirmation Required
                            </span>
                          </div>

                          {/* Resource Allocations Breakdown */}
                          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2.5">
                            {Object.entries(uiPayload.plan_proposal.estimated_allocations || {}).map(([k, v]) => (
                              <div key={k} className="p-2.5 rounded-xl bg-slate-950/70 border border-slate-800">
                                <div className="text-[10px] uppercase font-semibold text-slate-500">{k.replace('_', ' ')}</div>
                                <div className="text-xs font-bold text-slate-200 mt-0.5">{String(v)}</div>
                              </div>
                            ))}
                          </div>

                          {/* Resources Table */}
                          <div className="space-y-2">
                            {uiPayload.plan_proposal.resources.map((res, idx) => (
                              <div key={idx} className="p-3 rounded-xl bg-slate-800/40 border border-slate-700/50 space-y-2">
                                <div className="flex items-center justify-between">
                                  <span className="text-xs font-bold text-white">{res.name}</span>
                                  <span className="text-[10px] text-blue-400 font-mono uppercase">{res.type}</span>
                                </div>
                                <div className="grid grid-cols-2 gap-x-4 gap-y-1 text-[11px]">
                                  {Object.entries(res.details || {}).map(([dk, dv]) => (
                                    <div key={dk} className="flex justify-between border-b border-slate-800/60 py-0.5">
                                      <span className="text-slate-400">{dk}:</span>
                                      <span className="text-slate-200 font-medium truncate max-w-[140px]">{String(dv)}</span>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            ))}
                          </div>

                          {/* Action Buttons */}
                          {uiPayload.plan_proposal.status === 'awaiting_confirmation' && (
                            <div className="flex items-center gap-2 pt-2 border-t border-slate-800">
                              <button
                                onClick={() => handleApprovePlan(uiPayload!.plan_proposal!.plan_id)}
                                disabled={approvingPlanId === uiPayload.plan_proposal.plan_id}
                                className="flex-1 flex items-center justify-center gap-2 py-2 px-4 rounded-xl bg-blue-600 hover:bg-blue-500 text-white font-semibold text-xs shadow-lg shadow-blue-600/30 transition disabled:opacity-50"
                              >
                                {approvingPlanId === uiPayload.plan_proposal.plan_id ? (
                                  <>
                                    <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                                    <span>Provisioning Resources...</span>
                                  </>
                                ) : (
                                  <>
                                    <CheckCircle2 className="w-3.5 h-3.5" />
                                    <span>Confirm and Create</span>
                                  </>
                                )}
                              </button>
                              <button
                                onClick={() => handleSendMessage("I'd like to modify some configuration options")}
                                className="py-2 px-3 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-xs transition"
                              >
                                Modify
                              </button>
                              <button
                                onClick={() => handleCancelPlan(uiPayload!.plan_proposal!.plan_id)}
                                className="py-2 px-3 rounded-xl bg-rose-500/10 hover:bg-rose-500/20 text-rose-300 border border-rose-500/20 font-medium text-xs transition"
                              >
                                Cancel
                              </button>
                            </div>
                          )}
                        </div>
                      )}

                      {/* 3. Real-Time Provisioning Progress Tracker */}
                      {uiPayload.progress_steps && uiPayload.progress_steps.length > 0 && (
                        <div className="p-4 rounded-2xl bg-slate-950/80 border border-slate-800 space-y-3">
                          <div className="text-xs font-bold text-white flex items-center justify-between">
                            <span>Provisioning Pipeline Progress</span>
                            <span className="text-[11px] text-blue-400 font-mono">Live Proxmox VE Job</span>
                          </div>
                          <div className="space-y-2">
                            {uiPayload.progress_steps.map((step) => {
                              const isDone = step.status === 'completed';
                              const isInProg = step.status === 'in_progress';
                              const isFail = step.status === 'failed';
                              return (
                                <div key={step.step_id} className="flex items-center gap-2.5 text-xs">
                                  {isDone && <CheckCircle2 className="w-4 h-4 text-emerald-400 flex-shrink-0" />}
                                  {isInProg && <div className="w-3.5 h-3.5 rounded-full border-2 border-blue-400 border-t-transparent animate-spin flex-shrink-0" />}
                                  {isFail && <AlertCircle className="w-4 h-4 text-rose-400 flex-shrink-0" />}
                                  {!isDone && !isInProg && !isFail && (
                                    <div className="w-3.5 h-3.5 rounded-full border border-slate-700 flex-shrink-0" />
                                  )}
                                  <span className={isDone ? 'text-slate-200' : isInProg ? 'text-blue-400 font-semibold' : 'text-slate-500'}>
                                    {step.title}
                                  </span>
                                  {step.completed_at && (
                                    <span className="text-[10px] text-slate-500 font-mono ml-auto">{step.completed_at}</span>
                                  )}
                                </div>
                              );
                            })}
                          </div>
                        </div>
                      )}

                      {/* 4. Success Report Card */}
                      {uiPayload.success_data && (
                        <div className="p-4 rounded-2xl bg-emerald-950/20 border border-emerald-500/30 space-y-3">
                          <div className="flex items-center gap-2 text-xs font-bold text-emerald-400">
                            <CheckCircle2 className="w-4 h-4" />
                            <span>Service Provisioned & Verified Successfully</span>
                          </div>
                          {uiPayload.success_data.ssh_command && (
                            <div className="flex items-center justify-between p-2 rounded-xl bg-slate-950 border border-slate-800 font-mono text-xs text-slate-300">
                              <span>{String(uiPayload.success_data.ssh_command)}</span>
                              <button
                                onClick={() => copyToClipboard(String(uiPayload!.success_data!.ssh_command), 'ssh')}
                                className="p-1 hover:text-white"
                              >
                                {copiedKey === 'ssh' ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                              </button>
                            </div>
                          )}
                          <div className="flex items-center gap-2 pt-1">
                            <button
                              onClick={() => onNavigate('/dashboard/resources')}
                              className="px-3 py-1.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold shadow transition"
                            >
                              Open Resource Console
                            </button>
                          </div>
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </div>
            );
          })}

          {loading && (
            <div className="flex gap-3.5 max-w-xl">
              <div className="w-8 h-8 rounded-xl bg-gradient-to-br from-indigo-500 to-purple-600 flex items-center justify-center text-white font-bold text-xs shadow-md animate-pulse">
                <Sparkles className="w-4 h-4" />
              </div>
              <div className="p-4 rounded-2xl bg-slate-900 border border-slate-800 text-slate-400 text-xs flex items-center gap-2.5">
                <div className="w-3 h-3 rounded-full border-2 border-blue-400 border-t-transparent animate-spin" />
                <span>AI Cloud Agent is analyzing requirements and validating cluster capacity...</span>
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>

        {/* Bottom Input Area */}
        <div className="p-4 border-t border-slate-800 bg-slate-950/60">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              handleSendMessage();
            }}
            className="flex items-center gap-2"
          >
            <input
              type="text"
              value={inputText}
              onChange={(e) => setInputText(e.target.value)}
              placeholder="What would you like to build? (e.g. 'Create an Ubuntu VPS with 4GB RAM' or 'Deploy website')..."
              className="flex-1 bg-slate-900 border border-slate-800 rounded-xl px-4 py-3 text-xs sm:text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-blue-500 focus:ring-1 focus:ring-blue-500 transition"
              disabled={loading}
            />
            <button
              type="submit"
              disabled={!inputText.trim() || loading}
              className="p-3 rounded-xl bg-blue-600 hover:bg-blue-500 text-white shadow-lg shadow-blue-500/25 transition disabled:opacity-50"
            >
              <Send className="w-4 h-4" />
            </button>
          </form>
          <div className="flex items-center justify-between text-[11px] text-slate-500 mt-2 px-1">
            <span>Powered by Model Context Protocol (MCP) and Proxmox VE Hardware Orchestration</span>
            <span>State persisted in PostgreSQL</span>
          </div>
        </div>
      </div>
    </div>
  );
};
