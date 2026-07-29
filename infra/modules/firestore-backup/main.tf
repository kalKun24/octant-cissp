# Firestore の週次エクスポート（CLAUDE.md「運用」）。
#
# Cloud Scheduler が Firestore の exportDocuments API を直接叩く。
# Cloud Functions を挟まないのは、関数 1 つ分のビルド・保守・課金を増やさないため。
#
# 復元:
#   gcloud firestore import gs://<バケット>/<日時フォルダ> --project <project>

# エクスポートを実行する権限。プロジェクト単位でしか付けられないロール。
resource "google_project_iam_member" "import_export_admin" {
  project = var.project_id
  role    = "roles/datastore.importExportAdmin"
  member  = "serviceAccount:${var.service_account_email}"
}

resource "google_cloud_scheduler_job" "firestore_export" {
  project     = var.project_id
  region      = var.region
  name        = var.job_name
  description = "Firestore を GCS へ週次エクスポートする"
  schedule    = var.schedule
  time_zone   = var.time_zone

  # エクスポートは長時間かかる処理だが、API は Operation を返して即座に応答する。
  attempt_deadline = "320s"

  retry_config {
    retry_count = 1
  }

  http_target {
    http_method = "POST"

    # (default) データベースが対象。名前に括弧を含むが URL としてはそのまま使える。
    uri = "https://firestore.googleapis.com/v1/projects/${var.project_id}/databases/(default):exportDocuments"

    headers = {
      "Content-Type" = "application/json"
    }

    # **outputUriPrefix にパスを付けず、バケット直下を指定している。**
    # Firestore はプレフィックスがバケットのみの場合、開始時刻から
    # 日時フォルダを自動生成する。Scheduler の本文は静的で日時を差し込めないため、
    # この挙動に乗ることで「毎週別フォルダ + 90 日で自動削除」が成立する。
    # パスを付けると毎回同じ場所へ書き、世代が残らない。
    body = base64encode(jsonencode({
      outputUriPrefix = "gs://${var.backups_bucket_name}"
    }))

    oauth_token {
      service_account_email = var.service_account_email
      scope                 = "https://www.googleapis.com/auth/cloud-platform"
    }
  }

  # 権限が無い状態で最初の実行を迎えないよう、IAM の付与を先に済ませる。
  depends_on = [google_project_iam_member.import_export_admin]
}
