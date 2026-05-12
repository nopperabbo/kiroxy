package openai

import (
	"testing"
)

func TestListModels_IncludesKiroAndAliases(t *testing.T) {
	list := ListModels()
	if list.Object != ObjectList {
		t.Errorf("envelope object: %q", list.Object)
	}
	if len(list.Data) == 0 {
		t.Fatal("empty model list")
	}
	seen := make(map[string]struct{}, len(list.Data))
	for _, m := range list.Data {
		if m.Object != ObjectModel {
			t.Errorf("entry object: %+v", m)
		}
		if m.OwnedBy != "kiroxy" {
			t.Errorf("owned_by: %+v", m)
		}
		if m.Created == 0 {
			t.Errorf("created ts unset: %+v", m)
		}
		if m.ID == "" {
			t.Errorf("empty id: %+v", m)
		}
		seen[m.ID] = struct{}{}
	}
	// Spot-check one known Kiro model and two aliases.
	for _, want := range []string{"claude-sonnet-4.6", "gpt-4o", "gpt-3.5-turbo"} {
		if _, ok := seen[want]; !ok {
			t.Errorf("%q missing from /v1/models list", want)
		}
	}
}

func TestListModels_SortedStable(t *testing.T) {
	a := ListModels()
	b := ListModels()
	if len(a.Data) != len(b.Data) {
		t.Fatal("non-stable length")
	}
	for i := range a.Data {
		if a.Data[i].ID != b.Data[i].ID {
			t.Errorf("order not stable at %d: %q vs %q", i, a.Data[i].ID, b.Data[i].ID)
		}
	}
}

func TestListModels_Deduplicates(t *testing.T) {
	list := ListModels()
	seen := make(map[string]int)
	for _, m := range list.Data {
		seen[m.ID]++
	}
	for id, n := range seen {
		if n > 1 {
			t.Errorf("%q appears %d times", id, n)
		}
	}
}
