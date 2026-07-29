#!/usr/bin/env bash
#
# firebase/firestore.rules が「全ドキュメント拒否」のままであることを検証する。
#
# **なぜ要るか。**
# CI は roles/firebaserules.admin を持ち、develop / main への push でルールを
# そのままリリースする。`if false` を `if true` に変えた変更が 1 本通れば、
# CI がそれを本番へ配ってしまう。Firebase の Web API キーは公開バンドルに含まれ、
# プロジェクト ID も既知なので、**第三者が Firestore の全データを読み書きできる**。
#
# ルールのリリースは「デプロイ権限を持つジョブ」が行うため、この検証は
# **デプロイ権限を持たない verify ジョブ側**で走らせること（make check-config）。
#
# 設計上の前提（CLAUDE.md「守るべき制約」）:
#   ブラウザは Firestore に直接アクセスしない。読み書きは全て Go の REST API を通り、
#   サーバの SA（ルールを迂回する）だけが実際に触る。
#   **したがって allow が 1 行でも増えることは設計違反であり、事故である。**
#
# ここを緩める必要が出たと感じたら、まず設計の見直しを提案すること。

set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
RULES_FILE="${1:-${REPO_ROOT}/firebase/firestore.rules}"

if [ ! -f "$RULES_FILE" ]; then
	echo "エラー: ${RULES_FILE} がありません。" >&2
	exit 1
fi

# コメント（// 以降）と空行を落としてから検査する。
# コメント中の "allow read, write: if true" のような例示で誤検知しないため。
body="$(sed -e 's://.*::' "$RULES_FILE" | grep -v '^[[:space:]]*$')"

fail() {
	echo "エラー: firebase/firestore.rules が全拒否ではありません。" >&2
	echo "  $1" >&2
	echo "" >&2
	echo "  ブラウザは Firestore に直接アクセスしない設計です（CLAUDE.md）。" >&2
	echo "  ここに allow を足すのは設計違反であり、CI が本番へリリースします。" >&2
	exit 1
}

# 1) allow 文はちょうど 1 つ。
allow_count="$(printf '%s\n' "$body" | grep -c '\ballow\b' || true)"
if [ "$allow_count" -ne 1 ]; then
	fail "allow 文が ${allow_count} 個あります（1 個であるべきです）。"
fi

# 2) その 1 行は「読み書きを常に拒否する」ものであること。
#    空白の揺れは許すが、条件式は if false 以外を認めない。
allow_line="$(printf '%s\n' "$body" | grep '\ballow\b' | tr -s '[:space:]' ' ' | sed -e 's/^ //' -e 's/ $//')"
if [ "$allow_line" != "allow read, write: if false;" ]; then
	fail "allow 文が 'allow read, write: if false;' ではありません（実際: '${allow_line}'）。"
fi

# 3) 条件式に使える材料そのものを禁じる。
#    request.auth や resource を持ち込んだ時点で「第2の認可ロジック」が始まる。
for forbidden in "request.auth" "request.resource" "resource.data" "get(" "exists("; do
	if printf '%s\n' "$body" | grep -qF "$forbidden"; then
		fail "認可の判断材料（${forbidden}）が使われています。認可は Go の 1 箇所に集約する設計です。"
	fi
done

# 4) match は全ドキュメントを覆う 1 組であること（部分適用の穴を作らせない）。
if ! printf '%s\n' "$body" | grep -q 'match /{document=\*\*}'; then
	fail "match /{document=**} がありません（全ドキュメントを覆っていません）。"
fi

echo "firestore.rules は全ドキュメント拒否です: ${allow_line}"
