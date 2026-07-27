import { Link } from 'react-router';

export function NotFoundPage() {
  return (
    <section>
      <h1>ページが見つかりません</h1>
      <p>URL が変更されたか、削除された可能性があります。ホームから目的のノートを探してください。</p>
      <Link to="/">ホームへ戻ります</Link>
    </section>
  );
}
