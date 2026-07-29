# 予算アラート。
#
# **止められるのは通知だけで、課金そのものは止まらない。**
# それでも、設定ミス（min_instances を上げる、無限ループでの API 呼び出し）に
# 数日で気付けるかどうかは請求額に直結する。

# 予算のフィルタはプロジェクト ID ではなくプロジェクト番号を要求する。
data "google_project" "this" {
  project_id = var.project_id
}

# メール通知のチャンネル。宛先は変数化してリポジトリに残さない。
#
# 注: 新規に作った email チャンネルには Google から確認メールが届く場合がある。
# **届いたら承認すること。** 未確認のままだと通知が配信されないことがある。
resource "google_monitoring_notification_channel" "budget_email" {
  project      = var.project_id
  display_name = "${var.display_name} の通知先"
  type         = "email"

  labels = {
    email_address = var.notification_email
  }
}

resource "google_billing_budget" "monthly" {
  billing_account = var.billing_account
  display_name    = var.display_name

  budget_filter {
    projects = ["projects/${data.google_project.this.number}"]

    # 暦月ごとに実績をリセットする。
    calendar_period = "MONTH"
  }

  amount {
    specified_amount {
      currency_code = var.currency_code
      units         = tostring(var.amount)
    }
  }

  dynamic "threshold_rules" {
    for_each = var.threshold_percents
    content {
      threshold_percent = threshold_rules.value
      spend_basis       = "CURRENT_SPEND"
    }
  }

  all_updates_rule {
    monitoring_notification_channels = [google_monitoring_notification_channel.budget_email.id]

    # false のままにして、課金管理者（IAM）にも既定の通知を残す。
    # チャンネル側の設定ミスで通知が全滅する事態を避けるための二重化。
    disable_default_iam_recipients = false
  }
}
