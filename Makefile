# Octant のタスクランナー。CLAUDE.md「コマンド」の一覧を実体化したもの。
#
# 未実装のターゲットは not_implemented で異常終了させる（嘘の成功を返さないため）。

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

API_DIR      := $(CURDIR)/api
BACKEND_DIR  := $(CURDIR)/backend
FRONTEND_DIR := $(CURDIR)/frontend
SCRIPTS_DIR  := $(CURDIR)/scripts
INFRA_DIR    := $(CURDIR)/infra
GOLANGCI_CONFIG := $(CURDIR)/.golangci.yml

# Terraform の対象環境。tf-* ターゲットは ENV=dev|prod を要求する。
TERRAFORM  ?= terraform
TF_ENV_DIR := $(INFRA_DIR)/environments/$(ENV)

# GCP プロジェクト ID。TICKET-016 で作成したもの。
# Terraform 側は environments/*/main.tf が持つが、firebase CLI は
# --project を要求するため Makefile にも置く。**両者を食い違わせないこと。**
FIREBASE_PROJECT_dev  := octant-dev
FIREBASE_PROJECT_prod := octant-prod

# make gen の入力と出力。gen-check はこの出力の差分だけを見る。
OPENAPI_SPEC := $(API_DIR)/openapi.yaml
GO_API_GEN   := $(BACKEND_DIR)/internal/interface/openapi/openapi.gen.go
TS_API_GEN   := $(FRONTEND_DIR)/src/api/generated/schema.ts

# go install で入るツールは PATH に無いことが多いため、GOPATH/bin を通しておく。
GOPATH_BIN := $(shell go env GOPATH)/bin
export PATH := $(GOPATH_BIN):$(PATH)

# ツールのバージョンは固定する。CI と手元で挙動を揃えるため latest を使わない。
GOLANGCI_LINT_VERSION := v2.12.2
OAPI_CODEGEN_VERSION  := v2.8.0

# API コンテナのイメージ名（ローカルビルドの確認用）。
API_IMAGE := octant-api:local

# フロントの雛形がまだ無い状態でも Go 側だけ検証できるよう、
# フロントを触るターゲットは package.json の存在を先に確かめる。
define require_frontend
	@if [ ! -f "$(FRONTEND_DIR)/package.json" ]; then \
		echo "エラー: $(FRONTEND_DIR)/package.json がありません。先に make setup を実行するか、フロントの雛形を用意してください。" >&2; \
		exit 1; \
	fi
endef

# 未実装ターゲットの共通処理。担当チケットを示して異常終了する。
define not_implemented
	@echo "$(1) は未実装です（$(2) で実装予定）。" >&2
	@exit 1
endef

# ---------------------------------------------------------------------------
# 開発サーバ
# ---------------------------------------------------------------------------

.PHONY: dev
dev: ## エミュレータ（Auth 9099 / Firestore 8808）と API と Vite を同時起動する
	$(SCRIPTS_DIR)/dev.sh

# ---------------------------------------------------------------------------
# セットアップ
# ---------------------------------------------------------------------------

.PHONY: setup
setup: setup-tools setup-back setup-front ## 依存とツールを一括で導入する

.PHONY: setup-tools
setup-tools: ## golangci-lint と oapi-codegen を導入する
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
	go install github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@$(OAPI_CODEGEN_VERSION)

.PHONY: setup-back
setup-back: ## Go の依存を取得する
	cd $(BACKEND_DIR) && go mod download

.PHONY: setup-front
setup-front: ## npm の依存を取得する
	$(call require_frontend)
	@# package-lock.json があれば再現性のある npm ci を使う。無ければ npm install で作る。
	@#
	@# --ignore-scripts: 依存の postinstall を実行しない。
	@# **CI のデプロイジョブは GCP の資格情報を持った状態で走る**ため、
	@# 依存 1 つの侵害が octant-ci のなりすましに直結する。
	@# 現在の依存は postinstall 無しでビルドできることを確認済み（make build-front）。
	cd $(FRONTEND_DIR) && if [ -f package-lock.json ]; then npm ci --ignore-scripts; else npm install --ignore-scripts; fi

# ---------------------------------------------------------------------------
# コード生成（API First）
# ---------------------------------------------------------------------------
#
# openapi.yaml が API の正。Go の ServerInterface と TS の型はそこから作る。
# 生成物はコミットし、CI が gen-check で openapi.yaml との一致を検証する。

.PHONY: gen
gen: gen-back gen-front ## openapi.yaml から Go のサーバ型と TS の型を生成する

.PHONY: gen-back
gen-back: ## openapi.yaml から Go のサーバインターフェースを生成する
	@mkdir -p $(dir $(GO_API_GEN))
	@# oapi-codegen.yaml の output は相対パスのため、api/ を作業ディレクトリにして解決する。
	cd $(API_DIR) && oapi-codegen --config oapi-codegen.yaml openapi.yaml

.PHONY: gen-front
gen-front: ## openapi.yaml から TypeScript の型を生成する
	$(call require_frontend)
	cd $(FRONTEND_DIR) && npm run gen

.PHONY: gen-check
gen-check: gen ## 生成物が openapi.yaml と一致することを検証する（CI 用）
	@git rev-parse --is-inside-work-tree >/dev/null 2>&1 || { \
		echo "エラー: git リポジトリの外では生成物の差分を検証できません。" >&2; \
		exit 1; \
	}
	@changed="$$(git status --porcelain -- $(GO_API_GEN) $(TS_API_GEN))" || { \
		echo "エラー: git status の実行に失敗しました。検証を続行できません。" >&2; \
		exit 1; \
	}; \
	if [ -n "$$changed" ]; then \
		echo "エラー: 生成物が openapi.yaml と一致していません。" >&2; \
		echo "make gen を実行し、生成物をコミットしてください。" >&2; \
		git --no-pager diff -- $(GO_API_GEN) $(TS_API_GEN); \
		exit 1; \
	fi
	@echo "生成物は $(OPENAPI_SPEC) と一致しています。"

# ---------------------------------------------------------------------------
# 静的解析
# ---------------------------------------------------------------------------

.PHONY: lint
lint: lint-back lint-front ## golangci-lint と eslint と tsc --noEmit を実行する

.PHONY: lint-back
lint-back: ## Go を golangci-lint で検査する
	@# .golangci.yml はリポジトリ直下、Go コードは backend/ にあるため設定を明示する。
	cd $(BACKEND_DIR) && golangci-lint run --config $(GOLANGCI_CONFIG) ./...

.PHONY: lint-front
lint-front: ## フロントを eslint と tsc --noEmit で検査する
	$(call require_frontend)
	cd $(FRONTEND_DIR) && npm run lint
	cd $(FRONTEND_DIR) && npm run typecheck

# ---------------------------------------------------------------------------
# 整形
# ---------------------------------------------------------------------------

.PHONY: fmt
fmt: fmt-back fmt-front ## gofmt と prettier を実行する

.PHONY: fmt-back
fmt-back: ## Go を整形する（gofmt + goimports）
	# gofmt だけでは import の並びを直せず、lint-back の goimports で落ちる。
	# lint と同じ設定で整形し、「fmt を実行すれば lint が通る」状態を保つ。
	cd $(BACKEND_DIR) && golangci-lint fmt --config $(GOLANGCI_CONFIG) ./...

.PHONY: fmt-front
fmt-front: ## フロントを prettier で整形する
	$(call require_frontend)
	cd $(FRONTEND_DIR) && npm run format

# ---------------------------------------------------------------------------
# テスト
# ---------------------------------------------------------------------------

.PHONY: test
test: ## backend の全テストを実行する
	cd $(BACKEND_DIR) && go test ./...

.PHONY: test-integration
test-integration: ## Auth エミュレータを起動して統合テストを実行する
	@# 統合テストはエミュレータ前提のため、環境変数が無ければ自分で skip する。
	@# make test には含めない（CI と手元でエミュレータの起動条件が揃わないため）。
	$(SCRIPTS_DIR)/test-integration.sh

.PHONY: test-front
test-front: ## frontend のテスト（Vitest）を実行する
	$(call require_frontend)
	cd $(FRONTEND_DIR) && npm run test

# ---------------------------------------------------------------------------
# ビルド
# ---------------------------------------------------------------------------

.PHONY: build
build: build-front build-api ## フロントのビルドと API コンテナのビルドを実行する

.PHONY: build-front
build-front: ## フロントをビルドする
	$(call require_frontend)
	cd $(FRONTEND_DIR) && npm run build

.PHONY: build-api
build-api: ## API コンテナをビルドする
	docker build -t $(API_IMAGE) $(BACKEND_DIR)

# ---------------------------------------------------------------------------
# インフラ（Terraform）
# ---------------------------------------------------------------------------
#
# 実行例:
#   make tf-plan ENV=dev
#   make tf-apply ENV=prod
#
# 認証は手元の ADC（gcloud auth application-default login）を使う。
# **サービスアカウントキーの JSON は発行しない。**

.PHONY: tf-init tf-plan tf-apply tf-fmt tf-validate tf-output

tf-init: tf-guard ## Terraform を初期化する（ENV=dev|prod）
	cd $(TF_ENV_DIR) && $(TERRAFORM) init -input=false

tf-plan: tf-init ## Terraform の差分を確認する（ENV=dev|prod）
	cd $(TF_ENV_DIR) && $(TERRAFORM) plan -input=false

tf-apply: tf-init ## Terraform を適用する（ENV=dev|prod）
	cd $(TF_ENV_DIR) && $(TERRAFORM) apply

tf-output: tf-guard ## Terraform の出力を表示する（ENV=dev|prod）
	cd $(TF_ENV_DIR) && $(TERRAFORM) output

tf-fmt: ## Terraform のコードを整形する
	$(TERRAFORM) fmt -recursive $(INFRA_DIR)

# ---------------------------------------------------------------------------
# Firestore のセキュリティルール
# ---------------------------------------------------------------------------
#
# **ルールをデプロイしないと「ruleset が存在しない」状態になる。**
# その状態でも Firestore は全拒否するが、それは既定の副作用であって
# コードで固定された保証ではない。Firebase コンソールで Firestore の
# ページを開くと「テストモード（30日間 allow all）」を提示され、
# 押した瞬間に全公開になる。**明示的にリリースして塞いでおく。**

# ---------------------------------------------------------------------------
# デプロイ前の設定ゲート
# ---------------------------------------------------------------------------
#
# **デプロイ権限を持たないジョブで走らせる。**
# CI は firebaserules.admin と firebasehosting.admin を持ち、push された内容を
# そのままリリースする。つまり「リポジトリの設定ファイルの中身」が
# 本番の防壁そのものであり、レビュー以外に止める仕組みが無い。

.PHONY: check-config
check-config: check-rules check-csp ## firestore.rules と CSP の設定を検証する（CI 用）

.PHONY: check-rules
check-rules: ## firestore.rules が全ドキュメント拒否のままか検証する
	$(SCRIPTS_DIR)/check-firestore-rules.sh

.PHONY: check-csp
check-csp: ## インラインスクリプトが CSP のハッシュで許可されているか検証する
	$(SCRIPTS_DIR)/check-csp-hashes.sh

.PHONY: rules-deploy
rules-deploy: rules-guard ## firestore.rules をデプロイする（ENV=dev|prod）
	firebase deploy --only firestore:rules \
		--project $(FIREBASE_PROJECT_$(ENV)) \
		--config $(CURDIR)/firebase/firebase.json

.PHONY: rules-check
rules-check: rules-guard ## リリース済みの ruleset があることを確認する（ENV=dev|prod）
	@project="$(FIREBASE_PROJECT_$(ENV))"; \
	token="$$(gcloud auth print-access-token)"; \
	n="$$(curl -sf -H "Authorization: Bearer $$token" -H "x-goog-user-project: $$project" \
		"https://firebaserules.googleapis.com/v1/projects/$$project/releases" \
		| grep -c '"name"' || true)"; \
	if [ "$$n" -eq 0 ]; then \
		echo "エラー: $$project に ruleset がリリースされていません。" >&2; \
		echo "  make rules-deploy ENV=$(ENV) を実行してください。" >&2; \
		exit 1; \
	fi; \
	echo "$$project に ruleset がリリースされています。"

.PHONY: rules-guard
rules-guard:
	@case "$(ENV)" in \
		dev|prod) ;; \
		"") echo "エラー: ENV を指定してください（例: make rules-deploy ENV=dev）。" >&2; exit 1 ;; \
		*) echo "エラー: ENV は dev または prod です（指定値: $(ENV)）。" >&2; exit 1 ;; \
	esac

tf-validate: tf-init ## Terraform の構文と型を検証する（ENV=dev|prod）
	cd $(TF_ENV_DIR) && $(TERRAFORM) validate

# ENV の指定漏れ・打ち間違いと tfvars の未作成を、GCP を触る前に弾く。
.PHONY: tf-guard
tf-guard:
	@case "$(ENV)" in \
		dev|prod) ;; \
		"") echo "エラー: ENV を指定してください（例: make tf-plan ENV=dev）。" >&2; exit 1 ;; \
		*) echo "エラー: ENV は dev または prod です（指定値: $(ENV)）。" >&2; exit 1 ;; \
	esac
	@if [ ! -f "$(TF_ENV_DIR)/terraform.tfvars" ]; then \
		echo "エラー: $(TF_ENV_DIR)/terraform.tfvars がありません。" >&2; \
		echo "  cp $(TF_ENV_DIR)/terraform.tfvars.example $(TF_ENV_DIR)/terraform.tfvars" >&2; \
		echo "  を実行し、許可メールと通知先メールを埋めてください（このファイルはコミットしません）。" >&2; \
		exit 1; \
	fi

# ---------------------------------------------------------------------------
# デプロイ
# ---------------------------------------------------------------------------
#
# **通常は CI に任せる。** develop への push で dev、main への push で prod へ
# 自動デプロイされる（.github/workflows/deploy-*.yml）。
#
# ここにあるのは、CI と同じ手順を手元から再現するための出口。
# パイプラインの検証と、CI が動かせないときの緊急用に使う。
# **手で gcloud run deploy を打たない**（scripts/deploy.sh を通す）。

.PHONY: deploy
deploy: deploy-guard ## dev または prod へデプロイする（ENV=dev|prod。通常は CI に任せる）
	ENV=$(ENV) $(SCRIPTS_DIR)/deploy.sh

.PHONY: deploy-dev
deploy-dev: ## dev 環境へ手動デプロイする（通常は CI に任せる）
	$(MAKE) deploy ENV=dev

.PHONY: deploy-guard
deploy-guard:
	@case "$(ENV)" in \
		dev|prod) ;; \
		"") echo "エラー: ENV を指定してください（例: make deploy ENV=dev）。" >&2; exit 1 ;; \
		*) echo "エラー: ENV は dev または prod です（指定値: $(ENV)）。" >&2; exit 1 ;; \
	esac

# ---------------------------------------------------------------------------
# ヘルプ
# ---------------------------------------------------------------------------

.PHONY: help
help: ## ターゲットの一覧を表示する
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
