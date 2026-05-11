package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestM6_AuthMiddleware_TableDriven(t *testing.T) {
	goodKey := "sk-good-key-1234"
	srv := New(Options{APIKey: goodKey})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	tests := []struct {
		name       string
		setHeader  func(*http.Request)
		wantStatus int
		wantCode   string
	}{
		{
			name:       "valid X-Api-Key",
			setHeader:  func(r *http.Request) { r.Header.Set("X-Api-Key", goodKey) },
			wantStatus: 200,
		},
		{
			name:       "valid Bearer",
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+goodKey) },
			wantStatus: 200,
		},
		{
			name:       "valid Bearer lowercase scheme",
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "bearer "+goodKey) },
			wantStatus: 200,
		},
		{
			name:       "missing header",
			setHeader:  func(r *http.Request) {},
			wantStatus: 401,
			wantCode:   "missing_api_key",
		},
		{
			name:       "wrong key",
			setHeader:  func(r *http.Request) { r.Header.Set("X-Api-Key", "not-the-key") },
			wantStatus: 401,
			wantCode:   "invalid_api_key",
		},
		{
			name:       "malformed Bearer (no space)",
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "Bearer"+goodKey) },
			wantStatus: 401,
			wantCode:   "missing_api_key",
		},
		{
			name:       "Basic auth (wrong scheme)",
			setHeader:  func(r *http.Request) { r.Header.Set("Authorization", "Basic "+goodKey) },
			wantStatus: 401,
			wantCode:   "missing_api_key",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, _ := http.NewRequest("GET", ts.URL+"/healthz", nil)
			tc.setHeader(req)

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			_, _ = io.ReadAll(resp.Body)

			// /healthz bypasses auth. Verify by asking a protected route instead.
			req2, _ := http.NewRequest("POST", ts.URL+"/v1/messages", strings.NewReader("{}"))
			req2.Header.Set("Content-Type", "application/json")
			tc.setHeader(req2)

			resp2, err := http.DefaultClient.Do(req2)
			if err != nil {
				t.Fatal(err)
			}
			defer resp2.Body.Close()
			body, _ := io.ReadAll(resp2.Body)

			switch tc.wantStatus {
			case 200:
				if resp2.StatusCode == 401 {
					t.Fatalf("expected auth to pass; got 401 body=%s", body)
				}
			case 401:
				if resp2.StatusCode != 401 {
					t.Fatalf("want 401, got %d body=%s", resp2.StatusCode, body)
				}
				if ct := resp2.Header.Get("Content-Type"); ct != "application/problem+json" {
					t.Errorf("want problem+json, got %q", ct)
				}
				if !strings.Contains(string(body), tc.wantCode) {
					t.Errorf("want code %q in body, got %s", tc.wantCode, body)
				}
			}
		})
	}
}

func TestM6_HealthzBypassesAuth(t *testing.T) {
	srv := New(Options{APIKey: "sk-required"})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("healthz should bypass auth; got %d body=%s", resp.StatusCode, body)
	}
}

func TestM6_NoKeyConfiguredMeansOpen(t *testing.T) {
	srv := New(Options{APIKey: ""})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL+"/v1/messages", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 401 {
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "missing_api_key") || strings.Contains(string(body), "invalid_api_key") {
			t.Fatalf("auth middleware should be disabled when KIROXY_API_KEY empty; got %d body=%s", resp.StatusCode, body)
		}
	}
}
