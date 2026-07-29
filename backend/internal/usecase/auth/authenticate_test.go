package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
	"github.com/kalKun24/octant-cissp/backend/internal/domain/repository"
)

// fakeVerifier は repository.TokenVerifier のテスト用実装。
// トークン文字列をキーに、返す利用者かエラーを決める。
type fakeVerifier struct {
	user *entity.AuthUser
	err  error
	// calls は呼び出し回数。空トークンで検証器を呼んでいないことの確認に使う。
	calls int
}

func (f *fakeVerifier) VerifyIDToken(_ context.Context, _ string) (*entity.AuthUser, error) {
	f.calls++
	return f.user, f.err
}

func TestAuthenticate(t *testing.T) {
	t.Parallel()

	const allowedEmail = "allowed@example.com"

	tests := []struct {
		name     string
		token    string
		verifier *fakeVerifier
		wantErr  error
		wantUID  string
		// wantVerifierCalls は検証器を呼んだ回数の期待値。
		wantVerifierCalls int
	}{
		{
			name:  "許可メールの検証済みトークンは通す",
			token: "valid",
			verifier: &fakeVerifier{user: &entity.AuthUser{
				UID: "uid-1", Email: allowedEmail, EmailVerified: true,
			}},
			wantUID:           "uid-1",
			wantVerifierCalls: 1,
		},
		{
			name:  "許可メールの大文字小文字は区別しない",
			token: "valid",
			verifier: &fakeVerifier{user: &entity.AuthUser{
				UID: "uid-1", Email: "ALLOWED@EXAMPLE.COM", EmailVerified: true,
			}},
			wantUID:           "uid-1",
			wantVerifierCalls: 1,
		},
		{
			name:              "空のトークンは検証器を呼ばずに 401 相当",
			token:             "",
			verifier:          &fakeVerifier{},
			wantErr:           ErrUnauthenticated,
			wantVerifierCalls: 0,
		},
		{
			name:              "検証に失敗したトークンは 401 相当",
			token:             "broken",
			verifier:          &fakeVerifier{err: repository.ErrInvalidToken},
			wantErr:           ErrUnauthenticated,
			wantVerifierCalls: 1,
		},
		{
			name:  "email_verified が偽なら 401 相当",
			token: "unverified",
			verifier: &fakeVerifier{user: &entity.AuthUser{
				UID: "uid-2", Email: allowedEmail, EmailVerified: false,
			}},
			wantErr:           ErrUnauthenticated,
			wantVerifierCalls: 1,
		},
		{
			name:  "許可メール外は 403 相当",
			token: "outsider",
			verifier: &fakeVerifier{user: &entity.AuthUser{
				UID: "uid-3", Email: "outsider@example.com", EmailVerified: true,
			}},
			wantErr:           ErrNotAllowed,
			wantVerifierCalls: 1,
		},
		{
			name:  "メールが空なら 403 相当",
			token: "noemail",
			verifier: &fakeVerifier{user: &entity.AuthUser{
				UID: "uid-4", Email: "", EmailVerified: true,
			}},
			wantErr:           ErrNotAllowed,
			wantVerifierCalls: 1,
		},
		{
			name:  "uid が空なら 401 相当",
			token: "nouid",
			verifier: &fakeVerifier{user: &entity.AuthUser{
				UID: "", Email: allowedEmail, EmailVerified: true,
			}},
			wantErr:           ErrUnauthenticated,
			wantVerifierCalls: 1,
		},
		{
			name:              "検証器が nil を返したら 401 相当",
			token:             "nil",
			verifier:          &fakeVerifier{},
			wantErr:           ErrUnauthenticated,
			wantVerifierCalls: 1,
		},
	}

	allowlist, err := entity.NewEmailAllowlist(allowedEmail)
	if err != nil {
		t.Fatalf("許可リストの作成に失敗しました: %v", err)
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			authenticator := NewAuthenticator(tt.verifier, allowlist)
			user, err := authenticator.Authenticate(t.Context(), tt.token)

			if tt.verifier.calls != tt.wantVerifierCalls {
				t.Errorf("検証器の呼び出し回数: got %d, want %d", tt.verifier.calls, tt.wantVerifierCalls)
			}

			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("エラー: got %v, want %v にマッチ", err, tt.wantErr)
				}
				if user != nil {
					t.Errorf("失敗時に利用者を返しています: %+v", user)
				}
				// 401 と 403 は排他であること。取り違えるとアクセス制御の意味が変わる。
				if errors.Is(err, ErrUnauthenticated) && errors.Is(err, ErrNotAllowed) {
					t.Error("401 と 403 の両方に該当するエラーを返しています")
				}
				return
			}

			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if user == nil {
				t.Fatal("利用者が返っていません")
			}
			if user.UID != tt.wantUID {
				t.Errorf("UID: got %q, want %q", user.UID, tt.wantUID)
			}
		})
	}
}

// TestAuthenticateWithZeroAllowlist は許可リストが未初期化のときに誰も通さないことを確かめる。
func TestAuthenticateWithZeroAllowlist(t *testing.T) {
	t.Parallel()

	verifier := &fakeVerifier{user: &entity.AuthUser{
		UID: "uid-1", Email: "anyone@example.com", EmailVerified: true,
	}}

	authenticator := NewAuthenticator(verifier, entity.EmailAllowlist{})

	if _, err := authenticator.Authenticate(t.Context(), "valid"); !errors.Is(err, ErrNotAllowed) {
		t.Fatalf("エラー: got %v, want ErrNotAllowed にマッチ", err)
	}
}
