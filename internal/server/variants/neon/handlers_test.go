package neon

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
	req := httptest.NewRequest(http.MethodGet, "/dashboard-neon", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), `data-theme="neon"`) {
		t.Errorf("missing data-theme=\"neon\" attribute")
	}
	if !strings.Contains(string(body), "bg-grid") {
		t.Errorf("missing bg-grid — grid-as-chrome is a signature of neon")
	}
}

func TestHandleAsset_PathTraversal(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	for _, url := range []string{
		"/dashboard-neon/assets/../handlers.go",
		"/dashboard-neon/assets/%2e%2e/handlers.go",
		"/dashboard-neon/assets/..%2Fhandlers.go",
	} {
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
	req := httptest.NewRequest(http.MethodGet, "/dashboard-neon/assets/nope.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing asset returned %d; want 404", rec.Code)
	}
}

func TestHandleAsset_ServesJS(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/dashboard-neon/assets/app.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/javascript") {
		t.Errorf("Content-Type = %q, want application/javascript prefix", ct)
	}
}

func TestContentTypeFor(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"app.js", "application/javascript; charset=utf-8"},
		{"app.css", "text/css; charset=utf-8"},
		{"index.html", "text/html; charset=utf-8"},
		{"glyph.svg", "image/svg+xml"},
		{"unknown.xyz", "application/octet-stream"},
	} {
		if got := contentTypeFor(c.name); got != c.want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
