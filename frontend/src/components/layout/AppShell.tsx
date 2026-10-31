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
} from 'lucide-react';
import { useAuth } from '../../context/AuthContext';
import { AIAssistantWidget } from '../ai/AIAssistantWidget';

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
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  const customerNav: NavItem[] = [
    { label: 'Overview', path: '/dashboard', icon: Layers },
    { label: 'AI Cloud Agent', path: '/dashboard/ai', icon: Sparkles, highlight: true },
    { label: 'Cloud Resources', path: '/dashboard/resources', icon: Zap },
    { label: 'MCP Integrations', path: '/dashboard/integrations/mcp', icon: Code },
    { label: 'Compute (VMs)', path: '/dashboard/instances', icon: Server },
    { label: 'App Hosting', path: '/dashboard/apps', icon: Globe },
    { label: 'Automations (n8n)', path: '/dashboard/automation', icon: Workflow },
    { label: 'AI Agents', path: '/dashboard/ai-agents', icon: Bot },
    { label: 'Object Storage', path: '/dashboard/storage', icon: Database },
    { label: 'SSH Keys', path: '/dashboard/ssh-keys', icon: Key },
    { label: 'Usage & Quotas', path: '/dashboard/usage', icon: PieChart },
    { label: 'Activity Log', path: '/dashboard/activity', icon: Activity },
  ];

  const adminNav: NavItem[] = [
    { label: 'Admin Overview', path: '/admin', icon: Shield },
    { label: 'User Directory', path: '/admin/users', icon: Users },
    { label: 'All Instances', path: '/admin/instances', icon: Server },
    { label: 'Provisioning Jobs', path: '/admin/jobs', icon: RefreshCw },
    { label: 'OS ISOs & Media', path: '/admin/isos', icon: Disc },
    { label: 'OS Templates', path: '/admin/templates', icon: Layers },
    { label: 'Resource Plans', path: '/admin/plans', icon: HardDrive },
    { label: 'Hypervisor Nodes', path: '/admin/nodes', icon: Cpu },
    { label: 'Audit Logs', path: '/admin/audit-logs', icon: Activity },
  ];

  const handleNav = (path: string) => {
    onNavigate(path);
    setMobileMenuOpen(false);
  };

  return (
    <div className="flex h-screen bg-slate-950 text-slate-100 overflow-hidden font-sans">
      {/* Mobile Drawer Backdrop */}
      {mobileMenuOpen && (
        <div
          className="fixed inset-0 z-40 bg-slate-950/80 backdrop-blur-sm lg:hidden"
          onClick={() => setMobileMenuOpen(false)}
        />
      )}

      {/* Sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 bg-slate-900 border-r border-slate-800/80 flex flex-col transition-transform duration-200 lg:static lg:translate-x-0 ${
          mobileMenuOpen ? 'translate-x-0' : '-translate-x-full'
        }`}
      >
        {/* Brand / Logo */}
        <div className="h-16 px-6 flex items-center justify-between border-b border-slate-800">
          <div
            onClick={() => handleNav('/dashboard')}
            className="flex items-center gap-3 cursor-pointer select-none"
          >
            <div className="w-8 h-8 rounded-lg bg-blue-600 flex items-center justify-center text-white font-bold shadow-lg shadow-blue-500/20">
              <Zap className="w-4 h-4" />
            </div>
            <div>
              <span className="font-bold text-base tracking-tight text-white">Nevatal</span>
              <span className="ml-1 text-xs font-semibold px-1.5 py-0.5 rounded bg-blue-500/10 text-blue-400 border border-blue-500/20">
                CLOUD
              </span>
            </div>
          </div>
          <button
            onClick={() => setMobileMenuOpen(false)}
            className="lg:hidden text-slate-400 hover:text-white"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Navigation list */}
        <div className="flex-1 overflow-y-auto px-3 py-4 space-y-6">
          <div>
            <div className="px-3 mb-2 text-[10px] font-semibold uppercase tracking-wider text-slate-500">
              Compute & Cloud
            </div>
            <nav className="space-y-1">
              {customerNav.map((item) => {
                const Icon = item.icon;
                const active = currentPath === item.path;
                return (
                  <button
                    key={item.path}
                    onClick={() => handleNav(item.path)}
                    className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-sm font-medium transition-all ${
                      active
                        ? 'bg-blue-600 text-white shadow-md shadow-blue-600/20'
                        : item.highlight
                        ? 'text-blue-400 hover:bg-slate-800/80 hover:text-white'
                        : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
                    }`}
                  >
                    <Icon className="w-4 h-4 flex-shrink-0" />
                    <span>{item.label}</span>
                  </button>
                );
              })}
            </nav>
          </div>

          {/* Admin section */}
          {user?.role === 'admin' && (
            <div>
              <div className="px-3 mb-2 text-[10px] font-semibold uppercase tracking-wider text-amber-500/80 flex items-center justify-between">
                <span>Infrastructure Admin</span>
                <Shield className="w-3 h-3 text-amber-400" />
              </div>
              <nav className="space-y-1">
                {adminNav.map((item) => {
                  const Icon = item.icon;
                  const active = currentPath === item.path;
                  return (
                    <button
                      key={item.path}
                      onClick={() => handleNav(item.path)}
                      className={`w-full flex items-center gap-3 px-3 py-2 rounded-xl text-sm font-medium transition-all ${
                        active
                          ? 'bg-amber-600 text-white shadow-md shadow-amber-600/20'
                          : 'text-slate-400 hover:bg-slate-800/60 hover:text-slate-200'
                      }`}
                    >
                      <Icon className="w-4 h-4 flex-shrink-0" />
                      <span>{item.label}</span>
                    </button>
                  );
                })}
              </nav>
            </div>
          )}
        </div>

        {/* Demo Switcher Quick Bar */}
        <div className="p-3 mx-3 mb-2 bg-slate-950/60 border border-slate-800 rounded-xl">
          <div className="text-[10px] uppercase font-semibold text-slate-500 mb-1.5 flex items-center justify-between">
            <span>Role Switcher (Demo)</span>
            <span className="text-[10px] text-blue-400 font-mono">{user?.role}</span>
          </div>
          <div className="grid grid-cols-2 gap-1.5">
            <button
              onClick={() => switchDemoUser('customer')}
              className={`text-xs py-1 px-2 rounded-lg font-medium transition-colors ${
                user?.role === 'customer'
                  ? 'bg-blue-600 text-white'
                  : 'bg-slate-800 text-slate-400 hover:text-white'
              }`}
            >
              Customer
            </button>
            <button
              onClick={() => switchDemoUser('admin')}
              className={`text-xs py-1 px-2 rounded-lg font-medium transition-colors ${
                user?.role === 'admin'
                  ? 'bg-amber-600 text-white'
                  : 'bg-slate-800 text-slate-400 hover:text-white'
              }`}
            >
              Admin
            </button>
          </div>
        </div>

        {/* User Card & Logout */}
        <div className="p-3 border-t border-slate-800 bg-slate-900/40">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2.5 overflow-hidden">
              <div className="w-8 h-8 rounded-full bg-slate-800 border border-slate-700 flex items-center justify-center font-bold text-xs text-blue-400">
                {user?.display_name?.charAt(0) || 'U'}
              </div>
              <div className="truncate">
                <div className="text-xs font-semibold text-white truncate">
                  {user?.display_name}
                </div>
                <div className="text-[11px] text-slate-500 truncate">{user?.email}</div>
              </div>
            </div>
            <button
              onClick={logout}
              title="Logout"
              className="text-slate-400 hover:text-rose-400 p-1.5 rounded-lg hover:bg-slate-800 transition-colors"
            >
              <LogOut className="w-4 h-4" />
            </button>
          </div>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Top Navbar */}
        <header className="h-16 bg-slate-900/60 backdrop-blur border-b border-slate-800 px-4 sm:px-6 flex items-center justify-between">
          <div className="flex items-center gap-3">
            <button
              onClick={() => setMobileMenuOpen(true)}
              className="lg:hidden p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800"
            >
              <Menu className="w-5 h-5" />
            </button>
            <div className="flex items-center gap-2 text-xs font-medium text-slate-400">
              <span>Nevatal Cloud</span>
              <ChevronRight className="w-3.5 h-3.5 text-slate-600" />
              <span className="text-white capitalize">
                {currentPath.split('/').pop()?.replace('-', ' ') || 'Dashboard'}
              </span>
            </div>
          </div>

          <div className="flex items-center gap-3">
            {/* Cluster / Node Badge */}
            <div className="hidden sm:flex items-center gap-2 px-3 py-1 rounded-full bg-slate-800/70 border border-slate-700/60 text-xs">
              <div className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse" />
              <span className="text-slate-300 font-medium">Dell PowerEdge R640</span>
              <span className="text-slate-500 font-mono text-[11px]">Proxmox VE</span>
            </div>

            <button
              onClick={() => handleNav('/dashboard/instances/new')}
              className="flex items-center gap-1.5 text-xs font-semibold px-3 py-1.5 rounded-lg bg-blue-600 hover:bg-blue-500 text-white shadow-sm transition-colors"
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
