import React, { useState } from 'react';
import { useAuth } from '../context/AuthContext';
import { Zap, Lock, Mail, User, ArrowRight } from 'lucide-react';

interface RegisterProps {
  onNavigate: (path: string) => void;
}

export const Register: React.FC<RegisterProps> = ({ onNavigate }) => {
  const { register, loginWithOAuth } = useAuth();
  const [displayName, setDisplayName] = useState('');
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    if (password.length < 8) {
      setError('Password must be at least 8 characters long');
      return;
    }
    setLoading(true);
    try {
      await register(email, password, displayName);
      onNavigate('/dashboard');
    } catch (err: any) {
      setError(err.message || 'Registration failed');
    } finally {
      setLoading(false);
    }
  };

  const handleOAuthLogin = async (provider: 'google' | 'github' | 'wordpress') => {
    setError(null);
    setLoading(true);
    try {
      await loginWithOAuth(provider);
      onNavigate('/dashboard');
    } catch (err: any) {
      setError(err.message || `Failed to sign up with ${provider}`);
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-zinc-950 flex flex-col justify-center py-12 sm:px-6 lg:px-8 selection:bg-zinc-800 selection:text-white">
      <div className="sm:mx-auto sm:w-full sm:max-w-md text-center">
        <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-zinc-100 text-zinc-950 font-bold text-sm tracking-tight mb-3">
          NV
        </div>
        <h2 className="text-2xl font-semibold text-white tracking-tight">Create your Account</h2>
        <p className="mt-1 text-xs text-zinc-400 font-mono">
          Self-Hosted Cloud Infrastructure · Proxmox VE
        </p>
      </div>

      <div className="mt-6 sm:mx-auto sm:w-full sm:max-w-md">
        <div className="bg-zinc-950 border border-zinc-800 py-6 px-5 rounded-xl sm:px-8 space-y-5">
          {error && (
            <div className="p-2.5 rounded-md bg-rose-500/10 border border-rose-500/20 text-rose-400 text-xs flex items-center gap-2">
              <span>{error}</span>
            </div>
          )}

          {/* Social Sign-in Options */}
          <div className="space-y-2">
            <button
              type="button"
              disabled={loading}
              onClick={() => handleOAuthLogin('google')}
              className="w-full flex items-center justify-center gap-2.5 py-1.5 px-3 rounded-md bg-zinc-900 hover:bg-zinc-850 text-xs font-medium text-white border border-zinc-800 transition disabled:opacity-60"
            >
              <svg className="w-3.5 h-3.5" viewBox="0 0 24 24">
                <path
                  fill="#4285F4"
                  d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
                />
                <path
                  fill="#34A853"
                  d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
                />
                <path
                  fill="#FBBC05"
                  d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.06H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.94l2.85-2.22.81-.63z"
                />
                <path
                  fill="#EA4335"
                  d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.06l3.66 2.84c.87-2.6 3.3-4.52 6.16-4.52z"
                />
              </svg>
              <span>Continue with Google</span>
            </button>

            <button
              type="button"
              disabled={loading}
              onClick={() => handleOAuthLogin('github')}
              className="w-full flex items-center justify-center gap-2.5 py-1.5 px-3 rounded-md bg-zinc-900 hover:bg-zinc-850 text-xs font-medium text-white border border-zinc-800 transition disabled:opacity-60"
            >
              <svg className="w-3.5 h-3.5 fill-current text-white" viewBox="0 0 24 24">
                <path fillRule="evenodd" clipRule="evenodd" d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.53 1.032 1.53 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z" />
              </svg>
              <span>Continue with GitHub</span>
            </button>

            <button
              type="button"
              disabled={loading}
              onClick={() => handleOAuthLogin('wordpress')}
              className="w-full flex items-center justify-center gap-2.5 py-1.5 px-3 rounded-md bg-zinc-900 hover:bg-zinc-850 text-xs font-medium text-white border border-zinc-800 transition disabled:opacity-60"
            >
              <svg className="w-3.5 h-3.5 fill-current text-zinc-300" viewBox="0 0 24 24">
                <path d="M12 2C6.477 2 2 6.477 2 12c0 4.237 2.636 7.855 6.356 9.312L4.62 11.23c-.31-.83-.434-1.492-.434-2.035 0-1.897 1.385-3.328 3.328-3.328.124 0 .245.008.367.016L12 2zm8.016 4.09c.65 1.258 1.016 2.688 1.016 4.203 0 2.92-1.383 5.518-3.535 7.184l3.193-8.875c.01-.035.016-.07.016-.105 0-.895-.563-1.637-1.39-1.984l.7-.423zm-7.984 8.785l-2.484 7.22C10.22 22.04 11.1 22.09 12 22.09c.99 0 1.948-.06 2.87-.174l-2.838-7.042zm-5.748-4.48c.414 0 .783.055 1.107.165l2.42 6.942-1.696-4.996c-.302-.893-.524-1.455-.664-1.688-.23-.377-.557-.577-.967-.577-.107 0-.21.014-.308.032l.108.122zM12 0C5.373 0 0 5.373 0 12s5.373 12 12 12 12-5.373 12-12S18.627 0 12 0z" />
              </svg>
              <span>Continue with WordPress</span>
            </button>
          </div>

          <div className="relative flex py-1 items-center">
            <div className="flex-grow border-t border-zinc-800"></div>
            <span className="flex-shrink mx-3 text-[10px] uppercase font-mono tracking-wider text-zinc-500">
              Or email
            </span>
            <div className="flex-grow border-t border-zinc-800"></div>
          </div>

          <form className="space-y-4" onSubmit={handleSubmit}>
            <div>
              <label className="block text-xs font-mono text-zinc-400 mb-1">Full Name</label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none text-zinc-500">
                  <User className="h-3.5 w-3.5" />
                </div>
                <input
                  type="text"
                  required
                  value={displayName}
                  onChange={(e) => setDisplayName(e.target.value)}
                  placeholder="Sarah Connor"
                  className="block w-full pl-8 pr-3 py-1.5 bg-zinc-900 border border-zinc-800 rounded-md text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-700 transition"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-mono text-zinc-400 mb-1">Email Address</label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none text-zinc-500">
                  <Mail className="h-3.5 w-3.5" />
                </div>
                <input
                  type="email"
                  required
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="sarah@example.com"
                  className="block w-full pl-8 pr-3 py-1.5 bg-zinc-900 border border-zinc-800 rounded-md text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-700 transition"
                />
              </div>
            </div>

            <div>
              <label className="block text-xs font-mono text-zinc-400 mb-1">Password (Min. 8 chars)</label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-2.5 flex items-center pointer-events-none text-zinc-500">
                  <Lock className="h-3.5 w-3.5" />
                </div>
                <input
                  type="password"
                  required
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  className="block w-full pl-8 pr-3 py-1.5 bg-zinc-900 border border-zinc-800 rounded-md text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-zinc-700 transition"
                />
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full flex justify-center items-center gap-2 py-2 px-4 rounded-md text-xs font-medium text-zinc-950 bg-white hover:bg-zinc-200 transition-colors disabled:opacity-60 cursor-pointer"
            >
              {loading ? 'Creating Account...' : 'Register'}
              <ArrowRight className="w-3.5 h-3.5" />
            </button>
          </form>

          <div className="text-center pt-2">
            <button
              onClick={() => onNavigate('/login')}
              className="text-xs text-zinc-400 hover:text-white transition-colors"
            >
              Already have an account? <span className="underline text-white">Sign In</span>
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
