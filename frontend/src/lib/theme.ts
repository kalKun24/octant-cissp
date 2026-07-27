import { createContext } from 'react';

/** 利用者が選べるテーマ。`system` は OS の設定（prefers-color-scheme）に従います。 */
export type Theme = 'light' | 'dark' | 'system';

/** 実際に画面へ適用されるテーマ。`system` は解決済みでここには現れません。 */
export type ResolvedTheme = 'light' | 'dark';

/** localStorage のキー。index.html の初期化スクリプトと同じ値にすること。 */
export const THEME_STORAGE_KEY = 'octant-theme';

export const DARK_MEDIA_QUERY = '(prefers-color-scheme: dark)';

export interface ThemeContextValue {
  /** 保存されている選択（`system` を含む）。 */
  theme: Theme;
  /** 実際に適用されているテーマ。 */
  resolvedTheme: ResolvedTheme;
  setTheme: (theme: Theme) => void;
  /** ライトとダークを入れ替えます。 */
  toggleTheme: () => void;
}

export const ThemeContext = createContext<ThemeContextValue | null>(null);

function isTheme(value: string | null): value is Theme {
  return value === 'light' || value === 'dark' || value === 'system';
}

/** localStorage から選択を読みます。未保存・利用不可なら `system` を返します。 */
export function readStoredTheme(): Theme {
  try {
    const stored = localStorage.getItem(THEME_STORAGE_KEY);
    return isTheme(stored) ? stored : 'system';
  } catch {
    return 'system';
  }
}

/** 選択を localStorage に保存します。保存できない環境では黙って諦めます。 */
export function persistTheme(theme: Theme): void {
  try {
    localStorage.setItem(THEME_STORAGE_KEY, theme);
  } catch {
    // プライベートモード等で保存できない場合は、そのセッション内だけ有効にします。
  }
}

/** OS がダークテーマを要求しているかを返します（useSyncExternalStore のスナップショット）。 */
export function getSystemPrefersDark(): boolean {
  return window.matchMedia(DARK_MEDIA_QUERY).matches;
}

/** OS のテーマ変更を購読します（useSyncExternalStore の購読関数）。 */
export function subscribeToSystemTheme(onChange: () => void): () => void {
  const mediaQuery = window.matchMedia(DARK_MEDIA_QUERY);
  mediaQuery.addEventListener('change', onChange);
  return () => {
    mediaQuery.removeEventListener('change', onChange);
  };
}

/** OS の設定から実際のテーマを求めます。 */
export function resolveTheme(theme: Theme): ResolvedTheme {
  if (theme !== 'system') {
    return theme;
  }
  return getSystemPrefersDark() ? 'dark' : 'light';
}

/** <html> にテーマを反映します。ネイティブUI（スクロールバー等）も追随させます。 */
export function applyTheme(resolved: ResolvedTheme): void {
  const root = document.documentElement;
  root.classList.toggle('dark', resolved === 'dark');
  root.style.colorScheme = resolved;
}
