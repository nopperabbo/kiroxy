package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestKiroUpstreamURL_EnvOverride verifies that KIROXY_UPSTREAM_URL is
// picked up by FromEnvAndFlags and surfaced on Config.KiroUpstreamURL.
// Downstream, main.go wires this into kiroclient.WithBaseURL.
func TestKiroUpstreamURL_EnvOverride(t *testing.T) {
	tests := []struct {
		name string
		set  string
		want string
	}{
		{"unset defaults to empty", "", ""},
		{"explicit override", "http://127.0.0.1:9999/", "http://127.0.0.1:9999/"},
		{"mock_kiro URL", "http://localhost:4000", "http://localhost:4000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearKiroxyEnv(t)
			if tc.set == "" {
				t.Setenv("KIROXY_UPSTREAM_URL", "")
			} else {
				t.Setenv("KIROXY_UPSTREAM_URL", tc.set)
			}
			cfg, err := FromEnvAndFlags(nil)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.KiroUpstreamURL != tc.want {
				t.Errorf("KiroUpstreamURL = %q, want %q", cfg.KiroUpstreamURL, tc.want)
			}
		})
	}
}

// clearKiroxyEnv resets every KIROXY_* variable for hermetic test runs.
// t.Setenv is per-test scope so leakage between subtests is contained, but
// the parent test's environment can still poison defaults if the developer
// runs tests with an .envrc file loaded.
func clearKiroxyEnv(t *testing.T) {
	t.Helper()
	for _, v := range []string{
		"KIROXY_BIND",
		"KIROXY_PORT",
		"KIROXY_API_KEY",
		"KIROXY_DB_PATH",
		"KIROXY_LOG_LEVEL",
		"KIROXY_KIRO_REGION",
		"KIROXY_KIRO_DB_PATH",
		"KIROXY_UPSTREAM_URL",
		"KIROXY_SHUTDOWN_TIMEOUT",
	} {
		t.Setenv(v, "")
	}
}

func TestFromEnvAndFlags_Defaults(t *testing.T) {
	clearKiroxyEnv(t)
	cfg, err := FromEnvAndFlags(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Bind != "127.0.0.1" {
		t.Errorf("Bind default: got %q, want 127.0.0.1", cfg.Bind)
	}
	if cfg.Port != 8787 {
		t.Errorf("Port default: got %d, want 8787", cfg.Port)
	}
	if cfg.APIKey != "" {
		t.Errorf("APIKey default: got %q, want empty", cfg.APIKey)
	}
	if cfg.LogLevelRaw != "info" {
		t.Errorf("LogLevelRaw default: got %q, want info", cfg.LogLevelRaw)
	}
	if cfg.KiroRegion != "us-east-1" {
		t.Errorf("KiroRegion default: got %q, want us-east-1", cfg.KiroRegion)
	}
	if cfg.ShutdownTimeout != 30*time.Second {
		t.Errorf("ShutdownTimeout default: got %v, want 30s", cfg.ShutdownTimeout)
	}
	home, _ := os.UserHomeDir()
	wantDBPath := filepath.Join(home, ".kiroxy", "tokens.db")
	if cfg.DBPath != wantDBPath {
		t.Errorf("DBPath default: got %q, want %q", cfg.DBPath, wantDBPath)
	}
}

// TestFromEnvAndFlags_EnvOverrides exercises every KIROXY_* env that
// FromEnvAndFlags consumes and asserts the resulting Config field. One
// table-driven test per env keeps a regression localized.
func TestFromEnvAndFlags_EnvOverrides(t *testing.T) {
	tests := []struct {
		name   string
		env    string
		value  string
		check  func(t *testing.T, cfg Config)
		expErr bool
	}{
		{
			name: "BIND override",
			env:  "KIROXY_BIND", value: "0.0.0.0",
			check: func(t *testing.T, cfg Config) {
				if cfg.Bind != "0.0.0.0" {
					t.Errorf("Bind = %q, want 0.0.0.0", cfg.Bind)
				}
			},
		},
		{
			name: "PORT valid override",
			env:  "KIROXY_PORT", value: "9000",
			check: func(t *testing.T, cfg Config) {
				if cfg.Port != 9000 {
					t.Errorf("Port = %d, want 9000", cfg.Port)
				}
			},
		},
		{
			name: "PORT invalid (non-digit)", env: "KIROXY_PORT", value: "abc", expErr: true,
		},
		{
			name: "PORT zero is rejected", env: "KIROXY_PORT", value: "0", expErr: true,
		},
		{
			name: "PORT >65535 rejected", env: "KIROXY_PORT", value: "70000", expErr: true,
		},
		{
			name: "API_KEY override",
			env:  "KIROXY_API_KEY", value: "secret123",
			check: func(t *testing.T, cfg Config) {
				if cfg.APIKey != "secret123" {
					t.Errorf("APIKey = %q, want secret123", cfg.APIKey)
				}
			},
		},
		{
			name: "DB_PATH override",
			env:  "KIROXY_DB_PATH", value: "/tmp/custom.db",
			check: func(t *testing.T, cfg Config) {
				if cfg.DBPath != "/tmp/custom.db" {
					t.Errorf("DBPath = %q, want /tmp/custom.db", cfg.DBPath)
				}
			},
		},
		{
			name: "LOG_LEVEL override",
			env:  "KIROXY_LOG_LEVEL", value: "debug",
			check: func(t *testing.T, cfg Config) {
				if cfg.LogLevelRaw != "debug" {
					t.Errorf("LogLevelRaw = %q, want debug", cfg.LogLevelRaw)
				}
			},
		},
		{
			name: "KIRO_REGION override",
			env:  "KIROXY_KIRO_REGION", value: "eu-central-1",
			check: func(t *testing.T, cfg Config) {
				if cfg.KiroRegion != "eu-central-1" {
					t.Errorf("KiroRegion = %q, want eu-central-1", cfg.KiroRegion)
				}
			},
		},
		{
			name: "KIRO_DB_PATH override",
			env:  "KIROXY_KIRO_DB_PATH", value: "/path/to/data.sqlite3",
			check: func(t *testing.T, cfg Config) {
				if cfg.KiroDBPath != "/path/to/data.sqlite3" {
					t.Errorf("KiroDBPath = %q, want /path/to/data.sqlite3", cfg.KiroDBPath)
				}
			},
		},
		{
			name: "SHUTDOWN_TIMEOUT override",
			env:  "KIROXY_SHUTDOWN_TIMEOUT", value: "60",
			check: func(t *testing.T, cfg Config) {
				if cfg.ShutdownTimeout != 60*time.Second {
					t.Errorf("ShutdownTimeout = %v, want 60s", cfg.ShutdownTimeout)
				}
			},
		},
		{
			name: "SHUTDOWN_TIMEOUT invalid", env: "KIROXY_SHUTDOWN_TIMEOUT", value: "5min", expErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			clearKiroxyEnv(t)
			t.Setenv(tc.env, tc.value)
			cfg, err := FromEnvAndFlags(nil)
			if tc.expErr {
				if err == nil {
					t.Errorf("expected error for %s=%q, got nil", tc.env, tc.value)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error for %s=%q: %v", tc.env, tc.value, err)
			}
			if tc.check != nil {
				tc.check(t, cfg)
			}
		})
	}
}

// TestFromEnvAndFlags_FlagOverrides confirms -port and -bind win over env.
func TestFromEnvAndFlags_FlagOverrides(t *testing.T) {
	clearKiroxyEnv(t)
	t.Setenv("KIROXY_PORT", "8787")
	t.Setenv("KIROXY_BIND", "127.0.0.1")

	cfg, err := FromEnvAndFlags([]string{"-port", "9999", "-bind", "0.0.0.0"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Port != 9999 {
		t.Errorf("Port flag override: got %d, want 9999", cfg.Port)
	}
	if cfg.Bind != "0.0.0.0" {
		t.Errorf("Bind flag override: got %q, want 0.0.0.0", cfg.Bind)
	}
}

// TestFromEnvAndFlags_FlagInvalidPort hits the post-flag port validation.
func TestFromEnvAndFlags_FlagInvalidPort(t *testing.T) {
	clearKiroxyEnv(t)
	tests := []struct {
		name string
		args []string
	}{
		{"port zero", []string{"-port", "0"}},
		{"port too high", []string{"-port", "65536"}},
		{"port negative", []string{"-port", "-1"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := FromEnvAndFlags(tc.args)
			if err == nil {
				t.Errorf("expected error for %v, got nil", tc.args)
			}
		})
	}
}

// TestFromEnvAndFlags_FlagParseError covers malformed CLI arguments.
func TestFromEnvAndFlags_FlagParseError(t *testing.T) {
	clearKiroxyEnv(t)
	_, err := FromEnvAndFlags([]string{"-unknown-flag"})
	if err == nil {
		t.Error("expected parse error for unknown flag")
	}
}

func TestConfig_LogLevel(t *testing.T) {
	tests := []struct {
		raw  string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"DEBUG", slog.LevelDebug},
		{"Debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"warning", slog.LevelWarn},
		{"error", slog.LevelError},
		{"", slog.LevelInfo},
		{"unknown", slog.LevelInfo},
	}
	for _, tc := range tests {
		t.Run(tc.raw, func(t *testing.T) {
			c := Config{LogLevelRaw: tc.raw}
			if got := c.LogLevel(); got != tc.want {
				t.Errorf("LogLevel(%q) = %v, want %v", tc.raw, got, tc.want)
			}
		})
	}
}

func TestEnvOr(t *testing.T) {
	t.Setenv("KIROXY_TEST_ENVOR", "")
	if got := envOr("KIROXY_TEST_ENVOR", "fallback"); got != "fallback" {
		t.Errorf("envOr empty: got %q, want fallback", got)
	}
	t.Setenv("KIROXY_TEST_ENVOR", "value")
	if got := envOr("KIROXY_TEST_ENVOR", "fallback"); got != "value" {
		t.Errorf("envOr set: got %q, want value", got)
	}
}

// TestAtoiWithDefault locks the parser's behavior. The custom impl rejects
// negative numbers, leading whitespace, and the '+' prefix that strconv.Atoi
// would accept; tests pin those choices so a future refactor doesn't
// silently widen the contract.
func TestAtoiWithDefault(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		fallback int
		want     int
		wantErr  bool
	}{
		{"empty uses fallback", "", 42, 42, false},
		{"valid digit", "100", 0, 100, false},
		{"zero", "0", 99, 0, false},
		{"large number", "65535", 0, 65535, false},
		{"leading zero accepted", "0042", 0, 42, false},
		{"non-digit rejected", "abc", 0, 0, true},
		{"negative rejected", "-1", 0, 0, true},
		{"plus prefix rejected", "+10", 0, 0, true},
		{"trailing space rejected", "10 ", 0, 0, true},
		{"hex form rejected", "0x10", 0, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := atoiWithDefault(tc.input, tc.fallback)
			if tc.wantErr {
				if err == nil {
					t.Errorf("input=%q: expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("input=%q: unexpected error %v", tc.input, err)
			}
			if got != tc.want {
				t.Errorf("input=%q: got %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// TestFromEnvAndFlags_ErrorMessageContext verifies that error messages
// include the env var name so operators can locate misconfigured fields
// without grepping the source.
func TestFromEnvAndFlags_ErrorMessageContext(t *testing.T) {
	clearKiroxyEnv(t)
	t.Setenv("KIROXY_PORT", "abc")
	_, err := FromEnvAndFlags(nil)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "KIROXY_PORT") {
		t.Errorf("error %q should mention KIROXY_PORT", err.Error())
	}
}
