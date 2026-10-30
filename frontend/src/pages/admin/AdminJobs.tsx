import React, { useState, useEffect } from 'react';
import { ProvisioningJob } from '../../types';
import { apiRequest } from '../../lib/api';
import { StatusBadge } from '../../components/ui/Badge';
import { RefreshCw, Play, AlertCircle, CheckCircle2 } from 'lucide-react';

export const AdminJobs: React.FC = () => {
  const [jobs, setJobs] = useState<ProvisioningJob[]>([]);
  const [loading, setLoading] = useState(true);
  const [retrying, setRetrying] = useState<string | null>(null);

  const fetchJobs = async () => {
    try {
      const data = await apiRequest<ProvisioningJob[]>('/admin/jobs');
      setJobs(data || []);
    } catch (err) {
      console.error('Failed to load jobs:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchJobs();
    const interval = setInterval(fetchJobs, 4000);
    return () => clearInterval(interval);
  }, []);

  const handleRetry = async (jobId: string) => {
    setRetrying(jobId);
    try {
      await apiRequest(`/admin/jobs/${jobId}/retry`, { method: 'POST' });
      await fetchJobs();
    } catch (err: any) {
      alert(err.message || 'Failed to retry job');
    } finally {
      setRetrying(null);
    }
  };

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">Provisioning Pipeline</h1>
          <p className="text-xs sm:text-sm text-slate-400 mt-0.5">
            Real-time hypervisor execution queue and task failure diagnostics
          </p>
        </div>
      </div>

      <div className="bg-slate-900 border border-slate-800 rounded-2xl overflow-hidden shadow-sm">
        {loading ? (
          <div className="p-12 text-center text-slate-400 text-xs">Loading queue...</div>
        ) : jobs.length === 0 ? (
          <div className="p-12 text-center text-slate-400 text-xs">No provisioning jobs queued.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left border-collapse text-xs">
              <thead>
                <tr className="border-b border-slate-800 bg-slate-950/60 text-slate-400 font-medium uppercase tracking-wider">
                  <th className="py-3.5 px-4">Job ID</th>
                  <th className="py-3.5 px-4">Operation</th>
                  <th className="py-3.5 px-4">Status</th>
                  <th className="py-3.5 px-4">Current Step</th>
                  <th className="py-3.5 px-4">Progress</th>
                  <th className="py-3.5 px-4">Error Diagnostics</th>
                  <th className="py-3.5 px-4 text-right">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60 font-mono">
                {jobs.map((job) => (
                  <tr key={job.id} className="hover:bg-slate-800/30 transition">
                    <td className="py-3.5 px-4 text-slate-300 select-all">
                      {job.id.substring(0, 13)}...
                    </td>
                    <td className="py-3.5 px-4 font-semibold text-blue-400 uppercase font-sans">
                      {job.operation_type}
                    </td>
                    <td className="py-3.5 px-4 font-sans">
                      <StatusBadge status={job.status} size="sm" />
                    </td>
                    <td className="py-3.5 px-4 text-slate-300">
                      {job.current_step}
                    </td>
                    <td className="py-3.5 px-4">
                      <div className="flex items-center gap-2">
                        <div className="w-16 h-1.5 bg-slate-800 rounded-full overflow-hidden">
                          <div
                            className={`h-full rounded-full ${
                              job.status === 'completed'
                                ? 'bg-emerald-500'
                                : job.status === 'failed'
                                ? 'bg-rose-500'
                                : 'bg-blue-500'
                            }`}
                            style={{ width: `${job.progress_percent}%` }}
                          />
                        </div>
                        <span className="text-[11px] text-slate-400">{job.progress_percent}%</span>
                      </div>
                    </td>
                    <td className="py-3.5 px-4 text-rose-400 text-[11px] max-w-xs truncate font-sans">
                      {job.error_message || <span className="text-slate-600 font-mono">—</span>}
                    </td>
                    <td className="py-3.5 px-4 text-right font-sans">
                      {job.status === 'failed' && (
                        <button
                          onClick={() => handleRetry(job.id)}
                          disabled={retrying === job.id}
                          className="px-2.5 py-1 rounded-lg bg-amber-600/20 text-amber-300 hover:bg-amber-600 hover:text-white border border-amber-700/60 text-xs font-semibold transition"
                        >
                          {retrying === job.id ? 'Retrying...' : 'Retry Job'}
                        </button>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};
