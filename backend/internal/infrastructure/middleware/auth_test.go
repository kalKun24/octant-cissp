package middleware

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/kalKun24/octant-cissp/backend/internal/contextkey"
	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
	"github.com/kalKun24/octant-cissp/backend/internal/usecase/auth"
)

const (
	validToken     = "valid-token"
	forbiddenToken = "forbidden-token"
	testUID        = "uid-under-test"
)

// fakeAuthenticator はトークン文字列で結果を決めるテスト用の認証器。
type fakeAuthenticator struct{}

func (fakeAuthenticator) Authenticate(_ context.Context, idToken string) (*entity.AuthUser, error) {
	switch idToken {
	case validToken:
		return &entity.AuthUser{UID: testUID, Email: "allowed@example.com", EmailVerified: true}, nil
	case forbiddenToken:
		return nil, auth.ErrNotAllowed
	default:
		return nil, auth.ErrUnauthenticated
	}
}

// newTestRouter は「免除される /public」と「認証が要る /private」を持つルータを作る。
// 認証ミドルウェアは本番と同じくルート照合の後（ハンドラの直前）に適用する。
func newTestRouter(t *testing.T, public PublicRoutes) http.Handler {
	t.Helper()

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	authMW := Auth(logger, fakeAuthenticator{}, public)

	// 到達したことと、context に uid が入っていることを応答で見せるハンドラ。
	reached := func(w http.ResponseWriter, r *http.Request) {
		uid, _ := contextkey.UID(r.Context())
		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, uid); err != nil {
			t.Errorf("応答の書き出しに失敗しました: %v", err)
		}
	}

	r := chi.NewRouter()
	r.Group(func(r chi.Router) {
		r.With(authMW).Get("/api/public", reached)
		r.With(authMW).Head("/api/public", reached)
		r.With(authMW).Get("/api/private", reached)
		r.With(authMW).Get("/api/items/{itemID}", reached)
	})

	return r
}

func TestAuthRejectsWithoutValidToken(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		path       string
		authHeader []string
		wantStatus int
		wantCode   string
	}{
		{
			name:       "トークン無しは 401",
			path:       "/api/private",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
		{
			name:       "Bearer 以外の方式は 401",
			path:       "/api/private",
			authHeader: []string{"Basic " + validToken},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
		{
			name:       "Bearer だけで値が無いのは 401",
			path:       "/api/private",
			authHeader: []string{"Bearer "},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
		{
			name:       "方式名の大小は区別しない",
			path:       "/api/private",
			authHeader: []string{"bearer " + validToken},
			wantStatus: http.StatusOK,
		},
		{
			name:       "検証に失敗するトークンは 401",
			path:       "/api/private",
			authHeader: []string{"Bearer broken"},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
		{
			name:       "許可されていない利用者は 403",
			path:       "/api/private",
			authHeader: []string{"Bearer " + forbiddenToken},
			wantStatus: http.StatusForbidden,
			wantCode:   "FORBIDDEN",
		},
		{
			// どちらを検証したのかが中間装置ごとに変わりうるため、まとめて拒否する。
			name:       "Authorization ヘッダが複数あるのは 401",
			path:       "/api/private",
			authHeader: []string{"Bearer " + validToken, "Bearer " + forbiddenToken},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
		{
			name:       "正しいトークンは通る",
			path:       "/api/private",
			authHeader: []string{"Bearer " + validToken},
			wantStatus: http.StatusOK,
		},
		{
			name:       "URL パラメータを含むルートもトークン無しは 401",
			path:       "/api/items/abc",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "UNAUTHENTICATED",
		},
	}

	public, err := NewPublicRoutes("GET /api/public", "HEAD /api/public")
	if err != nil {
		t.Fatalf("免除リストの作成に失敗しました: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, tt.path, nil)
			for _, v := range tt.authHeader {
				req.Header.Add("Authorization", v)
			}

			rec := httptest.NewRecorder()
			newTestRouter(t, public).ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Fatalf("ステータス: got %d, want %d（body=%s）", rec.Code, tt.wantStatus, rec.Body)
			}

			if tt.wantStatus == http.StatusOK {
				if got := rec.Body.String(); got != testUID {
					t.Errorf("ハンドラが受け取った uid: got %q, want %q", got, testUID)
				}
				return
			}

			// 失敗応答も {data, error} の封筒であること。
			var envelope struct {
				Data  json.RawMessage `json:"data"`
				Error *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("応答の解析に失敗しました: %v（body=%s）", err, rec.Body)
			}
			if envelope.Error == nil {
				t.Fatalf("error が入っているべきです: body=%s", rec.Body)
			}
			if envelope.Error.Code != tt.wantCode {
				t.Errorf("error.code: got %q, want %q", envelope.Error.Code, tt.wantCode)
			}
			if string(envelope.Data) != "null" {
				t.Errorf("失敗時の data: got %s, want null", envelope.Data)
			}

			// 401 には WWW-Authenticate を付ける（RFC 9110 §11.6.1）。
			wantChallenge := tt.wantStatus == http.StatusUnauthorized
			if hasChallenge := rec.Header().Get("WWW-Authenticate") != ""; hasChallenge != wantChallenge {
				t.Errorf("WWW-Authenticate の有無: got %v, want %v", hasChallenge, wantChallenge)
			}
		})
	}
}

// TestAuthErrorsDoNotLeakDetails は拒否の応答に内部情報が載らないことを確かめる。
func TestAuthErrorsDoNotLeakDetails(t *testing.T) {
	t.Parallel()

	public, err := NewPublicRoutes("GET /api/public")
	if err != nil {
		t.Fatalf("免除リストの作成に失敗しました: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/private", nil)
	req.Header.Set("Authorization", "Bearer "+forbiddenToken)

	rec := httptest.NewRecorder()
	newTestRouter(t, public).ServeHTTP(rec, req)

	body := rec.Body.String()
	for _, leak := range []string{forbiddenToken, "allowed@example.com", "example.com"} {
		if strings.Contains(body, leak) {
			t.Errorf("応答に %q が含まれています: %s", leak, body)
		}
	}
}

// TestExemptRouteSkipsAuthentication は免除ルートが認証を通らずに到達することを確かめる。
func TestExemptRouteSkipsAuthentication(t *testing.T) {
	t.Parallel()

	public, err := NewPublicRoutes("GET /api/public", "HEAD /api/public")
	if err != nil {
		t.Fatalf("免除リストの作成に失敗しました: %v", err)
	}

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			t.Parallel()

			req := httptest.NewRequestWithContext(t.Context(), method, "/api/public", nil)
			rec := httptest.NewRecorder()
			newTestRouter(t, public).ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータス: got %d, want 200", rec.Code)
			}
		})
	}
}

// TestExemptionIsPerMethod は免除がメソッド単位であることを確かめる。
// パス単位にすると、同じパスに後から生えた書き込み系メソッドが無認証になる。
func TestExemptionIsPerMethod(t *testing.T) {
	t.Parallel()

	// GET だけを免除し、HEAD /api/public は免除しない。
	public, err := NewPublicRoutes("GET /api/public")
	if err != nil {
		t.Fatalf("免除リストの作成に失敗しました: %v", err)
	}

	req := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/api/public", nil)
	rec := httptest.NewRecorder()
	newTestRouter(t, public).ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("HEAD /api/public: got %d, want 401（免除は GET のみ）", rec.Code)
	}
}

// TestEncodedPathDoesNotBypassAuth は、パスのエンコード差で免除判定がずれないことを確かめる。
//
// TICKET-002 の QA で「GET /api/hea%6Cth は 404 になるがログには /api/health と出る」
// ことが実測されている。免除をパス文字列で比較していると、この差が
// 認証バイパスの入口になる。RoutePattern で判定していれば、
// **ルートが決まらない限り免除されない**。
func TestEncodedPathDoesNotBypassAuth(t *testing.T) {
	t.Parallel()

	public, err := NewPublicRoutes("GET /api/public")
	if err != nil {
		t.Fatalf("免除リストの作成に失敗しました: %v", err)
	}

	// %70ublic は "public" のエンコード。デコード後の文字列（r.URL.Path）は
	// "/api/public" になるが、chi の照合は RawPath を優先するため一致しない。
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/%70ublic", nil)
	rec := httptest.NewRecorder()
	newTestRouter(t, public).ServeHTTP(rec, req)

	// ルートに一致しない以上ハンドラへは行かない。**200 になってはならない。**
	if rec.Code == http.StatusOK {
		t.Fatalf("エンコードされたパスがハンドラに到達しました: got %d", rec.Code)
	}
}

// TestAuthWithoutRouteContextIsFailClosed は、chi のルート情報が無い状況で
// 免除されない（=認証必須側に倒れる）ことを確かめる。
//
// ミドルウェアをルート照合より前に置いてしまった場合がこれに当たる。
// 「一致しなければ免除」の実装だと、この構成で**全エンドポイントが無認証**になる。
func TestAuthWithoutRouteContextIsFailClosed(t *testing.T) {
	t.Parallel()

	public, err := NewPublicRoutes("GET /api/public")
	if err != nil {
		t.Fatalf("免除リストの作成に失敗しました: %v", err)
	}

	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	handler := Auth(logger, fakeAuthenticator{}, public)(
		http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	// chi のルータを通さない = RouteContext が無い。
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/public", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("ルート情報が無いのに免除されました: got %d, want 401", rec.Code)
	}
}
