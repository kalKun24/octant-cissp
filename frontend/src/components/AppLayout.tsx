import { Link, Outlet } from 'react-router';

import { ThemeToggle } from '@/components/ThemeToggle';

/** 全画面共通の骨格。ヘッダとコンテンツ領域を持ちます。 */
export function AppLayout() {
  return (
    <div className="bg-background text-foreground flex min-h-dvh flex-col">
      <a
        href="#main"
        className="bg-background focus-visible:ring-ring sr-only rounded-md px-4 py-2 focus-visible:not-sr-only focus-visible:absolute focus-visible:top-2 focus-visible:left-2 focus-visible:z-50 focus-visible:ring-2"
      >
        本文へ移動します
      </a>

      <header className="border-border bg-background/90 sticky top-0 z-40 border-b backdrop-blur">
        <div className="mx-auto flex w-full max-w-3xl items-center justify-between gap-2 px-4 py-2">
          <Link
            to="/"
            className="flex min-h-tap items-center rounded-md px-2 text-lg font-semibold tracking-tight"
          >
            Octant
          </Link>
          <ThemeToggle />
        </div>
      </header>

      <main id="main" className="mx-auto w-full max-w-3xl flex-1 px-4 py-6">
        <Outlet />
      </main>
    </div>
  );
}
