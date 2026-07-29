output "attachments_bucket_name" {
  description = "添付画像バケットの名前。"
  value       = google_storage_bucket.attachments.name
}

output "attachments_bucket_url" {
  description = "添付画像バケットの gs:// URL。"
  value       = google_storage_bucket.attachments.url
}

output "backups_bucket_name" {
  description = "バックアップバケットの名前。"
  value       = google_storage_bucket.backups.name
}

output "backups_bucket_url" {
  description = "バックアップバケットの gs:// URL。復元は gcloud firestore import <この URL>/<フォルダ>。"
  value       = google_storage_bucket.backups.url
}
