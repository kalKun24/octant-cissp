# TICKET-004: Terraform（dev / prod のインフラ定義）

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-004 |
| ステータス | 🔴 未着手 |
| 作成日 | 2026-07-27 |
| 着手日 | - |
| 完了日 | - |
| ブランチ名 | - |
| PR番号 | - |
| PRリンク | - |

## 概要

GCP のリソースを Terraform で定義する。Cloud Run・Firestore・Cloud Storage・
Artifact Registry・Secret Manager・予算アラート・週次バックアップまで。dev / prod の2環境。

## 背景・目的

手作業で作ったリソースは再現できず、コスト事故の温床になる。
特に **`min_instances = 0`** と **Artifact Registry のクリーンアップポリシー**は
固定費ゼロを維持するための必須設定であり、コードで固定しておく必要がある。

## 受け入れ条件

- [ ] `make tf-plan ENV=dev` と `make tf-plan ENV=prod` がエラーなく差分を出力する
- [ ] tfstate が GCS バックエンド（環境ごとに別バケット、バージョニング有効）に保存される
- [ ] **Cloud Run の `min_instances` が 0** に設定されている
- [ ] **Cloud Run の `max_instances` が dev: 2 / prod: 5** に設定されている
- [ ] **Artifact Registry にクリーンアップポリシー（最新3世代のみ保持）**が設定されている
- [ ] Firestore が PITR 有効・削除保護有効で作成される
- [ ] GCS のバックアップバケットに 90 日のライフサイクルルールがある
- [ ] 予算アラート（月1,000円、50/90/100%）が設定され、通知先メールが変数化されている
- [ ] Cloud Scheduler が週次で Firestore を GCS へエクスポートする
- [ ] Secret Manager に `anthropic-api-key` の入れ物があり、Cloud Run が環境変数として参照する
- [ ] **サービスアカウントキーの JSON を生成する定義が存在しない**
- [ ] `infra/modules/` が再利用単位に分かれ、`environments/dev|prod` は値だけを持つ

## サブチケット（コミット計画）

- [ ] `feat(infra): GCS バックエンドとプロバイダ設定を追加`
- [ ] `feat(infra): firestore モジュールを追加`
- [ ] `feat(infra): artifact-registry モジュール（クリーンアップポリシー付き）を追加`
- [ ] `feat(infra): cloud-run-service モジュールを追加`
- [ ] `feat(infra): storage モジュール（添付・バックアップ）を追加`
- [ ] `feat(infra): 予算アラートと週次バックアップの定義を追加`
- [ ] `feat(infra): dev / prod 環境の値を追加`
- [ ] `chore(repo): make tf-plan / tf-apply を Makefile に追加`

## 関連情報

- CLAUDE.md「インフラ・デプロイ」「コスト規約」
- GCPプロジェクト: dev `octant-dev` / prod `octant`。リージョン `asia-northeast1`
- **`min_instances = 1` にするとアイドル課金で月2,000円規模**になる。0 を厳守
- 外部ロードバランサは使わない（転送ルールだけで月約2,800円の固定費が出るため）
