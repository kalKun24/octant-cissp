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
		{name: "前後の空白は落とす", csv: " a@example.com , b@example.com ", wantSize: 2},
		{name: "大文字は正規化して重複を潰す", csv: "A@Example.COM,a@example.com", wantSize: 1},
		{name: "空の要素は無視する", csv: "a@example.com,,b@example.com,", wantSize: 2},
		{name: "空文字はエラー", csv: "", wantErr: true},
		{name: "カンマだけはエラー", csv: ",,,", wantErr: true},
		{name: "@ が無い要素はエラー", csv: "a@example.com,notanemail", wantErr: true},
		{name: "ローカル部が空はエラー", csv: "@example.com", wantErr: true},
		{name: "ドメイン部が空はエラー", csv: "a@", wantErr: true},
		{name: "ドメインにドットが無いのはエラー", csv: "a@localhost", wantErr: true},
		{name: "@ が2つはエラー", csv: "a@b@example.com", wantErr: true},
		{name: "空白を含む要素はエラー", csv: "a b@example.com", wantErr: true},
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
		{name: "前後の空白は吸収する", email: "  allowed@example.com  ", want: true},
		{name: "別のアドレスは拒否する", email: "attacker@example.com", want: false},
		{name: "部分一致では通さない", email: "allowed@example.com.evil.test", want: false},
		{name: "接頭辞では通さない", email: "xallowed@example.com", want: false},
		{name: "空文字は拒否する", email: "", want: false},
		{name: "空白のみは拒否する", email: "   ", want: false},
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
