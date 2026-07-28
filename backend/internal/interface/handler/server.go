package handler

import (
	"github.com/kalKun24/octant-cissp/backend/internal/interface/openapi"
)

// Server は生成された openapi.ServerInterface の実装。
//
// エンドポイントごとのハンドラを埋め込んで束ねる。openapi.yaml に
// オペレーションを足すと、対応するメソッドを持つハンドラを
// ここに埋め込むまでコンパイルが通らなくなる（実装漏れを検知できる）。
type Server struct {
	*Health
}

// openapi.yaml のすべてのオペレーションを実装していることを確かめる。
var _ openapi.ServerInterface = (*Server)(nil)

// NewServer は各ハンドラを束ねて Server を作る。
func NewServer(health *Health) *Server {
	return &Server{Health: health}
}
