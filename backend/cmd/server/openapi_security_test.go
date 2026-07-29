package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// specPath は API 仕様の場所。cmd/server から見た相対パス。
var specPath = filepath.Join("..", "..", "..", "api", "openapi.yaml")

// httpMethods は openapi.yaml の path item でオペレーションとして扱うキー。
// これ以外のキー（parameters / summary / description / $ref）はオペレーションではない。
var httpMethods = map[string]struct{}{
	"get": {}, "put": {}, "post": {}, "delete": {},
	"options": {}, "head": {}, "patch": {}, "trace": {},
}

// openAPISpec は security の検証に必要な部分だけを取り出す。
type openAPISpec struct {
	// Security はトップレベルの既定。**これが空だと全オペレーションが無認証になる。**
	Security *[]yaml.Node `yaml:"security"`
	// Paths は path → (method → オペレーション)。
	// path item にはオペレーション以外のキーも来るため、値は生のノードで受ける。
	Paths map[string]map[string]yaml.Node `yaml:"paths"`
}

// operationSecurity はオペレーション単位の security。
//
// ポインタで受けるのは「書かれていない」と「空配列が書かれている」を
// 区別するため。前者はトップレベルの既定を継承（認証必須）、
// 後者が明示的な免除（security: []）を意味する。
type operationSecurity struct {
	Security *[]yaml.Node `yaml:"security"`
}

// TestPublicRoutesMatchOpenAPISecurity は、認証免除の定義が
// **api/openapi.yaml と実装で完全に一致している**ことを検証する。
//
// この2つは今まで人手で同期していた。仕様に security: [] を書き忘れても、
// 実装の publicRouteEntries にだけ足しても、どちらも CI 全緑のまま通ってしまう。
// 特に後者は「仕様上は認証必須のはずのエンドポイントが実際には無認証」という
// 状態を生み、レビューでも気づきにくい。
//
// 集合として完全一致を要求することで、片側だけの変更を必ず落とす。
func TestPublicRoutesMatchOpenAPISecurity(t *testing.T) {
	t.Parallel()

	spec := loadSpec(t)

	// トップレベルの security が無い・空だと、OpenAPI の既定は「認証不要」になる。
	// その状態では個々の security: [] に意味が無くなり、この検証自体が無意味になる。
	if spec.Security == nil || len(*spec.Security) == 0 {
		t.Fatal("openapi.yaml のトップレベルに security がありません。\n" +
			"  既定が「認証不要」になり、書き忘れたエンドポイントが無認証で公開されます。")
	}

	if len(spec.Paths) == 0 {
		t.Fatal("openapi.yaml から paths を1つも読み取れませんでした。" +
			"仕様の構造が変わっていないか確認してください。")
	}

	var (
		exempt    []string
		operation int
	)

	for path, item := range spec.Paths {
		for key, node := range item {
			if _, ok := httpMethods[strings.ToLower(key)]; !ok {
				continue
			}
			operation++

			var op operationSecurity
			if err := node.Decode(&op); err != nil {
				t.Fatalf("%s %s の security を読み取れませんでした: %v", key, path, err)
			}
			if op.Security == nil {
				// 未指定 = トップレベルの既定を継承 = 認証必須。
				continue
			}
			if len(*op.Security) != 0 {
				// security を上書きしているが空ではない。
				// 認証方式の差し替えは想定していないため、気づけるように落とす。
				t.Errorf("%s %s が security を空以外で上書きしています。\n"+
					"  このプロジェクトが想定するのは「継承（認証必須）」か"+
					"「security: []（免除）」の2択だけです。", strings.ToUpper(key), path)
				continue
			}

			// openapi.yaml の paths は基底パスを含まない（基底は servers 側）。
			// chi のルートパターンと突き合わせるため apiBasePath を足す。
			exempt = append(exempt, strings.ToUpper(key)+" "+apiBasePath+path)
		}
	}

	if operation == 0 {
		t.Fatal("openapi.yaml からオペレーションを1つも読み取れませんでした。" +
			"仕様の構造が変わっていないか確認してください。")
	}

	want := append([]string(nil), publicRouteEntries...)
	sort.Strings(want)
	sort.Strings(exempt)

	if strings.Join(exempt, "\n") == strings.Join(want, "\n") {
		return
	}

	for _, only := range difference(exempt, want) {
		t.Errorf("openapi.yaml では security: [] なのに、実装の publicRouteEntries にありません: %s\n"+
			"  仕様上は無認証のはずのエンドポイントが 401 を返します。", only)
	}
	for _, only := range difference(want, exempt) {
		t.Errorf("実装の publicRouteEntries にあるのに、openapi.yaml に security: [] がありません: %s\n"+
			"  **仕様上は認証必須のエンドポイントが、実際には無認証で公開されています。**\n"+
			"  免除をやめるか、openapi.yaml 側に security: [] を明記してください。", only)
	}
}

// loadSpec は api/openapi.yaml を読み込む。
func loadSpec(t *testing.T) openAPISpec {
	t.Helper()

	raw, err := os.ReadFile(specPath)
	if err != nil {
		t.Fatalf("api/openapi.yaml を読めませんでした: %v", err)
	}

	var spec openAPISpec
	if err := yaml.Unmarshal(raw, &spec); err != nil {
		t.Fatalf("api/openapi.yaml の解析に失敗しました: %v", err)
	}

	return spec
}
