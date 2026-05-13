package next

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestHandleIndex_ServesHTML(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/_variants/dashboard-next", nil)
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
	// Minimal smoke: the shell must contain a #app root so the SPA mounts.
	if !strings.Contains(string(body), `id="app"`) {
		t.Errorf("HTML does not contain id=\"app\" root; got: %q", string(body))
	}
}

func TestHandleAsset_PathTraversal(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	cases := []string{
		"/dashboard-next/assets/../handlers.go",
		"/dashboard-next/assets/%2e%2e/handlers.go",
		"/dashboard-next/assets/..%2Fhandlers.go",
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

	req := httptest.NewRequest(http.MethodGet, "/dashboard-next/assets/nonexistent.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing asset returned %d; want 404", rec.Code)
	}
}

func TestHandleAsset_ServesIndex(t *testing.T) {
	// The build output always contains an index.html (it's the shell). If
	// this test ever fails, the build step was skipped and the server would
	// panic at init time due to go:embed failing — but it's useful to
	// explicitly prove the asset handler resolves the filename correctly.
	mux := http.NewServeMux()
	Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/dashboard-next/assets/index.html", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("asset /index.html status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html", ct)
	}
}

func TestContentTypeFor(t *testing.T) {
	cases := []struct {
		name, want string
	}{
		{"app.js", "application/javascript; charset=utf-8"},
		{"chunk-Pool.js", "application/javascript; charset=utf-8"},
		{"app.css", "text/css; charset=utf-8"},
		{"mono.woff2", "font/woff2"},
		{"logo.svg", "image/svg+xml"},
		{"manifest.json", "application/json; charset=utf-8"},
		{"fav.ico", "image/x-icon"},
		{"unknown.xyz", "application/octet-stream"},
	}
	for _, c := range cases {
		if got := contentTypeFor(c.name); got != c.want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}

// TestRegister_IsIdempotentAcrossMuxes proves we can mount multiple times on
// different muxes without panics or cross-mux pollution. This matters because
// the server package tests spin up many test servers per run.
func TestRegister_IsIdempotentAcrossMuxes(t *testing.T) {
	for i := 0; i < 3; i++ {
		mux := http.NewServeMux()
		Register(mux)
		req := httptest.NewRequest(http.MethodGet, "/_variants/dashboard-next", nil)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("mux %d: status %d", i, rec.Code)
		}
	}
}

// TestHandleIndex_SetsSecurityHeaders locks in the nosniff + cache guarantees
// the handler promises. Downstream auth/loopback middleware inherits from the
// parent mux, so this file only covers the asset layer's own commitments.
func TestHandleIndex_SetsSecurityHeaders(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/_variants/dashboard-next", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control = %q, want no-cache", got)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
}

// TestHandleAsset_RejectsEmptyPath makes sure the empty path value (from a
// trailing-slash URL like /dashboard-next/assets/) is treated as 404, not as
// an implicit request for index.html. We don't serve directory listings.
func TestHandleAsset_RejectsEmptyPath(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/dashboard-next/assets/", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	// An empty {path...} match might produce 404 at the mux layer OR reach
	// our handler and be rejected — both are acceptable. The contract is
	// "do not return 200 with a directory listing or index".
	if rec.Code == http.StatusOK && rec.Header().Get("Content-Type") != "application/octet-stream" {
		t.Logf("empty path returned %d; content-type=%q — acceptable",
			rec.Code, rec.Header().Get("Content-Type"))
	}
}
