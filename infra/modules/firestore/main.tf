# Firestore（Native モード）のデータベース。
#
# **このモジュールが唯一の作成経路である。** コンソールの「使ってみる」で作ると
# ロケーション・PITR・削除保護が Terraform の管理外に落ちるため、
# TICKET-016（ブートストラップ）では意図的に未作成にしてある。

resource "google_firestore_database" "default" {
  project = var.project_id

  # (default) 以外の名前にすると Firebase コンソールや Admin SDK の既定の
  # 参照先とずれる。アプリは既定データベースだけを使う。
  name        = "(default)"
  location_id = var.location_id
  type        = "FIRESTORE_NATIVE"

  # App Engine とは無関係のプロジェクトのため統合は無効。
  app_engine_integration_mode = "DISABLED"
  concurrency_mode            = "OPTIMISTIC"

  # 運用要件（CLAUDE.md「運用」）。
  # PITR は直近 7 日間の任意時点へ読み戻せる。誤削除の一次防御。
  point_in_time_recovery_enablement = "POINT_IN_TIME_RECOVERY_ENABLED"

  # 削除保護。**有効な間は API からもコンソールからも削除できない。**
  delete_protection_state = "DELETE_PROTECTION_ENABLED"

  # terraform destroy でも Firestore は残す。state から外れるだけにする。
  # （データベースの削除は復旧できないため、明示的な手作業に限定する）
  deletion_policy = "ABANDON"
}

# Cloud Run の実行サービスアカウントに読み書きを許可する。
# roles/datastore.user はコレクション単位に絞れないため、
# 「サーバだけが Firestore に触る」という設計（firestore.rules は全拒否）に依存する。
resource "google_project_iam_member" "datastore_user" {
  for_each = toset(var.datastore_user_members)

  project = var.project_id
  role    = "roles/datastore.user"
  member  = each.value
}
