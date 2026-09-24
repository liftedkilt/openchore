import React, { createContext, useCallback, useContext, useMemo } from 'react';
import { THEME_CONFIG } from './types';
import type { ThemeConfig } from './types';
import { useAuth } from './AuthContext';
import { api } from './api';
import { isPersonColor, resolveSkin, type PersonColor, type Skin } from './design';

// The signed-in person's skin and colour. Screens that belong to one person
// render inside <SkinScope skin={skin} color={color}>; nothing here touches
// <body> any more, so shared screens (picker, admin, ambient) stay in House.
interface ThemeContextType {
  /** The person's skin: their stored theme, or by age when unset. */
  skin: Skin;
  /** The person's colour key, or null when they have none. */
  color: PersonColor | null;
  setSkin: (skin: Skin) => Promise<void>;
  setColor: (color: PersonColor) => Promise<void>;
  /** Per-skin feedback: sounds and vibration. */
  config: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export const ThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { user, setUser } = useAuth();
  const skin = resolveSkin(user?.theme, user?.age);
  const color = isPersonColor(user?.color) ? user.color : null;

  const setSkin = useCallback(async (next: Skin) => {
    if (!user) return;
    const previous = user;
    // Optimistic: the screen re-skins at once, and rolls back on failure.
    setUser({ ...user, theme: next });
    try {
      const updated = await api.users.updateTheme(user.id, next);
      setUser({ ...previous, ...updated });
    } catch (e) {
      console.error('Failed to save skin:', e);
      setUser(previous);
      throw e;
    }
  }, [user, setUser]);

  const setColor = useCallback(async (next: PersonColor) => {
    if (!user) return;
    const previous = user;
    setUser({ ...user, color: next });
    try {
      const updated = await api.users.updateColor(user.id, next);
      setUser({ ...previous, ...updated });
    } catch (e) {
      console.error('Failed to save colour:', e);
      setUser(previous);
      throw e;
    }
  }, [user, setUser]);

  const value = useMemo<ThemeContextType>(() => ({
    skin, color, setSkin, setColor, config: THEME_CONFIG[skin] ?? THEME_CONFIG.sunroom,
  }), [skin, color, setSkin, setColor]);

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
};

export const useTheme = () => {
  const context = useContext(ThemeContext);
  if (!context) throw new Error('useTheme must be used within ThemeProvider');
  return context;
};
