// Package handler は HTTP ハンドラを提供する。
package handler

import (
	"net/http"

	"github.com/kalKun24/octant-cissp/backend/internal/interface/dto"
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

// HealthResponse は GET /health の応答。
type HealthResponse struct {
	Status   string `json:"status"`
	Revision string `json:"revision"`
}

// Get は GET /health を処理する。
func (h *Health) Get(w http.ResponseWriter, _ *http.Request) {
	dto.WriteJSON(w, http.StatusOK, HealthResponse{Status: "ok", Revision: h.revision})
}
