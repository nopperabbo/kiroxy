// Embedded operator documentation surface for the Mansion dashboard.
//
// Exposes GET /dashboard/api/docs/index — a JSON catalog of selected
// docs/*.md files shipped inside the binary via go:embed. The Mansion
// command palette (⌘K) consumes this to provide inline doc search
// instead of linking out to GitHub.
//
// Source of truth: ./embedded_docs/ is a mirror of the authoritative
// tree under ../../docs/. Go's //go:embed directive cannot traverse
// upward out of the current package directory, so we keep a curated
// copy here and refresh it via `make docs-sync` (see Makefile).
//
// Rationale for a curated subset (README, ARCHITECTURE, TROUBLESHOOTING,
// OPENCODE, OPENAI, METRICS, VISION): these are the seven operator-
// facing references the palette needs. Design-system docs, variant
// manifestos, and roadmap files are intentionally omitted — they are
// contributor artefacts, not operator help.
//
// Caching: the handler serializes the catalog once at package init and
// serves the same byte slice on every request. The catalog is tiny
// (~75KB uncompressed across seven docs) and the embed is already
// compiled into the binary; rebuilding JSON per request would be pure
// overhead. Browsers can additionally rely on Cache-Control:no-cache
// so operators see freshness after a server restart without surprises.

package server

import (
	"embed"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"sync"
)

// embeddedDocsFS contains the curated operator docs rendered inline by
// the Mansion command palette. The `all:` prefix is defensive — none of
// these files start with "." today but future additions shouldn't need
// an accompanying directive change.
//
//go:embed all:embedded_docs
var embeddedDocsFS embed.FS

// DocEntry is one row of the /dashboard/api/docs/index response. Shape
// is intentionally flat: a TypeScript client can `type` it with a plain
// interface and feed it into a fuzzy-search ranker without unwrapping.
type DocEntry struct {
	// Path is the relative path used as the stable identifier. Always
	// `<filename>.md` (flat layout — no nested folders in the curated
	// set). Clients treat this as opaque and never concatenate it into
	// outbound URLs.
	Path string `json:"path"`
	// Title is the first H1 of the document, or the filename-derived
	// fallback when no H1 is present. Used as the palette row label.
	Title string `json:"title"`
	// Content is the full markdown body. The client renders it inline
	// via a tiny markdown-to-HTML pass; we do not strip anything
	// server-side because the palette's preview pane wants fidelity.
	Content string `json:"content"`
	// Bytes is the content size so clients can show a soft "long doc"
	// hint without having to measure the string themselves. Encoded
	// separately from Content.length because runes vs bytes differ for
	// non-ASCII content.
	Bytes int `json:"bytes"`
}

// docsCatalog is built once and reused on every request.
var (
	docsCatalogOnce  sync.Once
	docsCatalogBytes []byte
	docsCatalogErr   error
)

func buildDocsCatalog() ([]byte, error) {
	sub, err := fs.Sub(embeddedDocsFS, "embedded_docs")
	if err != nil {
		return nil, err
	}
	entries, err := fs.ReadDir(sub, ".")
	if err != nil {
		return nil, err
	}
	// Stable ordering — the palette renders results in catalog order
	// until the operator types a query. README first, then the other
	// files alphabetically so the most-obvious entry point is always
	// at the top.
	sort.Slice(entries, func(i, j int) bool {
		ni, nj := entries[i].Name(), entries[j].Name()
		if ni == "README.md" {
			return true
		}
		if nj == "README.md" {
			return false
		}
		return ni < nj
	})
	out := make([]DocEntry, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".md") {
			continue
		}
		raw, rerr := fs.ReadFile(sub, name)
		if rerr != nil {
			return nil, rerr
		}
		content := string(raw)
		out = append(out, DocEntry{
			Path:    name,
			Title:   extractTitle(content, name),
			Content: content,
			Bytes:   len(raw),
		})
	}
	if len(out) == 0 {
		// Defensive: an empty embed would ship a {"docs":[]} catalog
		// silently. We'd rather fail loudly so a refactor that breaks
		// the embed is caught by the smoke test on first boot.
		return nil, errors.New("embedded_docs: catalog empty — check go:embed directive")
	}
	return json.Marshal(struct {
		Docs []DocEntry `json:"docs"`
	}{Docs: out})
}

// extractTitle pulls the first H1 out of a markdown body. We tolerate
// blank leading lines, trim trailing annotations like "— kiroxy", and
// strip a trailing ".md" (some docs use their filename as the H1 for
// symmetry) so the palette label stays short and human.
func extractTitle(body, fallback string) string {
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "# ") {
			title := strings.TrimSpace(strings.TrimPrefix(trim, "# "))
			if idx := strings.Index(title, " — "); idx >= 0 {
				title = strings.TrimSpace(title[:idx])
			}
			title = strings.TrimSuffix(title, ".md")
			if title != "" {
				return title
			}
		}
	}
	name := strings.TrimSuffix(fallback, ".md")
	return name
}

// docsCatalogJSON returns the marshaled catalog, building it once. Used
// by tests to exercise the loader without spinning a whole http.Server.
func docsCatalogJSON() ([]byte, error) {
	docsCatalogOnce.Do(func() {
		docsCatalogBytes, docsCatalogErr = buildDocsCatalog()
	})
	if docsCatalogErr != nil {
		return nil, docsCatalogErr
	}
	return docsCatalogBytes, nil
}

func (s *Server) handleDocsIndex(w http.ResponseWriter, _ *http.Request) {
	b, err := docsCatalogJSON()
	if err != nil {
		// The catalog build failure is a programmer error (bad embed
		// directive, empty source tree); surface it honestly so the
		// operator isn't confused by a palette showing "0 docs".
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "docs catalog unavailable: " + err.Error(),
		})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	// The palette caches the response on first open; asking browsers
	// to revalidate keeps restart semantics predictable without giving
	// up the cache entirely.
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	_, _ = w.Write(b)
}
