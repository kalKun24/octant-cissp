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
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/kalKun24/octant-cissp/backend/internal/config"
	"github.com/kalKun24/octant-cissp/backend/internal/infrastructure/middleware"
	"github.com/kalKun24/octant-cissp/backend/internal/interface/dto"
	"github.com/kalKun24/octant-cissp/backend/internal/interface/handler"
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
)

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

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           newRouter(logger, cfg),
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

// newRouter はルータを組み立てる。ミドルウェアは外側から順に適用される。
//
// RequestID → RequestLogger → Recovery の順にしているのは、
// RequestLogger が trace_id を読めるように RequestID を先に置き、
// panic 時も 500 として1行のアクセスログが残るように Recovery を内側に置くため。
func newRouter(logger *slog.Logger, cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	r.Use(chimw.RequestID)
	r.Use(middleware.RequestLogger(logger))
	r.Use(middleware.Recovery(logger))

	// chi の既定はプレーンテキストを返すため、{data, error} の封筒に差し替える。
	// 末尾スラッシュ（/health/）は別パスとして 404 にする。
	// StripSlashes / RedirectSlashes は使わない（1リソース1URLを保つため）。
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		dto.WriteError(w, http.StatusNotFound, "NOT_FOUND",
			"指定されたエンドポイントは存在しません。URL を確認してください。")
	})
	r.MethodNotAllowed(methodNotAllowed(r))

	// /health は死活監視用のため認証を要しない。
	// HEAD は curl -I や外部監視ツールが使うため、このパスだけ登録する
	// （業務 API は GET のみ。openapi.yaml に無いメソッドを実装に生やさない）。
	health := handler.NewHealth(cfg.Revision)
	r.Get("/health", health.Get)
	r.Head("/health", health.Head)

	return r
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
