import React, { useState, useEffect, useRef } from 'react';
import {
  Bot,
  Sparkles,
  X,
  Send,
  Maximize2,
  RefreshCw,
  Server,
  Play,
  Key,
  Cpu,
  ArrowRight,
  CheckCircle2,
  AlertCircle,
  Zap,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { AIAssistantMessage, AIAssistantChatResponse, AIAssistantActionProposal } from '../../types';

interface AIAssistantWidgetProps {
  onNavigate: (path: string, param?: string) => void;
  currentPath: string;
}

export const AIAssistantWidget: React.FC<AIAssistantWidgetProps> = ({ onNavigate, currentPath }) => {
  // If we are already on the dedicated /dashboard/ai-assistant page, don't show the widget
  if (currentPath === '/dashboard/ai-assistant') {
    return null;
  }

  const [isOpen, setIsOpen] = useState(false);
  const [messages, setMessages] = useState<AIAssistantMessage[]>([]);
  const [inputText, setInputText] = useState('');
  const [loading, setLoading] = useState(false);
  const [executingMsgId, setExecutingMsgId] = useState<string | null>(null);
  const [statusNotice, setStatusNotice] = useState<string | null>(null);

  const messagesEndRef = useRef<HTMLDivElement>(null);

  const scrollToBottom = () => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  };

  useEffect(() => {
    if (isOpen) {
      scrollToBottom();
      if (messages.length === 0) {
        // Load recent history or set welcome
        loadHistory();
      }
    }
  }, [isOpen, messages.length]);

  const loadHistory = async () => {
    try {
      const hist = await apiRequest<AIAssistantMessage[]>('/ai-assistant/history');
      if (hist && hist.length > 0) {
        setMessages(hist.slice(-6)); // Show last 6 messages
      } else {
        setMessages([
          {
            id: 'widget-welcome',
            user_id: '',
            role: 'assistant',
            content: `👋 **Hi! Need help with Nevatal Cloud?**\n\nI can create containers, analyze quotas, or troubleshoot your instances directly from here!\n\nTry asking:\n- *"Create an Ubuntu 24.04 container"* \n- *"How do I connect via SSH?"*`,
            model: 'openai/gpt-4o-mini',
            action_executed: false,
            created_at: new Date().toISOString(),
          },
        ]);
      }
    } catch {
      // fallback
    }
  };

  const handleSend = async (quickText?: string) => {
    const text = (quickText || inputText).trim();
    if (!text || loading) return;

    setInputText('');
    setLoading(true);

    const tempUserMsg: AIAssistantMessage = {
      id: `widget-temp-${Date.now()}`,
      user_id: '',
      role: 'user',
      content: text,
      model: 'openai/gpt-4o-mini',
      action_executed: false,
      created_at: new Date().toISOString(),
    };

    setMessages((prev) => [...prev, tempUserMsg]);

    const customKey = localStorage.getItem('nevatal_openrouter_key') || undefined;

    try {
      const res = await apiRequest<AIAssistantChatResponse>('/ai-assistant/chat', {
        method: 'POST',
        headers: customKey ? { 'X-OpenRouter-Key': customKey } : undefined,
        body: JSON.stringify({
          message: text,
          api_key: customKey,
        }),
      });

      if (res && res.message) {
        setMessages((prev) => [
          ...prev.filter((m) => m.id !== tempUserMsg.id),
          { ...tempUserMsg, id: `user-${res.message.id}` },
          res.message,
        ]);
      }
    } catch (err: any) {
      setMessages((prev) => [
        ...prev,
        {
          id: `err-${Date.now()}`,
          user_id: '',
          role: 'assistant',
          content: `⚠️ ${err.message || 'Error communicating with assistant.'}`,
          model: 'openai/gpt-4o-mini',
          action_executed: false,
          created_at: new Date().toISOString(),
        },
      ]);
    } finally {
      setLoading(false);
    }
  };

  const handleExecuteAction = async (msg: AIAssistantMessage) => {
    if (!msg.action_payload || executingMsgId) return;
    setExecutingMsgId(msg.id);
    setStatusNotice(null);

    try {
      const params = JSON.parse(msg.action_payload);
      const res = await apiRequest<any>('/ai-assistant/execute', {
        method: 'POST',
        body: JSON.stringify({
          message_id: msg.id,
          action: msg.action_type || 'create_instance',
          params: params,
        }),
      });

      setStatusNotice(`Provisioning queued! Job ID: ${res.job_id || 'active'}`);
      setMessages((prev) =>
        prev.map((m) => (m.id === msg.id ? { ...m, action_executed: true } : m))
      );

      if (res.job_id) {
        setTimeout(() => {
          setIsOpen(false);
          onNavigate('/dashboard/jobs', res.job_id);
        }, 1200);
      }
    } catch (err: any) {
      setStatusNotice(`Failed: ${err.message}`);
    } finally {
      setExecutingMsgId(null);
    }
  };

  return (
    <>
      {/* Floating Toggle Button */}
      {!isOpen && (
        <button
          onClick={() => setIsOpen(true)}
          className="fixed bottom-6 right-6 z-40 p-3.5 rounded-2xl bg-gradient-to-tr from-purple-600 to-blue-600 text-white shadow-xl shadow-purple-500/30 hover:scale-105 active:scale-95 transition-all duration-200 cursor-pointer flex items-center gap-2 group border border-purple-400/30"
          title="Open AI Assistant"
        >
          <div className="relative">
            <Bot className="w-6 h-6" />
            <span className="absolute -top-1 -right-1 w-2.5 h-2.5 rounded-full bg-emerald-400 border-2 border-slate-900 animate-pulse" />
          </div>
          <span className="text-xs font-semibold tracking-tight pr-1 hidden sm:inline">
            AI Assistant
          </span>
        </button>
      )}

      {/* Floating Chat Drawer */}
      {isOpen && (
        <div className="fixed bottom-6 right-6 z-50 w-[92vw] sm:w-[420px] h-[550px] max-h-[85vh] bg-slate-900/95 border border-slate-700/80 rounded-2xl shadow-2xl flex flex-col overflow-hidden backdrop-blur-xl animate-in fade-in slide-in-from-bottom-4 duration-200">
          {/* Header */}
          <div className="p-3.5 border-b border-slate-800 bg-slate-950/60 flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <div className="w-8 h-8 rounded-xl bg-gradient-to-tr from-purple-600 to-blue-600 flex items-center justify-center text-white shadow-md shadow-purple-500/20">
                <Bot className="w-4 h-4" />
              </div>
              <div>
                <div className="flex items-center gap-1.5">
                  <span className="text-xs font-bold text-white tracking-tight">AI Cloud Copilot</span>
                  <span className="text-[10px] font-semibold px-1.5 py-0.2 rounded bg-purple-500/10 text-purple-400 border border-purple-500/20">
                    OpenRouter
                  </span>
                </div>
                <span className="text-[10px] text-slate-400 block">Proxmox VE & Container Assistant</span>
              </div>
            </div>

            <div className="flex items-center gap-1">
              <button
                onClick={() => {
                  setIsOpen(false);
                  onNavigate('/dashboard/ai-assistant');
                }}
                title="Expand to Full Page"
                className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition cursor-pointer"
              >
                <Maximize2 className="w-4 h-4" />
              </button>
              <button
                onClick={() => setIsOpen(false)}
                className="p-1.5 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition cursor-pointer"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>

          {/* Messages */}
          <div className="flex-1 p-3.5 overflow-y-auto space-y-3 text-xs">
            {messages.map((m, idx) => {
              const isUser = m.role === 'user';
              let params: any = null;
              if (m.action_payload && m.action_type === 'create_instance') {
                try {
                  params = JSON.parse(m.action_payload);
                } catch {
                  // ignore
                }
              }

              return (
                <div
                  key={m.id || idx}
                  className={`flex gap-2 max-w-[90%] ${isUser ? 'ml-auto flex-row-reverse' : 'mr-auto'}`}
                >
                  <div
                    className={`p-3 rounded-xl leading-relaxed whitespace-pre-wrap ${
                      isUser
                        ? 'bg-blue-600 text-white rounded-tr-none'
                        : 'bg-slate-950 border border-slate-800 text-slate-200 rounded-tl-none'
                    }`}
                  >
                    <div>{m.content}</div>

                    {/* Action Card inside Widget */}
                    {params && (
                      <div className="mt-2.5 p-2.5 rounded-lg bg-slate-900 border border-purple-500/30 space-y-2">
                        <div className="flex items-center justify-between text-[11px] font-semibold text-white">
                          <span className="flex items-center gap-1 text-purple-400">
                            <Server className="w-3.5 h-3.5" />
                            {params.name || 'new-container'}
                          </span>
                          <span className="text-[10px] text-slate-400">
                            {params.vcpus} vCPU • {(params.memory_mb / 1024).toFixed(1)}GB
                          </span>
                        </div>

                        {!m.action_executed ? (
                          <button
                            onClick={() => handleExecuteAction(m)}
                            disabled={executingMsgId === m.id}
                            className="w-full py-1.5 rounded-lg bg-purple-600 hover:bg-purple-500 text-white font-medium text-[11px] transition flex items-center justify-center gap-1 cursor-pointer disabled:opacity-50"
                          >
                            {executingMsgId === m.id ? (
                              <>
                                <RefreshCw className="w-3 h-3 animate-spin" />
                                <span>Deploying...</span>
                              </>
                            ) : (
                              <>
                                <Play className="w-3 h-3 fill-current" />
                                <span>Deploy Container Now</span>
                              </>
                            )}
                          </button>
                        ) : (
                          <div className="text-[10px] text-emerald-400 flex items-center gap-1 font-semibold">
                            <CheckCircle2 className="w-3.5 h-3.5" />
                            <span>Provisioning Queued ✓</span>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}

            {loading && (
              <div className="flex items-center gap-2 text-slate-400 text-xs py-2 px-3 bg-slate-950/60 rounded-xl w-fit">
                <RefreshCw className="w-3 h-3 animate-spin text-purple-400" />
                <span>Generating response...</span>
              </div>
            )}

            {statusNotice && (
              <div className="p-2 rounded-lg bg-purple-500/10 border border-purple-500/20 text-purple-300 text-[11px] flex items-center gap-1.5">
                <Sparkles className="w-3.5 h-3.5 flex-shrink-0" />
                <span>{statusNotice}</span>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>

          {/* Quick Prompts */}
          <div className="px-3 py-1.5 border-t border-slate-800/80 bg-slate-950/40 flex items-center gap-1.5 overflow-x-auto no-scrollbar">
            <button
              onClick={() => handleSend('Create an Ubuntu 24.04 container with 2 vCPUs and 4GB RAM')}
              className="text-[10px] whitespace-nowrap px-2 py-0.5 rounded-md bg-blue-500/10 text-blue-400 border border-blue-500/20 hover:bg-blue-500/20 cursor-pointer"
            >
              + Ubuntu Container
            </button>
            <button
              onClick={() => handleSend('How do I connect to instances via SSH?')}
              className="text-[10px] whitespace-nowrap px-2 py-0.5 rounded-md bg-cyan-500/10 text-cyan-400 border border-cyan-500/20 hover:bg-cyan-500/20 cursor-pointer"
            >
              SSH Help
            </button>
            <button
              onClick={() => handleSend('What are my current quotas and remaining capacity?')}
              className="text-[10px] whitespace-nowrap px-2 py-0.5 rounded-md bg-purple-500/10 text-purple-400 border border-purple-500/20 hover:bg-purple-500/20 cursor-pointer"
            >
              Check Quota
            </button>
          </div>

          {/* Input Footer */}
          <div className="p-2.5 border-t border-slate-800 bg-slate-950">
            <form
              onSubmit={(e) => {
                e.preventDefault();
                handleSend();
              }}
              className="flex items-center gap-2"
            >
              <input
                type="text"
                value={inputText}
                onChange={(e) => setInputText(e.target.value)}
                placeholder="Ask to create container or support..."
                disabled={loading}
                className="flex-1 bg-slate-900 border border-slate-800 rounded-xl px-3 py-1.5 text-xs text-white placeholder-slate-500 focus:outline-none focus:border-purple-500"
              />
              <button
                type="submit"
                disabled={loading || !inputText.trim()}
                className="p-2 rounded-xl bg-purple-600 hover:bg-purple-500 text-white disabled:opacity-40 transition cursor-pointer"
              >
                <Send className="w-3.5 h-3.5" />
              </button>
            </form>
          </div>
        </div>
      )}
    </>
  );
};
