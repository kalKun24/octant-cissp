# GitHub Actions がキーレスでデプロイするための Workload Identity Federation。
#
# **サービスアカウントキー（JSON）を発行しない**ための仕組み（CLAUDE.md「インフラ・デプロイ」）。
# GitHub が発行する OIDC トークンを GCP の STS が検証し、
# 許可した「リポジトリ × ブランチ」の組み合わせだけがデプロイ用 SA を借用できる。
#
# 鶏と卵: この定義を適用するのは**手元の ADC**（gcloud auth application-default login）。
# CI から Terraform を動かす構成にはしていない（後述の権限設計の理由による）。

data "google_project" "this" {
  project_id = var.project_id
}

# ---------------------------------------------------------------------------
# ID プール（GitHub の OIDC を受け入れる入口）
#
# **プール ID とプロバイダ ID は変更しないこと。** 削除しても 30 日間は
# 論理削除として残り、同じ ID をすぐには作り直せない。
# ---------------------------------------------------------------------------

resource "google_iam_workload_identity_pool" "github" {
  project                   = var.project_id
  workload_identity_pool_id = var.pool_id
  display_name              = "GitHub Actions"
  description               = "GitHub Actions からのキーレス認証（Octant ${var.env}）"
}

resource "google_iam_workload_identity_pool_provider" "github" {
  project                            = var.project_id
  workload_identity_pool_id          = google_iam_workload_identity_pool.github.workload_identity_pool_id
  workload_identity_pool_provider_id = var.provider_id
  display_name                       = "GitHub Actions OIDC"

  # **この条件が最後の砦。** これが無い（= 常に true）と、GitHub の
  # **任意のリポジトリ**が発行した OIDC トークンでプールに入れてしまう。
  # SA 側の principalSet でブランチまで絞るが、入口でもリポジトリを固定する。
  attribute_condition = "assertion.repository == \"${var.github_repository}\" && assertion.repository_owner == \"${local.repository_owner}\""

  attribute_mapping = {
    "google.subject"       = "assertion.sub"
    "attribute.repository" = "assertion.repository"
    "attribute.ref"        = "assertion.ref"

    # リポジトリとブランチを結合した属性。
    # これを principalSet に使うことで「このリポジトリの develop だけ」という
    # 粒度で SA の借用を許可できる（repository だけで絞ると、
    # 同じリポジトリの**任意のブランチ**からデプロイできてしまう）。
    "attribute.repository_ref" = "assertion.repository + \"@\" + assertion.ref"
  }

  oidc {
    issuer_uri = "https://token.actions.githubusercontent.com"
    # allowed_audiences を指定しない場合、既定の audience は
    # https://iam.googleapis.com/<プロバイダのリソース名> になる。
    # google-github-actions/auth が既定で要求する audience と一致するため空のままにする。
  }
}

# ---------------------------------------------------------------------------
# デプロイ用サービスアカウント
# ---------------------------------------------------------------------------

resource "google_service_account" "ci" {
  project      = var.project_id
  account_id   = var.service_account_id
  display_name = "Octant CI デプロイ (${var.env})"
  description  = "GitHub Actions が Workload Identity Federation で借用するデプロイ専用 SA。キーは発行しない"
}

# 借用できる主体を「リポジトリ × ブランチ」で限定する。
# dev のプールは develop、prod のプールは main だけを許す。
# これにより develop への push が prod を触ることは構造的に不可能になる。
resource "google_service_account_iam_member" "workload_identity_user" {
  for_each = toset(var.allowed_refs)

  service_account_id = google_service_account.ci.name
  role               = "roles/iam.workloadIdentityUser"
  member             = "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository_ref/${var.github_repository}@${each.value}"
}

# ---------------------------------------------------------------------------
# 権限（ここが本題）
#
# **roles/run.admin をプロジェクトに付けない。**
# Cloud Run の image は Terraform の ignore_changes 対象で、terraform plan は
# 不正なイメージ差し替えを検出できない。デプロイ権限が広いほど
# 「octant-api SA（Firestore 全アクセス）で動く任意のコード」を
# 静かに動かせる余地が広がるため、次の 3 点で絞っている。
#
#   1. Cloud Run の更新権限は **octant-api サービス 1 つだけ**に付ける
#   2. actAs できる SA は **octant-api SA 1 つだけ**
#   3. プロジェクト全体に付けるのは、上記を成立させるのに必要な最小限だけ
# ---------------------------------------------------------------------------

locals {
  member = "serviceAccount:${google_service_account.ci.email}"

  # "owner/repo" の owner 部分。attribute_condition で使う。
  repository_owner = split("/", var.github_repository)[0]
}

# (1) Cloud Run: **サービス単位**の run.developer。
# 新しいサービスの作成も、他サービスの更新もできない。
resource "google_cloud_run_v2_service_iam_member" "developer" {
  project  = var.project_id
  location = var.region
  name     = var.cloud_run_service_name
  role     = "roles/run.developer"
  member   = local.member
}

# (2) actAs: **octant-api SA だけ**。
# Cloud Run のデプロイには実行 SA への iam.serviceAccounts.actAs が要る。
# プロジェクトレベルで付けると octant-backup など他の SA も引き受けられるため、
# SA リソース単位に限定する。
resource "google_service_account_iam_member" "act_as_runtime" {
  service_account_id = var.runtime_service_account_id
  role               = "roles/iam.serviceAccountUser"
  member             = local.member
}

# (3) デプロイの完了待ち（長時間オペレーションのポーリング）に使う権限。
#
# オペレーションは IAM のバインド対象にできずプロジェクトレベルで評価されるため、
# ここだけはプロジェクトに付けるしかない。roles/run.viewer で足りるが、
# それだと**全 Cloud Run サービスの構成（= 環境変数の中身。ALLOWED_EMAILS は
# 個人のメールアドレス）を読める**ので、権限 1 つだけのカスタムロールにしている。
resource "google_project_iam_custom_role" "run_operation_viewer" {
  project     = var.project_id
  role_id     = "octantCiRunOperationViewer"
  title       = "Octant CI: Cloud Run オペレーション参照"
  description = "デプロイの完了待ちに必要な run.operations.get だけを持つ"
  permissions = ["run.operations.get"]
}

resource "google_project_iam_member" "run_operation_viewer" {
  project = var.project_id
  role    = google_project_iam_custom_role.run_operation_viewer.id
  member  = local.member
}

# Artifact Registry: **リポジトリ単位**の writer（push と pull のみ。削除はできない）。
resource "google_artifact_registry_repository_iam_member" "writer" {
  project    = var.project_id
  location   = var.region
  repository = var.artifact_registry_repository_id
  role       = "roles/artifactregistry.writer"
  member     = local.member
}

# Firebase Hosting のデプロイ。
# サイト単位の IAM は提供されていないためプロジェクトレベル。
# 権限は Hosting のサイト操作と、プロジェクト・アプリのメタ情報の参照に限られる
# （firebase apps:sdkconfig で firebaseConfig を取得するのにも使う）。
resource "google_project_iam_member" "firebase_hosting_admin" {
  project = var.project_id
  role    = "roles/firebasehosting.admin"
  member  = local.member
}

# firestore.rules のデプロイ。
# **CI がルールを毎回リリースし直すことで、全拒否からの退行を検知できる状態にする。**
resource "google_project_iam_member" "firebase_rules_admin" {
  project = var.project_id
  role    = "roles/firebaserules.admin"
  member  = local.member
}

# firebase CLI と gcloud が、このプロジェクトを「クォータの請求先」として
# 使うために要る。API の有効化・無効化はできない。
resource "google_project_iam_member" "service_usage_consumer" {
  project = var.project_id
  role    = "roles/serviceusage.serviceUsageConsumer"
  member  = local.member
}

# ---------------------------------------------------------------------------
# 付けていない権限（意図的な不在。足すときは理由を書くこと）
#
#   roles/run.admin           … サービスの新規作成・IAM 変更ができてしまう
#   roles/editor / owner      … 論外
#   roles/datastore.*         … CI は Firestore のデータに触る必要がない
#   roles/secretmanager.*     … CI は Anthropic API キーを読む必要がない
#   roles/iam.serviceAccountKeyAdmin … キーを発行させない
#   roles/resourcemanager.projectIamAdmin … CI に自分の権限を増やさせない
#   Terraform 実行に要る各種 admin … CI から terraform apply しない方針のため
# ---------------------------------------------------------------------------
