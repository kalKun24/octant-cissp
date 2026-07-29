output "cloud_run_service_name" {
  description = "Cloud Run サービス名（Firebase Hosting の rewrites と CI のデプロイ先）。"
  value       = module.cloud_run.service_name
}

output "cloud_run_url" {
  description = "Cloud Run の URL。疎通確認は <URL>/api/health。"
  value       = module.cloud_run.service_url
}

output "api_service_account_email" {
  description = "Cloud Run の実行サービスアカウント。"
  value       = google_service_account.api.email
}

output "backup_service_account_email" {
  description = "Firestore エクスポート用サービスアカウント。"
  value       = google_service_account.backup.email
}

output "artifact_registry_repository_url" {
  description = "docker push の宛先。イメージは <この URL>/api:<コミット SHA>。"
  value       = module.artifact_registry.repository_url
}

output "ci_workload_identity_provider" {
  description = "GitHub Actions の認証に使う WIF プロバイダのリソース名。"
  value       = module.ci.workload_identity_provider
}

output "ci_service_account_email" {
  description = "GitHub Actions が借用するデプロイ用サービスアカウント。"
  value       = module.ci.service_account_email
}

output "ci_allowed_principals" {
  description = "デプロイ用 SA の借用を許可した principalSet（リポジトリ × ブランチ）。"
  value       = module.ci.allowed_principals
}

output "attachments_bucket_name" {
  description = "添付画像バケット。"
  value       = module.storage.attachments_bucket_name
}

output "backups_bucket_name" {
  description = "Firestore エクスポート先バケット。"
  value       = module.storage.backups_bucket_name
}

output "firestore_database_name" {
  description = "Firestore データベース名。"
  value       = module.firestore.database_name
}

output "anthropic_api_key_secret_id" {
  description = "Anthropic API キーのシークレット ID。値の投入は gcloud secrets versions add で行う。"
  value       = module.secret.secret_id
}

output "firestore_backup_job" {
  description = "Firestore の週次エクスポートジョブ。"
  value       = "${module.firestore_backup.job_name} (${module.firestore_backup.schedule})"
}
