// Package test は統合テスト用のヘルパを提供する。
//
// **本番のコードから import しないこと。** ここに置くのは、
// エミュレータを相手にするテストが共通で必要とする道具だけ。
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
)

// AuthEmulatorHostEnv は Firebase Admin SDK が Auth エミュレータの接続先として読む環境変数。
const AuthEmulatorHostEnv = "FIREBASE_AUTH_EMULATOR_HOST"

// emulatorAPIKey は Auth エミュレータが要求する API キー。
// エミュレータは値を検証しないが、パラメータ自体は必須。
// 本物の資格情報ではないため、静的解析の指摘を明示的に抑制する。
const emulatorAPIKey = "fake-api-key" //nolint:gosec // エミュレータ用のダミー値

// httpTimeout はエミュレータへの1回の呼び出しに許す上限。
const httpTimeout = 10 * time.Second

// SkipUnlessAuthEmulator はエミュレータが無い環境でテストを skip する。
//
// **skip はテストの緩和ではない。** 統合テストは make test-integration から
// エミュレータ付きで走る。ここで skip されるのは `go test ./...` を素で
// 実行した場合であり、その場合は「検証していない」ことが出力に残る。
func SkipUnlessAuthEmulator(t *testing.T) (host, projectID string) {
	t.Helper()

	host = os.Getenv(AuthEmulatorHostEnv)
	projectID = os.Getenv("FIREBASE_PROJECT_ID")

	if host == "" || projectID == "" {
		t.Skipf("Auth エミュレータが設定されていないため skip します"+
			"（%s と FIREBASE_PROJECT_ID が必要です。make test-integration を使ってください）",
			AuthEmulatorHostEnv)
	}

	return host, projectID
}

// AuthEmulator は Firebase Auth エミュレータを操作するテスト用クライアント。
type AuthEmulator struct {
	host      string
	projectID string
	admin     *auth.Client
	client    *http.Client
}

// NewAuthEmulator は Auth エミュレータのクライアントを作る。
func NewAuthEmulator(t *testing.T, host, projectID string) *AuthEmulator {
	t.Helper()

	app, err := firebase.NewApp(t.Context(), &firebase.Config{ProjectID: projectID})
	if err != nil {
		t.Fatalf("Firebase アプリの初期化に失敗しました: %v", err)
	}
	admin, err := app.Auth(t.Context())
	if err != nil {
		t.Fatalf("Auth クライアントの生成に失敗しました: %v", err)
	}

	return &AuthEmulator{
		host:      host,
		projectID: projectID,
		admin:     admin,
		client:    &http.Client{Timeout: httpTimeout},
	}
}

// User はエミュレータ上に作った利用者と、その利用者の ID トークン。
type User struct {
	UID   string
	Email string
	// IDToken は Authorization: Bearer に載せる値。
	IDToken string
}

// SignUp はメールとパスワードで利用者を作り、ID トークンを取得する。
//
// emailVerified を true にする場合は、Admin SDK で利用者を更新してから
// **サインインし直して**トークンを取り直す。ID トークンのクレームは
// 発行時点の利用者情報で固定されるため、作成直後のトークンを使い回すと
// email_verified が false のままになる。
func (e *AuthEmulator) SignUp(t *testing.T, email, password string, emailVerified bool) User {
	t.Helper()

	signUp := e.identityToolkit(t, "accounts:signUp", map[string]any{
		"email":             email,
		"password":          password,
		"returnSecureToken": true,
	})

	uid, ok := signUp["localId"].(string)
	if !ok || uid == "" {
		t.Fatalf("accounts:signUp が localId を返しませんでした: %v", signUp)
	}

	if !emailVerified {
		token, ok := signUp["idToken"].(string)
		if !ok || token == "" {
			t.Fatal("accounts:signUp が idToken を返しませんでした")
		}
		return User{UID: uid, Email: email, IDToken: token}
	}

	update := (&auth.UserToUpdate{}).EmailVerified(true)
	if _, err := e.admin.UpdateUser(t.Context(), uid, update); err != nil {
		t.Fatalf("email_verified の更新に失敗しました: %v", err)
	}

	return User{UID: uid, Email: email, IDToken: e.SignIn(t, email, password)}
}

// SignIn は既存の利用者としてサインインし、新しい ID トークンを返す。
func (e *AuthEmulator) SignIn(t *testing.T, email, password string) string {
	t.Helper()

	res := e.identityToolkit(t, "accounts:signInWithPassword", map[string]any{
		"email":             email,
		"password":          password,
		"returnSecureToken": true,
	})

	token, ok := res["idToken"].(string)
	if !ok || token == "" {
		t.Fatalf("accounts:signInWithPassword が idToken を返しませんでした: %v", res)
	}
	return token
}

// DeleteUser はエミュレータ上の利用者を消す。テストの後始末に使う。
func (e *AuthEmulator) DeleteUser(t *testing.T, uid string) {
	t.Helper()

	if err := e.admin.DeleteUser(context.WithoutCancel(t.Context()), uid); err != nil {
		t.Logf("利用者の削除に失敗しました（後続に影響はありません）: %v", err)
	}
}

// identityToolkit はエミュレータの Identity Toolkit API を呼ぶ。
func (e *AuthEmulator) identityToolkit(t *testing.T, method string, payload map[string]any) map[string]any {
	t.Helper()

	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("リクエストの組み立てに失敗しました: %v", err)
	}

	url := fmt.Sprintf("http://%s/identitytoolkit.googleapis.com/v1/%s?key=%s",
		e.host, method, emulatorAPIKey)

	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("リクエストの作成に失敗しました: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := e.client.Do(req)
	if err != nil {
		t.Fatalf("エミュレータへの接続に失敗しました: %v", err)
	}
	defer func() { _ = res.Body.Close() }()

	var decoded map[string]any
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		t.Fatalf("エミュレータの応答の解析に失敗しました: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("%s が %d を返しました: %v", method, res.StatusCode, decoded)
	}

	return decoded
}
