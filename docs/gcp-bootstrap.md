# GCP ブートストラップ手順

GCP をゼロから立ち上げ、Terraform が動き出せる状態にするまでの手順（TICKET-016）。

**ブラウザでの操作が必要なのは「Google サインインの有効化」1箇所だけです。**
残りはすべて `gcloud` と `firebase` CLI で完結します。

## 実施済みの構成（2026-07-29）

| 環境 | プロジェクト ID | tfstate バケット |
|---|---|---|
| prod | `octant-prod` | `gs://octant-prod-tfstate` |
| dev | `octant-dev` | `gs://octant-dev-tfstate` |

課金アカウント: `013D28-C16A68-E3B1DA`（My Billing Account）
リージョン: `asia-northeast1`（東京）

> **旧プロジェクト `octant-cissp` には触れていません。** 旧 octant（単一HTML版）が
> 稼働中のため、新規に2つ作って完全に分離しています。

## 前半: スクリプトで自動化される部分

```bash
BILLING_ACCOUNT=013D28-C16A68-E3B1DA scripts/bootstrap-gcp.sh
```

**冪等なので何度実行しても構いません。** 以下を行います。

1. プロジェクトの作成（既にあればスキップ）
2. 課金アカウントの紐付け（済んでいればスキップ）
3. API の有効化 15 件
4. tfstate 用 GCS バケットの作成（バージョニング有効・公開アクセス防止・削除保護）

別の ID を使う場合:

```bash
BILLING_ACCOUNT=... PROD_PROJECT=my-prod DEV_PROJECT=my-dev scripts/bootstrap-gcp.sh
```

### 補足: プロジェクト ID の空き確認について

**`gcloud projects describe` では空き状況を判別できません。** 存在しない ID でも
「他者が所有する ID」でも同じ `PERMISSION_DENIED` を返します（存在の有無を秘匿する仕様）。
エラーメッセージにも `(or it may not exist)` と書かれています。

**実際に `gcloud projects create` を試すのが唯一の確認方法**です。
スクリプトはこの前提で書かれており、失敗したら別 ID を促します。

---

## 後半: Firebase の設定

**ブラウザでの操作が必要なのは「Google サインインの有効化」だけです。**
残りは `firebase` CLI で完結します。

### 1. Firebase をプロジェクトに追加（CLI で可）

```bash
firebase projects:addfirebase octant-prod
firebase projects:addfirebase octant-dev
```

> コンソールから行う場合は**必ず「既存の GCP プロジェクトを選ぶ」**こと。
> Firebase コンソールで新規作成すると GCP プロジェクト ID が自動採番され、
> 作成済みのプロジェクトと別物になります。

### 2. Authentication で Google サインインを有効化（**ここだけブラウザ必須**）

各プロジェクトで:

1. **Authentication** → **始める**
2. **Sign-in method** タブ → **Google** を選択 → **有効にする**
3. **プロジェクトの公開名**とサポートメールを設定して保存

これで OAuth クライアントが自動生成されます。

### 3. 承認済みドメインの確認（CLI で可）

```bash
TOKEN=$(gcloud auth print-access-token)
for p in octant-prod octant-dev; do
  curl -s -H "Authorization: Bearer $TOKEN" -H "x-goog-user-project: $p" \
    "https://identitytoolkit.googleapis.com/admin/v2/projects/$p/config" \
    | python3 -c "import sys,json;print(json.load(sys.stdin).get('authorizedDomains'))"
done
```

`localhost` / `<project-id>.firebaseapp.com` / `<project-id>.web.app` が
既定で入ります。**入っていないドメインからはサインインできません。**

> **`x-goog-user-project` ヘッダが必須です。** 付けないと
> 「quota project が未設定」という 403 になります（API 無効と紛らわしい）。

### 4. ウェブアプリを登録して firebaseConfig を取得（CLI で可）

```bash
firebase apps:create WEB octant-web --project octant-dev
firebase apps:list WEB --project octant-dev        # appId を確認
firebase apps:sdkconfig WEB <appId> --project octant-dev
```

**Hosting サイトはプロジェクトに Firebase を追加した時点で自動作成されます**
（`octant-dev` / `octant-prod`）。確認は次のとおり。

```bash
firebase hosting:sites:list --project octant-dev
```

### 5. 取得した値の置き場

**`firebaseConfig` は秘密情報ではありません**（クライアントに配布されます）が、
プロジェクト固有なので**リポジトリに実値を置きません**。

`frontend/.env.example` のキーに対応する環境変数として渡します。

```bash
cp frontend/.env.example frontend/.env
# 控えた値を frontend/.env に貼る（.env は .gitignore 済み）
```

CI とデプロイでは GitHub Secrets / Cloud Run の環境変数として渡します（TICKET-005）。

---

## やらないこと

### Firestore データベースを作らない

**コンソールで「Firestore を使ってみる」を押さないでください。**

押すとリージョン固定でデータベースが作られ、**PITR や削除保護の設定が
Terraform の管理外**になります。TICKET-004 の Terraform が作ります。

### サービスアカウントキーを発行しない

認証は手元の ADC（`gcloud auth application-default login`）と、
CI の Workload Identity Federation だけを使います。
**JSON キーを発行するとリポジトリや CI に置く誘惑が生まれる**ため作りません。

---

## 完了後の確認

```bash
# プロジェクトと課金
for p in octant-prod octant-dev; do
  echo "$p: $(gcloud billing projects describe "$p" --format='value(billingEnabled)')"
done

# tfstate バケット
for p in octant-prod octant-dev; do
  gcloud storage buckets describe "gs://${p}-tfstate" \
    --format='value(name,versioning_enabled,public_access_prevention)'
done

# Firestore が未作成であること（0 件が正しい）
for p in octant-prod octant-dev; do
  echo "$p: $(gcloud firestore databases list --project "$p" --format='value(name)' | wc -l) 件"
done
```

## 次のステップ

**TICKET-004（Terraform）** に進みます。その際に必要なもの:

```bash
gcloud auth application-default login

# 既定プロジェクトとクォータプロジェクトを dev に向ける。
# **--project を付け忘れたコマンドの着地点を安全側にするため。**
# 既定のままだと旧 octant-cissp（稼働中）に向かい、削除系の誤爆が事故になる。
gcloud config set project octant-dev
gcloud auth application-default set-quota-project octant-dev
```

Terraform は ADC を使うため、これを済ませておく必要があります。
**初回の `apply` は手元の ADC で行います**（CI から動かすための WIF を
作るのが Terraform 自身であり、鶏と卵になるため）。

> `gcloud auth application-default login` は**クォータプロジェクトを
> gcloud の既定プロジェクトに合わせて自動設定します。** 既定が旧プロジェクトのままだと
> ADC もそちらを向くため、上記2行で明示的に付け替えています。
> Terraform 側では `billing_project` と `user_project_override` を
> プロバイダに設定し、ADC の設定に依存しない形にします（TICKET-004）。
