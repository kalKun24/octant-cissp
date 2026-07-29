# 個人情報にあたる値だけを変数にしている。
# **terraform.tfvars（.gitignore 済み）に書く。** 例は terraform.tfvars.example を参照。

variable "allowed_emails" {
  description = "API の利用を許可するメールアドレスの一覧。"
  type        = list(string)
  sensitive   = true
}

variable "budget_notification_email" {
  description = "予算アラートの通知先メールアドレス。"
  type        = string
  sensitive   = true
}
