output "secret_id" {
  description = "シークレット ID。"
  value       = google_secret_manager_secret.this.secret_id
}

output "secret_name" {
  description = "シークレットの完全な名前（projects/.../secrets/...）。"
  value       = google_secret_manager_secret.this.name
}
