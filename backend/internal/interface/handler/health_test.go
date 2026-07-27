package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthGet(t *testing.T) {
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
			req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/health", nil)

			NewHealth(tt.revision).Get(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("ステータス: got %d, want %d", rec.Code, http.StatusOK)
			}
			if got, want := rec.Header().Get("Content-Type"), "application/json; charset=utf-8"; got != want {
				t.Errorf("Content-Type: got %q, want %q", got, want)
			}

			var body struct {
				Data  HealthResponse `json:"data"`
				Error *struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
				t.Fatalf("レスポンスの解析に失敗しました: %v", err)
			}

			if body.Error != nil {
				t.Errorf("error は null であるべきです: got %+v", body.Error)
			}
			if body.Data.Status != "ok" {
				t.Errorf("status: got %q, want %q", body.Data.Status, "ok")
			}
			if body.Data.Revision != tt.wantRevision {
				t.Errorf("revision: got %q, want %q", body.Data.Revision, tt.wantRevision)
			}
		})
	}
}
