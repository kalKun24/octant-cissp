import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';

import { App } from '@/App';

describe('App の配線', () => {
  it('ルーティングと Query の配線が通り、トップページが表示されます', () => {
    render(<App />);

    expect(screen.getByRole('heading', { level: 1, name: 'Octant' })).toBeInTheDocument();
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
