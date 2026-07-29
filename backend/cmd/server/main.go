// Package main は API サーバのエントリポイント。依存の配線はこのファイル1箇所で行う。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/kalKun24/octant-cissp/backend/internal/config"
	"github.com/kalKun24/octant-cissp/backend/internal/infrastructure/firebaseauth"
	"github.com/kalKun24/octant-cissp/backend/internal/infrastructure/middleware"
	"github.com/kalKun24/octant-cissp/backend/internal/interface/dto"
	"github.com/kalKun24/octant-cissp/backend/internal/interface/handler"
	"github.com/kalKun24/octant-cissp/backend/internal/interface/openapi"
	"github.com/kalKun24/octant-cissp/backend/internal/usecase/auth"
)

const (
	// readHeaderTimeout はヘッダ読み込みの上限。Slowloris 対策として必ず設定する。
	readHeaderTimeout = 10 * time.Second
	// readTimeout はリクエスト全体の読み込みの上限。
	readTimeout = 30 * time.Second
	// writeTimeout はレスポンス書き出しの上限。AI 生成は同期 REST なので長めに取る。
	writeTimeout = 120 * time.Second
	// idleTimeout は keep-alive 接続を保持する上限。
	idleTimeout = 120 * time.Second
	// shutdownTimeout は停止時に処理中のリクエストを待つ上限。
	// Cloud Run は SIGTERM の 10 秒後に SIGKILL を送るため、それより短くする。
	shutdownTimeout = 8 * time.Second

	// apiBasePath は全エンドポイントの基底パス。openapi.yaml の servers と揃える。
	//
	// Firebase Hosting の rewrites は /api/** をパスを書き換えずに Cloud Run へ
	// 転送するため、サーバ側が /api を含むパスで待ち受ける必要がある。
	// ローカルの Vite proxy も同じくプレフィックスを剥がさない。
	apiBasePath = "/api"

	// verifierInitTimeout は Firebase Admin SDK の初期化に許す上限。
	// 資格情報の取得でメタデータサーバへ問い合わせるため、無期限に待たせない。
	verifierInitTimeout = 15 * time.Second
)

// publicRouteEntries は**認証を免除するルート**の全集合。
//
// 形式は "METHOD /route/pattern"。api/openapi.yaml で `security: []` を
// 書いたオペレーションと1対1で対応させること。
//
// **ここに書かれていないルートはすべて認証必須になる**（fail-closed）。
// 新しいエンドポイントを足しても、この一覧を触らない限り無認証にはならない。
// 逆にここへ1行足すことは「認証を外す」という明示的な変更であり、
// cmd/server の TestNonPublicRoutesRequireAuth がその影響範囲を検証する。
var publicRouteEntries = []string{
	// 死活監視。Cloud Run と GCP ロードバランサが認証情報を持たずに叩く。
	"GET /api/health",
	"HEAD /api/health",
}

func main() {
	if err := run(); err != nil {
		// ロガー初期化前に落ちる可能性があるため、ここでは標準エラーへ出す。
		fmt.Fprintf(os.Stderr, "サーバの起動に失敗しました: %v\n", err)
		os.Exit(1)
	}
}

// run は設定の読み込みからサーバの停止までを担う。
// os.Exit を使わず error を返すことで defer が確実に走るようにしている。
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("設定の読み込みに失敗しました: %w", err)
	}

	logger := newLogger(cfg)
	slog.SetDefault(logger)

	// SIGTERM / SIGINT を受けたら ctx がキャンセルされる。Cloud Run は停止時に SIGTERM を送る。
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	authenticator, err := newAuthenticator(ctx, cfg)
	if err != nil {
		return fmt.Errorf("認証の初期化に失敗しました: %w", err)
	}

	router, err := newRouter(logger, cfg, authenticator, publicRouteEntries)
	if err != nil {
		return fmt.Errorf("ルータの組み立てに失敗しました: %w", err)
	}

	logStartupPolicy(ctx, logger, cfg)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	// ListenAndServe はブロックするため別ゴルーチンで動かし、結果をチャネルで受ける。
	serveErr := make(chan error, 1)
	go func() {
		// env / revision はベースロガーが常に付けるため、ここでは指定しない。
		logger.LogAttrs(ctx, slog.LevelInfo, "サーバを起動しました",
			slog.String("port", cfg.Port),
		)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serveErr <- fmt.Errorf("待ち受けに失敗しました: %w", err)
			return
		}
		serveErr <- nil
	}()

	select {
	case err := <-serveErr:
		return err
	case <-ctx.Done():
		// 以降のシグナルは既定動作（即時終了）に戻し、停止処理が固まっても抜けられるようにする。
		stop()
		logger.Info("停止シグナルを受け取りました。処理中のリクエストの完了を待ちます")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("グレースフルシャットダウンに失敗しました: %w", err)
	}

	logger.Info("サーバを停止しました")
	return nil
}

// newLogger は Cloud Logging が解釈できる構造化 JSON ロガーを作る。
//
// Cloud Run が読むのは severity / message で、slog の既定は level / msg。
// そのままでは全ログが DEFAULT 重大度に落ち、panic のエラーログも
// 障害として検出されない。ReplaceAttr でキー名と値を変換する。
func newLogger(cfg *config.Config) *slog.Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:       cfg.LogLevel,
		ReplaceAttr: toCloudLogging,
	})

	return slog.New(handler).With(
		slog.String("env", cfg.Env),
		slog.String("revision", cfg.Revision),
	)
}

// toCloudLogging は slog の既定フィールド名を Cloud Logging の語彙へ写す。
func toCloudLogging(groups []string, a slog.Attr) slog.Attr {
	// グループ内の同名フィールドまで巻き込まないよう、トップレベルだけを対象にする。
	if len(groups) > 0 {
		return a
	}

	switch a.Key {
	case slog.LevelKey:
		a.Key = "severity"
		if level, ok := a.Value.Any().(slog.Level); ok {
			a.Value = slog.StringValue(cloudSeverity(level))
		}
	case slog.MessageKey:
		a.Key = "message"
	}

	return a
}

// cloudSeverity は slog のレベルを Cloud Logging の LogSeverity に写す。
// WARN だけ名称が異なる（Cloud Logging では WARNING）。
func cloudSeverity(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "ERROR"
	case level >= slog.LevelWarn:
		return "WARNING"
	case level >= slog.LevelInfo:
		return "INFO"
	default:
		return "DEBUG"
	}
}

// newAuthenticator は ID トークンの検証器と許可メールの照合を束ねる。
func newAuthenticator(ctx context.Context, cfg *config.Config) (*auth.Authenticator, error) {
	initCtx, cancel := context.WithTimeout(ctx, verifierInitTimeout)
	defer cancel()

	verifier, err := firebaseauth.NewVerifier(initCtx, cfg.FirebaseProjectID)
	if err != nil {
		return nil, fmt.Errorf("ID トークン検証器の生成に失敗しました: %w", err)
	}

	return auth.NewAuthenticator(verifier, cfg.AllowedEmails), nil
}

// logStartupPolicy は起動時に有効な認証まわりの設定を記録する。
// **許可メールそのものは出さない**（件数だけを出す）。
func logStartupPolicy(ctx context.Context, logger *slog.Logger, cfg *config.Config) {
	logger.LogAttrs(ctx, slog.LevelInfo, "認証設定を読み込みました",
		slog.String("firebase_project_id", cfg.FirebaseProjectID),
		slog.Int("allowed_emails", cfg.AllowedEmails.Size()),
		slog.Int("public_routes", len(publicRouteEntries)),
	)

	if config.UsesAuthEmulator() {
		// 署名検証が省略される構成であることを運用者に見せる。
		// config.Load が local 以外でこの構成を弾いているため、ここに来るのは local だけ。
		logger.LogAttrs(ctx, slog.LevelWarn,
			"Firebase Auth エミュレータに接続します。ID トークンの署名は検証されません")
	}
}

// newRouter はルータを組み立てる。ミドルウェアは外側から順に適用される。
//
// SecurityHeaders → RequestID → RequestLogger → Recovery の順にしているのは、
// 404 / 405 / パニック時の 500 にもセキュリティヘッダとアクセスログを残したいため。
// RequestLogger が trace_id を読めるように RequestID を先に置き、
// panic 時も 500 として1行のアクセスログが残るように Recovery を内側に置く。
//
// **認証ミドルウェアだけはここに置かない。** 生成コードの Middlewares へ渡し、
// ルート照合の後に走らせる（免除判定に chi の RoutePattern が要るため。
// 詳細は middleware.PublicRoutes のコメント）。
//
// publicEntries を引数で受けるのは、テストが**免除リストを空にしたルータ**を
// 組み立てられるようにするため。免除が効いていない状態で /api/health が
// 401 になることを固定でき、免除の分岐が実際に働いていることを検証できる。
// 本番の値は publicRouteEntries 1箇所だけ。
func newRouter(
	logger *slog.Logger,
	cfg *config.Config,
	authenticator middleware.Authenticator,
	publicEntries []string,
) (http.Handler, error) {
	public, err := middleware.NewPublicRoutes(publicEntries...)
	if err != nil {
		return nil, fmt.Errorf("認証免除ルートの定義が不正です: %w", err)
	}

	r := chi.NewRouter()

	r.Use(middleware.SecurityHeaders)
	r.Use(middleware.RequestID)
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Recovery(logger))

	// chi の既定はプレーンテキストを返すため、{data, error} の封筒に差し替える。
	// 末尾スラッシュ（/api/health/）は別パスとして 404 にする。
	// StripSlashes / RedirectSlashes は使わない（1リソース1URLを保つため）。
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		dto.WriteError(w, http.StatusNotFound, "NOT_FOUND",
			"指定されたエンドポイントは存在しません。URL を確認してください。")
	})
	r.MethodNotAllowed(methodNotAllowed(r))

	// ルートの登録は openapi.yaml から生成した HandlerWithOptions に任せる。
	// 手で r.Get(...) を書かないことで、仕様に無いエンドポイントが実装に生えない。
	// /api/health は死活監視用のため認証を要しない。HEAD も openapi.yaml で
	// 明示しているパスだけに生成される（chi は GET から HEAD を自動生成しない）。
	srv := handler.NewServer(handler.NewHealth(cfg.Revision))
	openapi.HandlerWithOptions(srv, openapi.ChiServerOptions{
		BaseURL:    apiBasePath,
		BaseRouter: r,
		// 生成コードは登録した全ハンドラにこのミドルウェアを通す。
		// openapi.yaml にオペレーションを足すと自動的に認証の対象になる。
		Middlewares:      []openapi.MiddlewareFunc{middleware.Auth(logger, authenticator, public)},
		ErrorHandlerFunc: parameterErrorHandler(logger),
	})

	return r, nil
}

// parameterErrorHandler は生成コードがパラメータの解釈に失敗したときの応答を作る。
// 既定は http.Error でプレーンテキストを返すため、封筒に差し替える。
func parameterErrorHandler(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		// 詳細は利用者に返さず記録だけ残す（内部情報を漏らさないため）。
		logger.LogAttrs(r.Context(), slog.LevelWarn, "リクエストパラメータの解釈に失敗しました",
			slog.String("path", r.URL.Path),
			slog.String("error", err.Error()),
		)
		dto.WriteError(w, http.StatusBadRequest, "INVALID_PARAMETER",
			"リクエストパラメータの形式が正しくありません。値を確認してください。")
	}
}

// probeMethods は Allow ヘッダを組み立てるときにルータへ問い合わせるメソッド。
// chi が扱う CONNECT / TRACE はこの API で使わないため含めない。
var probeMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodPost,
	http.MethodPut,
	http.MethodPatch,
	http.MethodDelete,
	http.MethodOptions,
}

// methodNotAllowed は 405 応答のハンドラを作る。
//
// RFC 9110 §15.5.6 は 405 に Allow ヘッダを付けることを MUST としている。
// chi は既定のハンドラでこれを組み立てるが、許可メソッドは非公開フィールド
// （Context.methodsAllowed）にあり、差し替えたハンドラからは読めない。
// そのため公開 API の Mux.Match でルータに問い合わせ直して組み立てる。
func methodNotAllowed(router *chi.Mux) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if allow := allowedMethods(router, routingPath(r)); allow != "" {
			w.Header().Set("Allow", allow)
		}
		dto.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED",
			"このエンドポイントでは指定の HTTP メソッドを使用できません。")
	}
}

// allowedMethods は path に登録されているメソッドをカンマ区切りで返す。
// 1つも登録が無ければ空文字を返す（この場合は Allow を付けない）。
func allowedMethods(router *chi.Mux, path string) string {
	allowed := make([]string, 0, len(probeMethods))
	for _, method := range probeMethods {
		// Match は渡した Context を書き換えるため、毎回新しいものを渡す。
		if router.Match(chi.NewRouteContext(), method, path) {
			allowed = append(allowed, method)
		}
	}
	return strings.Join(allowed, ", ")
}

// routingPath は chi がルーティングに使うパスを返す。
// chi 本体（Mux.ServeHTTP）と同じ優先順位で RawPath を先に見る。
func routingPath(r *http.Request) string {
	if r.URL.RawPath != "" {
		return r.URL.RawPath
	}
	return r.URL.Path
}
