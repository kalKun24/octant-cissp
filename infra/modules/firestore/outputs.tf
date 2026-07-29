output "database_name" {
  description = "Firestore データベースのリソース名。"
  value       = google_firestore_database.default.name
}

output "database_id" {
  description = "Firestore データベースの完全な ID（projects/.../databases/...）。"
  value       = google_firestore_database.default.id
}

output "location_id" {
  description = "Firestore のロケーション。"
  value       = google_firestore_database.default.location_id
}
