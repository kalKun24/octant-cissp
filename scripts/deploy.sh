#!/usr/bin/env bash
#
# 1 環境分のデプロイ。GitHub Actions（.github/workflows/deploy.yml）と
# make deploy-dev の両方がこのスクリプトを呼ぶ。
#
# **手順を CI のワークフローに書かず、ここに集約している。**
# ワークフロー側にしか無い手順は手元で再現できず、
# 「CI では通るが手元では違うことが起きる」状態を生むため。
#
# 使い方:
#   ENV=dev scripts/deploy.sh
#   ENV=prod GIT_SHA=<コミット> scripts/deploy.sh
#
# 前提:
#   - gcloud が認証済み（CI は Workload Identity Federation、手元は ADC）
#   - docker が使える
#   - firebase CLI と node が入っている
#
# **環境変数（APP_ENV / FIREBASE_PROJECT_ID / ALLOWED_EMAILS / LOG_LEVEL /
# ANTHROPIC_API_KEY）は Terraform の管理物なので、ここでは一切上書きしない。**
# Cloud Run に渡すのはイメージだけ（「Terraform は構成を、CI はイメージを持つ」）。

set -Eeuo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# ---------------------------------------------------------------------------
# 入力の検証
# ---------------------------------------------------------------------------

case "${ENV:-}" in
dev | prod) ;;
"")
	echo "エラー: ENV を指定してください（例: ENV=dev scripts/deploy.sh）。" >&2
	exit 1
	;;
*)
	echo "エラー: ENV は dev または prod です（指定値: ${ENV}）。" >&2
	exit 1
	;;
esac

for cmd in gcloud docker firebase node; do
	command -v "$cmd" >/dev/null 2>&1 || {
		echo "エラー: ${cmd} が見つかりません。" >&2
		exit 1
	}
done

PROJECT_ID="octant-${ENV}"
REGION="asia-northeast1"
SERVICE="octant-api"
AR_REPOSITORY="octant"
# Hosting のサイト ID はプロジェクト ID と同じ（既定サイト）。
HOSTING_SITE="$PROJECT_ID"
HOSTING_URL="https://${HOSTING_SITE}.web.app"

: "${GIT_SHA:=$(git rev-parse HEAD)}"
SHORT_SHA="${GIT_SHA:0:7}"

IMAGE="${REGION}-docker.pkg.dev/${PROJECT_ID}/${AR_REPOSITORY}/api:${GIT_SHA}"

# リビジョン名は octant-api-<suffix>。**同じコミットを再デプロイしても衝突しない**よう
# 実行ごとに変わる要素を混ぜる（リビジョン名は再利用できない）。
: "${REVISION_SUFFIX:=${SHORT_SHA}-$(date -u +%Y%m%d%H%M%S)}"
REVISION_NAME="${SERVICE}-${REVISION_SUFFIX}"

echo "==> デプロイ対象"
echo "    環境        : ${ENV} (${PROJECT_ID})"
echo "    コミット    : ${GIT_SHA}"
echo "    イメージ    : ${IMAGE}"
echo "    リビジョン  : ${REVISION_NAME}"

# ---------------------------------------------------------------------------
# 1. API コンテナのビルドと push
#
# タグはコミット SHA。**latest を使わない**（どのコミットが動いているか
# リビジョンの一覧から辿れなくなるため）。
# ---------------------------------------------------------------------------

echo "==> API コンテナをビルドします"
docker build --platform linux/amd64 -t "$IMAGE" "$REPO_ROOT/backend"

echo "==> Artifact Registry へ push します"
gcloud auth configure-docker "${REGION}-docker.pkg.dev" --quiet
docker push "$IMAGE"

# ---------------------------------------------------------------------------
# 2. Cloud Run の更新
#
# deploy ではなく update を使う。**サービスが無ければ失敗してほしい**ため
# （サービスの作成は Terraform の責務）。
# ---------------------------------------------------------------------------

echo "==> Cloud Run を更新します"
gcloud run services update "$SERVICE" \
	--project "$PROJECT_ID" \
	--region "$REGION" \
	--image "$IMAGE" \
	--revision-suffix "$REVISION_SUFFIX" \
	--quiet

# 新しいリビジョンが「準備完了」かつ「トラフィックを受けている」ことを確認する。
# **設定漏れは、新リビジョンが起動せず旧リビジョンにトラフィックが残る形で現れ、
# デプロイコマンド自体は成功することがある**（TICKET-003 からの申し送り）。
latest_ready="$(gcloud run services describe "$SERVICE" \
	--project "$PROJECT_ID" --region "$REGION" \
	--format='value(status.latestReadyRevisionName)')"

if [ "$latest_ready" != "$REVISION_NAME" ]; then
	echo "エラー: 新しいリビジョンが準備完了になっていません。" >&2
	echo "  期待: ${REVISION_NAME}" >&2
	echo "  実際: ${latest_ready:-（なし）}" >&2
	echo "  Cloud Run のログで起動失敗の原因（環境変数の欠落など）を確認してください。" >&2
	exit 1
fi
echo "==> リビジョン ${latest_ready} が準備完了です"

# ---------------------------------------------------------------------------
# 3. フロントエンドのビルド
#
# firebaseConfig は **firebase apps:sdkconfig で取得する**。
# 秘密情報ではないがプロジェクト固有のため、リポジトリにも GitHub Secrets にも
# 実値を置かず、Firebase 側を唯一の正とする（アプリを作り直しても追従する）。
# ---------------------------------------------------------------------------

echo "==> firebaseConfig を取得します（${PROJECT_ID}）"
sdkconfig="$(firebase apps:sdkconfig WEB --project "$PROJECT_ID" --json)"

# 取得した JSON を VITE_ の環境変数に展開する。**値は表示しない**
# （API キー自体は公開値だが、ログに残す必要も無い）。
#
# node の失敗を握り潰さないよう、いったん変数に受けてから eval する
# （eval "$(...)" の形だと node が落ちても set -e が効かない）。
vite_env="$(printf '%s' "$sdkconfig" | node -e '
const chunks = [];
process.stdin.on("data", (c) => chunks.push(c));
process.stdin.on("end", () => {
  const parsed = JSON.parse(chunks.join(""));
  const c = parsed.result && parsed.result.sdkConfig;
  const keys = {
    VITE_FIREBASE_API_KEY: "apiKey",
    VITE_FIREBASE_AUTH_DOMAIN: "authDomain",
    VITE_FIREBASE_PROJECT_ID: "projectId",
    VITE_FIREBASE_STORAGE_BUCKET: "storageBucket",
    VITE_FIREBASE_MESSAGING_SENDER_ID: "messagingSenderId",
    VITE_FIREBASE_APP_ID: "appId",
  };
  const out = [];
  for (const [envName, field] of Object.entries(keys)) {
    const value = c && c[field];
    if (!value) {
      process.stderr.write(`エラー: firebaseConfig に ${field} がありません。\n`);
      process.exit(1);
    }
    out.push(`export ${envName}=${JSON.stringify(String(value))}`);
  }
  process.stdout.write(out.join("\n") + "\n");
});
')"
eval "$vite_env"

# **本番ビルドで Auth エミュレータへ接続させない。**
export VITE_USE_AUTH_EMULATOR=false

echo "==> フロントエンドをビルドします"
make build-front

# Hosting の公開ディレクトリは firebase.json のある場所より外を指せない
# （firebase CLI が "outside of project directory" で拒否する）。
# ビルド成果物を firebase/dist へ複製してから配る。
# **毎回消してから複製する。** 前回の残骸が混ざると、消したはずのファイルが
# 配信され続ける。
HOSTING_DIR="${REPO_ROOT}/firebase/dist"
rm -rf "$HOSTING_DIR"
cp -R "${REPO_ROOT}/frontend/dist" "$HOSTING_DIR"

# ---------------------------------------------------------------------------
# 4. Firestore ルールと Hosting のデプロイ
#
# ルールは毎回リリースし直す。**全拒否からの退行を CI が検知できる状態**にするため
# （リリースされていない状態でも既定で拒否されるが、それは保証ではない）。
# ---------------------------------------------------------------------------

echo "==> firestore.rules と Hosting をデプロイします"
firebase deploy \
	--only "firestore:rules,hosting" \
	--project "$PROJECT_ID" \
	--config "$REPO_ROOT/firebase/firebase.json" \
	--non-interactive

# ---------------------------------------------------------------------------
# 5. 疎通確認
#
# **Hosting 経由で見る。** rewrites が壊れていれば index.html が返り、
# Cloud Run を直接叩く確認では気づけない。
# 返ってきた revision が今デプロイしたリビジョンと一致することまで見て、
# 「古いリビジョンが応答し続けている」状態を弾く。
# ---------------------------------------------------------------------------

echo "==> ${HOSTING_URL}/api/health を確認します"

health_ok=""
for attempt in $(seq 1 20); do
	body="$(curl -sS --max-time 20 -w '\n%{http_code}' "${HOSTING_URL}/api/health" || true)"
	status="$(printf '%s' "$body" | tail -n1)"
	payload="$(printf '%s' "$body" | sed '$d')"

	if [ "$status" = "200" ]; then
		revision="$(printf '%s' "$payload" | node -e '
const chunks = [];
process.stdin.on("data", (c) => chunks.push(c));
process.stdin.on("end", () => {
  try {
    const b = JSON.parse(chunks.join(""));
    process.stdout.write(String((b.data && b.data.revision) || ""));
  } catch {
    process.stdout.write("");
  }
});
')"
		if [ "$revision" = "$REVISION_NAME" ]; then
			health_ok="yes"
			echo "==> 200 OK（revision=${revision}）"
			break
		fi
		echo "    200 だが revision が ${revision:-不明} です（期待: ${REVISION_NAME}）。再試行します（${attempt}/20）"
	else
		echo "    HTTP ${status:-不明} です。再試行します（${attempt}/20）"
	fi
	sleep 6
done

if [ -z "$health_ok" ]; then
	echo "エラー: ${HOSTING_URL}/api/health が期待どおりに応答しませんでした。" >&2
	echo "  最後の応答: ${payload}" >&2
	exit 1
fi

echo "==> デプロイが完了しました: ${HOSTING_URL}"
