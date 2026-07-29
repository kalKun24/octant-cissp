# prod 環境（GCP プロジェクト octant-prod）。
#
# **このファイルは値だけを持つ。** 構成は infra/modules/stack にある。
# dev との差は max_instances とログレベルと APP_ENV だけであり、
# それ以外が食い違ったら構成のバグとして扱う。

terraform {
  required_version = "~> 1.9"

  required_providers {
    google = {
      source  = "hashicorp/google"
      version = "~> 7.0"
    }
  }

  # TICKET-016 で作成済みのバケット（バージョニング有効）。
  # **Terraform は自分の state 置き場を同じ apply では作れない**ため、
  # ここでバケットを作らない。
  backend "gcs" {
    bucket = "octant-prod-tfstate"
    prefix = "terraform/state"
  }
}

provider "google" {
  project = local.project_id
  region  = local.region

  # ADC のクォータプロジェクト設定に依存しないための明示。
  # Identity Toolkit など一部の API は「どのプロジェクトに割り当てるか」を
  # 要求し、未設定だと API 無効と紛らわしい 403 になる。
  billing_project       = local.project_id
  user_project_override = true
}

locals {
  project_id = "octant-prod"
  region     = "asia-northeast1"

  # 課金アカウント ID。秘密情報ではない（docs/gcp-bootstrap.md にも記載がある）が、
  # 取り違えると別の請求先に予算を作ってしまうため定数として置く。
  billing_account = "013D28-C16A68-E3B1DA"
}

module "octant" {
  source = "../../modules/stack"

  project_id         = local.project_id
  env                = "prod"
  region             = local.region
  firestore_location = local.region

  # コスト規約: prod は 5。min_instances は 0 固定（モジュール側で変数にしていない）。
  max_instances = 5

  log_level = "info"

  billing_account           = local.billing_account
  budget_amount             = 1000
  budget_notification_email = var.budget_notification_email

  allowed_emails = var.allowed_emails
}
