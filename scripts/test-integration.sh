#!/usr/bin/env bash
#
# make test-integration の実体。
#
# Firebase Auth エミュレータを起動し、`Integration` を含むテストだけを走らせる。
# 統合テストはエミュレータが無ければ自分で skip するため、make test には含めない
# （CI と手元でエミュレータの起動条件が揃わないため、明示的に走らせる）。
#
# Firestore を使うテストは TICKET-006 以降で追加する。そのときにこのスクリプトへ
# `--only auth,firestore` と FIRESTORE_EMULATOR_HOST を足すこと。

set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

: "${FIREBASE_PROJECT_ID:=demo-octant}"
: "${AUTH_EMULATOR_PORT:=9099}"

export FIREBASE_PROJECT_ID
export FIREBASE_AUTH_EMULATOR_HOST="127.0.0.1:${AUTH_EMULATOR_PORT}"
# 統合テストは検証済みトークンの経路も通すため、許可メールを1件与えておく。
# テスト側はこの値を読んで、許可されるアドレスと許可されないアドレスを作る。
export ALLOWED_EMAILS="allowed@example.com"

EMULATOR_PID=""

cleanup() {
	trap - EXIT INT TERM
	if [ -n "$EMULATOR_PID" ]; then
		echo "==> エミュレータを停止します"
		kill -TERM -- "-$EMULATOR_PID" 2>/dev/null || kill -TERM "$EMULATOR_PID" 2>/dev/null || true
		wait "$EMULATOR_PID" 2>/dev/null || true
	fi
}
trap cleanup EXIT INT TERM

command -v firebase >/dev/null 2>&1 || {
	echo "エラー: firebase が見つかりません。npm i -g firebase-tools で導入してください。" >&2
	exit 1
}

echo "==> Firebase Auth エミュレータを起動します（project=${FIREBASE_PROJECT_ID}）"
setsid firebase emulators:start \
	--only auth \
	--project "$FIREBASE_PROJECT_ID" \
	--config firebase/firebase.json &
EMULATOR_PID=$!

tries=0
until (exec 3<>"/dev/tcp/127.0.0.1/${AUTH_EMULATOR_PORT}") 2>/dev/null; do
	tries=$((tries + 1))
	if [ "$tries" -ge 120 ]; then
		echo "エラー: Auth エミュレータ（ポート ${AUTH_EMULATOR_PORT}）が起動しませんでした。" >&2
		exit 1
	fi
	sleep 0.5
done
exec 3<&- 2>/dev/null || true
echo "==> Auth エミュレータが起動しました"

echo "==> 統合テストを実行します"
# -count=1 でキャッシュを無効にする（エミュレータの状態に依存するため）。
cd backend && go test ./... -run Integration -count=1 -v
