import React, { useState, useEffect } from 'react';
import { ProvisioningJob } from '../types';
import { apiRequest } from '../lib/api';
import {
  Server,
  CheckCircle2,
  AlertTriangle,
  RefreshCw,
  ArrowRight,
  ShieldCheck,
  Cpu,
  Globe,
  Zap,
} from 'lucide-react';

interface JobProgressProps {
  jobId: string;
  onNavigate: (path: string, param?: string) => void;
}

export const JobProgress: React.FC<JobProgressProps> = ({ jobId, onNavigate }) => {
  const [job, setJob] = useState<ProvisioningJob | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchJob = async () => {
    try {
      const data = await apiRequest<ProvisioningJob>(`/jobs/${jobId}`);
      setJob(data);
    } catch (err) {
      console.error('Failed to poll job:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchJob();
    const interval = setInterval(() => {
      fetchJob();
    }, 1500);

    return () => clearInterval(interval);
  }, [jobId]);

  const steps = [
    { key: 'cloning_template', label: 'Clone OS Template', desc: 'Cloning verified template in Proxmox storage' },
    { key: 'configuring_hardware', label: 'Configure Hardware', desc: 'Allocating vCPU cores and RAM memory' },
    { key: 'configuring_cloud_init', label: 'Cloud-Init & SSH', desc: 'Injecting authorized SSH keys and user data' },
    { key: 'starting_vm', label: 'Start Virtual Machine', desc: 'Booting guest operating system on hypervisor' },
    { key: 'waiting_for_network', label: 'Network Discovery', desc: 'Acquiring private IP lease via QEMU agent' },
    { key: 'completed', label: 'Ready for Connection', desc: 'Virtual server is live and accepting SSH sessions' },
  ];

  const getStepStatus = (stepKey: string, index: number) => {
    if (!job) return 'pending';
    if (job.status === 'failed') return 'failed';
    if (job.status === 'completed') return 'done';

    const currentStepIndex = steps.findIndex((s) => s.key === job.current_step);
    if (currentStepIndex === -1) {
      // In early queue
      return index === 0 ? 'active' : 'pending';
    }

    if (index < currentStepIndex) return 'done';
    if (index === currentStepIndex) return 'active';
    return 'pending';
  };

  return (
    <div className="max-w-2xl mx-auto py-8 space-y-6">
      <div className="text-center space-y-2">
        <div className="inline-flex items-center justify-center w-12 h-12 rounded-2xl bg-blue-600/20 text-blue-400 border border-blue-500/30 mb-2">
          {job?.status === 'completed' ? (
            <CheckCircle2 className="w-6 h-6 text-emerald-400" />
          ) : job?.status === 'failed' ? (
            <AlertTriangle className="w-6 h-6 text-rose-400" />
          ) : (
            <RefreshCw className="w-6 h-6 animate-spin text-blue-400" />
          )}
        </div>
        <h1 className="text-2xl font-bold text-white tracking-tight">
          {job?.status === 'completed'
            ? 'Instance Successfully Deployed!'
            : job?.status === 'failed'
            ? 'Provisioning Encountered an Error'
            : 'Provisioning Virtual Machine...'}
        </h1>
        <p className="text-xs text-slate-400 font-mono">Job ID: {jobId}</p>
      </div>

      {/* Progress Card */}
      <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-xl space-y-6">
        {/* Progress Bar */}
        <div>
          <div className="flex justify-between text-xs text-slate-400 mb-2 font-medium">
            <span>Overall Progress</span>
            <span className="font-mono text-white font-bold">{job?.progress_percent || 10}%</span>
          </div>
          <div className="h-2.5 w-full bg-slate-950 rounded-full overflow-hidden border border-slate-800">
            <div
              className={`h-full rounded-full transition-all duration-700 ${
                job?.status === 'completed'
                  ? 'bg-emerald-500'
                  : job?.status === 'failed'
                  ? 'bg-rose-500'
                  : 'bg-blue-500'
              }`}
              style={{ width: `${job?.progress_percent || 10}%` }}
            />
          </div>
        </div>

        {/* Failure message if failed */}
        {job?.status === 'failed' && (
          <div className="p-4 rounded-xl bg-rose-950/60 border border-rose-800 text-rose-300 text-xs space-y-1">
            <p className="font-semibold">Error: {job.error_code || 'PROVISIONING_FAILED'}</p>
            <p className="text-rose-300/80">{job.error_message || 'An unexpected hypervisor error occurred.'}</p>
          </div>
        )}

        {/* Step-by-step checklist */}
        <div className="space-y-4 pt-2">
          {steps.map((s, idx) => {
            const status = getStepStatus(s.key, idx);
            return (
              <div key={s.key} className="flex items-start gap-3.5 text-xs">
                <div className="mt-0.5">
                  {status === 'done' ? (
                    <div className="w-5 h-5 rounded-full bg-emerald-950 text-emerald-400 border border-emerald-800 flex items-center justify-center">
                      <CheckCircle2 className="w-3.5 h-3.5 stroke-[2.5]" />
                    </div>
                  ) : status === 'active' ? (
                    <div className="w-5 h-5 rounded-full bg-blue-950 text-blue-400 border border-blue-800 flex items-center justify-center">
                      <RefreshCw className="w-3 h-3 animate-spin" />
                    </div>
                  ) : (
                    <div className="w-5 h-5 rounded-full bg-slate-950 text-slate-600 border border-slate-800 flex items-center justify-center">
                      <span className="text-[10px] font-mono">{idx + 1}</span>
                    </div>
                  )}
                </div>

                <div className="flex-1">
                  <div
                    className={`font-semibold ${
                      status === 'done'
                        ? 'text-slate-200'
                        : status === 'active'
                        ? 'text-blue-400'
                        : 'text-slate-500'
                    }`}
                  >
                    {s.label}
                  </div>
                  <div className="text-[11px] text-slate-500 mt-0.5">{s.desc}</div>
                </div>
              </div>
            );
          })}
        </div>

        {/* Bottom Actions */}
        <div className="pt-4 border-t border-slate-800 flex justify-between items-center">
          <button
            onClick={() => onNavigate('/dashboard/instances')}
            className="text-xs text-slate-400 hover:text-white"
          >
            Back to instances
          </button>

          {job?.status === 'completed' && job.instance_id && (
            <button
              onClick={() => onNavigate('/dashboard/instances', job.instance_id)}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-emerald-600 hover:bg-emerald-500 text-white text-xs font-semibold shadow-lg shadow-emerald-500/20 transition"
            >
              <span>Connect to Instance</span>
              <ArrowRight className="w-4 h-4" />
            </button>
          )}

          {job?.status === 'failed' && (
            <button
              onClick={() => onNavigate('/dashboard/instances/new')}
              className="flex items-center gap-2 px-4 py-2 rounded-xl bg-slate-800 text-white text-xs font-semibold hover:bg-slate-700"
            >
              Try Again
            </button>
          )}
        </div>
      </div>
    </div>
  );
};
