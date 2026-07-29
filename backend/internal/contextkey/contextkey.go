// Package contextkey は request-scoped な値の出し入れを1箇所に集める。
//
// キーを非公開の型にしているのは、他パッケージが同じキーで値を差し込めないようにするため。
// **認証済み利用者を context に入れてよいのは認証ミドルウェアだけ**であり、
// ここを通さずに uid を偽装する経路を作らない。
package contextkey

import (
	"context"

	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
)

// key は context のキーの型。非公開にして衝突と外部からの差し込みを防ぐ。
type key int

const authUserKey key = iota

// WithAuthUser は検証済みの利用者を context に入れた新しい context を返す。
//
// **呼び出してよいのは認証ミドルウェアだけ。** ハンドラやユースケースから呼ばない。
func WithAuthUser(ctx context.Context, user *entity.AuthUser) context.Context {
	return context.WithValue(ctx, authUserKey, user)
}

// AuthUser は context から検証済みの利用者を取り出す。
// 認証を通っていない場合は ok が false になる。
func AuthUser(ctx context.Context) (*entity.AuthUser, bool) {
	user, ok := ctx.Value(authUserKey).(*entity.AuthUser)
	if !ok || user == nil {
		return nil, false
	}
	return user, true
}

// UID は context から検証済みの uid を取り出す。
//
// **個人データ（/users/{uid}/**）のパスは必ずこの値で組み立てる。**
// リクエストボディやパスパラメータから来た uid を信用しない（CLAUDE.md「認証・認可」）。
//
// 認証を通っていない場合は ok が false になる。呼び出し側は
// **ok を確認せずに uid を使ってはならない**（空文字がパスに混ざる）。
func UID(ctx context.Context) (string, bool) {
	user, ok := AuthUser(ctx)
	if !ok || user.UID == "" {
		return "", false
	}
	return user.UID, true
}
