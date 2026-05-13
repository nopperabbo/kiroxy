package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"local/kiroxy/internal/pool"
	"local/kiroxy/internal/server"
	"local/kiroxy/internal/tokenvault"
)

// settingsProvider populates the bundled /dashboard/api/settings response
// from the same vault + pool the dashboard reads, plus runtime info.
type settingsProvider struct {
	version   string
	vaultPath string
	logLevel  slog.Level
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
			LogLevel:  s.logLevel.String(),
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
