package config

import (
	"log/slog"
	"testing"
)

// baseEnv は Load が成功する最小の環境変数。各テストはここから1つだけ変える。
func baseEnv(t *testing.T) {
	t.Helper()

	t.Setenv("APP_ENV", EnvLocal)
	t.Setenv("FIREBASE_PROJECT_ID", "demo-octant")
	t.Setenv("ALLOWED_EMAILS", "dev@example.com")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("PORT", "")
	t.Setenv("K_REVISION", "")
	t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "")
}

func TestLoadAppEnv(t *testing.T) {
	tests := []struct {
		name    string
		appEnv  string
		wantErr bool
		want    string
	}{
		{name: "local", appEnv: EnvLocal, want: EnvLocal},
		{name: "dev", appEnv: EnvDev, want: EnvDev},
		{name: "prod", appEnv: EnvProd, want: EnvProd},
		// 既定値を持たせない。既定を local にすると、Cloud Run で
		// 環境変数を入れ忘れたときにエミュレータガードが無効な状態で起動する。
		{name: "未設定は起動失敗", appEnv: "", wantErr: true},
		{name: "列挙値以外は起動失敗", appEnv: "production", wantErr: true},
		{name: "大文字は受け付けない", appEnv: "PROD", wantErr: true},
		{name: "打ち間違いは起動失敗", appEnv: "prd", wantErr: true},
		{name: "空白入りは起動失敗", appEnv: " prod", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseEnv(t)
			t.Setenv("APP_ENV", tt.appEnv)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatalf("起動に失敗するべきです: got Env=%q", cfg.Env)
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if cfg.Env != tt.want {
				t.Errorf("Env: got %q, want %q", cfg.Env, tt.want)
			}
		})
	}
}

func TestLoadRequiresFirebaseProjectID(t *testing.T) {
	baseEnv(t)
	t.Setenv("FIREBASE_PROJECT_ID", "")

	if _, err := Load(); err == nil {
		t.Fatal("FIREBASE_PROJECT_ID が無い場合は起動に失敗するべきです")
	}
}

func TestLoadAllowedEmails(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		wantErr bool
		want    int
	}{
		{name: "1件", value: "a@example.com", want: 1},
		{name: "複数件", value: "a@example.com,b@example.com", want: 2},
		{name: "未設定は起動失敗", value: "", wantErr: true},
		{name: "形式不正は起動失敗", value: "notanemail", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseEnv(t)
			t.Setenv("ALLOWED_EMAILS", tt.value)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("起動に失敗するべきです")
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if cfg.AllowedEmails.Size() != tt.want {
				t.Errorf("許可メール件数: got %d, want %d", cfg.AllowedEmails.Size(), tt.want)
			}
		})
	}
}

// TestLoadRejectsAuthEmulatorOutsideLocal は local 以外でエミュレータ接続を拒むことを確かめる。
//
// エミュレータ接続時は ID トークンの署名検証が省略される。
// dev / prod にこの環境変数が紛れ込むと、誰でも自作トークンで認証を通せてしまう。
func TestLoadRejectsAuthEmulatorOutsideLocal(t *testing.T) {
	tests := []struct {
		name    string
		appEnv  string
		wantErr bool
	}{
		{name: "local では許す", appEnv: EnvLocal, wantErr: false},
		{name: "dev では起動失敗", appEnv: EnvDev, wantErr: true},
		{name: "prod では起動失敗", appEnv: EnvProd, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseEnv(t)
			t.Setenv("APP_ENV", tt.appEnv)
			t.Setenv("FIREBASE_AUTH_EMULATOR_HOST", "127.0.0.1:9099")

			_, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("エラー: got %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoadLogLevel(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    slog.Level
		wantErr bool
	}{
		{name: "debug", value: "debug", want: slog.LevelDebug},
		{name: "info", value: "info", want: slog.LevelInfo},
		{name: "大文字も受け付ける", value: "WARN", want: slog.LevelWarn},
		{name: "未設定は info", value: "", want: slog.LevelInfo},
		{name: "不正な値は起動失敗", value: "verbose", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseEnv(t)
			t.Setenv("LOG_LEVEL", tt.value)

			cfg, err := Load()
			if tt.wantErr {
				if err == nil {
					t.Fatal("起動に失敗するべきです")
				}
				return
			}
			if err != nil {
				t.Fatalf("予期しないエラー: %v", err)
			}
			if cfg.LogLevel != tt.want {
				t.Errorf("LogLevel: got %v, want %v", cfg.LogLevel, tt.want)
			}
		})
	}
}

func TestLoadDefaults(t *testing.T) {
	baseEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port: got %q, want %q", cfg.Port, "8080")
	}
	if cfg.Revision != "unknown" {
		t.Errorf("Revision: got %q, want %q", cfg.Revision, "unknown")
	}
}
