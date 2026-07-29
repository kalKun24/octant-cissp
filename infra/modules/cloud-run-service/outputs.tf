output "service_name" {
  description = "Cloud Run サービス名。Firebase Hosting の rewrites と CI のデプロイ先に使う。"
  value       = google_cloud_run_v2_service.this.name
}

output "service_url" {
  description = "Cloud Run の URL。疎通確認は <URL>/api/health。"
  value       = google_cloud_run_v2_service.this.uri
}

output "location" {
  description = "Cloud Run のリージョン。"
  value       = google_cloud_run_v2_service.this.location
}
