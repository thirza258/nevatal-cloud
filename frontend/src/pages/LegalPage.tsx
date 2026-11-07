import React, { useState } from 'react';
import { Shield, FileText, Server, Cpu, ExternalLink, ArrowLeft } from 'lucide-react';

interface LegalPageProps {
  onNavigate?: (path: string) => void;
  initialTab?: 'privacy' | 'terms';
}

export const LegalPage: React.FC<LegalPageProps> = ({ onNavigate, initialTab = 'privacy' }) => {
  const [activeTab, setActiveTab] = useState<'privacy' | 'terms'>(initialTab);

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 flex flex-col font-sans">
      <header className="border-b border-zinc-850 px-6 py-4 flex items-center justify-between bg-zinc-900/50 backdrop-blur">
        <button
          onClick={() => onNavigate ? onNavigate('/') : (window.location.href = '/')}
          className="inline-flex items-center gap-2 text-sm text-zinc-400 hover:text-white transition-colors"
        >
          <ArrowLeft size={16} /> Back to Nevatal Cloud
        </button>
        <div className="text-xs font-mono text-zinc-400">
          cloud.nevatal.id · Self-Hosted Infrastructure
        </div>
      </header>

      <main className="flex-1 max-w-4xl mx-auto px-4 py-12 w-full">
        <div className="flex flex-wrap items-center justify-between gap-4 border-b border-zinc-800 pb-6 mb-8">
          <div>
            <h1 className="text-3xl font-bold tracking-tight text-white flex items-center gap-3">
              {activeTab === 'privacy' ? <Shield className="text-indigo-400" size={32} /> : <FileText className="text-indigo-400" size={32} />}
              {activeTab === 'privacy' ? 'Privacy Policy' : 'Terms of Service'}
            </h1>
            <p className="text-sm text-zinc-400 mt-1">
              Effective Date: November 2026 · Nevatal Cloud Virtual Infrastructure
            </p>
          </div>

          <div className="flex bg-zinc-900 p-1 rounded-lg border border-zinc-800">
            <button
              type="button"
              onClick={() => setActiveTab('privacy')}
              className={`px-4 py-2 rounded-md text-sm font-medium transition-all ${
                activeTab === 'privacy'
                  ? 'bg-zinc-100 text-zinc-950 shadow-sm'
                  : 'text-zinc-400 hover:text-white'
              }`}
            >
              Privacy Policy
            </button>
            <button
              type="button"
              onClick={() => setActiveTab('terms')}
              className={`px-4 py-2 rounded-md text-sm font-medium transition-all ${
                activeTab === 'terms'
                  ? 'bg-zinc-100 text-zinc-950 shadow-sm'
                  : 'text-zinc-400 hover:text-white'
              }`}
            >
              Terms of Service
            </button>
          </div>
        </div>

        {activeTab === 'privacy' ? (
          <div className="space-y-8 text-sm leading-relaxed text-zinc-300">
            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3 flex items-center gap-2">
                <Server className="text-indigo-400" size={20} />
                1. Fully Self-Hosted Cloud Infrastructure
              </h2>
              <p className="mb-3">
                Nevatal Cloud operates as a strictly self-hosted cloud and virtualization management platform on your private servers
                (Proxmox VE hypervisors, QEMU/KVM virtual machines, LXC containers, and local S3 MinIO storage).
              </p>
              <p>
                All VM disks, operating system snapshots, SSH keys, user passwords (hashed via Argon2),
                and private networking configurations remain completely isolated within your local deployment.
                There are no external telemetry pingbacks, analytics tracking pixels, or third-party data broker integrations.
              </p>
            </section>

            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-855">
              <h2 className="text-lg font-semibold text-white mb-3 flex items-center gap-2">
                <Cpu className="text-indigo-400" size={20} />
                2. AI Copilot & OpenRouter Inference Disclosure
              </h2>
              <p className="mb-3">
                Nevatal Cloud features an AI Cloud Assistant and automation copilot.
                AI inference is powered by <strong>OpenRouter</strong> as the external upstream model router.
              </p>
              <p className="mb-3">
                When you interact with the AI Cloud Assistant:
              </p>
              <ul className="list-disc list-inside space-y-1 pl-2 mb-4">
                <li>Only the prompt query and contextual configuration parameters you approve are transmitted to OpenRouter.</li>
                <li>Your root hypervisor credentials, private SSH keys, and encrypted disk volumes are never transmitted.</li>
              </ul>
              <div className="bg-zinc-950 p-4 rounded-lg border border-zinc-800 space-y-2">
                <p className="text-xs uppercase tracking-wider font-semibold text-zinc-300">
                  OpenRouter Terms &amp; Privacy Links
                </p>
                <div className="flex flex-wrap gap-4 text-xs">
                  <a
                    href="https://openrouter.ai/privacy"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1.5 text-indigo-400 hover:underline"
                  >
                    OpenRouter Privacy Policy <ExternalLink size={12} />
                  </a>
                  <a
                    href="https://openrouter.ai/terms"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="inline-flex items-center gap-1.5 text-indigo-400 hover:underline"
                  >
                    OpenRouter Terms of Service <ExternalLink size={12} />
                  </a>
                </div>
              </div>
            </section>

            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3">3. Storage &amp; Cookies</h2>
              <p>
                Session state and authorization tokens are maintained via HTTP cookies and local storage exclusively for platform operation.
                These tokens never leave your configured domain.
              </p>
            </section>

            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3">4. Privacy Contact</h2>
              <p>
                For questions regarding data practices or privacy policies, please contact:{" "}
                <a href="mailto:nevatalapp@gmail.com" className="text-indigo-400 hover:underline font-mono">
                  nevatalapp@gmail.com
                </a>
              </p>
            </section>
          </div>
        ) : (
          <div className="space-y-8 text-sm leading-relaxed text-zinc-300">
            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3">1. Agreement to Terms</h2>
              <p>
                By deploying, accessing, or configuring instances in Nevatal Cloud, you agree to these Terms of Service.
                You are responsible for safeguarding access credentials to your compute nodes and administrative panels.
              </p>
            </section>

            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3">2. System Operations &amp; Backups</h2>
              <p className="mb-2">
                As a self-hosted platform, the administrator of the host environment is solely responsible for
                maintaining periodic backups of virtual machines, PostgreSQL databases, and Proxmox storage pools.
              </p>
              <p>
                Nevatal Cloud software does not provide automatic offsite replication unless configured by the host administrator.
              </p>
            </section>

            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3">3. AI Copilot Guidelines</h2>
              <p>
                Commands and infrastructure advice suggested by the AI Copilot must be reviewed by the system administrator
                prior to execution. Maintainers are not liable for accidental data loss caused by AI-suggested automation scripts.
              </p>
            </section>

            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3">4. Published Sites &amp; Advertising</h2>
              <p className="mb-2">
                Nevatal Cloud is free to use. When you publish a VM port through Nevatal Domains
                (<code className="font-mono text-zinc-200">https://&lt;name&gt;.&lt;PUBLIC_DOMAIN_ROOT&gt;</code>), the edge proxy
                may add a small, clearly labelled advertisement served by the platform&rsquo;s AdControl deployment to the HTML
                pages it proxies. This is how the free platform is funded.
              </p>
              <p>
                The advertisement is a script and placement container appended before the closing body tag; it does not alter
                your application data, API responses, or non-HTML content. A strict Content-Security-Policy on your app may block
                it. Do not publish pages where third-party advertising is prohibited by your own agreements.
              </p>
            </section>

            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3">5. Disclaimer of Warranties</h2>
              <p>
                The software is provided &ldquo;AS IS&rdquo;, without warranty of any kind, express or implied.
                The authors or copyright holders shall not be liable for any claim, damages, or other liability arising from the software.
              </p>
            </section>

            <section className="bg-zinc-900/60 p-6 rounded-xl border border-zinc-850">
              <h2 className="text-lg font-semibold text-white mb-3">6. Terms Contact</h2>
              <p>
                For terms inquiries or administrative notices, contact:{" "}
                <a href="mailto:nevatalapp@gmail.com" className="text-indigo-400 hover:underline font-mono">
                  nevatalapp@gmail.com
                </a>
              </p>
            </section>
          </div>
        )}
      </main>

      <footer className="border-t border-zinc-850 bg-zinc-950 py-6 text-center text-xs text-zinc-500 space-y-1">
        <p>Nevatal Cloud · Self-Hosted Cloud Platform · Legal &amp; Policies</p>
        <p>This project is under nevatal.id for educational purposes.</p>
      </footer>
    </div>
  );
};
