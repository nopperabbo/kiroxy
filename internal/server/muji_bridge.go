// Package server: bridge between the parent server's
// DashboardStateProvider and the muji variant's decoupled Snapshot
// type. Lives in its own file so concurrent edits to dashboard.go
// and server.go don't collide with the muji wiring.
package server

import (
	"context"
	"time"

	"github.com/nopperabbo/kiroxy/internal/server/variants/muji"
)

// mujiSnap is the SnapFn the muji.Register call expects. It pulls the
// current state via the existing DashboardStateProvider plumbing and
// projects it into the muji-local Snapshot/Account types.
//
// The muji variant lives in its own package and deliberately does NOT
// depend on internal/server (cycle-prevention + isolation). The bridge
// here owns the type translation in one place.
func (s *Server) mujiSnap(ctx context.Context) muji.Snapshot {
	out := muji.Snapshot{
		Version: s.opts.Version,
		UptimeS: int64(time.Since(s.startedAt).Seconds()),
	}
	if s.opts.DashboardStateProvider == nil {
		return out
	}
	state := s.opts.DashboardStateProvider.DashboardSnapshot(ctx)
	if state.Version != "" {
		out.Version = state.Version
	}
	if state.UptimeS != 0 {
		out.UptimeS = state.UptimeS
	}
	out.Ready = state.Ready
	out.ReadyText = state.ReadyDetail
	out.VaultOK = state.VaultOK
	out.Accounts = make([]muji.Account, 0, len(state.Accounts))
	for _, a := range state.Accounts {
		out.Accounts = append(out.Accounts, muji.Account{
			ID:            a.ID,
			Enabled:       a.Enabled,
			Requests:      a.Requests,
			Errors:        a.Errors,
			CooldownUntil: a.CooldownUntil,
			LastError:     a.LastError,
		})
	}
	return out
}
