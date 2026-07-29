package firebaseauth_test

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/kalKun24/octant-cissp/backend/internal/domain/repository"
	"github.com/kalKun24/octant-cissp/backend/internal/infrastructure/firebaseauth"
	"github.com/kalKun24/octant-cissp/backend/test"
)

const testPassword = "emulator-password-1234"

// TestVerifierIntegration は Auth エミュレータが発行した ID トークンを
// Firebase Admin SDK 経由で検証できることを確かめる。
func TestVerifierIntegration(t *testing.T) {
	host, projectID := test.SkipUnlessAuthEmulator(t)

	emulator := test.NewAuthEmulator(t, host, projectID)
	verifier, err := firebaseauth.NewVerifier(t.Context(), projectID)
	if err != nil {
		t.Fatalf("検証器の生成に失敗しました: %v", err)
	}

	verified := emulator.SignUp(t, "verifier-verified@example.com", testPassword, true)
	t.Cleanup(func() { emulator.DeleteUser(t, verified.UID) })

	unverified := emulator.SignUp(t, "verifier-unverified@example.com", testPassword, false)
	t.Cleanup(func() { emulator.DeleteUser(t, unverified.UID) })

	t.Run("検証済みのトークンから uid とメールが取れる", func(t *testing.T) {
		user, err := verifier.VerifyIDToken(t.Context(), verified.IDToken)
		if err != nil {
			t.Fatalf("検証に失敗しました: %v", err)
		}
		if user.UID != verified.UID {
			t.Errorf("UID: got %q, want %q", user.UID, verified.UID)
		}
		if user.Email != verified.Email {
			t.Errorf("Email: got %q, want %q", user.Email, verified.Email)
		}
		if !user.EmailVerified {
			t.Error("EmailVerified: got false, want true")
		}
	})

	t.Run("メール未確認のトークンは EmailVerified が false", func(t *testing.T) {
		user, err := verifier.VerifyIDToken(t.Context(), unverified.IDToken)
		if err != nil {
			t.Fatalf("検証に失敗しました: %v", err)
		}
		if user.EmailVerified {
			t.Error("EmailVerified: got true, want false")
		}
	})

	// **エミュレータは署名を検証しない。** そのため「署名だけ差し替えたトークン」は
	// ここでは拒否されない（実測で確認済み）。署名の検証は本番の SDK 経路でのみ効く。
	//
	// これが、config.Load が APP_ENV=local 以外で FIREBASE_AUTH_EMULATOR_HOST を
	// 拒否している理由そのものである。dev / prod にこの環境変数が紛れ込めば、
	// 誰でも自作トークンで認証を通せる状態になる。
	//
	// 一方、**発行者・対象・有効期限の検証はエミュレータでも効く**ため、
	// クレームを書き換えたトークンで検証経路が生きていることを確かめる。
	invalid := []struct {
		name  string
		token string
	}{
		{name: "空文字", token: ""},
		{name: "JWT ですらない文字列", token: "not-a-jwt"},
		{name: "セグメント数が足りない", token: "aaa.bbb"},
		{name: "ペイロードが JSON ではない", token: replacePayload(verified.IDToken, []byte("not-json"))},
		{name: "別プロジェクト宛のトークン", token: withClaims(t, verified.IDToken, map[string]any{
			"aud": "some-other-project",
		})},
		{name: "発行者が別のトークン", token: withClaims(t, verified.IDToken, map[string]any{
			"iss": "https://securetoken.google.com/some-other-project",
		})},
		{name: "期限切れのトークン", token: withClaims(t, verified.IDToken, map[string]any{
			"exp": time.Now().Add(-1 * time.Hour).Unix(),
		})},
		{name: "uid が空のトークン", token: withClaims(t, verified.IDToken, map[string]any{
			"sub": "",
		})},
	}

	for _, tt := range invalid {
		t.Run("不正なトークンを拒否する/"+tt.name, func(t *testing.T) {
			user, err := verifier.VerifyIDToken(t.Context(), tt.token)
			if err == nil {
				t.Fatalf("検証を通してしまいました: %+v", user)
			}
			if !errors.Is(err, repository.ErrInvalidToken) {
				t.Errorf("エラー: got %v, want repository.ErrInvalidToken にマッチ", err)
			}
			if user != nil {
				t.Errorf("失敗時に利用者を返しています: %+v", user)
			}
		})
	}
}

// withClaims は既存のトークンのクレームを差し替えたトークンを組み立てる。
//
// 署名は元のまま。エミュレータは署名を見ないため、
// **クレームに対する検証だけを切り出して確かめられる**。
func withClaims(t *testing.T, token string, overrides map[string]any) string {
	t.Helper()

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("トークンの形式が想定と違います: セグメント数 %d", len(parts))
	}

	raw, decodeErr := base64.RawURLEncoding.DecodeString(parts[1])
	if decodeErr != nil {
		t.Fatalf("ペイロードのデコードに失敗しました: %v", decodeErr)
	}

	var claims map[string]any
	if unmarshalErr := json.Unmarshal(raw, &claims); unmarshalErr != nil {
		t.Fatalf("ペイロードの解析に失敗しました: %v", unmarshalErr)
	}
	for k, v := range overrides {
		claims[k] = v
	}

	encoded, marshalErr := json.Marshal(claims)
	if marshalErr != nil {
		t.Fatalf("ペイロードの再構築に失敗しました: %v", marshalErr)
	}

	return replacePayload(token, encoded)
}

// replacePayload はトークンのペイロード部分を差し替える。
func replacePayload(token string, payload []byte) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return token + ".tampered"
	}
	return parts[0] + "." + base64.RawURLEncoding.EncodeToString(payload) + "." + parts[2]
}
