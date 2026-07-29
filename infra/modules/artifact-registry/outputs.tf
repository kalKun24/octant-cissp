output "repository_id" {
  description = "リポジトリ ID。"
  value       = google_artifact_registry_repository.this.repository_id
}

output "repository_url" {
  description = "docker push の宛先（<location>-docker.pkg.dev/<project>/<repo>）。TICKET-005 の CI が使う。"
  value       = "${google_artifact_registry_repository.this.location}-docker.pkg.dev/${var.project_id}/${google_artifact_registry_repository.this.repository_id}"
}
