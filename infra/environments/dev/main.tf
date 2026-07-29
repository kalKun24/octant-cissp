# dev 環境（GCP プロジェクト octant-dev）。
#
# **このファイルは値だけを持つ。** 構成は infra/modules/stack にある。
# dev と prod で構成が食い違わないようにするため、リソースをここに直接書かない。

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
    bucket = "octant-dev-tfstate"
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
  project_id = "octant-dev"
  region     = "asia-northeast1"

  # 課金アカウント ID。秘密情報ではない（docs/gcp-bootstrap.md にも記載がある）が、
  # 取り違えると別の請求先に予算を作ってしまうため定数として置く。
  billing_account = "013D28-C16A68-E3B1DA"
}

module "octant" {
  source = "../../modules/stack"

  project_id         = local.project_id
  env                = "dev"
  region             = local.region
  firestore_location = local.region

  # コスト規約: dev は 2。min_instances は 0 固定（モジュール側で変数にしていない）。
  max_instances = 2

  # dev は挙動の確認が目的なのでログを詳しくする。
  log_level = "debug"

  billing_account           = local.billing_account
  budget_amount             = 1000
  budget_notification_email = var.budget_notification_email

  allowed_emails = var.allowed_emails
}
