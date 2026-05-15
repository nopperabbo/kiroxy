package main

import (
	"context"
	"testing"
	"time"

	"github.com/nopperabbo/kiroxy/internal/pool"
	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

func TestDashboardProvider_HealthFieldsSurface(t *testing.T) {
	dir := t.TempDir()
	v, err := tokenvault.Open(context.Background(), dir+"/vault.db")
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })

	p := pool.New(pool.DefaultPolicy())
	p.Add(pool.Account{ID: "acc1", Label: "acc1", Provider: "kiro", Region: "us-east-1", Enabled: true})
	p.Add(pool.Account{ID: "acc2", Label: "acc2", Provider: "kiro", Region: "us-east-1", Enabled: true})

	// Drive some activity so health metrics aren't all zeros.
	p.RecordSuccessWithLatency("acc1", 150*time.Millisecond)
	p.RecordFailure("acc2", pool.FailureQuota, "429")

	d := &dashboardProvider{
		version:   "test",
		vaultPath: dir + "/vault.db",
		vault:     v,
		pool:      p,
		startedAt: time.Now(),
	}
	state := d.DashboardSnapshot(context.Background())

	if len(state.Accounts) != 2 {
		t.Fatalf("want 2 accounts in snapshot, got %d", len(state.Accounts))
	}

	var row1, row2 *int
	for i := range state.Accounts {
		switch state.Accounts[i].ID {
		case "acc1":
			row1 = &i
		case "acc2":
			row2 = &i
		}
	}
	if row1 == nil || row2 == nil {
		t.Fatalf("missing expected account rows; got %+v", state.Accounts)
	}

	a1 := state.Accounts[*row1]
	a2 := state.Accounts[*row2]

	// acc1: success recorded, weight should be near ceiling.
	if a1.Weight < 0.5 {
		t.Errorf("acc1 weight after success should be healthy, got %f", a1.Weight)
	}
	if a1.AvgLatencyMs == 0 {
		t.Errorf("acc1 AvgLatencyMs should be populated from RecordSuccessWithLatency, got 0")
	}

	// acc2: quota failure, weight depressed, LastRateLimit populated.
	if a2.LastRateLimit == "" {
		t.Errorf("acc2 LastRateLimit should be RFC3339-formatted after quota failure, got empty")
	}
	if a2.Weight >= a1.Weight {
		t.Errorf("acc2 weight (%f) should be below acc1 weight (%f) after quota failure", a2.Weight, a1.Weight)
	}
}
