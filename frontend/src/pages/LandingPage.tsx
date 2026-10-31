import React, { useState } from 'react';
import { useAuth } from '../context/AuthContext';
import {
  Server,
  Zap,
  Gamepad2,
  Smartphone,
  Cpu,
  Layers,
  Terminal,
  Shield,
  Workflow,
  Database,
  Sparkles,
  ArrowRight,
  Check,
  Copy,
  ExternalLink,
  ChevronDown,
  BookOpen,
  Code,
  Activity,
  HardDrive,
  Globe,
  PlusCircle,
  Radio,
} from 'lucide-react';

interface LandingPageProps {
  onNavigate: (path: string, param?: string) => void;
}

export const LandingPage: React.FC<LandingPageProps> = ({ onNavigate }) => {
  const { user } = useAuth();
  const [activeTab, setActiveTab] = useState<'compute' | 'game' | 'android' | 'ai'>('compute');
  const [copiedCurl, setCopiedCurl] = useState(false);
  const [openFaq, setOpenFaq] = useState<number | null>(null);
  const [wireframeInvert, setWireframeInvert] = useState(true);

  const copyCurl = () => {
    navigator.clipboard.writeText('curl -s https://cloud.nevatal.id/api/v1/health');
    setCopiedCurl(true);
    setTimeout(() => setCopiedCurl(false), 2000);
  };

  const toggleFaq = (idx: number) => {
    setOpenFaq(openFaq === idx ? null : idx);
  };

  const faqs = [
    {
      q: 'Is Nevatal Cloud free to use?',
      a: 'Yes! Nevatal Cloud is currently 100% free for all registered developers during our public community phase. You receive generous compute, memory, storage quotas, game servers, Waydroid Android containers, and S3 buckets without needing a credit card or billing profile.',
    },
    {
      q: 'How does Nevatal Cloud compare to raw Proxmox VE?',
      a: 'Proxmox VE is an enterprise hypervisor built for system administrators. Nevatal Cloud connects directly to Proxmox VE via REST API v2, wrapping it in a developer-friendly web control plane. It automates cloud-init OS provisioning, user quota governance, multi-tenant network isolation, S3 storage, game servers, and Waydroid Android containers without needing manual Linux CLI interventions.',
    },
    {
      q: 'Can I upload custom ISOs and operating system images?',
      a: 'Yes. Administrators can upload custom ISO images directly through the Administrator ISO Catalog (/admin/isos). Standard users can provision pre-baked, cloud-init automated images including Ubuntu 24.04 LTS, Debian 12, Alpine Linux 3.20, and WordPress 6.7 LEMP appliances.',
    },
    {
      q: 'How are Minecraft and Terraria game servers kept persistent?',
      a: 'Game servers run as dedicated, isolated containerized daemons with persistent world data mounted directly to high-speed NVMe ZFS storage pools. Automated snapshot routines preserve worlds before software upgrades, and you can download or restore world archives in 1-click.',
    },
    {
      q: 'What is Waydroid and how does browser streaming work?',
      a: 'Waydroid runs an unmodified Android OS (Android 11 or 13) inside containerized Linux namespaces (LXC) on the host server. Instead of slow CPU emulation, it shares the physical Xeon CPU directly. The display is encoded with hardware acceleration and streamed to your web browser with touch, mouse, and keyboard synchronization. Network ADB is also available on port 5555 for automated testing and APK installation.',
    },
    {
      q: 'Is there an API and CLI for automated deployments?',
      a: 'Yes. Every action in the web control plane is backed by the versioned REST API (/api/v1). You can authenticate using session JWT tokens to spin up instances, trigger reboots, monitor telemetry, and automate workflows in GitHub Actions or CI/CD pipelines. We also provide native Model Context Protocol (MCP) server integration for Claude Desktop and Cursor.',
    },
  ];

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 flex flex-col font-sans selection:bg-zinc-800 selection:text-white">
      {/* Top Navigation */}
      <header className="sticky top-0 z-50 bg-zinc-950/85 backdrop-blur-md border-b border-zinc-800/80 px-4 sm:px-8 h-14 flex items-center justify-between">
        <div className="flex items-center gap-6">
          {/* Brand Logo */}
          <div
            onClick={() => onNavigate('/')}
            className="flex items-center gap-2.5 cursor-pointer select-none"
          >
            <div className="w-6 h-6 rounded bg-zinc-100 flex items-center justify-center text-zinc-950 font-bold text-xs tracking-tighter">
              NV
            </div>
            <div className="flex items-center gap-1.5">
              <span className="font-semibold text-sm tracking-tight text-white">Nevatal</span>
              <span className="text-[10px] font-mono text-zinc-400 px-1 py-0.2 rounded bg-zinc-900 border border-zinc-800">
                cloud
              </span>
            </div>
          </div>

          {/* Desktop Nav Links */}
          <nav className="hidden md:flex items-center gap-5 text-xs text-zinc-400">
            <a href="#compute" className="hover:text-zinc-100 transition-colors">Compute</a>
            <a href="#games" className="hover:text-zinc-100 transition-colors">Game Servers</a>
            <a href="#android" className="hover:text-zinc-100 transition-colors">Android Cloud</a>
            <button
              onClick={() => onNavigate('/docs')}
              className="text-zinc-300 hover:text-white flex items-center gap-1 transition-colors"
            >
              <BookOpen className="w-3 h-3" />
              <span>Docs</span>
            </button>
          </nav>
        </div>

        {/* Right CTA / Auth Status */}
        <div className="flex items-center gap-3">
          <div className="hidden lg:flex items-center gap-2 px-2.5 py-1 rounded-md bg-emerald-950/40 border border-emerald-800/60 text-[11px] font-mono text-emerald-400">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
            <span>100% Free · Dell R640 Cluster</span>
          </div>

          {user ? (
            <button
              onClick={() => onNavigate('/dashboard')}
              className="flex items-center gap-1.5 text-xs font-medium px-3.5 py-1.5 rounded-md bg-white text-zinc-950 hover:bg-zinc-200 transition-colors"
            >
              <Layers className="w-3.5 h-3.5" />
              <span>Console</span>
            </button>
          ) : (
            <div className="flex items-center gap-2">
              <button
                onClick={() => onNavigate('/login')}
                className="text-xs text-zinc-400 hover:text-white px-2.5 py-1.5 transition-colors"
              >
                Sign In
              </button>
              <button
                onClick={() => onNavigate('/register')}
                className="text-xs font-medium px-3.5 py-1.5 rounded-md bg-white text-zinc-950 hover:bg-zinc-200 transition-colors"
              >
                Get Started
              </button>
            </div>
          )}
        </div>
      </header>

      {/* Hero Section */}
      <section className="relative pt-16 sm:pt-24 pb-16 px-4 sm:px-8 border-b border-zinc-850 overflow-hidden">
        {/* Subtle Ambient Blueprint Wireframe Watermark in Background */}
        <div
          aria-hidden="true"
          className="absolute inset-0 pointer-events-none flex items-center justify-center opacity-[0.035] select-none overflow-hidden"
        >
          <img
            src="/dell-r640-blueprint.png"
            alt=""
            className="w-[140%] max-w-none filter invert hue-rotate-180 brightness-150 transform -rotate-1 scale-110"
          />
        </div>

        <div className="max-w-5xl mx-auto text-center space-y-6 relative z-10">
          {/* Hardware & Engine Eyebrow */}
          <div className="inline-flex items-center gap-2 px-3.5 py-1 rounded-full bg-emerald-950/40 border border-emerald-800/60 text-[11px] font-mono text-emerald-300">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
            <span className="font-semibold text-emerald-300">100% FREE FOR NOW</span>
            <span className="text-emerald-700">·</span>
            <span>NO CREDIT CARD REQUIRED</span>
            <span className="text-emerald-700">·</span>
            <span>PROXMOX VE 8 · DELL R640</span>
          </div>

          {/* Main Headline */}
          <h1 className="text-3xl sm:text-5xl lg:text-6xl font-bold tracking-tight text-white max-w-4xl mx-auto leading-[1.12]">
            Cloud compute, game servers, and Android environments.
            <span className="block text-zinc-400 font-normal">Bare-metal fast. 100% free.</span>
          </h1>

          {/* Subtitle */}
          <p className="text-sm sm:text-base text-zinc-400 max-w-2xl mx-auto leading-relaxed">
            Nevatal Cloud abstracts enterprise virtualization into a minimalist developer control plane.
            Deploy Linux VPS instances, one-click Minecraft &amp; Terraria worlds, Waydroid Android containers, and autonomous AI agents in under 30 seconds—completely free.
          </p>

          {/* CTAs */}
          <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
            <button
              onClick={() => onNavigate(user ? '/dashboard/instances/new' : '/register')}
              className="flex items-center gap-2 text-xs sm:text-sm font-medium px-5 py-2.5 rounded-md bg-white text-zinc-950 hover:bg-zinc-200 transition-colors shadow-sm"
            >
              <span>Deploy a Server Now — Free</span>
              <ArrowRight className="w-4 h-4" />
            </button>
            <button
              onClick={() => onNavigate('/docs')}
              className="flex items-center gap-2 text-xs sm:text-sm font-medium px-4 py-2.5 rounded-md bg-zinc-900 border border-zinc-800 hover:border-zinc-700 text-zinc-300 hover:text-white transition-colors"
            >
              <BookOpen className="w-4 h-4" />
              <span>Explore Documentation</span>
            </button>
          </div>

          <div className="text-xs text-zinc-400 font-mono">
            All workloads are currently <span className="text-emerald-400 font-semibold">100% free</span> for developers. Zero billing or credit card setup.
          </div>

          {/* Curl Command Pill */}
          <div className="pt-2 flex items-center justify-center">
            <div className="inline-flex items-center gap-2 px-3 py-1.5 rounded-md bg-zinc-900/80 border border-zinc-800 font-mono text-[11px] text-zinc-400">
              <span className="text-zinc-400">$</span>
              <span className="text-zinc-200">curl -s https://cloud.nevatal.id/api/v1/health</span>
              <button
                onClick={copyCurl}
                className="ml-1 text-zinc-400 hover:text-white transition-colors"
                title="Copy to clipboard"
              >
                {copiedCurl ? (
                  <Check className="w-3.5 h-3.5 text-emerald-400" />
                ) : (
                  <Copy className="w-3.5 h-3.5" />
                )}
              </button>
            </div>
          </div>
        </div>

        {/* Dell PowerEdge R640 Hardware Blueprint Showcase */}
        <div className="max-w-5xl mx-auto mt-12 rounded-xl border border-zinc-800 bg-zinc-950/90 overflow-hidden shadow-2xl relative z-10">
          {/* Blueprint Chassis Header Bar */}
          <div className="flex flex-wrap items-center justify-between px-4 py-2.5 bg-zinc-900 border-b border-zinc-800 gap-2">
            <div className="flex items-center gap-2">
              <span className="w-2 h-2 rounded-full bg-emerald-500 animate-pulse" />
              <span className="text-xs font-semibold text-white font-mono tracking-tight">
                DELL POWEREDGE R640
              </span>
              <span className="text-zinc-600 font-mono">·</span>
              <span className="text-[11px] font-mono text-zinc-400">
                1U ULTRA-DENSE BARE-METAL HYPERVISOR WIREFRAME
              </span>
            </div>

            <div className="flex items-center gap-2">
              <button
                onClick={() => setWireframeInvert(!wireframeInvert)}
                className="text-[10px] font-mono px-2 py-0.5 rounded bg-zinc-800 hover:bg-zinc-700 text-zinc-300 border border-zinc-700 transition-colors"
                title="Toggle schematic color mode"
              >
                {wireframeInvert ? 'Mode: Dark Wireframe' : 'Mode: Original CAD'}
              </button>
              <div className="hidden sm:flex items-center gap-1.5 text-[10px] font-mono text-zinc-400 pl-2 border-l border-zinc-800">
                <span className="px-1.5 py-0.5 rounded bg-zinc-950 border border-zinc-800 text-zinc-300">2x Xeon Gold</span>
                <span className="px-1.5 py-0.5 rounded bg-zinc-950 border border-zinc-800 text-zinc-300">10x NVMe U.2</span>
                <span className="px-1.5 py-0.5 rounded bg-zinc-950 border border-zinc-800 text-zinc-300">Proxmox VE 8</span>
              </div>
            </div>
          </div>

          {/* Blueprint Image Visual Stage */}
          <div className={`relative px-4 sm:px-8 py-6 sm:py-8 flex flex-col items-center justify-center transition-colors duration-300 ${
            wireframeInvert
              ? 'bg-zinc-950 bg-[radial-gradient(#27272a_1px,transparent_1px)] [background-size:20px_20px]'
              : 'bg-zinc-900/60 bg-[radial-gradient(#3f3f46_1px,transparent_1px)] [background-size:20px_20px]'
          }`}>
            <div className="w-full relative group">
              <img
                src="/dell-r640-blueprint.png"
                alt="Dell PowerEdge R640 Bare-Metal Server Blueprint Wireframe"
                className={`w-full h-auto max-h-[320px] object-contain mx-auto select-none transition-all duration-300 ${
                  wireframeInvert
                    ? 'filter invert hue-rotate-180 brightness-110 contrast-125 drop-shadow-[0_0_20px_rgba(255,255,255,0.08)]'
                    : 'filter drop-shadow-[0_0_20px_rgba(0,0,0,0.5)]'
                }`}
              />
            </div>

            {/* Chassis Technical Callout Grid */}
            <div className="w-full grid grid-cols-2 md:grid-cols-4 gap-2.5 mt-6 pt-5 border-t border-zinc-850/80 font-mono text-[11px]">
              <div className="p-2.5 rounded-lg bg-zinc-900/50 border border-zinc-800/80">
                <div className="flex items-center gap-1.5 text-zinc-400 text-[10px] uppercase tracking-wider mb-0.5">
                  <Server className="w-3 h-3 text-zinc-400" />
                  <span>Chassis Form</span>
                </div>
                <div className="text-xs font-semibold text-white font-mono">1U Ultra-Dense Rack</div>
                <div className="text-[10px] text-zinc-400 mt-0.5">Dual Hot-Swap PSUs</div>
              </div>

              <div className="p-2.5 rounded-lg bg-zinc-900/50 border border-zinc-800/80">
                <div className="flex items-center gap-1.5 text-zinc-400 text-[10px] uppercase tracking-wider mb-0.5">
                  <Cpu className="w-3 h-3 text-zinc-400" />
                  <span>Compute Core</span>
                </div>
                <div className="text-xs font-semibold text-white font-mono">Dual Intel Xeon Scalable</div>
                <div className="text-[10px] text-zinc-400 mt-0.5">Up to 56 Cores / 112 Threads</div>
              </div>

              <div className="p-2.5 rounded-lg bg-zinc-900/50 border border-zinc-800/80">
                <div className="flex items-center gap-1.5 text-zinc-400 text-[10px] uppercase tracking-wider mb-0.5">
                  <Layers className="w-3 h-3 text-zinc-400" />
                  <span>Memory Subsystem</span>
                </div>
                <div className="text-xs font-semibold text-white font-mono">24x DDR4 ECC RDIMM</div>
                <div className="text-[10px] text-zinc-400 mt-0.5">High-Speed Mirrored Channels</div>
              </div>

              <div className="p-2.5 rounded-lg bg-zinc-900/50 border border-zinc-800/80">
                <div className="flex items-center gap-1.5 text-zinc-400 text-[10px] uppercase tracking-wider mb-0.5">
                  <HardDrive className="w-3 h-3 text-zinc-400" />
                  <span>Storage Pool</span>
                </div>
                <div className="text-xs font-semibold text-white font-mono">10x 2.5" NVMe PCIe Gen3</div>
                <div className="text-[10px] text-zinc-400 mt-0.5">High-IOPS Enterprise ZFS</div>
              </div>
            </div>
          </div>

          {/* Connected Virtual Control Plane Window Bar */}
          <div className="flex flex-wrap items-center justify-between px-4 py-2 bg-zinc-900 border-t border-b border-zinc-800 gap-2">
            <div className="flex items-center gap-2">
              <div className="flex items-center gap-1.5">
                <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
                <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
                <div className="w-2.5 h-2.5 rounded-full bg-zinc-700" />
              </div>
              <span className="text-[11px] font-mono text-zinc-400 ml-1.5">
                nevatal-cloud-control-plane · live workload telemetry
              </span>
            </div>

            {/* Segmented Tab Controls */}
            <div className="flex items-center gap-1 bg-zinc-950 p-0.5 rounded border border-zinc-800 text-[11px] font-mono">
              <button
                onClick={() => setActiveTab('compute')}
                className={`px-2 py-1 rounded transition-colors ${
                  activeTab === 'compute'
                    ? 'bg-zinc-800 text-white font-medium'
                    : 'text-zinc-400 hover:text-zinc-200'
                }`}
              >
                Linux VPS
              </button>
              <button
                onClick={() => setActiveTab('game')}
                className={`px-2 py-1 rounded transition-colors ${
                  activeTab === 'game'
                    ? 'bg-zinc-800 text-white font-medium'
                    : 'text-zinc-400 hover:text-zinc-200'
                }`}
              >
                Game Server
              </button>
              <button
                onClick={() => setActiveTab('android')}
                className={`px-2 py-1 rounded transition-colors ${
                  activeTab === 'android'
                    ? 'bg-zinc-800 text-white font-medium'
                    : 'text-zinc-400 hover:text-zinc-200'
                }`}
              >
                Android Cloud
              </button>
              <button
                onClick={() => setActiveTab('ai')}
                className={`px-2 py-1 rounded transition-colors ${
                  activeTab === 'ai'
                    ? 'bg-zinc-800 text-white font-medium'
                    : 'text-zinc-400 hover:text-zinc-200'
                }`}
              >
                AI Agent &amp; MCP
              </button>
            </div>
          </div>

          {/* Tab Content Display */}
          <div className="p-5 font-mono text-xs text-zinc-300 min-h-[220px]">
            {activeTab === 'compute' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-[11px] text-zinc-400 border-b border-zinc-900 pb-2">
                  <span className="text-emerald-400">● RUNNING (VMID #104)</span>
                  <span>IP: 192.168.1.142 · Node: pve-dell-r640 · QEMU Guest Agent Active</span>
                </div>
                <div className="space-y-1 text-zinc-400">
                  <p className="text-zinc-300">$ ssh ubuntu@192.168.1.142</p>
                  <p className="text-zinc-400">Welcome to Ubuntu 24.04 LTS (GNU/Linux 6.8.0-31-generic x86_64)</p>
                  <p className="text-zinc-400">* System information: 2 vCPU · 4.0 GB RAM · 20.0 GB NVMe ZFS</p>
                  <p className="text-zinc-400">* Cloud-init bootstrap: FINISHED in 24.8s (SSH keys authorized)</p>
                  <p className="text-emerald-400">* WordPress LEMP Stack: Nginx 1.26 + PHP 8.3-FPM + MariaDB running</p>
                </div>
                <div className="pt-2 flex items-center gap-2">
                  <span className="px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-[10px] text-zinc-300">
                    HTML5 noVNC Console Ready
                  </span>
                  <span className="px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-[10px] text-zinc-300">
                    ACPI Graceful Power Management
                  </span>
                </div>
              </div>
            )}

            {activeTab === 'game' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-[11px] text-zinc-400 border-b border-zinc-900 pb-2">
                  <span className="text-emerald-400">● MINECRAFT PAPER 1.21 · ONLINE</span>
                  <span>Port: 25565 · Players: 14/20 · Latency: 11ms</span>
                </div>
                <div className="space-y-1 text-zinc-400">
                  <p className="text-zinc-400">[10:42:01 INFO]: Preparing start region for dimension minecraft:overworld</p>
                  <p className="text-zinc-400">[10:42:03 INFO]: Time elapsed: 1824 ms</p>
                  <p className="text-emerald-400">[10:42:04 INFO]: Done (4.212s)! For help, type "help"</p>
                  <p className="text-zinc-300">&gt; say Server backup completed. NVMe snapshot stored to S3.</p>
                  <p className="text-zinc-400">[10:42:15 INFO]: [Server] Server backup completed. NVMe snapshot stored to S3.</p>
                </div>
                <div className="pt-2 flex items-center gap-2">
                  <span className="px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-[10px] text-zinc-300">
                    Auto-Snapshot: Enabled
                  </span>
                  <span className="px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-[10px] text-zinc-300">
                    RAM: 3.1 GB / 4.0 GB
                  </span>
                </div>
              </div>
            )}

            {activeTab === 'android' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-[11px] text-zinc-400 border-b border-zinc-900 pb-2">
                  <span className="text-emerald-400">● WAYDROID LXC CONTAINER · ACTIVE</span>
                  <span>Android 13 (Tiramisu) · Resolution: 720x1280 (240 DPI)</span>
                </div>
                <div className="space-y-1 text-zinc-400">
                  <p className="text-zinc-300">$ adb connect 192.168.1.142:5555</p>
                  <p className="text-emerald-400">connected to 192.168.1.142:5555</p>
                  <p className="text-zinc-400">$ adb install -r staging-build-v2.4.apk</p>
                  <p className="text-zinc-400">Success · Application installed and launched in virtual display session</p>
                  <p className="text-zinc-400">* Browser Touch Streaming: 60 FPS WebRTC low-latency session open</p>
                </div>
                <div className="pt-2 flex items-center gap-2">
                  <span className="px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-[10px] text-zinc-300">
                    Host Kernel LXC Acceleration
                  </span>
                  <span className="px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-[10px] text-zinc-300">
                    Zero QEMU Emulation Penalty
                  </span>
                </div>
              </div>
            )}

            {activeTab === 'ai' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-[11px] text-zinc-400 border-b border-zinc-900 pb-2">
                  <span className="text-purple-400">● NEVATAL AI CLOUD AGENT · READY</span>
                  <span>Model Context Protocol (MCP) Server Active</span>
                </div>
                <div className="space-y-1 text-zinc-400">
                  <p className="text-zinc-200">&gt; "Provision an Ubuntu 24.04 instance with 4GB RAM named web-prod-01"</p>
                  <p className="text-purple-400">* Tool Invocation: nevatal.provision_instance(os="ubuntu-24.04", ram_mb=4096, name="web-prod-01")</p>
                  <p className="text-zinc-400">* Checking quota allocations: 4 vCPU remaining, 8 GB RAM remaining... APPROVED</p>
                  <p className="text-emerald-400">✓ Instance #105 created on pve-dell-r640. IP assigned: 192.168.1.145</p>
                </div>
                <div className="pt-2 flex items-center gap-2">
                  <span className="px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-[10px] text-zinc-300">
                    Claude Desktop / Cursor MCP Connected
                  </span>
                </div>
              </div>
            )}
          </div>
        </div>
      </section>

      {/* Hardware Telemetry Proof Bar */}
      <section className="py-10 px-4 sm:px-8 border-b border-zinc-850 bg-zinc-900/20">
        <div className="max-w-5xl mx-auto grid grid-cols-2 lg:grid-cols-4 gap-4">
          <div className="p-4 rounded-lg bg-zinc-900/40 border border-zinc-850">
            <div className="text-[10px] font-mono text-zinc-400 uppercase tracking-wider mb-1">Compute Node</div>
            <div className="text-sm sm:text-base font-semibold text-white">Dell PowerEdge R640</div>
            <div className="text-[11px] text-zinc-400 font-mono mt-0.5">Dual Intel Xeon Scalable</div>
          </div>
          <div className="p-4 rounded-lg bg-zinc-900/40 border border-zinc-850">
            <div className="text-[10px] font-mono text-zinc-400 uppercase tracking-wider mb-1">Provision Time</div>
            <div className="text-sm sm:text-base font-semibold text-white">&lt; 30 Seconds</div>
            <div className="text-[11px] text-zinc-400 font-mono mt-0.5">Cloud-Init to SSH Ready</div>
          </div>
          <div className="p-4 rounded-lg bg-zinc-900/40 border border-zinc-850">
            <div className="text-[10px] font-mono text-zinc-400 uppercase tracking-wider mb-1">Storage Subsystem</div>
            <div className="text-sm sm:text-base font-semibold text-white">Enterprise NVMe ZFS</div>
            <div className="text-[11px] text-zinc-400 font-mono mt-0.5">High-IOPS Mirrored Pools</div>
          </div>
          <div className="p-4 rounded-lg bg-zinc-900/40 border border-zinc-850">
            <div className="text-[10px] font-mono text-zinc-400 uppercase tracking-wider mb-1">Hypervisor Core</div>
            <div className="text-sm sm:text-base font-semibold text-white">Proxmox VE 8.x</div>
            <div className="text-[11px] text-zinc-400 font-mono mt-0.5">REST API v2 Orchestration</div>
          </div>
        </div>
      </section>

      {/* 6 Product Pillars */}
      <section id="compute" className="py-16 sm:py-24 px-4 sm:px-8 border-b border-zinc-850">
        <div className="max-w-5xl mx-auto space-y-12">
          <div className="text-center space-y-2 max-w-xl mx-auto">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-white">
              Engineered for developer productivity.
            </h2>
            <p className="text-xs sm:text-sm text-zinc-400">
              Six dedicated cloud pillars running on isolated, high-performance bare-metal hardware.
            </p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-5">
            {/* 1. Linux VPS */}
            <div className="p-5 rounded-lg border border-zinc-800 bg-zinc-900/30 hover:border-zinc-700 transition-colors flex flex-col justify-between">
              <div className="space-y-3">
                <div className="w-8 h-8 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-200">
                  <Server className="w-4 h-4" />
                </div>
                <h3 className="text-sm font-semibold text-white">Bare-Metal Linux Compute</h3>
                <p className="text-xs text-zinc-400 leading-relaxed">
                  Fast KVM virtual servers with automated cloud-init setup. Launch Ubuntu 24.04, Debian 12, Alpine, or WordPress LEMP with full root SSH and HTML5 noVNC console.
                </p>
              </div>
              <div className="mt-4 pt-3 border-t border-zinc-850 flex items-center justify-between text-[11px] font-mono text-zinc-400">
                <span>Cloud-Init Templates</span>
                <span className="text-zinc-300">#9000 - #9003</span>
              </div>
            </div>

            {/* 2. Game Servers */}
            <div id="games" className="p-5 rounded-lg border border-zinc-800 bg-zinc-900/30 hover:border-zinc-700 transition-colors flex flex-col justify-between">
              <div className="space-y-3">
                <div className="w-8 h-8 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-200">
                  <Gamepad2 className="w-4 h-4" />
                </div>
                <h3 className="text-sm font-semibold text-white">Managed Game Servers</h3>
                <p className="text-xs text-zinc-400 leading-relaxed">
                  Turnkey multiplayer servers for Minecraft Java (Paper, Purpur, Fabric, Forge) &amp; Bedrock, plus Terraria. Visual properties editor, automated backups, and live console.
                </p>
              </div>
              <div className="mt-4 pt-3 border-t border-zinc-850 flex items-center justify-between text-[11px] font-mono text-zinc-400">
                <span>Minecraft &amp; Terraria</span>
                <span className="text-zinc-300">&lt; 15ms Latency</span>
              </div>
            </div>

            {/* 3. Android Cloud */}
            <div id="android" className="p-5 rounded-lg border border-zinc-800 bg-zinc-900/30 hover:border-zinc-700 transition-colors flex flex-col justify-between">
              <div className="space-y-3">
                <div className="w-8 h-8 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-200">
                  <Smartphone className="w-4 h-4" />
                </div>
                <h3 className="text-sm font-semibold text-white">Waydroid Android Cloud</h3>
                <p className="text-xs text-zinc-400 leading-relaxed">
                  Containerized Android OS running directly on Linux without CPU emulation penalties. Stream interactive touch sessions in your browser and connect via network ADB.
                </p>
              </div>
              <div className="mt-4 pt-3 border-t border-zinc-850 flex items-center justify-between text-[11px] font-mono text-zinc-400">
                <span>Waydroid LXC</span>
                <span className="text-zinc-300">ADB Port 5555</span>
              </div>
            </div>

            {/* 4. AI Cloud Agent */}
            <div className="p-5 rounded-lg border border-zinc-800 bg-zinc-900/30 hover:border-zinc-700 transition-colors flex flex-col justify-between">
              <div className="space-y-3">
                <div className="w-8 h-8 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-200">
                  <Sparkles className="w-4 h-4" />
                </div>
                <h3 className="text-sm font-semibold text-white">AI Cloud Agent &amp; MCP</h3>
                <p className="text-xs text-zinc-400 leading-relaxed">
                  Manage your cloud using natural language. Query instance telemetry, trigger rollbacks, and connect infrastructure directly to Claude Desktop &amp; Cursor via Model Context Protocol.
                </p>
              </div>
              <div className="mt-4 pt-3 border-t border-zinc-850 flex items-center justify-between text-[11px] font-mono text-zinc-400">
                <span>OpenRouter &amp; MCP</span>
                <span className="text-zinc-300">Claude / Cursor</span>
              </div>
            </div>

            {/* 5. App Deployments & Automation */}
            <div className="p-5 rounded-lg border border-zinc-800 bg-zinc-900/30 hover:border-zinc-700 transition-colors flex flex-col justify-between">
              <div className="space-y-3">
                <div className="w-8 h-8 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-200">
                  <Workflow className="w-4 h-4" />
                </div>
                <h3 className="text-sm font-semibold text-white">Git Deploy &amp; Automations</h3>
                <p className="text-xs text-zinc-400 leading-relaxed">
                  Continuous deployment for Node.js, Python, and Go web apps straight from Git. Spin up dedicated n8n workflow engines with 400+ SaaS connectors and webhook triggers.
                </p>
              </div>
              <div className="mt-4 pt-3 border-t border-zinc-850 flex items-center justify-between text-[11px] font-mono text-zinc-400">
                <span>Git Push &amp; n8n</span>
                <span className="text-zinc-300">400+ Nodes</span>
              </div>
            </div>

            {/* 6. Object Storage */}
            <div className="p-5 rounded-lg border border-zinc-800 bg-zinc-900/30 hover:border-zinc-700 transition-colors flex flex-col justify-between">
              <div className="space-y-3">
                <div className="w-8 h-8 rounded bg-zinc-900 border border-zinc-800 flex items-center justify-center text-zinc-200">
                  <Database className="w-4 h-4" />
                </div>
                <h3 className="text-sm font-semibold text-white">S3 Object Storage</h3>
                <p className="text-xs text-zinc-400 leading-relaxed">
                  High-throughput MinIO S3-compatible object buckets. Manage access keys, pre-signed upload URLs, and browse files via the built-in graphical file manager.
                </p>
              </div>
              <div className="mt-4 pt-3 border-t border-zinc-850 flex items-center justify-between text-[11px] font-mono text-zinc-400">
                <span>MinIO S3 Compatible</span>
                <span className="text-zinc-300">AWS SDK Ready</span>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* Developer FAQ */}
      <section className="py-16 sm:py-24 px-4 sm:px-8 border-b border-zinc-850">
        <div className="max-w-3xl mx-auto space-y-8">
          <div className="text-center space-y-2">
            <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-white">
              Frequently Asked Questions
            </h2>
            <p className="text-xs sm:text-sm text-zinc-400">
              Technical answers regarding architecture, networking, and platform operations.
            </p>
          </div>

          <div className="space-y-3">
            {faqs.map((faq, idx) => (
              <div
                key={idx}
                className="rounded-lg border border-zinc-800 bg-zinc-900/30 overflow-hidden"
              >
                <button
                  onClick={() => toggleFaq(idx)}
                  className="w-full p-4 text-left flex items-center justify-between text-xs sm:text-sm font-medium text-white hover:text-zinc-200 transition-colors"
                >
                  <span>{faq.q}</span>
                  <ChevronDown
                    className={`w-4 h-4 text-zinc-400 transition-transform duration-200 flex-shrink-0 ml-3 ${
                      openFaq === idx ? 'rotate-180 text-white' : ''
                    }`}
                  />
                </button>
                {openFaq === idx && (
                  <div className="px-4 pb-4 pt-1 text-xs text-zinc-400 leading-relaxed border-t border-zinc-850/60">
                    {faq.a}
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      </section>

      {/* Bottom CTA */}
      <section className="py-16 sm:py-20 px-4 sm:px-8 bg-zinc-900/30 text-center">
        <div className="max-w-3xl mx-auto space-y-5">
          <div className="inline-flex items-center gap-2 px-3 py-1 rounded-full bg-emerald-950/40 border border-emerald-800/60 text-[11px] font-mono text-emerald-300">
            <span className="w-1.5 h-1.5 rounded-full bg-emerald-400" />
            <span>COMMUNITY ACCESS · 100% FREE FOR NOW</span>
          </div>
          <h2 className="text-2xl sm:text-3xl font-bold tracking-tight text-white">
            Ready to deploy your next workload?
          </h2>
          <p className="text-xs sm:text-sm text-zinc-400 max-w-lg mx-auto">
            Experience bare-metal cloud performance backed by dedicated Dell PowerEdge R640 nodes. It is completely free for now—no payment or credit card required.
          </p>
          <div className="flex flex-wrap items-center justify-center gap-3 pt-2">
            <button
              onClick={() => onNavigate(user ? '/dashboard' : '/register')}
              className="text-xs sm:text-sm font-medium px-5 py-2.5 rounded-md bg-white text-zinc-950 hover:bg-zinc-200 transition-colors"
            >
              Get Started for Free
            </button>
            <button
              onClick={() => onNavigate('/docs')}
              className="text-xs sm:text-sm font-medium px-4 py-2.5 rounded-md bg-zinc-900 border border-zinc-800 hover:border-zinc-700 text-zinc-300 hover:text-white transition-colors"
            >
              Read Docs
            </button>
          </div>
        </div>
      </section>

      {/* Minimal Footer */}
      <footer className="border-t border-zinc-850 px-4 sm:px-8 py-10 bg-zinc-950 text-xs text-zinc-400">
        <div className="max-w-6xl mx-auto flex flex-col md:flex-row items-center justify-between gap-6">
          <div className="flex items-center gap-3">
            <div className="w-5 h-5 rounded bg-zinc-100 flex items-center justify-center text-zinc-950 font-bold text-[10px]">
              NV
            </div>
            <span className="font-semibold text-white">Nevatal Cloud</span>
            <span className="text-zinc-600">·</span>
            <span>Self-hosted Proxmox VE Control Plane</span>
          </div>

          <div className="flex flex-wrap items-center gap-5 text-zinc-400 font-mono text-[11px]">
            <button onClick={() => onNavigate('/docs')} className="hover:text-white transition-colors">
              Documentation
            </button>
            <button onClick={() => onNavigate('/docs/rest-api')} className="hover:text-white transition-colors">
              API Reference
            </button>
            <button onClick={() => onNavigate('/dashboard')} className="hover:text-white transition-colors">
              Console
            </button>
            <a
              href="https://github.com/thirza258/nevatal-cloud"
              target="_blank"
              rel="noreferrer"
              className="hover:text-white transition-colors flex items-center gap-1"
            >
              <span>GitHub</span>
              <ExternalLink className="w-3 h-3" />
            </a>
          </div>
        </div>

        <div className="max-w-6xl mx-auto mt-6 pt-6 border-t border-zinc-900 flex flex-col sm:flex-row items-center justify-between text-[11px] text-zinc-400 gap-3">
          <div>&copy; {new Date().getFullYear()} Nevatal Cloud. All rights reserved.</div>
          <div className="font-mono">Node: Dell PowerEdge R640 · Hypervisor: Proxmox VE 8</div>
        </div>
      </footer>
    </div>
  );
};
