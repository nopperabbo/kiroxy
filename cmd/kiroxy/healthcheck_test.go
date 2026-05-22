package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDefaultHealthcheckURL_DefaultPort(t *testing.T) {
	t.Setenv("KIROXY_PORT", "")
	got := defaultHealthcheckURL()
	if got != "http://127.0.0.1:8787/healthz" {
		t.Errorf("got %q", got)
	}
}

func TestDefaultHealthcheckURL_CustomPort(t *testing.T) {
	t.Setenv("KIROXY_PORT", "9999")
	got := defaultHealthcheckURL()
	if got != "http://127.0.0.1:9999/healthz" {
		t.Errorf("got %q", got)
	}
}

func TestRunHealthcheck_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	}))
	defer srv.Close()

	err := runHealthcheck(context.Background(), []string{"--url", srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunHealthcheck_Non200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte("not ready"))
	}))
	defer srv.Close()

	err := runHealthcheck(context.Background(), []string{"--url", srv.URL})
	if err == nil {
		t.Fatal("expected error for non-200")
	}
}

func TestRunHealthcheck_BadStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "degraded"})
	}))
	defer srv.Close()

	err := runHealthcheck(context.Background(), []string{"--url", srv.URL})
	if err == nil {
		t.Fatal("expected error for status != ok")
	}
}

func TestRunHealthcheck_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer srv.Close()

	err := runHealthcheck(context.Background(), []string{"--url", srv.URL})
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestRunHealthcheck_Unreachable(t *testing.T) {
	err := runHealthcheck(context.Background(), []string{"--url", "http://127.0.0.1:1/healthz", "--timeout", "1"})
	if err == nil {
		t.Fatal("expected error for unreachable host")
	}
}
