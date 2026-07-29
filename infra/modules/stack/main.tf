# 1 環境分の Octant を構成するモジュール。
#
# environments/dev と environments/prod はこのモジュールを呼ぶだけで、
# **環境ごとの違いは値だけ**になる（構成の差分がレビューで見落とされないようにするため）。
#
# ここが持つのは「モジュールをまたぐ配線」だけ:
#   - サービスアカウントの定義（作る場所を 1 箇所にして循環参照を避ける）
#   - プロジェクト単位でしか付けられない IAM

locals {
  service_name  = "octant-api"
  repository_id = "octant"

  labels = {
    app        = "octant"
    env        = var.env
    managed-by = "terraform"
  }
}

# ---------------------------------------------------------------------------
# サービスアカウント
#
# **キー（JSON）は発行しない。** Cloud Run は SA を引き受けて動き、
# CI は Workload Identity Federation（TICKET-005）で権限借用する。
# ---------------------------------------------------------------------------

# Cloud Run の実行 SA。既定のコンピュート SA（編集者相当）を使わないためのもの。
resource "google_service_account" "api" {
  project      = var.project_id
  account_id   = "octant-api"
  display_name = "Octant API (Cloud Run 実行用)"
  description  = "Cloud Run の API コンテナが引き受けるサービスアカウント"
}

# 週次エクスポートを実行する SA。API 用とは分けて、権限の混在を避ける。
resource "google_service_account" "backup" {
  project      = var.project_id
  account_id   = "octant-backup"
  display_name = "Octant Firestore バックアップ"
  description  = "Cloud Scheduler が Firestore のエクスポートを呼ぶためのサービスアカウント"
}

# 構造化ログの書き込み。カスタム SA を使う場合は明示的な付与が必要。
resource "google_project_iam_member" "api_log_writer" {
  project = var.project_id
  role    = "roles/logging.logWriter"
  member  = "serviceAccount:${google_service_account.api.email}"
}

# ---------------------------------------------------------------------------
# データ層
# ---------------------------------------------------------------------------

module "firestore" {
  source = "../firestore"

  project_id  = var.project_id
  location_id = var.firestore_location

  # 読み書きするのは API だけ。ブラウザは REST API 経由でしか触れない
  # （firestore.rules は全拒否）。
  datastore_user_members = ["serviceAccount:${google_service_account.api.email}"]
}

module "storage" {
  source = "../storage"

  project_id              = var.project_id
  location                = var.region
  attachments_bucket_name = "${var.project_id}-attachments"
  backups_bucket_name     = "${var.project_id}-backups"
  backup_retention_days   = var.backup_retention_days
  labels                  = local.labels

  attachments_object_admin_members = ["serviceAccount:${google_service_account.api.email}"]
  backups_admin_members            = ["serviceAccount:${google_service_account.backup.email}"]
}

# ---------------------------------------------------------------------------
# アプリケーション層
# ---------------------------------------------------------------------------

module "artifact_registry" {
  source = "../artifact-registry"

  project_id    = var.project_id
  location      = var.region
  repository_id = local.repository_id
  keep_count    = var.image_keep_count
  labels        = local.labels
}

module "secret" {
  source = "../secret"

  project_id       = var.project_id
  location         = var.region
  secret_id        = "anthropic-api-key"
  accessor_members = ["serviceAccount:${google_service_account.api.email}"]
  labels           = local.labels
}

module "cloud_run" {
  source = "../cloud-run-service"

  project_id            = var.project_id
  region                = var.region
  service_name          = local.service_name
  service_account_email = google_service_account.api.email
  image                 = var.api_image
  labels                = local.labels

  # 起動に必須の 3 つ。欠けるとリビジョンが立ち上がらない。
  app_env             = var.env
  firebase_project_id = var.project_id
  allowed_emails      = var.allowed_emails
  log_level           = var.log_level

  anthropic_api_key_secret_id = module.secret.secret_id
  max_instances               = var.max_instances
  default_uri_disabled        = var.default_uri_disabled

  # シークレットの参照権限が無いままサービスを作るとリビジョンの起動に失敗する。
  # モジュール単位で順序を固定しておく。
  depends_on = [module.secret]
}

# ---------------------------------------------------------------------------
# CI/CD
# ---------------------------------------------------------------------------

module "ci" {
  source = "../ci-workload-identity"

  project_id = var.project_id
  env        = var.env
  region     = var.region

  github_repository          = var.github_repository
  github_repository_owner_id = var.github_repository_owner_id
  allowed_refs               = var.ci_allowed_refs

  # デプロイ権限はこの 2 つのリソースにだけ紐づける
  # （権限設計の理由はモジュール側のコメントを参照）。
  cloud_run_service_name     = module.cloud_run.service_name
  runtime_service_account_id = google_service_account.api.name

  artifact_registry_repository_id = module.artifact_registry.repository_id
}

# ---------------------------------------------------------------------------
# 運用
# ---------------------------------------------------------------------------

module "firestore_backup" {
  source = "../firestore-backup"

  project_id            = var.project_id
  region                = var.region
  service_account_email = google_service_account.backup.email
  backups_bucket_name   = module.storage.backups_bucket_name

  # データベースが無い状態でジョブが起動しても失敗するだけなので、
  # 作成順を固定しておく。
  depends_on = [module.firestore]
}

module "budget" {
  source = "../budget"

  project_id         = var.project_id
  billing_account    = var.billing_account
  display_name       = "Octant ${var.env} 月次予算"
  amount             = var.budget_amount
  notification_email = var.budget_notification_email
}
