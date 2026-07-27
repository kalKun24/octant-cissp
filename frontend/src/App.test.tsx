import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';

import { App } from '@/App';

describe('App の配線', () => {
  it('ルーティングとテーマの配線が通り、トップページが表示されます', () => {
    render(<App />);

    expect(screen.getByRole('heading', { level: 1, name: 'Octant' })).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'ダークテーマに切り替えます' })).toBeInTheDocument();
  });

  it('ヘッダのトグルでアプリ全体のテーマが切り替わります', async () => {
    const user = userEvent.setup();
    render(<App />);

    expect(screen.getByText('light')).toBeInTheDocument();

    await user.click(screen.getByRole('button', { name: 'ダークテーマに切り替えます' }));

    expect(document.documentElement).toHaveClass('dark');
    expect(screen.getByText('dark')).toBeInTheDocument();
  });

  it('未知のパスでは見つかりませんの画面を表示します', () => {
    window.history.pushState({}, '', '/存在しないパス');
    render(<App />);

    expect(
      screen.getByRole('heading', { level: 1, name: 'ページが見つかりません' }),
    ).toBeInTheDocument();

    window.history.pushState({}, '', '/');
  });
});
