package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kalKun24/octant-cissp/backend/internal/interface/openapi"
)

func TestHealthGetHealth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		revision     string
		wantRevision string
	}{
		{name: "リビジョンが渡されればそのまま返す", revision: "octant-00001-abc", wantRevision: "octant-00001-abc"},
		{name: "リビジョンが空なら unknown を返す", revision: "", wantRevision: "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rec := httptest.NewRecorder()
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil)

			NewHealth(tt.revision).GetHealth(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータス: got %d, want %d", rec.Code, http.StatusOK)
			}
			if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
				t.Errorf("Content-Type: got %q, want %q", got, want)
			}

			var body struct {
				Data  openapi.Health `json:"data"`
				Error *openapi.Error `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("レスポンスの解析に失敗しました: %v", err)
			}

			if body.Error != nil {
				t.Errorf("error は null であるべきです: got %+v", body.Error)
			}
			if body.Data.Status != openapi.Ok {
				t.Errorf("status: got %q, want %q", body.Data.Status, openapi.Ok)
			}
			if body.Data.Revision != tt.wantRevision {
				t.Errorf("revision: got %q, want %q", body.Data.Revision, tt.wantRevision)
			}
		})
	}
}

// TestHealthHeadHealth は HEAD が GET と同じステータスをボディなしで返すことを確かめる。
// httptest.ResponseRecorder は net/http のような HEAD 向けのボディ抑制を行わないため、
// ハンドラ自身が書き出していないことをここで検証できる。
func TestHealthHeadHealth(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(t.Context(), http.MethodHead, "/api/health", nil)

	NewHealth("octant-00001-abc").HeadHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ステータス: got %d, want %d", rec.Code, http.StatusOK)
	}
	if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
		t.Errorf("Content-Type: got %q, want %q", got, want)
	}
	if got := rec.Body.Len(); got != 0 {
		t.Errorf("ボディ: got %d バイト, want 0（HEAD はボディを返しません）", got)
	}
}
