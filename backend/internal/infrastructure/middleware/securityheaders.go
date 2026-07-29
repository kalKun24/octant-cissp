package middleware

import "net/http"

// SecurityHeaders は全ての応答に共通のセキュリティヘッダを付ける。
//
//   - Cache-Control: no-store
//     この API の応答は認証済み利用者の個人データを含む。共有プロキシや
//     ブラウザのディスクキャッシュに残ると、同じ端末の別利用者や
//     ログアウト後の「戻る」操作で読めてしまう。個別のハンドラで
//     付け忘れると事故になるため、ここで一律に付ける
//   - X-Content-Type-Options: nosniff
//     応答は常に JSON だが、ブラウザが中身から型を推測すると
//     利用者由来の文字列（ノート本文など）が HTML と解釈されうる。
//     推測を止めて Content-Type どおりに扱わせる
//
// **404 / 405 やパニック時の 500 にも付ける必要がある**ため、
// ルート照合より外側（ルータ直下の Use）に置く。
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Cache-Control", "no-store")
		header.Set("X-Content-Type-Options", "nosniff")

		next.ServeHTTP(w, r)
	})
}
