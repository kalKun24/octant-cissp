variable "project_id" {
  description = "Firestore を作成する GCP プロジェクト ID。"
  type        = string
}

variable "location_id" {
  description = "Firestore のロケーション。**作成後は変更できない**（変更すると再作成になる）。"
  type        = string
}

variable "datastore_user_members" {
  description = <<-EOT
    roles/datastore.user を付与するプリンシパルの一覧（"serviceAccount:..." 形式）。
    Firestore の読み書きはプロジェクト単位のロールでしか制御できないため、
    データベース側ではなくプロジェクト IAM への追加になる。
  EOT
  type        = list(string)
  default     = []
}
