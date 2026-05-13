package main

import (
	"context"
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/pool"
	"local/kiroxy/internal/server"
	"local/kiroxy/internal/tokenvault"
)

// TestDashboardProvider_UsageFieldsPropagate confirms that a fresh
// poll result reaches the JSON wire format consumed by the Mansion
// frontend and the legacy /dashboard/api/state endpoint.
func TestDashboardProvider_UsageFieldsPropagate(t *testing.T) {
	dir := t.TempDir()
	v, err := tokenvault.Open(context.Background(), dir+"/vault.db")
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })

	p := pool.New(pool.DefaultPolicy())
	p.Add(pool.Account{ID: "fresh", Provider: "kiro", Region: "us-east-1", Enabled: true})
	p.Add(pool.Account{ID: "dim", Provider: "kiro", Region: "us-east-1", Enabled: true})
	p.Add(pool.Account{ID: "unpolled", Provider: "kiro", Region: "us-east-1", Enabled: true})

	stamp := time.Now().Add(-30 * time.Second).UTC().Truncate(time.Second)
	p.SetUsage("fresh", &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      120,
		MonthlyCreditsRemaining: 880,
		PercentUsed:             0.12,
		LastQueryTime:           stamp,
		DaysUntilReset:          11,
	})
	p.SetUsage("dim", &kiroclient.UsageLimits{
		MonthlyCap:              1000,
		MonthlyCreditsUsed:      950,
		MonthlyCreditsRemaining: 50,
		PercentUsed:             0.95,
		LastQueryTime:           stamp,
	})

	d := &dashboardProvider{
		version:   "test",
		vaultPath: dir + "/vault.db",
		vault:     v,
		pool:      p,
		startedAt: time.Now(),
	}
	state := d.DashboardSnapshot(context.Background())

	rows := map[string]server.DashboardAccount{}
	for _, a := range state.Accounts {
		rows[a.ID] = a
	}

	fresh, ok := rows["fresh"]
	if !ok {
		t.Fatalf("fresh row missing; got %+v", rows)
	}
	if !fresh.UsageKnown {
		t.Error("fresh.UsageKnown must be true after SetUsage")
	}
	if fresh.UsageRemaining != 880 || fresh.UsageCap != 1000 {
		t.Errorf("fresh usage drift: cap=%d remaining=%d", fresh.UsageCap, fresh.UsageRemaining)
	}
	if fresh.UsageDaysUntilRst != 11 {
		t.Errorf("fresh days until reset: got %d, want 11", fresh.UsageDaysUntilRst)
	}
	if fresh.UsageLastPolled == "" {
		t.Error("fresh.UsageLastPolled should be RFC3339-formatted")
	}

	dim := rows["dim"]
	if dim.UsageRemaining != 50 {
		t.Errorf("dim row remaining: %d, want 50", dim.UsageRemaining)
	}
	if dim.Weight >= fresh.Weight {
		t.Errorf("dim (5%% remaining) weight %f should be below fresh weight %f", dim.Weight, fresh.Weight)
	}

	unpolled := rows["unpolled"]
	if unpolled.UsageKnown {
		t.Error("never-polled account must not report UsageKnown=true")
	}
	if unpolled.UsageRemaining != 0 || unpolled.UsageCap != 0 {
		t.Error("never-polled account must have zero usage fields")
	}
}

// TestDashboardAPIState_UsageFieldsRoundTripJSON exercises the full
// /dashboard/api/state pipeline: the cmd-side dashboardProvider feeds
// the server-side JSON encoder, and the wire bytes contain the
// expected fields. This is what the Mansion API shim and the legacy
// dashboard JS both consume.
func TestDashboardAPIState_UsageFieldsRoundTripJSON(t *testing.T) {
	dir := t.TempDir()
	v, err := tokenvault.Open(context.Background(), dir+"/vault.db")
	if err != nil {
		t.Fatalf("open vault: %v", err)
	}
	t.Cleanup(func() { _ = v.Close() })

	p := pool.New(pool.DefaultPolicy())
	p.Add(pool.Account{ID: "live", Provider: "kiro", Region: "us-east-1", Enabled: true})
	p.SetUsage("live", &kiroclient.UsageLimits{
		MonthlyCap:              2000,
		MonthlyCreditsUsed:      400,
		MonthlyCreditsRemaining: 1600,
		PercentUsed:             0.20,
		LastQueryTime:           time.Now(),
		DaysUntilReset:          7,
	})

	d := &dashboardProvider{
		version:   "test-version",
		vaultPath: dir + "/vault.db",
		vault:     v,
		pool:      p,
		startedAt: time.Now(),
	}

	srv := server.New(server.Options{
		APIKey:                 "secret",
		DashboardStateProvider: d,
	})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/dashboard/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("state api want 200, got %d", resp.StatusCode)
	}

	var got server.DashboardState
	if err := json.UnmarshalRead(resp.Body, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Accounts) != 1 {
		t.Fatalf("want 1 account in snapshot, got %d", len(got.Accounts))
	}
	row := got.Accounts[0]
	if !row.UsageKnown {
		t.Error("UsageKnown should serialize as true (omitempty respects the bool=true case)")
	}
	if row.UsageRemaining != 1600 {
		t.Errorf("UsageRemaining: got %d, want 1600", row.UsageRemaining)
	}
	if row.UsageDaysUntilRst != 7 {
		t.Errorf("UsageDaysUntilRst: got %d, want 7", row.UsageDaysUntilRst)
	}
}

// TestDashboardAPIState_OmitsUsageFieldsWhenUnknown is a contract
// test confirming the JSON omits the usage block for never-polled
// accounts. Mansion uses field presence to decide whether to render
// the credit pill.
func TestDashboardAPIState_OmitsUsageFieldsWhenUnknown(t *testing.T) {
	dir := t.TempDir()
	v, err := tokenvault.Open(context.Background(), dir+"/vault.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })

	p := pool.New(pool.DefaultPolicy())
	p.Add(pool.Account{ID: "never", Provider: "kiro", Region: "us-east-1", Enabled: true})

	d := &dashboardProvider{
		version: "test", vaultPath: dir + "/vault.db",
		vault: v, pool: p, startedAt: time.Now(),
	}
	srv := server.New(server.Options{APIKey: "secret", DashboardStateProvider: d})
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/dashboard/api/state")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	body := string(bodyBytes)
	for _, banned := range []string{"\"usage_known\":true", "\"usage_cap\":", "\"usage_remaining\":"} {
		if strings.Contains(body, banned) {
			t.Errorf("body must omit %s for never-polled account; body=%s", banned, body)
		}
	}
}
