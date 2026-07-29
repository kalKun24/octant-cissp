# Anthropic API キーの入れ物。
#
# **実際のキーは Terraform で管理しない。** tfvars にも state にも本物を置かない。
# 値の投入は運用作業として手で行う:
#
#   printf '%s' "$ANTHROPIC_API_KEY" | \
#     gcloud secrets versions add anthropic-api-key --project <project> --data-file=-
#
# Cloud Run は latest を参照するので、新しいバージョンを足せば
# 次に起動するインスタンスから新しい値が使われる。

resource "google_secret_manager_secret" "this" {
  project   = var.project_id
  secret_id = var.secret_id
  labels    = var.labels

  # automatic ではなくリージョン指定にして、保管先を東京に固定する。
  replication {
    user_managed {
      replicas {
        location = var.location
      }
    }
  }
}

# 初期バージョン（プレースホルダ）。
#
# **これが無いと Cloud Run のリビジョンが起動できない。**
# secretKeyRef が latest を解決できず「Secret ... was not found」で
# デプロイそのものが失敗するため、空の入れ物だけでは足りない。
#
# 値は本物のキーではなく、投入漏れが AI 機能の 4xx として現れる文字列にしてある。
# 手で追加した本物のバージョン（v2 以降）は Terraform が触らないので、
# apply でプレースホルダに巻き戻ることはない。
resource "google_secret_manager_secret_version" "placeholder" {
  secret      = google_secret_manager_secret.this.id
  secret_data = "PLACEHOLDER-SET-VIA-GCLOUD"

  lifecycle {
    # 手で無効化・破棄したプレースホルダを apply で作り直さない。
    ignore_changes = [secret_data, enabled]
  }
}

resource "google_secret_manager_secret_iam_member" "accessor" {
  for_each = toset(var.accessor_members)

  project   = google_secret_manager_secret.this.project
  secret_id = google_secret_manager_secret.this.secret_id
  role      = "roles/secretmanager.secretAccessor"
  member    = each.value
}
