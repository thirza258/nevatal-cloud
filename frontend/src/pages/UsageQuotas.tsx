import React, { useState, useEffect } from 'react';
import { QuotaUsage } from '../types';
import { apiRequest } from '../lib/api';
import { ProgressBar } from '../components/ui/ProgressBar';
import { PieChart, ShieldCheck, Cpu, HardDrive, Server, Layers, Info } from 'lucide-react';

export const UsageQuotas: React.FC = () => {
  const [usage, setUsage] = useState<QuotaUsage | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const fetchUsage = async () => {
      try {
        const data = await apiRequest<QuotaUsage>('/usage');
        setUsage(data);
      } catch (err) {
        console.error('Failed to fetch usage:', err);
      } finally {
        setLoading(false);
      }
    };
    fetchUsage();
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">Resource Quotas & Allocation</h1>
        <p className="text-xs sm:text-sm text-slate-400 mt-0.5">
          Authoritative allocation accounting enforced by the platform control plane
        </p>
      </div>

      {loading || !usage ? (
        <div className="p-12 text-center text-slate-400 text-xs">Loading quota usage...</div>
      ) : (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 space-y-4 shadow-sm">
              <div className="flex items-center gap-2.5 pb-2 border-b border-slate-800">
                <Cpu className="w-5 h-5 text-blue-400" />
                <h3 className="text-sm font-bold text-white">vCPU Compute Cores</h3>
              </div>
              <ProgressBar
                label="Allocated Virtual Cores"
                used={usage.used_vcpus}
                total={usage.max_vcpus}
                unit="Cores"
              />
              <p className="text-xs text-slate-500">
                Available to allocate:{' '}
                <strong className="text-white">
                  {Math.max(0, usage.max_vcpus - usage.used_vcpus)} vCPUs
                </strong>
              </p>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 space-y-4 shadow-sm">
              <div className="flex items-center gap-2.5 pb-2 border-b border-slate-800">
                <Server className="w-5 h-5 text-purple-400" />
                <h3 className="text-sm font-bold text-white">System Memory (RAM)</h3>
              </div>
              <ProgressBar
                label="Allocated Memory"
                used={Math.round(usage.used_memory_mb / 1024)}
                total={Math.round(usage.max_memory_mb / 1024)}
                unit="GB"
              />
              <p className="text-xs text-slate-500">
                Available to allocate:{' '}
                <strong className="text-white">
                  {Math.max(0, Math.round((usage.max_memory_mb - usage.used_memory_mb) / 1024))} GB
                </strong>{' '}
                ({Math.max(0, usage.max_memory_mb - usage.used_memory_mb)} MB)
              </p>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 space-y-4 shadow-sm">
              <div className="flex items-center gap-2.5 pb-2 border-b border-slate-800">
                <HardDrive className="w-5 h-5 text-emerald-400" />
                <h3 className="text-sm font-bold text-white">Storage Capacity (SSD)</h3>
              </div>
              <ProgressBar
                label="Allocated Virtual Disks"
                used={usage.used_storage_gb}
                total={usage.max_storage_gb}
                unit="GB"
              />
              <p className="text-xs text-slate-500">
                Available to allocate:{' '}
                <strong className="text-white">
                  {Math.max(0, usage.max_storage_gb - usage.used_storage_gb)} GB
                </strong>
              </p>
            </div>

            <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 space-y-4 shadow-sm">
              <div className="flex items-center gap-2.5 pb-2 border-b border-slate-800">
                <Layers className="w-5 h-5 text-amber-400" />
                <h3 className="text-sm font-bold text-white">Max Virtual Instances</h3>
              </div>
              <ProgressBar
                label="Active & Queued Instances"
                used={usage.used_instances}
                total={usage.max_instances}
                unit="VMs"
              />
              <p className="text-xs text-slate-500">
                Available to launch:{' '}
                <strong className="text-white">
                  {Math.max(0, usage.max_instances - usage.used_instances)} instances
                </strong>
              </p>
            </div>
          </div>

          {/* PRD Section 13.2 Allocation Policy Disclosure */}
          <div className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm space-y-3">
            <div className="flex items-center gap-2 text-blue-400 font-semibold text-sm">
              <Info className="w-4 h-4" />
              <span>Allocation Accounting Policy</span>
            </div>
            <p className="text-xs text-slate-400 leading-relaxed">
              In accordance with Nevatal Cloud PRD v1.0 Section 13.2, resource quotas measure{' '}
              <strong className="text-slate-200">reserved allocations</strong> rather than guest-level
              momentary utilization. Stopping an instance retains its allocated disk, CPU, and memory quota
              so that it can immediately reboot with guaranteed physical capacity on the Dell PowerEdge R640.
              Permanently destroying an instance releases its quotas back to your pool.
            </p>
          </div>
        </>
      )}
    </div>
  );
};
