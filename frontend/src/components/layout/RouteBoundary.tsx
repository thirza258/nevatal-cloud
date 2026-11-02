import React from 'react';
import { AlertTriangle, RefreshCw } from 'lucide-react';

const Spinner: React.FC = () => (
  <div className="flex items-center gap-2 text-xs font-mono text-zinc-500">
    <span className="w-3.5 h-3.5 rounded-full border-2 border-zinc-700 border-t-zinc-300 animate-spin" />
    <span>Loading…</span>
  </div>
);

/** Suspense fallback inside the AppShell content area (sidebar stays visible). */
export const PageFallback: React.FC = () => (
  <div className="flex items-center justify-center py-24">
    <Spinner />
  </div>
);

/** Suspense fallback for full-screen public routes (login, register, docs, legal). */
export const FullPageFallback: React.FC = () => (
  <div className="min-h-screen bg-zinc-950 flex items-center justify-center">
    <Spinner />
  </div>
);

const CHUNK_ERROR_RE =
  /dynamically imported module|Importing a module script failed|error loading dynamically imported module|Unable to preload CSS|ChunkLoadError/i;

interface RouteErrorBoundaryProps {
  children: React.ReactNode;
  fullScreen?: boolean;
}

interface RouteErrorBoundaryState {
  error: Error | null;
}

/**
 * Catches render errors and failed lazy-chunk loads (e.g. a tab left open across a deploy, whose
 * hashed chunks no longer exist) so they don't blank the whole app. Key it by route to reset on
 * navigation.
 */
export class RouteErrorBoundary extends React.Component<RouteErrorBoundaryProps, RouteErrorBoundaryState> {
  state: RouteErrorBoundaryState = { error: null };

  static getDerivedStateFromError(error: Error): RouteErrorBoundaryState {
    return { error };
  }

  componentDidCatch(error: Error, info: React.ErrorInfo) {
    console.error('Route failed to render:', error, info.componentStack);
  }

  render() {
    const { error } = this.state;
    if (!error) return this.props.children;

    const chunkFailure = CHUNK_ERROR_RE.test(error.message || '');
    const content = (
      <div className="max-w-md mx-auto text-center space-y-3 py-16 px-4">
        <AlertTriangle className="w-6 h-6 text-amber-400 mx-auto" />
        <h3 className="text-sm font-semibold text-white">
          {chunkFailure ? 'This page could not be loaded' : 'Something went wrong on this page'}
        </h3>
        <p className="text-xs text-zinc-400 leading-relaxed">
          {chunkFailure
            ? 'Nevatal Cloud was probably updated since this tab was opened. Reload to fetch the latest version.'
            : 'An unexpected error occurred while rendering. Reloading usually helps.'}
        </p>
        <p className="text-[11px] font-mono text-zinc-500 break-words">{error.message}</p>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-md bg-white text-zinc-950 text-xs font-medium hover:bg-zinc-200 transition-colors"
        >
          <RefreshCw className="w-3.5 h-3.5" />
          <span>Reload</span>
        </button>
      </div>
    );

    return this.props.fullScreen ? (
      <div className="min-h-screen bg-zinc-950 flex items-center justify-center">{content}</div>
    ) : (
      content
    );
  }
}
