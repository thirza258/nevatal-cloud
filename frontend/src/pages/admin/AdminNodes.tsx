import React, { useState, useEffect } from 'react';
import { apiRequest } from '../../lib/api';
import { Cpu, HardDrive, Server, ShieldCheck, RefreshCw, CheckCircle2 } from 'lucide-react';

export const AdminNodes: React.FC = () => {
  const [nodes, setNodes] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchNodes = async () => {
    try {
      const data = await apiRequest<any[]>('/admin/nodes');
      setNodes(data || []);
    } catch (err) {
      console.error('Failed to load nodes:', err);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchNodes();
  }, []);

  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-xl sm:text-2xl font-bold text-white tracking-tight">Hypervisor Nodes</h1>
        <p className="text-xs sm:text-sm text-slate-400 mt-0.5">
          Proxmox VE cluster compute nodes and storage endpoints
        </p>
      </div>

      {loading ? (
        <div className="p-12 text-center text-slate-400 text-xs">Querying cluster nodes...</div>
      ) : (
        <div className="grid grid-cols-1 gap-6">
          {nodes.map((n) => {
            const live = n.live_status;
            return (
              <div
                key={n.id}
                className="bg-slate-900 border border-slate-800 rounded-2xl p-6 shadow-sm space-y-6"
              >
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 border-b border-slate-800 pb-4">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-xl bg-slate-800 flex items-center justify-center text-blue-400">
                      <Server className="w-5 h-5" />
                    </div>
                    <div>
                      <h3 className="text-base font-bold text-white">{n.name}</h3>
                      <p className="text-xs text-slate-500 font-mono">
                        Node identifier: {n.proxmox_node_name} • Architecture: x86_64
                      </p>
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="w-2.5 h-2.5 rounded-full bg-emerald-400 animate-pulse" />
                    <span className="text-xs font-semibold text-emerald-400 font-mono">
                      HYPERVISOR ACTIVE
                    </span>
                  </div>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                  <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4">
                    <span className="text-[11px] font-semibold uppercase text-slate-500 block">
                      Processor Cores
                    </span>
                    <span className="text-lg font-bold text-white mt-1 block">
                      {n.total_cpu_threads} Threads
                    </span>
                    <span className="text-[11px] text-slate-500">Dual Intel Xeon Scalable</span>
                  </div>

                  <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4">
                    <span className="text-[11px] font-semibold uppercase text-slate-500 block">
                      Installed Memory
                    </span>
                    <span className="text-lg font-bold text-white mt-1 block">
                      {Math.round(n.total_memory_mb / 1024)} GB RAM
                    </span>
                    <span className="text-[11px] text-slate-500">ECC Registered DDR4</span>
                  </div>

                  <div className="bg-slate-950/60 border border-slate-800 rounded-xl p-4">
                    <span className="text-[11px] font-semibold uppercase text-slate-500 block">
                      Primary Storage Pool
                    </span>
                    <span className="text-lg font-bold text-white mt-1 block">
                      {Math.round(n.total_disk_gb / 1024)} TB Capacity
                    </span>
                    <span className="text-[11px] text-slate-500">Enterprise ZFS Pool</span>
                  </div>
                </div>

                {live && (
                  <div className="p-4 rounded-xl bg-slate-950/40 border border-slate-800 text-xs space-y-2">
                    <div className="font-semibold text-slate-300">Proxmox VE Daemon Telemetry:</div>
                    <div className="grid grid-cols-2 sm:grid-cols-3 gap-2 font-mono text-[11px] text-slate-400">
                      <div>Version: {live.pveversion}</div>
                      <div>Kernel: {live.kversion}</div>
                      <div>Uptime: {Math.floor((live.uptime || 0) / 86400)} days</div>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
