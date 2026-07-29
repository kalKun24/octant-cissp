package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	chimw "github.com/go-chi/chi/v5/middleware"
)

const (
	// requestIDHeader はクライアントとの間でやり取りする相関 ID のヘッダ名。
	requestIDHeader = "X-Request-Id"
	// maxRequestIDLen は受け入れる相関 ID の最大長。
	// ログ1行が肥大化するのと、ログ収集側のフィールド長制限を超えるのを防ぐ。
	maxRequestIDLen = 64
	// generatedIDBytes は自前で生成する相関 ID のバイト数（16 進で 32 文字になる）。
	generatedIDBytes = 16
)

// RequestID はリクエストの相関 ID を決めて context に入れる。
//
// chi 標準の middleware.RequestID は **X-Request-Id をそのまま採用する**。
// クライアントが与えた任意の文字列がログに載ると、
//
//   - 改行や制御文字を仕込んでログを1行偽装される（ログインジェクション）
//   - 極端に長い値でログ基盤を圧迫される
//   - 他人のリクエストと同じ ID を名乗って調査を妨害される
//
// といった問題が起きる。そこで**検証を通った値だけを採用**し、
// 通らなければサーバ側で生成する。値は chi と同じ context キーへ入れるため、
// 既存の chimw.GetReqID がそのまま使える。
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := acceptableRequestID(r)
		if !ok {
			id = generateRequestID()
		}

		// 調査時に突き合わせられるよう、採用した ID を応答にも返す。
		w.Header().Set(requestIDHeader, id)

		ctx := context.WithValue(r.Context(), chimw.RequestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// acceptableRequestID はクライアントが送ってきた相関 ID を検証する。
//
// 受け入れるのは英数字とハイフン・アンダースコア・ドットのみ、1〜64 文字。
// ヘッダが複数ある場合は**どれも採用しない**（どれが正かを決められないため）。
func acceptableRequestID(r *http.Request) (string, bool) {
	values := r.Header.Values(requestIDHeader)
	if len(values) != 1 {
		return "", false
	}

	id := values[0]
	if id == "" || len(id) > maxRequestIDLen {
		return "", false
	}
	for _, c := range []byte(id) {
		switch {
		case c >= '0' && c <= '9',
			c >= 'a' && c <= 'z',
			c >= 'A' && c <= 'Z',
			c == '-', c == '_', c == '.':
		default:
			return "", false
		}
	}

	return id, true
}

// generateRequestID はサーバ側で相関 ID を作る。
// crypto/rand は Go 1.24 以降エラーを返さない（失敗時はプロセスを落とす）。
func generateRequestID() string {
	buf := make([]byte, generatedIDBytes)
	_, _ = rand.Read(buf)
	return hex.EncodeToString(buf)
}
