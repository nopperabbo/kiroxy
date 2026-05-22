package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestSelectModels_EmptyFilter_ReturnsAll(t *testing.T) {
	all := []knownModel{
		{ID: "a", DisplayName: "A"},
		{ID: "b", DisplayName: "B"},
	}
	got, unknown, err := selectModels(all, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d", len(got))
	}
	if len(unknown) != 0 {
		t.Fatalf("want no unknowns, got %v", unknown)
	}
}

func TestSelectModels_FilterSubset(t *testing.T) {
	all := []knownModel{
		{ID: "a", DisplayName: "A"},
		{ID: "b", DisplayName: "B"},
		{ID: "c", DisplayName: "C"},
	}
	got, unknown, err := selectModels(all, "a,c")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "c" {
		t.Fatalf("unexpected selection: %+v", got)
	}
	if len(unknown) != 0 {
		t.Fatalf("want no unknowns, got %v", unknown)
	}
}

func TestSelectModels_UnknownEntries(t *testing.T) {
	all := []knownModel{{ID: "a", DisplayName: "A"}}
	got, unknown, err := selectModels(all, "a,bogus")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("want 1 selected, got %d", len(got))
	}
	if len(unknown) != 1 || unknown[0] != "bogus" {
		t.Fatalf("want [bogus], got %v", unknown)
	}
}

func TestSelectModels_AllUnknown_ReturnsEmpty(t *testing.T) {
	all := []knownModel{{ID: "a", DisplayName: "A"}}
	got, _, err := selectModels(all, "x,y")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("want 0 selected, got %d", len(got))
	}
}

func TestSelectModels_EmptyTokens_Error(t *testing.T) {
	all := []knownModel{{ID: "a", DisplayName: "A"}}
	_, _, err := selectModels(all, "  , , ")
	if err == nil {
		t.Fatal("expected error for empty tokens")
	}
}

func TestJoinIDs(t *testing.T) {
	ms := []knownModel{
		{ID: "x"},
		{ID: "y"},
		{ID: "z"},
	}
	got := joinIDs(ms)
	if got != "x, y, z" {
		t.Fatalf("want 'x, y, z', got %q", got)
	}
}

func TestJoinIDs_Empty(t *testing.T) {
	got := joinIDs(nil)
	if got != "" {
		t.Fatalf("want empty, got %q", got)
	}
}

func TestRunOpencodeConfigTo_DefaultOutput(t *testing.T) {
	t.Setenv("KIROXY_INBOUND_KEY", "")
	var stdout, stderr bytes.Buffer
	err := runOpencodeConfigTo(nil, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}

	// stdout should be valid JSON
	var snippet opencodeSnippet
	if err := json.Unmarshal(stdout.Bytes(), &snippet); err != nil {
		t.Fatalf("stdout not valid JSON: %v\n%s", err, stdout.String())
	}

	// Should have the default provider "kiroxy"
	entry, ok := snippet.Provider["kiroxy"]
	if !ok {
		t.Fatal("missing 'kiroxy' provider key")
	}
	if entry.NPM != "@ai-sdk/anthropic" {
		t.Errorf("npm = %q, want @ai-sdk/anthropic", entry.NPM)
	}
	if entry.Options.BaseURL != "http://localhost:8787" {
		t.Errorf("baseURL = %q", entry.Options.BaseURL)
	}
	// Default key is "changeme" when no env/flag
	if entry.Options.APIKey != "changeme" {
		t.Errorf("apiKey = %q, want changeme", entry.Options.APIKey)
	}
	// Should have all known models
	if len(entry.Models) != len(knownModels) {
		t.Errorf("models count = %d, want %d", len(entry.Models), len(knownModels))
	}

	// stderr should contain guidance
	if !strings.Contains(stderr.String(), "Paste the above") {
		t.Error("stderr missing guidance text")
	}
}

func TestRunOpencodeConfigTo_CustomFlags(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{
		"-base-url", "http://myhost:9999",
		"-api-key", "sk-test-key",
		"-provider-name", "myproxy",
		"-models", "claude-opus-4-7,claude-haiku-4.5",
	}
	err := runOpencodeConfigTo(args, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}

	var snippet opencodeSnippet
	if err := json.Unmarshal(stdout.Bytes(), &snippet); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	entry, ok := snippet.Provider["myproxy"]
	if !ok {
		t.Fatal("missing 'myproxy' provider key")
	}
	if entry.Options.BaseURL != "http://myhost:9999" {
		t.Errorf("baseURL = %q", entry.Options.BaseURL)
	}
	if entry.Options.APIKey != "sk-test-key" {
		t.Errorf("apiKey = %q", entry.Options.APIKey)
	}
	if len(entry.Models) != 2 {
		t.Errorf("models count = %d, want 2", len(entry.Models))
	}
}

func TestRunOpencodeConfigTo_UnknownModelWarning(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"-models", "claude-opus-4-7,fake-model"}
	err := runOpencodeConfigTo(args, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "fake-model") {
		t.Error("stderr should warn about unknown model")
	}
}

func TestRunOpencodeConfigTo_AllModelsUnknown_Error(t *testing.T) {
	var stdout, stderr bytes.Buffer
	args := []string{"-models", "nonexistent"}
	err := runOpencodeConfigTo(args, &stdout, &stderr)
	if err == nil {
		t.Fatal("expected error when no models match")
	}
}

func TestRunOpencodeConfigTo_OutputFile(t *testing.T) {
	dir := t.TempDir()
	outPath := dir + "/out.json"
	var stdout, stderr bytes.Buffer
	args := []string{"-output", outPath}
	err := runOpencodeConfigTo(args, &stdout, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	// stdout should be empty (written to file)
	if stdout.Len() != 0 {
		t.Errorf("stdout should be empty when -output is set, got %d bytes", stdout.Len())
	}
	if !strings.Contains(stderr.String(), "wrote") {
		t.Error("stderr should confirm file write")
	}
}
