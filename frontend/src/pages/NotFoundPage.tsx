import { Link } from 'react-router';

export function NotFoundPage() {
  return (
    <section className="space-y-4">
      <h1 className="text-2xl font-bold tracking-tight">ページが見つかりません</h1>
      <p className="text-muted-foreground leading-relaxed">
        URL が変更されたか、削除された可能性があります。ホームから目的のノートを探してください。
      </p>
      <Link
        to="/"
        className="text-primary min-h-tap inline-flex items-center rounded-md px-2 font-medium underline underline-offset-4"
      >
        ホームへ戻ります
      </Link>
    </section>
  );
}
