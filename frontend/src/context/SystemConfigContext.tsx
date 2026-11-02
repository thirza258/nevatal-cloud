import React, { createContext, useContext, useEffect, useState } from 'react';
import { SystemConfig } from '../types';
import { apiRequest } from '../lib/api';

/**
 * Fail-closed defaults used while the config is loading and whenever it cannot be
 * fetched or parsed. Demo affordances (role switcher, quick logins, demo badge) and
 * social sign-in must never render unless the backend explicitly enables them, so
 * nothing demo-related can flash on a production deployment.
 */
export const SAFE_SYSTEM_CONFIG: SystemConfig = {
  environment: 'production',
  demo_mode: false,
  proxmox_mock: false,
  version: '',
  features: {
    docker_viewer: false,
    domains: false,
  },
  domains: {
    enabled: false,
    root: '',
    max_per_user: 0,
  },
  auth: {
    password_login: true,
    social_login: false,
    registration: true,
  },
};

export type SystemConfigStatus = 'loading' | 'ready' | 'error';

interface SystemConfigState {
  config: SystemConfig;
  status: SystemConfigStatus;
}

// Default value doubles as the fail-closed fallback for consumers rendered outside the provider.
const SystemConfigContext = createContext<SystemConfigState>({
  config: SAFE_SYSTEM_CONFIG,
  status: 'loading',
});

const RETRY_DELAYS_MS = [2000, 5000];

const asRecord = (value: unknown): Record<string, unknown> =>
  value && typeof value === 'object' ? (value as Record<string, unknown>) : {};

// A DNS name of one or more labels (e.g. "nevatal.id"); anything else disables Domains.
const DOMAIN_ROOT_RE = /^(?=.{1,253}$)[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)*$/;

function normalizeDomainRoot(value: unknown): string {
  if (typeof value !== 'string') return '';
  const root = value.trim().toLowerCase().replace(/^\.+/, '').replace(/\.+$/, '');
  return DOMAIN_ROOT_RE.test(root) ? root : '';
}

function positiveInt(value: unknown): number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 ? Math.floor(value) : 0;
}

/** Strictly coerce the payload: only a literal `true` enables a flag. */
function normalizeConfig(raw: unknown): SystemConfig {
  const data = asRecord(raw);
  const features = asRecord(data.features);
  const domains = asRecord(data.domains);
  const auth = asRecord(data.auth);
  const strictTrue = (v: unknown) => v === true;
  const notFalse = (v: unknown) => v !== false;

  // Domains need both switches plus a usable root; otherwise every Domains surface stays hidden.
  const domainsRoot = normalizeDomainRoot(domains.root);
  const domainsEnabled = strictTrue(features.domains) && strictTrue(domains.enabled) && domainsRoot !== '';

  return {
    environment: typeof data.environment === 'string' && data.environment ? data.environment : 'production',
    demo_mode: strictTrue(data.demo_mode),
    proxmox_mock: strictTrue(data.proxmox_mock),
    version: typeof data.version === 'string' ? data.version : '',
    features: {
      docker_viewer: strictTrue(features.docker_viewer),
      domains: domainsEnabled,
    },
    domains: {
      enabled: domainsEnabled,
      root: domainsEnabled ? domainsRoot : '',
      max_per_user: positiveInt(domains.max_per_user),
    },
    auth: {
      password_login: notFalse(auth.password_login),
      social_login: strictTrue(auth.social_login),
      registration: notFalse(auth.registration),
    },
  };
}

export const SystemConfigProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [state, setState] = useState<SystemConfigState>({
    config: SAFE_SYSTEM_CONFIG,
    status: 'loading',
  });

  useEffect(() => {
    let cancelled = false;
    let retryTimer: ReturnType<typeof setTimeout> | undefined;
    const controller = new AbortController();

    const load = async (attempt: number) => {
      try {
        const raw = await apiRequest<unknown>('/system/config', { signal: controller.signal });
        if (cancelled) return;
        setState({ config: normalizeConfig(raw), status: 'ready' });
      } catch (err) {
        if (cancelled || (err as { name?: string })?.name === 'AbortError') return;
        if (attempt < RETRY_DELAYS_MS.length) {
          retryTimer = setTimeout(() => load(attempt + 1), RETRY_DELAYS_MS[attempt]);
          return;
        }
        console.warn('Failed to load system configuration; using safe production defaults.', err);
        setState({ config: SAFE_SYSTEM_CONFIG, status: 'error' });
      }
    };

    load(0);
    return () => {
      cancelled = true;
      controller.abort();
      if (retryTimer) clearTimeout(retryTimer);
    };
  }, []);

  return <SystemConfigContext.Provider value={state}>{children}</SystemConfigContext.Provider>;
};

export const useSystemConfig = (): SystemConfigState => useContext(SystemConfigContext);
