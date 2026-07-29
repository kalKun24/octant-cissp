# API を動かす Cloud Run サービス。
#
# 固定費ゼロを維持するための要点（CLAUDE.md「コスト規約」）:
#   - min_instance_count は 0。**変数にすらしていない。** 1 にするとアイドル課金で月 2,000 円規模
#   - max_instance_count に上限を設けて暴走時の青天井を防ぐ
#   - cpu_idle = true（リクエスト処理中だけ課金する既定の課金モデル）
#   - 外部ロードバランサは使わない。公開は Cloud Run の URL と Firebase Hosting の rewrites だけ

locals {
  # 必須の 3 つ + ログレベル。**この 3 つが欠けるとリビジョンは起動に失敗する。**
  # backend/internal/config が既定値で埋めずに起動失敗させる設計になっているため、
  # 設定漏れはデプロイ時に必ず表面化する。
  #
  # **FIREBASE_AUTH_EMULATOR_HOST は絶対に注入しない。**
  # 設定されていると APP_ENV が dev / prod のとき起動を拒否する
  # （エミュレータ接続時は ID トークンの署名検証が省略されるため）。
  #
  # 注: ALLOWED_EMAILS が機微な値のため、Terraform は env ブロック全体を
  # 機微扱いし、**plan には環境変数の中身が一切表示されない**（コレクション単位で
  # 伝播するため、分けても同じ）。個人のメールアドレスを plan のログや
  # PR のコメントへ流出させないことを優先している。
  # 適用後の実値は次で確認する:
  #   gcloud run services describe octant-api --region asia-northeast1 \
  #     --format='value(spec.template.spec.containers[0].env)'
  env_vars = merge({
    APP_ENV             = var.app_env
    FIREBASE_PROJECT_ID = var.firebase_project_id
    ALLOWED_EMAILS      = join(",", var.allowed_emails)
    LOG_LEVEL           = var.log_level
  }, var.extra_env_vars)
}

resource "google_cloud_run_v2_service" "this" {
  project  = var.project_id
  name     = var.service_name
  location = var.region
  labels   = var.labels

  # Firebase Hosting は公開エンドポイント経由で転送するため内部限定にはできない。
  ingress             = "INGRESS_TRAFFIC_ALL"
  deletion_protection = var.deletion_protection

  # *.run.app の既定 URL を塞ぐかどうか。
  # **Firebase Hosting の rewrites 経由でしか到達させたくない**場合に true にする。
  # 挙動は環境ごとに検証してから有効にすること（variables.tf のコメントを参照）。
  default_uri_disabled = var.default_uri_disabled

  template {
    service_account                  = var.service_account_email
    max_instance_request_concurrency = var.concurrency
    timeout                          = "${var.request_timeout_seconds}s"

    scaling {
      # **0 を厳守。** アイドル中のインスタンスを常駐させない。
      min_instance_count = 0
      max_instance_count = var.max_instances
    }

    containers {
      image = var.image

      ports {
        container_port = 8080
      }

      resources {
        limits = {
          cpu    = var.cpu
          memory = var.memory
        }

        # リクエストが無い間は CPU を割り当てない（= 課金しない）。
        # false にすると常時 CPU 課金になり、min_instances = 0 の意味が薄れる。
        cpu_idle = true

        # 起動時だけ CPU を増やす。追加料金は無く、コールドスタートが短くなる。
        startup_cpu_boost = true
      }

      dynamic "env" {
        for_each = local.env_vars
        content {
          name  = env.key
          value = env.value
        }
      }

      # Anthropic API キーは環境変数の平文ではなく Secret Manager から渡す。
      # **値が Terraform の state にも Cloud Run の設定にも残らない。**
      env {
        name = "ANTHROPIC_API_KEY"
        value_source {
          secret_key_ref {
            secret  = var.anthropic_api_key_secret_id
            version = "latest"
          }
        }
      }

      # 起動プローブ。TCP でも足りるが、ルータが応答することまで確認したいので
      # HTTP にしてある。**パスは /api/health**（/health は 404）。
      startup_probe {
        http_get {
          path = var.health_check_path
        }
        initial_delay_seconds = 0
        timeout_seconds       = 3
        period_seconds        = 5
        failure_threshold     = 6
      }
    }
  }

  traffic {
    type    = "TRAFFIC_TARGET_ALLOCATION_TYPE_LATEST"
    percent = 100
  }

  lifecycle {
    ignore_changes = [
      # **イメージは Terraform の管理対象から外す。**
      # 実際のデプロイは TICKET-005 の CI が行い、コミット SHA のタグを持つ
      # イメージへ差し替える。ここで無視しないと、次の terraform apply が
      # 本番を初期プレースホルダ（gcr.io/cloudrun/hello）へ巻き戻してしまう。
      # 「Terraform は構成を、CI はイメージを持つ」という分担にする。
      template[0].containers[0].image,

      # gcloud / GitHub Actions からデプロイすると付く発行元情報。
      # 差分として毎回出るだけで意味が無いので無視する。
      client,
      client_version,
    ]
  }
}

# Firebase Hosting の rewrites から転送できるようにする。
# **認可はアプリ側の 1 箇所（ID トークン検証 + 許可メール）で行う設計**であり、
# ここで IAM 認証を要求すると Hosting からの転送が 403 になる。
resource "google_cloud_run_v2_service_iam_member" "public_invoker" {
  count = var.allow_unauthenticated ? 1 : 0

  project  = google_cloud_run_v2_service.this.project
  location = google_cloud_run_v2_service.this.location
  name     = google_cloud_run_v2_service.this.name
  role     = "roles/run.invoker"
  member   = "allUsers"
}
