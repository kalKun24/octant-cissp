// Package repository は外側の世界へ出ていくためのインターフェースを定義する。
//
// 実装は infrastructure に置く。**このパッケージに具体的な SDK の型を露出させない**
// （CLAUDE.md「アーキテクチャ規約」）。インターフェースを内側に置くことで、
// usecase が Firebase や Firestore を知らずに済む。
package repository

import (
	"context"
	"errors"

	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
)

// ErrInvalidToken は ID トークンが受け付けられなかったことを表す。
//
// **理由の内訳（署名不正・期限切れ・発行者違い）を呼び出し側に伝えない。**
// 応答はいずれも 401 であり、細かく返すと攻撃者に検証ロジックを教えることになる。
var ErrInvalidToken = errors.New("ID トークンが不正です")

// TokenVerifier は ID トークンを検証して利用者を返す。
//
// 実装は infrastructure/firebaseauth。検証に失敗した場合は
// ErrInvalidToken でラップしたエラーを返すこと。
type TokenVerifier interface {
	// VerifyIDToken は idToken を検証し、検証済みの利用者を返す。
	// 検証に失敗した場合は ErrInvalidToken に一致するエラーを返す。
	VerifyIDToken(ctx context.Context, idToken string) (*entity.AuthUser, error)
}
