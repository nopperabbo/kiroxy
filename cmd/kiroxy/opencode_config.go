package main

// opencode_config.go emits an opencode.ai provider-config JSON snippet that
// points opencode at a running kiroxy instance. The user copies the snippet
// into ~/.config/opencode/opencode.json under the top-level `provider` key.
//
// Schema source (confirmed 2026-05-12):
//   - https://opencode.ai/docs/config/
//   - https://opencode.ai/docs/providers/
//
// Notable opencode schema quirks this emitter is careful about:
//   - Top-level key is `provider` (singular), not `providers`.
//   - `npm` identifies wire protocol. "@ai-sdk/anthropic" for Anthropic wire.
//     (Note: "@ai-sdk/anthropic-compatible" does NOT exist.)
//   - `models` is a MAP keyed by model-id, not an array.
//   - `options.baseURL` and `options.apiKey` are camelCase.
//   - `{env:VAR}` / `{file:~/path}` interpolation works inside any string.
//
// Model-ID policy (critical):
// The kirocc model resolver at internal/models/models.go does exact-string
// matching against `m.Anthropic` and `m.Kiro`. Unrecognised names fall
// through:
//   - claude-* prefix: passthrough to upstream as-is (may be rejected)
//   - non-claude-* prefix: SILENTLY rewritten to claude-sonnet-4.6
// To avoid silent fallback we emit only IDs that appear verbatim in the
// resolver's mapping table. Kiro UI display labels (e.g. "kiro/opus-4.7")
// are NOT valid API IDs and will silent-fallback; we do not emit them.
// See docs/OPENCODE.md for the full API-ID <-> UI-label mapping table.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// knownModel is one resolver-verified model plus metadata for the emitted
// opencode `models` map. Each ID here MUST appear as `Anthropic` in
// internal/models/models.go:modelMapOrdered so the resolver round-trips it
// without falling back to DefaultModel.
type knownModel struct {
	// ID is the opencode-facing model id (left column of opencode `models`
	// map, and the canonical Anthropic-form string). Must match
	// internal/models.modelMapOrdered[_].Anthropic EXACTLY.
	ID string
	// DisplayName is the human label shown in opencode's model picker.
	DisplayName string
}

// knownModels enumerates API IDs that the kirocc resolver recognises.
// Kept in the same order the resolver walks its map: specific [1m] variants
// first, then base variants, then legacy. The caller order does not affect
// correctness; it affects the JSON output order only.
//
// Rationale for the set (cross-referenced against modelMapOrdered):
//
//	claude-opus-4-7           → routes to Kiro "claude-opus-4.7"    (1M)
//	claude-opus-4-6           → routes to Kiro "claude-opus-4.6"    (1M)
//	claude-opus-4.5           → routes to Kiro "claude-opus-4.5"    (200K)
//	claude-sonnet-4-6         → routes to Kiro "claude-sonnet-4.6"  (200K)
//	claude-sonnet-4-6[1m]     → routes to Kiro "claude-sonnet-4.6-1m" (1M, thinking)
//	claude-sonnet-4.5         → routes to Kiro "claude-sonnet-4.5"  (200K)
//	claude-haiku-4.5          → routes to Kiro "claude-haiku-4.5"   (200K)
//
// Excluded (would silent-fallback to claude-sonnet-4.6):
//   - kiro/auto, kiro/sonnet-4 — not in resolver map
//   - kiro/deepseek-3.2, kiro/glm-5, kiro/minimax-m2.1,
//     kiro/minimax-m2.5, kiro/qwen3-coder-next — not in resolver map;
//     non-claude prefix guarantees silent fallback. Kiro UI may
//     surface these labels but kirocc has no route for them today.
var knownModels = []knownModel{
	{ID: "claude-opus-4-7", DisplayName: "Claude Opus 4.7 (1M via kiroxy)"},
	{ID: "claude-opus-4-6", DisplayName: "Claude Opus 4.6 (1M via kiroxy)"},
	{ID: "claude-opus-4.5", DisplayName: "Claude Opus 4.5 (via kiroxy)"},
	{ID: "claude-sonnet-4-6", DisplayName: "Claude Sonnet 4.6 (via kiroxy)"},
	{ID: "claude-sonnet-4-6[1m]", DisplayName: "Claude Sonnet 4.6 1M thinking (via kiroxy)"},
	{ID: "claude-sonnet-4.5", DisplayName: "Claude Sonnet 4.5 (via kiroxy)"},
	{ID: "claude-haiku-4.5", DisplayName: "Claude Haiku 4.5 (via kiroxy)"},
}

// opencodeModelEntry is the value shape under provider.<id>.models.<model-id>.
// opencode's schema allows `id`, `name`, `limit`, `experimental`, etc.; we
// emit only `name` for readability and let opencode infer the rest.
type opencodeModelEntry struct {
	Name string `json:"name"`
}

// opencodeProviderEntry is one entry under the top-level `provider` map.
type opencodeProviderEntry struct {
	NPM     string                        `json:"npm"`
	Name    string                        `json:"name"`
	Options opencodeProviderOptions       `json:"options"`
	Models  map[string]opencodeModelEntry `json:"models"`
}

type opencodeProviderOptions struct {
	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey"`
}

// opencodeSnippet is the outer shape the user pastes into opencode.json.
// We emit only the `provider` key — the user merges it with their existing
// config rather than replacing it.
type opencodeSnippet struct {
	Provider map[string]opencodeProviderEntry `json:"provider"`
}

// runOpencodeConfig implements the `kiroxy opencode-config` subcommand.
// Signature matches the other runX handlers in this package.
func runOpencodeConfig(_ context.Context, args []string) error {
	return runOpencodeConfigTo(args, os.Stdout, os.Stderr)
}

// runOpencodeConfigTo is the same as runOpencodeConfig but with pluggable
// writers, so tests can capture stdout/stderr without poking globals.
func runOpencodeConfigTo(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("opencode-config", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "usage: kiroxy opencode-config [flags]")
		fmt.Fprintln(stderr, "Emit an opencode.ai provider-config JSON snippet.")
		fmt.Fprintln(stderr, "")
		fs.PrintDefaults()
	}

	var (
		baseURL      = fs.String("base-url", "http://localhost:8787", "kiroxy base URL opencode will call")
		apiKey       = fs.String("api-key", "", "inbound API key (default: $KIROXY_INBOUND_KEY, else 'changeme')")
		providerName = fs.String("provider-name", "kiroxy", "provider id slug used in opencode.json (shows in model refs)")
		modelsFilter = fs.String("models", "", "optional comma-separated subset of API IDs; empty = emit all resolver-verified models")
		output       = fs.String("output", "", "write JSON to this file instead of stdout; empty = stdout")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	// Resolve the API key in order: --api-key > $KIROXY_INBOUND_KEY > "changeme".
	// We only fall back to "changeme" when BOTH sources are empty; this matches
	// how dev-mode kiroxy behaves and keeps the snippet obviously-placeholder.
	effectiveKey := *apiKey
	if effectiveKey == "" {
		effectiveKey = os.Getenv("KIROXY_INBOUND_KEY")
	}
	if effectiveKey == "" {
		effectiveKey = "changeme"
	}

	// Build the model list. If --models is set, we keep only IDs that are
	// present in knownModels AND in the filter set. Filter entries that
	// don't match a known model produce a stderr warning so users catch
	// typos early (the Kiro upstream would silently fallback otherwise).
	selected, unknownFilters, err := selectModels(knownModels, *modelsFilter)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return fmt.Errorf("no models selected after filter %q; available: %s", *modelsFilter, joinIDs(knownModels))
	}
	for _, u := range unknownFilters {
		fmt.Fprintf(stderr, "warning: --models filter entry %q is not in the resolver-verified set; omitted\n", u)
	}

	// Build the snippet. `models` is a map, not an array — opencode schema.
	modelsMap := make(map[string]opencodeModelEntry, len(selected))
	for _, m := range selected {
		modelsMap[m.ID] = opencodeModelEntry{Name: m.DisplayName}
	}
	snippet := opencodeSnippet{
		Provider: map[string]opencodeProviderEntry{
			*providerName: {
				NPM:  "@ai-sdk/anthropic",
				Name: "kiroxy (self-hosted Kiro proxy)",
				Options: opencodeProviderOptions{
					BaseURL: *baseURL,
					APIKey:  effectiveKey,
				},
				Models: modelsMap,
			},
		},
	}

	buf, err := json.MarshalIndent(snippet, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal opencode snippet: %w", err)
	}
	buf = append(buf, '\n')

	if *output != "" {
		if err := os.WriteFile(*output, buf, 0o600); err != nil {
			return fmt.Errorf("write %s: %w", *output, err)
		}
		fmt.Fprintf(stderr, "wrote %s (%d bytes, %d models)\n", *output, len(buf), len(selected))
	} else {
		if _, err := stdout.Write(buf); err != nil {
			return err
		}
	}

	// Guidance emitted to stderr only so stdout stays a clean JSON stream
	// that's safe to pipe: `kiroxy opencode-config | jq ...` just works.
	fmt.Fprintln(stderr, "# Paste the above under 'provider' in ~/.config/opencode/opencode.json")
	fmt.Fprintln(stderr, "# Do NOT overwrite existing providers — merge manually.")
	fmt.Fprintln(stderr, "# Top-level key is 'provider' (singular) per opencode schema.")
	if effectiveKey == "changeme" {
		fmt.Fprintln(stderr, "# WARNING: api-key is the placeholder 'changeme'. Set KIROXY_INBOUND_KEY or pass -api-key.")
	}

	return nil
}

// selectModels filters knownModels by the user's --models comma-separated
// list. Empty filter returns all. Returns selected models, unknown filter
// entries (for warning), and any parse error.
func selectModels(all []knownModel, filter string) ([]knownModel, []string, error) {
	filter = strings.TrimSpace(filter)
	if filter == "" {
		out := make([]knownModel, len(all))
		copy(out, all)
		return out, nil, nil
	}

	wanted := map[string]struct{}{}
	for _, tok := range strings.Split(filter, ",") {
		tok = strings.TrimSpace(tok)
		if tok == "" {
			continue
		}
		wanted[tok] = struct{}{}
	}
	if len(wanted) == 0 {
		return nil, nil, fmt.Errorf("--models was set but contained no non-empty entries")
	}

	known := map[string]struct{}{}
	for _, m := range all {
		known[m.ID] = struct{}{}
	}

	var selected []knownModel
	for _, m := range all {
		if _, ok := wanted[m.ID]; ok {
			selected = append(selected, m)
		}
	}

	var unknown []string
	for w := range wanted {
		if _, ok := known[w]; !ok {
			unknown = append(unknown, w)
		}
	}
	sort.Strings(unknown)
	return selected, unknown, nil
}

// joinIDs returns a human-friendly comma-separated list of the given
// models' IDs. Used in error messages only.
func joinIDs(ms []knownModel) string {
	ids := make([]string, len(ms))
	for i, m := range ms {
		ids[i] = m.ID
	}
	return strings.Join(ids, ", ")
}
