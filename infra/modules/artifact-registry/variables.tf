variable "project_id" {
  description = "リポジトリを作成する GCP プロジェクト ID。"
  type        = string
}

variable "location" {
  description = "リポジトリのリージョン。Cloud Run と同じリージョンにする（イメージ取得の遅延と下り課金を避けるため）。"
  type        = string
}

variable "repository_id" {
  description = "Artifact Registry のリポジトリ ID。"
  type        = string
}

variable "keep_count" {
  description = <<-EOT
    保持するイメージの世代数。**コスト規約により既定は 3。**
    無料枠は 0.5GB しかなく、イメージが溜まるとすぐ超過して課金される。
  EOT
  type        = number
  default     = 3

  validation {
    condition     = var.keep_count >= 1 && var.keep_count <= 10
    error_message = "keep_count は 1〜10 の範囲で指定してください（コスト規約により上限を設けています）。"
  }
}

variable "labels" {
  description = "リソースに付けるラベル。"
  type        = map(string)
  default     = {}
}
