package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"local/kiroxy/internal/pool"
	"local/kiroxy/internal/server"
	"local/kiroxy/internal/tokenvault"
)

type settingsProvider struct {
	version   string
	vaultPath string
	logLevel  *slog.LevelVar
	vault     *tokenvault.Vault
	pool      *pool.Pool
	startedAt time.Time
}

func (s *settingsProvider) Settings(ctx context.Context) server.SettingsSnapshot {
	snap := server.SettingsSnapshot{
		General: server.VaultGeneral{
			Version:   s.version,
			UptimeS:   int64(time.Since(s.startedAt).Seconds()),
			StartedAt: s.startedAt.UTC().Format(time.RFC3339),
			VaultPath: s.vaultPath,
			LogLevel:  s.currentLogLevelString(),
		},
		EnvVars: server.BuildEnvVars(),
	}
	if s.vault != nil && s.vaultPath != "" {
		if info, err := os.Stat(s.vaultPath); err == nil {
			snap.Vault.Path = s.vaultPath
			snap.Vault.SizeBytes = info.Size()
		}
		if active, total, err := s.vault.CountInboundKeys(ctx); err == nil {
			snap.Inbound = server.InboundKeyStats{Active: active, Total: total}
		}
	}
	if s.pool != nil {
		now := time.Now()
		for _, a := range s.pool.List() {
			snap.Vault.Total++
			switch {
			case !a.Enabled:
				snap.Vault.Disabled++
			case !a.CooldownUntil.IsZero() && a.CooldownUntil.After(now):
				snap.Vault.Cooldown++
			default:
				snap.Vault.Healthy++
			}
		}
	}
	return snap
}

func (s *settingsProvider) currentLogLevelString() string {
	if s.logLevel == nil {
		return ""
	}
	return strings.ToLower(s.logLevel.Level().String())
}

func (s *settingsProvider) UpdateLogLevel(_ context.Context, level string) error {
	if s.logLevel == nil {
		return fmt.Errorf("log level not mutable in this build")
	}
	parsed, err := parseSlogLevel(level)
	if err != nil {
		return err
	}
	s.logLevel.Set(parsed)
	slog.Info("log level updated via dashboard",
		slog.String("level", strings.ToLower(parsed.String())))
	return nil
}

func parseSlogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("invalid log level %q (expected debug/info/warn/error)", s)
	}
}
