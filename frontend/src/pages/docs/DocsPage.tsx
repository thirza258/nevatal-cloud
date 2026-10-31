import React, { useState, useEffect } from 'react';
import {
  DOCS_CATEGORIES,
  ALL_DOC_ARTICLES,
  DocArticle,
  getArticleById,
} from './docsData';
import {
  Search,
  BookOpen,
  Zap,
  Server,
  Gamepad2,
  Smartphone,
  Workflow,
  Database,
  Shield,
  Clock,
  Check,
  Copy,
  ChevronRight,
  ExternalLink,
  ArrowLeft,
  ArrowRight,
  Menu,
  X,
  Sparkles,
  Layers,
  Terminal,
  AlertCircle,
  Info,
  CheckCircle2,
  AlertTriangle,
} from 'lucide-react';
import { useAuth } from '../../context/AuthContext';

interface DocsPageProps {
  onNavigate: (path: string, param?: string) => void;
  initialArticleId?: string;
}

export const DocsPage: React.FC<DocsPageProps> = ({ onNavigate, initialArticleId }) => {
  const { user } = useAuth();
  const [currentArticleId, setCurrentArticleId] = useState<string>(
    initialArticleId || 'overview'
  );
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [copiedCodeIndex, setCopiedCodeIndex] = useState<number | null>(null);
  const [copiedLink, setCopiedLink] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  useEffect(() => {
    if (initialArticleId) {
      setCurrentArticleId(initialArticleId);
    }
  }, [initialArticleId]);

  const currentArticle = getArticleById(currentArticleId) || ALL_DOC_ARTICLES[0];

  // Category icons mapping
  const getCategoryIcon = (iconName: string) => {
    switch (iconName) {
      case 'Zap':
        return Zap;
      case 'Server':
        return Server;
      case 'Gamepad2':
        return Gamepad2;
      case 'Smartphone':
        return Smartphone;
      case 'Workflow':
        return Workflow;
      case 'Database':
        return Database;
      case 'Shield':
        return Shield;
      default:
        return BookOpen;
    }
  };

  const copyToClipboard = (text: string, index?: number) => {
    navigator.clipboard.writeText(text);
    if (typeof index === 'number') {
      setCopiedCodeIndex(index);
      setTimeout(() => setCopiedCodeIndex(null), 2000);
    } else {
      setCopiedLink(true);
      setTimeout(() => setCopiedLink(false), 2000);
    }
  };

  // Filter articles based on search query
  const filteredCategories = DOCS_CATEGORIES.map((cat) => ({
    ...cat,
    articles: cat.articles.filter(
      (art) =>
        art.title.toLowerCase().includes(searchQuery.toLowerCase()) ||
        art.summary.toLowerCase().includes(searchQuery.toLowerCase()) ||
        art.sections.some(
          (s) =>
            s.heading.toLowerCase().includes(searchQuery.toLowerCase()) ||
            s.body.toLowerCase().includes(searchQuery.toLowerCase())
        )
    ),
  })).filter((cat) => cat.articles.length > 0);

  // Find previous and next articles
  const currentIdx = ALL_DOC_ARTICLES.findIndex((a) => a.id === currentArticle.id);
  const prevArticle = currentIdx > 0 ? ALL_DOC_ARTICLES[currentIdx - 1] : null;
  const nextArticle =
    currentIdx < ALL_DOC_ARTICLES.length - 1
      ? ALL_DOC_ARTICLES[currentIdx + 1]
      : null;

  return (
    <div className="min-h-screen bg-zinc-950 text-zinc-100 flex flex-col font-sans selection:bg-zinc-800 selection:text-white">
      {/* Top Docs Header */}
      <header className="sticky top-0 z-40 bg-zinc-950/90 backdrop-blur border-b border-zinc-800 px-4 sm:px-6 h-14 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <button
            onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
            className="md:hidden p-1.5 rounded-md text-zinc-400 hover:text-white hover:bg-zinc-900"
          >
            {mobileMenuOpen ? <X className="w-5 h-5" /> : <Menu className="w-5 h-5" />}
          </button>

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
                docs
              </span>
            </div>
          </div>

          <div className="hidden sm:flex items-center gap-1.5 text-xs text-zinc-400 font-mono pl-3 border-l border-zinc-800">
            <span>Documentation</span>
            <span className="text-zinc-600">/</span>
            <span className="text-zinc-300">{currentArticle.category}</span>
          </div>
        </div>

        {/* Header Right Actions */}
        <div className="flex items-center gap-3">
          <button
            onClick={() => onNavigate('/')}
            className="text-xs text-zinc-400 hover:text-white transition-colors hidden sm:inline-block"
          >
            Home
          </button>

          {user ? (
            <button
              onClick={() => onNavigate('/dashboard')}
              className="flex items-center gap-1.5 text-xs font-medium px-3 py-1.5 rounded-md bg-white text-zinc-950 hover:bg-zinc-200 transition-colors"
            >
              <Layers className="w-3.5 h-3.5" />
              <span>Dashboard</span>
            </button>
          ) : (
            <div className="flex items-center gap-2">
              <button
                onClick={() => onNavigate('/login')}
                className="text-xs text-zinc-300 hover:text-white px-2.5 py-1.5"
              >
                Sign In
              </button>
              <button
                onClick={() => onNavigate('/register')}
                className="text-xs font-medium px-3 py-1.5 rounded-md bg-white text-zinc-950 hover:bg-zinc-200 transition-colors"
              >
                Get Started
              </button>
            </div>
          )}
        </div>
      </header>

      {/* Main Container: Sidebar + Article */}
      <div className="flex-1 flex max-w-7xl w-full mx-auto">
        {/* Left Sidebar Navigation */}
        <aside
          className={`fixed inset-y-14 left-0 z-30 w-72 bg-zinc-950 border-r border-zinc-800 p-4 overflow-y-auto transition-transform duration-200 md:static md:translate-x-0 ${
            mobileMenuOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'
          }`}
        >
          {/* Search Box */}
          <div className="relative mb-5">
            <Search className="w-3.5 h-3.5 absolute left-3 top-1/2 -translate-y-1/2 text-zinc-400" />
            <input
              type="text"
              placeholder="Search documentation..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full bg-zinc-900 border border-zinc-800 rounded-md pl-9 pr-3 py-1.5 text-xs text-zinc-100 placeholder-zinc-400 focus:outline-none focus:border-zinc-700"
            />
            {searchQuery && (
              <button
                onClick={() => setSearchQuery('')}
                className="absolute right-2.5 top-1/2 -translate-y-1/2 text-zinc-400 hover:text-white text-xs"
              >
                ×
              </button>
            )}
          </div>

          {/* Navigation Category Groups */}
          <div className="space-y-6">
            {filteredCategories.length === 0 ? (
              <div className="text-center py-6 text-xs text-zinc-400">
                No matching documentation articles found.
              </div>
            ) : (
              filteredCategories.map((cat) => {
                const Icon = getCategoryIcon(cat.iconName);
                return (
                  <div key={cat.name} className="space-y-1.5">
                    <div className="flex items-center gap-2 px-2 text-[10px] font-mono uppercase tracking-wider text-zinc-400">
                      <Icon className="w-3 h-3 text-zinc-400" />
                      <span>{cat.name}</span>
                    </div>

                    <div className="space-y-0.5">
                      {cat.articles.map((art) => {
                        const active = art.id === currentArticle.id;
                        return (
                          <button
                            key={art.id}
                            onClick={() => {
                              setCurrentArticleId(art.id);
                              setMobileMenuOpen(false);
                              window.scrollTo({ top: 0, behavior: 'smooth' });
                            }}
                            className={`w-full text-left text-xs py-1.5 rounded transition-colors flex items-center justify-between ${
                              active
                                ? 'bg-zinc-900 text-white font-medium border-l-2 border-white pl-2.5 pr-2'
                                : 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-900/60 pl-3 pr-2'
                            }`}
                          >
                            <span className="truncate">{art.title}</span>
                            {active && <ChevronRight className="w-3 h-3 text-white flex-shrink-0" />}
                          </button>
                        );
                      })}
                    </div>
                  </div>
                );
              })
            )}
          </div>

          {/* Bottom Help Box */}
          <div className="mt-8 pt-4 border-t border-zinc-850 p-3 rounded-lg bg-zinc-900/40 border border-zinc-800 text-xs">
            <div className="flex items-center gap-2 font-medium text-white mb-1">
              <Sparkles className="w-3.5 h-3.5 text-zinc-300" />
              <span>Need Assistance?</span>
            </div>
            <p className="text-[11px] text-zinc-400 leading-relaxed mb-2.5">
              Ask the Nevatal AI Cloud Agent for personalized troubleshooting or instant provisioning scripts.
            </p>
            <button
              onClick={() => onNavigate('/dashboard/ai')}
              className="w-full py-1 px-2 rounded bg-zinc-800 hover:bg-zinc-700 text-white text-[11px] font-medium transition-colors"
            >
              Open AI Cloud Agent
            </button>
          </div>
        </aside>

        {/* Main Article Content */}
        <main className="flex-1 min-w-0 p-4 sm:p-8 lg:p-10">
          <div className="max-w-3xl">
            {/* Breadcrumb & Meta */}
            <div className="flex flex-wrap items-center justify-between gap-3 mb-4">
              <div className="flex items-center gap-2 text-xs font-mono text-zinc-400">
                <span className="px-2 py-0.5 rounded bg-zinc-900 border border-zinc-800 text-zinc-300">
                  {currentArticle.category}
                </span>
                <span>·</span>
                <span className="flex items-center gap-1 text-zinc-400">
                  <Clock className="w-3 h-3" />
                  {currentArticle.readTime}
                </span>
              </div>

              <button
                onClick={() => copyToClipboard(window.location.href)}
                className="flex items-center gap-1.5 text-xs text-zinc-400 hover:text-white px-2 py-1 rounded bg-zinc-900/60 border border-zinc-800 hover:border-zinc-700 transition-colors"
              >
                {copiedLink ? (
                  <>
                    <Check className="w-3 h-3 text-emerald-400" />
                    <span className="text-emerald-400">Copied</span>
                  </>
                ) : (
                  <>
                    <Copy className="w-3 h-3" />
                    <span>Copy Link</span>
                  </>
                )}
              </button>
            </div>

            {/* Title & Summary */}
            <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-white mb-3">
              {currentArticle.title}
            </h1>
            <p className="text-sm sm:text-base text-zinc-350 leading-relaxed mb-8 pb-6 border-b border-zinc-800">
              {currentArticle.summary}
            </p>

            {/* Sections */}
            <div className="space-y-8">
              {currentArticle.sections.map((section, idx) => (
                <section key={idx} className="space-y-3.5">
                  <h2 className="text-lg font-semibold tracking-tight text-white">
                    {section.heading}
                  </h2>
                  <p className="text-xs sm:text-sm text-zinc-300 leading-relaxed whitespace-pre-line">
                    {section.body}
                  </p>

                  {/* Code Snippet Block */}
                  {section.codeSnippet && (
                    <div className="rounded-lg border border-zinc-800 bg-zinc-900/80 overflow-hidden my-3">
                      <div className="flex items-center justify-between px-3.5 py-1.5 bg-zinc-900 border-b border-zinc-800 text-[11px] font-mono text-zinc-400">
                        <div className="flex items-center gap-2">
                          <Terminal className="w-3.5 h-3.5 text-zinc-400" />
                          <span className="uppercase">{section.codeSnippet.language}</span>
                          {section.codeSnippet.caption && (
                            <span className="text-zinc-400 font-sans">
                              — {section.codeSnippet.caption}
                            </span>
                          )}
                        </div>
                        <button
                          onClick={() => copyToClipboard(section.codeSnippet!.code, idx)}
                          className="flex items-center gap-1 hover:text-white transition-colors"
                        >
                          {copiedCodeIndex === idx ? (
                            <>
                              <Check className="w-3 h-3 text-emerald-400" />
                              <span className="text-emerald-400">Copied</span>
                            </>
                          ) : (
                            <>
                              <Copy className="w-3 h-3" />
                              <span>Copy</span>
                            </>
                          )}
                        </button>
                      </div>
                      <pre className="p-4 text-xs font-mono text-zinc-200 overflow-x-auto leading-relaxed">
                        <code>{section.codeSnippet.code}</code>
                      </pre>
                    </div>
                  )}

                  {/* Alert Callout Block */}
                  {section.alert && (
                    <div
                      className={`p-3.5 rounded-lg border text-xs leading-relaxed flex items-start gap-3 my-3 ${
                        section.alert.type === 'tip'
                          ? 'bg-emerald-950/20 border-emerald-800/60 text-emerald-200'
                          : section.alert.type === 'warning'
                          ? 'bg-amber-950/20 border-amber-800/60 text-amber-200'
                          : section.alert.type === 'important'
                          ? 'bg-purple-950/20 border-purple-800/60 text-purple-200'
                          : 'bg-blue-950/20 border-blue-800/60 text-blue-200'
                      }`}
                    >
                      {section.alert.type === 'tip' && <CheckCircle2 className="w-4 h-4 flex-shrink-0 text-emerald-400 mt-0.5" />}
                      {section.alert.type === 'warning' && <AlertTriangle className="w-4 h-4 flex-shrink-0 text-amber-400 mt-0.5" />}
                      {section.alert.type === 'important' && <AlertCircle className="w-4 h-4 flex-shrink-0 text-purple-400 mt-0.5" />}
                      {section.alert.type === 'note' && <Info className="w-4 h-4 flex-shrink-0 text-blue-400 mt-0.5" />}
                      <div>{section.alert.text}</div>
                    </div>
                  )}

                  {/* Table Block */}
                  {section.table && (
                    <div className="rounded-lg border border-zinc-800 overflow-hidden my-3">
                      <div className="overflow-x-auto">
                        <table className="w-full text-left text-xs">
                          <thead className="bg-zinc-900 border-b border-zinc-800 text-zinc-300 font-mono">
                            <tr>
                              {section.table.headers.map((h, hIdx) => (
                                <th key={hIdx} className="px-3.5 py-2.5 font-semibold">
                                  {h}
                                </th>
                              ))}
                            </tr>
                          </thead>
                          <tbody className="divide-y divide-zinc-800/60">
                            {section.table.rows.map((row, rIdx) => (
                              <tr key={rIdx} className="hover:bg-zinc-900/40">
                                {row.map((cell, cIdx) => (
                                  <td
                                    key={cIdx}
                                    className={`px-3.5 py-2.5 text-zinc-300 ${
                                      cIdx === 0 ? 'font-mono text-white' : ''
                                    }`}
                                  >
                                    {cell}
                                  </td>
                                ))}
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    </div>
                  )}
                </section>
              ))}
            </div>

            {/* Pagination: Previous & Next Articles */}
            <div className="mt-12 pt-6 border-t border-zinc-800 flex items-center justify-between gap-4">
              {prevArticle ? (
                <button
                  onClick={() => {
                    setCurrentArticleId(prevArticle.id);
                    window.scrollTo({ top: 0, behavior: 'smooth' });
                  }}
                  className="flex flex-col items-start p-3 rounded-lg border border-zinc-800 hover:border-zinc-700 bg-zinc-900/30 hover:bg-zinc-900/60 transition-colors text-left max-w-[48%]"
                >
                  <span className="text-[10px] font-mono text-zinc-400 flex items-center gap-1">
                    <ArrowLeft className="w-3 h-3" /> Previous
                  </span>
                  <span className="text-xs font-medium text-white truncate mt-1">
                    {prevArticle.title}
                  </span>
                </button>
              ) : (
                <div />
              )}

              {nextArticle ? (
                <button
                  onClick={() => {
                    setCurrentArticleId(nextArticle.id);
                    window.scrollTo({ top: 0, behavior: 'smooth' });
                  }}
                  className="flex flex-col items-end p-3 rounded-lg border border-zinc-800 hover:border-zinc-700 bg-zinc-900/30 hover:bg-zinc-900/60 transition-colors text-right max-w-[48%]"
                >
                  <span className="text-[10px] font-mono text-zinc-400 flex items-center gap-1">
                    Next <ArrowRight className="w-3 h-3" />
                  </span>
                  <span className="text-xs font-medium text-white truncate mt-1">
                    {nextArticle.title}
                  </span>
                </button>
              ) : (
                <div />
              )}
            </div>

            {/* Feedback / GitHub footer */}
            <div className="mt-8 text-center text-xs text-zinc-400">
              Nevatal Cloud Documentation · Last updated {currentArticle.lastUpdated}
            </div>
          </div>
        </main>
      </div>
    </div>
  );
};
