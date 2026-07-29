variable "project_id" {
  description = "サービスを作成する GCP プロジェクト ID。"
  type        = string
}

variable "region" {
  description = "Cloud Run のリージョン。"
  type        = string
}

variable "service_name" {
  description = "Cloud Run サービス名。Firebase Hosting の rewrites から参照される。"
  type        = string
}

variable "service_account_email" {
  description = "コンテナの実行サービスアカウント。**既定のコンピュートサービスアカウント（編集者権限）を使わない。**"
  type        = string
}

variable "image" {
  description = <<-EOT
    コンテナイメージ。**初回作成用のプレースホルダを渡す。**
    実際のイメージは TICKET-005 の CI が差し替えるため、
    このモジュールは image の変更を無視する（main.tf の lifecycle を参照）。
  EOT
  type        = string
}

# ---------------------------------------------------------------------------
# アプリの必須環境変数
#
# **この 3 つが欠けるとリビジョンは起動しない**（backend/internal/config の設計）。
# 既定値を持たせず必須の入力にしてあるのは、渡し忘れを apply 前に発見するため。
# ---------------------------------------------------------------------------

variable "app_env" {
  description = "APP_ENV。dev または prod。**local を入れない**（Auth エミュレータ混入ガードが無効になる）。"
  type        = string

  validation {
    condition     = contains(["dev", "prod"], var.app_env)
    error_message = "app_env は dev または prod のいずれかにしてください（local は Cloud Run では使いません）。"
  }
}

variable "firebase_project_id" {
  description = "FIREBASE_PROJECT_ID。ID トークンの iss / aud の照合に使う。"
  type        = string

  validation {
    condition     = length(trimspace(var.firebase_project_id)) > 0
    error_message = "firebase_project_id は必須です（未設定だとリビジョンが起動しません）。"
  }
}

variable "allowed_emails" {
  description = "ALLOWED_EMAILS に入れる許可メールアドレスの一覧。**リポジトリに直書きせず tfvars から渡す。**"
  type        = list(string)
  sensitive   = true

  validation {
    condition     = length(var.allowed_emails) > 0
    error_message = "allowed_emails は 1 件以上必要です（0 件では API が起動しません）。"
  }

  validation {
    condition     = alltrue([for e in var.allowed_emails : can(regex("^[^,@[:space:]]+@[^,@[:space:]]+$", e))])
    error_message = "allowed_emails の各要素はカンマと空白を含まないメールアドレスにしてください。"
  }
}

variable "log_level" {
  description = "LOG_LEVEL。debug / info / warn / error。"
  type        = string
  default     = "info"

  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "log_level は debug / info / warn / error のいずれかにしてください。"
  }
}

variable "anthropic_api_key_secret_id" {
  description = "ANTHROPIC_API_KEY として注入する Secret Manager のシークレット ID。"
  type        = string
}

variable "extra_env_vars" {
  description = "追加で注入する環境変数。**秘密情報はここに入れない**（シークレットは secret_key_ref 経由）。"
  type        = map(string)
  default     = {}
}

# ---------------------------------------------------------------------------
# スケーリングとリソース
# ---------------------------------------------------------------------------

variable "max_instances" {
  description = "最大インスタンス数。暴走時の青天井を防ぐ上限（CLAUDE.md「コスト規約」: dev 2 / prod 5）。"
  type        = number

  validation {
    condition     = var.max_instances >= 1 && var.max_instances <= 10
    error_message = "max_instances は 1〜10 の範囲で指定してください（個人利用の規模を超える設定を防ぐため）。"
  }
}

variable "cpu" {
  description = "コンテナの CPU 上限。"
  type        = string
  default     = "1"
}

variable "memory" {
  description = "コンテナのメモリ上限。"
  type        = string
  default     = "512Mi"
}

variable "concurrency" {
  description = "1 インスタンスが同時に受けるリクエスト数。大きいほど必要インスタンス数が減る。"
  type        = number
  default     = 80
}

variable "request_timeout_seconds" {
  description = "リクエストのタイムアウト秒数。AI 生成の同期 REST 呼び出しを吸収できる長さにする。"
  type        = number
  default     = 120
}

variable "health_check_path" {
  description = "起動プローブのパス。**API の基底パスは /api なので /health ではなく /api/health。**"
  type        = string
  default     = "/api/health"
}

variable "allow_unauthenticated" {
  description = <<-EOT
    allUsers に roles/run.invoker を付与するか。
    **Firebase Hosting の rewrites は公開されたサービスにしか転送できない**ため true が既定。
    認可はアプリ側（ID トークン検証 + 許可メール）で行う。
  EOT
  type        = bool
  default     = true
}

variable "deletion_protection" {
  description = "Cloud Run サービスの削除保護。データは Firestore 側にあり、サービスは再作成できるため既定は false。"
  type        = bool
  default     = false
}

variable "labels" {
  description = "リソースに付けるラベル。"
  type        = map(string)
  default     = {}
}
