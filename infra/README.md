# infra（Terraform）

GCP プロジェクトの**中身**を定義する。プロジェクトそのもの・課金の紐付け・
API の有効化・tfstate バケットは **TICKET-016 のブートストラップ**が作る
（`docs/gcp-bootstrap.md`）。Terraform はそこから先だけを持つ。

## 構成

```
modules/
  stack/              1 環境分を組み立てる合成モジュール（environments はこれを呼ぶだけ）
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
