package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// InboundKeyView is the dashboard-safe projection of a vault inbound_key
// row. Plaintext is NEVER in this struct; the CreateInboundKey response
// uses its own richer CreatedInboundKey shape.
type InboundKeyView struct {
	ID         string `json:"id"`
	Label      string `json:"label,omitempty"`
	Tail       string `json:"tail"`
	CreatedAt  string `json:"created_at,omitempty"`
	LastUsedAt string `json:"last_used_at,omitempty"`
	Revoked    bool   `json:"revoked"`
}

// CreatedInboundKey is the one-shot response when a new key is minted.
// Plaintext appears exactly once — the UI must display and discard.
type CreatedInboundKey struct {
	InboundKeyView
	Plaintext string `json:"plaintext"`
}

// InboundKeyProvider is the narrow surface the server consumes for inbound
// key CRUD. main.go's dashboardProvider implements it against the vault.
//
// Kept separate from DashboardStateProvider + DashboardControlProvider so
// this subsystem is independently optional: when the vault is owned by a
// kiro-cli SQLite process (no kiroxy vault), the provider is simply nil
// and the handlers 404.
type InboundKeyProvider interface {
	List(ctx context.Context) ([]InboundKeyView, error)
	Create(ctx context.Context, label string) (*CreatedInboundKey, error)
	Revoke(ctx context.Context, id string) error
}

// registerInboundKeyHandlers mounts the three endpoints when the provider
// is configured. No-op when nil.
func (s *Server) registerInboundKeyHandlers(mux *http.ServeMux) {
	if s.opts.InboundKeyProvider == nil {
		return
	}
	mux.HandleFunc("GET /dashboard/api/inbound-keys", s.handleInboundKeysList)
	mux.HandleFunc("POST /dashboard/api/inbound-keys", s.handleInboundKeysCreate)
	mux.HandleFunc("DELETE /dashboard/api/inbound-keys/{id}", s.handleInboundKeysRevoke)
}

func (s *Server) handleInboundKeysList(w http.ResponseWriter, r *http.Request) {
	keys, err := s.opts.InboundKeyProvider.List(r.Context())
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "list_failed", err.Error())
		return
	}
	if keys == nil {
		keys = []InboundKeyView{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": keys,
	})
}

func (s *Server) handleInboundKeysCreate(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Label string `json:"label"`
	}
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "bad_body", err.Error())
			return
		}
	}
	body.Label = strings.TrimSpace(body.Label)
	if len(body.Label) > 128 {
		writeJSONError(w, http.StatusBadRequest, "label_too_long", "label max 128 chars")
		return
	}

	created, err := s.opts.InboundKeyProvider.Create(r.Context(), body.Label)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "create_failed", err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (s *Server) handleInboundKeysRevoke(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		writeJSONError(w, http.StatusBadRequest, "missing_id", "id path segment required")
		return
	}
	err := s.opts.InboundKeyProvider.Revoke(r.Context(), id)
	if err != nil {
		if err.Error() == "tokenvault: inbound key not found" {
			writeJSONError(w, http.StatusNotFound, "not_found", "inbound key not found")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "revoke_failed", err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// writeJSONError is a local helper — avoids pulling in httpx which already
// depends on encoding/json/v2 and would drag its error shape across.
func writeJSONError(w http.ResponseWriter, status int, code, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"code":    code,
		"message": msg,
		"ts":      time.Now().UTC().Format(time.RFC3339),
	})
}
