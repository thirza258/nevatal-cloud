import React, { useState } from 'react';
import { useAuth } from '../context/AuthContext';
import { useSystemConfig } from '../context/SystemConfigContext';
import { Lock, Mail, ArrowRight, Eye, EyeOff, CheckCircle2, Zap, ShieldCheck } from 'lucide-react';

interface RegisterProps {
  onNavigate: (path: string) => void;
}

export const Register: React.FC<RegisterProps> = ({ onNavigate }) => {
  const { register, loginWithOAuth } = useAuth();
  const { config } = useSystemConfig();
  const showSocialLogin = config.auth?.social_login;

  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [showPassword, setShowPassword] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    const trimmedEmail = email.trim().toLowerCase();
    if (!trimmedEmail || !trimmedEmail.includes('@')) {
      setError('Please enter a valid email address');
      return;
    }

    if (password.length < 8) {
      setError('Password must be at least 8 characters long');
      return;
    }

    setLoading(true);
    try {
      // Direct registration with only email and password - no email verification needed!
      await register(trimmedEmail, password);
      onNavigate('/dashboard');
    } catch (err: any) {
      setError(err.message || 'Registration failed. Please check your credentials and try again.');
    } finally {
      setLoading(false);
    }
  };

  const handleInstantSandbox = async () => {
    setError(null);
    setLoading(true);
    try {
      const rand = Math.random().toString(36).substring(2, 8);
      const testEmail = `user_${rand}@cloud.nevatal.id`;
      const testPass = `Nevatal_${rand}2026!`;
      await register(testEmail, testPass, `User ${rand}`);
      onNavigate('/dashboard');
    } catch (err: any) {
      setError(err.message || 'Quick signup failed. Please try manual registration.');
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
        <div className="inline-flex items-center justify-center w-10 h-10 rounded-lg bg-zinc-100 text-zinc-950 font-bold text-sm tracking-tight mb-3 shadow-md">
          NV
        </div>
        <h1 className="text-2xl font-bold text-white tracking-tight">Create your Account</h1>
        <p className="mt-1 text-xs text-zinc-400 font-mono">
          Instant Cloud Infrastructure · No Email Verification Required
        </p>

        {/* Feature badge */}
        <div className="mt-3 inline-flex items-center gap-1.5 px-3 py-1 rounded-full bg-emerald-950/60 border border-emerald-800/60 text-emerald-300 text-[11px] font-medium">
          <CheckCircle2 className="w-3.5 h-3.5 text-emerald-400" />
          <span>Instant activation: 4 vCPU · 8 GB RAM · 100 GB NVMe included</span>
        </div>
      </div>

      <div className="mt-6 sm:mx-auto sm:w-full sm:max-w-md">
        <div className="bg-zinc-950 border border-zinc-800 py-6 px-5 rounded-2xl sm:px-8 space-y-5 shadow-2xl">
          {error && (
            <div className="p-3 rounded-xl bg-rose-500/10 border border-rose-500/20 text-rose-300 text-xs flex items-start gap-2">
              <span className="shrink-0 text-rose-400 font-bold">✕</span>
              <span>{error}</span>
            </div>
          )}

          {/* Social Sign-in Options (only when enabled by backend) */}
          {showSocialLogin && (
            <>
              <div className="space-y-2">
                <button
                  type="button"
                  disabled={loading}
                  onClick={() => handleOAuthLogin('google')}
                  className="w-full flex items-center justify-center gap-2.5 py-2 px-3 rounded-xl bg-zinc-900 hover:bg-zinc-800 text-xs font-medium text-white border border-zinc-800 transition disabled:opacity-60 cursor-pointer"
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
              </div>

              <div className="relative flex py-1 items-center">
                <div className="flex-grow border-t border-zinc-800"></div>
                <span className="flex-shrink mx-3 text-[10px] uppercase font-mono tracking-wider text-zinc-500">
                  Or enter email
                </span>
                <div className="flex-grow border-t border-zinc-800"></div>
              </div>
            </>
          )}

          {/* Simple 2-Field Registration: ONLY Email & Password */}
          <form className="space-y-4" onSubmit={handleSubmit}>
            <div>
              <label className="block text-xs font-medium text-zinc-300 mb-1">
                Email Address
              </label>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-zinc-500">
                  <Mail className="h-4 w-4" />
                </div>
                <input
                  type="email"
                  required
                  autoFocus
                  autoComplete="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  placeholder="name@company.com"
                  className="block w-full pl-9 pr-3 py-2 bg-zinc-900 border border-zinc-800 rounded-xl text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-indigo-500 transition"
                />
              </div>
            </div>

            <div>
              <div className="flex justify-between items-center mb-1">
                <label className="block text-xs font-medium text-zinc-300">
                  Password
                </label>
                <span className={`text-[10px] font-mono transition-colors ${password.length >= 8 ? 'text-emerald-400 font-semibold' : 'text-zinc-500'}`}>
                  {password.length >= 8 ? '✓ Minimum 8 chars met' : 'Min. 8 characters'}
                </span>
              </div>
              <div className="relative">
                <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-zinc-500">
                  <Lock className="h-4 w-4" />
                </div>
                <input
                  type={showPassword ? 'text' : 'password'}
                  required
                  autoComplete="new-password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  placeholder="••••••••"
                  className="block w-full pl-9 pr-10 py-2 bg-zinc-900 border border-zinc-800 rounded-xl text-xs text-white placeholder-zinc-500 focus:outline-none focus:border-indigo-500 transition"
                />
                <button
                  type="button"
                  tabIndex={-1}
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute inset-y-0 right-0 pr-3 flex items-center text-zinc-500 hover:text-zinc-300"
                >
                  {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </button>
              </div>
            </div>

            <button
              type="submit"
              disabled={loading}
              className="w-full flex justify-center items-center gap-2 py-2.5 px-4 rounded-xl text-xs font-semibold text-zinc-950 bg-white hover:bg-zinc-200 transition-colors disabled:opacity-60 cursor-pointer shadow-lg mt-2"
            >
              {loading ? 'Creating Account & Logging In...' : 'Create Account & Start'}
              <ArrowRight className="w-3.5 h-3.5" />
            </button>
          </form>

          {/* Quick 1-Click Instant Sandbox Option */}
          <div className="pt-3 border-t border-zinc-900">
            <button
              type="button"
              disabled={loading}
              onClick={handleInstantSandbox}
              className="w-full flex items-center justify-center gap-2 py-2 px-3 rounded-xl bg-zinc-900/80 hover:bg-zinc-900 text-xs text-zinc-300 hover:text-white border border-zinc-800/80 hover:border-zinc-700 transition disabled:opacity-60 cursor-pointer"
            >
              <Zap className="w-3.5 h-3.5 text-amber-400" />
              <span>1-Click Quick Start (Instant Test Account)</span>
            </button>
          </div>

          <div className="text-center pt-2 space-y-3">
            <div>
              <button
                onClick={() => onNavigate('/login')}
                className="text-xs text-zinc-400 hover:text-white transition-colors"
              >
                Already have an account? <span className="underline font-semibold text-white">Sign In</span>
              </button>
            </div>
            <div className="flex items-center justify-center gap-3 text-[11px] text-zinc-500 font-mono">
              <button onClick={() => onNavigate('/')} className="hover:text-zinc-300 transition-colors">
                ← Home
              </button>
              <span>·</span>
              <button onClick={() => onNavigate('/docs')} className="hover:text-zinc-300 transition-colors">
                Documentation
              </button>
              <span>·</span>
              <span className="text-emerald-500">No Credit Card</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
