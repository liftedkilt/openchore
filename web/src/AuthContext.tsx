import React, { createContext, useContext, useState, useEffect, useCallback } from 'react';
import { api, SESSION_EXPIRED_EVENT } from './api';
import type { AuthSession, SessionInfo, User } from './types';

interface AuthContextType {
  user: User | null;
  session: SessionInfo | null;
  isLoading: boolean;
  isAdmin: boolean;
  // Adopt a session the server just issued (login, setup).
  signIn: (auth: AuthSession) => void;
  // Update the signed-in user's profile after an edit (theme, avatar, PIN...).
  setUser: (user: User) => void;
  // Re-read the session from the server (e.g. after returning from an OIDC
  // provider).
  refresh: () => Promise<void>;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// The session itself lives in an HttpOnly cookie; this context only mirrors
// what the server reports from /api/auth/me.
export const AuthProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [user, setUserState] = useState<User | null>(null);
  const [session, setSession] = useState<SessionInfo | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  const refresh = useCallback(async () => {
    try {
      const me = await api.auth.me();
      setUserState(me.user);
      setSession(me.session);
    } catch {
      setUserState(null);
      setSession(null);
    }
  }, []);

  useEffect(() => {
    // Profiles used to be remembered client-side; that key is meaningless now.
    try { localStorage.removeItem('openchore_user'); } catch { /* ignore */ }
    refresh().finally(() => setIsLoading(false));
  }, [refresh]);

  useEffect(() => {
    const onExpired = () => {
      setUserState(null);
      setSession(null);
    };
    window.addEventListener(SESSION_EXPIRED_EVENT, onExpired);
    return () => window.removeEventListener(SESSION_EXPIRED_EVENT, onExpired);
  }, []);

  const signIn = useCallback((auth: AuthSession) => {
    setUserState(auth.user);
    setSession(auth.session);
  }, []);

  const setUser = useCallback((u: User) => setUserState(u), []);

  const signOut = useCallback(async () => {
    try {
      await api.auth.logout();
    } catch {
      // The cookie is cleared server-side; nothing else to do.
    }
    setUserState(null);
    setSession(null);
  }, []);

  return (
    <AuthContext.Provider value={{
      user, session, isLoading, isAdmin: user?.role === 'admin',
      signIn, setUser, refresh, signOut,
    }}>
      {children}
    </AuthContext.Provider>
  );
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) throw new Error('useAuth must be used within AuthProvider');
  return context;
};
