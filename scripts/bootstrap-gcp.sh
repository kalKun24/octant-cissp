#!/usr/bin/env bash
#
# GCP プロジェクトのブートストラップ（TICKET-016）。
#
# gcloud で自動化できる範囲だけを扱う。**冪等**であり、何度実行してもよい。
#   1. プロジェクトの作成
#   2. 課金アカウントの紐付け
#   3. API の有効化
#   4. tfstate 用 GCS バケットの作成
#
# **Firestore データベースはここで作らない。** TICKET-004 の Terraform が作る。
# 先に作るとリージョンと PITR の設定が Terraform の管理外になる。
#
# コンソール操作が必要な残りの手順（Firebase の有効化・Auth の Google プロバイダ・
# ウェブアプリ登録）は docs/gcp-bootstrap.md を参照。
#
# 使い方:
#   BILLING_ACCOUNT=XXXXXX-XXXXXX-XXXXXX scripts/bootstrap-gcp.sh
#   BILLING_ACCOUNT=... PROD_PROJECT=my-prod DEV_PROJECT=my-dev scripts/bootstrap-gcp.sh

set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

: "${PROD_PROJECT:=octant-prod}"
: "${DEV_PROJECT:=octant-dev}"
: "${REGION:=asia-northeast1}"

# 有効化する API。**追加するときはこの配列だけを直す。**
# 用途をコメントで残すこと（後から「なぜ有効にしたか」が分からなくなるため）。
readonly SERVICES=(
	cloudresourcemanager.googleapis.com # プロジェクト操作
	serviceusage.googleapis.com         # API の有効化そのもの
	iam.googleapis.com                  # サービスアカウント
	iamcredentials.googleapis.com       # Workload Identity Federation
	sts.googleapis.com                  # Workload Identity Federation
	run.googleapis.com                  # Cloud Run
	artifactregistry.googleapis.com     # コンテナイメージ
	firestore.googleapis.com            # Firestore
	firebase.googleapis.com             # Firebase（Auth / Hosting の親）
	firebasehosting.googleapis.com      # Firebase Hosting
	identitytoolkit.googleapis.com      # Firebase Authentication
	secretmanager.googleapis.com        # ANTHROPIC_API_KEY の保管
	storage.googleapis.com              # GCS（tfstate・バックアップ・添付）
	cloudscheduler.googleapis.com       # Firestore の週次バックアップ
	billingbudgets.googleapis.com       # 予算アラート
)

require() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "エラー: $1 が見つかりません。$2" >&2
		exit 1
	}
}

require gcloud "https://cloud.google.com/sdk/docs/install を参照してください。"

if [ -z "${BILLING_ACCOUNT:-}" ]; then
	echo "エラー: BILLING_ACCOUNT が未設定です。" >&2
	echo "  gcloud billing accounts list で ACCOUNT_ID を確認して渡してください。" >&2
	exit 1
fi

# ---------------------------------------------------------------------------
# 1. プロジェクト
# ---------------------------------------------------------------------------

# 注: gcloud projects describe は「存在しない」と「他者が所有」を区別できず、
# どちらも PERMISSION_DENIED を返す（存在の有無を秘匿する仕様）。
# そのため事前確認はせず、作成を試みて ALREADY_EXISTS を無視する。
create_project() {
	local project="$1" display="$2"

	if gcloud projects describe "$project" --format='value(projectId)' >/dev/null 2>&1; then
		echo "==> プロジェクト ${project} は既にあります"
		return 0
	fi

	echo "==> プロジェクト ${project} を作成します"
	if ! gcloud projects create "$project" --name="$display" 2>&1; then
		echo "エラー: ${project} を作成できませんでした。" >&2
		echo "  プロジェクト ID は全世界で一意です。既に使われている場合は" >&2
		echo "  PROD_PROJECT / DEV_PROJECT に別の ID を指定してください。" >&2
		exit 1
	fi
}

create_project "$PROD_PROJECT" "Octant prod"
create_project "$DEV_PROJECT" "Octant dev"

# ---------------------------------------------------------------------------
# 2. 課金
# ---------------------------------------------------------------------------

link_billing() {
	local project="$1"

	if [ "$(gcloud billing projects describe "$project" --format='value(billingEnabled)' 2>/dev/null)" = "True" ]; then
		echo "==> ${project} の課金は紐付け済みです"
		return 0
	fi

	echo "==> ${project} に課金アカウントを紐付けます"
	gcloud billing projects link "$project" --billing-account="$BILLING_ACCOUNT" >/dev/null
}

link_billing "$PROD_PROJECT"
link_billing "$DEV_PROJECT"

# ---------------------------------------------------------------------------
# 3. API
# ---------------------------------------------------------------------------

# gcloud services enable は既に有効な API を渡しても成功する（冪等）。
# 一括で渡すことで往復を減らす。
enable_services() {
	local project="$1"

	echo "==> ${project} の API を有効化します（${#SERVICES[@]} 件）"
	gcloud services enable "${SERVICES[@]}" --project "$project"
}

enable_services "$PROD_PROJECT"
enable_services "$DEV_PROJECT"

# ---------------------------------------------------------------------------
# 4. tfstate 用バケット
# ---------------------------------------------------------------------------

# Terraform は自分の state 置き場を同じ apply では作れないため、ここで用意する。
# バージョニングは state 破損からの復旧手段、削除保護は事故防止。
create_state_bucket() {
	local project="$1" bucket="gs://${1}-tfstate"

	if gcloud storage buckets describe "$bucket" --project "$project" >/dev/null 2>&1; then
		echo "==> ${bucket} は既にあります"
	else
		echo "==> ${bucket} を作成します"
		gcloud storage buckets create "$bucket" \
			--project "$project" \
			--location "$REGION" \
			--uniform-bucket-level-access \
			--public-access-prevention
	fi

	gcloud storage buckets update "$bucket" --versioning >/dev/null
	echo "==> ${bucket} のバージョニングを有効にしました"
}

create_state_bucket "$PROD_PROJECT"
create_state_bucket "$DEV_PROJECT"

# ---------------------------------------------------------------------------
# 結果
# ---------------------------------------------------------------------------

cat <<EOS

--------------------------------------------------------------------
  完了しました。

  prod  ${PROD_PROJECT}   tfstate: gs://${PROD_PROJECT}-tfstate
  dev   ${DEV_PROJECT}    tfstate: gs://${DEV_PROJECT}-tfstate

  **ここから先はコンソール操作が必要です。**
  docs/gcp-bootstrap.md の「コンソールでの作業」に進んでください。

    - Firebase をプロジェクトに追加
    - Authentication で Google サインインを有効化
    - 承認済みドメインの確認
    - ウェブアプリを登録して firebaseConfig を取得

  **Firestore データベースはまだ作らないでください。**
  TICKET-004 の Terraform が作ります（リージョンと PITR を管理下に置くため）。
--------------------------------------------------------------------

EOS
