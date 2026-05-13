package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubInboundProvider struct {
	listCalls   int
	keys        []InboundKeyView
	created     *CreatedInboundKey
	createErr   error
	revokeErr   error
	revokedID   string
	listErr     error
	createLabel string
}

func (s *stubInboundProvider) List(ctx context.Context) ([]InboundKeyView, error) {
	s.listCalls++
	return s.keys, s.listErr
}
func (s *stubInboundProvider) Create(ctx context.Context, label string) (*CreatedInboundKey, error) {
	s.createLabel = label
	return s.created, s.createErr
}
func (s *stubInboundProvider) Revoke(ctx context.Context, id string) error {
	s.revokedID = id
	return s.revokeErr
}

func TestInboundKeys_DisabledReturns404(t *testing.T) {
	s := New(Options{Version: "test"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/inbound-keys")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 when provider nil, got %d", res.StatusCode)
	}
}

func TestInboundKeys_List(t *testing.T) {
	stub := &stubInboundProvider{
		keys: []InboundKeyView{{ID: "abc", Label: "ci", Tail: "xyz1", CreatedAt: "2026-05-13T00:00:00Z"}},
	}
	s := New(Options{InboundKeyProvider: stub})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/inbound-keys")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var payload struct {
		Keys []InboundKeyView `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Keys) != 1 || payload.Keys[0].ID != "abc" {
		t.Fatalf("want 1 key with id=abc, got %+v", payload.Keys)
	}
}

func TestInboundKeys_Create(t *testing.T) {
	stub := &stubInboundProvider{
		created: &CreatedInboundKey{
			InboundKeyView: InboundKeyView{ID: "newid", Label: "deploy", Tail: "abcd"},
			Plaintext:      "kxy_plaintextsecret",
		},
	}
	s := New(Options{InboundKeyProvider: stub})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	body := bytes.NewReader([]byte(`{"label":"deploy"}`))
	res, err := http.Post(ts.URL+"/dashboard/api/inbound-keys", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusCreated {
		t.Fatalf("want 201, got %d", res.StatusCode)
	}
	if stub.createLabel != "deploy" {
		t.Fatalf("want label=deploy, got %q", stub.createLabel)
	}
	var got CreatedInboundKey
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Plaintext != "kxy_plaintextsecret" {
		t.Fatalf("want plaintext echoed, got %q", got.Plaintext)
	}
}

func TestInboundKeys_CreateLabelTooLong(t *testing.T) {
	stub := &stubInboundProvider{}
	s := New(Options{InboundKeyProvider: stub})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	longLabel := string(bytes.Repeat([]byte("a"), 200))
	body := bytes.NewReader([]byte(`{"label":"` + longLabel + `"}`))
	res, err := http.Post(ts.URL+"/dashboard/api/inbound-keys", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 400 {
		t.Fatalf("want 400, got %d", res.StatusCode)
	}
}

func TestInboundKeys_Revoke(t *testing.T) {
	stub := &stubInboundProvider{}
	s := New(Options{InboundKeyProvider: stub})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("DELETE", ts.URL+"/dashboard/api/inbound-keys/abc123", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNoContent {
		t.Fatalf("want 204, got %d", res.StatusCode)
	}
	if stub.revokedID != "abc123" {
		t.Fatalf("want id=abc123 revoked, got %q", stub.revokedID)
	}
}

func TestInboundKeys_RevokeNotFound(t *testing.T) {
	stub := &stubInboundProvider{revokeErr: errors.New("tokenvault: inbound key not found")}
	s := New(Options{InboundKeyProvider: stub})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	req, _ := http.NewRequest("DELETE", ts.URL+"/dashboard/api/inbound-keys/nope", nil)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("DELETE: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res.StatusCode)
	}
}

type stubSettingsProvider struct {
	snap SettingsSnapshot
}

func (s *stubSettingsProvider) Settings(ctx context.Context) SettingsSnapshot { return s.snap }

func TestSettings_Snapshot(t *testing.T) {
	stub := &stubSettingsProvider{snap: SettingsSnapshot{
		General: VaultGeneral{Version: "1.1.0"},
		Vault:   VaultStats{Healthy: 3, Total: 3},
		Inbound: InboundKeyStats{Active: 1, Total: 2},
	}}
	s := New(Options{SettingsProvider: stub, Version: "test"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/settings")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var payload SettingsSnapshot
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.General.Version != "1.1.0" {
		t.Fatalf("want version=1.1.0, got %q", payload.General.Version)
	}
	if payload.Vault.Total != 3 {
		t.Fatalf("want total=3, got %d", payload.Vault.Total)
	}
	if payload.Inbound.Active != 1 {
		t.Fatalf("want active=1, got %d", payload.Inbound.Active)
	}
	if len(payload.EnvVars) == 0 {
		t.Fatalf("expected auto-populated env vars")
	}
}

func TestBuildEnvVars_RedactsSecrets(t *testing.T) {
	t.Setenv("KIROXY_API_KEY", "supersecret123")
	t.Setenv("KIROXY_BIND", "127.0.0.1")

	vars := BuildEnvVars()
	var sawKey, sawBind bool
	for _, v := range vars {
		if v.Key == "KIROXY_API_KEY" {
			sawKey = true
			if !v.Redacted || v.Value == "supersecret123" {
				t.Fatalf("API_KEY must be redacted, got %+v", v)
			}
			if v.Value != "****t123" {
				t.Fatalf("expected ****t123, got %q", v.Value)
			}
		}
		if v.Key == "KIROXY_BIND" {
			sawBind = true
			if v.Redacted || v.Value != "127.0.0.1" {
				t.Fatalf("BIND must NOT be redacted, got %+v", v)
			}
		}
	}
	if !sawKey || !sawBind {
		t.Fatalf("missing expected vars: key=%v bind=%v", sawKey, sawBind)
	}
}

func TestSettings_DisabledReturns404(t *testing.T) {
	s := New(Options{})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/settings")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404, got %d", res.StatusCode)
	}
}
