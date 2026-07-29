#!/usr/bin/env bash
#
# HTML 内のインラインスクリプトが firebase.json の CSP（script-src）で
# 許可されていることを検証する。
#
# **なぜ要るか。**
# CSP は 'unsafe-inline' を使わず、インラインスクリプトをハッシュで許可している。
# ハッシュは中身が 1 文字変われば一致しなくなり、**ブラウザが黙って実行を止める**。
# 現在のインラインスクリプトは初回表示のテーマ確定（ちらつき防止）で、
# 止まっても画面は出るため、実測しない限り気づけない。
#
# 使い方:
#   scripts/check-csp-hashes.sh [HTMLファイル]   （既定: frontend/index.html）
#
# CI の verify（デプロイ権限なし）はソースの HTML を、
# scripts/deploy.sh はビルド後の dist/index.html を検証する。

set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
HTML_FILE="${1:-${REPO_ROOT}/frontend/index.html}"
FIREBASE_JSON="${REPO_ROOT}/firebase/firebase.json"

for f in "$HTML_FILE" "$FIREBASE_JSON"; do
	if [ ! -f "$f" ]; then
		echo "エラー: ${f} がありません。" >&2
		exit 1
	fi
done

node - "$HTML_FILE" "$FIREBASE_JSON" <<'NODE'
const fs = require("node:fs");
const crypto = require("node:crypto");

const [htmlPath, configPath] = process.argv.slice(2);
const html = fs.readFileSync(htmlPath, "utf8");
const config = JSON.parse(fs.readFileSync(configPath, "utf8"));

const cspHeader = (config.hosting?.headers ?? [])
  .flatMap((h) => h.headers ?? [])
  .find((h) => h.key.toLowerCase() === "content-security-policy");

if (!cspHeader) {
  console.error("エラー: firebase.json に Content-Security-Policy がありません。");
  process.exit(1);
}

const scriptSrc = cspHeader.value
  .split(";")
  .map((d) => d.trim())
  .find((d) => d.startsWith("script-src"));

if (!scriptSrc) {
  console.error("エラー: CSP に script-src がありません。");
  process.exit(1);
}

if (/'unsafe-inline'|'unsafe-eval'/.test(scriptSrc)) {
  console.error(`エラー: script-src に unsafe-inline / unsafe-eval があります: ${scriptSrc}`);
  console.error("  インラインスクリプトはハッシュで許可してください（XSS の主要な入口を塞ぐため）。");
  process.exit(1);
}

// src を持たない <script> の中身がインラインスクリプト。
const inline = [...html.matchAll(/<script(?![^>]*\ssrc=)[^>]*>([\s\S]*?)<\/script>/gi)].map(
  (m) => m[1],
);

let missing = 0;
for (const body of inline) {
  const hash = "sha256-" + crypto.createHash("sha256").update(body, "utf8").digest("base64");
  if (scriptSrc.includes(`'${hash}'`)) {
    console.log(`許可済み: '${hash}'`);
    continue;
  }
  missing += 1;
  console.error(`エラー: ${htmlPath} のインラインスクリプトが CSP で許可されていません。`);
  console.error(`  firebase.json の script-src に '${hash}' を足してください。`);
}

if (missing > 0) process.exit(1);
console.log(`CSP はインラインスクリプト ${inline.length} 件を許可しています: ${htmlPath}`);
NODE
