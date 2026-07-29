variable "project_id" {
  description = "WIF プールとデプロイ用 SA を作る GCP プロジェクト ID。"
  type        = string
}

variable "env" {
  description = "環境名（表示名に使う）。"
  type        = string
}

variable "region" {
  description = "Cloud Run と Artifact Registry のリージョン。IAM をリソース単位で付けるために要る。"
  type        = string
}

variable "github_repository" {
  description = "デプロイを許可する GitHub リポジトリ（owner/repo）。**ここに書いた 1 つだけがプールに入れる。**"
  type        = string

  validation {
    condition     = can(regex("^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$", var.github_repository))
    error_message = "github_repository は owner/repo の形式で指定してください。"
  }
}

variable "allowed_refs" {
  description = <<-EOT
    デプロイ用 SA の借用を許可する Git の ref。**ブランチまで固定する。**
    dev は refs/heads/develop、prod は refs/heads/main のみを渡す。
    ここを広げると、任意のブランチに push した内容がその環境へ出る。
  EOT
  type        = list(string)

  validation {
    condition     = length(var.allowed_refs) > 0
    error_message = "allowed_refs は 1 件以上必要です（0 件では CI が認証できません）。"
  }

  validation {
    condition     = alltrue([for r in var.allowed_refs : startswith(r, "refs/heads/")])
    error_message = "allowed_refs はブランチの完全な ref（refs/heads/...）で指定してください。タグからのデプロイは許可しません。"
  }
}

variable "cloud_run_service_name" {
  description = "デプロイ先の Cloud Run サービス名。**この 1 つにだけ更新権限を付ける。**"
  type        = string
}

variable "runtime_service_account_id" {
  description = "Cloud Run の実行 SA のリソース ID（projects/.../serviceAccounts/...）。**この 1 つにだけ actAs を許す。**"
  type        = string
}

variable "artifact_registry_repository_id" {
  description = "イメージを push する Artifact Registry のリポジトリ ID。"
  type        = string
}

variable "pool_id" {
  description = "Workload Identity プール ID。**削除しても 30 日間は同じ ID を再利用できないため変更しない。**"
  type        = string
  default     = "github"
}

variable "provider_id" {
  description = "Workload Identity プロバイダ ID。**同上、変更しない。**"
  type        = string
  default     = "github-actions"
}

variable "service_account_id" {
  description = "デプロイ用サービスアカウントの ID。"
  type        = string
  default     = "octant-ci"
}
