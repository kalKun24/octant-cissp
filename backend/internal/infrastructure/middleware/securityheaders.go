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
//
// # 本番では Firebase Hosting の CDN が前段にいる
//
// ブラウザからの /api/** は Hosting の rewrites を経由して届く（docs/deploy.md）。
// **Hosting の CDN はオリジンの Cache-Control に従ってキャッシュする**ため、
// 「認証済み利用者の応答が CDN に載り、別の利用者へ配られる」ことを防いでいるのは
// 上の no-store 1 行である。ここを緩める変更は、単独では入れないこと
// （Hosting 側の実挙動を curl で確認してから）。
//
// なお *.run.app の直 URL は公開のままなので、この経路を通らない要求もありうる。
// **ミドルウェア側で付けているからこそ、どちらの経路でも同じ保証になる。**
func SecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := w.Header()
		header.Set("Cache-Control", "no-store")
		header.Set("X-Content-Type-Options", "nosniff")

		next.ServeHTTP(w, r)
	})
}
