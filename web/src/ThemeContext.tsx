import React, { createContext, useContext, useState, useEffect } from 'react';
import { THEME_CONFIG } from './types';
import type { Theme, ThemeConfig } from './types';
import { useAuth } from './AuthContext';
import { api } from './api';

interface ThemeContextType {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  config: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextType | undefined>(undefined);

export const ThemeProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const { user, setUser } = useAuth();
  const [theme, setThemeState] = useState<Theme>('default');

  useEffect(() => {
    if (user?.theme) {
      setThemeState(user.theme as Theme);
    } else if (!user) {
      setThemeState('default');
    }
  }, [user]);

  useEffect(() => {
    document.body.className = `theme-${theme}`;
  }, [theme]);

  const setTheme = async (newTheme: Theme) => {
    setThemeState(newTheme);
    if (user) {
      try {
        const updated = await api.users.updateTheme(user.id, newTheme);
        setUser({ ...user, ...updated });
      } catch (e) {
        console.error('Failed to save theme:', e);
      }
    }
  };

  const config = THEME_CONFIG[theme] || THEME_CONFIG.default;

  return (
    <ThemeContext.Provider value={{ theme, setTheme, config }}>
      {children}
    </ThemeContext.Provider>
  );
};

export const useTheme = () => {
  const context = useContext(ThemeContext);
  if (!context) throw new Error('useTheme must be used within ThemeProvider');
  return context;
};
