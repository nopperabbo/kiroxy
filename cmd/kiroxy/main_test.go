package main

import (
	"context"
	"testing"
	"time"
)

func TestStartsWithDash(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"-v", true},
		{"--help", true},
		{"serve", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := startsWithDash(tt.input); got != tt.want {
			t.Errorf("startsWithDash(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestRun_Version(t *testing.T) {
	err := run(context.Background(), []string{"--version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_VersionSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"version"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_Help(t *testing.T) {
	err := run(context.Background(), []string{"--help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_HelpSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"help"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_UnknownSubcommand(t *testing.T) {
	err := run(context.Background(), []string{"bogus-cmd"})
	if err == nil {
		t.Fatal("expected error for unknown subcommand")
	}
}

// Tests for import.go helpers (redact, truncate, parentDir)

func TestRedact(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ""},
		{"short", "*****"},
		{"12345678", "********"},
		{"abcdefghijklm", "abcd...jklm"},
	}
	for _, tt := range tests {
		if got := redact(tt.input); got != tt.want {
			t.Errorf("redact(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input string
		n     int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello", 5, "hello"},
		{"hello world", 5, "hello..."},
		{"", 5, ""},
	}
	for _, tt := range tests {
		if got := truncate(tt.input, tt.n); got != tt.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.n, got, tt.want)
		}
	}
}

func TestParentDir(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"/foo/bar/baz", "/foo/bar"},
		{"/foo", "."},
		{"foo", "."},
		{"", "."},
	}
	for _, tt := range tests {
		if got := parentDir(tt.input); got != tt.want {
			t.Errorf("parentDir(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestRun_DashV(t *testing.T) {
	err := run(context.Background(), []string{"-v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRun_EmptyArgs_DefaultsToServe(t *testing.T) {
	t.Setenv("KIROXY_DB_PATH", "/nonexistent/path/vault.db")
	t.Setenv("KIROXY_KIRO_DB_PATH", "")
	err := run(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error when vault path is invalid")
	}
}

func TestRun_ServeWithTempVault(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("KIROXY_DB_PATH", dir+"/vault.db")
	t.Setenv("KIROXY_KIRO_DB_PATH", "")
	t.Setenv("KIROXY_PORT", "19876")
	t.Setenv("KIROXY_BIND", "127.0.0.1")
	t.Setenv("KIROXY_API_KEY", "test-key-1234567890")
	t.Setenv("KIROXY_OTEL_ENABLED", "")
	t.Setenv("KIROXY_USAGE_POLL_DISABLED", "1")

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	_ = run(ctx, []string{"serve"})
}
