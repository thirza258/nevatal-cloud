import React, { useState, useEffect } from 'react';
import {
  Gamepad2,
  ArrowLeft,
  Check,
  ChevronRight,
  Shield,
  Zap,
  Server,
  Layers,
  Sparkles,
  AlertCircle,
  RefreshCw,
  HardDrive,
  Cpu,
  Users,
  Globe,
  Sliders,
} from 'lucide-react';
import { apiRequest } from '../../lib/api';
import {
  GameServerTemplate,
  GameServerPlan,
  GameType,
} from '../../types';

interface CreateGameServerProps {
  onNavigate: (path: string, param?: string) => void;
}

export const CreateGameServer: React.FC<CreateGameServerProps> = ({ onNavigate }) => {
  const [step, setStep] = useState<number>(1);
  const [templates, setTemplates] = useState<GameServerTemplate[]>([]);
  const [plans, setPlans] = useState<GameServerPlan[]>([]);
  const [loading, setLoading] = useState(true);
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Form State per PRD Section 50
  const [selectedGame, setSelectedGame] = useState<GameType>('minecraft_java');
  const [selectedSoftware, setSelectedSoftware] = useState<string>('paper');
  const [selectedVersion, setSelectedVersion] = useState<string>('1.21.4');
  const [serverName, setServerName] = useState<string>('my-game-server');
  const [motd, setMotd] = useState<string>('A Nevatal Cloud Game Server');
  const [selectedPlan, setSelectedPlan] = useState<string>('basic');
  const [customVCPUs, setCustomVCPUs] = useState<number>(2);
  const [customMemoryMB, setCustomMemoryMB] = useState<number>(4096);
  const [customStorageGB, setCustomStorageGB] = useState<number>(20);
  const [playerSlots, setPlayerSlots] = useState<number>(10);
  const [difficulty, setDifficulty] = useState<string>('normal');
  const [gamemode, setGamemode] = useState<string>('survival');
  const [worldSize, setWorldSize] = useState<string>('medium');
  const [pvpEnabled, setPvpEnabled] = useState<boolean>(true);
  const [whitelistEnabled, setWhitelistEnabled] = useState<boolean>(false);
  const [autoBackup, setAutoBackup] = useState<boolean>(true);
  const [backupSchedule, setBackupSchedule] = useState<string>('daily');
  const [customDomain, setCustomDomain] = useState<string>('');

  useEffect(() => {
    const fetchMetadata = async () => {
      try {
        setLoading(true);
        const [tpls, pls] = await Promise.all([
          apiRequest<GameServerTemplate[]>('/game-servers/templates'),
          apiRequest<GameServerPlan[]>('/game-servers/plans'),
        ]);
        setTemplates(tpls || []);
        setPlans(pls || []);
      } catch (err) {
        console.error('Failed to load creation templates:', err);
      } finally {
        setLoading(false);
      }
    };
    fetchMetadata();
  }, []);

  // Update default software when game changes
  const handleGameChange = (game: GameType) => {
    setSelectedGame(game);
    if (game === 'minecraft_java') {
      setSelectedSoftware('paper');
      setSelectedVersion('1.21.4');
      setServerName('minecraft-java-server');
    } else if (game === 'minecraft_bedrock') {
      setSelectedSoftware('official_bedrock');
      setSelectedVersion('1.21.50');
      setServerName('bedrock-realm-server');
    } else {
      setSelectedSoftware('official_terraria');
      setSelectedVersion('1.4.4.9');
      setServerName('terraria-world-server');
    }
  };

  const getAvailableSoftware = () => {
    if (selectedGame === 'minecraft_java') {
      return [
        { id: 'paper', name: 'Paper', desc: 'Recommended: High-performance plugin server (Bukkit/Spigot)' },
        { id: 'purpur', name: 'Purpur', desc: 'Extended Paper fork with extra customization knobs' },
        { id: 'fabric', name: 'Fabric', desc: 'Lightweight modern modding platform' },
        { id: 'forge', name: 'Forge', desc: 'Comprehensive modpacks & traditional modding ecosystem' },
        { id: 'neoforge', name: 'NeoForge', desc: 'Modern active fork of Minecraft Forge' },
        { id: 'vanilla', name: 'Vanilla', desc: 'Official Mojang dedicated server experience' },
      ];
    }
    if (selectedGame === 'minecraft_bedrock') {
      return [
        { id: 'official_bedrock', name: 'Official Bedrock Dedicated Server', desc: 'Direct support for Windows 10/11, iOS, Android, and consoles' },
      ];
    }
    return [
      { id: 'official_terraria', name: 'Official Terraria Dedicated Server', desc: 'Standard vanilla dedicated server' },
      { id: 'tmodloader', name: 'tModLoader Server', desc: 'Support for community mods and Steam Workshop content' },
    ];
  };

  const getAvailableVersions = () => {
    if (selectedGame === 'minecraft_java') {
      return ['1.21.4 (Latest)', '1.21.1', '1.20.4', '1.19.4', '1.18.2', '1.16.5'];
    }
    if (selectedGame === 'minecraft_bedrock') {
      return ['1.21.50 (Latest)', '1.21.20', '1.20.80'];
    }
    return ['1.4.4.9 (Latest)', '1.4.3.6'];
  };

  const handleSubmit = async () => {
    setError(null);
    setSubmitting(true);
    try {
      let vcpu = 2;
      let ram = 4096;
      let disk = 20;

      if (selectedPlan === 'custom') {
        vcpu = customVCPUs;
        ram = customMemoryMB;
        disk = customStorageGB;
      } else {
        const foundPlan = plans.find((p) => p.id === selectedPlan);
        if (foundPlan) {
          vcpu = foundPlan.vcpus;
          ram = foundPlan.memory_mb;
          disk = foundPlan.storage_gb;
        }
      }

      const payload = {
        name: serverName.trim() || 'game-server',
        game_type: selectedGame,
        server_software: selectedSoftware,
        game_version: selectedVersion.split(' ')[0],
        plan_name: selectedPlan,
        vcpus: vcpu,
        memory_mb: ram,
        storage_gb: disk,
        max_players: playerSlots,
        motd: motd,
        difficulty: difficulty,
        gamemode: gamemode,
        pvp: pvpEnabled,
        whitelist: whitelistEnabled,
        auto_backup_enabled: autoBackup,
        backup_schedule: backupSchedule,
        custom_domain: customDomain.trim() || undefined,
        world_size: worldSize,
      };

      const res = await apiRequest<{ id: string }>('/game-servers', {
        method: 'POST',
        body: JSON.stringify(payload),
      });

      onNavigate('/dashboard/game-servers', res.id);
    } catch (err: any) {
      setError(err.message || 'Failed to deploy game server');
      setSubmitting(false);
    }
  };

  if (loading) {
    return (
      <div className="py-20 flex flex-col items-center justify-center text-slate-400 gap-3">
        <RefreshCw className="w-8 h-8 animate-spin text-violet-500" />
        <span className="text-sm font-mono">Loading server templates & specs...</span>
      </div>
    );
  }

  return (
    <div className="max-w-4xl mx-auto space-y-8">
      {/* Back button & Title */}
      <div>
        <button
          onClick={() => onNavigate('/dashboard/game-servers')}
          className="inline-flex items-center gap-1.5 text-xs text-slate-400 hover:text-slate-200 transition mb-3"
        >
          <ArrowLeft className="w-3.5 h-3.5" />
          <span>Back to Game Servers</span>
        </button>
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          <div>
            <h1 className="text-2xl font-bold text-white tracking-tight flex items-center gap-2.5">
              <Gamepad2 className="w-6 h-6 text-violet-400" />
              <span>Create Game Server</span>
            </h1>
            <p className="text-sm text-slate-400 mt-1">
              Wizard-guided deployment on Dell PowerEdge R640 bare-metal with dedicated port allocation.
            </p>
          </div>

          <button
            onClick={() => onNavigate('/dashboard/ai')}
            className="flex items-center gap-2 px-3.5 py-1.5 rounded-xl bg-violet-600/10 hover:bg-violet-600/20 text-violet-400 border border-violet-500/20 text-xs font-medium transition"
          >
            <Sparkles className="w-3.5 h-3.5" />
            <span>Use AI Agent Instead</span>
          </button>
        </div>
      </div>

      {/* Stepper Progress */}
      <div className="grid grid-cols-4 gap-2 text-xs font-medium">
        <div
          onClick={() => setStep(1)}
          className={`cursor-pointer p-3 rounded-xl border transition ${
            step === 1
              ? 'bg-violet-600/10 border-violet-500/40 text-violet-300'
              : step > 1
              ? 'bg-slate-900 border-slate-800 text-emerald-400'
              : 'bg-slate-900/40 border-slate-800/60 text-slate-500'
          }`}
        >
          <div className="flex items-center justify-between">
            <span>1. Game & Engine</span>
            {step > 1 && <Check className="w-3.5 h-3.5" />}
          </div>
        </div>

        <div
          onClick={() => setStep(2)}
          className={`cursor-pointer p-3 rounded-xl border transition ${
            step === 2
              ? 'bg-violet-600/10 border-violet-500/40 text-violet-300'
              : step > 2
              ? 'bg-slate-900 border-slate-800 text-emerald-400'
              : 'bg-slate-900/40 border-slate-800/60 text-slate-500'
          }`}
        >
          <div className="flex items-center justify-between">
            <span>2. Hardware Plan</span>
            {step > 2 && <Check className="w-3.5 h-3.5" />}
          </div>
        </div>

        <div
          onClick={() => setStep(3)}
          className={`cursor-pointer p-3 rounded-xl border transition ${
            step === 3
              ? 'bg-violet-600/10 border-violet-500/40 text-violet-300'
              : step > 3
              ? 'bg-slate-900 border-slate-800 text-emerald-400'
              : 'bg-slate-900/40 border-slate-800/60 text-slate-500'
          }`}
        >
          <div className="flex items-center justify-between">
            <span>3. Gameplay Settings</span>
            {step > 3 && <Check className="w-3.5 h-3.5" />}
          </div>
        </div>

        <div
          onClick={() => setStep(4)}
          className={`cursor-pointer p-3 rounded-xl border transition ${
            step === 4
              ? 'bg-violet-600/10 border-violet-500/40 text-violet-300'
              : 'bg-slate-900/40 border-slate-800/60 text-slate-500'
          }`}
        >
          <div className="flex items-center justify-between">
            <span>4. Review & Deploy</span>
          </div>
        </div>
      </div>

      {error && (
        <div className="p-4 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-400 text-sm flex items-start gap-2.5">
          <AlertCircle className="w-5 h-5 shrink-0 mt-0.5" />
          <div>
            <strong>Deployment failed:</strong> {error}
          </div>
        </div>
      )}

      {/* Step 1: Game & Engine */}
      {step === 1 && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-6">
          <h2 className="text-base font-semibold text-white">Select Game Platform</h2>
          <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
            <div
              onClick={() => handleGameChange('minecraft_java')}
              className={`p-5 rounded-2xl border cursor-pointer transition flex flex-col justify-between ${
                selectedGame === 'minecraft_java'
                  ? 'bg-emerald-950/20 border-emerald-500/50 text-white shadow-lg shadow-emerald-950/50'
                  : 'bg-slate-900/40 border-slate-800 hover:border-slate-700 text-slate-300'
              }`}
            >
              <div>
                <div className="text-3xl mb-2">⛏️</div>
                <h3 className="font-semibold text-base">Minecraft Java Edition</h3>
                <p className="text-xs text-slate-400 mt-1">
                  Full PC support with Bukkit/Spigot plugins, Fabric mods, and Forge modpacks.
                </p>
              </div>
              <span className="mt-4 text-[11px] font-mono text-emerald-400 font-medium">Default Port: 25565 TCP</span>
            </div>

            <div
              onClick={() => handleGameChange('minecraft_bedrock')}
              className={`p-5 rounded-2xl border cursor-pointer transition flex flex-col justify-between ${
                selectedGame === 'minecraft_bedrock'
                  ? 'bg-teal-950/20 border-teal-500/50 text-white shadow-lg shadow-teal-950/50'
                  : 'bg-slate-900/40 border-slate-800 hover:border-slate-700 text-slate-300'
              }`}
            >
              <div>
                <div className="text-3xl mb-2">📱</div>
                <h3 className="font-semibold text-base">Minecraft Bedrock</h3>
                <p className="text-xs text-slate-400 mt-1">
                  Cross-platform multiplayer for iOS, Android, Windows 10/11, and Xbox/PlayStation consoles.
                </p>
              </div>
              <span className="mt-4 text-[11px] font-mono text-teal-400 font-medium">Default Port: 19132 UDP</span>
            </div>

            <div
              onClick={() => handleGameChange('terraria')}
              className={`p-5 rounded-2xl border cursor-pointer transition flex flex-col justify-between ${
                selectedGame === 'terraria'
                  ? 'bg-amber-950/20 border-amber-500/50 text-white shadow-lg shadow-amber-950/50'
                  : 'bg-slate-900/40 border-slate-800 hover:border-slate-700 text-slate-300'
              }`}
            >
              <div>
                <div className="text-3xl mb-2">🌳</div>
                <h3 className="font-semibold text-base">Terraria Dedicated</h3>
                <p className="text-xs text-slate-400 mt-1">
                  Official Re-Logic 2D action-adventure multiplayer world server with optional tModLoader.
                </p>
              </div>
              <span className="mt-4 text-[11px] font-mono text-amber-400 font-medium">Default Port: 7777 TCP</span>
            </div>
          </div>

          <div className="pt-4 border-t border-slate-800 space-y-4">
            <h3 className="text-sm font-semibold text-white">Select Server Software / Implementation</h3>
            <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
              {getAvailableSoftware().map((sw) => (
                <div
                  key={sw.id}
                  onClick={() => setSelectedSoftware(sw.id)}
                  className={`p-3.5 rounded-xl border cursor-pointer transition ${
                    selectedSoftware === sw.id
                      ? 'bg-violet-600/10 border-violet-500/50 text-white'
                      : 'bg-slate-900/40 border-slate-800 hover:border-slate-700 text-slate-300'
                  }`}
                >
                  <div className="flex items-center justify-between">
                    <span className="font-medium text-sm">{sw.name}</span>
                    {selectedSoftware === sw.id && <Check className="w-4 h-4 text-violet-400" />}
                  </div>
                  <p className="text-xs text-slate-400 mt-1">{sw.desc}</p>
                </div>
              ))}
            </div>
          </div>

          <div className="pt-4 border-t border-slate-800 grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Game Version</label>
              <select
                value={selectedVersion}
                onChange={(e) => setSelectedVersion(e.target.value)}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-200 focus:outline-none focus:border-violet-500 font-mono"
              >
                {getAvailableVersions().map((ver) => (
                  <option key={ver} value={ver}>
                    {ver}
                  </option>
                ))}
              </select>
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Server Display Name</label>
              <input
                type="text"
                value={serverName}
                onChange={(e) => setServerName(e.target.value)}
                placeholder="e.g. my-friends-server"
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-200 focus:outline-none focus:border-violet-500"
              />
            </div>
          </div>

          <div className="flex justify-end pt-4">
            <button
              onClick={() => setStep(2)}
              className="flex items-center gap-1.5 px-5 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-sm font-semibold transition"
            >
              <span>Continue to Hardware Plan</span>
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* Step 2: Hardware Plan Selection per PRD Section 50.2 */}
      {step === 2 && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-6">
          <div className="flex items-center justify-between">
            <h2 className="text-base font-semibold text-white">Choose Resource Plan</h2>
            <span className="text-xs text-slate-400">Allocated directly on Dell PowerEdge R640 local-zfs NVMe</span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
            {plans.map((p) => (
              <div
                key={p.id}
                onClick={() => setSelectedPlan(p.id)}
                className={`p-4 rounded-2xl border cursor-pointer transition flex flex-col justify-between ${
                  selectedPlan === p.id
                    ? 'bg-violet-600/10 border-violet-500 text-white shadow-lg shadow-violet-950/50'
                    : 'bg-slate-900/40 border-slate-800 hover:border-slate-700 text-slate-300'
                }`}
              >
                <div>
                  <div className="flex items-center justify-between mb-2">
                    <span className="font-semibold text-base capitalize">{p.name}</span>
                    {selectedPlan === p.id && <Check className="w-4 h-4 text-violet-400" />}
                  </div>
                  <div className="space-y-1 text-xs font-mono text-slate-300 my-3">
                    <div className="flex items-center justify-between">
                      <span className="text-slate-500">CPU</span>
                      <span>{p.vcpus} vCPU</span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-slate-500">Memory</span>
                      <span>{p.memory_mb / 1024} GB</span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-slate-500">Storage</span>
                      <span>{p.storage_gb} GB</span>
                    </div>
                    <div className="flex items-center justify-between">
                      <span className="text-slate-500">Slots</span>
                      <span>Up to {p.max_players}</span>
                    </div>
                  </div>
                </div>

                <div className="pt-3 border-t border-slate-800 text-[11px] text-slate-400">
                  {p.suggested_use}
                </div>
              </div>
            ))}
          </div>

          {/* Custom plan option */}
          <div
            onClick={() => setSelectedPlan('custom')}
            className={`p-4 rounded-2xl border cursor-pointer transition ${
              selectedPlan === 'custom'
                ? 'bg-violet-600/10 border-violet-500 text-white'
                : 'bg-slate-900/40 border-slate-800 hover:border-slate-700 text-slate-300'
            }`}
          >
            <div className="flex items-center justify-between mb-2">
              <span className="font-semibold text-sm flex items-center gap-2">
                <Sliders className="w-4 h-4 text-violet-400" />
                <span>Custom Resource Plan</span>
              </span>
              {selectedPlan === 'custom' && <Check className="w-4 h-4 text-violet-400" />}
            </div>
            <p className="text-xs text-slate-400">
              Customize exact vCPU cores, memory, and NVMe disk quotas for heavy modpacks.
            </p>

            {selectedPlan === 'custom' && (
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mt-4 pt-4 border-t border-slate-800">
                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1">
                    vCPUs: {customVCPUs}
                  </label>
                  <input
                    type="range"
                    min="1"
                    max="8"
                    step="1"
                    value={customVCPUs}
                    onChange={(e) => setCustomVCPUs(Number(e.target.value))}
                    className="w-full accent-violet-500"
                  />
                </div>
                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1">
                    Memory: {customMemoryMB / 1024} GB ({customMemoryMB} MB)
                  </label>
                  <input
                    type="range"
                    min="1024"
                    max="16384"
                    step="1024"
                    value={customMemoryMB}
                    onChange={(e) => setCustomMemoryMB(Number(e.target.value))}
                    className="w-full accent-violet-500"
                  />
                </div>
                <div>
                  <label className="block text-xs font-mono text-slate-300 mb-1">
                    NVMe Disk: {customStorageGB} GB
                  </label>
                  <input
                    type="range"
                    min="10"
                    max="120"
                    step="5"
                    value={customStorageGB}
                    onChange={(e) => setCustomStorageGB(Number(e.target.value))}
                    className="w-full accent-violet-500"
                  />
                </div>
              </div>
            )}
          </div>

          <div className="flex items-center justify-between pt-4">
            <button
              onClick={() => setStep(1)}
              className="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
            >
              Back
            </button>
            <button
              onClick={() => setStep(3)}
              className="flex items-center gap-1.5 px-5 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-sm font-semibold transition"
            >
              <span>Continue to Gameplay Settings</span>
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* Step 3: Gameplay Settings per PRD Section 51.2 & 52.2 */}
      {step === 3 && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-6">
          <h2 className="text-base font-semibold text-white">Configure Server Rules & Networking</h2>

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Player Slot Capacity</label>
              <input
                type="number"
                min="2"
                max="100"
                value={playerSlots}
                onChange={(e) => setPlayerSlots(Number(e.target.value))}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-200 focus:outline-none focus:border-violet-500 font-mono"
              />
            </div>

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">World Difficulty</label>
              <select
                value={difficulty}
                onChange={(e) => setDifficulty(e.target.value)}
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-200 focus:outline-none focus:border-violet-500 capitalize"
              >
                {selectedGame === 'terraria' ? (
                  <>
                    <option value="classic">Classic (Default)</option>
                    <option value="expert">Expert (Enhanced Bosses)</option>
                    <option value="master">Master (Maximum Challenge)</option>
                  </>
                ) : (
                  <>
                    <option value="peaceful">Peaceful</option>
                    <option value="easy">Easy</option>
                    <option value="normal">Normal</option>
                    <option value="hard">Hard</option>
                  </>
                )}
              </select>
            </div>

            {selectedGame.startsWith('minecraft') && (
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">Game Mode</label>
                <select
                  value={gamemode}
                  onChange={(e) => setGamemode(e.target.value)}
                  className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-200 focus:outline-none focus:border-violet-500 capitalize"
                >
                  <option value="survival">Survival</option>
                  <option value="creative">Creative</option>
                  <option value="adventure">Adventure</option>
                </select>
              </div>
            )}

            {selectedGame === 'terraria' && (
              <div>
                <label className="block text-xs font-medium text-slate-300 mb-1.5">Initial World Size</label>
                <select
                  value={worldSize}
                  onChange={(e) => setWorldSize(e.target.value)}
                  className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-200 focus:outline-none focus:border-violet-500 capitalize"
                >
                  <option value="small">Small (Fast generation)</option>
                  <option value="medium">Medium (Recommended)</option>
                  <option value="large">Large (Massive world)</option>
                </select>
              </div>
            )}

            <div>
              <label className="block text-xs font-medium text-slate-300 mb-1.5">Server MOTD (Message of the Day)</label>
              <input
                type="text"
                value={motd}
                onChange={(e) => setMotd(e.target.value)}
                placeholder="Welcome to our server!"
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-200 focus:outline-none focus:border-violet-500"
              />
            </div>
          </div>

          <div className="pt-4 border-t border-slate-800 grid grid-cols-1 sm:grid-cols-2 gap-4">
            <label className="flex items-center gap-3 p-3.5 rounded-xl bg-slate-900/40 border border-slate-800 cursor-pointer">
              <input
                type="checkbox"
                checked={pvpEnabled}
                onChange={(e) => setPvpEnabled(e.target.checked)}
                className="rounded accent-violet-500 w-4 h-4"
              />
              <div>
                <span className="text-sm font-medium text-slate-200">Enable PvP Combat</span>
                <p className="text-xs text-slate-500">Allow players to damage each other</p>
              </div>
            </label>

            <label className="flex items-center gap-3 p-3.5 rounded-xl bg-slate-900/40 border border-slate-800 cursor-pointer">
              <input
                type="checkbox"
                checked={whitelistEnabled}
                onChange={(e) => setWhitelistEnabled(e.target.checked)}
                className="rounded accent-violet-500 w-4 h-4"
              />
              <div>
                <span className="text-sm font-medium text-slate-200">Enforce Whitelist</span>
                <p className="text-xs text-slate-500">Only authorized players can join</p>
              </div>
            </label>
          </div>

          {/* Networking & Custom Domain preview per PRD Section 54 */}
          <div className="pt-4 border-t border-slate-800 space-y-3">
            <h3 className="text-sm font-semibold text-white flex items-center gap-2">
              <Globe className="w-4 h-4 text-violet-400" />
              <span>Networking & Friendly Address (Optional)</span>
            </h3>
            <p className="text-xs text-slate-400">
              The platform assigns a dedicated public port automatically. You can also configure a custom hostname via DNS SRV.
            </p>
            <div>
              <input
                type="text"
                value={customDomain}
                onChange={(e) => setCustomDomain(e.target.value)}
                placeholder="e.g. play.mycommunity.com"
                className="w-full bg-slate-900 border border-slate-800 rounded-xl px-3.5 py-2.5 text-sm text-slate-200 focus:outline-none focus:border-violet-500 font-mono"
              />
            </div>
          </div>

          <div className="flex items-center justify-between pt-4">
            <button
              onClick={() => setStep(2)}
              className="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
            >
              Back
            </button>
            <button
              onClick={() => setStep(4)}
              className="flex items-center gap-1.5 px-5 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-sm font-semibold transition"
            >
              <span>Review Deployment</span>
              <ChevronRight className="w-4 h-4" />
            </button>
          </div>
        </div>
      )}

      {/* Step 4: Review & Deploy */}
      {step === 4 && (
        <div className="bg-slate-900/60 border border-slate-800 rounded-2xl p-6 space-y-6">
          <h2 className="text-base font-semibold text-white">Review Configuration & Launch</h2>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-3">
              <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Server Profile</span>
              <div className="space-y-2 text-xs">
                <div className="flex justify-between">
                  <span className="text-slate-500">Server Name</span>
                  <span className="text-slate-200 font-medium">{serverName}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Game Platform</span>
                  <span className="text-slate-200 font-medium capitalize">{selectedGame.replace('_', ' ')}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Software Engine</span>
                  <span className="text-slate-200 font-medium capitalize">{selectedSoftware}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Game Version</span>
                  <span className="text-slate-200 font-mono">{selectedVersion}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Player Slots</span>
                  <span className="text-slate-200 font-medium">{playerSlots} concurrent players</span>
                </div>
              </div>
            </div>

            <div className="p-4 rounded-xl bg-slate-900 border border-slate-800 space-y-3">
              <span className="text-xs font-semibold text-slate-400 uppercase tracking-wider">Hardware Allocation</span>
              <div className="space-y-2 text-xs">
                <div className="flex justify-between">
                  <span className="text-slate-500">Selected Plan</span>
                  <span className="text-slate-200 font-medium capitalize">{selectedPlan}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">vCPU Allocation</span>
                  <span className="text-slate-200 font-mono">
                    {selectedPlan === 'custom'
                      ? `${customVCPUs} vCPUs`
                      : `${plans.find((p) => p.id === selectedPlan)?.vcpus || 2} vCPUs`}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Dedicated RAM</span>
                  <span className="text-slate-200 font-mono">
                    {selectedPlan === 'custom'
                      ? `${customMemoryMB / 1024} GB`
                      : `${(plans.find((p) => p.id === selectedPlan)?.memory_mb || 4096) / 1024} GB`}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">NVMe Disk</span>
                  <span className="text-slate-200 font-mono">
                    {selectedPlan === 'custom'
                      ? `${customStorageGB} GB`
                      : `${plans.find((p) => p.id === selectedPlan)?.storage_gb || 20} GB`}
                  </span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-500">Hypervisor Node</span>
                  <span className="text-violet-400 font-mono">pve-dell-r640</span>
                </div>
              </div>
            </div>
          </div>

          <div className="p-4 rounded-xl bg-violet-600/10 border border-violet-500/20 text-xs text-violet-300 flex items-center gap-3">
            <Zap className="w-5 h-5 text-violet-400 shrink-0" />
            <span>
              Your server will be provisioned instantly with a dedicated public port, automated restart guards, and persistent world storage.
            </span>
          </div>

          <div className="flex items-center justify-between pt-4">
            <button
              onClick={() => setStep(3)}
              className="px-4 py-2 rounded-xl text-xs font-medium text-slate-400 hover:text-white transition"
            >
              Back
            </button>
            <button
              disabled={submitting}
              onClick={handleSubmit}
              className="flex items-center gap-2 px-6 py-2.5 rounded-xl bg-violet-600 hover:bg-violet-500 text-white text-sm font-semibold transition shadow-lg shadow-violet-600/25 disabled:opacity-50"
            >
              {submitting ? (
                <>
                  <RefreshCw className="w-4 h-4 animate-spin" />
                  <span>Provisioning Server...</span>
                </>
              ) : (
                <>
                  <Gamepad2 className="w-4 h-4" />
                  <span>Confirm & Deploy Server</span>
                </>
              )}
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
