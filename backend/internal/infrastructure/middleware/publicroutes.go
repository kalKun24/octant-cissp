package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

// PublicRoutes は認証を免除するルートの集合。
//
// 要素は `"METHOD /route/pattern"`。パターンは **chi に登録したルートパターン**
// であって、リクエストの URL 文字列ではない。
//
// # なぜパス文字列で比較しないか
//
// 同じリクエストでもパスの「見え方」は場所によって違う。chi の照合は
// `r.URL.RawPath`（エスケープされたまま）を優先し、アクセスログは
// `r.URL.Path`（デコード後）を見る。実測では `GET /api/hea%6Cth` は
// ルート照合に失敗して 404 になる一方、ログには `/api/health` と出る。
//
// この差がある状態で「パスが /api/health なら認証不要」と文字列比較すると、
// **照合器が見る文字列と免除判定が見る文字列がずれ、認証バイパスの入口になる。**
// chi がルートを決めた**結果**（RoutePattern）で判定すれば、
// 「実際にどのハンドラへ行くのか」と免除判定が必ず一致する。
type PublicRoutes map[string]struct{}

// NewPublicRoutes は免除するルートの集合を作る。
// 要素の形式は `"METHOD /pattern"`。形式が不正なら**起動時に**エラーにする。
func NewPublicRoutes(entries ...string) (PublicRoutes, error) {
	routes := make(PublicRoutes, len(entries))

	for _, entry := range entries {
		method, pattern, found := strings.Cut(entry, " ")
		if !found || method == "" || pattern == "" {
			return nil, fmt.Errorf("認証免除ルートの形式が不正です: %q（\"METHOD /pattern\" の形で指定してください）", entry)
		}
		if method != strings.ToUpper(method) {
			return nil, fmt.Errorf("認証免除ルートのメソッドは大文字で指定してください: %q", entry)
		}
		if !strings.HasPrefix(pattern, "/") {
			return nil, fmt.Errorf("認証免除ルートのパターンは / で始めてください: %q", entry)
		}
		// ワイルドカードや URL パラメータを含むパターンは免除しない。
		// 1行で広い範囲を無認証にしてしまう事故を、設定の時点で防ぐ。
		if strings.ContainsAny(pattern, "*{}") {
			return nil, fmt.Errorf("認証免除ルートにワイルドカードやパラメータは使えません: %q", entry)
		}
		routes[method+" "+pattern] = struct{}{}
	}

	return routes, nil
}

// IsExempt はこのリクエストが認証免除かどうかを返す。
//
// **判定に必要な材料が揃わない場合は必ず false を返す（fail-closed）。**
// 「一致しなければ免除」ではなく「明示的に一致したときだけ免除」。
// ルート照合の前に呼ばれた場合や、chi のコンテキストが取れない場合は
// 認証必須側に倒れる。
func (p PublicRoutes) IsExempt(r *http.Request) bool {
	if len(p) == 0 || r == nil {
		return false
	}

	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		// ルート照合前、または chi 以外のルータ配下。判定材料が無い。
		return false
	}

	pattern := rctx.RoutePattern()
	if pattern == "" || strings.ContainsAny(pattern, "*{}") {
		// パターンが確定していない、または広い範囲に一致するパターン。
		return false
	}

	_, ok := p[r.Method+" "+pattern]
	return ok
}
