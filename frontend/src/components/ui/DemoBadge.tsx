import React from 'react';
import { FlaskConical } from 'lucide-react';
import { useSystemConfig } from '../../context/SystemConfigContext';

interface DemoBadgeProps {
  /** Show the short "Demo" label below the sm breakpoint. */
  compactOnMobile?: boolean;
  className?: string;
}

/** Small amber pill shown only when the backend reports demo_mode=true. */
export const DemoBadge: React.FC<DemoBadgeProps> = ({ compactOnMobile = false, className = '' }) => {
  const { config } = useSystemConfig();
  if (!config.demo_mode) return null;

  const label = config.proxmox_mock ? 'Demo environment · simulated hypervisor' : 'Demo environment';

  return (
    <span
      title={
        config.proxmox_mock
          ? 'This deployment runs in demo mode against a simulated Proxmox hypervisor. Data may be reset.'
          : 'This deployment runs in demo mode. Data may be reset.'
      }
      className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md border border-amber-500/20 bg-amber-500/10 text-[10px] font-mono text-amber-400 whitespace-nowrap select-none ${className}`}
    >
      <FlaskConical className="w-3 h-3 flex-shrink-0" />
      {compactOnMobile ? (
        <>
          <span className="sm:hidden">Demo</span>
          <span className="hidden sm:inline">{label}</span>
        </>
      ) : (
        <span>{label}</span>
      )}
    </span>
  );
};
