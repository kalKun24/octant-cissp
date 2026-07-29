output "job_name" {
  description = "Cloud Scheduler のジョブ名。手動実行は gcloud scheduler jobs run <名前> --location <region>。"
  value       = google_cloud_scheduler_job.firestore_export.name
}

output "schedule" {
  description = "エクスポートのスケジュール。"
  value       = "${google_cloud_scheduler_job.firestore_export.schedule} (${google_cloud_scheduler_job.firestore_export.time_zone})"
}
