package main

import (
	"log/slog"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/kalKun24/octant-cissp/backend/internal/config"
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

	handler := newRouter(slog.New(slog.DiscardHandler), &config.Config{})

	routes, ok := handler.(chi.Routes)
	if !ok {
		t.Fatalf("newRouter が chi.Routes を返していません: %T", handler)
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
