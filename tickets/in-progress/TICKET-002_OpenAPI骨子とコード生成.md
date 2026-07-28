# TICKET-002: OpenAPI 骨子とコード生成パイプライン

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-002 |
| ステータス | 🟡 作業中 |
| 作成日 | 2026-07-27 |
| 着手日 | 2026-07-28 |
| 完了日 | - |
| ブランチ名 | `feature/TICKET-002` |
| PR番号 | - |
| PRリンク | - |

## 概要

`api/openapi.yaml` を新規に書き、そこから Go のサーバインターフェースと TypeScript の型を
生成する `make gen` を整備する。共通のレスポンス封筒・エラー・カーソルページネーションの
スキーマをここで確定させる。

## 背景・目的

CLAUDE.md の最重要規約である API First を機械的に強制する土台。
以降のチケットはすべて「openapi.yaml を先に直す」前提で進む。
CI で生成物の差分を検知できないと、この規約は形骸化する。

## 受け入れ条件

- [x] `api/openapi.yaml` が OpenAPI 3.1 として妥当で、`/health` と共通スキーマを定義している
- [x] 共通スキーマとして `Envelope`（`data` / `error`）、`Error`（`code` / `message`）、
      カーソルページネーションのパラメータ（`limit` / `cursor`）とレスポンス（`items` / `next_cursor`）が定義されている
- [x] `make gen` で Go のサーバインターフェース（chi 向け）と TypeScript の型が生成される
- [x] 生成物がコミットされている
- [x] **CI で `make gen` を実行し、差分が出たら失敗する**
- [x] 生成コードに手編集の禁止を示すヘッダコメントが入っている
- [x] `/health` のハンドラが生成インターフェース経由で実装されている
- [x] **405 応答に `Allow` ヘッダが付く**（`POST /health` で `Allow: GET, HEAD` を実測確認）
- [x] **`HEAD /health` が 200 を返す**（ボディなし。GET と同じステータス）
- [x] **末尾スラッシュ（`/health/`）が 404 のまま**（封筒形式）

## サブチケット（コミット計画）

- [x] `feat(api): openapi.yaml の骨子と共通スキーマを追加`
- [x] `chore(repo): oapi-codegen と TS 型生成の設定を追加`
- [x] `chore(repo): make gen を Makefile に追加`
- [x] `fix(backend): 405 に Allow ヘッダを付与し HEAD /health を登録`
- [x] `refactor(backend): /health を生成インターフェース経由の実装に変更`
- [x] `ci(repo): make gen の差分検証をワークフローに追加`
- [x] `fix(backend): API の基底パスを /api にする`（着手後に判明。Hosting の rewrites がパスを剥がさないため）
- [x] `fix(TICKET-002): QAレビュー指摘の修正`

## 関連情報

- CLAUDE.md「API 規約」（レスポンス形式・ページネーション・ステータスコード）
- **オフセットページネーション（`page` / `per_page`）は使わない。** Firestore の `Offset()` は
  読み飛ばした分も課金されるため
- `error` は文字列ではなく `{ code, message }` のオブジェクト

### ルーティングの方針（2026-07-27 決定・全エンドポイントに適用）

TICKET-001 の QA で指摘され、`openapi.yaml` を書く前に確定させた方針。
**個別のエンドポイントごとに判断せず、以下を一律で適用する。**

| 項目 | 方針 |
|---|---|
| 405 の `Allow` ヘッダ | **付ける**（RFC 9110 §15.5.6 は MUST） |
| `HEAD` | **`/health` だけ登録する。** 業務APIは GET のみで、HEAD は 405 |
| 末尾スラッシュ | **404 のまま厳密に。** `StripSlashes` / `RedirectSlashes` は使わない |

補足:

- **`Allow` ヘッダ**: 現在 `main.go` の `r.MethodNotAllowed()` 上書きにより
  chi 既定の `Allow` 組み立てが失われている。許可メソッドをルータから取得して
  自前ハンドラで付与する。**chi 5.3.1 での取得方法は実装時に要確認**
- **`HEAD`**: chi の `r.Get` は HEAD を自動登録しない。全 GET ルートに自動登録すると
  `openapi.yaml` に無いオペレーションが実装に生えて **API First 原則と
  API Tester の契約検証に矛盾する**ため、死活監視の入口だけを例外にする。
  Cloud Run / GCP LB のヘルスチェック自体は GET を使うので必須ではないが、
  外部監視ツールや `curl -I` が HEAD を使うため
- **末尾スラッシュ**: 生成クライアントは常に正規パスを送るため実害がない。
  1リソース1URLを保ち、キャッシュと `openapi.yaml` の一意性を優先する
