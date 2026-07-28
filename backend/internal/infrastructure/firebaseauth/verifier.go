// Package firebaseauth は Firebase Authentication の ID トークン検証を担う。
//
// **このパッケージだけが Firebase Admin SDK を知っている。**
// 上位の層へは domain の型（entity.AuthUser）だけを返す。
package firebaseauth

import (
	"context"
	"errors"
	"fmt"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"

	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
	"github.com/kalKun24/octant-cissp/backend/internal/domain/repository"
)

// Verifier は Firebase Admin SDK による repository.TokenVerifier の実装。
type Verifier struct {
	client *auth.Client
}

// domain 側のインターフェースを満たしていることをコンパイル時に確かめる。
var _ repository.TokenVerifier = (*Verifier)(nil)

// NewVerifier は Firebase Admin SDK を初期化して Verifier を作る。
//
// projectID は必須。ID トークンの `iss` と `aud` の照合に使うため、
// SDK の自動検出（メタデータサーバや ADC）に委ねると、検出に失敗した環境で
// 照合が緩む可能性がある。**設定から明示的に渡す。**
//
// 認証情報は Application Default Credentials から取る。Cloud Run では
// サービスアカウントが自動的に付く。ローカルでは環境変数
// FIREBASE_AUTH_EMULATOR_HOST が設定されていれば SDK がエミュレータへ向く
// （この分岐は SDK 内部のものであり、アプリ側には検証をスキップする経路は無い）。
func NewVerifier(ctx context.Context, projectID string) (*Verifier, error) {
	if projectID == "" {
		return nil, errors.New("プロジェクト ID が空です")
	}

	app, err := firebase.NewApp(ctx, &firebase.Config{ProjectID: projectID})
	if err != nil {
		return nil, fmt.Errorf("firebase.NewApp に失敗しました: %w", err)
	}

	client, err := app.Auth(ctx)
	if err != nil {
		return nil, fmt.Errorf("auth クライアントの生成に失敗しました: %w", err)
	}

	return &Verifier{client: client}, nil
}

// VerifyIDToken は ID トークンを検証し、検証済みの利用者を返す。
//
// SDK が署名・発行者（iss）・対象（aud）・有効期限（exp / iat）を検証する。
// 失敗の理由は repository.ErrInvalidToken に畳んで返す。理由の内訳を
// 上位に伝えないのは、応答がいずれも 401 であり、
// 細分化すると検証ロジックの推測材料を与えるため。
func (v *Verifier) VerifyIDToken(ctx context.Context, idToken string) (*entity.AuthUser, error) {
	if idToken == "" {
		return nil, fmt.Errorf("ID トークンが空です: %w", repository.ErrInvalidToken)
	}

	// VerifyIDToken は失効・無効化の確認を行わない版。
	// エミュレータ利用時のみ SDK が追加で失効確認の RPC を撃つ（SDK 側の実装）。
	token, err := v.client.VerifyIDToken(ctx, idToken)
	if err != nil {
		// エラー文にはトークンの中身が含まれうるため、%w でラップして
		// 上位へ返すだけにとどめ、応答にもログにも出さない。
		return nil, fmt.Errorf("ID トークンの検証に失敗しました: %w", newInvalidTokenError(err))
	}

	return &entity.AuthUser{
		UID:           token.UID,
		Email:         claimString(token.Claims, "email"),
		EmailVerified: claimBool(token.Claims, "email_verified"),
	}, nil
}

// newInvalidTokenError は SDK のエラーを ErrInvalidToken として扱えるようにする。
func newInvalidTokenError(err error) error {
	return fmt.Errorf("%w: %w", repository.ErrInvalidToken, err)
}

// claimString はクレームを文字列として取り出す。型が違えば空文字を返す。
func claimString(claims map[string]any, name string) string {
	value, ok := claims[name].(string)
	if !ok {
		return ""
	}
	return value
}

// claimBool はクレームを真偽値として取り出す。
//
// **型が違う場合は false を返す**（fail-closed）。email_verified が
// 文字列 "true" などで入っていても真として扱わない。
func claimBool(claims map[string]any, name string) bool {
	value, ok := claims[name].(bool)
	if !ok {
		return false
	}
	return value
}
