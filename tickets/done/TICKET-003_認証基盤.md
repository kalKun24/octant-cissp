# TICKET-003: 認証基盤（Firebase Auth・IDトークン検証・エミュレータ）

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-003 |
| ステータス | 🟢 完了 |
| 作成日 | 2026-07-27 |
| 着手日 | 2026-07-28 |
| 完了日 | 2026-07-29 |
| ブランチ名 | `feature/TICKET-003` |
| PR番号 | #9 |
| PRリンク | https://github.com/kalKun24/octant-cissp/pull/9 |

## 概要

Firebase Auth の ID トークンを検証する認証ミドルウェアを実装し、許可メールのホワイトリストで
アクセスを絞る。あわせて `make dev` に Firestore / Auth の両エミュレータを組み込む。

## 背景・目的

このプロジェクトの防壁は「ID トークンの検証」「許可メールの照合」「uid による個人データ分離」の
3つだけで、すべてこのチケットで作るミドルウェアに集約される。
また、Auth エミュレータが無いとローカルでも CI でも認証付き API を検証できない。

## 受け入れ条件

- [x] 認証ミドルウェアが Firebase Admin SDK で ID トークンを検証し、`uid` と `email` を context に入れる
- [x] `email_verified == false` のトークンを 401 で拒否する
- [x] 環境変数 `ALLOWED_EMAILS`（カンマ区切り）に含まれないメールを **403** で拒否する
- [x] トークン無し・壊れたトークン・期限切れトークンを **401** で拒否する
- [x] `/health` 以外のすべてのエンドポイントが認証ミドルウェアを通る
- [x] `make dev` が Firestore エミュレータ（8808）と **Firebase Auth エミュレータ（9099）** を起動する
- [x] Auth エミュレータの `accounts:signUp` で取得した ID トークンで API を呼べることを統合テストで確認している
- [x] **認証をスキップする環境変数・分岐が実装されていない**
- [x] context から `uid` を取り出すヘルパが `contextkey` 経由で提供されている
- [x] ログにトークン・メールアドレスが出力されない

### TICKET-002 の QA からの申し送り（このチケットで対応する）

- [x] **認証免除は `chi.RouteContext(ctx).RoutePattern()` による allowlist で判定する。**
      **パス文字列の比較にしない。** `main.go` の `routingPath()` は `RawPath` を優先し、
      `logging.go` は `r.URL.Path`（デコード後）を見るため**パスの見え方が食い違う**
      （実測: `GET /api/hea%6Cth` は 404 だがログには `/api/health` と出る）。
      文字列比較にすると、この差が認証バイパスの入口になる
- [x] **allowlist は fail-closed。** 判定に失敗した場合・パターンが取れない場合は
      **認証必須側に倒す**こと。「一致しなければ免除」は禁止
- [x] **`Cache-Control: no-store` と `X-Content-Type-Options: nosniff` を全 API 応答に付ける。**
      認証を載せる前に入れる（TICKET-006 以降で同じ `dto.write` を個人データが通るため）
- [x] **`api/openapi.yaml` に `securitySchemes: bearerAuth` とトップレベル `security` を定義し、
      `/health` だけ `security: []` で opt-out する。** opt-in 運用にすると
      「書き忘れたエンドポイントが無認証」になり、`gen-check` でも検知できない
- [x] `APP_ENV` を `local` / `dev` / `prod` の列挙値として検証し、**不正値なら起動に失敗する**。
      現状は既定値 `local` で無検証のため、エミュレータ接続を `Env` で分岐させると
      **設定漏れが認証バイパス側に倒れる**
- [x] `logging.go` のアクセスログに `uid` を追加する（CLAUDE.md の要求フィールド）
- [x] `X-Request-Id` をクライアントから無検証で採用しない
      （サーバ側生成にするか、長さと文字種を検証する）

## サブチケット（コミット計画）

- [x] `feat(api): openapi.yaml に bearerAuth と security を定義`
- [x] `feat(backend): Firebase Admin SDK の初期化と設定読み込みを追加`
- [x] `feat(backend): IDトークン検証ミドルウェアを追加`
- [x] `feat(backend): 許可メールのホワイトリスト検証を追加`
- [x] `feat(backend): 認証免除の allowlist を RoutePattern 判定で追加`
- [x] `feat(backend): セキュリティヘッダとアクセスログの uid を追加`
- [x] `chore(repo): make dev に Firestore と Auth のエミュレータを追加`
- [x] `test(backend): 認証ミドルウェアの統合テストを追加`

## 関連情報

- CLAUDE.md「認証・認可」「守るべき制約」
- **許可メールをコードにハードコードしない**（環境変数 `ALLOWED_EMAILS`）
- Auth エミュレータのトークンは署名検証が省略されるが、それ以外は本番と同じ経路を通る
- API Tester エージェントがこのエミュレータ経由でトークンを取得する（`.claude/agents/testing-api-tester.md`）

## 完了時メモ

品質チェックを2回実施。**初回は NEEDS WORK**（認可バイパス）、修正後の再判定で
**CONDITIONAL・ブロッカー0件**。必須指摘はすべて解消してマージした。

### 見つかった認可バイパス2件

**1. 前後空白の切り詰め**（初回 QA）
`Allows()` がトークン由来の入力に `TrimSpace` を掛けていたため、
`" dev@example.com "` で作成した**別 uid のアカウント**が許可リストを通過した。
実測: `ALLOWED_EMAILS=dev@example.com` に対し 200（期待は 403）。

**2. ケルビン記号のなりすまし**（実装側が自主的に発見）
`strings.ToLower` は Unicode 対応で **U+212A を `k` に畳む**。
`kal@example.com` の許可リストに対し `Kal@example.com`（K が U+212A）の
別アカウントが一致し得た。見た目がほぼ同じでログでも気づけない。

どちらも `canonicalizeEmail`（切り詰めなし・ASCII 限定・非 ASCII は拒否）で塞いだ。

**Phase 2 で Security Engineer が「前後空白は通らない」と誤報し、API Tester が
正しく検知した。** Reality Checker が矛盾を裁定して API Tester 側を確定させている。
コード読解だけで結論を出すと見落とす類の欠陥だった。

### テストが実装を守っていなかった件（再判定）

既存の「ケルビン記号は k に畳まない」ケースは、**基底が `allowed@example.com` で
`'k'` を含まないため、実装を `strings.ToLower` に戻しても素通り**していた。
変異注入で全9パッケージ ok になることを実測確認し、基底に `'k'` を含む
`TestAllowsRejectsKelvinSignHomoglyph` を追加して同じ変異で FAIL することを確認した。

**「実装が正しい」ことと「テストが守っている」ことは別**という教訓。

### 仕様変更（ユーザー承認済み）

許可リストに**非 ASCII アドレスを設定すると起動失敗**する。Google サインインのみで
アドレスは ASCII、IDN も Punycode で ASCII 表現になるため実害なしと判断した。

### 環境の変化

ローカルに **Java 25（`default-jre-headless`）を導入**（2026-07-29）。
`make dev` が Auth(9099) / Firestore(8808) / API(8080) / Vite(5173) の4つを起動し、
SIGTERM で4つとも停止することを実測確認済み。

### TICKET-006 冒頭で対応すること

- `ALLOWED_EMAILS` の **ASCII 限定仕様**を `.env.example` / `docs/local-auth.md` に記載
- `config.go` の `checkEmulatorUsage` に **`FIRESTORE_EMULATOR_HOST` を追加**
  （Firestore を繋ぐ前に塞ぐ。本番/エミュレータの取り違え防止）
- `main.go` の `parameterErrorHandler` の `err.Error()` をパラメータ名のみに
  （TICKET-006 で検索語がログに載るため）
- `--only auth,firestore` にする際は **CI に `actions/setup-java` が必要**

### 未検証のまま残した項目

- **`-race`**: この環境に gcc が無く実行不可。CI（ubuntu-latest）では動くので別途要検討
- **署名検証**: Auth エミュレータは `alg: none` のため local では実測不能。
  代替として `checkEmulatorUsage` の dev/prod ガードで担保している
