import React, { useCallback, useEffect, useState } from 'react';
import {
  Activity,
  Bot,
  Database,
  Files,
  Layers,
  Network,
  ShieldCheck,
  Sparkles,
  Stethoscope,
} from 'lucide-react';
import { DatabasesTab } from './DatabasesTab';
import { TemplatesTab } from './TemplatesTab';
import { ChecksTab } from './ChecksTab';
import { PoliciesTab } from './PoliciesTab';
import { AdvisorTab } from './AdvisorTab';
import { TopologyTab } from './TopologyTab';
import { RolesTab } from './RolesTab';
import { DiagnosticsPanel } from './DiagnosticsPanel';

type TabID = 'databases' | 'templates' | 'checks' | 'policies' | 'advisor' | 'topology' | 'roles' | 'diagnostics';

interface OperationsPageProps {
  onNavigate: (path: string, param?: string) => void;
}

const TABS: { id: TabID; label: string; icon: React.ComponentType<{ className?: string }> }[] = [
  { id: 'databases', label: 'Databases', icon: Database },
  { id: 'templates', label: 'Templates', icon: Layers },
  { id: 'checks', label: 'Checks', icon: Activity },
  { id: 'policies', label: 'Policies', icon: ShieldCheck },
  { id: 'advisor', label: 'Advisor', icon: Sparkles },
  { id: 'topology', label: 'Topology', icon: Network },
  { id: 'roles', label: 'Agent roles', icon: Bot },
  { id: 'diagnostics', label: 'Diagnostics', icon: Stethoscope },
];

const isTab = (value: string | null): value is TabID => TABS.some((tab) => tab.id === value);

const readLocation = () => {
  const params = new URLSearchParams(window.location.search);
  const rawTab = params.get('tab');
  return {
    tab: (isTab(rawTab) ? rawTab : 'databases') as TabID,
    type: params.get('type') || undefined,
    id: params.get('id') || undefined,
    name: params.get('name') || undefined,
  };
};

export const OperationsPage: React.FC<OperationsPageProps> = ({ onNavigate }) => {
  const [location, setLocation] = useState(readLocation);

  useEffect(() => {
    const onPopState = () => setLocation(readLocation());
    window.addEventListener('popstate', onPopState);
    return () => window.removeEventListener('popstate', onPopState);
  }, []);

  const writeLocation = useCallback((next: ReturnType<typeof readLocation>) => {
    const params = new URLSearchParams();
    params.set('tab', next.tab);
    if (next.type) params.set('type', next.type);
    if (next.id) params.set('id', next.id);
    if (next.name) params.set('name', next.name);
    const url = `${window.location.pathname}?${params.toString()}`;
    if (url !== window.location.pathname + window.location.search) {
      window.history.replaceState({}, '', url);
    }
    setLocation(next);
  }, []);

  const changeTab = (tab: TabID) => {
    if (tab === 'diagnostics' && location.type && location.id) {
      writeLocation({ ...location, tab });
    } else {
      writeLocation({ tab, type: undefined, id: undefined, name: undefined });
    }
  };

  const openDiagnostics = (type: string, id: string, name: string) => {
    writeLocation({ tab: 'diagnostics', type, id, name });
  };

  return (
    <div className="space-y-5">
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-zinc-800 pb-5">
        <div>
          <div className="flex items-center gap-2">
            <h1 className="text-xl sm:text-2xl font-semibold text-white tracking-tight">Operations</h1>
            <span className="px-2 py-0.5 rounded-md bg-zinc-900 border border-zinc-800 text-zinc-400 font-mono text-[10px]">
              phases 2–4
            </span>
          </div>
          <p className="text-xs text-zinc-400 mt-1 font-mono">
            Managed databases, reusable templates, checks, policies, advisor, topology and diagnostics
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={() => onNavigate('/dashboard/resources')}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-zinc-900 hover:bg-zinc-850 text-zinc-300 border border-zinc-800 text-xs font-medium transition"
          >
            <Files className="w-3.5 h-3.5" />
            Cloud resources
          </button>
          <button
            onClick={() => onNavigate('/dashboard/ai')}
            className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white hover:bg-zinc-200 text-zinc-950 text-xs font-semibold transition"
          >
            <Sparkles className="w-3.5 h-3.5" />
            AI Cloud Agent
          </button>
        </div>
      </div>

      <div className="flex items-center gap-1 overflow-x-auto pb-1">
        {TABS.map((tab) => {
          const Icon = tab.icon;
          const active = location.tab === tab.id;
          return (
            <button
              key={tab.id}
              onClick={() => changeTab(tab.id)}
              className={`flex items-center gap-1.5 px-2.5 py-1.5 rounded-md text-xs font-medium whitespace-nowrap transition ${
                active
                  ? 'bg-zinc-800 text-white font-semibold'
                  : 'text-zinc-400 hover:text-white hover:bg-zinc-900'
              }`}
            >
              <Icon className="w-3.5 h-3.5" />
              {tab.label}
            </button>
          );
        })}
      </div>

      {location.tab === 'databases' && <DatabasesTab onNavigate={onNavigate} onDiagnose={openDiagnostics} />}
      {location.tab === 'templates' && <TemplatesTab onNavigate={onNavigate} />}
      {location.tab === 'checks' && <ChecksTab onNavigate={onNavigate} />}
      {location.tab === 'policies' && <PoliciesTab onNavigate={onNavigate} />}
      {location.tab === 'advisor' && <AdvisorTab onNavigate={onNavigate} />}
      {location.tab === 'topology' && <TopologyTab onNavigate={onNavigate} />}
      {location.tab === 'roles' && <RolesTab />}
      {location.tab === 'diagnostics' && (
        <DiagnosticsPanel
          onNavigate={onNavigate}
          initialType={location.type}
          initialId={location.id}
          initialName={location.name}
        />
      )}
    </div>
  );
};
