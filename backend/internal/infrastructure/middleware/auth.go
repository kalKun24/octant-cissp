package middleware

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/kalKun24/octant-cissp/backend/internal/contextkey"
	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
	"github.com/kalKun24/octant-cissp/backend/internal/interface/dto"
	"github.com/kalKun24/octant-cissp/backend/internal/usecase/auth"
)

// bearerPrefix は Authorization ヘッダの認証方式。RFC 9110 上、方式名は大小を区別しない。
const bearerPrefix = "bearer "

// Authenticator は ID トークンから利用者を確定させる。
// 実体は usecase/auth.Authenticator。テストで差し替えられるようここで受け側の型を宣言する。
type Authenticator interface {
	Authenticate(ctx context.Context, idToken string) (*entity.AuthUser, error)
}

// Auth は ID トークンを検証し、検証済みの利用者を context に入れる。
//
// **このミドルウェアは生成コード（openapi.HandlerWithOptions の Middlewares）
// 経由で全ハンドラに一律で適用する。** ルータ直下の Use ではなく生成コードに
// 渡す理由は2つある。
//
//  1. ルート照合の**後**に走るため chi.RouteContext の RoutePattern が確定していて、
//     免除判定をパス文字列ではなくルートパターンで行える
//  2. openapi.yaml に足したエンドポイントは自動的にこのミドルウェアを通る。
//     ハンドラごとに付け外しする方式だと、付け忘れたエンドポイントが無認証になる
//
// **認証をスキップする環境変数や動作環境による分岐は持たない。**
// ローカルでも Auth エミュレータが発行した本物と同形のトークンを、
// 本番と同じ検証経路に通す（CLAUDE.md「守るべき制約」）。
func Auth(logger *slog.Logger, authenticator Authenticator, public PublicRoutes) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if public.IsExempt(r) {
				next.ServeHTTP(w, r)
				return
			}

			token, ok := bearerToken(r)
			if !ok {
				logDenied(logger, r, "認証情報がありません")
				writeUnauthorized(w)
				return
			}

			user, err := authenticator.Authenticate(r.Context(), token)
			if err != nil {
				writeAuthError(w, logger, r, err)
				return
			}

			// ここから下流は uid を検証済みの値として扱ってよい。
			setLogUID(r.Context(), user.UID)
			next.ServeHTTP(w, r.WithContext(contextkey.WithAuthUser(r.Context(), user)))
		})
	}
}

// writeAuthError は認証・認可の失敗を応答へ写す。
//
// 401 と 403 の切り分けだけを行い、**失敗の理由をクライアントに伝えない**。
// 「署名が不正」「期限切れ」「メール未確認」を返し分けると、
// 攻撃者に検証ロジックの手掛かりを与える。
func writeAuthError(w http.ResponseWriter, logger *slog.Logger, r *http.Request, err error) {
	switch {
	case errors.Is(err, auth.ErrNotAllowed):
		logDenied(logger, r, "許可されていない利用者です")
		dto.WriteError(w, http.StatusForbidden, "FORBIDDEN",
			"このアカウントには利用が許可されていません。管理者に連絡してください。")
	case errors.Is(err, auth.ErrUnauthenticated):
		logDenied(logger, r, "認証に失敗しました")
		writeUnauthorized(w)
	default:
		// ユースケースが分類しなかったエラー。実装の不備なので 500 にして気づけるようにする。
		// 拒否する点は変わらない（アクセスは通さない）。
		logger.LogAttrs(r.Context(), slog.LevelError, "認証処理で想定外のエラーが発生しました",
			slog.String("trace_id", chimw.GetReqID(r.Context())),
			slog.String("error", err.Error()),
		)
		dto.WriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"サーバ内部でエラーが発生しました。時間をおいて操作をやり直してください。")
	}
}

func writeUnauthorized(w http.ResponseWriter) {
	// RFC 9110 §11.6.1 は 401 に WWW-Authenticate を付けることを MUST としている。
	w.Header().Set("WWW-Authenticate", `Bearer realm="octant"`)
	dto.WriteError(w, http.StatusUnauthorized, "UNAUTHENTICATED",
		"サインインが必要です。もう一度サインインしてください。")
}

// logDenied は拒否したことだけを記録する。
// **トークンとメールアドレスは出さない**（CLAUDE.md「品質」）。
func logDenied(logger *slog.Logger, r *http.Request, reason string) {
	logger.LogAttrs(r.Context(), slog.LevelWarn, "リクエストを拒否しました",
		slog.String("trace_id", chimw.GetReqID(r.Context())),
		slog.String("method", r.Method),
		slog.String("route", routePattern(r)),
		slog.String("reason", reason),
	)
}

// routePattern は chi が確定させたルートパターンを返す。
// 取れない場合は空文字。利用者が与えた文字列をログに載せないため、
// r.URL.Path ではなくパターンを使う。
func routePattern(r *http.Request) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return ""
	}
	return rctx.RoutePattern()
}

// bearerToken は Authorization ヘッダから ID トークンを取り出す。
//
// ヘッダが複数ある場合は**採用しない**。どれが正かを決められないうえ、
// 中間装置ごとに解釈が割れると検証したものと使うものがずれる危険がある。
func bearerToken(r *http.Request) (string, bool) {
	values := r.Header.Values("Authorization")
	if len(values) != 1 {
		return "", false
	}

	header := values[0]
	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}

	token := strings.TrimSpace(header[len(bearerPrefix):])
	if token == "" {
		return "", false
	}

	return token, true
}
