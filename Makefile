# Octant のタスクランナー。CLAUDE.md「コマンド」の一覧を実体化したもの。
#
# deploy-dev / tf-plan / tf-apply は後続チケットで実装する。
# 現時点では未実装である旨を表示して異常終了する（嘘の成功を返さないため）。

SHELL := /usr/bin/env bash
.DEFAULT_GOAL := help

API_DIR      := $(CURDIR)/api
BACKEND_DIR  := $(CURDIR)/backend
FRONTEND_DIR := $(CURDIR)/frontend
SCRIPTS_DIR  := $(CURDIR)/scripts
GOLANGCI_CONFIG := $(CURDIR)/.golangci.yml

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
	cd $(FRONTEND_DIR) && if [ -f package-lock.json ]; then npm ci; else npm install; fi

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
# 未実装（後続チケットで実装する）
# ---------------------------------------------------------------------------

.PHONY: deploy-dev
deploy-dev: ## [未実装] dev 環境へ手動デプロイする
	$(call not_implemented,make deploy-dev,TICKET-005 CI/CD自動デプロイ)

.PHONY: tf-plan
tf-plan: ## [未実装] Terraform の差分を確認する
	$(call not_implemented,make tf-plan,TICKET-004 Terraform)

.PHONY: tf-apply
tf-apply: ## [未実装] Terraform を適用する
	$(call not_implemented,make tf-apply,TICKET-004 Terraform)

# ---------------------------------------------------------------------------
# ヘルプ
# ---------------------------------------------------------------------------

.PHONY: help
help: ## ターゲットの一覧を表示する
	@grep -hE '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) \
		| sort \
		| awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'
