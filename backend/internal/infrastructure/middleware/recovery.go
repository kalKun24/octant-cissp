package middleware

import (
	"log/slog"
	"net/http"
	"runtime/debug"

	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/kalKun24/octant-cissp/backend/internal/interface/dto"
)

// Recovery は panic を 500 に変換する。
// スタックトレースはログにのみ残し、レスポンスには内部情報を一切含めない。
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				rec := recover()
				if rec == nil {
					return
				}
				// ErrAbortHandler は net/http が握る取り決めなのでそのまま伝播させる。
				if rec == http.ErrAbortHandler { //nolint:errorlint // 番兵値との同一性比較
					panic(rec)
				}

				logger.LogAttrs(r.Context(), slog.LevelError, "パニックから復帰しました",
					slog.String("trace_id", chimw.GetReqID(r.Context())),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
				)

				dto.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
					"サーバ内部でエラーが発生しました。時間をおいて操作をやり直してください。")
			}()

			next.ServeHTTP(w, r)
		})
	}
}
