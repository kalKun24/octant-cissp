// Package config は環境変数の読み込みと検証を行う。
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/kalKun24/octant-cissp/backend/internal/domain/entity"
)

// 動作環境の列挙値。ここに無い値は起動時に弾く。
const (
	EnvLocal = "local"
	EnvDev   = "dev"
	EnvProd  = "prod"
)

// validEnvs は APP_ENV に許可する値。
var validEnvs = []string{EnvLocal, EnvDev, EnvProd}

// authEmulatorHostEnv は Firebase Admin SDK が Auth エミュレータの接続先として読む環境変数。
// アプリはこの値で処理を分岐させない。設定の妥当性検証にだけ使う。
const authEmulatorHostEnv = "FIREBASE_AUTH_EMULATOR_HOST"

// Config はサーバの起動に必要な設定。
type Config struct {
	// Port は待ち受けポート。Cloud Run が PORT で指定してくる。
	Port string
	// Env は動作環境（local / dev / prod）。
	Env string
	// Revision はデプロイの識別子。Cloud Run が K_REVISION で渡してくる。
	Revision string
	// LogLevel は出力するログの下限。
	LogLevel slog.Level
	// FirebaseProjectID は ID トークンの発行者・対象の検証に使う Firebase プロジェクト ID。
	FirebaseProjectID string
	// AllowedEmails は API の利用を許可するメールアドレスの集合。
	AllowedEmails entity.EmailAllowlist
}

// Load は環境変数から設定を読み、妥当性を検証する。
//
// **不正な値では起動させない。** 既定値で黙って動くと、設定漏れが
// 認証バイパス側へ倒れる（例: APP_ENV の打ち間違いが本番を local 扱いにする）。
func Load() (*Config, error) {
	level, err := parseLogLevel(env("LOG_LEVEL", "info"))
	if err != nil {
		return nil, err
	}

	appEnv, err := parseEnv(os.Getenv("APP_ENV"))
	if err != nil {
		return nil, err
	}

	projectID := os.Getenv("FIREBASE_PROJECT_ID")
	if projectID == "" {
		return nil, fmt.Errorf("FIREBASE_PROJECT_ID が未設定です（ID トークンの発行者と対象の検証に必要です）")
	}

	allowed, err := entity.NewEmailAllowlist(os.Getenv("ALLOWED_EMAILS"))
	if err != nil {
		return nil, fmt.Errorf("ALLOWED_EMAILS の値が不正です: %w", err)
	}

	if err := checkEmulatorUsage(appEnv); err != nil {
		return nil, err
	}

	return &Config{
		Port:              env("PORT", "8080"),
		Env:               appEnv,
		Revision:          env("K_REVISION", "unknown"),
		LogLevel:          level,
		FirebaseProjectID: projectID,
		AllowedEmails:     allowed,
	}, nil
}

// UsesAuthEmulator は Auth エミュレータに接続する設定かどうかを返す。
// 起動ログでの警告にだけ使う。**リクエスト処理の分岐に使わないこと。**
func UsesAuthEmulator() bool {
	return os.Getenv(authEmulatorHostEnv) != ""
}

// checkEmulatorUsage は local 以外で Auth エミュレータへ向いている設定を拒否する。
//
// エミュレータ接続時、Firebase Admin SDK は ID トークンの**署名検証を省略する**。
// 本番や dev でこの環境変数が紛れ込むと、誰でも自作トークンで認証を通せる。
// 「エミュレータならスキップする」分岐ではなく、危険な組み合わせでは
// **起動そのものを失敗させる**（fail-closed）。
func checkEmulatorUsage(appEnv string) error {
	if appEnv == EnvLocal || !UsesAuthEmulator() {
		return nil
	}
	return fmt.Errorf(
		"APP_ENV=%s では %s を設定できません（エミュレータ接続時は ID トークンの署名検証が省略されるため）",
		appEnv, authEmulatorHostEnv)
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseEnv は APP_ENV を検証する。
//
// **既定値を持たない。未設定・空文字は起動失敗にする。**
// 既定を local にすると、Cloud Run で環境変数を入れ忘れたときに
// local 扱いで起動し、checkEmulatorUsage のガードが無効化される
// （dev / prod でのみ効くガードなので、local と誤認された時点で素通りする）。
// 設定漏れは**必ず起動失敗として現れる**ようにする。
func parseEnv(s string) (string, error) {
	if s == "" {
		return "", fmt.Errorf("APP_ENV が未設定です（%s のいずれかを指定してください）",
			strings.Join(validEnvs, " / "))
	}
	for _, valid := range validEnvs {
		if s == valid {
			return s, nil
		}
	}
	return "", fmt.Errorf("APP_ENV の値が不正です: %q（%s のいずれかを指定してください）",
		s, strings.Join(validEnvs, " / "))
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("LOG_LEVEL の値が不正です: %q（debug / info / warn / error のいずれかを指定してください）", s)
	}
}
