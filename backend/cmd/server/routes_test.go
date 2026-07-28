package main

import (
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
)

// wantRoutes は登録を許可するルートの全集合。
// api/openapi.yaml のオペレーションと1対1で対応する。
//
// **この一覧に手で行を足す前に、必ず api/openapi.yaml を更新して make gen を実行すること。**
// 先にここへ足すと、仕様に無いエンドポイントが実装に生えたまま通ってしまう。
var wantRoutes = []string{
	"GET /api/health",
	"HEAD /api/health",
}

// TestRegisteredRoutesMatchSpec は「実装 → 仕様」の方向を守る。
//
// 逆方向（仕様にあるのに実装が無い）は handler.Server の
// var _ openapi.ServerInterface アサーションがコンパイル時に検知するが、
// 「仕様に無いのに実装にある」（openapi.yaml を触らずに r.Get(...) を手書きする）は
// gen-check も lint もテストも検知できない。その穴をこのテストで塞ぐ。
func TestRegisteredRoutesMatchSpec(t *testing.T) {
	t.Parallel()

	got := registeredRoutes(t)

	sort.Strings(got)
	want := append([]string(nil), wantRoutes...)
	sort.Strings(want)

	if strings.Join(got, "\n") == strings.Join(want, "\n") {
		return
	}

	for _, extra := range difference(got, want) {
		t.Errorf("仕様に無いルートが登録されています: %s\n"+
			"  api/openapi.yaml に定義してから make gen を実行してください。"+
			"手書きの r.Get(...) を追加していませんか。", extra)
	}
	for _, missing := range difference(want, got) {
		t.Errorf("登録されているはずのルートがありません: %s", missing)
	}
}

// TestNonPublicRoutesRequireAuth は「免除リストに無いルートは必ず 401 になる」ことを、
// **登録されている全ルートを走査して**確かめる。
//
// 認証ミドルウェアは生成コードの Middlewares 経由で一律に適用されるため、
// 個々のエンドポイントで付け忘れは起きない設計になっている。
// このテストはその設計が実際に効いていることを毎回確認するもので、
// TICKET-006 以降でエンドポイントが増えたときに退行を検知する。
//
// 免除リスト（publicRouteEntries）に足す変更を入れると、そのルートが
// ここで「認証不要」側に移る。**差分レビューでそれが見えることに意味がある。**
func TestNonPublicRoutesRequireAuth(t *testing.T) {
	t.Parallel()

	public := make(map[string]struct{}, len(publicRouteEntries))
	for _, entry := range publicRouteEntries {
		public[entry] = struct{}{}
	}

	routes := registeredRoutes(t)
	if len(routes) == 0 {
		t.Fatal("ルートが1つも登録されていません")
	}

	for _, route := range routes {
		t.Run(route, func(t *testing.T) {
			t.Parallel()

			method, path, _ := strings.Cut(route, " ")

			router := newTestRouter(t)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), method, path, nil))

			if _, exempt := public[route]; exempt {
				if rec.Code == http.StatusUnauthorized {
					t.Errorf("免除ルートなのに 401 になりました。"+
						"publicRouteEntries のパターンが chi の RoutePattern と一致していますか: %s", route)
				}
				return
			}

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("トークン無しの %s が %d を返しました（401 であるべきです）。\n"+
					"  認証ミドルウェアを通らないルートが登録されていないか、"+
					"publicRouteEntries に意図せず追加されていないか確認してください。", route, rec.Code)
			}
		})
	}
}

// registeredRoutes は実際にルータへ登録されているルートを "METHOD /pattern" で返す。
func registeredRoutes(t *testing.T) []string {
	t.Helper()

	routes, ok := newTestRouter(t).(chi.Routes)
	if !ok {
		t.Fatal("newRouter が chi.Routes を返していません")
	}

	var got []string
	walk := func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		// chi はサブルータの結合部に "/*" を残すことがあるため落とす。
		route = strings.TrimSuffix(route, "/*")
		got = append(got, method+" "+route)
		return nil
	}
	if err := chi.Walk(routes, walk); err != nil {
		t.Fatalf("ルートの走査に失敗しました: %v", err)
	}

	return got
}

// difference は a のうち b に含まれない要素を返す。
func difference(a, b []string) []string {
	inB := make(map[string]struct{}, len(b))
	for _, v := range b {
		inB[v] = struct{}{}
	}

	var out []string
	for _, v := range a {
		if _, ok := inB[v]; !ok {
			out = append(out, v)
		}
	}

	return out
}
