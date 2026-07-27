# TICKET-001: リポジトリ基盤（Makefile・Go/Vite雛形・lint・CI骨子）

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-001 |
| ステータス | 🟢 完了 |
| 作成日 | 2026-07-27 |
| 着手日 | 2026-07-27 |
| 完了日 | 2026-07-27 |
| ブランチ名 | `feature/TICKET-001` |
| PR番号 | #3 |
| PRリンク | https://github.com/kalKun24/octant-cissp/pull/3 |

## 概要

空のディレクトリだけがある状態から、`make` コマンドが一通り動く土台を作る。
Go のモジュール初期化、Vite プロジェクトの作成、Makefile、lint 設定、GitHub Actions の CI 骨子まで。

## 背景・目的

以降のすべてのチケットが `make lint` / `make test` を通ることを受け入れ条件に持つため、
最初にこれらが存在しなければならない。CLAUDE.md「コマンド」の一覧を実体化する。

## 受け入れ条件

- [x] `make setup` で Go・npm の依存とツール（golangci-lint・oapi-codegen）が導入される
- [x] `make lint` が golangci-lint・eslint・`tsc --noEmit` を実行し、警告 0 件で終了する
- [x] `make test` が `go test ./...` を実行して成功する（テストが 0 件でもよい）
- [x] `make test-front` が Vitest を実行して成功する
- [x] `make fmt` が gofmt と prettier を実行する
- [x] `make build` が フロントのビルドと API のコンテナビルドを実行して成功する
- [x] `cmd/server/main.go` が起動し、`GET /health` が 200 を返す（認証不要）
- [x] PR で GitHub Actions が `make lint` / `make test` / `make test-front` を実行する
- [x] `frontend/` に Tailwind CSS と shadcn/ui が導入され、ライト/ダークの切替が機能する
- [x] 雛形の残骸 `backend/internal/usecase/reservation/` が削除されている

## サブチケット（コミット計画）

- [x] `chore(backend): Go モジュールと chi ルータの雛形を追加`
- [x] `chore(frontend): Vite + React 19 + TypeScript の雛形を追加`
- [x] `chore(frontend): Tailwind CSS と shadcn/ui を導入`
- [x] `chore(backend): /health エンドポイントと slog の構造化ログを追加`
- [x] `chore(backend): Dockerfile を追加`
- [x] `chore(repo): Makefile を追加`
- [x] `chore(repo): golangci-lint と eslint の設定を追加`
- [x] `ci(repo): PR で lint とテストを実行するワークフローを追加`
- [x] `chore(backend): 雛形の残骸 usecase/reservation を削除`

## 関連情報

- CLAUDE.md「コマンド」「ディレクトリ」「アーキテクチャ規約（Go）」
- ログのフィールド構成は `trace_id` / `uid` / `path` / `latency_ms`
- **`interface/repository/` は作らない**（インターフェースは `domain/repository/`）

## 完了時メモ

QA Team の判定は **CONDITIONAL**（🔴 / Critical / High は0件）。指摘のうち
`make fmt` の欠陥・ログ重大度・規約の実態反映を `6956e7a` で修正した。

`usecase/reservation/` は雛形に実在したが**空ディレクトリのため git 追跡外**で、
作業ツリーから削除した。対応するコミットは原理的に作れない。

### TICKET-002 の着手前に決めるべき方針（全エンドポイントに波及）

- 405 に `Allow` ヘッダを付けるか（RFC 9110 §15.5.6 は MUST。
  `r.MethodNotAllowed()` の上書きで chi 既定の `Allow` 組み立てが失われている）
- `HEAD /health` を 200 にするか（現状 405。chi の `r.Get` は HEAD を自動登録しない）
- 末尾スラッシュ `/health/` の扱い（現状 404。`StripSlashes` / `RedirectSlashes` 未使用）

### 後続チケットへの申し送り

- **TICKET-002**: CI に `make gen` の差分検証を追加する。
  フロントの fetch クライアントは 431 / 505 など**本文が JSON でない error 応答**を想定する
- **TICKET-003**: `X-Request-Id` の無検証採用を修正（詐称可能）。
  `APP_ENV` を `local|dev|prod` の列挙値として fail-fast 検証（**設定漏れが
  認証バイパス側に倒れる構造を防ぐ**）。`logging.go` に `uid` を追加
- **TICKET-004**: `firestore.rules` の deny-all を Firestore プロジェクト作成と
  **同一変更でデプロイする**（順序を誤ると一時的に全公開の窓が開く）
- **TICKET-005**: `X-Cloud-Trace-Context` → `logging.googleapis.com/trace` のマップ。
  Action の SHA ピン。`go.mod` を 1.26.5 以上に上げて GO-2026-5856 を解消。
  CI に `govulncheck` を追加
- **TICKET-006 / 012**: `http.MaxBytesReader` による共通ボディサイズ上限（超過時 413 を封筒形式で）
- **TICKET-008**: `queryClient.ts` の `staleTime` / `retry: 1` は先行導入のため、
  共通エラーハンドリング方針とあわせて再設計する
