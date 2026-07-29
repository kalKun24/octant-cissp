# API コンテナのイメージ置き場。
#
# **クリーンアップポリシーは必須**（CLAUDE.md「コスト規約」）。
# 無料枠は 0.5GB/月 で、distroless の API イメージでも数十 MB あるため、
# 毎コミットの push を放置すると数十世代で無料枠を超えて課金が始まる。

resource "google_artifact_registry_repository" "this" {
  project       = var.project_id
  location      = var.location
  repository_id = var.repository_id
  description   = "Octant の API コンテナイメージ（最新 ${var.keep_count} 世代のみ保持）"
  format        = "DOCKER"
  labels        = var.labels

  # true にするとポリシーは評価だけ行い削除しない。**必ず false のまま運用する。**
  cleanup_policy_dry_run = false

  # KEEP は DELETE より優先される。
  # 「タグの有無を問わず全て削除対象にしたうえで、新しい 3 世代だけ残す」
  # という 2 本立てが Artifact Registry の定石。片方だけでは意図した動きにならない。
  cleanup_policies {
    id     = "delete-all"
    action = "DELETE"

    condition {
      tag_state = "ANY"
    }
  }

  cleanup_policies {
    id     = "keep-recent-versions"
    action = "KEEP"

    most_recent_versions {
      keep_count = var.keep_count
    }
  }
}
