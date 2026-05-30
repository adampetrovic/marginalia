import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { api, ApiError } from '@/api/client';
import type { AuthResponse, User } from '@/api/types';

interface AuthContextValue {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<User>;
  register: (email: string, password: string, name: string) => Promise<User>;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const queryClient = useQueryClient();

  const loadMe = useCallback(async () => {
    try {
      const res = await api.get<AuthResponse>('/auth/me');
      setUser(res.user);
    } catch (err) {
      // 401 (or any failure) => treat as logged out.
      if (err instanceof ApiError && err.status !== 401) {
        // Non-auth error: still consider the user unauthenticated but don't crash.
        console.warn('Failed to load current user:', err.message);
      }
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    loadMe();
  }, [loadMe]);

  const login = useCallback(
    async (email: string, password: string) => {
      const res = await api.post<AuthResponse>('/auth/login', {
        email,
        password,
      });
      setUser(res.user);
      queryClient.clear();
      return res.user;
    },
    [queryClient],
  );

  const register = useCallback(
    async (email: string, password: string, name: string) => {
      const res = await api.post<AuthResponse>('/auth/register', {
        email,
        password,
        name,
      });
      setUser(res.user);
      queryClient.clear();
      return res.user;
    },
    [queryClient],
  );

  const logout = useCallback(async () => {
    try {
      await api.post<void>('/auth/logout');
    } catch {
      // ignore — clear local state regardless
    }
    setUser(null);
    queryClient.clear();
  }, [queryClient]);

  const value = useMemo(
    () => ({ user, loading, login, register, logout }),
    [user, loading, login, register, logout],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return ctx;
}
