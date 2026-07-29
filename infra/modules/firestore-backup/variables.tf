variable "project_id" {
  description = "対象の GCP プロジェクト ID。"
  type        = string
}

variable "region" {
  description = "Cloud Scheduler のリージョン。"
  type        = string
}

variable "job_name" {
  description = "Cloud Scheduler のジョブ名。"
  type        = string
  default     = "firestore-weekly-export"
}

variable "service_account_email" {
  description = "エクスポート API を呼ぶサービスアカウント。roles/datastore.importExportAdmin をこのモジュールが付与する。"
  type        = string
}

variable "backups_bucket_name" {
  description = "エクスポート先のバケット名。**バケット直下を指定する**（後述のとおり日時フォルダが自動生成される）。"
  type        = string
}

variable "schedule" {
  description = "cron 形式のスケジュール。既定は毎週月曜の 03:00。"
  type        = string
  default     = "0 3 * * 1"
}

variable "time_zone" {
  description = "スケジュールのタイムゾーン。"
  type        = string
  default     = "Asia/Tokyo"
}
