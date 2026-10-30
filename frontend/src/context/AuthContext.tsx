import React, { createContext, useContext, useState, useEffect } from 'react';
import { User } from '../types';
import { apiRequest } from '../lib/api';

interface AuthContextType {
  user: User | null;
  token: string | null;
  loading: boolean;
  login: (email: string, pass: string) => Promise<void>;
  register: (email: string, pass: string, name: string) => Promise<void>;
  loginWithOAuth: (provider: 'google' | 'github' | 'wordpress', payload?: any) => Promise<void>;
  logout: () => void;
  switchDemoUser: (role: 'admin' | 'customer') => Promise<void>;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [token, setToken] = useState<string | null>(() => localStorage.getItem('nevatal_token'));
  const [loading, setLoading] = useState(true);

  const refreshUser = async () => {
    try {
      if (!localStorage.getItem('nevatal_token')) {
        setUser(null);
        setLoading(false);
        return;
      }
      const data = await apiRequest<User>('/auth/me');
      setUser(data);
    } catch (err) {
      console.warn('Failed to fetch user with existing token, resetting session.', err);
      localStorage.removeItem('nevatal_token');
      setUser(null);
      setToken(null);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    refreshUser();
  }, []);

  const login = async (email: string, pass: string) => {
    const res = await apiRequest<{ user: User; token: string }>('/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password: pass }),
    });

    localStorage.setItem('nevatal_token', res.token);
    setToken(res.token);
    setUser(res.user);
  };

  const register = async (email: string, pass: string, name: string) => {
    const res = await apiRequest<{ user: User; token: string }>('/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password: pass, display_name: name }),
    });

    localStorage.setItem('nevatal_token', res.token);
    setToken(res.token);
    setUser(res.user);
  };

  const loginWithOAuth = async (provider: 'google' | 'github' | 'wordpress', payload?: any) => {
    const res = await apiRequest<{ user: User; token: string }>(`/auth/oauth/${provider}`, {
      method: 'POST',
      body: JSON.stringify(payload || {}),
    });

    localStorage.setItem('nevatal_token', res.token);
    setToken(res.token);
    setUser(res.user);
  };

  const logout = () => {
    apiRequest('/auth/logout', { method: 'POST' }).catch(() => {});
    localStorage.removeItem('nevatal_token');
    setToken(null);
    setUser(null);
  };

  const switchDemoUser = async (role: 'admin' | 'customer') => {
    if (role === 'admin') {
      await login('admin@nevatal.id', 'AdminPass123!');
    } else {
      await login('demo@nevatal.id', 'DemoPass123!');
    }
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        loading,
        login,
        register,
        loginWithOAuth,
        logout,
        switchDemoUser,
        refreshUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};
