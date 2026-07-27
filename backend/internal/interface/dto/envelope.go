// Package dto は API 境界の型と変換を扱う。
package dto

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// Envelope は全レスポンス共通の封筒。Data と Error はどちらか一方が必ず null になる。
type Envelope struct {
	Data  any    `json:"data"`
	Error *Error `json:"error"`
}

// Error は機械可読な Code と、そのまま画面に出せる日本語の Message を持つ。
type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

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
