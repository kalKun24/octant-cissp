# デプロイ

`develop` への push で dev、`main` への push で prod へ自動デプロイする（TICKET-005）。
**手で `gcloud run deploy` を打たない。**

```
develop へ push ──▶ deploy-dev.yml  ──┐
                                      ├─▶ deploy.yml（検証 → デプロイ）─▶ scripts/deploy.sh
main へ push    ──▶ deploy-prod.yml ──┘
```

| 環境 | ブランチ | GCP プロジェクト | 公開 URL |
|---|---|---|---|
| dev | `develop` | `octant-dev` | https://octant-dev.web.app |
| prod | `main` | `octant-prod` | https://octant-prod.web.app |

**`main` へのマージがそのまま本番リリース**になる。マージコミット自体が
prod のデプロイを発火させるため、`develop` で確認してから `main` へ入れる。

## 何が起きるか

`scripts/deploy.sh` が次を順に行う。CI と `make deploy-dev` は**同じスクリプト**を呼ぶ。

1. `backend/` のコンテナをビルドし、`api:<コミット SHA>` として Artifact Registry へ push
2. Cloud Run を **イメージだけ**差し替える（`gcloud run services update`）。
   リビジョン名は `octant-api-<短縮 SHA>-<実行番号>-<試行番号>`
3. 新しいリビジョンが**準備完了になっている**ことを確認する
4. `firebase apps:sdkconfig` で `firebaseConfig` を取得し、フロントをビルドする
5. `firestore.rules`（全拒否）と Hosting をデプロイする
6. **Hosting 経由**で `GET /api/health` が 200 を返し、`revision` が
   いま出したリビジョンと一致することを確認する

6 が肝心。環境変数の設定漏れは「新リビジョンが起動せず、旧リビジョンに
トラフィックが残ったまま」という形で現れ、**デプロイコマンド自体は成功する**。
`/api/health` の `revision` まで照合してこの状態を落とす。

## 認証（Workload Identity Federation）

**サービスアカウントキーの JSON は存在しない。** リポジトリにも GitHub Secrets にも無い。

GitHub Actions の OIDC トークンを GCP の STS が検証し、
`kalKun24/octant-cissp` の**特定ブランチ**だけがデプロイ用 SA（`octant-ci@<project>`）を
借用できる。定義は `infra/modules/ci-workload-identity/`。

| 環境 | 借用を許可した principalSet |
|---|---|
| dev | `...github/attribute.repository_ref/kalKun24/octant-cissp@refs/heads/develop` |
| prod | `...github/attribute.repository_ref/kalKun24/octant-cissp@refs/heads/main` |

`develop` への push が prod を触ることは、権限の構造上できない。

### デプロイ用 SA の権限

**`roles/run.admin` は付けていない。** Cloud Run の `image` は Terraform の
`ignore_changes` 対象で、`terraform plan` は不正なイメージ差し替えを検出できない。
デプロイ権限が広いほど「`octant-api` SA（Firestore 全アクセス）で動く任意のコード」を
静かに動かせる余地が広がるため、次のように絞ってある。

| 権限 | スコープ |
|---|---|
| `roles/run.developer` | **`octant-api` サービス 1 つだけ** |
| `roles/iam.serviceAccountUser`（actAs） | **`octant-api` SA 1 つだけ** |
| `roles/artifactregistry.writer` | **`octant` リポジトリだけ** |
| `octantCiRunOperationViewer`（カスタム。`run.operations.get` のみ） | プロジェクト |
| `roles/firebasehosting.admin` | プロジェクト（サイト単位の IAM が無いため） |
| `roles/firebaserules.admin` | プロジェクト（同上） |
| `roles/serviceusage.serviceUsageConsumer` | プロジェクト |

`run.operations.get` にカスタムロールを使っているのは、`roles/run.viewer` だと
**全 Cloud Run サービスの構成（= 環境変数の中身。`ALLOWED_EMAILS` は個人の
メールアドレス）を読めてしまう**ため。

**CI から `terraform apply` はしない。** Terraform に要る権限は広く、
それを CI へ渡すと上記の絞り込みが無意味になる。インフラの変更は手元の ADC で行う。

## 環境変数と firebaseConfig

- **`APP_ENV` / `FIREBASE_PROJECT_ID` / `ALLOWED_EMAILS` / `LOG_LEVEL` /
  `ANTHROPIC_API_KEY` は Terraform が持つ。** CI は上書きしない
  （「Terraform は構成を、CI はイメージを持つ」）
- **`firebaseConfig` は CI が `firebase apps:sdkconfig` で取得する。**
  秘密情報ではないがプロジェクト固有なので、リポジトリにも Secrets にも実値を置かない。
  Firebase 側を唯一の正にすることで、ウェブアプリを作り直しても設定がずれない

## Firebase Hosting の rewrites

`firebase/firebase.json`（JSON なのでコメントを書けないためここに残す）。

```
/api/**  →  Cloud Run（octant-api / asia-northeast1）
**       →  /index.html（SPA フォールバック）
```

**順序が命。** SPA フォールバックを先に置くと `/api/**` が `index.html` を返す。
rewrites は上から順に評価され、最初に一致したものだけが適用される。

この構成により**ブラウザからは同一オリジン**（`https://<site>.web.app/api/...`）で
API を呼べる。**CORS の設定は不要**であり、サーバに CORS ミドルウェアを入れない。

## 検証済みの不採用: Cloud Run の既定 URL を塞ぐ

`*.run.app` への直アクセスを塞げば「Hosting 経由だけ」に絞れるはずだったが、
**dev で実測した結果、Hosting の rewrites も同時に落ちるため採用しない。**

| 設定 | `*.run.app` 直アクセス | Hosting 経由 `/api/health` |
|---|---|---|
| `default_uri_disabled = false`（採用） | 200 | 200 |
| `default_uri_disabled = true` | 404 | **404** |

さらに `false` に戻しても Hosting は 404 のままで、
**`firebase deploy --only hosting` を打ち直すまで復旧しなかった。**
Terraform の変数（`default_uri_disabled`）は残してあるが、既定は `false`。

直アクセスを塞ぎたい場合の代替は Hosting 由来かどうかをアプリ側で見る等になるが、
**認可はアプリの 1 箇所（ID トークン検証 + 許可メール）**で成立しており、
`*.run.app` を直接叩いても認証を通らなければ何も読めない。優先度は低い。

## Terraform を適用するとリビジョンが増える

`terraform apply` が Cloud Run の構成を変えると、**自動採番のリビジョン
（`octant-api-00006-ww5` など）が作られる。** `image` は `ignore_changes` の
対象なので**中身は CI が出したイメージのまま**だが、コミット SHA を含む
リビジョン名は引き継がれない。

どのコミットが動いているかは `/api/health` の `revision` ではなく、
リビジョンのイメージタグ（`api:<コミット SHA>`）で辿ること。

## 手元から出す

CI が動かせないときの出口。**通常は使わない。**

```bash
make deploy-dev          # dev へ
make deploy ENV=prod     # prod へ（原則やらない）
```

必要なもの: `gcloud`（ADC 認証済み）・`docker`・`firebase`・`node`、
`make setup-front` 済みの `frontend/node_modules`。

## 詰まったときの確認先

```bash
# どのリビジョンが応答しているか（health の revision がそのまま K_REVISION）
curl -s https://octant-dev.web.app/api/health

# リビジョンの一覧とイメージ
gcloud run revisions list --service octant-api --region asia-northeast1 --project octant-dev

# 起動に失敗したリビジョンのログ
gcloud run services logs read octant-api --region asia-northeast1 --project octant-dev --limit 50

# ルールがリリースされているか
make rules-check ENV=dev
```

### ロールバック

イメージはコミット SHA でタグ付けされているので、戻したいコミットの
ワークフローを再実行するのが基本。Cloud Run 側だけ即時に戻すなら:

```bash
gcloud run services update-traffic octant-api \
  --to-revisions <戻したいリビジョン>=100 \
  --region asia-northeast1 --project octant-dev
```

**トラフィックの固定は Terraform の `traffic`（最新へ 100%）と食い違う。**
`terraform apply` で最新へ戻るため、暫定処置として使い、原因を直したら
正規のデプロイで追い越すこと。
