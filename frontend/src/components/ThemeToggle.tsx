import { MoonIcon, SunIcon } from 'lucide-react';

import { Button } from '@/components/ui/button';
import { useTheme } from '@/hooks/useTheme';

/** ライトテーマとダークテーマを切り替えるボタン。選択は localStorage に保存します。 */
export function ThemeToggle() {
  const { resolvedTheme, toggleTheme } = useTheme();
  const isDark = resolvedTheme === 'dark';
  const label = isDark ? 'ライトテーマに切り替えます' : 'ダークテーマに切り替えます';

  return (
    <Button
      type="button"
      variant="ghost"
      size="icon"
      // モバイルの最小タップ領域（44px）を確保します。
      className="size-tap"
      onClick={toggleTheme}
      aria-label={label}
      title={label}
    >
      {isDark ? <SunIcon aria-hidden="true" /> : <MoonIcon aria-hidden="true" />}
    </Button>
  );
}
