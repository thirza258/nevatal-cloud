import React, { useState, useEffect, useRef } from 'react';
import {
  Bot,
  Sparkles,
  Send,
  Terminal,
  Key,
  Cpu,
  HardDrive,
  Layers,
  Shield,
  CheckCircle2,
  AlertCircle,
  ExternalLink,
  Trash2,
  Settings2,
  ArrowRight,
  RefreshCw,
  Check,
  Copy,
  X,
  Server,
  Zap,
  HelpCircle,
  Globe,
  Sliders,
  ClipboardCheck,
} from 'lucide-react';
import { apiRequest } from '../lib/api';
import { planLinkPath } from '../lib/aiRoutes';
import {
  AIAssistantMessage,
  AIAssistantActionProposal,
  AIAssistantChatResponse,
  AIAssistantExecuteResult,
  AIPlatformContext,
  OpenRouterModelDefinition,
} from '../types';

interface AIAssistantProps {
  onNavigate: (path: string, param?: string) => void;
}

export const AIAssistant: React.FC<AIAssistantProps> = ({ onNavigate }) => {
  const [messages, setMessages] = useState<AIAssistantMessage[]>([]);
  const [inputText, setInputText] = useState('');
  const [loading, setLoading] = useState(false);
  const [contextData, setContextData] = useState<AIPlatformContext | null>(null);
  const [initialLoading, setInitialLoading] = useState(true);

  // OpenRouter Settings
  const [selectedModel, setSelectedModel] = useState<string>('openai/gpt-4o-mini');
  const [customApiKey, setCustomApiKey] = useState<string>(() => localStorage.getItem('nevatal_openrouter_key') || '');
  const [showSettingsModal, setShowSettingsModal] = useState(false);
  const [tempApiKey, setTempApiKey] = useState('');
  const [testingKey, setTestingKey] = useState(false);
  const [keyTestStatus, setKeyTestStatus] = useState<{ valid?: boolean; message?: string } | null>(null);

  // Executing Action State
  const [executingMessageId, setExecutingMessageId] = useState<string | null>(null);
  const [actionSuccess, setActionSuccess] = useState<{ [key: string]: string }>({});
  const [actionError, setActionError] = useState<{ [key: string]: string }>({});

  // Clear history: inline two-step confirmation and error (no browser dialogs).
  const [confirmClear, setConfirmClear] = useState(false);
  const [clearing, setClearing] = useState(false);
  const [clearError, setClearError] = useState<string | null>(null);

  // Mode Filter
  const [activeTab, setActiveTab] = useState<'all' | 'containers' | 'support'>('all');

  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    scrollToBottom();
  }, [messages, loading]);

  // Load Context & Chat History
  const loadData = async () => {
    try {
      setInitialLoading(true);
      const [ctx, hist] = await Promise.all([
        apiRequest<AIPlatformContext>('/ai-assistant/context'),
        apiRequest<AIAssistantMessage[]>('/ai-assistant/history'),
      ]);

      setContextData(ctx);
      if (ctx?.default_model && !selectedModel) {
        setSelectedModel(ctx.default_model);
      }

      if (hist && hist.length > 0) {
        setMessages(hist);
      } else {
        // Welcome message if fresh
        setMessages([
          {
            id: 'welcome',
            user_id: ctx?.user?.id || '',
            role: 'assistant',
            content: `👋 **Welcome to Nevatal Cloud AI Assistant!**\n\nI am powered by **OpenRouter** to help you manage your Dell PowerEdge R640 Proxmox cluster.\n\nHere are some things you can ask me:\n- 🚀 **"Create an Ubuntu 24.04 container with 2 vCPUs and 4GB RAM"**\n- ⚡ **"Deploy a lightweight Alpine Linux instance"**\n- 🌐 **"Spin up WordPress 6.7 LEMP stack"**\n- 📊 **"What are my current quotas and remaining capacity?"**\n- 🔑 **"How do I connect to my virtual servers via SSH?"**\n- 🛠️ **"Help me troubleshoot a stopped or failed VM"**`,
            model: ctx?.default_model || 'openai/gpt-4o-mini',
            action_executed: false,
            created_at: new Date().toISOString(),
          },
        ]);
      }
    } catch (err) {
      console.error('Failed to load AI Assistant data:', err);
    } finally {
      setInitialLoading(false);
    }
  };

  useEffect(() => {
    loadData();
  }, []);

  // Send Chat Message
  const handleSendMessage = async (textToSend?: string) => {
    const text = (textToSend || inputText).trim();
    if (!text || loading) return;

    setInputText('');
    setLoading(true);

    // Optimistically add user message to list
    const tempUserMsg: AIAssistantMessage = {
      id: `temp-${Date.now()}`,
      user_id: contextData?.user?.id || '',
      role: 'user',
      content: text,
      model: selectedModel,
      action_executed: false,
      created_at: new Date().toISOString(),
    };
    setMessages((prev) => [...prev, tempUserMsg]);

    try {
      const response = await apiRequest<AIAssistantChatResponse>('/ai-assistant/chat', {
        method: 'POST',
        headers: customApiKey ? { 'X-OpenRouter-Key': customApiKey } : undefined,
        body: JSON.stringify({
          message: text,
          model: selectedModel,
          api_key: customApiKey || undefined,
        }),
      });

      if (response && response.message) {
        setMessages((prev) => [
          ...prev.filter((m) => m.id !== tempUserMsg.id),
          { ...tempUserMsg, id: `user-${response.message.id}` },
          response.message,
        ]);
      }
    } catch (err: any) {
      console.error('Chat error:', err);
      // Append error message from assistant
      const errMsg: AIAssistantMessage = {
        id: `err-${Date.now()}`,
        user_id: contextData?.user?.id || '',
        role: 'assistant',
        content: `⚠️ **Error communicating with AI Assistant**: ${err.message || 'Failed to generate response'}. Please verify your OpenRouter API key in settings or try again.`,
        model: selectedModel,
        action_executed: false,
        created_at: new Date().toISOString(),
      };
      setMessages((prev) => [...prev, errMsg]);
    } finally {
      setLoading(false);
    }
  };

  // "Review plan": the action card becomes an orchestrator plan (a second click returns the same
  // plan) and the AI workspace opens on it. Nothing is created until the user approves the plan.
  const handleReviewPlan = async (msg: AIAssistantMessage) => {
    if (!msg.action_payload || executingMessageId) return;

    let params: any;
    try {
      params = JSON.parse(msg.action_payload);
    } catch {
      setActionError((prev) => ({ ...prev, [msg.id]: 'Invalid action payload format' }));
      return;
    }

    setExecutingMessageId(msg.id);
    setActionError((prev) => ({ ...prev, [msg.id]: '' }));

    try {
      const res = await apiRequest<AIAssistantExecuteResult>('/ai-assistant/execute', {
        method: 'POST',
        body: JSON.stringify({
          message_id: msg.id,
          action: msg.action_type || 'create_instance',
          params: params,
        }),
      });

      setActionSuccess((prev) => ({
        ...prev,
        [msg.id]:
          res.message || 'Plan created. Nothing is created until you approve it in the AI workspace.',
      }));
      setMessages((prev) =>
        prev.map((m) => (m.id === msg.id ? { ...m, action_executed: true } : m))
      );
      onNavigate(planLinkPath(res.url, res.plan_id));
    } catch (err: any) {
      console.error('Review plan error:', err);
      setActionError((prev) => ({
        ...prev,
        [msg.id]: `Could not create the plan: ${err?.message || 'unknown error'}`,
      }));
    } finally {
      setExecutingMessageId(null);
    }
  };

  // Clear Chat History
  const handleClearHistory = async () => {
    setClearing(true);
    setClearError(null);
    try {
      await apiRequest('/ai-assistant/history', { method: 'DELETE' });
      setMessages([]);
      setConfirmClear(false);
      loadData();
    } catch (err: any) {
      setClearError(`Failed to clear history: ${err?.message || 'unknown error'}`);
    } finally {
      setClearing(false);
    }
  };

  // Test OpenRouter Key
  const handleTestKey = async () => {
    if (!tempApiKey.trim()) return;
    setTestingKey(true);
    setKeyTestStatus(null);
    try {
      const res = await apiRequest<{ valid: boolean; data?: any }>('/ai-assistant/verify-key', {
        method: 'POST',
        body: JSON.stringify({ api_key: tempApiKey.trim() }),
      });
      if (res.valid) {
        setKeyTestStatus({ valid: true, message: 'Valid OpenRouter API Key! Connected successfully.' });
      } else {
        setKeyTestStatus({ valid: false, message: 'Invalid API Key or unauthorized.' });
      }
    } catch (err: any) {
      setKeyTestStatus({ valid: false, message: err.message || 'Verification failed' });
    } finally {
      setTestingKey(false);
    }
  };

  const handleSaveKeySettings = () => {
    if (tempApiKey.trim()) {
      localStorage.setItem('nevatal_openrouter_key', tempApiKey.trim());
      setCustomApiKey(tempApiKey.trim());
    } else {
      localStorage.removeItem('nevatal_openrouter_key');
      setCustomApiKey('');
    }
    setShowSettingsModal(false);
    setKeyTestStatus(null);
  };

  // Quick Action Prompts
  const quickPrompts = [
    {
      label: 'Deploy Ubuntu 24.04',
      prompt: 'Create an Ubuntu 24.04 server container with 2 vCPUs and 4GB RAM',
      icon: Server,
      color: 'text-blue-400 bg-blue-500/10 border-blue-500/20',
    },
    {
      label: 'Deploy Alpine Micro',
      prompt: 'Deploy a lightweight Alpine Linux micro-container with 1 vCPU and 1GB RAM',
      icon: Zap,
      color: 'text-amber-400 bg-amber-500/10 border-amber-500/20',
    },
    {
      label: 'WordPress LEMP Stack',
      prompt: 'Create a WordPress 6.7 LEMP stack container with 2 vCPUs and 4GB RAM',
      icon: Globe,
      color: 'text-purple-400 bg-purple-500/10 border-purple-500/20',
    },
    {
      label: 'Check Quota & Capacity',
      prompt: 'What are my current resource quotas and remaining capacity on the cluster?',
      icon: Cpu,
      color: 'text-emerald-400 bg-emerald-500/10 border-emerald-500/20',
    },
    {
      label: 'SSH Connection Help',
      prompt: 'How do I connect to my instances via SSH with my registered keys?',
      icon: Key,
      color: 'text-cyan-400 bg-cyan-500/10 border-cyan-500/20',
    },
    {
      label: 'Hypervisor Specs (R640)',
      prompt: 'Explain the Dell PowerEdge R640 Proxmox VE hypervisor architecture',
      icon: Layers,
      color: 'text-rose-400 bg-rose-500/10 border-rose-500/20',
    },
  ];

  if (initialLoading) {
    return (
      <div className="flex flex-col items-center justify-center py-24 text-slate-400 space-y-3">
        <div className="w-8 h-8 border-2 border-purple-500 border-t-transparent rounded-full animate-spin" />
        <span className="text-sm font-medium">Connecting to Nevatal AI DevOps Assistant...</span>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-[calc(100vh-7.5rem)] max-w-6xl mx-auto">
      {/* Header Bar */}
      <div className="bg-slate-900/80 border border-slate-800 rounded-2xl p-4 mb-4 backdrop-blur shadow-xl">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
          <div className="flex items-center gap-3">
            <div className="w-10 h-10 rounded-xl bg-gradient-to-tr from-purple-600 to-blue-600 flex items-center justify-center text-white shadow-lg shadow-purple-500/20">
              <Bot className="w-5 h-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="text-lg font-bold text-white tracking-tight">AI DevOps Assistant</h1>
                <span className="text-[11px] font-semibold px-2 py-0.5 rounded-full bg-purple-500/10 text-purple-400 border border-purple-500/20 flex items-center gap-1">
                  <Sparkles className="w-3 h-3" />
                  OpenRouter
                </span>
              </div>
              <p className="text-xs text-slate-400">
                Container provisioning copilot and platform technical support powered by OpenRouter LLMs.
              </p>
            </div>
          </div>

          {/* Right Controls: Model Selector & Settings */}
          <div className="flex items-center gap-2">
            {/* Model Selector */}
            <div className="relative">
              <select
                value={selectedModel}
                onChange={(e) => setSelectedModel(e.target.value)}
                className="bg-slate-950 border border-slate-700/80 rounded-xl px-3 py-1.5 text-xs text-slate-200 font-medium focus:outline-none focus:border-purple-500 appearance-none pr-7 cursor-pointer"
              >
                {(contextData?.available_models || []).map((m) => (
                  <option key={m.id} value={m.id}>
                    {m.name} ({m.provider})
                  </option>
                ))}
              </select>
              <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center px-2 text-slate-400">
                <Sliders className="w-3 h-3" />
              </div>
            </div>

            {/* API Key Status / Config Button */}
            <button
              onClick={() => {
                setTempApiKey(customApiKey);
                setShowSettingsModal(true);
              }}
              className={`flex items-center gap-1.5 px-3 py-1.5 rounded-xl text-xs font-medium border transition cursor-pointer ${
                customApiKey || contextData?.has_server_api_key
                  ? 'bg-slate-800 hover:bg-slate-700 text-slate-200 border-slate-700'
                  : 'bg-amber-500/10 hover:bg-amber-500/20 text-amber-300 border-amber-500/30'
              }`}
              title="Configure OpenRouter API Key"
            >
              <Key className="w-3.5 h-3.5" />
              <span className="hidden md:inline">
                {customApiKey
                  ? 'Custom Key'
                  : contextData?.has_server_api_key
                  ? 'System Key'
                  : 'Add API Key'}
              </span>
            </button>

            {/* Clear History (two-step: the first click asks inline) */}
            {confirmClear ? (
              <div className="flex items-center gap-1.5 text-[11px]" role="group" aria-label="Confirm clearing the chat history">
                <span className="text-slate-300">Clear chat history?</span>
                <button
                  onClick={handleClearHistory}
                  disabled={clearing}
                  className="px-2 py-1 rounded-lg bg-rose-500/20 hover:bg-rose-500/30 text-rose-300 border border-rose-500/30 font-semibold transition cursor-pointer disabled:opacity-50"
                >
                  {clearing ? 'Clearing...' : 'Clear'}
                </button>
                <button
                  onClick={() => {
                    setConfirmClear(false);
                    setClearError(null);
                  }}
                  disabled={clearing}
                  className="px-2 py-1 rounded-lg bg-slate-800/80 hover:bg-slate-700 text-slate-300 border border-slate-700/80 transition cursor-pointer disabled:opacity-50"
                >
                  Cancel
                </button>
              </div>
            ) : (
              <button
                onClick={() => {
                  setClearError(null);
                  setConfirmClear(true);
                }}
                title="Clear Conversation"
                className="p-2 rounded-xl bg-slate-800/80 hover:bg-rose-500/10 text-slate-400 hover:text-rose-400 border border-slate-700/80 transition cursor-pointer"
              >
                <Trash2 className="w-3.5 h-3.5" />
              </button>
            )}
          </div>
        </div>

        {clearError && (
          <div
            role="alert"
            className="mt-3 p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2"
          >
            <AlertCircle className="w-4 h-4 flex-shrink-0" />
            <span>{clearError}</span>
          </div>
        )}

        {/* Quota Glance Bar */}
        {contextData?.quota_usage && (
          <div className="mt-3 pt-3 border-t border-slate-800/80 flex flex-wrap items-center gap-4 text-[11px] text-slate-400">
            <span className="font-semibold text-slate-300 flex items-center gap-1">
              <Cpu className="w-3.5 h-3.5 text-blue-400" />
              vCPUs: {contextData.quota_usage.used_vcpus} / {contextData.quota_usage.max_vcpus}
            </span>
            <span className="font-semibold text-slate-300 flex items-center gap-1">
              <HardDrive className="w-3.5 h-3.5 text-purple-400" />
              RAM: {(contextData.quota_usage.used_memory_mb / 1024).toFixed(1)} /{' '}
              {(contextData.quota_usage.max_memory_mb / 1024).toFixed(0)} GB
            </span>
            <span className="font-semibold text-slate-300 flex items-center gap-1">
              <Server className="w-3.5 h-3.5 text-emerald-400" />
              Instances: {contextData.instances.length} / {contextData.quota_usage.max_instances}
            </span>
            <span className="ml-auto text-slate-500 hidden sm:inline">
              Cluster: Dell PowerEdge R640 (Proxmox VE 8.x)
            </span>
          </div>
        )}
      </div>

      {/* Chat Messages Container */}
      <div className="flex-1 bg-slate-900/40 border border-slate-800/80 rounded-2xl p-4 sm:p-5 overflow-y-auto space-y-4 backdrop-blur shadow-inner">
        {messages.map((msg, index) => {
          const isUser = msg.role === 'user';
          let proposal: AIAssistantActionProposal | null = null;
          if (msg.action_payload) {
            try {
              proposal = {
                action: (msg.action_type as any) || 'create_instance',
                title: msg.action_type === 'create_instance' ? 'Recommended Container' : 'Infrastructure Action',
                description:
                  'Review plan creates a plan you review and approve in the AI workspace. Nothing is created until you approve it.',
                params: JSON.parse(msg.action_payload),
                executed: msg.action_executed,
              };
            } catch {
              // ignore json parse error
            }
          }

          return (
            <div
              key={msg.id || index}
              className={`flex gap-3 max-w-4xl ${isUser ? 'ml-auto flex-row-reverse' : 'mr-auto'}`}
            >
              {/* Avatar */}
              <div
                className={`w-8 h-8 rounded-xl flex-shrink-0 flex items-center justify-center text-xs font-bold shadow ${
                  isUser
                    ? 'bg-blue-600 text-white shadow-blue-500/20'
                    : 'bg-gradient-to-tr from-purple-600 to-blue-600 text-white shadow-purple-500/20'
                }`}
              >
                {isUser ? contextData?.user?.display_name?.charAt(0) || 'U' : <Bot className="w-4 h-4" />}
              </div>

              {/* Message Content Bubble */}
              <div
                className={`flex-1 rounded-2xl p-4 text-xs sm:text-sm leading-relaxed ${
                  isUser
                    ? 'bg-blue-600 text-white shadow-md'
                    : 'bg-slate-900 border border-slate-800 text-slate-200 shadow-sm'
                }`}
              >
                {/* Header for Assistant */}
                {!isUser && (
                  <div className="flex items-center justify-between gap-2 mb-2 pb-2 border-b border-slate-800 text-[11px] text-slate-400">
                    <span className="font-semibold text-purple-400 flex items-center gap-1">
                      <Sparkles className="w-3 h-3" />
                      Nevatal Cloud Copilot
                    </span>
                    <span className="font-mono text-[10px] text-slate-500">{msg.model}</span>
                  </div>
                )}

                {/* Body Formatted */}
                <div className="prose prose-invert prose-xs max-w-none space-y-2 whitespace-pre-wrap font-sans">
                  {msg.content}
                </div>

                {/* Action Proposal Card (e.g. Container Creation) */}
                {proposal && proposal.action === 'create_instance' && (
                  <div className="mt-4 p-4 rounded-xl bg-slate-950/80 border border-purple-500/30 shadow-lg space-y-3">
                    <div className="flex items-center justify-between gap-2 border-b border-slate-800/80 pb-2.5">
                      <div className="flex items-center gap-2">
                        <div className="p-1.5 rounded-lg bg-purple-500/10 text-purple-400 border border-purple-500/20">
                          <Server className="w-4 h-4" />
                        </div>
                        <div>
                          <div className="text-xs font-bold text-white tracking-tight">
                            {proposal.title || 'Container proposal'}
                          </div>
                          <div className="text-[11px] text-slate-400">
                            {proposal.params?.template_name || 'Linux Container Template'}
                          </div>
                        </div>
                      </div>

                      {msg.action_executed ? (
                        <span className="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20 flex items-center gap-1">
                          <Check className="w-3 h-3" />
                          Plan created
                        </span>
                      ) : (
                        <span className="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-purple-500/10 text-purple-400 border border-purple-500/20">
                          Proposal
                        </span>
                      )}
                    </div>

                    {/* Sizing Grid */}
                    <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-[11px]">
                      <div className="p-2 rounded-lg bg-slate-900/60 border border-slate-800">
                        <span className="text-slate-500 block text-[10px] uppercase">Name</span>
                        <span className="font-mono font-medium text-slate-200 truncate block">
                          {proposal.params?.name}
                        </span>
                      </div>
                      <div className="p-2 rounded-lg bg-slate-900/60 border border-slate-800">
                        <span className="text-slate-500 block text-[10px] uppercase">Compute</span>
                        <span className="font-mono font-medium text-slate-200">
                          {proposal.params?.vcpus} vCPU
                        </span>
                      </div>
                      <div className="p-2 rounded-lg bg-slate-900/60 border border-slate-800">
                        <span className="text-slate-500 block text-[10px] uppercase">Memory</span>
                        <span className="font-mono font-medium text-slate-200">
                          {(proposal.params?.memory_mb / 1024).toFixed(1)} GB
                        </span>
                      </div>
                      <div className="p-2 rounded-lg bg-slate-900/60 border border-slate-800">
                        <span className="text-slate-500 block text-[10px] uppercase">Storage</span>
                        <span className="font-mono font-medium text-slate-200">
                          {proposal.params?.disk_gb} GB NVMe
                        </span>
                      </div>
                    </div>

                    <p className="text-[11px] text-slate-400 leading-relaxed">
                      {msg.action_executed
                        ? 'A plan was created for this proposal. Nothing is created until you approve it in the AI workspace.'
                        : proposal.description}
                    </p>

                    {proposal.params?.ssh_key_name && (
                      <div className="text-[11px] text-slate-400 flex items-center gap-1.5">
                        <Key className="w-3 h-3 text-cyan-400" />
                        <span>SSH Key:</span>
                        <span className="text-slate-200 font-mono">{proposal.params.ssh_key_name}</span>
                      </div>
                    )}

                    {/* Error Banner */}
                    {actionError[msg.id] && (
                      <div className="p-2.5 rounded-lg bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2">
                        <AlertCircle className="w-4 h-4 flex-shrink-0" />
                        <span>{actionError[msg.id]}</span>
                      </div>
                    )}

                    {/* Success Banner */}
                    {actionSuccess[msg.id] && (
                      <div className="p-2.5 rounded-lg bg-emerald-500/10 border border-emerald-500/20 text-emerald-400 text-xs flex items-center gap-2">
                        <CheckCircle2 className="w-4 h-4 flex-shrink-0" />
                        <span>{actionSuccess[msg.id]}</span>
                      </div>
                    )}

                    {/* Action Execution Buttons */}
                    <div className="flex items-center justify-end gap-2 pt-1">
                      <button
                        onClick={() => onNavigate('/dashboard/instances/new')}
                        className="px-3 py-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition cursor-pointer flex items-center gap-1"
                      >
                        <span>Open in Wizard</span>
                        <ArrowRight className="w-3 h-3" />
                      </button>

                      <button
                        onClick={() => handleReviewPlan(msg)}
                        disabled={executingMessageId === msg.id}
                        className="px-4 py-1.5 rounded-lg bg-gradient-to-r from-purple-600 to-blue-600 hover:from-purple-500 hover:to-blue-500 text-white text-xs font-semibold shadow-md shadow-purple-500/20 transition cursor-pointer flex items-center gap-1.5 disabled:opacity-50"
                      >
                        {executingMessageId === msg.id ? (
                          <>
                            <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                            <span>Preparing plan...</span>
                          </>
                        ) : (
                          <>
                            <ClipboardCheck className="w-3.5 h-3.5" />
                            <span>Review plan</span>
                          </>
                        )}
                      </button>
                    </div>
                  </div>
                )}
              </div>
            </div>
          );
        })}

        {loading && (
          <div className="flex gap-3 items-center mr-auto text-slate-400 text-xs bg-slate-900 border border-slate-800 rounded-2xl px-4 py-3">
            <div className="w-5 h-5 rounded-lg bg-purple-600 flex items-center justify-center text-white">
              <Bot className="w-3 h-3" />
            </div>
            <div className="flex items-center gap-2">
              <RefreshCw className="w-3.5 h-3.5 animate-spin text-purple-400" />
              <span>Thinking with OpenRouter ({selectedModel})...</span>
            </div>
          </div>
        )}

        <div ref={messagesEndRef} />
      </div>

      {/* Quick Prompts Bar */}
      <div className="py-2.5 overflow-x-auto no-scrollbar">
        <div className="flex items-center gap-2 min-w-max">
          <span className="text-[11px] font-semibold text-slate-500 uppercase tracking-wider flex items-center gap-1 pr-1">
            <Sparkles className="w-3 h-3 text-purple-400" />
            Suggested:
          </span>
          {quickPrompts.map((qp, idx) => {
            const Icon = qp.icon;
            return (
              <button
                key={idx}
                onClick={() => handleSendMessage(qp.prompt)}
                className={`inline-flex items-center gap-1.5 px-3 py-1 rounded-xl text-xs font-medium border transition cursor-pointer hover:scale-105 active:scale-95 ${qp.color}`}
              >
                <Icon className="w-3 h-3" />
                <span>{qp.label}</span>
              </button>
            );
          })}
        </div>
      </div>

      {/* Input Box */}
      <div className="bg-slate-900/90 border border-slate-800 rounded-2xl p-2.5 shadow-xl backdrop-blur">
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
            placeholder="Ask me to create a container, analyze quotas, or troubleshoot your cloud instances..."
            disabled={loading}
            className="flex-1 bg-transparent px-3 py-2 text-sm text-slate-100 placeholder-slate-500 focus:outline-none"
          />
          <button
            type="submit"
            disabled={loading || !inputText.trim()}
            className="px-4 py-2 rounded-xl bg-gradient-to-r from-purple-600 to-blue-600 hover:from-purple-500 hover:to-blue-500 text-white font-medium text-xs shadow-md shadow-purple-500/20 transition cursor-pointer flex items-center gap-1.5 disabled:opacity-40 disabled:cursor-not-allowed"
          >
            <span>Send</span>
            <Send className="w-3.5 h-3.5" />
          </button>
        </form>
      </div>

      {/* OpenRouter API Key Settings Modal */}
      {showSettingsModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-slate-950/80 backdrop-blur-sm">
          <div className="w-full max-w-md bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-2xl space-y-4">
            <div className="flex items-center justify-between border-b border-slate-800 pb-3">
              <div className="flex items-center gap-2">
                <div className="p-2 rounded-lg bg-purple-500/10 text-purple-400 border border-purple-500/20">
                  <Key className="w-4 h-4" />
                </div>
                <div>
                  <h3 className="text-base font-bold text-white tracking-tight">OpenRouter API Key</h3>
                  <p className="text-xs text-slate-400">Configure key for live multi-provider reasoning</p>
                </div>
              </div>
              <button
                onClick={() => setShowSettingsModal(false)}
                className="text-slate-400 hover:text-white p-1 rounded-lg hover:bg-slate-800 transition"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            <div className="space-y-3">
              <p className="text-xs text-slate-300 leading-relaxed">
                Connect your personal OpenRouter account to unlock frontier models like Claude 3.5 Sonnet, GPT-4o, or DeepSeek V3 for infrastructure provisioning and support.
              </p>

              <div>
                <label className="block text-xs font-semibold text-slate-300 mb-1">
                  OpenRouter API Key
                </label>
                <input
                  type="password"
                  value={tempApiKey}
                  onChange={(e) => setTempApiKey(e.target.value)}
                  placeholder="sk-or-v1-••••••••••••••••"
                  className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-xl text-xs font-mono text-slate-200 focus:outline-none focus:border-purple-500"
                />
                <span className="text-[11px] text-slate-500 mt-1 block">
                  Keys are stored in your secure local session. Leave empty to use system default.
                </span>
              </div>

              {keyTestStatus && (
                <div
                  className={`p-3 rounded-xl text-xs flex items-center gap-2 ${
                    keyTestStatus.valid
                      ? 'bg-emerald-500/10 border border-emerald-500/20 text-emerald-400'
                      : 'bg-rose-500/10 border border-rose-500/20 text-rose-400'
                  }`}
                >
                  {keyTestStatus.valid ? (
                    <CheckCircle2 className="w-4 h-4 flex-shrink-0" />
                  ) : (
                    <AlertCircle className="w-4 h-4 flex-shrink-0" />
                  )}
                  <span>{keyTestStatus.message}</span>
                </div>
              )}

              <div className="flex items-center justify-between text-xs pt-2">
                <a
                  href="https://openrouter.ai/keys"
                  target="_blank"
                  rel="noreferrer"
                  className="text-purple-400 hover:underline flex items-center gap-1"
                >
                  <span>Get OpenRouter API Key</span>
                  <ExternalLink className="w-3 h-3" />
                </a>

                {tempApiKey.trim() && (
                  <button
                    type="button"
                    onClick={handleTestKey}
                    disabled={testingKey}
                    className="px-2.5 py-1 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs transition cursor-pointer"
                  >
                    {testingKey ? 'Verifying...' : 'Test Connection'}
                  </button>
                )}
              </div>
            </div>

            <div className="flex items-center justify-end gap-2 pt-3 border-t border-slate-800">
              <button
                type="button"
                onClick={() => setShowSettingsModal(false)}
                className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-medium transition cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={handleSaveKeySettings}
                className="px-4 py-2 rounded-xl bg-purple-600 hover:bg-purple-500 text-white text-xs font-medium transition shadow-lg shadow-purple-500/20 cursor-pointer"
              >
                Save Settings
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
