output "budget_name" {
  description = "予算のリソース名。"
  value       = google_billing_budget.monthly.name
}

output "notification_channel_id" {
  description = "予算通知に使う Cloud Monitoring の通知チャンネル ID。"
  value       = google_monitoring_notification_channel.budget_email.id
}
