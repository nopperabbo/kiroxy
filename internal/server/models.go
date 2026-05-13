package server

import (
	"encoding/json"
	"net/http"

	"local/kiroxy/internal/models"
)

// ModelEntry is one row of the /dashboard/api/models response. The shape
// is dashboard-shaped: stable strings, no Go types, all fields safe to
// render directly. Usage stats are fetched separately from /metrics by
// the client when it wants them.
type ModelEntry struct {
	// Anthropic is the canonical client-facing ID, e.g. "claude-sonnet-4-6".
	Anthropic string `json:"anthropic"`
	// Kiro is the upstream Kiro SKU sent on the wire to AWS Q.
	Kiro string `json:"kiro"`
	// Kiro1M, when non-empty, is the 1M-context variant Kiro SKU.
	Kiro1M string `json:"kiro_1m,omitempty"`
	// ContextWindowSize is the routed context window in tokens, in
	// thousands ("200K"/"1M" rendering happens client-side).
	ContextWindowSize int `json:"context_window_size"`
	// Family is "opus" / "sonnet" / "haiku" derived from the Anthropic id.
	// Lets the dashboard group rows.
	Family string `json:"family"`
	// Tier is "free" or "pro" — derived from Family. Opus is Pro-tier on
	// Kiro, the rest are Free. Heuristic; operators may override later.
	Tier string `json:"tier"`
	// IsThinking is true for [1m]-suffixed entries (always-1M variants).
	IsThinking bool `json:"is_thinking"`
}

// ModelsResponse is the shape returned by /dashboard/api/models.
type ModelsResponse struct {
	Models       []ModelEntry `json:"models"`
	DefaultModel string       `json:"default_model"`
}

// registerModelsHandler is unconditional — the model table is static and
// always available; no provider injection needed.
func (s *Server) registerModelsHandler(mux *http.ServeMux) {
	mux.HandleFunc("GET /dashboard/api/models", s.handleModels)
}

func (s *Server) handleModels(w http.ResponseWriter, _ *http.Request) {
	resp := ModelsResponse{
		Models:       BuildModelTable(),
		DefaultModel: models.DefaultAnthropicModel,
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "max-age=60")
	_ = json.NewEncoder(w).Encode(resp)
}

// BuildModelTable converts the canonical model list into the dashboard
// shape. The table is derived by calling models.Resolve() on each Kiro
// SKU returned by models.ListModels(), so changes to canonical mappings
// flow into the dashboard automatically without touching models.go.
func BuildModelTable() []ModelEntry {
	out := make([]ModelEntry, 0, 8)
	seen := make(map[string]bool, 8)

	// The curated Anthropic IDs we advertise. The resolver accepts many
	// dashed/dotted aliases; we show only the canonical form most Claude
	// clients send ("claude-sonnet-4-6" etc). This list matches the base
	// rows in internal/models/models.go modelMapOrdered.
	anthropicIDs := []string{
		"claude-opus-4-7[1m]",
		"claude-opus-4-7",
		"claude-opus-4-6",
		"claude-opus-4-5",
		"claude-sonnet-4-6",
		"claude-sonnet-4-5",
		"claude-haiku-4-5",
	}

	for _, aid := range anthropicIDs {
		// Dedup on the input ID, NOT the resolver's echoed form: the
		// resolver appends [1m] to always-1M models so two distinct
		// inputs (claude-opus-4-7 vs claude-opus-4-7[1m]) collapse to
		// the same echoed name.
		if seen[aid] {
			continue
		}
		seen[aid] = true
		kiro, thinking, ctxWindow, echoed := models.Resolve(aid, false)
		entry := ModelEntry{
			Anthropic:         aid,
			Kiro:              kiro,
			ContextWindowSize: ctxWindow,
			Family:            familyOf(aid),
			Tier:              tierOf(aid),
			IsThinking:        thinking || endsWithThinking(echoed),
		}
		kiro1m, _, ctx1m, _ := models.Resolve(aid, true)
		if kiro1m != kiro {
			entry.Kiro1M = kiro1m
			if ctx1m > entry.ContextWindowSize {
				entry.ContextWindowSize = ctx1m
			}
		}
		out = append(out, entry)
	}
	return out
}

func familyOf(id string) string {
	switch {
	case containsLowerSubstring(id, "opus"):
		return "opus"
	case containsLowerSubstring(id, "sonnet"):
		return "sonnet"
	case containsLowerSubstring(id, "haiku"):
		return "haiku"
	default:
		return "other"
	}
}

func tierOf(id string) string {
	if containsLowerSubstring(id, "opus") {
		return "pro"
	}
	return "free"
}

func endsWithThinking(id string) bool {
	const suffix = "[1m]"
	if len(id) < len(suffix) {
		return false
	}
	return id[len(id)-len(suffix):] == suffix
}

// containsLowerSubstring is local to keep the file dependency-free.
func containsLowerSubstring(haystack, needle string) bool {
	return strContainsFold(haystack, needle)
}
