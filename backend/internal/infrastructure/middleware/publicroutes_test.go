package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewPublicRoutes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entries []string
		wantErr bool
	}{
		{name: "正しい形式", entries: []string{"GET /api/health"}},
		{name: "複数件", entries: []string{"GET /api/health", "HEAD /api/health"}},
		{name: "空の集合", entries: nil},
		{name: "メソッドが無い", entries: []string{"/api/health"}, wantErr: true},
		{name: "パターンが無い", entries: []string{"GET "}, wantErr: true},
		{name: "メソッドが小文字", entries: []string{"get /api/health"}, wantErr: true},
		{name: "パターンが / で始まらない", entries: []string{"GET api/health"}, wantErr: true},
		{name: "ワイルドカードは拒否", entries: []string{"GET /api/*"}, wantErr: true},
		{name: "URL パラメータは拒否", entries: []string{"GET /api/notes/{noteID}"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewPublicRoutes(tt.entries...)
			if (err != nil) != tt.wantErr {
				t.Fatalf("エラー: got %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestPublicRoutesIsExemptIsFailClosed は判定材料が無い場合に免除しないことを確かめる。
//
// **この関数が true 側に倒れると、全エンドポイントが無認証になる。**
func TestPublicRoutesIsExemptIsFailClosed(t *testing.T) {
	t.Parallel()

	public, err := NewPublicRoutes("GET /api/health")
	if err != nil {
		t.Fatalf("免除リストの作成に失敗しました: %v", err)
	}

	// chi のルータを通していないため RouteContext が無い。
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/health", nil)

	if public.IsExempt(req) {
		t.Error("ルート情報が無いのに免除されました")
	}
	if public.IsExempt(nil) {
		t.Error("リクエストが nil なのに免除されました")
	}

	var empty PublicRoutes
	if empty.IsExempt(req) {
		t.Error("空の免除リストが免除しました")
	}
}
