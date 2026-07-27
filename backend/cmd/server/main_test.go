package main

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kalKun24/octant-cissp/backend/internal/config"
)

func TestCloudSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
		want  string
	}{
		{name: "DEBUG", level: slog.LevelDebug, want: "DEBUG"},
		{name: "INFO", level: slog.LevelInfo, want: "INFO"},
		{name: "WARN は Cloud Logging では WARNING", level: slog.LevelWarn, want: "WARNING"},
		{name: "ERROR", level: slog.LevelError, want: "ERROR"},
		{name: "既定より低いレベルは DEBUG に丸める", level: slog.LevelDebug - 4, want: "DEBUG"},
		{name: "既定より高いレベルは ERROR に丸める", level: slog.LevelError + 4, want: "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := cloudSeverity(tt.level); got != tt.want {
				t.Errorf("cloudSeverity(%v): got %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

// TestToCloudLoggingKeys は、実際に出力される JSON が Cloud Logging の
// 期待するキー（severity / message）になっていることを確かめる。
// level / msg のままだと全ログが DEFAULT 重大度に落ちるため、退行を検知する。
func TestToCloudLoggingKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: toCloudLogging,
	}))

	logger.Error("パニックから復帰しました", slog.String("path", "/api/health"))

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("ログの解析に失敗しました: %v", err)
	}

	if got["severity"] != "ERROR" {
		t.Errorf("severity: got %v, want %q", got["severity"], "ERROR")
	}
	if got["message"] != "パニックから復帰しました" {
		t.Errorf("message: got %v", got["message"])
	}
	for _, key := range []string{"level", "msg"} {
		if _, ok := got[key]; ok {
			t.Errorf("%q が残っています。Cloud Logging は severity / message を読みます", key)
		}
	}
	// 独自フィールドは変換対象外であること。
	if got["path"] != "/api/health" {
		t.Errorf("path: got %v, want %q", got["path"], "/api/health")
	}
}

// TestRouterRouting は TICKET-002 で決めたルーティング方針を固定する。
//
//   - 405 には Allow ヘッダを付ける（RFC 9110 §15.5.6 の MUST）
//   - HEAD は /api/health だけ登録し、ボディを返さない
//   - 末尾スラッシュは別パスとして 404 にする（StripSlashes は使わない）
func TestRouterRouting(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantAllow  string
		wantBody   bool
		wantCode   string
	}{
		{
			name:       "GET /api/health は 200 と封筒を返す",
			method:     http.MethodGet,
			path:       "/api/health",
			wantStatus: http.StatusOK,
			wantBody:   true,
		},
		{
			name:       "HEAD /api/health は 200 でボディを返さない",
			method:     http.MethodHead,
			path:       "/api/health",
			wantStatus: http.StatusOK,
			wantBody:   false,
		},
		{
			name:       "POST /api/health は 405 と Allow ヘッダを返す",
			method:     http.MethodPost,
			path:       "/api/health",
			wantStatus: http.StatusMethodNotAllowed,
			wantAllow:  "GET, HEAD",
			wantBody:   true,
			wantCode:   "METHOD_NOT_ALLOWED",
		},
		{
			name:       "末尾スラッシュは別パスとして 404 になる",
			method:     http.MethodGet,
			path:       "/api/health/",
			wantStatus: http.StatusNotFound,
			wantBody:   true,
			wantCode:   "NOT_FOUND",
		},
		{
			name:       "未登録のパスは 404 になる",
			method:     http.MethodGet,
			path:       "/unknown",
			wantStatus: http.StatusNotFound,
			wantBody:   true,
			wantCode:   "NOT_FOUND",
		},
		{
			// 基底パスは /api だけ。二重登録すると openapi.yaml に無いパスが
			// 実装に生えることになるため、プレフィックス無しは 404 のままにする。
			name:       "基底パスを欠いた /health は 404 になる",
			method:     http.MethodGet,
			path:       "/health",
			wantStatus: http.StatusNotFound,
			wantBody:   true,
			wantCode:   "NOT_FOUND",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			router := newTestRouter()
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("ステータス: got %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Errorf("Allow: got %q, want %q", got, tt.wantAllow)
			}

			body, err := io.ReadAll(rec.Body)
			if err != nil {
				t.Fatalf("ボディの読み出しに失敗しました: %v", err)
			}
			if tt.wantBody != (len(body) > 0) {
				t.Fatalf("ボディの有無: got %d バイト, want %v", len(body), tt.wantBody)
			}
			if !tt.wantBody {
				return
			}

			// 成否にかかわらず {data, error} の封筒で返ること。
			var envelope struct {
				Data  json.RawMessage `json:"data"`
				Error *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if err := json.Unmarshal(body, &envelope); err != nil {
				t.Fatalf("レスポンスの解析に失敗しました: %v（body=%s）", err, body)
			}

			if tt.wantCode == "" {
				if envelope.Error != nil {
					t.Errorf("error は null であるべきです: got %+v", envelope.Error)
				}
				return
			}
			if envelope.Error == nil {
				t.Fatalf("error が入っているべきです: body=%s", body)
			}
			if envelope.Error.Code != tt.wantCode {
				t.Errorf("error.code: got %q, want %q", envelope.Error.Code, tt.wantCode)
			}
			if envelope.Error.Message == "" {
				t.Error("error.message が空です")
			}
			if string(envelope.Data) != "null" {
				t.Errorf("失敗時の data: got %s, want null", envelope.Data)
			}
		})
	}
}

// newTestRouter はテスト用のルータを作る。ログは捨てる。
func newTestRouter() http.Handler {
	logger := slog.New(slog.NewJSONHandler(io.Discard, nil))
	return newRouter(logger, &config.Config{Revision: "test-revision"})
}
