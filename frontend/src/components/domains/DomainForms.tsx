import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AlertCircle,
  AlertTriangle,
  Check,
  CheckCircle2,
  Clock,
  ExternalLink,
  PlusCircle,
  RefreshCw,
  XCircle,
} from 'lucide-react';
import { Domain, Instance } from '../../types';
import { StatusBadge } from '../ui/Badge';
import { CopyButton } from '../docker/DockerBits';
import { LiveStatusBadge, LiveStatusNote } from './DomainBits';
import {
  AVAILABILITY_DEBOUNCE_MS,
  DomainErrorInfo,
  PORT_PRESETS,
  SUBDOMAIN_MAX_LENGTH,
  SUBDOMAIN_MIN_LENGTH,
  checkSubdomain,
  createDomain,
  describeDomainError,
  domainUrl,
  fetchMyInstances,
  isAbortError,
  isDomainsDisabledError,
  normalizeSubdomainInput,
  parsePort,
  sameId,
  sortInstancesForPicker,
  subdomainProblem,
  updateDomain,
} from '../../lib/domains';

type Navigate = (path: string, param?: string) => void;

const inputBase =
  'bg-zinc-950 border rounded-md px-3 py-2 text-xs text-white placeholder-zinc-600 focus:outline-none transition-colors';
const borderFor = (invalid: boolean) =>
  invalid ? 'border-rose-500/50 focus:border-rose-500/70' : 'border-zinc-800 focus:border-zinc-600';

const FieldLabel: React.FC<{ htmlFor?: string; id?: string; children: React.ReactNode; hint?: string }> = ({
  htmlFor,
  id,
  children,
  hint,
}) => (
  <div className="flex items-baseline justify-between gap-2 mb-1.5">
    <label htmlFor={htmlFor} id={id} className="text-xs font-medium text-zinc-200">
      {children}
    </label>
    {hint && <span className="text-[10px] font-mono text-zinc-500">{hint}</span>}
  </div>
);

const FieldError: React.FC<{ id?: string; children: React.ReactNode }> = ({ id, children }) => (
  <p id={id} className="mt-1.5 flex items-start gap-1.5 text-[11px] text-rose-300">
    <AlertCircle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-rose-400" />
    <span>{children}</span>
  </p>
);

const FormBanner: React.FC<{ tone: 'rose' | 'amber'; children: React.ReactNode }> = ({ tone, children }) => (
  <div
    role={tone === 'rose' ? 'alert' : undefined}
    className={`flex items-start gap-2 p-2.5 rounded-md border text-[11px] ${
      tone === 'rose' ? 'bg-rose-500/5 border-rose-500/20 text-rose-300' : 'bg-amber-500/5 border-amber-500/20 text-amber-300'
    }`}
  >
    <AlertTriangle className={`w-3.5 h-3.5 mt-px flex-shrink-0 ${tone === 'rose' ? 'text-rose-400' : 'text-amber-400'}`} />
    <div className="min-w-0 break-words">{children}</div>
  </div>
);

const secondaryBtn =
  'px-3 py-1.5 rounded-md bg-zinc-900 border border-zinc-800 text-xs font-medium text-zinc-300 hover:text-white hover:bg-zinc-800 transition-colors disabled:opacity-50 disabled:pointer-events-none';
const primaryBtn =
  'inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white text-zinc-950 text-xs font-medium hover:bg-zinc-200 transition-colors disabled:opacity-40 disabled:pointer-events-none';

// ---- availability ---------------------------------------------------------------------------

type Availability =
  | { state: 'idle' }
  | { state: 'invalid'; message: string }
  | { state: 'checking' }
  | { state: 'available'; hostname: string }
  | { state: 'unavailable'; code: string; message: string }
  | { state: 'error'; message: string };

const CHECK_REASONS: Record<string, string> = {
  INVALID: 'This name is not valid.',
  RESERVED: 'This name is reserved.',
  TAKEN: 'This name is already in use.',
  QUARANTINED: 'This name was released recently and is quarantined for a while.',
};

/**
 * Client validation first; only a name that passes it is sent to /domains/check, 400 ms after
 * the last keystroke. Each new value aborts the previous request, and a result is shown only
 * while the input still equals the value it was fetched for.
 */
function useSubdomainAvailability(value: string, root: string): Availability {
  const [result, setResult] = useState<{ value: string; availability: Availability } | null>(null);
  const problem = value ? subdomainProblem(value) : null;

  useEffect(() => {
    if (!value || problem) return;
    const controller = new AbortController();
    const timer = setTimeout(() => {
      checkSubdomain(value, controller.signal)
        .then((r) => {
          if (controller.signal.aborted) return;
          setResult({
            value,
            availability: r.available
              ? { state: 'available', hostname: r.hostname || `${value}.${root}` }
              : {
                  state: 'unavailable',
                  code: r.reason_code,
                  message: r.reason || CHECK_REASONS[r.reason_code] || 'This name is not available.',
                },
          });
        })
        .catch((err) => {
          if (controller.signal.aborted || isAbortError(err)) return;
          setResult({ value, availability: { state: 'error', message: describeDomainError(err).message } });
        });
    }, AVAILABILITY_DEBOUNCE_MS);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [value, problem, root]);

  if (!value) return { state: 'idle' };
  if (problem) return { state: 'invalid', message: problem };
  if (result && result.value === value) return result.availability;
  return { state: 'checking' };
}

const AvailabilityLine: React.FC<{ availability: Availability; id: string }> = ({ availability, id }) => {
  const base = 'mt-1.5 flex items-start gap-1.5 text-[11px]';
  switch (availability.state) {
    case 'idle':
      return (
        <p id={id} className={`${base} text-zinc-500`}>
          {SUBDOMAIN_MIN_LENGTH}-{SUBDOMAIN_MAX_LENGTH} characters: lowercase letters, digits and hyphens.
        </p>
      );
    case 'invalid':
      return (
        <p id={id} className={`${base} text-rose-300`}>
          <AlertCircle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-rose-400" />
          <span>{availability.message}</span>
        </p>
      );
    case 'checking':
      return (
        <p id={id} className={`${base} text-zinc-400`} aria-live="polite">
          <RefreshCw className="w-3.5 h-3.5 mt-px flex-shrink-0 animate-spin text-zinc-500" />
          <span>Checking availability…</span>
        </p>
      );
    case 'available':
      return (
        <p id={id} className={`${base} text-emerald-300`} aria-live="polite">
          <Check className="w-3.5 h-3.5 mt-px flex-shrink-0 text-emerald-400" />
          <span>
            <span className="font-mono">{availability.hostname}</span> is available
          </span>
        </p>
      );
    case 'unavailable': {
      const quarantined = availability.code === 'QUARANTINED';
      const Icon = quarantined ? Clock : XCircle;
      return (
        <p id={id} className={`${base} ${quarantined ? 'text-amber-300' : 'text-rose-300'}`} aria-live="polite">
          <Icon className={`w-3.5 h-3.5 mt-px flex-shrink-0 ${quarantined ? 'text-amber-400' : 'text-rose-400'}`} />
          <span>{availability.message}</span>
        </p>
      );
    }
    case 'error':
      return (
        <p id={id} className={`${base} text-zinc-400`} title={availability.message}>
          <AlertTriangle className="w-3.5 h-3.5 mt-px flex-shrink-0 text-amber-400" />
          <span>Could not check availability right now. You can still try to publish it.</span>
        </p>
      );
  }
};

// ---- fields -----------------------------------------------------------------------------------

const SubdomainField: React.FC<{
  value: string;
  onChange: (value: string) => void;
  root: string;
  availability: Availability;
  serverError?: string;
  autoFocus?: boolean;
}> = ({ value, onChange, root, availability, serverError, autoFocus }) => {
  const invalid = !!serverError || availability.state === 'invalid' || availability.state === 'unavailable';
  return (
    <div>
      <FieldLabel htmlFor="domain-subdomain">Name</FieldLabel>
      <div
        className={`flex items-stretch rounded-md border bg-zinc-950 transition-colors ${
          invalid ? 'border-rose-500/50 focus-within:border-rose-500/70' : 'border-zinc-800 focus-within:border-zinc-600'
        }`}
      >
        <span className="pl-3 flex items-center text-xs font-mono text-zinc-500 select-none">https://</span>
        <input
          id="domain-subdomain"
          type="text"
          value={value}
          onChange={(e) => onChange(normalizeSubdomainInput(e.target.value))}
          placeholder="myapp"
          autoComplete="off"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          maxLength={100}
          autoFocus={autoFocus}
          aria-invalid={invalid}
          aria-describedby="domain-subdomain-status"
          className="flex-1 min-w-0 bg-transparent py-2 px-0.5 text-xs font-mono text-white placeholder-zinc-600 focus:outline-none"
        />
        <span className="pr-3 flex items-center text-xs font-mono text-zinc-400 select-none truncate max-w-[55%]" title={`.${root}`}>
          .{root}
        </span>
      </div>
      {serverError ? (
        <FieldError id="domain-subdomain-status">{serverError}</FieldError>
      ) : (
        <AvailabilityLine availability={availability} id="domain-subdomain-status" />
      )}
    </div>
  );
};

/** Hook: the user's VMs, fetched on mount (so every open of a form sees a fresh list). */
function useMyInstances() {
  const [instances, setInstances] = useState<Instance[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [reloadKey, setReloadKey] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    setError(null);
    fetchMyInstances(controller.signal)
      .then((list) => {
        if (!controller.signal.aborted) setInstances(sortInstancesForPicker(list));
      })
      .catch((err) => {
        if (controller.signal.aborted || isAbortError(err)) return;
        setError(describeDomainError(err).message);
      });
    return () => controller.abort();
  }, [reloadKey]);

  const reload = useCallback(() => setReloadKey((k) => k + 1), []);
  return { instances, error, reload };
}

const InstancePicker: React.FC<{
  instances: Instance[] | null;
  error: string | null;
  onRetry: () => void;
  value: string | null;
  onChange: (id: string) => void;
  serverError?: string;
  onNavigate: Navigate;
}> = ({ instances, error, onRetry, value, onChange, serverError, onNavigate }) => {
  const selected = instances?.find((i) => sameId(i.id, value));
  let body: React.ReactNode;

  if (error && !instances) {
    body = (
      <div className="flex items-center justify-between gap-2 p-3 rounded-md border border-rose-500/20 bg-rose-500/5 text-[11px] text-rose-300">
        <span className="min-w-0 break-words">Could not load your VMs: {error}</span>
        <button type="button" onClick={onRetry} className={secondaryBtn}>
          Retry
        </button>
      </div>
    );
  } else if (!instances) {
    body = (
      <div className="flex items-center gap-2 p-3 rounded-md border border-zinc-800 text-[11px] text-zinc-500 font-mono">
        <RefreshCw className="w-3.5 h-3.5 animate-spin" />
        <span>Loading your VMs…</span>
      </div>
    );
  } else if (instances.length === 0) {
    body = (
      <div className="flex items-center justify-between gap-2 p-3 rounded-md border border-dashed border-zinc-800 text-[11px] text-zinc-400">
        <span>You have no VMs yet. A domain forwards to a port of one of your VMs.</span>
        <button type="button" onClick={() => onNavigate('/dashboard/instances/new')} className={secondaryBtn}>
          Create a VM
        </button>
      </div>
    );
  } else {
    body = (
      <div
        role="radiogroup"
        aria-labelledby="domain-instance-label"
        className={`max-h-52 overflow-y-auto rounded-md border divide-y divide-zinc-800/60 ${
          serverError ? 'border-rose-500/50' : 'border-zinc-800'
        }`}
      >
        {instances.map((inst) => {
          const checked = sameId(inst.id, value);
          return (
            <label
              key={inst.id}
              className={`flex items-center gap-3 px-3 py-2 cursor-pointer transition-colors has-[input:focus-visible]:bg-zinc-800/80 ${
                checked ? 'bg-zinc-800/60' : 'hover:bg-zinc-900'
              }`}
            >
              <input
                type="radio"
                name="domain-instance"
                value={inst.id}
                checked={checked}
                onChange={() => onChange(inst.id)}
                className="sr-only"
              />
              <span
                aria-hidden="true"
                className={`w-3 h-3 rounded-full border flex-shrink-0 ${
                  checked ? 'border-white bg-white shadow-[inset_0_0_0_2px_rgb(24_24_27)]' : 'border-zinc-600'
                }`}
              />
              <span className="min-w-0 flex-1">
                <span className={`block text-xs truncate ${checked ? 'font-semibold text-white' : 'font-medium text-zinc-200'}`}>
                  {inst.name || inst.hostname}
                </span>
                <span className="block text-[10px] font-mono text-zinc-500 truncate">
                  {inst.private_ip || 'no IP yet'}
                  {inst.hostname && inst.hostname !== inst.name ? ` · ${inst.hostname}` : ''}
                </span>
              </span>
              <StatusBadge status={inst.status} size="sm" />
            </label>
          );
        })}
      </div>
    );
  }

  return (
    <div>
      <FieldLabel id="domain-instance-label" hint={instances && instances.length > 0 ? 'running first' : undefined}>
        Virtual machine
      </FieldLabel>
      {body}
      {serverError && <FieldError>{serverError}</FieldError>}
      {!serverError && selected && selected.status !== 'running' && (
        <p className="mt-1.5 flex items-start gap-1.5 text-[11px] text-amber-300">
          <Clock className="w-3.5 h-3.5 mt-px flex-shrink-0 text-amber-400" />
          <span>This VM is {selected.status}. The domain goes live once it is running and reports an IP.</span>
        </p>
      )}
    </div>
  );
};

const PortField: React.FC<{ value: string; onChange: (value: string) => void; serverError?: string }> = ({
  value,
  onChange,
  serverError,
}) => {
  const port = parsePort(value);
  const showRangeError = value !== '' && port === null;
  return (
    <div>
      <FieldLabel htmlFor="domain-port" hint="1-65535">
        Port inside the VM
      </FieldLabel>
      <div className="flex flex-wrap items-center gap-2">
        <input
          id="domain-port"
          type="text"
          inputMode="numeric"
          autoComplete="off"
          value={value}
          onChange={(e) => onChange(e.target.value.replace(/\D/g, '').slice(0, 5))}
          placeholder="3000"
          aria-invalid={showRangeError || !!serverError}
          className={`${inputBase} ${borderFor(showRangeError || !!serverError)} w-24 font-mono`}
        />
        <div className="flex flex-wrap gap-1" aria-label="Common ports">
          {PORT_PRESETS.map((preset) => {
            const active = port === preset;
            return (
              <button
                key={preset}
                type="button"
                aria-pressed={active}
                onClick={() => onChange(String(preset))}
                className={`px-2 py-1 rounded-md border font-mono text-[11px] transition-colors ${
                  active
                    ? 'bg-zinc-800 border-zinc-600 text-white'
                    : 'bg-zinc-900 border-zinc-800 text-zinc-400 hover:text-zinc-200'
                }`}
              >
                {preset}
              </button>
            );
          })}
        </div>
      </div>
      {serverError ? (
        <FieldError>{serverError}</FieldError>
      ) : showRangeError ? (
        <FieldError>Enter a port from 1 to 65535.</FieldError>
      ) : (
        <p className="mt-1.5 text-[11px] text-zinc-500">
          The app must listen on <span className="font-mono text-zinc-300">0.0.0.0:{port ?? '<port>'}</span> inside the
          VM, not 127.0.0.1.
        </p>
      )}
    </div>
  );
};

// ---- create -----------------------------------------------------------------------------------

export interface CreateDomainFormProps {
  root: string;
  maxPerUser: number;
  usedCount: number;
  /** VM to preselect (from `?instance=`), already validated as a UUID. */
  initialInstanceId: string | null;
  onBusyChange: (busy: boolean) => void;
  onCreated: (domain: Domain | null) => void;
  onDomainsDisabled: () => void;
  onClose: () => void;
  onNavigate: Navigate;
}

export const CreateDomainForm: React.FC<CreateDomainFormProps> = ({
  root,
  maxPerUser,
  usedCount,
  initialInstanceId,
  onBusyChange,
  onCreated,
  onDomainsDisabled,
  onClose,
  onNavigate,
}) => {
  const { instances, error: instancesError, reload: reloadInstances } = useMyInstances();
  const [subdomain, setSubdomain] = useState('');
  const [instanceId, setInstanceId] = useState<string | null>(null);
  const [portText, setPortText] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<DomainErrorInfo | null>(null);
  const [missingInitial, setMissingInitial] = useState(false);
  const [created, setCreated] = useState<{ hostname: string; port: number; domain: Domain | null } | null>(null);

  const availability = useSubdomainAvailability(subdomain, root);
  const port = parsePort(portText);
  const atLimit = maxPerUser > 0 && usedCount >= maxPerUser;

  // Preselect once the VM list arrives: the VM from the link, else the only VM there is.
  const [preselected, setPreselected] = useState(false);
  useEffect(() => {
    if (preselected || !instances) return;
    setPreselected(true);
    if (initialInstanceId) {
      const match = instances.find((i) => sameId(i.id, initialInstanceId));
      if (match) setInstanceId(match.id);
      else setMissingInitial(true);
    } else if (instances.length === 1) {
      setInstanceId(instances[0].id);
    }
  }, [instances, initialInstanceId, preselected]);

  const fieldError = (field: DomainErrorInfo['field']) =>
    submitError && submitError.field === field ? submitError.message : undefined;

  const clearFieldError = (field: DomainErrorInfo['field']) =>
    setSubmitError((current) => (current && current.field === field ? null : current));

  const canSubmit =
    !submitting &&
    !atLimit &&
    availability.state !== 'invalid' &&
    availability.state !== 'unavailable' &&
    availability.state !== 'idle' &&
    !!instanceId &&
    port !== null;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit || !instanceId || port === null) return;
    setSubmitting(true);
    onBusyChange(true);
    setSubmitError(null);
    try {
      const domain = await createDomain({ subdomain, instance_id: instanceId, target_port: port });
      setCreated({ hostname: domain?.hostname || `${subdomain}.${root}`, port, domain });
      onCreated(domain);
    } catch (err) {
      if (isDomainsDisabledError(err)) {
        onDomainsDisabled();
        return;
      }
      setSubmitError(describeDomainError(err, maxPerUser));
    } finally {
      setSubmitting(false);
      onBusyChange(false);
    }
  };

  const addAnother = () => {
    setCreated(null);
    setSubdomain('');
    setSubmitError(null);
  };

  if (created) {
    const url = domainUrl(created.hostname);
    return (
      <div className="space-y-4">
        <div className="flex items-start gap-3 p-3 rounded-lg bg-emerald-500/5 border border-emerald-500/20">
          <CheckCircle2 className="w-4 h-4 mt-0.5 flex-shrink-0 text-emerald-400" />
          <div className="min-w-0">
            <div className="text-xs font-semibold text-emerald-300">Domain published</div>
            <p className="text-[11px] text-zinc-400 mt-0.5 leading-relaxed">
              It may take a few seconds to go live. Requests are forwarded to port{' '}
              <span className="font-mono text-zinc-200">{created.port}</span> on the VM.
            </p>
          </div>
        </div>

        <div className="flex items-center gap-1 bg-zinc-950 border border-zinc-800 rounded-md pl-3 pr-1 py-1">
          {url ? (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              className="flex-1 min-w-0 truncate font-mono text-xs font-semibold text-white hover:underline"
            >
              {url}
            </a>
          ) : (
            <span className="flex-1 min-w-0 truncate font-mono text-xs text-zinc-200">{created.hostname}</span>
          )}
          <CopyButton text={url || created.hostname} title="Copy URL" label="Copy" />
          {url && (
            <a
              href={url}
              target="_blank"
              rel="noopener noreferrer"
              title="Open in a new tab"
              className="p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-800 transition-colors"
            >
              <ExternalLink className="w-3.5 h-3.5" />
            </a>
          )}
        </div>

        {created.domain && created.domain.live_status && (
          <div className="flex flex-wrap items-start gap-x-3 gap-y-1.5">
            <LiveStatusBadge domain={created.domain} />
            <LiveStatusNote domain={created.domain} className="flex-1 min-w-[12rem]" />
          </div>
        )}

        <p className="text-[11px] text-zinc-500 leading-relaxed">
          Not loading? Make sure the app listens on{' '}
          <span className="font-mono text-zinc-300">0.0.0.0:{created.port}</span> inside the VM.
        </p>

        <div className="flex items-center justify-end gap-2 pt-3 border-t border-zinc-800">
          <button type="button" onClick={addAnother} className={secondaryBtn} disabled={atLimit}>
            Add another
          </button>
          <button type="button" onClick={onClose} className={primaryBtn}>
            Done
          </button>
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-4" noValidate>
      {atLimit && (
        <FormBanner tone="amber">
          You are using {usedCount} of {maxPerUser} domains. Release a domain before adding another.
        </FormBanner>
      )}

      <SubdomainField
        value={subdomain}
        onChange={(v) => {
          setSubdomain(v);
          clearFieldError('subdomain');
        }}
        root={root}
        availability={availability}
        serverError={fieldError('subdomain')}
        autoFocus
      />

      {missingInitial && (
        <FormBanner tone="amber">The VM from the link is not one of your VMs. Pick a VM below.</FormBanner>
      )}

      <InstancePicker
        instances={instances}
        error={instancesError}
        onRetry={reloadInstances}
        value={instanceId}
        onChange={(id) => {
          setInstanceId(id);
          setMissingInitial(false);
          clearFieldError('instance');
        }}
        serverError={fieldError('instance')}
        onNavigate={onNavigate}
      />

      <PortField
        value={portText}
        onChange={(v) => {
          setPortText(v);
          clearFieldError('port');
        }}
        serverError={fieldError('port')}
      />

      {submitError && submitError.field === 'form' && <FormBanner tone="rose">{submitError.message}</FormBanner>}

      <div className="flex items-center justify-end gap-2 pt-3 border-t border-zinc-800">
        <button type="button" onClick={onClose} className={secondaryBtn} disabled={submitting}>
          Cancel
        </button>
        <button type="submit" className={primaryBtn} disabled={!canSubmit}>
          {submitting ? <RefreshCw className="w-3.5 h-3.5 animate-spin" /> : <PlusCircle className="w-3.5 h-3.5" />}
          <span>{submitting ? 'Publishing…' : 'Publish domain'}</span>
        </button>
      </div>
    </form>
  );
};

// ---- edit -------------------------------------------------------------------------------------

export interface EditDomainFormProps {
  domain: Domain;
  onBusyChange: (busy: boolean) => void;
  onSaved: (domain: Domain | null) => void;
  onDomainsDisabled: () => void;
  onClose: () => void;
  onNavigate: Navigate;
}

/** Re-point an existing domain at another VM and/or port. The name itself cannot change. */
export const EditDomainForm: React.FC<EditDomainFormProps> = ({
  domain,
  onBusyChange,
  onSaved,
  onDomainsDisabled,
  onClose,
  onNavigate,
}) => {
  const { instances, error: instancesError, reload: reloadInstances } = useMyInstances();
  const [instanceId, setInstanceId] = useState<string | null>(domain.instance_id);
  const [portText, setPortText] = useState(domain.target_port ? String(domain.target_port) : '');
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<DomainErrorInfo | null>(null);

  const port = parsePort(portText);
  const instanceChanged = !!instanceId && !sameId(instanceId, domain.instance_id);
  const portChanged = port !== null && port !== domain.target_port;
  const canSubmit = !submitting && !!instanceId && port !== null && (instanceChanged || portChanged);
  const currentMissing = useMemo(
    () => !!instances && !!domain.instance_id && !instances.some((i) => sameId(i.id, domain.instance_id)),
    [instances, domain.instance_id]
  );

  const fieldError = (field: DomainErrorInfo['field']) =>
    submitError && submitError.field === field ? submitError.message : undefined;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!canSubmit || !instanceId || port === null) return;
    setSubmitting(true);
    onBusyChange(true);
    setSubmitError(null);
    try {
      const updated = await updateDomain(domain.id, {
        ...(instanceChanged ? { instance_id: instanceId } : {}),
        ...(portChanged ? { target_port: port } : {}),
      });
      onSaved(updated);
    } catch (err) {
      if (isDomainsDisabledError(err)) {
        onDomainsDisabled();
        return;
      }
      setSubmitError(describeDomainError(err));
    } finally {
      setSubmitting(false);
      onBusyChange(false);
    }
  };

  return (
    <form onSubmit={handleSubmit} className="space-y-4" noValidate>
      <div>
        <FieldLabel>Name</FieldLabel>
        <div className="px-3 py-2 rounded-md border border-zinc-800 bg-zinc-900/40 font-mono text-xs text-zinc-300 truncate">
          https://{domain.hostname || domain.subdomain}
        </div>
        <p className="mt-1.5 text-[11px] text-zinc-500">
          The name cannot be changed. Release this domain and create a new one to use another name.
        </p>
      </div>

      {(domain.instance_id === null || currentMissing) && (
        <FormBanner tone="amber">The VM this domain pointed to no longer exists. Pick another VM.</FormBanner>
      )}

      <InstancePicker
        instances={instances}
        error={instancesError}
        onRetry={reloadInstances}
        value={instanceId}
        onChange={(id) => {
          setInstanceId(id);
          setSubmitError((c) => (c && c.field === 'instance' ? null : c));
        }}
        serverError={fieldError('instance')}
        onNavigate={onNavigate}
      />

      <PortField
        value={portText}
        onChange={(v) => {
          setPortText(v);
          setSubmitError((c) => (c && c.field === 'port' ? null : c));
        }}
        serverError={fieldError('port')}
      />

      {submitError && (submitError.field === 'form' || submitError.field === 'subdomain') && (
        <FormBanner tone="rose">{submitError.message}</FormBanner>
      )}

      <p className="text-[11px] text-zinc-500">Changes can take a few seconds to go live.</p>

      <div className="flex items-center justify-end gap-2 pt-3 border-t border-zinc-800">
        <button type="button" onClick={onClose} className={secondaryBtn} disabled={submitting}>
          Cancel
        </button>
        <button type="submit" className={primaryBtn} disabled={!canSubmit}>
          {submitting && <RefreshCw className="w-3.5 h-3.5 animate-spin" />}
          <span>{submitting ? 'Saving…' : 'Save changes'}</span>
        </button>
      </div>
    </form>
  );
};
