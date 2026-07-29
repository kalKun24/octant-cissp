variable "project_id" {
  description = "シークレットを作成する GCP プロジェクト ID。"
  type        = string
}

variable "location" {
  description = "シークレットを複製するリージョン。"
  type        = string
}

variable "secret_id" {
  description = "シークレット ID。"
  type        = string
}

variable "accessor_members" {
  description = "roles/secretmanager.secretAccessor を付与するプリンシパル（Cloud Run の実行サービスアカウント）。"
  type        = list(string)
  default     = []
}

variable "labels" {
  description = "リソースに付けるラベル。"
  type        = map(string)
  default     = {}
}
