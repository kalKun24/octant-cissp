output "cloud_run_service_name" {
  description = "Cloud Run サービス名。"
  value       = module.octant.cloud_run_service_name
}

output "cloud_run_url" {
  description = "Cloud Run の URL。"
  value       = module.octant.cloud_run_url
}

output "artifact_registry_repository_url" {
  description = "イメージの push 先。"
  value       = module.octant.artifact_registry_repository_url
}

output "api_service_account_email" {
  description = "Cloud Run の実行サービスアカウント。"
  value       = module.octant.api_service_account_email
}

output "backups_bucket_name" {
  description = "Firestore エクスポート先バケット。"
  value       = module.octant.backups_bucket_name
}

output "attachments_bucket_name" {
  description = "添付画像バケット。"
  value       = module.octant.attachments_bucket_name
}

output "firestore_backup_job" {
  description = "Firestore の週次エクスポートジョブ。"
  value       = module.octant.firestore_backup_job
}
