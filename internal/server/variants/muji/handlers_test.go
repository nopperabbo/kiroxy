package muji

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// stubSnap returns a fixed snapshot used to drive the template render
// tests. Lets us assert on rendered HTML without a live server.
func stubSnap(_ context.Context) Snapshot {
	return Snapshot{
		Version: "v1.0.0",
		UptimeS: 3700,
		Ready:   true,
		VaultOK: true,
		Accounts: []Account{
			{ID: "acct-1", Enabled: true, Requests: 42, Errors: 0},
			{ID: "acct-2", Enabled: false, Requests: 0, Errors: 0},
			{ID: "acct-3", Enabled: true, Requests: 7, Errors: 1, LastError: "403 forbidden"},
		},
	}
}

func TestHandleIndex_RendersHTML(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, stubSnap)
	req := httptest.NewRequest(http.MethodGet, "/_variants/muji", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("Content-Type = %q, want text/html prefix", ct)
	}
	body, _ := io.ReadAll(rec.Body)
	bs := string(body)
	if !strings.Contains(bs, `data-theme="muji"`) {
		t.Errorf("missing data-theme=\"muji\" attribute")
	}
	if !strings.Contains(bs, `meta http-equiv="refresh"`) {
		t.Errorf("missing meta refresh — muji is zero-JS and depends on it")
	}
	if !strings.Contains(bs, "v1.0.0") {
		t.Errorf("expected version v1.0.0 to render in body")
	}
	if !strings.Contains(bs, "49 requests") {
		t.Errorf("expected 49 requests total in summary; body did not contain it")
	}
	if !strings.Contains(bs, "acct-1") {
		t.Errorf("expected acct-1 to render in pool table")
	}
	// muji philosophy: NO <script> tag whatsoever in the rendered page.
	if strings.Contains(strings.ToLower(bs), "<script") {
		t.Errorf("muji rendered HTML must contain NO <script> tag — philosophy")
	}
}

func TestHandleIndex_NilSnap(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, nil)
	req := httptest.NewRequest(http.MethodGet, "/_variants/muji", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 with nil snap fn", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !strings.Contains(string(body), "no accounts yet") {
		t.Errorf("nil snap must render the empty-pool void message")
	}
}

func TestHandleAsset_PathTraversal(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, stubSnap)
	for _, url := range []string{
		"/dashboard-muji/assets/../handlers.go",
		"/dashboard-muji/assets/%2e%2e/handlers.go",
		"/dashboard-muji/assets/..%2Fhandlers.go",
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
	Register(mux, stubSnap)
	req := httptest.NewRequest(http.MethodGet, "/dashboard-muji/assets/nope.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing asset returned %d; want 404", rec.Code)
	}
}

func TestHandleAsset_ServesCSS(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, stubSnap)
	req := httptest.NewRequest(http.MethodGet, "/dashboard-muji/assets/app.css", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		t.Errorf("Content-Type = %q, want text/css", ct)
	}
}

func TestFormatUptime(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{-1, "—"},
		{45, "0m"},
		{120, "2m"},
		{3700, "1h 1m"},
		{90000, "1d 1h"},
	}
	for _, c := range cases {
		if got := formatUptime(c.in); got != c.want {
			t.Errorf("formatUptime(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestAccountState(t *testing.T) {
	cases := []struct {
		a    Account
		want string
	}{
		{Account{Enabled: true}, "active"},
		{Account{Enabled: false}, "paused"},
		{Account{Enabled: true, LastError: "oops"}, "error"},
		{Account{Enabled: true, CooldownUntil: "2026-05-13T11:00:00Z"}, "cooling"},
	}
	for _, c := range cases {
		if got := accountState(c.a); got != c.want {
			t.Errorf("accountState(%+v) = %q, want %q", c.a, got, c.want)
		}
	}
}
