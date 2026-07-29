package contextkey

import (
	"context"
	"testing"

	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
)

func TestUID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		ctx     func(context.Context) context.Context
		wantUID string
		wantOK  bool
	}{
		{
			name: "認証済みなら uid が取れる",
			ctx: func(ctx context.Context) context.Context {
				return WithAuthUser(ctx, &entity.AuthUser{UID: "uid-1", Email: "a@example.com"})
			},
			wantUID: "uid-1",
			wantOK:  true,
		},
		{
			name:   "何も入っていなければ取れない",
			ctx:    func(ctx context.Context) context.Context { return ctx },
			wantOK: false,
		},
		{
			name: "nil を入れても取れない",
			ctx: func(ctx context.Context) context.Context {
				return WithAuthUser(ctx, nil)
			},
			wantOK: false,
		},
		{
			// 空の uid を Firestore のパスに使うと別の場所を指してしまう。
			// 取れなかったことを呼び出し側が判別できること。
			name: "uid が空なら取れない",
			ctx: func(ctx context.Context) context.Context {
				return WithAuthUser(ctx, &entity.AuthUser{UID: "", Email: "a@example.com"})
			},
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uid, ok := UID(tt.ctx(t.Context()))
			if ok != tt.wantOK {
				t.Fatalf("ok: got %v, want %v", ok, tt.wantOK)
			}
			if uid != tt.wantUID {
				t.Errorf("uid: got %q, want %q", uid, tt.wantUID)
			}
		})
	}
}

// TestAuthUserCannotBeForgedWithOtherKey は、外部から同じキーで値を差し込めないことを示す。
// キーの型が非公開なので、他パッケージが作った同じ整数値のキーとは一致しない。
func TestAuthUserCannotBeForgedWithOtherKey(t *testing.T) {
	t.Parallel()

	type foreignKey int

	//nolint:staticcheck // 他パッケージからの差し込みを再現するため、あえて独自キーで入れる
	ctx := context.WithValue(t.Context(), foreignKey(0), &entity.AuthUser{UID: "attacker"})

	if _, ok := AuthUser(ctx); ok {
		t.Error("別のキーで入れた値が利用者として読めてしまいました")
	}
}
