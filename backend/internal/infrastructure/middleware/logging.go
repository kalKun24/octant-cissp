// Package middleware は HTTP の横断的関心事（ログ・復帰・認証・セキュリティヘッダ）を扱う。
package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	chimw "github.com/go-chi/chi/v5/middleware"
)

// stateKey は requestState を context に入れるためのキーの型。
type stateKey int

const requestStateKey stateKey = iota

// requestState は1リクエストの間だけ生きる、後から書き足せる記録先。
//
// アクセスログは全体を包む必要がある（404 や 405 も1行残すため）一方、
// uid が決まるのは内側の認証ミドルウェアである。context の値は内側へしか
// 伝わらないため、外側があらかじめ置いた入れ物へ内側が書き込む形にしている。
//
// 書き込みと読み出しが別ゴルーチンになる可能性（ハンドラが goroutine を
// 起こす場合）に備えて atomic で扱う。
type requestState struct {
	uid atomic.Pointer[string]
}

// setLogUID はアクセスログに載せる uid を記録する。
// 記録先が無い場合（アクセスログを通っていない場合）は何もしない。
func setLogUID(ctx context.Context, uid string) {
	state, ok := ctx.Value(requestStateKey).(*requestState)
	if !ok || state == nil {
		return
	}
	state.uid.Store(&uid)
}

// RequestLogger は1リクエストにつき1行の構造化ログを出す。
//
// **ノート本文・メールアドレス・トークンは記録しない**（CLAUDE.md「品質」）。
// クエリ文字列も検索語がそのまま残るため出力しない。
// uid は個人を特定する値だが、CLAUDE.md がログの要求フィールドとして
// 明示しているため記録する（メールアドレスとは別扱い）。
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)

			// 内側の認証ミドルウェアが uid を書き込むための入れ物を仕込む。
			state := &requestState{}
			ctx := context.WithValue(r.Context(), requestStateKey, state)
			r = r.WithContext(ctx)

			next.ServeHTTP(ww, r)

			uid := ""
			if stored := state.uid.Load(); stored != nil {
				uid = *stored
			}

			logger.LogAttrs(ctx, slog.LevelInfo, "リクエストを処理しました",
				slog.String("trace_id", chimw.GetReqID(ctx)),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				// 認証を通っていないリクエストでは空になる。
				slog.String("uid", uid),
				slog.Int("status", ww.Status()),
				slog.Int64("latency_ms", time.Since(start).Milliseconds()),
			)
		})
	}
}
