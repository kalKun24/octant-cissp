// Package handler は HTTP ハンドラを提供する。
// 型とルートは api/openapi.yaml から生成した openapi パッケージが正であり、
// ここではその ServerInterface を実装する。
package handler

import (
	"net/http"

	"github.com/kalKun24/octant-cissp/backend/internal/interface/dto"
	"github.com/kalKun24/octant-cissp/backend/internal/interface/openapi"
)

// Health は死活監視のハンドラ。認証を要しない唯一のエンドポイント。
type Health struct {
	revision string
}

// NewHealth は Health を作る。revision が空なら "unknown" を使う。
func NewHealth(revision string) *Health {
	if revision == "" {
		revision = "unknown"
	}
	return &Health{revision: revision}
}

// GetHealth は GET /api/health を処理する。
func (h *Health) GetHealth(w http.ResponseWriter, _ *http.Request) {
	dto.WriteJSON(w, http.StatusOK, openapi.Health{
		Status:   openapi.HealthStatusOk,
		Revision: h.revision,
	})
}

// HeadHealth は HEAD /api/health を処理する。GET と同じステータスをボディなしで返す。
// 判定に revision は要らないため、レシーバの状態は参照しない。
func (*Health) HeadHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}
