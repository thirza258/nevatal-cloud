import React, { useState } from 'react';
import { useAuth } from './context/AuthContext';
import { AppShell } from './components/layout/AppShell';

// Customer Compute & Core Pages
import { Login } from './pages/Login';
import { Register } from './pages/Register';
import { Dashboard } from './pages/Dashboard';
import { InstancesList } from './pages/InstancesList';
import { InstanceDetails } from './pages/InstanceDetails';
import { CreateInstance } from './pages/CreateInstance';
import { JobProgress } from './pages/JobProgress';
import { SSHKeys } from './pages/SSHKeys';
import { UsageQuotas } from './pages/UsageQuotas';
import { ActivityPage } from './pages/ActivityPage';

// Applications (Git & Web Hosting)
import { AppsList } from './pages/apps/AppsList';
import { CreateApp } from './pages/apps/CreateApp';
import { AppDetails } from './pages/apps/AppDetails';

// Workflow Automation (n8n)
import { AutomationList } from './pages/automation/AutomationList';
import { CreateAutomation } from './pages/automation/CreateAutomation';
import { AutomationDetails } from './pages/automation/AutomationDetails';

// AI Personal Assistants (OpenClaw)
import { OpenClawList } from './pages/ai-agents/OpenClawList';
import { CreateOpenClaw } from './pages/ai-agents/CreateOpenClaw';
import { OpenClawDetails } from './pages/ai-agents/OpenClawDetails';

// S3 Object Storage & File Manager
import { StorageOverviewPage } from './pages/storage/StorageOverview';
import { FileManager } from './pages/storage/FileManager';
import { AccessKeys } from './pages/storage/AccessKeys';

// Admin Pages
import { AdminOverview } from './pages/admin/AdminOverview';
import { AdminUsers } from './pages/admin/AdminUsers';
import { AdminInstances } from './pages/admin/AdminInstances';
import { AdminJobs } from './pages/admin/AdminJobs';
import { AdminISOs } from './pages/admin/AdminISOs';
import { AdminTemplates } from './pages/admin/AdminTemplates';
import { AdminPlans } from './pages/admin/AdminPlans';
import { AdminNodes } from './pages/admin/AdminNodes';
import { AdminAudit } from './pages/admin/AdminAudit';

export const App: React.FC = () => {
  const { user, loading } = useAuth();
  const [currentPath, setCurrentPath] = useState<string>('/dashboard');
  const [param, setParam] = useState<string | undefined>(undefined);

  const handleNavigate = (path: string, routeParam?: string) => {
    setCurrentPath(path);
    setParam(routeParam);
    window.scrollTo(0, 0);
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-slate-950 flex items-center justify-center text-slate-400">
        <div className="flex flex-col items-center gap-3">
          <div className="w-8 h-8 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
          <span className="text-xs font-mono">Initializing Nevatal Cloud Control Plane...</span>
        </div>
      </div>
    );
  }

  // Not authenticated view
  if (!user) {
    if (currentPath === '/register') {
      return <Register onNavigate={handleNavigate} />;
    }
    return <Login onNavigate={handleNavigate} />;
  }

  // Authenticated view within AppShell
  return (
    <AppShell currentPath={currentPath} onNavigate={handleNavigate}>
      {/* Customer Compute Routes */}
      {currentPath === '/dashboard' && <Dashboard onNavigate={handleNavigate} />}

      {currentPath === '/dashboard/instances' && !param && (
        <InstancesList onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/instances' && param && (
        <InstanceDetails instanceId={param} onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/instances/new' && (
        <CreateInstance onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/jobs' && param && (
        <JobProgress jobId={param} onNavigate={handleNavigate} />
      )}

      {/* Applications (Git & Web Hosting) */}
      {currentPath === '/dashboard/apps' && !param && (
        <AppsList onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/apps' && param && (
        <AppDetails appId={param} onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/apps/new' && (
        <CreateApp onNavigate={handleNavigate} />
      )}

      {/* Workflow Automation (n8n) */}
      {currentPath === '/dashboard/automation' && !param && (
        <AutomationList onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/automation' && param && (
        <AutomationDetails instanceId={param} onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/automation/new' && (
        <CreateAutomation onNavigate={handleNavigate} />
      )}

      {/* AI Agents (OpenClaw) */}
      {currentPath === '/dashboard/ai-agents' && !param && (
        <OpenClawList onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/ai-agents' && param && (
        <OpenClawDetails instanceId={param} onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/ai-agents/new' && (
        <CreateOpenClaw onNavigate={handleNavigate} />
      )}

      {/* Object Storage (MinIO S3) */}
      {currentPath === '/dashboard/storage' && (
        <StorageOverviewPage onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/storage/files' && param && (
        <FileManager bucketId={param} onNavigate={handleNavigate} />
      )}

      {currentPath === '/dashboard/storage/access-keys' && (
        <AccessKeys onNavigate={handleNavigate} />
      )}

      {/* Account & Administration */}
      {currentPath === '/dashboard/ssh-keys' && <SSHKeys />}
      {currentPath === '/dashboard/usage' && <UsageQuotas />}
      {currentPath === '/dashboard/activity' && <ActivityPage />}

      {/* Admin Routes */}
      {currentPath === '/admin' && <AdminOverview />}
      {currentPath === '/admin/users' && <AdminUsers />}
      {currentPath === '/admin/instances' && <AdminInstances />}
      {currentPath === '/admin/jobs' && <AdminJobs />}
      {currentPath === '/admin/isos' && <AdminISOs />}
      {currentPath === '/admin/templates' && <AdminTemplates />}
      {currentPath === '/admin/plans' && <AdminPlans />}
      {currentPath === '/admin/nodes' && <AdminNodes />}
      {currentPath === '/admin/audit-logs' && <AdminAudit />}
    </AppShell>
  );
};
