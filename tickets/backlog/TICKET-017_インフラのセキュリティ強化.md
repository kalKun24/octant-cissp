# TICKET-017: インフラのセキュリティ強化（TICKET-004 の残指摘）

## 基本情報

| 項目 | 内容 |
|---|---|
| チケットID | TICKET-017 |
| ステータス | 🔴 未着手 |
| 作成日 | 2026-07-30 |
| 着手日 | - |
| 完了日 | - |
| ブランチ名 | - |
| PR番号 | - |
| PRリンク | - |

## 概要

TICKET-004 のセキュリティレビューで指摘された Medium 5件のうち、
**同チケット内で対応しなかったもの**を片付ける。いずれも
「今すぐ悪用できるわけではないが、多層防御として欠けている」もの。

> **前提**: TICKET-004 で最優先3件（`firestore.rules` のデプロイ・
> `firebase-adminsdk` SA の `tokenCreator` 剥奪・`octant-backup` SA の権限最小化）は
> 対応済み。このチケットは残りを扱う。

## 背景・目的

このプロジェクトは「**認可はアプリの1箇所だけ**」という設計を採っている。
その1箇所が堅牢であることは TICKET-003 で確認したが、**迂回経路が残っていると
設計の前提が崩れる**。また、迂回されたときに**検知する手段が無い**。

コスト面では、予算アラートが**通知のみで課金を止めない**ため、
上限が予算の数十倍になりうる状態が残っている。

## 受け入れ条件

### 1. データアクセス監査ログの有効化（最優先）

現状 `auditConfigs` が dev / prod とも `null` で、**Firestore のデータ読み書きと
Secret Manager のアクセスが一切ログに残らない**。認可を迂回された場合
（SA キーの漏洩など）、**誰が何を読んだのか永久に分からない**。

- [ ] Terraform の `google_project_iam_audit_config` で
      `datastore.googleapis.com` と `secretmanager.googleapis.com` の
      `DATA_READ` / `DATA_WRITE` を有効化する
- [ ] dev / prod の両方に apply 済み
- [ ] **実際に Firestore へ読み書きしてログに記録されることを確認**する
      （記録されない設定になっていないかの実測）
- [ ] ログ量が Cloud Logging の無料枠（50GiB/月）に収まる見込みであることを確認

### 2. tfstate バケットのレガシーロール削除

`gs://octant-{dev,prod}-tfstate` に以下が付いており、UBLA を有効にしても残る。

```
roles/storage.legacyBucketOwner  → projectEditor, projectOwner
roles/storage.legacyObjectOwner  → projectEditor, projectOwner
roles/storage.legacyObjectReader → projectViewer
```

**既定のコンピュート SA（`roles/editor` 保有）が tfstate を読み書き・削除できる。**
state を改ざんされると、次の apply で任意のリソースを差し替えられる。

- [ ] tfstate バケットから `legacyBucketOwner` / `legacyObjectOwner` /
      `legacyObjectReader` を削除する
- [ ] **既定のコンピュート SA から `roles/editor` を剥奪**する
      （`576603340630-compute@` / `540828365194-compute@`）
- [ ] 剥奪後に `terraform plan` / `apply` が通ることを確認する
- [ ] TICKET-005 の WIF 用 SA を作る際に、必要な権限だけを明示的に付与する方針を記録

### 3. サービスレベル `scaling` の Terraform 管理

モジュールは `template.scaling` しか設定しておらず、**サービスレベルの
`scaling.maxInstanceCount = 20` が Terraform 管理外**になっている。

- [ ] `google_cloud_run_v2_service` のサービスレベル `scaling` も明示する
- [ ] **ロールアウト中に新旧2リビジョンで実効上限が倍になる**点を
      許容するか、`scaling_mode` で制御するかを判断して記録する
- [ ] `terraform plan` が該当値の変更を検出することを実測確認する

### 4. 課金の自動停止

予算アラートは**通知のみで課金を止めない**。しかも Cloud Billing の予算評価は
数時間〜1日遅延するため、週末に流されると請求が積み上がった後に気づく。

Cloud Run は `allUsers` に公開されており（Firebase Hosting の rewrites の制約）、
**401 で弾かれるリクエストも課金対象**。prod を1ヶ月飽和させた場合の上限は
概算で **月5万円規模**（予算は1,000円）。

- [ ] 予算 → Pub/Sub → Cloud Functions で**課金を自動無効化**する仕組みを入れる
      （Google 公式の "capping costs" パターン）
- [ ] **`billing.admin` を持つ SA が増えるトレードオフ**を評価し、判断を記録する
- [ ] dev で実際に発火させて課金が止まることを確認する（prod では試さない）
- [ ] **誤発火時の復旧手順**を `infra/README.md` に記載する

### 5. Browser API キーのリファラ制限

Firebase が自動生成した Browser キーに制限が一切かかっていない
（`browserKeyRestrictions = {}`、`apiTargets` は27サービス）。

API キー自体は公開前提の値だが、無制限だと第三者が `identitytoolkit` /
`securetoken` を自由に叩ける（クォータ消費・認証フローの試行）。

- [ ] Hosting のドメイン確定後、HTTP リファラ制限を
      `https://<project>.web.app/*` と `https://<project>.firebaseapp.com/*` に設定
- [ ] `apiTargets` を `identitytoolkit` / `securetoken` / `firebase` /
      `firebasehosting` 程度に絞る
- [ ] **`localhost` からの開発が壊れないことを確認**する
      （エミュレータ利用時はキーを使わないはずだが要検証）

## サブチケット（コミット計画）

- [ ] `feat(infra): データアクセス監査ログを有効化`
- [ ] `fix(infra): tfstate バケットのレガシーロールを削除`
- [ ] `fix(infra): 既定のコンピュート SA から editor を剥奪`
- [ ] `fix(infra): サービスレベルの scaling を Terraform 管理下に置く`
- [ ] `feat(infra): 予算超過時に課金を自動停止する`
- [ ] `fix(infra): Browser API キーにリファラ制限を設定`
- [ ] `docs(infra): 復旧手順と判断の記録を追記`

## 関連情報

- CLAUDE.md「インフラ・デプロイ」「コスト規約」
- TICKET-004 の完了時メモ（セキュリティレビューの全指摘）
- `infra/README.md`「Terraform 管理外の設定」

### 実行順序の推奨

**1 → 2 → 3 → 5 → 4** の順。4（課金自動停止）は `billing.admin` を持つ
仕組みが増えるため、他が片付いてから慎重に入れる。

### 依存

- **5（リファラ制限）は Hosting のドメインが確定してから**。TICKET-005 の後
- 2 の「コンピュート SA の editor 剥奪」は、**TICKET-005 で Cloud Build を
  使う設計にした場合に影響する**可能性がある。005 の実装方針を確認してから行う

### やらないと決めたこと

- **CMEK による tfstate の暗号化**: state を読める主体はすべて IAM 境界の内側で、
  CMEK を入れても同じ IAM で復号できるため実効的な追加防御にならない。
  Cloud KMS は月額課金でコスト規約にも反する。**GMEK のままとする**
- **IAP / 外部ロードバランサによる Cloud Run の保護**: 転送ルールだけで
  月約2,800円の固定費。コスト規約に反する。認可はアプリ側に集約する方針を維持
- **`ingress = internal-and-cloud-load-balancing`**: Firebase Hosting の
  rewrites が 403 になるため採用不可（実装者が検証済み）
