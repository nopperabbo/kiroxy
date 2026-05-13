package server

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type stubDashboardProvider struct{}

func (stubDashboardProvider) DashboardSnapshot(_ context.Context) DashboardState {
	return DashboardState{
		Version: "test",
		Ready:   true,
		VaultOK: true,
		Accounts: []DashboardAccount{
			{ID: "a1", Enabled: true, Requests: 7, Errors: 0},
		},
	}
}

func TestM10_DashboardHTMLServed(t *testing.T) {
	srv := New(Options{
		APIKey:                 "secret",
		DashboardStateProvider: stubDashboardProvider{},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Avoid the default redirect-following so we can inspect the 302.
	client := &http.Client{
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(ts.URL + "/dashboard")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Fatalf("dashboard (loopback) want 302, got %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/dashboard-mansion" {
		t.Fatalf("redirect Location want /dashboard-mansion, got %q", loc)
	}
}

func TestM10_DashboardStateEndpointReturnsSnapshot(t *testing.T) {
	srv := New(Options{
		APIKey:                 "secret",
		DashboardStateProvider: stubDashboardProvider{},
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/dashboard/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		t.Fatalf("state api want 200, got %d", resp.StatusCode)
	}
	for _, want := range []string{
		`"ready":true`,
		`"vault_ok":true`,
		`"accounts":`,
		`"id":"a1"`,
		`"requests":7`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("missing %s in state body: %s", want, body)
		}
	}
}

// TestM10_DashboardRequiresKeyFromNonLoopback fakes a non-loopback RemoteAddr
// via an httptest server addressed by 0.0.0.0 to exercise the auth-bypass
// condition. We cannot force a non-loopback RemoteAddr on a local httptest
// loopback listener, so we instead unit-test the isLoopback helper + wrap()
// against a synthetic request.
func TestM10_DashboardRequiresKeyFromNonLoopback(t *testing.T) {
	srv := New(Options{
		APIKey:                 "secret",
		DashboardStateProvider: stubDashboardProvider{},
	})
	h := srv.Handler()

	req := httptest.NewRequest("GET", "/dashboard", nil)
	req.RemoteAddr = "203.0.113.9:55555"
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != 401 {
		t.Fatalf("non-loopback dashboard without key: want 401, got %d body=%s",
			rr.Code, rr.Body.String())
	}

	// With a valid key, the redirect path runs to completion (302).
	req = httptest.NewRequest("GET", "/dashboard", nil)
	req.RemoteAddr = "203.0.113.9:55555"
	req.Header.Set("X-Api-Key", "secret")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusFound {
		t.Fatalf("non-loopback dashboard with correct key: want 302, got %d body=%s",
			rr.Code, rr.Body.String())
	}
}
