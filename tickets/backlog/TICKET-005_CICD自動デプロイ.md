# TICKET-005: CI/CD（WIF・自動デプロイ・Hosting rewrites）

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-005 |
| ステータス | 🔴 未着手 |
| 作成日 | 2026-07-27 |
| 着手日 | - |
| 完了日 | - |
| ブランチ名 | - |
| PR番号 | - |
| PRリンク | - |

## 概要

GitHub Actions から Workload Identity Federation でキーレス認証し、
`develop` → dev 環境、`main` → prod 環境へ自動デプロイする。
Firebase Hosting の `/api/**` rewrites で Cloud Run にプロキシする構成を確定させる。

> **前提条件**: **TICKET-016（GCP ブートストラップ）→ TICKET-004（Terraform）の順で
> 完了していること。** GCP プロジェクトが無ければ WIF の作成先が無く、
> Cloud Run サービスや Artifact Registry が無い状態で自動デプロイが走れば失敗する。
>
> ```
> 016（ブートストラップ）→ 004（Terraform）→ 005（このチケット）
> ```
>
> **鶏と卵に注意**: WIF を作るのは Terraform だが、その Terraform を CI から動かすにも
> WIF が要る。**WIF の初回作成は手元の ADC で `apply` する**こと。
>
> **⚠ `main` へマージした瞬間に prod デプロイが走る。**
> GitHub Actions は `push` イベントで「push されたコミット内のワークフローファイル」を使うため、
> このチケットを `main` にマージしたマージコミット自体が prod デプロイを発火させる。
> そのため **このチケットの完了（マージ）は `develop` までに留め、
> `main` へのリリースは TICKET-006 以降が揃ってから別途判断する。**

## 背景・目的

手作業デプロイを禁止する方針（CLAUDE.md）を実行可能にする。
また Hosting の rewrites によって CORS 設定が不要になるため、この構成を先に固めておくと
以降のフロント実装が単純になる。

## 受け入れ条件

- [ ] Workload Identity Federation の設定が Terraform で定義されている
- [ ] **サービスアカウントキーの JSON がリポジトリにも Secrets にも存在しない**
- [ ] `develop` への push で dev 環境（Cloud Run + Hosting）へ自動デプロイされる
- [ ] `main` への push で prod 環境へ自動デプロイされる
- [ ] Cloud Run のリビジョンにコミット SHA がタグ付けされる
- [ ] `firebase/firebase.json` の rewrites で `/api/**` が Cloud Run に転送される
- [ ] ブラウザから同一オリジンで API を呼べる（CORS 設定が不要であることを確認）
- [ ] `firebase/firestore.rules` が全ドキュメント拒否の状態でデプロイされる
- [ ] デプロイ前に lint とテストが実行され、失敗したらデプロイされない

## サブチケット（コミット計画）

- [ ] `feat(infra): Workload Identity Federation の定義を追加`
- [ ] `feat(repo): firebase.json と /api rewrites を追加`
- [ ] `feat(repo): firestore.rules（全拒否）と indexes.json を追加`
- [ ] `ci(repo): develop への push で dev へデプロイするワークフローを追加`
- [ ] `ci(repo): main への push で prod へデプロイするワークフローを追加`

## 関連情報

- CLAUDE.md「インフラ・デプロイ」「全体構成」
- **`firestore.rules` は全拒否のままにする。** ブラウザは Firestore に直接アクセスしない設計であり、
  ここに `allow` を書き足すことは設計違反
- 手で `gcloud run deploy` を打たない
