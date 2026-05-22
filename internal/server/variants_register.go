//go:build dashboard_variants

package server

import (
	"net/http"

	"github.com/nopperabbo/kiroxy/internal/server/variants/brutal"
	"github.com/nopperabbo/kiroxy/internal/server/variants/linearpremium"
	"github.com/nopperabbo/kiroxy/internal/server/variants/muji"
	"github.com/nopperabbo/kiroxy/internal/server/variants/neon"
	"github.com/nopperabbo/kiroxy/internal/server/variants/nord"
	"github.com/nopperabbo/kiroxy/internal/server/variants/paper"
)

func (s *Server) registerVariants(mux *http.ServeMux) {
	brutal.Register(mux)
	paper.Register(mux)
	nord.Register(mux)
	neon.Register(mux)
	muji.Register(mux, s.mujiSnap)
	linearpremium.Register(mux)
}
