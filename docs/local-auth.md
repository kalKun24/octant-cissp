# ローカルで認証付き API を叩く

`make dev` は Firebase Auth エミュレータ（9099）を起動します。
ここで発行した ID トークンは本番のものと同じ形をしており、
**サーバは本番と同じ検証経路を通します**。認証をスキップする環境変数や
分岐は実装にありません。

## 前提

- `.env.example` を `.env` にコピーし、`ALLOWED_EMAILS` に試したいアドレスを書く
- `make dev` を起動しておく（API は `http://127.0.0.1:8080`）

以下の例では `ALLOWED_EMAILS=dev@example.com` を前提にします。

## 1. 利用者を作って ID トークンを取る

エミュレータの `accounts:signUp` は API キーの中身を検証しないため、
`key` には任意の文字列を渡せます。

```bash
EMU=127.0.0.1:9099
EMAIL=dev@example.com
PASS=emulator-password-1234

curl -s -X POST \
  "http://$EMU/identitytoolkit.googleapis.com/v1/accounts:signUp?key=fake-api-key" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"returnSecureToken\":true}" \
  | jq -r '.localId, .idToken'
```

**この時点のトークンは `email_verified` が false** です。そのまま API を叩くと
401 になります（仕様どおり）。

## 2. メールを確認済みにしてトークンを取り直す

`email_verified` はトークン発行時点の利用者情報で固定されます。
確認済みに変えたら、**サインインし直して新しいトークンを取ります**。

```bash
UID=<手順1で出た localId>

# 確認済みにする（エミュレータ専用の管理エンドポイント）
curl -s -X POST \
  "http://$EMU/identitytoolkit.googleapis.com/v1/projects/demo-octant/accounts:update" \
  -H 'Content-Type: application/json' \
  -H 'Authorization: Bearer owner' \
  -d "{\"localId\":\"$UID\",\"emailVerified\":true}" > /dev/null

# 新しい ID トークンを取る
TOKEN=$(curl -s -X POST \
  "http://$EMU/identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=fake-api-key" \
  -H 'Content-Type: application/json' \
  -d "{\"email\":\"$EMAIL\",\"password\":\"$PASS\",\"returnSecureToken\":true}" \
  | jq -r .idToken)
```

## 3. API を叩く

```bash
# 認証が要るエンドポイント（TICKET-006 以降で増えます）
curl -s -H "Authorization: Bearer $TOKEN" http://127.0.0.1:8080/api/notes | jq

# /api/health は認証を要しません
curl -s http://127.0.0.1:8080/api/health | jq
```

## 期待する結果

| 送るもの | 結果 |
|---|---|
| トークン無し | 401 `UNAUTHENTICATED` |
| 壊れた・期限切れのトークン | 401 `UNAUTHENTICATED` |
| `email_verified` が false のトークン | 401 `UNAUTHENTICATED` |
| `ALLOWED_EMAILS` に無いアドレスのトークン | **403 `FORBIDDEN`** |
| 許可メールの確認済みトークン | 200 |
| `/api/health`（トークンの有無を問わない） | 200 |

すべての応答に `Cache-Control: no-store` と `X-Content-Type-Options: nosniff` が付きます。

## 自動テスト

同じ内容を `make test-integration` が自動で検証します
（`backend/test/auth_integration_test.go`）。手で確かめる前にまずこちらを実行してください。

## 注意

**エミュレータは ID トークンの署名を検証しません。** 発行者・対象・有効期限・
クレームの検証は効きますが、署名だけを差し替えたトークンはエミュレータ上では通ります。
これは Firebase の仕様です。

そのため `APP_ENV` が `local` 以外のときに `FIREBASE_AUTH_EMULATOR_HOST` が
設定されていると、**サーバは起動を拒否します**。dev / prod にこの環境変数が
紛れ込むと、誰でも自作トークンで認証を通せる状態になるためです。
