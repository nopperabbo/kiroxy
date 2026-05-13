package main

import (
	"context"
	"time"

	"local/kiroxy/internal/pool"
	"local/kiroxy/internal/server"
	"local/kiroxy/internal/tokenvault"
)

type dashboardProvider struct {
	version   string
	vaultPath string
	vault     *tokenvault.Vault
	pool      *pool.Pool
	startedAt time.Time
}

func (d *dashboardProvider) DashboardSnapshot(ctx context.Context) server.DashboardState {
	state := server.DashboardState{
		Version:   d.version,
		UptimeS:   int64(time.Since(d.startedAt).Seconds()),
		VaultPath: d.vaultPath,
	}
	if d.vault != nil {
		if _, err := d.vault.ListByProvider(ctx, "kiro"); err == nil {
			state.VaultOK = true
		}
	}
	if d.pool != nil {
		snap := d.pool.List()
		// HealthSnapshots is keyed by account ID for O(1) lookup while
		// we merge the rolling metrics into each row.
		healthByID := make(map[string]pool.HealthSnapshot)
		for _, hs := range d.pool.HealthSnapshots() {
			healthByID[hs.AccountID] = hs
		}
		for _, a := range snap {
			row := server.DashboardAccount{
				ID:        a.ID,
				Enabled:   a.Enabled,
				Requests:  a.RequestCount,
				Errors:    a.ErrorCount,
				LastError: a.LastError,
			}
			if !a.CooldownUntil.IsZero() && a.CooldownUntil.After(time.Now()) {
				row.CooldownUntil = a.CooldownUntil.Format(time.RFC3339)
			}
			if hs, ok := healthByID[a.ID]; ok {
				row.SuccessRate = hs.SuccessRate
				row.Weight = hs.Weight
				row.RequestsLast5m = hs.RequestsInWindow
				if hs.AvgLatency > 0 {
					row.AvgLatencyMs = hs.AvgLatency.Milliseconds()
				}
				if !hs.LastRateLimit.IsZero() {
					row.LastRateLimit = hs.LastRateLimit.Format(time.RFC3339)
				}
				if hs.UsageKnown {
					row.UsageKnown = true
					row.UsageCap = hs.UsageCap
					row.UsageUsed = hs.UsageUsed
					row.UsageRemaining = hs.UsageRemaining
					row.UsagePercentUsed = hs.UsagePercentUsed
					row.UsageDaysUntilRst = hs.UsageDaysUntilRst
					if !hs.UsageLastPolled.IsZero() {
						row.UsageLastPolled = hs.UsageLastPolled.Format(time.RFC3339)
					}
				}
			}
			state.Accounts = append(state.Accounts, row)
		}
		if state.VaultOK && d.pool.Count() > 0 {
			state.Ready = true
		} else {
			state.ReadyDetail = "no accounts configured; run `kiroxy add-account`"
		}
	}
	return state
}
