import React, { useState, useEffect, lazy, Suspense } from 'react';
import { useAuth } from './context/AuthContext';
import { AppShell } from './components/layout/AppShell';
import { FullPageFallback, PageFallback, RouteErrorBoundary } from './components/layout/RouteBoundary';
import { AI_WORKSPACE_PATH, conversationIdFromPath, isAIWorkspacePath } from './lib/aiRoutes';

// Public Landing Page stays in the entry chunk (first paint for most visitors).
import { LandingPage } from './pages/LandingPage';

// Every other route is code-split and loaded on demand.
// Public Documentation & Legal
const DocsPage = lazy(() => import('./pages/docs/DocsPage').then((m) => ({ default: m.DocsPage })));
const LegalPage = lazy(() => import('./pages/LegalPage').then((m) => ({ default: m.LegalPage })));

// Customer Compute & Core Pages
const Login = lazy(() => import('./pages/Login').then((m) => ({ default: m.Login })));
const Register = lazy(() => import('./pages/Register').then((m) => ({ default: m.Register })));
const Dashboard = lazy(() => import('./pages/Dashboard').then((m) => ({ default: m.Dashboard })));
const InstancesList = lazy(() => import('./pages/InstancesList').then((m) => ({ default: m.InstancesList })));
const InstanceDetails = lazy(() => import('./pages/InstanceDetails').then((m) => ({ default: m.InstanceDetails })));
const CreateInstance = lazy(() => import('./pages/CreateInstance').then((m) => ({ default: m.CreateInstance })));
const JobProgress = lazy(() => import('./pages/JobProgress').then((m) => ({ default: m.JobProgress })));
const SSHKeys = lazy(() => import('./pages/SSHKeys').then((m) => ({ default: m.SSHKeys })));
const UsageQuotas = lazy(() => import('./pages/UsageQuotas').then((m) => ({ default: m.UsageQuotas })));
const ActivityPage = lazy(() => import('./pages/ActivityPage').then((m) => ({ default: m.ActivityPage })));

// Public Domains (https://<name>.<root> forwarding to a VM port; each page gates itself on the config)
const DomainsPage = lazy(() => import('./pages/domains/DomainsPage').then((m) => ({ default: m.DomainsPage })));

// Applications (Git & Web Hosting)
const AppsList = lazy(() => import('./pages/apps/AppsList').then((m) => ({ default: m.AppsList })));
const CreateApp = lazy(() => import('./pages/apps/CreateApp').then((m) => ({ default: m.CreateApp })));
const AppDetails = lazy(() => import('./pages/apps/AppDetails').then((m) => ({ default: m.AppDetails })));

// Workflow Automation (n8n)
const AutomationList = lazy(() =>
  import('./pages/automation/AutomationList').then((m) => ({ default: m.AutomationList }))
);
const CreateAutomation = lazy(() =>
  import('./pages/automation/CreateAutomation').then((m) => ({ default: m.CreateAutomation }))
);
const AutomationDetails = lazy(() =>
  import('./pages/automation/AutomationDetails').then((m) => ({ default: m.AutomationDetails }))
);

// AI Personal Assistants (OpenClaw)
const OpenClawList = lazy(() => import('./pages/ai-agents/OpenClawList').then((m) => ({ default: m.OpenClawList })));
const CreateOpenClaw = lazy(() =>
  import('./pages/ai-agents/CreateOpenClaw').then((m) => ({ default: m.CreateOpenClaw }))
);
const OpenClawDetails = lazy(() =>
  import('./pages/ai-agents/OpenClawDetails').then((m) => ({ default: m.OpenClawDetails }))
);

// AI Cloud Assistant (OpenRouter Copilot)
const AIAssistant = lazy(() => import('./pages/AIAssistant').then((m) => ({ default: m.AIAssistant })));

// Nevatal AI Cloud Agent (PRD Section 33-48)
const AICloudAgent = lazy(() => import('./pages/ai/AICloudAgent').then((m) => ({ default: m.AICloudAgent })));
const UnifiedResources = lazy(() =>
  import('./pages/resources/UnifiedResources').then((m) => ({ default: m.UnifiedResources }))
);
const OperationsPage = lazy(() =>
  import('./pages/operations/OperationsPage').then((m) => ({ default: m.OperationsPage }))
);
const MCPRegistry = lazy(() => import('./pages/integrations/MCPRegistry').then((m) => ({ default: m.MCPRegistry })));

// S3 Object Storage & File Manager
const StorageOverviewPage = lazy(() =>
  import('./pages/storage/StorageOverview').then((m) => ({ default: m.StorageOverviewPage }))
);
const FileManager = lazy(() => import('./pages/storage/FileManager').then((m) => ({ default: m.FileManager })));
const AccessKeys = lazy(() => import('./pages/storage/AccessKeys').then((m) => ({ default: m.AccessKeys })));

// Game Server Hosting (PRD Addendum 49-65)
const GameServersList = lazy(() =>
  import('./pages/game-servers/GameServersList').then((m) => ({ default: m.GameServersList }))
);
const CreateGameServer = lazy(() =>
  import('./pages/game-servers/CreateGameServer').then((m) => ({ default: m.CreateGameServer }))
);
const GameServerDetails = lazy(() =>
  import('./pages/game-servers/GameServerDetails').then((m) => ({ default: m.GameServerDetails }))
);

// Managed Waydroid Android Cloud (PRD Addendum 66-83)
const AndroidInstancesList = lazy(() =>
  import('./pages/android/AndroidInstancesList').then((m) => ({ default: m.AndroidInstancesList }))
);
const CreateAndroidInstance = lazy(() =>
  import('./pages/android/CreateAndroidInstance').then((m) => ({ default: m.CreateAndroidInstance }))
);
const AndroidInstanceDetails = lazy(() =>
  import('./pages/android/AndroidInstanceDetails').then((m) => ({ default: m.AndroidInstanceDetails }))
);
const AdminAndroidImages = lazy(() =>
  import('./pages/admin/AdminAndroidImages').then((m) => ({ default: m.AdminAndroidImages }))
);

// Admin Pages
const AdminOverview = lazy(() => import('./pages/admin/AdminOverview').then((m) => ({ default: m.AdminOverview })));
const AdminUsers = lazy(() => import('./pages/admin/AdminUsers').then((m) => ({ default: m.AdminUsers })));
const AdminInstances = lazy(() =>
  import('./pages/admin/AdminInstances').then((m) => ({ default: m.AdminInstances }))
);
const AdminDocker = lazy(() => import('./pages/admin/AdminDocker').then((m) => ({ default: m.AdminDocker })));
const AdminJobs = lazy(() => import('./pages/admin/AdminJobs').then((m) => ({ default: m.AdminJobs })));
const AdminISOs = lazy(() => import('./pages/admin/AdminISOs').then((m) => ({ default: m.AdminISOs })));
const AdminTemplates = lazy(() =>
  import('./pages/admin/AdminTemplates').then((m) => ({ default: m.AdminTemplates }))
);
const AdminPlans = lazy(() => import('./pages/admin/AdminPlans').then((m) => ({ default: m.AdminPlans })));
const AdminNodes = lazy(() => import('./pages/admin/AdminNodes').then((m) => ({ default: m.AdminNodes })));
const AdminAudit = lazy(() => import('./pages/admin/AdminAudit').then((m) => ({ default: m.AdminAudit })));
const AdminDomains = lazy(() => import('./pages/admin/AdminDomains').then((m) => ({ default: m.AdminDomains })));

/** Routing matches on the pathname only; any `?query`/`#hash` is left for the page to read. */
const pathnameOf = (path: string): string => {
  const cut = path.search(/[?#]/);
  return (cut === -1 ? path : path.slice(0, cut)) || '/';
};

export const App: React.FC = () => {
  const { user, loading } = useAuth();
  const [currentPath, setCurrentPath] = useState<string>(() => {
    const pathname = window.location.pathname;
    return pathname && pathname !== '' ? pathname : '/';
  });
  const [param, setParam] = useState<string | undefined>(undefined);

  useEffect(() => {
    const handlePopState = () => {
      const pathname = window.location.pathname;
      setCurrentPath(pathname && pathname !== '' ? pathname : '/');
    };
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  const handleNavigate = (path: string, routeParam?: string) => {
    const pathname = pathnameOf(path);
    const samePage = pathname === window.location.pathname;
    setCurrentPath(pathname);
    setParam(routeParam);
    if (window.location.pathname + window.location.search !== path) {
      window.history.pushState({}, '', path);
      // Only the query string changed (e.g. /admin/docker?instance=…): the routed page stays
      // mounted, so let it re-read the URL exactly as it does on back/forward.
      if (samePage) window.dispatchEvent(new PopStateEvent('popstate'));
    }
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

  // Full-screen public routes: own Suspense boundary (no shell around them).
  const publicRoute = (node: React.ReactNode) => (
    <RouteErrorBoundary key={currentPath} fullScreen>
      <Suspense fallback={<FullPageFallback />}>{node}</Suspense>
    </RouteErrorBoundary>
  );

  // Authenticated routes: the Suspense boundary sits inside the shell's content area, so the
  // sidebar/top bar stay put while a page chunk loads.
  const shellRoute = (shellPath: string, node: React.ReactNode) => (
    <AppShell currentPath={shellPath} onNavigate={handleNavigate}>
      <RouteErrorBoundary key={shellPath}>
        <Suspense fallback={<PageFallback />}>{node}</Suspense>
      </RouteErrorBoundary>
    </AppShell>
  );

  // Public Landing Page
  if (currentPath === '/' || currentPath === '/home' || currentPath === '/landing') {
    return <LandingPage onNavigate={handleNavigate} />;
  }

  // Legal & Compliance Pages
  if (currentPath === '/privacy' || currentPath === '/terms') {
    return publicRoute(
      <LegalPage onNavigate={handleNavigate} initialTab={currentPath === '/terms' ? 'terms' : 'privacy'} />
    );
  }

  // Public Documentation Portal
  if (currentPath.startsWith('/docs')) {
    const cleanPath = currentPath.replace(/\/+$/, '');
    const slug = (cleanPath !== '/docs' ? cleanPath.replace(/^\/docs\//, '') : undefined) || param || 'overview';
    return publicRoute(<DocsPage onNavigate={handleNavigate} initialArticleId={slug} />);
  }

  // Not authenticated view for protected or auth pages
  if (!user) {
    if (currentPath === '/register') {
      return publicRoute(<Register onNavigate={handleNavigate} />);
    }
    return publicRoute(<Login onNavigate={handleNavigate} />);
  }

  // Authenticated redirect from login/register
  if (currentPath === '/login' || currentPath === '/register') {
    return shellRoute('/dashboard', <Dashboard onNavigate={handleNavigate} />);
  }

  // The AI workspace owns /dashboard/ai, /dashboard/ai/new and /dashboard/ai/conversations[/<id>]
  // (deep link: /dashboard/ai/conversations/<conversationId>?plan=<planId>). One shell key for all of
  // them, so switching conversations does not remount the workspace (live execution streams survive).
  const aiWorkspace =
    isAIWorkspacePath(currentPath) &&
    (currentPath === AI_WORKSPACE_PATH ||
      currentPath === '/dashboard/ai/new' ||
      currentPath === '/dashboard/ai/conversations' ||
      currentPath.startsWith('/dashboard/ai/conversations/'));
  const aiConversationId = aiWorkspace ? conversationIdFromPath(currentPath).id ?? param : undefined;

  // Authenticated view within AppShell
  return shellRoute(
    aiWorkspace ? AI_WORKSPACE_PATH : currentPath,
    <>
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

      {/* Public Domains (renders a neutral notice unless the backend enables the feature) */}
      {currentPath === '/dashboard/domains' && <DomainsPage onNavigate={handleNavigate} />}

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

      {/* Nevatal AI Cloud Agent / Infrastructure Orchestrator workspace (PRD Section 33-48) */}
      {aiWorkspace && <AICloudAgent conversationId={aiConversationId} onNavigate={handleNavigate} />}

      {/* Cloud Resources Inventory (PRD Section 34.1) */}
      {currentPath === '/dashboard/resources' && (
        <UnifiedResources onNavigate={handleNavigate} />
      )}

      {/* Operations Center: managed databases, templates, checks, policies, advisor, topology, diagnostics */}
      {currentPath === '/dashboard/operations' && (
        <OperationsPage onNavigate={handleNavigate} />
      )}

      {/* Model Context Protocol (MCP) Server Registry & Connections (PRD Section 37.4) */}
      {currentPath === '/dashboard/integrations/mcp' && (
        <MCPRegistry onNavigate={handleNavigate} />
      )}

      {/* AI Assistant (OpenRouter Copilot) */}
      {currentPath === '/dashboard/ai-assistant' && (
        <AIAssistant onNavigate={handleNavigate} />
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

      {/* Game Server Hosting Routes (PRD Addendum 49-65) */}
      {currentPath === '/dashboard/game-servers' && !param && (
        <GameServersList onNavigate={handleNavigate} />
      )}
      {currentPath === '/dashboard/game-servers' && param && (
        <GameServerDetails serverId={param} onNavigate={handleNavigate} />
      )}
      {currentPath === '/dashboard/game-servers/new' && (
        <CreateGameServer onNavigate={handleNavigate} />
      )}

      {/* Managed Waydroid Android Cloud Routes (PRD Addendum 66-83) */}
      {currentPath === '/dashboard/android' && !param && (
        <AndroidInstancesList onNavigate={handleNavigate} />
      )}
      {currentPath === '/dashboard/android/create' && (
        <CreateAndroidInstance onNavigate={handleNavigate} />
      )}
      {currentPath.startsWith('/dashboard/android/') && currentPath !== '/dashboard/android/create' && (
        <AndroidInstanceDetails
          instanceId={currentPath.replace('/dashboard/android/', '').split('/')[0]}
          initialTab={param || (currentPath.includes('/console') ? 'console' : 'overview')}
          onNavigate={handleNavigate}
        />
      )}
      {currentPath === '/dashboard/android' && param && (
        <AndroidInstanceDetails instanceId={param} onNavigate={handleNavigate} />
      )}

      {/* Account & Administration */}
      {currentPath === '/dashboard/ssh-keys' && <SSHKeys />}
      {currentPath === '/dashboard/usage' && <UsageQuotas />}
      {currentPath === '/dashboard/activity' && <ActivityPage />}

      {/* Admin Routes */}
      {currentPath === '/admin' && <AdminOverview />}
      {currentPath === '/admin/users' && <AdminUsers />}
      {currentPath === '/admin/instances' && <AdminInstances onNavigate={handleNavigate} />}
      {currentPath === '/admin/docker' && <AdminDocker onNavigate={handleNavigate} />}
      {currentPath === '/admin/domains' && <AdminDomains onNavigate={handleNavigate} />}
      {currentPath === '/admin/jobs' && <AdminJobs />}
      {currentPath === '/admin/isos' && <AdminISOs />}
      {currentPath === '/admin/templates' && <AdminTemplates onNavigate={handleNavigate} />}
      {currentPath === '/admin/plans' && <AdminPlans />}
      {currentPath === '/admin/nodes' && <AdminNodes />}
      {currentPath === '/admin/android/images' && <AdminAndroidImages />}
      {currentPath === '/admin/audit-logs' && <AdminAudit />}
    </>
  );
};
