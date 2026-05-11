// This file is derived loosely from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package server

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type authMiddleware struct {
	expectedHash [32]byte
	requireKey   bool
}

func newAuthMiddleware(apiKey string) *authMiddleware {
	m := &authMiddleware{}
	if apiKey == "" {
		return m
	}
	m.expectedHash = sha256.Sum256([]byte(apiKey))
	m.requireKey = true
	return m
}

func (m *authMiddleware) wrap(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !m.requireKey {
			next.ServeHTTP(w, r)
			return
		}
		if r.URL.Path == "/healthz" {
			next.ServeHTTP(w, r)
			return
		}
		// Dashboard bypasses auth for loopback requests (personal-use UX).
		// Non-loopback dashboard access still requires the API key.
		if strings.HasPrefix(r.URL.Path, "/dashboard") && isLoopback(r) {
			next.ServeHTTP(w, r)
			return
		}
		provided := extractAPIKey(r)
		if provided == "" {
			writeAuthProblem(w, http.StatusUnauthorized, "missing_api_key", "set X-Api-Key header or Authorization: Bearer <token>")
			return
		}
		providedHash := sha256.Sum256([]byte(provided))
		if subtle.ConstantTimeCompare(providedHash[:], m.expectedHash[:]) != 1 {
			writeAuthProblem(w, http.StatusUnauthorized, "invalid_api_key", "the provided API key does not match")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func extractAPIKey(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Api-Key")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("Authorization")); v != "" {
		if strings.HasPrefix(strings.ToLower(v), "bearer ") {
			return strings.TrimSpace(v[len("Bearer "):])
		}
	}
	return ""
}

func writeAuthProblem(w http.ResponseWriter, status int, code, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":   "https://kiroxy.local/errors/" + code,
		"title":  "Unauthorized",
		"status": status,
		"code":   code,
		"detail": detail,
	})
}

// isLoopback reports whether the request's RemoteAddr is a loopback address.
// Used by dashboard auth in M10 to bypass the API key check for localhost.
func isLoopback(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
