import '@testing-library/jest-dom/vitest';

import { cleanup } from '@testing-library/react';
import { afterEach, beforeEach, vi } from 'vitest';

/**
 * jsdom は matchMedia を実装していないため、テストから制御できる形で差し替えます。
 * `setPrefersDark` で OS のダークテーマ設定を模擬できます。
 */
let prefersDark = false;
const listeners = new Set<(event: MediaQueryListEvent) => void>();

export function setPrefersDark(value: boolean): void {
  prefersDark = value;
  const event = { matches: value, media: '(prefers-color-scheme: dark)' } as MediaQueryListEvent;
  for (const listener of listeners) {
    listener(event);
  }
}

function installMatchMedia(): void {
  vi.stubGlobal(
    'matchMedia',
    (query: string): MediaQueryList =>
      ({
        matches: query.includes('prefers-color-scheme: dark') ? prefersDark : false,
        media: query,
        onchange: null,
        addEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
          listeners.add(listener);
        },
        removeEventListener: (_type: string, listener: (event: MediaQueryListEvent) => void) => {
          listeners.delete(listener);
        },
        addListener: () => {},
        removeListener: () => {},
        dispatchEvent: () => false,
      }) as unknown as MediaQueryList,
  );
}

beforeEach(() => {
  prefersDark = false;
  listeners.clear();
  localStorage.clear();
  document.documentElement.className = '';
  document.documentElement.style.colorScheme = '';
  installMatchMedia();
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});
