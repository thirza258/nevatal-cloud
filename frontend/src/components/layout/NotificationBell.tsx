import React, { useCallback, useEffect, useRef, useState } from 'react';
import { AlertTriangle, Bell, CheckCheck, ChevronRight, Info, RefreshCw } from 'lucide-react';
import { apiRequest } from '../../lib/api';
import type { NotificationItem, NotificationList } from '../../types';
import { errorMessage, formatRelative } from '../../pages/operations/utils';

interface NotificationBellProps {
  onNavigate: (path: string) => void;
}

export const NotificationBell: React.FC<NotificationBellProps> = ({ onNavigate }) => {
  const [open, setOpen] = useState(false);
  const [unread, setUnread] = useState(0);
  const [notifications, setNotifications] = useState<NotificationItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const rootRef = useRef<HTMLDivElement>(null);

  const refreshCount = useCallback(async () => {
    try {
      const res = await apiRequest<{ unread_count: number }>('/notifications/unread-count');
      setUnread(res.unread_count || 0);
    } catch {
      // Notifications are non-critical; keep the last known count.
    }
  }, []);

  useEffect(() => {
    refreshCount();
    const interval = window.setInterval(refreshCount, 30000);
    return () => window.clearInterval(interval);
  }, [refreshCount]);

  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKeyDown);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKeyDown);
    };
  }, [open]);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await apiRequest<NotificationList>('/notifications?limit=30');
      setNotifications(res.notifications || []);
      setUnread(res.unread_count || 0);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  };

  const toggle = () => {
    const next = !open;
    setOpen(next);
    if (next) void load();
  };

  const markRead = async (notification: NotificationItem) => {
    if (!notification.read_at) {
      try {
        await apiRequest(`/notifications/${notification.id}/read`, { method: 'POST' });
        setNotifications((items) =>
          items.map((item) =>
            item.id === notification.id ? { ...item, read_at: new Date().toISOString() } : item
          )
        );
        setUnread((count) => Math.max(0, count - 1));
      } catch {
        // Navigation is still useful if marking read fails.
      }
    }
    if (notification.action_url) {
      setOpen(false);
      onNavigate(notification.action_url);
    }
  };

  const markAll = async () => {
    try {
      await apiRequest('/notifications/read-all', { method: 'POST' });
      setNotifications((items) =>
        items.map((item) => ({ ...item, read_at: item.read_at || new Date().toISOString() }))
      );
      setUnread(0);
    } catch (err) {
      setError(errorMessage(err));
    }
  };

  const severityIcon = (severity: string) => {
    switch (severity) {
      case 'critical':
        return <AlertTriangle className="w-3.5 h-3.5 text-rose-400" />;
      case 'warning':
        return <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />;
      default:
        return <Info className="w-3.5 h-3.5 text-blue-400" />;
    }
  };

  return (
    <div className="relative" ref={rootRef}>
      <button
        onClick={toggle}
        className="relative p-2 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-900 border border-transparent hover:border-zinc-800 transition-colors"
        title="Notifications"
        aria-label={`Notifications${unread > 0 ? `, ${unread} unread` : ''}`}
      >
        <Bell className="w-4 h-4" />
        {unread > 0 && (
          <span className="absolute -top-0.5 -right-0.5 min-w-[15px] h-[15px] px-1 rounded-full bg-rose-500 text-white text-[9px] font-mono font-semibold flex items-center justify-center border border-zinc-950">
            {unread > 99 ? '99+' : unread}
          </span>
        )}
      </button>

      {open && (
        <div className="absolute right-0 mt-2 w-80 sm:w-96 rounded-xl border border-zinc-800 bg-zinc-950 shadow-2xl z-50 overflow-hidden">
          <div className="px-3 py-2.5 border-b border-zinc-800 flex items-center justify-between gap-2">
            <div>
              <div className="text-xs font-semibold text-white">Notifications</div>
              <div className="text-[10px] font-mono text-zinc-500">{unread} unread</div>
            </div>
            <div className="flex items-center gap-1">
              <button
                onClick={load}
                disabled={loading}
                className="p-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-400 hover:text-white border border-zinc-800 disabled:opacity-50"
                title="Refresh notifications"
              >
                <RefreshCw className={`w-3 h-3 ${loading ? 'animate-spin' : ''}`} />
              </button>
              <button
                onClick={markAll}
                disabled={unread === 0}
                className="inline-flex items-center gap-1 px-2 py-1 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-[10px] disabled:opacity-40"
                title="Mark all as read"
              >
                <CheckCheck className="w-3 h-3" />
                Read all
              </button>
            </div>
          </div>

          {error && (
            <div className="px-3 py-2 border-b border-rose-500/20 bg-rose-500/10 text-[10px] text-rose-300">
              {error}
            </div>
          )}

          <div className="max-h-80 overflow-y-auto divide-y divide-zinc-850">
            {loading && notifications.length === 0 ? (
              <div className="p-6 text-center text-[11px] text-zinc-500 flex items-center justify-center gap-2">
                <RefreshCw className="w-3.5 h-3.5 animate-spin" />
                Loading notifications…
              </div>
            ) : notifications.length === 0 ? (
              <div className="p-6 text-center text-[11px] text-zinc-500">
                <Bell className="w-6 h-6 mx-auto mb-2 text-zinc-600" />
                No notifications yet.
              </div>
            ) : (
              notifications.map((notification) => (
                <button
                  key={notification.id}
                  onClick={() => markRead(notification)}
                  className={`w-full text-left px-3 py-2.5 transition hover:bg-zinc-900 ${
                    notification.read_at ? 'opacity-65' : 'bg-zinc-900/30'
                  }`}
                >
                  <div className="flex items-start gap-2">
                    <div className="mt-0.5 flex-shrink-0">{severityIcon(notification.severity)}</div>
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center justify-between gap-2">
                        <span className="text-[11px] font-semibold text-zinc-100 truncate">
                          {notification.title}
                        </span>
                        {!notification.read_at && <span className="w-1.5 h-1.5 rounded-full bg-blue-400 flex-shrink-0" />}
                      </div>
                      {notification.message && (
                        <p className="text-[10px] text-zinc-400 mt-0.5 line-clamp-2">{notification.message}</p>
                      )}
                      <div className="flex items-center justify-between mt-1 text-[9px] font-mono text-zinc-500">
                        <span>{formatRelative(notification.created_at)}</span>
                        {notification.action_url && (
                          <span className="inline-flex items-center gap-0.5 text-zinc-400">
                            open <ChevronRight className="w-2.5 h-2.5" />
                          </span>
                        )}
                      </div>
                    </div>
                  </div>
                </button>
              ))
            )}
          </div>

          <div className="px-3 py-2 border-t border-zinc-800 bg-zinc-950">
            <button
              onClick={() => {
                setOpen(false);
                onNavigate('/dashboard/operations?tab=checks');
              }}
              className="w-full text-center text-[10px] font-mono text-zinc-400 hover:text-white"
            >
              Open operations center
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
