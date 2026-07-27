// Package middleware は HTTP の横断的関心事（ログ・復帰・認証）を扱う。
package middleware

import (
	"log/slog"
	"net/http"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// RequestLogger は1リクエストにつき1行の構造化ログを出す。
//
// ノート本文・メールアドレス・トークンは記録しない（CLAUDE.md「品質」）。
// クエリ文字列も検索語がそのまま残るため出力しない。
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			next.ServeHTTP(ww, r)

			logger.LogAttrs(r.Context(), slog.LevelInfo, "リクエストを処理しました",
				slog.String("trace_id", chimw.GetReqID(r.Context())),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", ww.Status()),
				slog.Int64("latency_ms", time.Since(start).Milliseconds()),
			)
		})
	}
}
