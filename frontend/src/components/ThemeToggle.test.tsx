import { act, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { ThemeProvider } from '@/components/ThemeProvider';
import { ThemeToggle } from '@/components/ThemeToggle';
import { THEME_STORAGE_KEY } from '@/lib/theme';
import { setPrefersDark } from '@/test/setup';

function renderToggle() {
  return render(
    <ThemeProvider>
      <ThemeToggle />
    </ThemeProvider>,
  );
}

const isDarkApplied = () => document.documentElement.classList.contains('dark');

describe('テーマ切替', () => {
  it('保存された設定が無いとき OS のライト設定に従います', () => {
    setPrefersDark(false);
    renderToggle();

    expect(isDarkApplied()).toBe(false);
    expect(screen.getByRole('button', { name: 'ダークテーマに切り替えます' })).toBeInTheDocument();
  });

  it('保存された設定が無いとき OS のダーク設定に従います', () => {
    setPrefersDark(true);
    renderToggle();

    expect(isDarkApplied()).toBe(true);
    expect(document.documentElement.style.colorScheme).toBe('dark');
  });

  it('保存された設定は OS の設定より優先されます', () => {
    setPrefersDark(true);
    localStorage.setItem(THEME_STORAGE_KEY, 'light');
    renderToggle();

    expect(isDarkApplied()).toBe(false);
  });

  it('ボタンを押すとテーマが切り替わり localStorage に保存されます', async () => {
    const user = userEvent.setup();
    setPrefersDark(false);
    renderToggle();

    await user.click(screen.getByRole('button', { name: 'ダークテーマに切り替えます' }));

    expect(isDarkApplied()).toBe(true);
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('dark');

    await user.click(screen.getByRole('button', { name: 'ライトテーマに切り替えます' }));

    expect(isDarkApplied()).toBe(false);
    expect(localStorage.getItem(THEME_STORAGE_KEY)).toBe('light');
  });

  it('キーボードだけでも切り替えられます', async () => {
    const user = userEvent.setup();
    setPrefersDark(false);
    renderToggle();

    await user.tab();
    expect(screen.getByRole('button', { name: 'ダークテーマに切り替えます' })).toHaveFocus();

    await user.keyboard('{Enter}');
    expect(isDarkApplied()).toBe(true);
  });

  it('system のまま OS 設定が変わると追随します', () => {
    setPrefersDark(false);
    renderToggle();
    expect(isDarkApplied()).toBe(false);

    act(() => {
      setPrefersDark(true);
    });
    expect(isDarkApplied()).toBe(true);
  });
});
