import React, { useState } from 'react';
import {
  Globe,
  ArrowLeft,
  GitBranch,
  Terminal,
  Cpu,
  Layers,
  Sparkles,
  CheckCircle2,
  AlertTriangle,
  Plus,
  Trash2
} from 'lucide-react';

interface CreateAppProps {
  onNavigate: (path: string, param?: string) => void;
}

interface EnvVarItem {
  key: string;
  value: string;
}

export const CreateApp: React.FC<CreateAppProps> = ({ onNavigate }) => {
  const [name, setName] = useState('');
  const [appType, setAppType] = useState('nextjs');
  const [sourceType, setSourceType] = useState<'git' | 'template'>('git');
  const [repoUrl, setRepoUrl] = useState('https://github.com/nevatal/web-starter');
  const [branch, setBranch] = useState('main');
  const [rootDirectory, setRootDirectory] = useState('/');
  const [buildCommand, setBuildCommand] = useState('npm run build');
  const [outputDirectory, setOutputDirectory] = useState('.next');
  const [startCommand, setStartCommand] = useState('npm run start');
  const [appPort, setAppPort] = useState(3000);
  const [vcpus, setVcpus] = useState(2);
  const [memoryMB, setMemoryMB] = useState(2048);
  const [diskGB, setDiskGB] = useState(15);
  const [envVars, setEnvVars] = useState<EnvVarItem[]>([
    { key: 'NODE_ENV', value: 'production' }
  ]);

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const presets: Record<string, { build: string; output: string; start: string; port: number }> = {
    nextjs: { build: 'npm run build', output: '.next', start: 'npm run start', port: 3000 },
    react: { build: 'npm run build', output: 'dist', start: 'serve -s dist -l 3000', port: 3000 },
    static: { build: '', output: '.', start: 'nginx -g "daemon off;"', port: 80 },
    nodejs: { build: 'npm install', output: '.', start: 'node server.js', port: 8080 },
    go: { build: 'go build -o app .', output: '.', start: './app', port: 8080 },
    python: { build: 'pip install -r requirements.txt', output: '.', start: 'uvicorn main:app --host 0.0.0.0 --port 8000', port: 8000 },
    docker: { build: 'docker build -t app .', output: '.', start: 'docker run -p 8080:8080 app', port: 8080 }
  };

  const handleTypeChange = (type: string) => {
    setAppType(type);
    const p = presets[type];
    if (p) {
      setBuildCommand(p.build);
      setOutputDirectory(p.output);
      setStartCommand(p.start);
      setAppPort(p.port);
    }
  };

  const addEnvVar = () => {
    setEnvVars([...envVars, { key: '', value: '' }]);
  };

  const removeEnvVar = (index: number) => {
    setEnvVars(envVars.filter((_, i) => i !== index));
  };

  const updateEnvVar = (index: number, field: 'key' | 'value', val: string) => {
    const updated = [...envVars];
    updated[index][field] = val;
    setEnvVars(updated);
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim()) {
      setError('Application name is required');
      return;
    }

    try {
      setLoading(true);
      setError(null);
      const token = localStorage.getItem('token');

      const envMap: Record<string, string> = {};
      envVars.forEach((ev) => {
        if (ev.key.trim()) {
          envMap[ev.key.trim()] = ev.value;
        }
      });

      const res = await fetch('/api/v1/apps', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`
        },
        body: JSON.stringify({
          name: name.trim(),
          application_type: appType,
          deployment_source_type: sourceType,
          repository_url: repoUrl,
          production_branch: branch,
          root_directory: rootDirectory,
          build_command: buildCommand,
          output_directory: outputDirectory,
          start_command: startCommand,
          application_port: appPort,
          health_check_path: '/',
          vcpus,
          memory_mb: memoryMB,
          disk_gb: diskGB,
          env_vars: envMap
        })
      });

      const data = await res.json();
      if (!res.ok) throw new Error(data.error?.message || 'Failed to create application');

      onNavigate('/dashboard/apps', data.data.id);
    } catch (err: any) {
      setError(err.message);
      setLoading(false);
    }
  };

  return (
    <div className="max-w-4xl mx-auto space-y-6">
      {/* Header */}
      <div className="flex items-center gap-4 border-b border-slate-800 pb-4">
        <button
          onClick={() => onNavigate('/dashboard/apps')}
          className="p-2 rounded-lg bg-slate-900 border border-slate-800 text-slate-400 hover:text-white hover:bg-slate-800 transition cursor-pointer"
        >
          <ArrowLeft className="w-4 h-4" />
        </button>
        <div>
          <h1 className="text-xl font-bold text-white tracking-tight">Deploy New Web Application</h1>
          <p className="text-xs text-slate-400">Configure continuous build settings, framework, and resources.</p>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-lg bg-red-500/10 border border-red-500/20 text-red-400 text-sm flex items-center gap-3">
          <AlertTriangle className="w-5 h-5 flex-shrink-0" />
          <span>{error}</span>
        </div>
      )}

      <form onSubmit={handleSubmit} className="space-y-6">
        {/* Step 1: Framework / Runtime */}
        <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4">
          <h2 className="text-sm font-semibold text-slate-200 uppercase tracking-wider flex items-center gap-2">
            <Sparkles className="w-4 h-4 text-blue-400" />
            1. Select Framework & Runtime
          </h2>

          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
            {[
              { id: 'nextjs', name: 'Next.js', tag: 'Fullstack / SSR' },
              { id: 'react', name: 'React / Vite', tag: 'Client SPA' },
              { id: 'static', name: 'Static HTML', tag: 'Fast CDN' },
              { id: 'nodejs', name: 'Node.js', tag: 'Express / Fastify' },
              { id: 'go', name: 'Go', tag: 'Gin / Fiber API' },
              { id: 'python', name: 'Python', tag: 'FastAPI / Django' },
              { id: 'docker', name: 'Docker', tag: 'Custom Container' }
            ].map((fw) => (
              <button
                type="button"
                key={fw.id}
                onClick={() => handleTypeChange(fw.id)}
                className={`p-3 rounded-lg border text-left transition cursor-pointer ${
                  appType === fw.id
                    ? 'bg-blue-600/10 border-blue-500 text-white shadow-sm'
                    : 'bg-slate-950/40 border-slate-800/80 text-slate-400 hover:border-slate-700'
                }`}
              >
                <div className="font-semibold text-sm">{fw.name}</div>
                <div className="text-[11px] text-slate-500 mt-0.5">{fw.tag}</div>
              </button>
            ))}
          </div>
        </div>

        {/* Step 2: Repository & Settings */}
        <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4">
          <h2 className="text-sm font-semibold text-slate-200 uppercase tracking-wider flex items-center gap-2">
            <GitBranch className="w-4 h-4 text-blue-400" />
            2. Application & Repository Details
          </h2>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Application Name</label>
              <input
                type="text"
                required
                placeholder="e.g. acme-web-portal"
                value={name}
                onChange={(e) => setName(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
              />
              <span className="text-[11px] text-slate-500 mt-1 block">
                Assigned URL:{' '}
                <span className="text-blue-400 font-mono">
                  {name ? `${name.toLowerCase().replace(/\s+/g, '-')}.apps.nevatal.id` : 'your-app.apps.nevatal.id'}
                </span>
              </span>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Git Repository URL</label>
              <input
                type="url"
                required
                placeholder="https://github.com/org/repo"
                value={repoUrl}
                onChange={(e) => setRepoUrl(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Production Branch</label>
              <input
                type="text"
                value={branch}
                onChange={(e) => setBranch(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Root Directory</label>
              <input
                type="text"
                value={rootDirectory}
                onChange={(e) => setRootDirectory(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
              />
            </div>
          </div>
        </div>

        {/* Step 3: Build & Execution Config */}
        <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4">
          <h2 className="text-sm font-semibold text-slate-200 uppercase tracking-wider flex items-center gap-2">
            <Terminal className="w-4 h-4 text-blue-400" />
            3. Build & Output Configuration
          </h2>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Build Command</label>
              <input
                type="text"
                value={buildCommand}
                onChange={(e) => setBuildCommand(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Output Directory</label>
              <input
                type="text"
                value={outputDirectory}
                onChange={(e) => setOutputDirectory(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Start Command</label>
              <input
                type="text"
                value={startCommand}
                onChange={(e) => setStartCommand(e.target.value)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Internal Port</label>
              <input
                type="number"
                value={appPort}
                onChange={(e) => setAppPort(parseInt(e.target.value) || 8080)}
                className="w-full px-3 py-2 bg-slate-950 border border-slate-800 rounded-lg text-sm text-white focus:outline-none focus:border-blue-500 font-mono"
              />
            </div>
          </div>
        </div>

        {/* Step 4: Environment Variables */}
        <div className="bg-slate-900/60 border border-slate-800/80 rounded-xl p-5 space-y-4">
          <div className="flex items-center justify-between">
            <h2 className="text-sm font-semibold text-slate-200 uppercase tracking-wider flex items-center gap-2">
              <Layers className="w-4 h-4 text-blue-400" />
              4. Environment Variables
            </h2>
            <button
              type="button"
              onClick={addEnvVar}
              className="text-xs text-blue-400 hover:text-blue-300 flex items-center gap-1 cursor-pointer"
            >
              <Plus className="w-3.5 h-3.5" /> Add Variable
            </button>
          </div>

          <div className="space-y-2">
            {envVars.map((ev, idx) => (
              <div key={idx} className="flex items-center gap-2">
                <input
                  type="text"
                  placeholder="KEY (e.g. DATABASE_URL)"
                  value={ev.key}
                  onChange={(e) => updateEnvVar(idx, 'key', e.target.value)}
                  className="flex-1 px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono uppercase"
                />
                <input
                  type="text"
                  placeholder="VALUE"
                  value={ev.value}
                  onChange={(e) => updateEnvVar(idx, 'value', e.target.value)}
                  className="flex-1 px-3 py-1.5 bg-slate-950 border border-slate-800 rounded-lg text-xs text-white font-mono"
                />
                <button
                  type="button"
                  onClick={() => removeEnvVar(idx)}
                  className="p-2 text-slate-500 hover:text-red-400 cursor-pointer"
                >
                  <Trash2 className="w-4 h-4" />
                </button>
              </div>
            ))}
          </div>
        </div>

        {/* Submit */}
        <div className="flex items-center justify-end gap-3 pt-2">
          <button
            type="button"
            onClick={() => onNavigate('/dashboard/apps')}
            className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-sm font-medium transition cursor-pointer"
          >
            Cancel
          </button>
          <button
            type="submit"
            disabled={loading}
            className="px-6 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-sm font-medium transition shadow-lg shadow-blue-500/20 flex items-center gap-2 cursor-pointer"
          >
            {loading ? (
              <>
                <div className="w-4 h-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                <span>Deploying...</span>
              </>
            ) : (
              <>
                <CheckCircle2 className="w-4 h-4" />
                <span>Deploy Application</span>
              </>
            )}
          </button>
        </div>
      </form>
    </div>
  );
};
