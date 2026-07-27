import { useContext } from 'react';

import { ThemeContext, type ThemeContextValue } from '@/lib/theme';

/** テーマの取得と変更を行います。ThemeProvider の内側でのみ使えます。 */
export function useTheme(): ThemeContextValue {
  const context = useContext(ThemeContext);
  if (context === null) {
    throw new Error('useTheme は ThemeProvider の内側で使用してください。');
  }
  return context;
}
