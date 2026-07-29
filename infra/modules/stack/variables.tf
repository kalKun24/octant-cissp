variable "project_id" {
  description = "GCP プロジェクト ID。"
  type        = string
}

variable "env" {
  description = "環境名。dev または prod。APP_ENV とラベルに使う。"
  type        = string

  validation {
    condition     = contains(["dev", "prod"], var.env)
    error_message = "env は dev または prod のいずれかにしてください。"
  }
}

variable "region" {
  description = "Cloud Run・Artifact Registry・Cloud Scheduler のリージョン。"
  type        = string
  default     = "asia-northeast1"
}

variable "firestore_location" {
  description = "Firestore のロケーション。**作成後は変更できない。**"
  type        = string
  default     = "asia-northeast1"
}

variable "allowed_emails" {
  description = "API の利用を許可するメールアドレス。**tfvars から渡す。リポジトリに置かない。**"
  type        = list(string)
  sensitive   = true
}

variable "max_instances" {
  description = "Cloud Run の最大インスタンス数（dev 2 / prod 5）。"
  type        = number
}

variable "api_image" {
  description = <<-EOT
    Cloud Run の初期イメージ。**初回作成のためのプレースホルダ。**
    実イメージへの差し替えは TICKET-005 の CI が行い、Terraform は image の変更を無視する。
  EOT
  type        = string
  default     = "gcr.io/cloudrun/hello"
}

variable "default_uri_disabled" {
  description = <<-EOT
    Cloud Run の *.run.app 直アクセスを塞ぐか。
    **false のまま使う。** dev での実測で Hosting の rewrites ごと落ちることを確認済み
    （詳細は modules/cloud-run-service/variables.tf のコメント）。
  EOT
  type        = bool
  default     = false
}

variable "log_level" {
  description = "API のログレベル。"
  type        = string
  default     = "info"
}

variable "github_repository" {
  description = "デプロイ元の GitHub リポジトリ（owner/repo）。**ここに書いたリポジトリだけが WIF で認証できる。**"
  type        = string
  default     = "kalKun24/octant-cissp"
}

variable "github_repository_owner_id" {
  description = "GitHub アカウントの数値 ID（改名しても変わらない不変の識別子）。gh api users/<owner> --jq .id で確認する。"
  type        = string
  default     = "107126133"
}

variable "ci_allowed_refs" {
  description = <<-EOT
    この環境へのデプロイを許可するブランチの ref。
    dev は refs/heads/develop、prod は refs/heads/main。
    **一時的な検証で広げたら、検証が終わったら必ず戻すこと。**
  EOT
  type        = list(string)
}

variable "billing_account" {
  description = "予算アラートを作る課金アカウント ID。"
  type        = string
}

variable "budget_amount" {
  description = "月あたりの予算額（円）。"
  type        = number
  default     = 1000
}

variable "budget_notification_email" {
  description = "予算アラートの通知先メールアドレス。**tfvars から渡す。**"
  type        = string
  sensitive   = true
}

variable "backup_retention_days" {
  description = "Firestore エクスポートの保持日数。"
  type        = number
  default     = 90
}

variable "image_keep_count" {
  description = "Artifact Registry に残すイメージの世代数。"
  type        = number
  default     = 3
}
