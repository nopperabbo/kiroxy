// Package config centralises the process-wide configuration for kiroxy.
// Values flow from environment variables with optional CLI override.
package config

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Config is the validated process configuration.
type Config struct {
	// Bind is the interface address the HTTP server listens on. Default 127.0.0.1.
	Bind string

	// Port is the TCP port. Default 8787.
	Port int

	// APIKey is the static inbound credential (KIROXY_API_KEY). Empty means
	// authentication is not configured yet; M6 will enforce that requests fail
	// when APIKey == "".
	APIKey string

	// DBPath is the absolute path to the SQLite token vault. Default
	// ~/.kiroxy/tokens.db.
	DBPath string

	// LogLevelRaw is the raw string from env (debug|info|warn|error).
	LogLevelRaw string

	// ShutdownTimeout is the deadline for http.Server.Shutdown. Default 30s.
	ShutdownTimeout time.Duration

	// KiroRegion is the AWS region for Kiro upstream. Default us-east-1.
	KiroRegion string
}

// LogLevel returns the slog.Level parsed from LogLevelRaw.
func (c Config) LogLevel() slog.Level {
	switch strings.ToLower(c.LogLevelRaw) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FromEnvAndFlags reads KIROXY_* environment variables and applies optional
// flag overrides. Flag list (all optional):
//
//	-port N        override KIROXY_PORT
//	-bind HOST     override KIROXY_BIND
func FromEnvAndFlags(args []string) (Config, error) {
	cfg := Config{
		Bind:            envOr("KIROXY_BIND", "127.0.0.1"),
		APIKey:          os.Getenv("KIROXY_API_KEY"),
		LogLevelRaw:     envOr("KIROXY_LOG_LEVEL", "info"),
		KiroRegion:      envOr("KIROXY_KIRO_REGION", "us-east-1"),
		ShutdownTimeout: 30 * time.Second,
	}

	// Port from env with default
	if p, err := atoiWithDefault(os.Getenv("KIROXY_PORT"), 8787); err != nil {
		return cfg, fmt.Errorf("KIROXY_PORT: %w", err)
	} else {
		cfg.Port = p
	}

	// DBPath default
	if p := os.Getenv("KIROXY_DB_PATH"); p != "" {
		cfg.DBPath = p
	} else {
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg, fmt.Errorf("home dir lookup: %w", err)
		}
		cfg.DBPath = filepath.Join(home, ".kiroxy", "tokens.db")
	}

	// Shutdown timeout
	if s := os.Getenv("KIROXY_SHUTDOWN_TIMEOUT"); s != "" {
		n, err := atoiWithDefault(s, 30)
		if err != nil {
			return cfg, fmt.Errorf("KIROXY_SHUTDOWN_TIMEOUT: %w", err)
		}
		cfg.ShutdownTimeout = time.Duration(n) * time.Second
	}

	// Flag overrides (thin layer)
	fs := flag.NewFlagSet("serve", flag.ContinueOnError)
	fs.IntVar(&cfg.Port, "port", cfg.Port, "listen port (overrides KIROXY_PORT)")
	fs.StringVar(&cfg.Bind, "bind", cfg.Bind, "bind address (overrides KIROXY_BIND)")
	// Route help text through fs.Output so we don't print anything when called
	// from tests.
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return cfg, err
	}

	if cfg.Port < 1 || cfg.Port > 65535 {
		return cfg, fmt.Errorf("invalid port %d", cfg.Port)
	}

	return cfg, nil
}

func envOr(name, fallback string) string {
	if v := os.Getenv(name); v != "" {
		return v
	}
	return fallback
}

func atoiWithDefault(s string, fallback int) (int, error) {
	if s == "" {
		return fallback, nil
	}
	var n int
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("%q is not a number", s)
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}
