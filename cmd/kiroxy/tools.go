package main

import (
	"context"

	"github.com/nopperabbo/kiroxy/internal/doctor"
	"github.com/nopperabbo/kiroxy/internal/pool"
	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

// toolsProvider adapts the existing vault + pool into server.ToolsProvider.
// Keeping it thin — the doctor package does all the actual work; this
// file just gives it the runtime-specific bits it can't know about.
type toolsProvider struct {
	vaultPath   string
	upstreamURL string
	vault       *tokenvault.Vault
	pool        *pool.Pool
}

func (t *toolsProvider) Doctor(ctx context.Context) *doctor.Report {
	c := &doctor.Checker{
		VaultPath:   t.vaultPath,
		UpstreamURL: t.upstreamURL,
	}
	if t.pool != nil {
		c.AccountsHint = func() int { return t.pool.Count() }
	} else if t.vault != nil {
		c.AccountsHint = func() int {
			accts, err := t.vault.ListByProvider(ctx, "kiro")
			if err != nil {
				return 0
			}
			return len(accts)
		}
	}
	return c.Run(ctx)
}
