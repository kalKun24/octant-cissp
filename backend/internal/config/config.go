// Package config は環境変数の読み込みと検証を行う。
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
)

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
}

// Load は環境変数から設定を読み、妥当性を検証する。
func Load() (*Config, error) {
	level, err := parseLogLevel(env("LOG_LEVEL", "info"))
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:     env("PORT", "8080"),
		Env:      env("APP_ENV", "local"),
		Revision: env("K_REVISION", "unknown"),
		LogLevel: level,
	}, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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
