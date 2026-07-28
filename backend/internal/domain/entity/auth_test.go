package entity

import "testing"

func TestNewEmailAllowlist(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		csv      string
		wantErr  bool
		wantSize int
	}{
		{name: "1件", csv: "a@example.com", wantSize: 1},
		{name: "複数件", csv: "a@example.com,b@example.com", wantSize: 2},
		// 許可リストは設定（環境変数）由来なので、要素前後の空白は区切りの
		// 書き方の揺れとみなして落とす。**トークン側とは扱いが違う。**
		{name: "要素前後の空白は落とす", csv: " a@example.com , b@example.com ", wantSize: 2},
		{name: "大文字は正規化して重複を潰す", csv: "A@Example.COM,a@example.com", wantSize: 1},
		{name: "空の要素は無視する", csv: "a@example.com,,b@example.com,", wantSize: 2},
		{name: "空文字はエラー", csv: "", wantErr: true},
		{name: "カンマだけはエラー", csv: ",,,", wantErr: true},
		{name: "@ が無い要素はエラー", csv: "a@example.com,notanemail", wantErr: true},
		{name: "ローカル部が空はエラー", csv: "@example.com", wantErr: true},
		{name: "ドメイン部が空はエラー", csv: "a@", wantErr: true},
		{name: "ドメインにドットが無いのはエラー", csv: "a@localhost", wantErr: true},
		{name: "@ が2つはエラー", csv: "a@b@example.com", wantErr: true},
		{name: "要素内部に空白があるのはエラー", csv: "a b@example.com", wantErr: true},
		// 非 ASCII を許すと、照合時の畳み込みで別アドレスと一致しうる。
		// **黙って無視せず起動時に落とす**（設定した本人が気づけるように）。
		{name: "非 ASCII を含む要素はエラー", csv: "\u00e9@example.com", wantErr: true},
		{name: "NBSP を含む要素はエラー", csv: "a\u00a0b@example.com", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := NewEmailAllowlist(tt.csv)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("エラーになるべきです: got size=%d", got.Size())
				}
				// エラー時は誰も許可しない値が返ること。
				if got.Allows("a@example.com") {
					t.Error("エラー時に返る許可リストが誰かを許可しています")
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if got.Size() != tt.wantSize {
				t.Errorf("Size(): got %d, want %d", got.Size(), tt.wantSize)
			}
		})
	}
}

func TestEmailAllowlistAllows(t *testing.T) {
	t.Parallel()

	allowed, err := NewEmailAllowlist("Allowed@Example.com, other@example.com")
	if err != nil {
		t.Fatalf("許可リストの作成に失敗しました: %v", err)
	}

	tests := []struct {
		name  string
		email string
		want  bool
	}{
		{name: "完全一致", email: "allowed@example.com", want: true},
		{name: "大文字小文字の違いは吸収する", email: "ALLOWED@EXAMPLE.COM", want: true},
		{name: "別のアドレスは拒否する", email: "attacker@example.com", want: false},
		{name: "部分一致では通さない", email: "allowed@example.com.evil.test", want: false},
		{name: "接頭辞では通さない", email: "xallowed@example.com", want: false},
		{name: "空文字は拒否する", email: "", want: false},
		{name: "空白のみは拒否する", email: "   ", want: false},

		// --- ここから下は認可バイパスの回帰テスト ---
		//
		// **Allows の引数はトークン由来の信用できない入力**である。
		// ここで前後の空白を落とすと、「見た目が同じで uid が違う別アカウント」が
		// 許可リストを通過する。実際に " dev@example.com "（前後に空白）で作った
		// 別 uid のアカウントが 200 を得られる状態だった。
		//
		// Auth エミュレータが空白付きメールでの登録を許すことは実測済みであり、
		// **防壁の成立を IdP 側のバリデーションに依存させない。**
		{name: "前後の空白は吸収しない", email: "  allowed@example.com  ", want: false},
		{name: "先頭の空白は吸収しない", email: " allowed@example.com", want: false},
		{name: "末尾の空白は吸収しない", email: "allowed@example.com ", want: false},
		{name: "先頭のタブは吸収しない", email: "\tallowed@example.com", want: false},
		{name: "末尾のタブは吸収しない", email: "allowed@example.com\t", want: false},
		{name: "末尾の改行は吸収しない", email: "allowed@example.com\n", want: false},
		{name: "末尾の復帰は吸収しない", email: "allowed@example.com\r", want: false},
		{name: "先頭の NBSP は吸収しない", email: "\u00a0allowed@example.com", want: false},
		{name: "末尾の NBSP は吸収しない", email: "allowed@example.com\u00a0", want: false},
		{name: "末尾のゼロ幅スペースは吸収しない", email: "allowed@example.com\u200b", want: false},
		{name: "末尾の全角スペースは吸収しない", email: "allowed@example.com\u3000", want: false},
		{name: "内部の空白は吸収しない", email: "allowed @example.com", want: false},
		{name: "NUL は吸収しない", email: "allowed@example.com\x00", want: false},

		// Unicode 対応の小文字化（strings.ToLower）は U+212A（ケルビン記号）を
		// 'k' へ畳む。それに頼ると、許可アドレスの k を U+212A に差し替えた
		// 別アカウントが一致しうる。小文字化は ASCII の範囲だけで行う。
		{name: "ケルビン記号は k に畳まない", email: "allowed@example.\u212aom", want: false},
		{name: "非 ASCII を含むアドレスは拒否する", email: "allowed\u00e9@example.com", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := allowed.Allows(tt.email); got != tt.want {
				t.Errorf("Allows(%q): got %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

// TestAllowsRejectsEveryWhitespaceVariant は、許可アドレスに空白類を1つ足しただけの
// 入力がすべて拒否されることを網羅的に確かめる。
//
// 認可バイパスの本体はここだった。個別のケースを表に並べるだけだと
// 「表に無い空白文字」が抜けるため、機械的に総当たりする。
func TestAllowsRejectsEveryWhitespaceVariant(t *testing.T) {
	t.Parallel()

	const base = "allowed@example.com"

	allowed, err := NewEmailAllowlist(base)
	if err != nil {
		t.Fatalf("許可リストの作成に失敗しました: %v", err)
	}
	if !allowed.Allows(base) {
		t.Fatalf("前提が崩れています: %q が許可されていません", base)
	}

	// 半角空白・タブ・改行・復帰・垂直タブ・改ページ・NUL・DEL に加え、
	// Unicode の空白類（NEL / NBSP / EN QUAD / ゼロ幅スペース /
	// 行区切り / 段落区切り / 表意文字空白 / BOM）。
	spaces := []string{
		" ", "\t", "\n", "\r", "\v", "\f", "\x00", "\x7f",
		"\u0085", // NEL
		"\u00a0", // NBSP
		"\u2000", // EN QUAD
		"\u200b", // ZERO WIDTH SPACE
		"\u2028", // LINE SEPARATOR
		"\u2029", // PARAGRAPH SEPARATOR
		"\u3000", // IDEOGRAPHIC SPACE
		"\ufeff", // ZERO WIDTH NO-BREAK SPACE (BOM)
	}

	for _, s := range spaces {
		for _, variant := range []string{s + base, base + s, s + base + s} {
			if allowed.Allows(variant) {
				t.Errorf("空白類 %+q を含む %q が許可されました", s, variant)
			}
		}
	}
}

// TestZeroEmailAllowlistDeniesEveryone はゼロ値の許可リストが誰も許可しないことを確かめる。
// **ここが true 側に倒れると、初期化漏れがそのまま全員許可になる。**
func TestZeroEmailAllowlistDeniesEveryone(t *testing.T) {
	t.Parallel()

	var zero EmailAllowlist

	for _, email := range []string{"", "a@example.com", "admin@example.com"} {
		if zero.Allows(email) {
			t.Errorf("ゼロ値の許可リストが %q を許可しました", email)
		}
	}
	if zero.Size() != 0 {
		t.Errorf("Size(): got %d, want 0", zero.Size())
	}
}
