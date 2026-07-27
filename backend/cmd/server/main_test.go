package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func TestCloudSeverity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		level slog.Level
		want  string
	}{
		{name: "DEBUG", level: slog.LevelDebug, want: "DEBUG"},
		{name: "INFO", level: slog.LevelInfo, want: "INFO"},
		{name: "WARN は Cloud Logging では WARNING", level: slog.LevelWarn, want: "WARNING"},
		{name: "ERROR", level: slog.LevelError, want: "ERROR"},
		{name: "既定より低いレベルは DEBUG に丸める", level: slog.LevelDebug - 4, want: "DEBUG"},
		{name: "既定より高いレベルは ERROR に丸める", level: slog.LevelError + 4, want: "ERROR"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := cloudSeverity(tt.level); got != tt.want {
				t.Errorf("cloudSeverity(%v): got %q, want %q", tt.level, got, tt.want)
			}
		})
	}
}

// TestToCloudLoggingKeys は、実際に出力される JSON が Cloud Logging の
// 期待するキー（severity / message）になっていることを確かめる。
// level / msg のままだと全ログが DEFAULT 重大度に落ちるため、退行を検知する。
func TestToCloudLoggingKeys(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{
		Level:       slog.LevelDebug,
		ReplaceAttr: toCloudLogging,
	}))

	logger.Error("パニックから復帰しました", slog.String("path", "/health"))

	var got map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("ログの解析に失敗しました: %v", err)
	}

	if got["severity"] != "ERROR" {
		t.Errorf("severity: got %v, want %q", got["severity"], "ERROR")
	}
	if got["message"] != "パニックから復帰しました" {
		t.Errorf("message: got %v", got["message"])
	}
	for _, key := range []string{"level", "msg"} {
		if _, ok := got[key]; ok {
			t.Errorf("%q が残っています。Cloud Logging は severity / message を読みます", key)
		}
	}
	// 独自フィールドは変換対象外であること。
	if got["path"] != "/health" {
		t.Errorf("path: got %v, want %q", got["path"], "/health")
	}
}
