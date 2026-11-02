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

// Pages with their own full-height chat: the floating bubble would cover their composer.
const HIDDEN_ON = new Set(['/dashboard/ai-assistant', '/dashboard/ai']);

/**
 * Hides the widget on HIDDEN_ON paths. The check lives in this wrapper (not before the hooks of
 * the widget itself) so navigating to or from those pages never changes the hook order.
 */
export const AIAssistantWidget: React.FC<AIAssistantWidgetProps> = (props) =>
  HIDDEN_ON.has(props.currentPath) ? null : <AIAssistantWidgetPanel {...props} />;

const AIAssistantWidgetPanel: React.FC<AIAssistantWidgetProps> = ({ onNavigate }) => {
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
          className="fixed bottom-5 right-5 z-40 px-3 py-2 rounded-full bg-zinc-900 hover:bg-zinc-850 text-zinc-300 hover:text-white border border-zinc-750 shadow-lg text-xs font-medium flex items-center gap-2 transition-all cursor-pointer"
          title="Open AI Assistant"
        >
          <Bot className="w-3.5 h-3.5 text-zinc-300" />
          <span className="text-xs">AI Assistant</span>
        </button>
      )}

      {/* Floating Chat Drawer */}
      {isOpen && (
        <div className="fixed bottom-5 right-5 z-50 w-[92vw] sm:w-[390px] h-[520px] max-h-[85vh] bg-zinc-950 border border-zinc-800 rounded-xl shadow-2xl flex flex-col overflow-hidden animate-in fade-in duration-150">
          {/* Header */}
          <div className="p-3 border-b border-zinc-850 bg-zinc-900/60 flex items-center justify-between">
            <div className="flex items-center gap-2.5">
              <div className="w-7 h-7 rounded-md bg-zinc-800 border border-zinc-700 flex items-center justify-center text-zinc-200">
                <Bot className="w-3.5 h-3.5" />
              </div>
              <div>
                <div className="flex items-center gap-1.5">
                  <span className="text-xs font-semibold text-white tracking-tight">AI Cloud Assistant</span>
                  <span className="text-[10px] font-mono text-zinc-400 px-1 py-0.2 rounded bg-zinc-900 border border-zinc-800">
                    copilot
                  </span>
                </div>
                <span className="text-[10px] text-zinc-400 font-mono block">Proxmox VE &amp; Instances</span>
              </div>
            </div>

            <div className="flex items-center gap-1">
              <button
                onClick={() => {
                  setIsOpen(false);
                  onNavigate('/dashboard/ai-assistant');
                }}
                title="Expand to Full Page"
                className="p-1.5 text-zinc-400 hover:text-white rounded-md hover:bg-zinc-900 transition cursor-pointer"
              >
                <Maximize2 className="w-3.5 h-3.5" />
              </button>
              <button
                onClick={() => setIsOpen(false)}
                className="p-1.5 text-zinc-400 hover:text-white rounded-md hover:bg-zinc-900 transition cursor-pointer"
              >
                <X className="w-3.5 h-3.5" />
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
                    className={`p-2.5 rounded-lg leading-relaxed whitespace-pre-wrap ${
                      isUser
                        ? 'bg-zinc-800 text-white'
                        : 'bg-zinc-900 border border-zinc-800 text-zinc-200'
                    }`}
                  >
                    <div>{m.content}</div>

                    {/* Action Card inside Widget */}
                    {params && (
                      <div className="mt-2 p-2 rounded bg-zinc-950 border border-zinc-800 space-y-1.5">
                        <div className="flex items-center justify-between text-[11px] font-mono text-zinc-300">
                          <span className="flex items-center gap-1 text-zinc-300">
                            <Server className="w-3 h-3 text-zinc-400" />
                            {params.name || 'new-container'}
                          </span>
                          <span className="text-[10px] text-zinc-500">
                            {params.vcpus} vCPU • {(params.memory_mb / 1024).toFixed(1)}GB
                          </span>
                        </div>

                        {!m.action_executed ? (
                          <button
                            onClick={() => handleExecuteAction(m)}
                            disabled={executingMsgId === m.id}
                            className="w-full py-1 rounded bg-white hover:bg-zinc-200 text-zinc-950 font-medium text-[11px] transition flex items-center justify-center gap-1 cursor-pointer disabled:opacity-50"
                          >
                            {executingMsgId === m.id ? (
                              <>
                                <RefreshCw className="w-3 h-3 animate-spin" />
                                <span>Deploying...</span>
                              </>
                            ) : (
                              <>
                                <Play className="w-3 h-3 fill-current" />
                                <span>Deploy Container</span>
                              </>
                            )}
                          </button>
                        ) : (
                          <div className="text-[10px] text-emerald-400 flex items-center gap-1 font-mono">
                            <CheckCircle2 className="w-3 h-3" />
                            <span>Queued</span>
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              );
            })}

            {loading && (
              <div className="flex items-center gap-2 text-zinc-500 text-xs py-1 px-2 font-mono">
                <RefreshCw className="w-3 h-3 animate-spin" />
                <span>Thinking...</span>
              </div>
            )}

            {statusNotice && (
              <div className="p-2 rounded bg-zinc-900 border border-zinc-800 text-zinc-300 text-[11px] flex items-center gap-1.5">
                <CheckCircle2 className="w-3.5 h-3.5 text-zinc-400 flex-shrink-0" />
                <span>{statusNotice}</span>
              </div>
            )}

            <div ref={messagesEndRef} />
          </div>

          {/* Quick Prompts */}
          <div className="px-2.5 py-1.5 border-t border-zinc-850 bg-zinc-950 flex items-center gap-1.5 overflow-x-auto no-scrollbar">
            <button
              onClick={() => handleSend('Create an Ubuntu 24.04 container with 2 vCPUs and 4GB RAM')}
              className="text-[10px] whitespace-nowrap px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-zinc-400 hover:text-white hover:bg-zinc-850 transition cursor-pointer font-mono"
            >
              + Ubuntu Container
            </button>
            <button
              onClick={() => handleSend('How do I connect to instances via SSH?')}
              className="text-[10px] whitespace-nowrap px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-zinc-400 hover:text-white hover:bg-zinc-850 transition cursor-pointer font-mono"
            >
              SSH Help
            </button>
            <button
              onClick={() => handleSend('What are my current quotas and remaining capacity?')}
              className="text-[10px] whitespace-nowrap px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-zinc-400 hover:text-white hover:bg-zinc-850 transition cursor-pointer font-mono"
            >
              Quotas
            </button>
          </div>

          {/* Input Footer */}
          <div className="p-2 border-t border-zinc-850 bg-zinc-950">
            <form
              onSubmit={(e) => {
                e.preventDefault();
                handleSend();
              }}
              className="flex items-center gap-1.5"
            >
              <input
                type="text"
                value={inputText}
                onChange={(e) => setInputText(e.target.value)}
                placeholder="Ask or plan infrastructure..."
                disabled={loading}
                className="flex-1 bg-zinc-900 border border-zinc-800 rounded-md px-2.5 py-1.5 text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-700"
              />
              <button
                type="submit"
                disabled={loading || !inputText.trim()}
                className="p-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 disabled:opacity-40 transition cursor-pointer"
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
