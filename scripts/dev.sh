#!/usr/bin/env bash
#
# make dev の実体。エミュレータ・API・Vite をまとめて起動し、
# Ctrl-C でまとめて止める。
#
#   Firebase Auth エミュレータ  127.0.0.1:9099
#   Firestore エミュレータ      127.0.0.1:8808
#   API（Go）                   127.0.0.1:8080
#   Vite（フロント）            127.0.0.1:5173
#
# **認証をスキップする分岐は持たない。** ローカルでも Auth エミュレータが
# 発行した本物と同形の ID トークンを、本番と同じ検証経路に通す。
# エミュレータへの接続は FIREBASE_AUTH_EMULATOR_HOST / FIRESTORE_EMULATOR_HOST を
# SDK が読むことで成立しており、アプリ側のコードには現れない。

set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ---------------------------------------------------------------------------
# 設定
# ---------------------------------------------------------------------------

# .env があれば読む（コミットしない。書き方は .env.example を参照）。
if [ -f "$REPO_ROOT/.env" ]; then
	echo "==> .env を読み込みます"
	set -a
	# shellcheck disable=SC1091
	. "$REPO_ROOT/.env"
	set +a
fi

# エミュレータは demo- で始まるプロジェクト ID を使うと完全にオフラインで動く。
# 本物の GCP プロジェクトへ誤って接続する事故を防ぐため、既定をこれにしている。
: "${FIREBASE_PROJECT_ID:=demo-octant}"

# 許可メール。**ここに実在のアドレスをハードコードしない。**
# 自分のアドレスで試すときは .env に ALLOWED_EMAILS を書く。
: "${ALLOWED_EMAILS:=dev@example.com}"

: "${APP_ENV:=local}"
: "${LOG_LEVEL:=debug}"
: "${API_PORT:=8080}"
: "${AUTH_EMULATOR_PORT:=9099}"
: "${FIRESTORE_EMULATOR_PORT:=8808}"

export FIREBASE_PROJECT_ID ALLOWED_EMAILS APP_ENV LOG_LEVEL
export FIREBASE_AUTH_EMULATOR_HOST="127.0.0.1:${AUTH_EMULATOR_PORT}"
export FIRESTORE_EMULATOR_HOST="127.0.0.1:${FIRESTORE_EMULATOR_PORT}"
export PORT="$API_PORT"

PIDS=()

# ---------------------------------------------------------------------------
# 後始末
# ---------------------------------------------------------------------------

cleanup() {
	trap - EXIT INT TERM
	echo ""
	echo "==> 停止します"
	for pid in "${PIDS[@]:-}"; do
		[ -n "$pid" ] || continue
		# プロセスグループごと止める。firebase も vite も子プロセスを持つため、
		# 親だけ落とすとエミュレータや esbuild が残る。
		kill -TERM -- "-$pid" 2>/dev/null || kill -TERM "$pid" 2>/dev/null || true
	done
	wait 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# ---------------------------------------------------------------------------
# 前提の確認
# ---------------------------------------------------------------------------

require() {
	command -v "$1" >/dev/null 2>&1 || {
		echo "エラー: $1 が見つかりません。$2" >&2
		exit 1
	}
}

require firebase "npm i -g firebase-tools で導入してください。"
require go "Go 1.26 以降を導入してください。"
require java "Firestore エミュレータは Java を必要とします（例: apt install default-jre）。"

# ---------------------------------------------------------------------------
# 起動
# ---------------------------------------------------------------------------

# 指定のポートで待ち受けが始まるまで待つ。
wait_for_port() {
	local port="$1" name="$2" tries=0
	until (exec 3<>"/dev/tcp/127.0.0.1/${port}") 2>/dev/null; do
		tries=$((tries + 1))
		if [ "$tries" -ge 120 ]; then
			echo "エラー: ${name}（ポート ${port}）が起動しませんでした。" >&2
			exit 1
		fi
		sleep 0.5
	done
	exec 3<&- 2>/dev/null || true
	echo "==> ${name} が起動しました（127.0.0.1:${port}）"
}

echo "==> エミュレータを起動します（project=${FIREBASE_PROJECT_ID}）"
setsid firebase emulators:start \
	--only auth,firestore \
	--project "$FIREBASE_PROJECT_ID" \
	--config firebase/firebase.json &
PIDS+=($!)

wait_for_port "$AUTH_EMULATOR_PORT" "Firebase Auth エミュレータ"
wait_for_port "$FIRESTORE_EMULATOR_PORT" "Firestore エミュレータ"

echo "==> API を起動します"
(cd backend && exec setsid go run ./cmd/server) &
PIDS+=($!)

# go run はビルドを挟むため、待ち受けが始まるまで待ってから次へ進む。
wait_for_port "$API_PORT" "API"

echo "==> Vite を起動します"
(cd frontend && exec setsid npm run dev) &
PIDS+=($!)

cat <<EOS

--------------------------------------------------------------------
  API        http://127.0.0.1:${API_PORT}/api/health
  フロント   http://127.0.0.1:5173
  Auth       http://127.0.0.1:${AUTH_EMULATOR_PORT}
  Firestore  http://127.0.0.1:${FIRESTORE_EMULATOR_PORT}

  許可メール ${ALLOWED_EMAILS}
  ID トークンの取り方は docs/local-auth.md を参照してください。
  停止は Ctrl-C。
--------------------------------------------------------------------

EOS

# いずれかが落ちたら全体を止める。
wait -n
echo "エラー: いずれかのプロセスが終了しました。" >&2
exit 1
