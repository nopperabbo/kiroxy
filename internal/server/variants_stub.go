//go:build !dashboard_variants

package server

import "net/http"

func (s *Server) registerVariants(_ *http.ServeMux) {}
