output "workload_identity_provider" {
  description = <<-EOT
    GitHub Actions の google-github-actions/auth に渡す provider のリソース名。
    **秘密情報ではない**（リポジトリとブランチが一致しなければ使えない）。
  EOT
  value       = google_iam_workload_identity_pool_provider.github.name
}

output "service_account_email" {
  description = "GitHub Actions が借用するデプロイ用 SA。"
  value       = google_service_account.ci.email
}

output "allowed_principals" {
  description = "SA の借用を許可した principalSet（リポジトリ × ブランチ）。"
  value       = [for r in var.allowed_refs : "principalSet://iam.googleapis.com/${google_iam_workload_identity_pool.github.name}/attribute.repository_ref/${var.github_repository}@${r}"]
}
