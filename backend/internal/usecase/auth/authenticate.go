// Package auth は ID トークンから利用者を確定させるユースケースを持つ。
//
// このパッケージは HTTP を知らない。ステータスコードへの写像は
// interface / infrastructure 側の責務（CLAUDE.md「アーキテクチャ規約」）。
package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
	"github.com/kalKun24/octant-cissp/backend/internal/domain/repository"
)

// 認証・認可の失敗を表す番兵エラー。HTTP のステータスコードはこの2つで決まる。
var (
	// ErrUnauthenticated は本人を確定できなかったことを表す（401 に対応する）。
	// トークンが無い・壊れている・失効している・メール未確認がこれに当たる。
	ErrUnauthenticated = errors.New("認証されていません")

	// ErrNotAllowed は本人は確定したが利用を許可されていないことを表す（403 に対応する）。
	ErrNotAllowed = errors.New("利用を許可されていません")
)

// Authenticator は ID トークンから利用者を確定させる。
type Authenticator struct {
	verifier repository.TokenVerifier
	allowed  entity.EmailAllowlist
}

// NewAuthenticator は Authenticator を作る。依存はコンストラクタで受け取る（DI）。
func NewAuthenticator(verifier repository.TokenVerifier, allowed entity.EmailAllowlist) *Authenticator {
	return &Authenticator{verifier: verifier, allowed: allowed}
}

// Authenticate は ID トークンを検証し、利用が許可された利用者を返す。
//
// 判定は次の順序で行う。**順序には意味がある**。
//
//  1. トークンの検証（署名・発行者・対象・有効期限）→ 失敗は ErrUnauthenticated
//  2. email_verified の確認 → 偽なら ErrUnauthenticated
//  3. 許可メールのホワイトリスト照合 → 不一致は ErrNotAllowed
//
// 2 を 3 より先に置くのは、未確認のメールアドレスは名乗りに過ぎず、
// ホワイトリストとの照合材料にしてはならないため。逆順にすると
// 「他人のアドレスを未確認のまま登録して 403 ではなく 200 を得る」経路ができる。
func (a *Authenticator) Authenticate(ctx context.Context, idToken string) (*entity.AuthUser, error) {
	if idToken == "" {
		return nil, fmt.Errorf("ID トークンが空です: %w", ErrUnauthenticated)
	}

	user, err := a.verifier.VerifyIDToken(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("ID トークンの検証に失敗しました: %w", errors.Join(err, ErrUnauthenticated))
	}
	if user == nil {
		return nil, fmt.Errorf("検証結果が空です: %w", ErrUnauthenticated)
	}
	if user.UID == "" {
		return nil, fmt.Errorf("トークンに uid が含まれていません: %w", ErrUnauthenticated)
	}
	if !user.EmailVerified {
		return nil, fmt.Errorf("メールアドレスが未確認です: %w", ErrUnauthenticated)
	}
	if !a.allowed.Allows(user.Email) {
		return nil, fmt.Errorf("許可メールに含まれていません: %w", ErrNotAllowed)
	}

	return user, nil
}
