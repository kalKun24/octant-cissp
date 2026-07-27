import { useTheme } from '@/hooks/useTheme';

export function HomePage() {
  const { resolvedTheme } = useTheme();

  return (
    <section className="space-y-4">
      <h1 className="text-2xl font-bold tracking-tight">Octant</h1>
      <p className="text-muted-foreground leading-relaxed">
        CISSP試験対策のための共有学習ノートアプリです。ノートとドリルの機能はこれから追加します。
      </p>
      <p className="text-muted-foreground text-sm">
        現在のテーマ: <span className="text-foreground font-medium">{resolvedTheme}</span>
      </p>
    </section>
  );
}
