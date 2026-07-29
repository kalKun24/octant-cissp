# TICKET-004: Terraform（dev / prod のインフラ定義）

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-004 |
| ステータス | 🟢 完了 |
| 作成日 | 2026-07-27 |
| 着手日 | 2026-07-29 |
| 完了日 | 2026-07-30 |
| ブランチ名 | `feature/TICKET-004` |
| PR番号 | #13 |
| PRリンク | https://github.com/kalKun24/octant-cissp/pull/13 |

## 概要

GCP のリソースを Terraform で定義する。Cloud Run・Firestore・Cloud Storage・
Artifact Registry・Secret Manager・予算アラート・週次バックアップまで。dev / prod の2環境。

> **⚠ 前提条件: TICKET-016（GCP ブートストラップ）が完了していること。**
> GCP プロジェクト・課金の紐付け・API の有効化・**tfstate 用 GCS バケット**が
> 無いと `terraform init` すら通らない。**このチケットはプロジェクトを作らない。**
> Terraform が管理するのは「プロジェクトの中身」だけ。
>
> **順序の制約**: このチケットの `apply` は **TICKET-005（CI/CD）より先に完了している必要がある**。
> Cloud Run サービスや Artifact Registry が存在しない状態で自動デプロイが走れば失敗するため。
> **prod への `apply` も、TICKET-005 を `main` にマージする前に済ませておくこと。**
>
> ```
> 016（ブートストラップ）→ 004（このチケット）→ 005（CI/CD）
> ```

## 背景・目的

手作業で作ったリソースは再現できず、コスト事故の温床になる。
特に **`min_instances = 0`** と **Artifact Registry のクリーンアップポリシー**は
固定費ゼロを維持するための必須設定であり、コードで固定しておく必要がある。

## 受け入れ条件

- [x] `make tf-plan ENV=dev` と `make tf-plan ENV=prod` がエラーなく差分を出力する
- [x] **dev と prod の両方に `apply` が完了している**（TICKET-005 の前提条件）
- [x] tfstate が **TICKET-016 で作成済みの GCS バケット**を backend として使っている
      （**このチケットでバケットを作らない**。Terraform は自分の state 置き場を同じ apply では作れない）
- [x] **Firestore データベースをこの Terraform で作成する**（TICKET-016 では作らない方針。
      コンソールで先に作るとリージョンと PITR が Terraform 管理外になる）
- [x] 初回 `apply` は手元の ADC（`gcloud auth application-default login`）で実行する。
      **CI からの Terraform 実行は TICKET-005 で WIF を作った後**（鶏と卵のため）
- [x] **Cloud Run の `min_instances` が 0** に設定されている
- [x] **Cloud Run の `max_instances` が dev: 2 / prod: 5** に設定されている
- [x] **Artifact Registry にクリーンアップポリシー（最新3世代のみ保持）**が設定されている
- [x] Firestore が PITR 有効・削除保護有効で作成される
- [x] GCS のバックアップバケットに 90 日のライフサイクルルールがある
- [x] 予算アラート（月1,000円、50/90/100%）が設定され、通知先メールが変数化されている
- [x] Cloud Scheduler が週次で Firestore を GCS へエクスポートする
- [x] Secret Manager に `anthropic-api-key` の入れ物があり、Cloud Run が環境変数として参照する
- [x] **サービスアカウントキーの JSON を生成する定義が存在しない**
- [x] `infra/modules/` が再利用単位に分かれ、`environments/dev|prod` は値だけを持つ

## サブチケット（コミット計画）

- [x] `feat(infra): GCS バックエンドとプロバイダ設定を追加`
- [x] `feat(infra): firestore モジュールを追加`
- [x] `feat(infra): artifact-registry モジュール（クリーンアップポリシー付き）を追加`
- [x] `feat(infra): cloud-run-service モジュールを追加`
- [x] `feat(infra): storage モジュール（添付・バックアップ）を追加`
- [x] `feat(infra): 予算アラートと週次バックアップの定義を追加`
- [x] `feat(infra): dev / prod 環境の値を追加`
- [x] `chore(repo): make tf-plan / tf-apply を Makefile に追加`

## 関連情報

### TICKET-003 からの申し送り: Cloud Run への環境変数注入は必須

**次の3つが欠けると、リビジョンは起動に失敗する**（設定漏れを黙って既定値で
埋めると、認証の緩い側へ倒れるため、あえて起動失敗にしてある）。
Terraform の `cloud-run-service` モジュールで**必ず注入すること。**

| 環境変数 | 値 | 欠けた場合 |
|---|---|---|
| `APP_ENV` | `dev` または `prod` | 起動失敗。**既定値は無い**（`local` に落とさない） |
| `FIREBASE_PROJECT_ID` | 実際の Firebase プロジェクト ID | 起動失敗（ID トークンの `iss` / `aud` 照合に必要） |
| `ALLOWED_EMAILS` | 許可メールのカンマ区切り | 起動失敗（1件も無い設定では起動しない） |

- `APP_ENV` を `local` にしないこと。`local` 以外でのみ
  `FIREBASE_AUTH_EMULATOR_HOST` の混入ガードが効く
- **`FIREBASE_AUTH_EMULATOR_HOST` は絶対に設定しない。** 設定されていると
  `APP_ENV` が `dev` / `prod` のとき起動を拒否する（エミュレータ接続時は
  ID トークンの署名検証が省略されるため）
- `ALLOWED_EMAILS` は個人のメールアドレスなので、`.tfvars` に直書きせず
  変数経由で渡すこと（`*.tfvars` は `.gitignore` 済み）

### そのほか

- CLAUDE.md「インフラ・デプロイ」「コスト規約」
- **前提: TICKET-016（GCP ブートストラップ）が完了していること**
- GCPプロジェクト: dev **`octant-dev`** / prod **`octant-prod`**。リージョン `asia-northeast1`
  （TICKET-016 で作成済み。`octant` は取得できなかったため `octant-prod` になった）
- tfstate バケットは **`gs://octant-dev-tfstate`** / **`gs://octant-prod-tfstate`**（作成済み）
- **旧プロジェクト `octant-cissp` には触れない**（旧 octant が稼働中。新規2つと分離済み）
- **Firestore はこの Terraform で作る。** TICKET-016 では意図的に未作成にしてある
  （コンソールで先に作るとリージョンと PITR が管理外になるため）
- **`min_instances = 1` にするとアイドル課金で月2,000円規模**になる。0 を厳守
- 外部ロードバランサは使わない（転送ルールだけで月約2,800円の固定費が出るため）
- 後続: **TICKET-005（CI/CD）はこのチケットの `apply` 完了を前提にしている**

## 完了時メモ

### 確定した構成

dev / prod の両方に apply 済み。`terraform plan` は両環境とも No changes。

| リソース | dev | prod |
|---|---|---|
| Cloud Run `octant-api` | min=0 / max=2 | min=0 / max=5 |
| Firestore | PITR + 削除保護 + asia-northeast1 | 同左 |
| Artifact Registry | クリーンアップ（最新3世代） | 同左 |
| GCS | 添付 + バックアップ（90日） | 同左 |
| Secret Manager | `anthropic-api-key`（**プレースホルダのまま**） | 同左 |
| Cloud Scheduler | 週次エクスポート（月 03:00 JST） | 同左 |
| 予算 | 月1,000円（50/90/100%） | 同左 |

**固定費ゼロ、実質 ¥0〜50/月。** 旧 `octant-cissp` は無傷。

### セキュリティレビューで見つかり、このチケットで塞いだ3件

**1. `firestore.rules` が未デプロイだった**
現在の安全性が「ruleset が存在しない」という既定の副作用に依存し、
デプロイする仕組みが Makefile にも CI にも無かった。Firebase コンソールで
Firestore を開くと「テストモード（30日 allow all）」を提示され、
押した瞬間に全公開になる状態。**データが0件のうちに**両環境へデプロイし、
`make rules-deploy` / `rules-check` を追加した。

**2. `firebase-adminsdk` SA が全防壁を迂回できた**
`roles/iam.serviceAccountTokenCreator` がプロジェクトレベルで付与され、
`octant-api` を含む全 SA を借用できた。**アプリはこの SA を一切使っていない**
にもかかわらず、Firestore 全読み書き・ルール書き換え・任意ユーザー作成が
可能だった。両環境から剥奪し、剥奪後に統合テストと `/api/health` の正常を確認。

**3. `octant-backup` SA の権限が過剰だった**
`storage.admin` はバケットスコープでも `setIamPolicy` / `objects.delete` を含み、
侵害されるとバックアップを**復旧不能に破壊**でき、外部アカウントへの
閲覧権付与でノート本文を持ち出せた。`importExportAdmin` は import を含むため
**Firestore を丸ごと差し替え**られた。最小権限に置き換え、
**dev で絞った権限のままバックアップが SUCCESSFUL になることを実測**してから
prod に適用した。

### 設計判断

- **`min_instances` を変数化しない**（リテラル直書き）。環境ごとの書き間違いで
  課金事故が起きる余地を構造的に消すため
- **`ignore_changes` を image に付ける**。TICKET-005 の CI がイメージを差し替えるため、
  無視しないと次の apply が本番をプレースホルダに巻き戻す。
  **Terraform は構成を、CI はイメージを持つ**
- **Secret にプレースホルダ版を1つ作る**。バージョンが皆無だと Cloud Run が
  `latest` を解決できずリビジョンの起動自体が失敗し、apply が通らない
- **`allUsers` に `run.invoker` を付与**。Firebase Hosting の rewrites は
  公開サービスにしか転送できないため回避不能。代替（ingress 制限・IAP・
  カスタムヘッダ）はすべて不可か固定費が発生することを検証済み

### Terraform 管理外（`terraform plan` が差分を検出しない）

- **Firestore のセキュリティルール**（`firebase deploy` でリリース）
- **`firebase-adminsdk` SA の権限**（Firebase が自動作成・自動付与）

**再付与されても気づけない**ため、確認コマンドと再剥奪手順を
`infra/README.md`「Terraform 管理外の設定」に記録した。
組織が無いため `constraints/iam.disableServiceAccountKeyCreation` を
強制できず、**この SA の JSON キーが1つ漏れると全終了**である点も明記。

### 残作業

- **Anthropic API キーの投入**（現在プレースホルダ。AI 機能は TICKET-012）
- **予算通知メールの承認**（確認メールを承認しないとアラートが飛ばない）
- **TICKET-017** に残 Medium 5件を切り出し済み（監査ログ・tfstate のレガシーロール・
  サービスレベル scaling・課金自動停止・Browser API キーの制限）

### TICKET-005 への申し送り

- WIF の CI 用 SA は **`roles/run.admin` ではなく `roles/run.developer` +
  特定サービスへの `actAs`** に絞ること（`ignore_changes` により
  `terraform plan` が不正なイメージ差し替えを検出しないため）
- **`~/go/bin` キャッシュを WIF ワークフローと共有しないこと**
- `firebase.json` に hosting セクションがまだ無い。`/api/**` の rewrites は
  **SPA フォールバックより前**に置くこと
- **`default_uri_disabled`** で `*.run.app` の直接アクセスを塞げる可能性があるが、
  **Hosting の rewrites が動くか未検証**。dev で先に試すこと。未検証のまま prod に入れない
