# infra（Terraform）

GCP プロジェクトの**中身**を定義する。プロジェクトそのもの・課金の紐付け・
API の有効化・tfstate バケットは **TICKET-016 のブートストラップ**が作る
（`docs/gcp-bootstrap.md`）。Terraform はそこから先だけを持つ。

## 構成

```
modules/
  stack/              1 環境分を組み立てる合成モジュール（environments はこれを呼ぶだけ）
  ci-workload-identity/ GitHub Actions のキーレス認証（WIF）とデプロイ用 SA
  firestore/          Firestore（PITR・削除保護）
  artifact-registry/  イメージ置き場（最新 3 世代のみ保持）
  cloud-run-service/  API サービス（min_instances = 0 固定）
  storage/            添付バケットとバックアップバケット（90 日で削除）
  secret/             anthropic-api-key の入れ物
  budget/             月次予算アラート
  firestore-backup/   Cloud Scheduler による週次エクスポート
environments/
  dev/                octant-dev の値
  prod/               octant-prod の値
```

**environments には値しか書かない。** リソースを直接足すと dev と prod の構成が
静かにずれるため、変更は必ず `modules/` 側に入れる。

## 使い方

```bash
gcloud auth application-default login          # 初回だけ

cp environments/dev/terraform.tfvars.example environments/dev/terraform.tfvars
$EDITOR environments/dev/terraform.tfvars      # 許可メールと通知先メールを埋める

make tf-plan  ENV=dev
make tf-apply ENV=dev
```

`terraform.tfvars` は `.gitignore` 済み。**個人のメールアドレスをリポジトリに置かない。**

## 変えてはいけない値

| 設定 | 値 | 理由 |
|---|---|---|
| Cloud Run `min_instance_count` | **0** | 1 にするとアイドル課金で月 2,000 円規模。変数にもしていない |
| Cloud Run `max_instance_count` | dev 2 / prod 5 | 暴走時の青天井を防ぐ |
| Artifact Registry のクリーンアップ | 最新 3 世代 | 無料枠 0.5GB をイメージで超えないため |
| 外部ロードバランサ | **作らない** | 転送ルールだけで月約 2,800 円の固定費 |
| Firestore の削除保護 / PITR | 有効 | 誤削除からの復旧手段 |

## 手作業が要るもの

Terraform が作るのは**入れ物まで**で、値は入れない。

```bash
# Anthropic API キーの投入（キーは state にもリポジトリにも置かない）
printf '%s' "$ANTHROPIC_API_KEY" | \
  gcloud secrets versions add anthropic-api-key --project octant-dev --data-file=-
```

## CI/CD（Workload Identity Federation）

GitHub Actions は `octant-ci@<project>` を **キーレスで借用**してデプロイする。
定義は `modules/ci-workload-identity/`、パイプライン全体の説明は `docs/deploy.md`。

**鶏と卵に注意。** WIF を作るのは Terraform だが、その Terraform を CI から
動かすにも WIF が要る。**初回作成は手元の ADC で `make tf-apply ENV=dev|prod`** を実行する。

**CI から `terraform apply` はしない。** Terraform に要る権限は広く、
それを CI に渡すとデプロイ用 SA を絞った意味が消える。
インフラの変更は手元の ADC で行い、CI はイメージと配信物だけを持つ。

借用を許可しているのは「リポジトリ × ブランチ」の組み合わせだけ。

```bash
make tf-output ENV=dev | grep ci_      # プロバイダ・SA・許可した principalSet
```

**`ci_allowed_refs` を一時的に広げて検証したら、必ず元に戻すこと。**

## イメージの扱い

Cloud Run は初回だけ `gcr.io/cloudrun/hello` で作る。実イメージへの差し替えは
**TICKET-005 の CI** が行い、Terraform は `template[0].containers[0].image` の
変更を無視する。無視しないと、次の `terraform apply` が本番をプレースホルダへ
巻き戻してしまう。**Terraform は構成を、CI はイメージを持つ。**

## バックアップと復元

Cloud Scheduler が毎週月曜 03:00（JST）に Firestore を
`gs://<project>-backups/<日時フォルダ>` へエクスポートする。90 日で自動削除。

```bash
gcloud scheduler jobs run firestore-weekly-export \
  --location asia-northeast1 --project octant-dev     # 手動実行
gcloud firestore import gs://octant-dev-backups/<日時フォルダ> --project octant-dev
```

## Terraform 管理外の設定（手作業で維持する）

以下は Firebase / GCP が自動作成するため Terraform が管理していない。
**`terraform plan` は差分を検出しないので、変わっても気づけない。**
定期的に、あるいはプロジェクトを作り直したときに手で確認すること。

### firebase-adminsdk SA から剥奪済みのロール

Firebase をプロジェクトに追加すると `firebase-adminsdk-fbsvc@` が作られ、
**`roles/iam.serviceAccountTokenCreator` がプロジェクトレベルで付与される。**

このロールは `octant-api` / `octant-backup` を含む**プロジェクト内の全 SA を
借用できる**ことを意味する。この SA はアプリから一切使っていない
（Cloud Run 上の Firebase Admin SDK は ADC ＝ `octant-api` SA で動き、
ID トークンの検証は公開鍵だけで完結する）にもかかわらず、
残しておくと「認可はアプリ1箇所」という設計を迂回できる待機中の攻撃面になる。

**2026-07-30 に dev / prod の両方から剥奪済み。** 剥奪後も統合テストと
`/api/health` は正常だった。

```bash
# 確認（0 件が正しい）
for p in octant-dev octant-prod; do
  gcloud projects get-iam-policy "$p" --flatten="bindings[].members" \
    --filter="bindings.role:roles/iam.serviceAccountTokenCreator" \
    --format='value(bindings.members)'
done

# 再付与されていた場合の剥奪
gcloud projects remove-iam-policy-binding <project> \
  --member="serviceAccount:firebase-adminsdk-fbsvc@<project>.iam.gserviceaccount.com" \
  --role="roles/iam.serviceAccountTokenCreator"
```

**組織が無いため `constraints/iam.disableServiceAccountKeyCreation` を強制できない。**
この SA の JSON キーが1つ漏れると Firestore の全読み書き・ルールの書き換え・
任意ユーザーの作成がすべて可能になる。**キーを発行しないこと。**

### Firestore のセキュリティルール

`firebase deploy` でリリースするため Terraform 管理外。

```bash
make rules-deploy ENV=dev    # デプロイ
make rules-check  ENV=dev    # リリース済みか検証
```

**ruleset がリリースされていない状態でも Firestore は全拒否する**が、
それは既定の副作用であってコードで固定された保証ではない。
Firebase コンソールで Firestore のページを開くと
「テストモード（30日間 allow all）」を提示され、押した瞬間に全公開になる。
