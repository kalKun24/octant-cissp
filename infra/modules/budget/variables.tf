variable "project_id" {
  description = "予算の対象プロジェクト。通知チャンネルもこのプロジェクトに作る。"
  type        = string
}

variable "billing_account" {
  description = "課金アカウント ID。**予算は課金アカウント配下のリソース**であり、プロジェクト配下ではない。"
  type        = string
}

variable "display_name" {
  description = "予算の表示名。"
  type        = string
}

variable "amount" {
  description = "月あたりの予算額（通貨の最小単位ではなく通貨単位）。"
  type        = number
}

variable "currency_code" {
  description = "予算の通貨。**課金アカウントの通貨と一致しないと作成に失敗する。**"
  type        = string
  default     = "JPY"
}

variable "threshold_percents" {
  description = "通知するしきい値（実績額に対する割合）。"
  type        = list(number)
  default     = [0.5, 0.9, 1.0]
}

variable "notification_email" {
  description = "予算超過を通知するメールアドレス。**リポジトリに直書きせず tfvars から渡す。**"
  type        = string
  sensitive   = true
}
