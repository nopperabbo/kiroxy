package mansion

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newServer mounts the mansion routes on a fresh mux so each test runs in
// isolation and closes the server when done.
func newServer(t *testing.T) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	Register(mux)
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	return ts
}

// TestRegister_IndexServesHTML covers the happy path for /dashboard-mansion.
// We assert both the response code and a signal that the embedded HTML
// actually made it through (placeholder dist has "placeholder" in it; a
// built dist has "<div id=\"app\""; both contain "<html").
func TestRegister_IndexServesHTML(t *testing.T) {
	ts := newServer(t)

	res, err := http.Get(ts.URL + "/dashboard-mansion")
	if err != nil {
		t.Fatalf("GET /dashboard-mansion: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Fatalf("want text/html, got %q", ct)
	}
	if cc := res.Header.Get("Cache-Control"); cc != "no-cache" {
		t.Fatalf("want Cache-Control=no-cache, got %q", cc)
	}
	if nosniff := res.Header.Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Fatalf("want X-Content-Type-Options=nosniff, got %q", nosniff)
	}
	body, _ := io.ReadAll(res.Body)
	if !strings.Contains(string(body), "<html") {
		t.Fatalf("response missing <html tag, got %q", string(body[:min(120, len(body))]))
	}
}

// TestRegister_AssetNotFound makes sure unknown asset requests 404 cleanly
// rather than leaking stack traces or serving index.html as a fallback
// (SPA-style fallback is deliberately NOT done here — mansion is embedded
// static, not a virtual-routed SPA).
func TestRegister_AssetNotFound(t *testing.T) {
	ts := newServer(t)

	res, err := http.Get(ts.URL + "/dashboard-mansion/assets/does-not-exist.js")
	if err != nil {
		t.Fatalf("GET missing asset: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res.StatusCode)
	}
}

// TestRegister_AssetTraversalRejected exercises the defense-in-depth for
// directory traversal. Even though Go's ServeMux + fs.Sub would usually
// catch this, we encode the "..", force the handler to decode it, and
// assert we still get 404.
func TestRegister_AssetTraversalRejected(t *testing.T) {
	ts := newServer(t)

	res, err := http.Get(ts.URL + "/dashboard-mansion/assets/..%2F..%2Fetc%2Fpasswd")
	if err != nil {
		t.Fatalf("GET traversal: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res.StatusCode)
	}
}

// TestContentTypeFor covers the happy cases for each extension we care
// about. Parameterized because more extensions get added as we grow the
// asset set (icons, fonts).
func TestContentTypeFor(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{"app.js", "application/javascript; charset=utf-8"},
		{"chunk-foo.mjs", "application/javascript; charset=utf-8"},
		{"app.css", "text/css; charset=utf-8"},
		{"icon.svg", "image/svg+xml"},
		{"index.html", "text/html; charset=utf-8"},
		{"manifest.json", "application/json; charset=utf-8"},
		{"inter.woff2", "font/woff2"},
		{"inter.woff", "font/woff"},
		{"app.js.map", "application/json; charset=utf-8"},
		{"favicon.ico", "image/x-icon"},
		{"screenshot.png", "image/png"},
		{"random.bin", "application/octet-stream"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			if got := contentTypeFor(tc.name); got != tc.want {
				t.Fatalf("contentTypeFor(%q) = %q, want %q", tc.name, got, tc.want)
			}
		})
	}
}
