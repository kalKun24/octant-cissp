import {
  useCallback,
  useEffect,
  useMemo,
  useState,
  useSyncExternalStore,
  type ReactNode,
} from 'react';

import {
  applyTheme,
  persistTheme,
  readStoredTheme,
  subscribeToSystemTheme,
  getSystemPrefersDark,
  ThemeContext,
  type Theme,
} from '@/lib/theme';

interface ThemeProviderProps {
  children: ReactNode;
}

/**
 * テーマの状態を保持し、<html> のクラスへ反映します。
 * 初期値は localStorage の保存値、無ければ OS の prefers-color-scheme に従います。
 */
export function ThemeProvider({ children }: ThemeProviderProps) {
  const [theme, setTheme] = useState<Theme>(readStoredTheme);

  // OS の設定は外部ストアとして購読します（`system` のあいだは切り替えに追随します）。
  const systemPrefersDark = useSyncExternalStore(
    subscribeToSystemTheme,
    getSystemPrefersDark,
    () => false,
  );

  // 実際に適用するテーマは state を増やさず、描画時に導出します。
  const resolvedTheme = theme === 'system' ? (systemPrefersDark ? 'dark' : 'light') : theme;

  // DOM は React の外側にあるので、反映は副作用として行います。
  useEffect(() => {
    applyTheme(resolvedTheme);
  }, [resolvedTheme]);

  useEffect(() => {
    persistTheme(theme);
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme(resolvedTheme === 'dark' ? 'light' : 'dark');
  }, [resolvedTheme]);

  const value = useMemo(
    () => ({ theme, resolvedTheme, setTheme, toggleTheme }),
    [theme, resolvedTheme, toggleTheme],
  );

  return <ThemeContext value={value}>{children}</ThemeContext>;
}
