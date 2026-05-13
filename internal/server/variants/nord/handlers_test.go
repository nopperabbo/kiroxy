package nord

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
	req := httptest.NewRequest(http.MethodGet, "/_variants/nord", nil)
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
	if !strings.Contains(string(body), `data-theme="nord"`) {
		t.Errorf("missing data-theme=\"nord\" attribute")
	}
}

func TestHandleAsset_PathTraversal(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	for _, url := range []string{
		"/dashboard-nord/assets/../handlers.go",
		"/dashboard-nord/assets/%2e%2e/handlers.go",
		"/dashboard-nord/assets/..%2Fhandlers.go",
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
	req := httptest.NewRequest(http.MethodGet, "/dashboard-nord/assets/nope.js", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing asset returned %d; want 404", rec.Code)
	}
}

func TestHandleAsset_ServesCSS(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)
	req := httptest.NewRequest(http.MethodGet, "/dashboard-nord/assets/app.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
}

func TestContentTypeFor(t *testing.T) {
	for _, c := range []struct{ name, want string }{
		{"app.js", "application/javascript; charset=utf-8"},
		{"app.css", "text/css; charset=utf-8"},
		{"index.html", "text/html; charset=utf-8"},
		{"x.svg", "image/svg+xml"},
		{"unknown.xyz", "application/octet-stream"},
	} {
		if got := contentTypeFor(c.name); got != c.want {
			t.Errorf("contentTypeFor(%q) = %q, want %q", c.name, got, c.want)
		}
	}
}
