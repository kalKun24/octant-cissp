// Package entity はエンティティとドメインルールを持つ。
// **標準ライブラリ以外を import しない**（CLAUDE.md「アーキテクチャ規約」）。
package entity

import (
	"fmt"
	"strings"
)

// AuthUser は検証済み ID トークンから取り出した利用者。
//
// **この構造体の値はトークンの検証を通った後にだけ作ること。**
// リクエストボディやパスパラメータ由来の uid を入れてはならない。
type AuthUser struct {
	// UID は Firebase Authentication の利用者 ID。個人データのパスはこの値だけで組み立てる。
	UID string
	// Email はトークンに含まれるメールアドレス。**ログに出さない。**
	Email string
	// EmailVerified はメールアドレスが確認済みかどうか。
	EmailVerified bool
}

// EmailAllowlist は API の利用を許可するメールアドレスの集合。
//
// 許可メールは環境変数 `ALLOWED_EMAILS` で与える。**コードに埋めない。**
type EmailAllowlist struct {
	// emails は正規化済みのメールアドレスをキーに持つ。
	// ゼロ値（nil）の許可リストは誰も許可しない（fail-closed）。
	emails map[string]struct{}
}

// NewEmailAllowlist はカンマ区切りの文字列から許可リストを作る。
//
// **入力は設定（環境変数 ALLOWED_EMAILS）であり、運用者が書いたもの**なので、
// 要素の前後の空白は区切り文字の書き方の揺れとみなして落とす。
// 空白のみの要素は無視する。**1件も残らない場合はエラーを返す**。
// 「誰も許可しない設定」で黙って起動すると、設定漏れなのか意図なのかを
// 運用時に区別できないため、起動時に気づけるようにしている。
//
// 照合用の鍵は canonicalizeEmail で作る。トークン側とまったく同じ関数を通すことで、
// **両者の正規化がずれて意図しない一致が起きる余地を無くしている。**
func NewEmailAllowlist(csv string) (EmailAllowlist, error) {
	emails := make(map[string]struct{})

	for _, raw := range strings.Split(csv, ",") {
		// 設定由来の入力に限り、要素の前後の空白を落とす。
		entry := strings.TrimSpace(raw)
		if entry == "" {
			continue
		}

		key, ok := canonicalizeEmail(entry)
		if !ok {
			// 値そのものは返さない（設定内容がログや応答に載るのを避けるため）。
			return EmailAllowlist{}, fmt.Errorf(
				"空白・制御文字・非 ASCII 文字を含む要素があります（ASCII のメールアドレスだけを指定してください）")
		}
		if err := validateEmail(key); err != nil {
			return EmailAllowlist{}, err
		}
		emails[key] = struct{}{}
	}

	if len(emails) == 0 {
		return EmailAllowlist{}, fmt.Errorf("許可するメールアドレスが1件も指定されていません")
	}

	return EmailAllowlist{emails: emails}, nil
}

// Allows は email が許可リストに含まれるかを返す。
//
// **引数はトークン由来の信用できない入力である。**
// ここで前後の空白を落としてはならない。落とすと ` dev@example.com `（前後に空白）で
// 作った**別 uid のアカウント**が dev@example.com として通ってしまう
// （IdP が空白付きメールでの登録を許すことは Auth エミュレータで実測済み）。
// このプロジェクトの認可境界はこの1関数しかないため、
// **成立を IdP 側のバリデーションに依存させない。**
//
// **ゼロ値の EmailAllowlist は常に false を返す**（fail-closed）。
// 許可リストの構築に失敗した状態が、うっかり「全員許可」にならないようにするため。
func (a EmailAllowlist) Allows(email string) bool {
	key, ok := canonicalizeEmail(email)
	if !ok {
		return false
	}
	_, found := a.emails[key]
	return found
}

// Size は許可されているメールアドレスの件数を返す。
// 件数は起動ログに出してよい（アドレスそのものは出さない）。
func (a EmailAllowlist) Size() int {
	return len(a.emails)
}

// canonicalizeEmail は照合用の鍵を作る。作れない入力は ok=false を返す。
//
// **切り詰め（trim）は一切しない。** 受け付けるのは
// 「空白・制御文字を含まない ASCII 印字可能文字だけの文字列」で、
// 大文字は ASCII の範囲でのみ小文字化する。
//
// 意図的に strings.ToLower を使っていない。あれは Unicode 対応であり、
// 例えば U+212A（ケルビン記号 K）を 'k' に畳む。そのため許可リストが
// kal@example.com のとき、攻撃者が "Kal@example.com" で登録した
// 別アカウントが一致してしまう。ASCII に限れば、この種の畳み込みは起きない。
//
// 空白類を弾く判定に unicode.IsSpace を使っていないのも同じ理由で、
// 非 ASCII をまとめて拒否するほうが判定が単純で漏れがない
// （NBSP U+00A0 やゼロ幅スペース U+200B も非 ASCII として弾かれる）。
//
// RFC 5321 上はローカル部が大小を区別するが、実運用のプロバイダ
// （Google を含む）は区別しない。区別すると同一人物のアドレスが
// 許可リストと一致せず 403 になる事故が起きるため、小文字化は残している。
func canonicalizeEmail(email string) (string, bool) {
	if email == "" {
		return "", false
	}

	// バイト単位で走査する。非 ASCII は先頭バイトが 0x80 以上になるため、
	// UTF-8 のデコードをせずにこのループで確実に検出できる。
	buf := make([]byte, 0, len(email))
	for i := range len(email) {
		c := email[i]
		switch {
		case c > 0x7e || c <= ' ':
			// 非 ASCII・制御文字・半角空白・DEL。いずれも受け付けない。
			return "", false
		case c >= 'A' && c <= 'Z':
			buf = append(buf, c+('a'-'A'))
		default:
			buf = append(buf, c)
		}
	}

	return string(buf), true
}

// validateEmail は許可リストの要素として最低限の形をしているかを見る。
// メールアドレスの完全な検証はしない（発行元は Firebase であり、ここは設定ミスの検出が目的）。
// 引数は canonicalizeEmail を通した後の値であること。
func validateEmail(email string) error {
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" {
		return fmt.Errorf("メールアドレスの形式ではない要素が含まれています")
	}
	if strings.Contains(domain, "@") {
		return fmt.Errorf("メールアドレスの形式ではない要素が含まれています")
	}
	if !strings.Contains(domain, ".") {
		return fmt.Errorf("メールアドレスの形式ではない要素が含まれています")
	}
	return nil
}
