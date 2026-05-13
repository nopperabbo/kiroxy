package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDocsCatalog_BuildsAndContainsCoreDocs(t *testing.T) {
	raw, err := docsCatalogJSON()
	if err != nil {
		t.Fatalf("catalog build failed: %v", err)
	}
	var payload struct {
		Docs []DocEntry `json:"docs"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatalf("catalog JSON parse failed: %v", err)
	}
	if len(payload.Docs) == 0 {
		t.Fatalf("catalog is empty — embed directive likely broken")
	}
	paths := map[string]bool{}
	for _, d := range payload.Docs {
		paths[d.Path] = true
		if d.Title == "" {
			t.Errorf("doc %q has empty title", d.Path)
		}
		if d.Content == "" {
			t.Errorf("doc %q has empty content", d.Path)
		}
		if d.Bytes <= 0 {
			t.Errorf("doc %q reports non-positive byte count %d", d.Path, d.Bytes)
		}
	}
	for _, required := range []string{
		"README.md", "ARCHITECTURE.md", "TROUBLESHOOTING.md",
		"OPENCODE.md", "OPENAI.md", "METRICS.md", "VISION.md",
	} {
		if !paths[required] {
			t.Errorf("expected %s in catalog, missing", required)
		}
	}
	if payload.Docs[0].Path != "README.md" {
		t.Errorf("expected README.md first; got %q", payload.Docs[0].Path)
	}
}

func TestDocsIndexHandler_RespondsJSON(t *testing.T) {
	s := &Server{}
	req := httptest.NewRequest(http.MethodGet, "/dashboard/api/docs/index", nil)
	rec := httptest.NewRecorder()
	s.handleDocsIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("expected application/json content type; got %q", ct)
	}
	if rec.Header().Get("Cache-Control") == "" {
		t.Errorf("expected Cache-Control header to be set")
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff")
	}
	var payload struct {
		Docs []DocEntry `json:"docs"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("response JSON parse failed: %v", err)
	}
	if len(payload.Docs) < 3 {
		t.Errorf("expected at least 3 docs in response; got %d", len(payload.Docs))
	}
}

func TestExtractTitle_HandlesH1AndFallback(t *testing.T) {
	cases := []struct {
		in       string
		fallback string
		want     string
	}{
		{"# Hello\nbody", "x.md", "Hello"},
		{"\n\n# Hello World — subtitle\nbody", "x.md", "Hello World"},
		{"no heading here", "readme.md", "readme"},
		{"# \nempty h1", "z.md", "z"},
		{"# VISION.md — kiroxy\nbody", "VISION.md", "VISION"},
		{"# docs.md\nbody", "docs.md", "docs"},
	}
	for _, c := range cases {
		if got := extractTitle(c.in, c.fallback); got != c.want {
			t.Errorf("extractTitle(%q,%q) = %q, want %q", c.in, c.fallback, got, c.want)
		}
	}
}
