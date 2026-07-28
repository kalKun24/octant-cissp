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
// 空白のみの要素は無視する。**1件も残らない場合はエラーを返す**。
// 「誰も許可しない設定」で黙って起動すると、設定漏れなのか意図なのかを
// 運用時に区別できないため、起動時に気づけるようにしている。
func NewEmailAllowlist(csv string) (EmailAllowlist, error) {
	emails := make(map[string]struct{})

	for _, raw := range strings.Split(csv, ",") {
		entry := NormalizeEmail(raw)
		if entry == "" {
			continue
		}
		if err := validateEmail(entry); err != nil {
			// 値そのものは返さない（設定内容がログや応答に載るのを避けるため）。
			return EmailAllowlist{}, err
		}
		emails[entry] = struct{}{}
	}

	if len(emails) == 0 {
		return EmailAllowlist{}, fmt.Errorf("許可するメールアドレスが1件も指定されていません")
	}

	return EmailAllowlist{emails: emails}, nil
}

// Allows は email が許可リストに含まれるかを返す。
//
// **ゼロ値の EmailAllowlist は常に false を返す**（fail-closed）。
// 許可リストの構築に失敗した状態が、うっかり「全員許可」にならないようにするため。
func (a EmailAllowlist) Allows(email string) bool {
	normalized := NormalizeEmail(email)
	if normalized == "" {
		return false
	}
	_, ok := a.emails[normalized]
	return ok
}

// Size は許可されているメールアドレスの件数を返す。
// 件数は起動ログに出してよい（アドレスそのものは出さない）。
func (a EmailAllowlist) Size() int {
	return len(a.emails)
}

// NormalizeEmail は比較のためにメールアドレスを正規化する。
//
// 前後の空白を落とし小文字化する。RFC 5321 上はローカル部が大小を区別するが、
// 実運用のプロバイダ（Google を含む）は区別しないため、区別すると
// 同一人物のアドレスが許可リストと一致せず 403 になる事故が起きる。
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

// validateEmail は許可リストの要素として最低限の形をしているかを見る。
// メールアドレスの完全な検証はしない（発行元は Firebase であり、ここは設定ミスの検出が目的）。
func validateEmail(email string) error {
	local, domain, found := strings.Cut(email, "@")
	if !found || local == "" || domain == "" {
		return fmt.Errorf("メールアドレスの形式ではない要素が含まれています")
	}
	if strings.ContainsAny(email, " \t") || strings.Contains(domain, "@") {
		return fmt.Errorf("メールアドレスの形式ではない要素が含まれています")
	}
	if !strings.Contains(domain, ".") {
		return fmt.Errorf("メールアドレスの形式ではない要素が含まれています")
	}
	return nil
}
