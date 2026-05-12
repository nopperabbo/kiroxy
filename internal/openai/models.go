// Model alias resolution and the /v1/models list builder.
//
// OpenAI-flavor aliases (gpt-4o, gpt-4-turbo, gpt-3.5-turbo) map to sensible
// Claude equivalents per the operator's best-cost preference. The caller
// resolves the alias BEFORE handing off to messages.Service, so the
// downstream models.Resolve sees only Claude IDs it already understands.
// Unknown aliases pass through unchanged — if a caller already knows the
// Claude name (e.g. "claude-sonnet-4-6") it just works.

package openai

import (
	"sort"
	"strings"
	"time"

	"local/kiroxy/internal/models"
)

// aliasTable is the canonical OpenAI → Claude alias map. Order in the
// /v1/models listing is stable (sorted by OpenAI alias) so callers that
// read the first entry always get the same pick.
var aliasTable = map[string]string{
	"gpt-4o":        "claude-sonnet-4-6",
	"gpt-4o-mini":   "claude-sonnet-4-6",
	"gpt-4-turbo":   "claude-opus-4-7",
	"gpt-4":         "claude-opus-4-7",
	"gpt-3.5-turbo": "claude-haiku-4.5",
	"o1":            "claude-opus-4-7",
	"o1-mini":       "claude-sonnet-4-6",
}

// ResolveModel maps an OpenAI-facing model name to the ID sent down to the
// Anthropic pipeline. Pass-through rules:
//
//   - aliasTable hit → mapped Claude ID
//   - "openai/<x>" prefix → strip, then re-resolve x
//   - "claude-..." already → pass through verbatim
//   - anything else → pass through verbatim (models.Resolve has its own
//     fallback; an unknown non-claude model will fall back to the default
//     there rather than erroring here, so OpenAI clients with odd model IDs
//     still get a response)
func ResolveModel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return name
	}
	if strings.HasPrefix(name, "openai/") {
		return ResolveModel(strings.TrimPrefix(name, "openai/"))
	}
	if mapped, ok := aliasTable[name]; ok {
		return mapped
	}
	return name
}

// ListModels returns the combined set of model IDs accepted by the
// OpenAI-compat surface: every Kiro model from internal/models plus every
// OpenAI alias. Each entry shares the same `created` timestamp (process
// start) to keep the listing stable across a single process lifetime.
func ListModels() ModelList {
	seen := make(map[string]struct{})
	var ids []string

	for _, m := range models.ListModels() {
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		ids = append(ids, m)
	}
	for alias := range aliasTable {
		if _, ok := seen[alias]; ok {
			continue
		}
		seen[alias] = struct{}{}
		ids = append(ids, alias)
	}
	sort.Strings(ids)

	created := time.Now().Unix()
	data := make([]Model, 0, len(ids))
	for _, id := range ids {
		data = append(data, Model{
			ID:      id,
			Object:  ObjectModel,
			Created: created,
			OwnedBy: "kiroxy",
		})
	}
	return ModelList{Object: ObjectList, Data: data}
}
