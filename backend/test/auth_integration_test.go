package test_test

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/kalKun24/octant-cissp/backend/internal/contextkey"
	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
	"github.com/kalKun24/octant-cissp/backend/internal/infrastructure/firebaseauth"
	"github.com/kalKun24/octant-cissp/backend/internal/infrastructure/middleware"
	"github.com/kalKun24/octant-cissp/backend/internal/usecase/auth"
	"github.com/kalKun24/octant-cissp/backend/test"
)

const testPassword = "emulator-password-1234"

// TestAuthFlowIntegration は Auth エミュレータが発行した本物と同形の ID トークンを
// **本番と同じ検証経路**（firebaseauth.Verifier → usecase/auth → 認証ミドルウェア）に
// 通し、受け入れ条件の判定を実際の HTTP で確かめる。
//
// 検証をスキップする分岐が実装に入り込んでいないことも、ここで担保される。
// スキップがあれば「トークン無しで 200」や「許可メール外で 200」として現れる。
func TestAuthFlowIntegration(t *testing.T) {
	host, projectID := test.SkipUnlessAuthEmulator(t)

	allowedEmail := os.Getenv("ALLOWED_EMAILS")
	if allowedEmail == "" {
		t.Fatal("ALLOWED_EMAILS が設定されていません（make test-integration から実行してください）")
	}

	emulator := test.NewAuthEmulator(t, host, projectID)

	// 許可メール・検証済み。通るべき利用者。
	allowed := emulator.SignUp(t, allowedEmail, testPassword, true)
	t.Cleanup(func() { emulator.DeleteUser(t, allowed.UID) })

	// 許可メール外・検証済み。403 になるべき利用者。
	outsider := emulator.SignUp(t, "outsider@example.com", testPassword, true)
	t.Cleanup(func() { emulator.DeleteUser(t, outsider.UID) })

	// 許可メールだがメール未確認。401 になるべき利用者。
	unverified := emulator.SignUp(t, "unverified-"+allowedEmail, testPassword, false)
	t.Cleanup(func() { emulator.DeleteUser(t, unverified.UID) })

	server := httptest.NewServer(newAuthTestRouter(t, projectID, allowedEmail))
	t.Cleanup(server.Close)

	tests := []struct {
		name       string
		path       string
		token      string
		wantStatus int
		wantCode   string
		wantUID    string
	}{
		{
			name:       "トークン無しは 401",
			path:       "/api/probe",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
		{
			name:       "壊れたトークンは 401",
			path:       "/api/probe",
			token:      "not-a-jwt",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
		{
			name:       "許可メールの検証済みトークンは 200",
			path:       "/api/probe",
			token:      allowed.IDToken,
			wantStatus: http.StatusOK,
			wantUID:    allowed.UID,
		},
		{
			name:       "許可メール外のトークンは 403",
			path:       "/api/probe",
			token:      outsider.IDToken,
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			name:       "email_verified が false のトークンは 401",
			path:       "/api/probe",
			token:      unverified.IDToken,
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
		{
			name:       "/api/health はトークン無しでも 200",
			path:       "/api/health",
			wantStatus: http.StatusOK,
		},
		{
			name:       "/api/health は不正なトークンを付けても 200",
			path:       "/api/health",
			token:      "not-a-jwt",
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, server.URL+tt.path, nil)
			if err != nil {
				t.Fatalf("リクエストの作成に失敗しました: %v", err)
			}
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}

			res, err := server.Client().Do(req)
			if err != nil {
				t.Fatalf("リクエストに失敗しました: %v", err)
			}
			defer func() { _ = res.Body.Close() }()

			body, err := io.ReadAll(res.Body)
			if err != nil {
				t.Fatalf("応答の読み出しに失敗しました: %v", err)
			}

			if res.StatusCode != tt.wantStatus {
				t.Fatalf("ステータス: got %d, want %d（body=%s）", res.StatusCode, tt.wantStatus, body)
			}

			// 全応答にセキュリティヘッダが付くこと。
			if got := res.Header.Get("Cache-Control"); got != "no-store" {
				t.Errorf("Cache-Control: got %q, want %q", got, "no-store")
			}
			if got := res.Header.Get("X-Content-Type-Options"); got != "nosniff" {
				t.Errorf("X-Content-Type-Options: got %q, want %q", got, "nosniff")
			}

			var envelope struct {
				Data  json.RawMessage `json:"data"`
				Error *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(body, &envelope); err != nil {
				t.Fatalf("応答の解析に失敗しました: %v（body=%s）", err, body)
			}

			if tt.wantCode == "" {
				if envelope.Error != nil {
					t.Errorf("error は null であるべきです: %+v", envelope.Error)
				}
				if tt.wantUID != "" && string(envelope.Data) != fmt.Sprintf("%q", tt.wantUID) {
					t.Errorf("ハンドラが受け取った uid: got %s, want %q", envelope.Data, tt.wantUID)
				}
				return
			}

			if envelope.Error == nil {
				t.Fatalf("error が入っているべきです: body=%s", body)
			}
			if envelope.Error.Code != tt.wantCode {
				t.Errorf("error.code: got %q, want %q", envelope.Error.Code, tt.wantCode)
			}
			if string(envelope.Data) != "null" {
				t.Errorf("失敗時の data: got %s, want null", envelope.Data)
			}
		})
	}
}

// newAuthTestRouter は cmd/server と同じ構成でルータを組む。
//
// 認証ミドルウェアはルート照合の後（ハンドラの直前）に適用する。
// 本番では生成コードの Middlewares 経由で同じ位置に入る。
// /api/probe は認証が要るエンドポイントの代わり（TICKET-006 まで実物が無いため）。
func newAuthTestRouter(t *testing.T, projectID, allowedEmail string) http.Handler {
	t.Helper()

	verifier, err := firebaseauth.NewVerifier(t.Context(), projectID)
	if err != nil {
		t.Fatalf("検証器の生成に失敗しました: %v", err)
	}

	allowlist, err := entity.NewEmailAllowlist(allowedEmail)
	if err != nil {
		t.Fatalf("許可リストの作成に失敗しました: %v", err)
	}

	public, err := middleware.NewPublicRoutes("GET /api/health")
	if err != nil {
		t.Fatalf("免除リストの作成に失敗しました: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	authMW := middleware.Auth(logger, auth.NewAuthenticator(verifier, allowlist), public)

	r := chi.NewRouter()
	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Recovery(logger))

	// 認証を通った uid を封筒の data に入れて返す。
	// context の uid がトークンの uid と一致することを外から確かめられるようにする。
	r.With(authMW).Get("/api/probe", func(w http.ResponseWriter, r *http.Request) {
		uid, ok := contextkey.UID(r.Context())
		if !ok {
			t.Error("認証を通ったのに context から uid が取れません")
		}
		writeJSON(t, w, uid)
	})
	r.With(authMW).Get("/api/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, "ok")
	})

	return r
}

func writeJSON(t *testing.T, w http.ResponseWriter, data any) {
	t.Helper()

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	body := struct {
		Data  any `json:"data"`
		Error any `json:"error"`
	}{Data: data}

	if err := json.NewEncoder(w).Encode(body); err != nil {
		t.Errorf("応答の書き出しに失敗しました: %v", err)
	}
}
