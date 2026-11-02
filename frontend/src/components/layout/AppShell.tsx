import React, { useState } from 'react';
import {
  Server,
  Layers,
  Key,
  PieChart,
  Activity,
  PlusCircle,
  Shield,
  Users,
  HardDrive,
  LogOut,
  Menu,
  X,
  ChevronRight,
  Cpu,
  RefreshCw,
  Zap,
  Globe,
  Workflow,
  Bot,
  Database,
  Disc,
  Sparkles,
  Code,
  Gamepad2,
  Smartphone,
  BookOpen,
  Container,
} from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { useSystemConfig } from '../../context/SystemConfigContext';
import { AIAssistantWidget } from '../ai/AIAssistantWidget';
import { DemoBadge } from '../ui/DemoBadge';

interface AppShellProps {
  currentPath: string;
  onNavigate: (path: string) => void;
  children: React.ReactNode;
}

interface NavItem {
  label: string;
  path: string;
  icon: any;
  highlight?: boolean;
}

export const AppShell: React.FC<AppShellProps> = ({ currentPath, onNavigate, children }) => {
  const { user, logout, switchDemoUser } = useAuth();
  const { config } = useSystemConfig();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const handleSwitchDemoUser = (role: 'admin' | 'customer') => {
    switchDemoUser(role).catch((err) => console.warn('Demo role switch failed:', err));
  };

  const customerNav: NavItem[] = [
    { label: 'Overview', path: '/dashboard', icon: Layers },
    { label: 'AI Cloud Agent', path: '/dashboard/ai', icon: Sparkles, highlight: true },
    { label: 'Cloud Resources', path: '/dashboard/resources', icon: Zap },
    { label: 'MCP Integrations', path: '/dashboard/integrations/mcp', icon: Code },
    { label: 'Compute (VMs)', path: '/dashboard/instances', icon: Server },
    // Public Domains: listed only when the backend config enables them (fail closed).
    ...(config.domains.enabled ? [{ label: 'Domains', path: '/dashboard/domains', icon: Globe }] : []),
    { label: 'App Hosting', path: '/dashboard/apps', icon: Globe },
    { label: 'Automations (n8n)', path: '/dashboard/automation', icon: Workflow },
    { label: 'AI Agents', path: '/dashboard/ai-agents', icon: Bot },
    { label: 'Object Storage', path: '/dashboard/storage', icon: Database },
    { label: 'Game Servers', path: '/dashboard/game-servers', icon: Gamepad2 },
    { label: 'Android Cloud', path: '/dashboard/android', icon: Smartphone },
    { label: 'SSH Keys', path: '/dashboard/ssh-keys', icon: Key },
    { label: 'Usage & Quotas', path: '/dashboard/usage', icon: PieChart },
    { label: 'Activity Log', path: '/dashboard/activity', icon: Activity },
  ];

  const adminNav: NavItem[] = [
    { label: 'Admin Overview', path: '/admin', icon: Shield },
    { label: 'User Directory', path: '/admin/users', icon: Users },
    { label: 'All Instances', path: '/admin/instances', icon: Server },
    { label: 'Docker Viewer', path: '/admin/docker', icon: Container },
    ...(config.domains.enabled ? [{ label: 'Domains', path: '/admin/domains', icon: Globe }] : []),
    { label: 'Provisioning Jobs', path: '/admin/jobs', icon: RefreshCw },
    { label: 'OS ISOs & Media', path: '/admin/isos', icon: Disc },
    { label: 'OS Templates', path: '/admin/templates', icon: Layers },
    { label: 'Resource Plans', path: '/admin/plans', icon: HardDrive },
    { label: 'Hypervisor Nodes', path: '/admin/nodes', icon: Cpu },
    { label: 'Android Images', path: '/admin/android/images', icon: Smartphone },
    { label: 'Audit Logs', path: '/admin/audit-logs', icon: Activity },
  ];

  const handleNav = (path: string) => {
    onNavigate(path);
    setMobileMenuOpen(false);
  };

  return (
    <div className="flex h-screen bg-zinc-950 text-zinc-100 overflow-hidden font-sans">
      {/* Mobile Drawer Backdrop */}
      {mobileMenuOpen && (
        <div
          className="fixed inset-0 z-40 bg-black/80 backdrop-blur-sm lg:hidden"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-60 bg-zinc-950 border-r border-zinc-800/80 flex flex-col transition-transform duration-200 lg:static lg:translate-x-0 ${
          mobileMenuOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        {/* Brand / Logo */}
        <div className="h-14 px-4 flex items-center justify-between border-b border-zinc-800">
          <div
            onClick={() => handleNav('/dashboard')}
            className="flex items-center gap-2.5 cursor-pointer select-none"
          >
            <div className="w-6 h-6 rounded bg-zinc-100 flex items-center justify-center text-zinc-950 font-bold text-xs tracking-tighter">
              NV
            </div>
            <div className="flex items-center gap-1.5">
              <span className="font-semibold text-sm tracking-tight text-white">Nevatal</span>
              <span className="text-[10px] font-mono text-zinc-400 px-1 py-0.2 rounded bg-zinc-900 border border-zinc-800">
                cloud
              </span>
            </div>
          </div>
          <button
            onClick={() => setMobileMenuOpen(false)}
            className="lg:hidden text-zinc-400 hover:text-white"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        {/* Navigation list */}
        <div className="flex-1 overflow-y-auto px-2 py-3 space-y-4">
          <div>
            <div className="px-2 mb-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-400">
              Compute &amp; Cloud
            </div>
            <nav className="space-y-0.5">
              {customerNav.map((item) => {
                const Icon = item.icon;
                const active = currentPath === item.path;
                return (
                  <button
                    key={item.path}
                    onClick={() => handleNav(item.path)}
                    className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors ${
                      active
                        ? 'bg-zinc-800 text-white font-semibold'
                        : 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-200'
                    }`}
                  >
                    <Icon className={`w-3.5 h-3.5 flex-shrink-0 ${active ? 'text-white' : 'text-zinc-400'}`} />
                    <span className="truncate">{item.label}</span>
                  </button>
                );
              })}
            </nav>
          </div>

          {/* Resources & Docs section */}
          <div>
            <div className="px-2 mb-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-400">
              Documentation &amp; Portal
            </div>
            <nav className="space-y-0.5">
              <button
                onClick={() => handleNav('/docs')}
                className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors ${
                  currentPath.startsWith('/docs')
                    ? 'bg-zinc-800 text-white font-semibold'
                    : 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-200'
                }`}
              >
                <BookOpen className="w-3.5 h-3.5 flex-shrink-0 text-zinc-400" />
                <span className="truncate">Documentation</span>
              </button>
              <button
                onClick={() => handleNav('/')}
                className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors ${
                  currentPath === '/'
                    ? 'bg-zinc-800 text-white font-semibold'
                    : 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-200'
                }`}
              >
                <Globe className="w-3.5 h-3.5 flex-shrink-0 text-zinc-400" />
                <span className="truncate">Landing Page</span>
              </button>
            </nav>
          </div>

          {/* Admin section */}
          {user?.role === 'admin' && (
            <div>
              <div className="px-2 mb-1.5 text-[10px] font-mono uppercase tracking-wider text-zinc-400 flex items-center justify-between">
                <span>Infrastructure Admin</span>
                <Shield className="w-3 h-3 text-zinc-400" />
              </div>
              <nav className="space-y-0.5">
                {adminNav.map((item) => {
                  const Icon = item.icon;
                  const active = currentPath === item.path;
                  return (
                    <button
                      key={item.path}
                      onClick={() => handleNav(item.path)}
                      className={`w-full flex items-center gap-2.5 px-2.5 py-1.5 rounded-md text-xs font-medium transition-colors ${
                        active
                          ? 'bg-zinc-800 text-white font-semibold'
                          : 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-200'
                      }`}
                    >
                      <Icon className={`w-3.5 h-3.5 flex-shrink-0 ${active ? 'text-white' : 'text-zinc-400'}`} />
                      <span className="truncate">{item.label}</span>
                    </button>
                  );
                })}
              </nav>
            </div>
          )}
        </div>

        {/* Demo Switcher Quick Bar (only on demo deployments) */}
        {config.demo_mode && (
          <div className="p-2 mx-2 mb-2 bg-zinc-900/40 border border-zinc-800 rounded-lg">
            <div className="text-[10px] font-mono uppercase text-zinc-400 mb-1 flex items-center justify-between">
              <span>Role Switcher (Demo)</span>
              <span className="text-zinc-300 font-mono text-[10px]">{user?.role}</span>
            </div>
            <div className="grid grid-cols-2 gap-1">
              <button
                onClick={() => handleSwitchDemoUser('customer')}
                className={`text-[11px] py-1 px-2 rounded font-medium transition-colors ${
                  user?.role === 'customer'
                    ? 'bg-zinc-800 text-white font-semibold'
                    : 'bg-zinc-900 text-zinc-400 hover:text-white'
                }`}
              >
                Customer
              </button>
              <button
                onClick={() => handleSwitchDemoUser('admin')}
                className={`text-[11px] py-1 px-2 rounded font-medium transition-colors ${
                  user?.role === 'admin'
                    ? 'bg-zinc-800 text-white font-semibold'
                    : 'bg-zinc-900 text-zinc-400 hover:text-white'
                }`}
              >
                Admin
              </button>
            </div>
          </div>
        )}

        {/* User Card & Logout */}
        <div className="p-2.5 border-t border-zinc-800 bg-zinc-950">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2 overflow-hidden">
              <div className="w-7 h-7 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center font-mono text-xs text-zinc-300">
                {user?.display_name?.charAt(0) || 'U'}
              </div>
              <div className="truncate">
                <div className="text-xs font-medium text-white truncate">
                  {user?.display_name}
                </div>
                <div className="text-[10px] text-zinc-400 font-mono truncate">{user?.email}</div>
              </div>
            </div>
            <button
              onClick={logout}
              title="Logout"
              className="text-zinc-400 hover:text-white p-1 rounded-md hover:bg-zinc-900 transition-colors"
            >
              <LogOut className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden bg-zinc-950">
        {/* Top Navbar */}
        <header className="h-14 bg-zinc-950/80 backdrop-blur border-b border-zinc-800 px-4 sm:px-6 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button
              onClick={() => setMobileMenuOpen(true)}
              className="lg:hidden p-1.5 text-zinc-400 hover:text-white rounded-md hover:bg-zinc-900"
            >
              <Menu className="w-4 h-4" />
            </button>
            <div className="flex items-center gap-1.5 text-xs text-zinc-400 font-mono">
              <span className="text-zinc-400">nevatal</span>
              <span className="text-zinc-600">/</span>
              <span className="text-zinc-200 capitalize">
                {currentPath.split('/').pop()?.replace('-', ' ') || 'Dashboard'}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {/* Demo deployment marker (renders nothing unless demo_mode) */}
            <DemoBadge compactOnMobile />

            {/* Cluster / Node Badge */}
            <div className="hidden sm:flex items-center gap-2 px-2.5 py-1 rounded-md bg-zinc-900/60 border border-zinc-800 text-xs font-mono text-zinc-300">
              <div className="w-1.5 h-1.5 rounded-full bg-emerald-500" />
              <span>Dell PowerEdge R640</span>
              <span className="text-zinc-400">·</span>
              <span className="text-zinc-400">Proxmox VE</span>
            </div>

            <button
              onClick={() => handleNav('/docs')}
              className="hidden md:flex items-center gap-1.5 text-xs text-zinc-400 hover:text-white px-2.5 py-1.5 rounded-md hover:bg-zinc-900 border border-transparent hover:border-zinc-800 transition-colors"
              title="Documentation"
            >
              <BookOpen className="w-3.5 h-3.5" />
              <span>Docs</span>
            </button>

            <button
              onClick={() => handleNav('/dashboard/instances/new')}
              className="flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md bg-white text-zinc-950 hover:bg-zinc-200 transition-colors"
            >
              <PlusCircle className="w-3.5 h-3.5" />
              <span className="hidden sm:inline">New Instance</span>
            </button>
          </div>
        </header>

        {/* Scrollable Page Body */}
        <main className="flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8">
          <div className="max-w-7xl mx-auto space-y-6">{children}</div>
        </main>
      </div>

      {/* Global Floating AI Assistant Widget */}
      <AIAssistantWidget currentPath={currentPath} onNavigate={onNavigate} />
    </div>
  );
};
