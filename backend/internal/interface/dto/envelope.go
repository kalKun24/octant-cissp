// Package dto は API 境界の型と変換を扱う。
package dto

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/kalKun24/octant-cissp/backend/internal/interface/openapi"
)

// Envelope は全レスポンス共通の封筒。Data と Error はどちらか一方が必ず null になる。
// 形は api/openapi.yaml の Envelope が正。手書きで二重定義しないため別名にする。
type Envelope = openapi.Envelope

// Error は機械可読な Code と、そのまま画面に出せる日本語の Message を持つ。
// 形は api/openapi.yaml の Error が正。
type Error = openapi.Error

// WriteJSON は data を封筒に入れて書き出す。
func WriteJSON(w http.ResponseWriter, status int, data any) {
	write(w, status, Envelope{Data: data})
}

// WriteError は code と message を封筒に入れて書き出す。
// message はそのまま画面に表示されるため、内部情報を含めないこと。
func WriteError(w http.ResponseWriter, status int, code, message string) {
	write(w, status, Envelope{Error: &Error{Code: code, Message: message}})
}

func write(w http.ResponseWriter, status int, body Envelope) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)

	if err := json.NewEncoder(w).Encode(body); err != nil {
		// ヘッダ送出後のため応答は変えられない。記録だけ残す。
		slog.Error("レスポンスの書き出しに失敗しました", "error", err)
	}
}
