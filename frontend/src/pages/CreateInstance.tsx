import React, { useState, useEffect } from 'react';
import { OSTemplate, ResourcePlan, SSHKey, QuotaUsage } from '../types';
import { apiRequest } from '../lib/api';
import { Modal } from '../components/ui/Modal';
import {
  Server,
  Cpu,
  HardDrive,
  Key,
  Shield,
  ArrowRight,
  ArrowLeft,
  Check,
  AlertTriangle,
  Plus,
  RefreshCw,
  Globe,
  Zap,
} from 'lucide-react';

interface CreateInstanceProps {
  onNavigate: (path: string, param?: string) => void;
}

export const CreateInstance: React.FC<CreateInstanceProps> = ({ onNavigate }) => {
  const [step, setStep] = useState(1);
  const [templates, setTemplates] = useState<OSTemplate[]>([]);
  const [plans, setPlans] = useState<ResourcePlan[]>([]);
  const [sshKeys, setSSHKeys] = useState<SSHKey[]>([]);
  const [usage, setUsage] = useState<QuotaUsage | null>(null);
  const [loading, setLoading] = useState(true);

  // Form State
  const [selectedTemplate, setSelectedTemplate] = useState<OSTemplate | null>(null);
  const [useCustomSpecs, setUseCustomSpecs] = useState(false);
  const [selectedPlan, setSelectedPlan] = useState<ResourcePlan | null>(null);
  const [customVCPUs, setCustomVCPUs] = useState(2);
  const [customMemoryGB, setCustomMemoryGB] = useState(4);
  const [customDiskGB, setCustomDiskGB] = useState(40);
  const [name, setName] = useState('');
  const [hostname, setHostname] = useState('');
  const [selectedSSHKeyId, setSelectedSSHKeyId] = useState<string>('');

  // Inline SSH Key creation modal state
  const [showAddKeyModal, setShowAddKeyModal] = useState(false);
  const [newKeyName, setNewKeyName] = useState('');
  const [newPublicKey, setNewPublicKey] = useState('');
  const [keyError, setKeyError] = useState<string | null>(null);
  const [creatingKey, setCreatingKey] = useState(false);

  // Submit state
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [tplRes, plansRes, keysRes, usageRes] = await Promise.all([
          apiRequest<OSTemplate[]>('/templates'),
          apiRequest<ResourcePlan[]>('/plans'),
          apiRequest<SSHKey[]>('/ssh-keys'),
          apiRequest<QuotaUsage>('/usage'),
        ]);

        setTemplates(tplRes || []);
        if (tplRes && tplRes.length > 0) setSelectedTemplate(tplRes[0]);

        setPlans(plansRes || []);
        if (plansRes && plansRes.length > 2) {
          setSelectedPlan(plansRes[2]); // Default to Developer
        } else if (plansRes && plansRes.length > 0) {
          setSelectedPlan(plansRes[0]);
        }

        setSSHKeys(keysRes || []);
        if (keysRes && keysRes.length > 0) setSelectedSSHKeyId(keysRes[0].id);

        setUsage(usageRes);
      } catch (err) {
        console.error('Failed to load provisioning options:', err);
      } finally {
        setLoading(false);
      }
    };
    fetchData();
  }, []);

  const handleCreateKey = async () => {
    setKeyError(null);
    setCreatingKey(true);
    try {
      const created = await apiRequest<SSHKey>('/ssh-keys', {
        method: 'POST',
        body: JSON.stringify({ name: newKeyName, public_key: newPublicKey }),
      });
      setSSHKeys([created, ...sshKeys]);
      setSelectedSSHKeyId(created.id);
      setShowAddKeyModal(false);
      setNewKeyName('');
      setNewPublicKey('');
    } catch (err: any) {
      setKeyError(err.message || 'Failed to register SSH key');
    } finally {
      setCreatingKey(false);
    }
  };

  const getEffectiveResources = () => {
    if (useCustomSpecs) {
      return {
        vcpus: customVCPUs,
        memoryMB: customMemoryGB * 1024,
        diskGB: customDiskGB,
      };
    }
    if (selectedPlan) {
      return {
        vcpus: selectedPlan.vcpus,
        memoryMB: selectedPlan.memory_mb,
        diskGB: selectedPlan.disk_gb,
      };
    }
    return { vcpus: 1, memoryMB: 1024, diskGB: 20 };
  };

  const currentResources = getEffectiveResources();

  // Quota validation
  const exceedsVCPU = usage ? usage.used_vcpus + currentResources.vcpus > usage.max_vcpus : false;
  const exceedsRAM = usage
    ? usage.used_memory_mb + currentResources.memoryMB > usage.max_memory_mb
    : false;
  const exceedsDisk = usage
    ? usage.used_storage_gb + currentResources.diskGB > usage.max_storage_gb
    : false;
  const exceedsInstances = usage ? usage.used_instances + 1 > usage.max_instances : false;
  const hasQuotaError = exceedsVCPU || exceedsRAM || exceedsDisk || exceedsInstances;

  const handleSubmit = async () => {
    if (!selectedTemplate) return;
    setSubmitError(null);
    setSubmitting(true);

    try {
      const idempotencyKey = `create-${Date.now()}-${Math.random().toString(36).substring(7)}`;

      const payload = {
        name: name.trim().toLowerCase(),
        hostname: hostname.trim() || name.trim().toLowerCase(),
        template_id: selectedTemplate.id,
        plan_id: useCustomSpecs ? null : selectedPlan?.id,
        vcpus: currentResources.vcpus,
        memory_mb: currentResources.memoryMB,
        disk_gb: currentResources.diskGB,
        ssh_key_id: selectedSSHKeyId || null,
        idempotency_key: idempotencyKey,
      };

      const res = await apiRequest<{ instance_id: string; job_id: string; status: string }>(
        '/instances',
        {
          method: 'POST',
          headers: {
            'X-Idempotency-Key': idempotencyKey,
          },
          body: JSON.stringify(payload),
        }
      );

      // Redirect to provisioning progress page
      onNavigate('/dashboard/jobs', res.job_id);
    } catch (err: any) {
      setSubmitError(err.message || 'Provisioning request failed');
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-slate-400">
        <RefreshCw className="w-6 h-6 animate-spin text-blue-500 mr-2" />
        <span>Loading template and plan catalogs...</span>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Wizard Header */}
      <div>
        <button
          onClick={() => onNavigate('/dashboard/instances')}
          className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-white mb-2"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>Back to instances</span>
        </button>
        <h1 className="text-2xl font-bold text-white tracking-tight">Deploy Virtual Machine</h1>
        <p className="text-xs text-slate-400 mt-1">
          Automated cloud-init VM deployment on Dell PowerEdge R640 Proxmox VE
        </p>
      </div>

      {/* Stepper Progress Indicator */}
      <div className="grid grid-cols-5 gap-2 pb-2">
        {[
          { num: 1, title: 'OS Template' },
          { num: 2, title: 'Resources' },
          { num: 3, title: 'Authentication' },
          { num: 4, title: 'Networking' },
          { num: 5, title: 'Review & Deploy' },
        ].map((s) => (
          <div
            key={s.num}
            onClick={() => s.num < step && setStep(s.num)}
            className={`cursor-pointer border-t-2 pt-2 text-xs transition ${
              step === s.num
                ? 'border-blue-500 text-blue-400 font-semibold'
                : step > s.num
                ? 'border-emerald-500 text-emerald-400'
                : 'border-slate-800 text-slate-600'
            }`}
          >
            <div className="flex items-center gap-1.5">
              <span>Step {s.num}</span>
            </div>
            <div className="truncate text-[11px] text-slate-300 hidden sm:block">{s.title}</div>
          </div>
        ))}
      </div>

      {submitError && (
        <div className="p-4 rounded-xl bg-rose-950/60 border border-rose-800 text-rose-300 text-xs flex items-start gap-3">
          <AlertTriangle className="w-4 h-4 flex-shrink-0 mt-0.5 text-rose-400" />
          <div>
            <p className="font-semibold">Unable to accept provisioning request</p>
            <p className="mt-0.5">{submitError}</p>
          </div>
        </div>
      )}

      {/* STEP 1: OS Template */}
      {step === 1 && (
        <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm space-y-4">
          <div>
            <h2 className="text-base font-bold text-white">Select Operating System Image</h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Approved Linux distributions pre-configured with QEMU Guest Agent & Cloud-Init.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-3 gap-4 pt-2">
            {templates.map((tpl) => {
              const selected = selectedTemplate?.id === tpl.id;
              return (
                <div
                  key={tpl.id}
                  onClick={() => setSelectedTemplate(tpl)}
                  className={`relative p-5 rounded-xl border cursor-pointer transition-all ${
                    selected
                      ? 'bg-blue-950/40 border-blue-500 ring-2 ring-blue-500/20'
                      : 'bg-slate-950/60 border-slate-800 hover:border-slate-700'
                  }`}
                >
                  {selected && (
                    <div className="absolute top-3 right-3 w-5 h-5 rounded-full bg-blue-500 text-white flex items-center justify-center">
                      <Check className="w-3 h-3 stroke-[3]" />
                    </div>
                  )}
                  <div className="w-9 h-9 rounded-lg bg-slate-800 flex items-center justify-center text-blue-400 mb-3">
                    <Server className="w-5 h-5" />
                  </div>
                  <h3 className="font-semibold text-white text-sm">{tpl.name}</h3>
                  <p className="text-[11px] text-slate-400 mt-1 leading-relaxed">
                    {tpl.description}
                  </p>
                  <div className="mt-4 pt-3 border-t border-slate-800/80 flex items-center justify-between text-[11px] text-slate-400">
                    <span className="font-mono">{tpl.architecture}</span>
                    <span className="text-emerald-400 font-medium">Cloud-Init Ready</span>
                  </div>
                </div>
              );
            })}
          </div>

          <div className="flex justify-end pt-4 border-t border-slate-800">
            <button
              onClick={() => setStep(2)}
              disabled={!selectedTemplate}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow transition disabled:opacity-50"
            >
              <span>Continue to Resources</span>
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* STEP 2: Configure Resources */}
      {step === 2 && (
        <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm space-y-6">
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <div>
              <h2 className="text-base font-bold text-white">Choose Compute & Memory Plan</h2>
              <p className="text-xs text-slate-400 mt-0.5">
                Select from predefined tiers or specify custom allocations within your quota.
              </p>
            </div>
            <div className="flex items-center gap-2 text-xs">
              <button
                onClick={() => setUseCustomSpecs(false)}
                className={`px-3 py-1.5 rounded-lg font-medium transition ${
                  !useCustomSpecs
                    ? 'bg-blue-600 text-white'
                    : 'bg-slate-800 text-slate-400 hover:text-white'
                }`}
              >
                Standard Plans
              </button>
              <button
                onClick={() => setUseCustomSpecs(true)}
                className={`px-3 py-1.5 rounded-lg font-medium transition ${
                  useCustomSpecs
                    ? 'bg-blue-600 text-white'
                    : 'bg-slate-800 text-slate-400 hover:text-white'
                }`}
              >
                Custom Specs
              </button>
            </div>
          </div>

          {!useCustomSpecs ? (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {plans.map((p) => {
                const selected = selectedPlan?.id === p.id;
                return (
                  <div
                    key={p.id}
                    onClick={() => setSelectedPlan(p)}
                    className={`relative p-5 rounded-xl border cursor-pointer transition-all ${
                      selected
                        ? 'bg-blue-950/40 border-blue-500 ring-2 ring-blue-500/20'
                        : 'bg-slate-950/60 border-slate-800 hover:border-slate-700'
                    }`}
                  >
                    {selected && (
                      <div className="absolute top-3 right-3 w-5 h-5 rounded-full bg-blue-500 text-white flex items-center justify-center">
                        <Check className="w-3 h-3 stroke-[3]" />
                      </div>
                    )}
                    <span className="text-xs font-semibold text-blue-400 uppercase tracking-wider block">
                      {p.name}
                    </span>
                    <div className="mt-2 text-lg font-bold text-white font-mono">
                      {p.vcpus} vCPU
                    </div>
                    <div className="text-xs text-slate-300 font-mono">
                      {Math.round(p.memory_mb / 1024)} GB RAM • {p.disk_gb} GB SSD
                    </div>
                    <p className="text-[11px] text-slate-500 mt-2">{p.description}</p>
                  </div>
                );
              })}
            </div>
          ) : (
            <div className="space-y-5 bg-slate-950/60 border border-slate-800 rounded-xl p-5">
              <div>
                <div className="flex justify-between text-xs text-slate-300 mb-2 font-medium">
                  <span>vCPU Allocation</span>
                  <span className="text-blue-400 font-mono font-bold">{customVCPUs} Cores</span>
                </div>
                <input
                  type="range"
                  min="1"
                  max="8"
                  value={customVCPUs}
                  onChange={(e) => setCustomVCPUs(parseInt(e.target.value))}
                  className="w-full accent-blue-500"
                />
              </div>

              <div>
                <div className="flex justify-between text-xs text-slate-300 mb-2 font-medium">
                  <span>Memory (RAM)</span>
                  <span className="text-blue-400 font-mono font-bold">{customMemoryGB} GB</span>
                </div>
                <input
                  type="range"
                  min="1"
                  max="16"
                  value={customMemoryGB}
                  onChange={(e) => setCustomMemoryGB(parseInt(e.target.value))}
                  className="w-full accent-blue-500"
                />
              </div>

              <div>
                <div className="flex justify-between text-xs text-slate-300 mb-2 font-medium">
                  <span>Virtual Disk (ZFS Storage)</span>
                  <span className="text-blue-400 font-mono font-bold">{customDiskGB} GB</span>
                </div>
                <input
                  type="range"
                  min="20"
                  max="200"
                  step="10"
                  value={customDiskGB}
                  onChange={(e) => setCustomDiskGB(parseInt(e.target.value))}
                  className="w-full accent-blue-500"
                />
              </div>
            </div>
          )}

          {/* Quota Check Notice */}
          {hasQuotaError && (
            <div className="p-4 rounded-xl bg-amber-950/50 border border-amber-800 text-amber-300 text-xs space-y-1">
              <div className="font-semibold flex items-center gap-1.5">
                <AlertTriangle className="w-4 h-4 text-amber-400" />
                <span>Resource Quota Exceeded</span>
              </div>
              <p className="text-[11px] text-amber-300/80">
                This configuration exceeds your available quota (
                {exceedsVCPU && 'vCPU limit; '}
                {exceedsRAM && 'RAM limit; '}
                {exceedsDisk && 'Storage limit; '}
                {exceedsInstances && 'Instance count limit'}
                ). Please choose a smaller plan or delete unused instances.
              </p>
            </div>
          )}

          <div className="flex justify-between pt-4 border-t border-slate-800">
            <button
              onClick={() => setStep(1)}
              className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold"
            >
              Back
            </button>
            <button
              onClick={() => setStep(3)}
              disabled={hasQuotaError}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow transition disabled:opacity-50"
            >
              <span>Continue to Access</span>
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* STEP 3: Configure Access & SSH Keys */}
      {step === 3 && (
        <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm space-y-5">
          <div>
            <h2 className="text-base font-bold text-white">Configure Instance Identity & SSH</h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Set the server name and select an SSH public key for rootless cloud-init authentication.
            </p>
          </div>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">
                Instance Name <span className="text-rose-400">*</span>
              </label>
              <input
                type="text"
                placeholder="my-development-server"
                value={name}
                onChange={(e) => {
                  setName(e.target.value);
                  if (!hostname) setHostname(e.target.value);
                }}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
              />
              <span className="text-[10px] text-slate-500 mt-1 block">
                Lowercase alphanumeric and hyphens (3-64 chars)
              </span>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Hostname</label>
              <input
                type="text"
                placeholder="dev-server"
                value={hostname}
                onChange={(e) => setHostname(e.target.value)}
                className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-blue-500 font-mono"
              />
            </div>
          </div>

          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-xs font-medium text-slate-300">SSH Public Key</label>
              <button
                onClick={() => setShowAddKeyModal(true)}
                className="text-xs text-blue-400 hover:text-blue-300 font-medium flex items-center gap-1"
              >
                <Plus className="w-3.5 h-3.5" />
                <span>Add New Key</span>
              </button>
            </div>

            {sshKeys.length === 0 ? (
              <div className="p-4 rounded-xl bg-slate-950/60 border border-slate-800 text-center text-xs text-slate-400 space-y-2">
                <p>No SSH public keys registered in your account.</p>
                <button
                  onClick={() => setShowAddKeyModal(true)}
                  className="px-3 py-1.5 rounded-lg bg-blue-600 text-white font-medium hover:bg-blue-500"
                >
                  Register SSH Public Key
                </button>
              </div>
            ) : (
              <div className="space-y-2">
                {sshKeys.map((key) => (
                  <div
                    key={key.id}
                    onClick={() => setSelectedSSHKeyId(key.id)}
                    className={`flex items-center justify-between p-3 rounded-xl border cursor-pointer text-xs transition ${
                      selectedSSHKeyId === key.id
                        ? 'bg-blue-950/40 border-blue-500 text-white'
                        : 'bg-slate-950/60 border-slate-800 text-slate-300 hover:border-slate-700'
                    }`}
                  >
                    <div className="flex items-center gap-2.5">
                      <Key className="w-4 h-4 text-blue-400" />
                      <div>
                        <span className="font-semibold">{key.name}</span>
                        <span className="text-[11px] text-slate-500 font-mono ml-2">
                          {key.fingerprint}
                        </span>
                      </div>
                    </div>
                    {selectedSSHKeyId === key.id && (
                      <Check className="w-4 h-4 text-blue-400 stroke-[3]" />
                    )}
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="flex justify-between pt-4 border-t border-slate-800">
            <button
              onClick={() => setStep(2)}
              className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold"
            >
              Back
            </button>
            <button
              onClick={() => setStep(4)}
              disabled={!name.trim()}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow transition disabled:opacity-50"
            >
              <span>Continue to Networking</span>
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* STEP 4: Networking */}
      {step === 4 && (
        <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm space-y-5">
          <div>
            <h2 className="text-base font-bold text-white">Default Cluster Network</h2>
            <p className="text-xs text-slate-400 mt-0.5">
              The MVP provisions workloads onto an isolated Proxmox bridge with automated DHCP.
            </p>
          </div>

          <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-5 space-y-4 text-xs">
            <div className="flex items-center gap-3 pb-3 border-b border-slate-800">
              <Globe className="w-5 h-5 text-blue-400" />
              <div>
                <span className="font-semibold text-white block">Hypervisor Bridge: vmbr0</span>
                <span className="text-slate-400">
                  Pre-configured private subnet with outbound NAT gateway.
                </span>
              </div>
            </div>

            <div className="grid grid-cols-2 gap-4 text-slate-300">
              <div>
                <span className="text-slate-500 block">IP Allocation</span>
                <span className="font-semibold">Dynamic DHCP Lease</span>
              </div>
              <div>
                <span className="text-slate-500 block">Outbound Internet</span>
                <span className="text-emerald-400 font-semibold">Enabled (Package downloads)</span>
              </div>
              <div>
                <span className="text-slate-500 block">Public Inbound IPv4</span>
                <span className="text-slate-400">Available via SSH / Overlay</span>
              </div>
              <div>
                <span className="text-slate-500 block">Isolation</span>
                <span className="text-emerald-400 font-semibold">Tenant-restricted</span>
              </div>
            </div>
          </div>

          <div className="flex justify-between pt-4 border-t border-slate-800">
            <button
              onClick={() => setStep(3)}
              className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold"
            >
              Back
            </button>
            <button
              onClick={() => setStep(5)}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow transition"
            >
              <span>Review & Deploy</span>
              <ArrowRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* STEP 5: Review and Deploy */}
      {step === 5 && (
        <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm space-y-6">
          <div>
            <h2 className="text-base font-bold text-white">Review Configuration Summary</h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Confirm your specifications. Provisioning runs asynchronously on Proxmox VE.
            </p>
          </div>

          <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-5 divide-y divide-slate-800/80 text-xs">
            <div className="py-2.5 flex justify-between">
              <span className="text-slate-400">Instance Name</span>
              <span className="font-semibold text-white font-mono">{name}</span>
            </div>
            <div className="py-2.5 flex justify-between">
              <span className="text-slate-400">Operating System</span>
              <span className="font-semibold text-white">{selectedTemplate?.name}</span>
            </div>
            <div className="py-2.5 flex justify-between">
              <span className="text-slate-400">Compute (vCPU)</span>
              <span className="font-semibold text-white font-mono">{currentResources.vcpus} vCPU</span>
            </div>
            <div className="py-2.5 flex justify-between">
              <span className="text-slate-400">Memory (RAM)</span>
              <span className="font-semibold text-white font-mono">
                {Math.round(currentResources.memoryMB / 1024)} GB ({currentResources.memoryMB} MB)
              </span>
            </div>
            <div className="py-2.5 flex justify-between">
              <span className="text-slate-400">Storage (SSD)</span>
              <span className="font-semibold text-white font-mono">{currentResources.diskGB} GB</span>
            </div>
            <div className="py-2.5 flex justify-between">
              <span className="text-slate-400">SSH Public Key</span>
              <span className="font-semibold text-blue-400 font-mono">
                {sshKeys.find((k) => k.id === selectedSSHKeyId)?.name || 'Default key'}
              </span>
            </div>
            <div className="py-2.5 flex justify-between">
              <span className="text-slate-400">Hypervisor Target</span>
              <span className="font-semibold text-emerald-400">Dell PowerEdge R640 (pve-dell-r640)</span>
            </div>
          </div>

          <div className="p-3.5 rounded-xl bg-blue-950/30 border border-blue-900/40 text-xs text-blue-300 flex items-start gap-2.5">
            <Zap className="w-4 h-4 flex-shrink-0 mt-0.5 text-blue-400" />
            <div>
              <p className="font-semibold">Asynchronous Provisioning</p>
              <p className="mt-0.5 text-blue-300/80 leading-relaxed">
                Clicking Create Instance will enqueue an automated job. The system clones the base
                cloud image, sets up cloud-init with your SSH key, boots the VM, and assigns an IP address.
                You can monitor real-time progress or navigate away at any time.
              </p>
            </div>
          </div>

          <div className="flex justify-between pt-4 border-t border-slate-800">
            <button
              onClick={() => setStep(4)}
              className="px-4 py-2 rounded-xl bg-slate-800 hover:bg-slate-700 text-slate-300 text-xs font-semibold"
            >
              Back
            </button>
            <button
              onClick={handleSubmit}
              disabled={submitting}
              className="flex items-center gap-2 px-6 py-2.5 rounded-xl bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold shadow-lg shadow-blue-500/25 transition disabled:opacity-50"
            >
              {submitting ? (
                <>
                  <RefreshCw className="w-4 h-4 animate-spin" />
                  <span>Enqueuing Provisioning Job...</span>
                </>
              ) : (
                <>
                  <span>Create Instance</span>
                  <ArrowRight className="w-4 h-4" />
                </>
              )}
            </button>
          </div>
        </div>
      )}

      {/* Modal: Add SSH Key */}
      <Modal
        isOpen={showAddKeyModal}
        onClose={() => setShowAddKeyModal(false)}
        title="Add New SSH Public Key"
        description="Paste your public key (.pub) to authenticate into your Linux servers."
      >
        <div className="space-y-4">
          {keyError && (
            <div className="p-3 rounded-xl bg-rose-950/60 border border-rose-800 text-rose-300 text-xs">
              {keyError}
            </div>
          )}

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">Key Name</label>
            <input
              type="text"
              placeholder="e.g. personal-macbook"
              value={newKeyName}
              onChange={(e) => setNewKeyName(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-600 focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
          </div>

          <div>
            <label className="block text-xs font-medium text-slate-300 mb-1">
              Public Key Content
            </label>
            <textarea
              rows={4}
              placeholder="ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAA... user@host"
              value={newPublicKey}
              onChange={(e) => setNewPublicKey(e.target.value)}
              className="w-full bg-slate-950 border border-slate-800 rounded-xl px-3 py-2 text-xs text-white placeholder-slate-600 font-mono focus:outline-none focus:ring-2 focus:ring-blue-500"
            />
            <span className="text-[10px] text-slate-500 mt-1 block">
              Only public keys are stored. Private keys are never accepted.
            </span>
          </div>

          <div className="flex justify-end gap-3 pt-3 border-t border-slate-800">
            <button
              onClick={() => setShowAddKeyModal(false)}
              className="px-4 py-2 rounded-xl bg-slate-800 text-slate-300 text-xs font-semibold hover:bg-slate-700"
            >
              Cancel
            </button>
            <button
              onClick={handleCreateKey}
              disabled={creatingKey || !newKeyName.trim() || !newPublicKey.trim()}
              className="px-4 py-2 rounded-xl bg-blue-600 text-white text-xs font-semibold hover:bg-blue-500 disabled:opacity-50"
            >
              {creatingKey ? 'Validating...' : 'Save SSH Key'}
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
};
