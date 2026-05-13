package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestModels_ReturnsCanonicalTable(t *testing.T) {
	s := New(Options{Version: "test"})
	ts := httptest.NewServer(s.Handler())
	defer ts.Close()

	res, err := http.Get(ts.URL + "/dashboard/api/models")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("want 200, got %d", res.StatusCode)
	}
	var payload ModelsResponse
	if err := json.NewDecoder(res.Body).Decode(&payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(payload.Models) == 0 {
		t.Fatalf("want non-empty models table")
	}
	if payload.DefaultModel == "" {
		t.Fatalf("want non-empty default_model")
	}

	var sawSonnet, sawOpus, sawHaiku bool
	for _, m := range payload.Models {
		if m.Family == "sonnet" {
			sawSonnet = true
		}
		if m.Family == "opus" {
			sawOpus = true
		}
		if m.Family == "haiku" {
			sawHaiku = true
		}
		if m.Anthropic == "" || m.Kiro == "" {
			t.Fatalf("empty fields on model row: %+v", m)
		}
		if m.ContextWindowSize <= 0 {
			t.Fatalf("non-positive context window: %+v", m)
		}
	}
	if !sawSonnet || !sawOpus || !sawHaiku {
		t.Fatalf("missing families: sonnet=%v opus=%v haiku=%v", sawSonnet, sawOpus, sawHaiku)
	}
}

func TestBuildModelTable_TierAssignment(t *testing.T) {
	entries := BuildModelTable()
	for _, e := range entries {
		switch e.Family {
		case "opus":
			if e.Tier != "pro" {
				t.Errorf("opus %q should be pro, got %q", e.Anthropic, e.Tier)
			}
		case "sonnet", "haiku":
			if e.Tier != "free" {
				t.Errorf("%s %q should be free, got %q", e.Family, e.Anthropic, e.Tier)
			}
		}
	}
}

func TestBuildModelTable_ThinkingVariants(t *testing.T) {
	entries := BuildModelTable()
	var foundThinking bool
	for _, e := range entries {
		if e.IsThinking {
			foundThinking = true
			if e.ContextWindowSize < 1_000_000 {
				t.Errorf("thinking variant %q should have 1M context, got %d", e.Anthropic, e.ContextWindowSize)
			}
		}
	}
	if !foundThinking {
		t.Fatalf("expected at least one thinking/1M variant in table")
	}
}
