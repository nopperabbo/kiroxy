package brutal

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleIndex_ServesHTML is the minimal mount + render smoke test,
// mirroring the shape of next/mansion tests so all three variants are
// audited the same way.
func TestHandleIndex_ServesHTML(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/_variants/brutal", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("missing X-Content-Type-Options: nosniff header")
	}
	body, _ := io.ReadAll(rec.Body)
	if len(body) == 0 {
		t.Fatal("empty body; expected HTML shell")
	}
	// Philosophy-check: the shell must contain the "POOL MONITOR"
	// signature so we don't accidentally ship a generic template.
	if !strings.Contains(string(body), "POOL MONITOR") {
		t.Errorf("shell missing POOL MONITOR signature; got first 120 chars: %q",
			string(body)[:min(120, len(body))])
	}
}

// TestHandleAsset_PathTraversal mirrors next/mansion tests — identical
// hardening across all variants is a maintainability property.
func TestHandleAsset_PathTraversal(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	cases := []string{
		"/dashboard-brutal/assets/../handlers.go",
		"/dashboard-brutal/assets/%2e%2e/handlers.go",
		"/dashboard-brutal/assets/..%2Fhandlers.go",
	}
	for _, url := range cases {
		req := httptest.NewRequest(http.MethodGet, url, nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code == http.StatusOK {
			t.Errorf("traversal %q returned 200; want 404", url)
		}
	}
}

func TestHandleAsset_NotFound(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/dashboard-brutal/assets/nope.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing asset returned %d; want 404", rec.Code)
	}
}

// TestHandleAsset_ServesCSS verifies the asset handler resolves a known
// filename and sets the right Content-Type — a regression-catch for
// contentTypeFor changes.
func TestHandleAsset_ServesCSS(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/dashboard-brutal/assets/app.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css prefix", ct)
	}
}

func TestContentTypeFor(t *testing.T) {
	cases := []struct {
		name, want string
	}{
		{"app.js", "application/javascript; charset=utf-8"},
		{"app.css", "text/css; charset=utf-8"},
		{"index.html", "text/html; charset=utf-8"},
		{"mono.woff2", "font/woff2"},
		{"logo.svg", "image/svg+xml"},
		{"unknown.xyz", "application/octet-stream"},
	}
	for _, c := range cases {
		if got := contentTypeFor(c.name); got != c.want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
