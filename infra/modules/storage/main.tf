# Cloud Storage のバケット 2 つ。
#
#   attachments … ノートの添付画像（v2 の機能。署名付き URL で読み書きする）
#   backups     … Firestore の週次エクスポート先
#
# tfstate 用バケットはここでは作らない。**Terraform は自分の state 置き場を
# 同じ apply では作れない**ため、TICKET-016 のブートストラップで作成済み。

locals {
  # 公開防止と均一アクセス制御は 2 つのバケットで共通。
  # ACL を残すと「バケットは非公開なのにオブジェクトが公開」という事故が起きる。
  common = {
    uniform_bucket_level_access = true
    public_access_prevention    = "enforced"
  }
}

resource "google_storage_bucket" "attachments" {
  project                     = var.project_id
  name                        = var.attachments_bucket_name
  location                    = var.location
  storage_class               = "STANDARD"
  uniform_bucket_level_access = local.common.uniform_bucket_level_access
  public_access_prevention    = local.common.public_access_prevention
  labels                      = var.labels

  # 中身があるバケットを terraform destroy で消させない。
  force_destroy = false

  # **ソフト削除は既定 7 日で、削除済みオブジェクトも保存料の課金対象になる。**
  # 添付は Firestore 側のメタデータから復元でき、二重の保険は不要なので無効化する。
  soft_delete_policy {
    retention_duration_seconds = 0
  }

  # 中断したマルチパートアップロードの残骸は課金されるだけで価値がない。
  lifecycle_rule {
    action {
      type = "AbortIncompleteMultipartUpload"
    }
    condition {
      age = 7
    }
  }
}

resource "google_storage_bucket" "backups" {
  project                     = var.project_id
  name                        = var.backups_bucket_name
  location                    = var.location
  storage_class               = "STANDARD"
  uniform_bucket_level_access = local.common.uniform_bucket_level_access
  public_access_prevention    = local.common.public_access_prevention
  labels                      = var.labels

  force_destroy = false

  soft_delete_policy {
    retention_duration_seconds = 0
  }

  # 保持期間を過ぎたエクスポートを削除する（CLAUDE.md「コスト規約」）。
  # 週次エクスポートは毎回タイムスタンプ付きのフォルダに書かれるため、
  # このルールが無いと古い世代が無限に積み上がる。
  lifecycle_rule {
    action {
      type = "Delete"
    }
    condition {
      age = var.backup_retention_days
    }
  }

  lifecycle_rule {
    action {
      type = "AbortIncompleteMultipartUpload"
    }
    condition {
      age = 7
    }
  }
}

# バケット単位の IAM。プロジェクト全体の storage ロールは付けない
# （tfstate バケットまで読めてしまうため）。
resource "google_storage_bucket_iam_member" "attachments_object_admin" {
  for_each = toset(var.attachments_object_admin_members)

  bucket = google_storage_bucket.attachments.name
  role   = "roles/storage.objectAdmin"
  member = each.value
}

# バックアップ書き込みに必要な最小権限。
#
# **roles/storage.admin を使わない。** バケットスコープでも
# storage.buckets.setIamPolicy / objects.delete / buckets.delete を含むため、
# この SA が侵害されると次の連鎖が成立する:
#   1. 全バックアップを削除（ソフト削除もバージョニングも無いため復旧不能）
#   2. setIamPolicy で外部アカウントに閲覧権を付与しノート本文を持ち出す
#      （public_access_prevention は allUsers を防ぐが特定アカウントは防げない）
#
# Firestore の exportDocuments に実際に要るのは
# 「オブジェクトの作成」と「バケットの存在確認」だけ。
resource "google_storage_bucket_iam_member" "backups_object_creator" {
  for_each = toset(var.backups_admin_members)

  bucket = google_storage_bucket.backups.name
  role   = "roles/storage.objectCreator"
  member = each.value
}

resource "google_storage_bucket_iam_member" "backups_bucket_reader" {
  for_each = toset(var.backups_admin_members)

  bucket = google_storage_bucket.backups.name
  role   = "roles/storage.legacyBucketReader" # storage.buckets.get のみ
  member = each.value
}
