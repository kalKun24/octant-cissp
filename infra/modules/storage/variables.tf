variable "project_id" {
  description = "バケットを作成する GCP プロジェクト ID。"
  type        = string
}

variable "location" {
  description = "バケットのロケーション。単一リージョンにする（マルチリージョンは保存単価が高い）。"
  type        = string
}

variable "attachments_bucket_name" {
  description = "添付画像用バケットの名前。"
  type        = string
}

variable "backups_bucket_name" {
  description = "Firestore エクスポート用バケットの名前。"
  type        = string
}

variable "backup_retention_days" {
  description = "バックアップの保持日数。これを過ぎたオブジェクトはライフサイクルで削除する。"
  type        = number
  default     = 90

  validation {
    condition     = var.backup_retention_days >= 1 && var.backup_retention_days <= 365
    error_message = "backup_retention_days は 1〜365 の範囲で指定してください（保存料の青天井を防ぐため）。"
  }
}

variable "attachments_object_admin_members" {
  description = "添付バケットに対して roles/storage.objectAdmin を付与するプリンシパル（署名付き URL を発行する API 用）。"
  type        = list(string)
  default     = []
}

variable "backups_admin_members" {
  description = <<-EOT
    バックアップバケットに対して roles/storage.admin を付与するプリンシパル。
    Firestore のエクスポートはバケットのメタデータ取得とオブジェクト作成の両方を要求する
    （Google の「データのエクスポートをスケジュール設定する」手順に準拠）。
  EOT
  type        = list(string)
  default     = []
}

variable "labels" {
  description = "リソースに付けるラベル。"
  type        = map(string)
  default     = {}
}
