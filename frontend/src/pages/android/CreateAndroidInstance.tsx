import React, { useState, useEffect } from 'react';
import {
  Smartphone,
  Layers,
  Cpu,
  Monitor,
  Shield,
  ArrowRight,
  ArrowLeft,
  Check,
  Sparkles,
  AlertCircle,
  HardDrive,
  RefreshCw,
  Terminal,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import { AndroidImage, AndroidPlan } from '../../types';

interface CreateAndroidInstanceProps {
  onNavigate: (path: string, param?: string) => void;
}

export const CreateAndroidInstance: React.FC<CreateAndroidInstanceProps> = ({ onNavigate }) => {
  const [step, setStep] = useState<1 | 2 | 3 | 4>(1);
  const [images, setImages] = useState<AndroidImage[]>([]);
  const [plans, setPlans] = useState<AndroidPlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Form State
  const [name, setName] = useState('my-android-device');
  const [selectedImageId, setSelectedImageId] = useState('');
  const [selectedPlan, setSelectedPlan] = useState('developer');
  const [customVCPUs, setCustomVCPUs] = useState(2);
  const [customRAMMB, setCustomRAMMB] = useState(4096);
  const [customStorageGB, setCustomStorageGB] = useState(32);

  // Display & Access
  const [resolution, setResolution] = useState('720x1280');
  const [density, setDensity] = useState(240);
  const [orientation, setOrientation] = useState<'portrait' | 'landscape'>('portrait');
  const [adbEnabled, setAdbEnabled] = useState(true);
  const [autoBackupEnabled, setAutoBackupEnabled] = useState(true);

  useEffect(() => {
    const fetchPrerequisites = async () => {
      try {
        setLoading(true);
        const [imgs, pls] = await Promise.all([
          apiRequest<AndroidImage[]>('/android/images'),
          apiRequest<AndroidPlan[]>('/android/plans'),
        ]);
        setImages(imgs || []);
        setPlans(pls || []);
        if (imgs && imgs.length > 0) {
          setSelectedImageId(imgs[0].id);
        }
      } catch (err: any) {
        console.error('Failed to load provisioning prerequisites:', err);
        setError('Failed to fetch available Android images and plans.');
      } finally {
        setLoading(false);
      }
    };
    fetchPrerequisites();
  }, []);

  const handleResolutionChange = (res: string) => {
    setResolution(res);
    if (res.startsWith('1280x720') || res.startsWith('1920x1080')) {
      setOrientation('landscape');
    } else {
      setOrientation('portrait');
    }
  };

  const handleDeploy = async () => {
    try {
      setSubmitting(true);
      setError(null);

      let vcpus = 2;
      let memory_mb = 4096;
      let storage_gb = 32;

      const p = plans.find((x) => x.id === selectedPlan);
      if (p) {
        vcpus = p.vcpus;
        memory_mb = p.memory_mb;
        storage_gb = p.storage_gb;
      } else if (selectedPlan === 'custom') {
        vcpus = customVCPUs;
        memory_mb = customRAMMB;
        storage_gb = customStorageGB;
      }

      const payload = {
        name,
        image_id: selectedImageId,
        plan_name: selectedPlan,
        vcpus,
        memory_mb,
        storage_gb,
        display_resolution: resolution,
        display_density: density,
        orientation,
        remote_access_type: 'browser',
        adb_enabled: adbEnabled,
        auto_backup_enabled: autoBackupEnabled,
      };

      const res = await apiRequest<{ id: string }>('/android/instances', {
        method: 'POST',
        body: JSON.stringify(payload),
      });

      onNavigate(`/dashboard/android/${res.id}`);
    } catch (err: any) {
      setError(err.message || 'Provisioning failed.');
    } finally {
      setSubmitting(false);
    }
  };

  const selectedImageObj = images.find((i) => i.id === selectedImageId);
  const selectedPlanObj = plans.find((p) => p.id === selectedPlan);

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <button
            onClick={() => onNavigate('/dashboard/android')}
            className="flex items-center gap-2 text-xs text-slate-400 hover:text-white transition-colors mb-2"
          >
            <ArrowLeft className="w-3.5 h-3.5" />
            Back to Android Instances
          </button>
          <h1 className="text-2xl font-bold text-white tracking-tight flex items-center gap-3">
            <Smartphone className="w-7 h-7 text-emerald-400" />
            Provision Android Waydroid Instance
          </h1>
          <p className="text-sm text-slate-400 mt-0.5">
            Deploy an on-demand containerized Android environment in seconds
          </p>
        </div>

        <button
          onClick={() => onNavigate('/dashboard/ai')}
          className="hidden sm:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-emerald-500/10 hover:bg-emerald-500/20 border border-emerald-500/30 text-emerald-300 text-xs font-medium transition-colors"
        >
          <Sparkles className="w-3.5 h-3.5" />
          <span>Provision via AI Agent</span>
        </button>
      </div>

      {/* Stepper Progress */}
      <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800">
        <div className="grid grid-cols-4 gap-2 text-center text-xs">
          <div
            className={`flex items-center justify-center gap-2 p-2 rounded-lg font-medium ${
              step >= 1
                ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                : 'text-slate-500'
            }`}
          >
            <span className="w-5 h-5 rounded-full bg-emerald-500/20 flex items-center justify-center text-[10px]">
              1
            </span>
            <span className="hidden sm:inline">OS Image</span>
          </div>
          <div
            className={`flex items-center justify-center gap-2 p-2 rounded-lg font-medium ${
              step >= 2
                ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                : 'text-slate-500'
            }`}
          >
            <span className="w-5 h-5 rounded-full bg-emerald-500/20 flex items-center justify-center text-[10px]">
              2
            </span>
            <span className="hidden sm:inline">Resources</span>
          </div>
          <div
            className={`flex items-center justify-center gap-2 p-2 rounded-lg font-medium ${
              step >= 3
                ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                : 'text-slate-500'
            }`}
          >
            <span className="w-5 h-5 rounded-full bg-emerald-500/20 flex items-center justify-center text-[10px]">
              3
            </span>
            <span className="hidden sm:inline">Display & ADB</span>
          </div>
          <div
            className={`flex items-center justify-center gap-2 p-2 rounded-lg font-medium ${
              step >= 4
                ? 'bg-emerald-500/20 text-emerald-300 border border-emerald-500/30'
                : 'text-slate-500'
            }`}
          >
            <span className="w-5 h-5 rounded-full bg-emerald-500/20 flex items-center justify-center text-[10px]">
              4
            </span>
            <span className="hidden sm:inline">Review & Deploy</span>
          </div>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-red-950/40 border border-red-500/40 flex items-center gap-3 text-red-300 text-sm">
          <AlertCircle className="w-5 h-5 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Wizard Steps */}
      {loading ? (
        <div className="p-16 text-center text-slate-400 bg-slate-900/40 rounded-xl border border-slate-800">
          <RefreshCw className="w-8 h-8 text-emerald-400 animate-spin mx-auto mb-3" />
          <p className="text-sm">Fetching available Android images and resource tiers...</p>
        </div>
      ) : (
        <div className="p-6 rounded-2xl bg-slate-900/90 border border-slate-800 space-y-6">
          {/* STEP 1: OS Image Selection */}
          {step === 1 && (
            <div className="space-y-5">
              <div>
                <h3 className="text-base font-semibold text-white">Select Android Image</h3>
                <p className="text-xs text-slate-400 mt-1">
                  Choose an optimized Waydroid system image for your workload.
                </p>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                {images.map((img) => {
                  const isSelected = selectedImageId === img.id;
                  return (
                    <div
                      key={img.id}
                      onClick={() => setSelectedImageId(img.id)}
                      className={`relative p-5 rounded-xl border transition-all cursor-pointer flex flex-col justify-between ${
                        isSelected
                          ? 'bg-emerald-500/10 border-emerald-500/80 shadow-lg shadow-emerald-500/10'
                          : 'bg-slate-950/60 border-slate-800 hover:border-slate-700'
                      }`}
                    >
                      <div>
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-mono uppercase px-2 py-0.5 rounded-full bg-slate-800 text-slate-300 border border-slate-700">
                            Android {img.android_version}
                          </span>
                          {isSelected && <Check className="w-4 h-4 text-emerald-400" />}
                        </div>
                        <h4 className="text-sm font-semibold text-white mt-3">{img.name}</h4>
                        <p className="text-xs text-slate-400 mt-1 line-clamp-3 leading-relaxed">
                          {img.description}
                        </p>
                      </div>

                      <div className="mt-4 pt-3 border-t border-slate-800/80 text-[11px] text-slate-500 flex justify-between">
                        <span>Min {img.min_cpu} vCPU</span>
                        <span>{Math.round(img.min_ram_mb / 1024)} GB RAM</span>
                      </div>
                    </div>
                  );
                })}
              </div>

              <div className="pt-4 border-t border-slate-800">
                <label className="block text-xs font-medium text-slate-300 mb-1.5">
                  Instance Name
                </label>
                <input
                  type="text"
                  value={name}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. android-qa-phone-01"
                  className="w-full max-w-md px-3.5 py-2 rounded-lg bg-slate-950 border border-slate-800 text-white text-sm focus:outline-none focus:border-emerald-500"
                />
              </div>
            </div>
          )}

          {/* STEP 2: Resource Plan */}
          {step === 2 && (
            <div className="space-y-5">
              <div>
                <h3 className="text-base font-semibold text-white">Select Hardware Sizing Tier</h3>
                <p className="text-xs text-slate-400 mt-1">
                  Waydroid shares the Linux kernel while isolating user space and graphical framebuffers.
                </p>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                {plans.map((p) => {
                  const isSelected = selectedPlan === p.id;
                  return (
                    <div
                      key={p.id}
                      onClick={() => setSelectedPlan(p.id)}
                      className={`p-5 rounded-xl border transition-all cursor-pointer flex flex-col justify-between ${
                        isSelected
                          ? 'bg-emerald-500/10 border-emerald-500/80 shadow-lg shadow-emerald-500/10'
                          : 'bg-slate-950/60 border-slate-800 hover:border-slate-700'
                      }`}
                    >
                      <div>
                        <div className="flex items-center justify-between">
                          <h4 className="text-sm font-semibold text-white">{p.name}</h4>
                          {isSelected && <Check className="w-4 h-4 text-emerald-400" />}
                        </div>
                        <div className="mt-3 space-y-1.5 text-xs text-slate-300">
                          <div className="flex items-center gap-2">
                            <Cpu className="w-3.5 h-3.5 text-slate-500" />
                            <span>{p.vcpus} vCPU</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <Layers className="w-3.5 h-3.5 text-slate-500" />
                            <span>{Math.round(p.memory_mb / 1024)} GB Memory</span>
                          </div>
                          <div className="flex items-center gap-2">
                            <HardDrive className="w-3.5 h-3.5 text-slate-500" />
                            <span>{p.storage_gb} GB ZFS Data</span>
                          </div>
                        </div>
                      </div>

                      <p className="mt-4 pt-3 border-t border-slate-800/80 text-[11px] text-slate-400">
                        {p.suggested_use}
                      </p>
                    </div>
                  );
                })}
              </div>
            </div>
          )}

          {/* STEP 3: Display & Networking */}
          {step === 3 && (
            <div className="space-y-6">
              <div>
                <h3 className="text-base font-semibold text-white">Display & Remote Streaming</h3>
                <p className="text-xs text-slate-400 mt-1">
                  Configure virtual screen dimensions, DPI scaling, and ADB debugging options.
                </p>
              </div>

              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <div className="space-y-3">
                  <label className="block text-xs font-medium text-slate-300">
                    Virtual Screen Resolution
                  </label>
                  <div className="grid grid-cols-2 gap-2">
                    {[
                      { id: '720x1280', label: '720 × 1280 (Phone Portrait)' },
                      { id: '1280x720', label: '1280 × 720 (Tablet Landscape)' },
                      { id: '1080x1920', label: '1080 × 1920 (FHD Portrait)' },
                      { id: '1920x1080', label: '1920 × 1080 (FHD Landscape)' },
                    ].map((opt) => (
                      <button
                        key={opt.id}
                        type="button"
                        onClick={() => handleResolutionChange(opt.id)}
                        className={`p-3 rounded-lg border text-left text-xs transition-all ${
                          resolution === opt.id
                            ? 'bg-emerald-500/20 border-emerald-500/80 text-white font-medium'
                            : 'bg-slate-950/60 border-slate-800 text-slate-400 hover:text-white'
                        }`}
                      >
                        {opt.label}
                      </button>
                    ))}
                  </div>
                </div>

                <div className="space-y-3">
                  <label className="block text-xs font-medium text-slate-300">
                    Display Density (DPI)
                  </label>
                  <div className="grid grid-cols-3 gap-2">
                    {[
                      { val: 160, label: '160 DPI (MDPI)' },
                      { val: 240, label: '240 DPI (HDPI)' },
                      { val: 320, label: '320 DPI (XHDPI)' },
                    ].map((dpiOpt) => (
                      <button
                        key={dpiOpt.val}
                        type="button"
                        onClick={() => setDensity(dpiOpt.val)}
                        className={`p-3 rounded-lg border text-center text-xs transition-all ${
                          density === dpiOpt.val
                            ? 'bg-emerald-500/20 border-emerald-500/80 text-white font-medium'
                            : 'bg-slate-950/60 border-slate-800 text-slate-400 hover:text-white'
                        }`}
                      >
                        {dpiOpt.label}
                      </button>
                    ))}
                  </div>
                </div>
              </div>

              <div className="pt-4 border-t border-slate-800 space-y-4">
                <div className="flex items-center justify-between p-3.5 rounded-xl bg-slate-950/60 border border-slate-800">
                  <div className="flex items-center gap-3">
                    <Terminal className="w-5 h-5 text-emerald-400" />
                    <div>
                      <div className="text-sm font-medium text-white">Enable ADB over TCP/IP</div>
                      <div className="text-xs text-slate-400">
                        Allow connecting via <span className="font-mono text-emerald-300">adb connect cloud.nevatal.id:5555</span> for developer tooling.
                      </div>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={adbEnabled}
                    onChange={(e) => setAdbEnabled(e.target.checked)}
                    className="w-4 h-4 rounded text-emerald-500 focus:ring-emerald-400 focus:ring-offset-slate-900 border-slate-700 bg-slate-800"
                  />
                </div>

                <div className="flex items-center justify-between p-3.5 rounded-xl bg-slate-950/60 border border-slate-800">
                  <div className="flex items-center gap-3">
                    <Shield className="w-5 h-5 text-emerald-400" />
                    <div>
                      <div className="text-sm font-medium text-white">Automated Daily Snapshots</div>
                      <div className="text-xs text-slate-400">
                        Persist and snapshot Android user data (<span className="font-mono text-slate-300">/data</span> partition) automatically every 24 hours.
                      </div>
                    </div>
                  </div>
                  <input
                    type="checkbox"
                    checked={autoBackupEnabled}
                    onChange={(e) => setAutoBackupEnabled(e.target.checked)}
                    className="w-4 h-4 rounded text-emerald-500 focus:ring-emerald-400 focus:ring-offset-slate-900 border-slate-700 bg-slate-800"
                  />
                </div>
              </div>
            </div>
          )}

          {/* STEP 4: Review & Deploy */}
          {step === 4 && (
            <div className="space-y-6">
              <div>
                <h3 className="text-base font-semibold text-white">Review & Launch</h3>
                <p className="text-xs text-slate-400 mt-1">
                  Confirm your container parameters before provisioning.
                </p>
              </div>

              <div className="p-5 rounded-xl bg-slate-950 border border-slate-800 space-y-4">
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 text-xs">
                  <div>
                    <div className="text-slate-500">Instance Name:</div>
                    <div className="text-white font-medium mt-0.5">{name}</div>
                  </div>
                  <div>
                    <div className="text-slate-500">Android Image:</div>
                    <div className="text-white font-medium mt-0.5">
                      {selectedImageObj?.name || 'LineageOS 20'}
                    </div>
                  </div>
                  <div>
                    <div className="text-slate-500">Selected Plan:</div>
                    <div className="text-white font-medium mt-0.5 capitalize">
                      {selectedPlanObj?.name || selectedPlan}
                    </div>
                  </div>
                  <div>
                    <div className="text-slate-500">Hardware Allocation:</div>
                    <div className="text-white font-medium mt-0.5">
                      {selectedPlanObj?.vcpus} vCPU • {Math.round((selectedPlanObj?.memory_mb || 0) / 1024)} GB RAM
                    </div>
                  </div>
                  <div>
                    <div className="text-slate-500">Persistent Storage:</div>
                    <div className="text-white font-medium mt-0.5">
                      {selectedPlanObj?.storage_gb} GB ZFS Volume
                    </div>
                  </div>
                  <div>
                    <div className="text-slate-500">Display Resolution:</div>
                    <div className="text-white font-medium mt-0.5">
                      {resolution} ({density} DPI, {orientation})
                    </div>
                  </div>
                  <div>
                    <div className="text-slate-500">ADB Bridging:</div>
                    <div className="text-emerald-400 font-medium mt-0.5">
                      {adbEnabled ? 'Enabled (Port 5555)' : 'Disabled'}
                    </div>
                  </div>
                  <div>
                    <div className="text-slate-500">Remote Access:</div>
                    <div className="text-white font-medium mt-0.5">Browser WebRTC/Canvas Stream</div>
                  </div>
                  <div>
                    <div className="text-slate-500">Daily Snapshots:</div>
                    <div className="text-white font-medium mt-0.5">
                      {autoBackupEnabled ? 'Enabled' : 'Disabled'}
                    </div>
                  </div>
                </div>
              </div>

              <div className="p-4 rounded-xl bg-emerald-500/10 border border-emerald-500/20 text-xs text-emerald-300 leading-relaxed">
                🚀 Your Android environment will be provisioned inside a sandboxed Linux container with direct hardware binder virtualization. Once active, you will be automatically redirected to the browser remote display!
              </div>
            </div>
          )}

          {/* Stepper Navigation Buttons */}
          <div className="pt-4 border-t border-slate-800 flex items-center justify-between">
            {step > 1 ? (
              <button
                type="button"
                onClick={() => setStep((s) => (s - 1) as any)}
                className="flex items-center gap-1.5 px-4 py-2 rounded-lg border border-slate-700 bg-slate-800 text-slate-300 hover:text-white text-xs font-medium transition-colors"
              >
                <ArrowLeft className="w-3.5 h-3.5" />
                Previous
              </button>
            ) : (
              <div />
            )}

            {step < 4 ? (
              <button
                type="button"
                onClick={() => setStep((s) => (s + 1) as any)}
                className="flex items-center gap-1.5 px-5 py-2 rounded-lg bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-medium transition-all shadow-lg shadow-emerald-500/20"
              >
                Next Step
                <ArrowRight className="w-3.5 h-3.5" />
              </button>
            ) : (
              <button
                type="button"
                disabled={submitting}
                onClick={handleDeploy}
                className="flex items-center gap-2 px-6 py-2 rounded-lg bg-gradient-to-r from-emerald-600 to-teal-600 hover:from-emerald-500 hover:to-teal-500 text-white text-sm font-semibold transition-all shadow-lg shadow-emerald-500/20 disabled:opacity-50"
              >
                {submitting ? (
                  <>
                    <RefreshCw className="w-4 h-4 animate-spin" />
                    <span>Provisioning Waydroid...</span>
                  </>
                ) : (
                  <>
                    <Check className="w-4 h-4" />
                    <span>Confirm & Launch Instance</span>
                  </>
                )}
              </button>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
