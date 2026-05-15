// kiroxy addition — not derived from upstream.

package pool

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/nopperabbo/kiroxy/internal/metrics"
)

func TestSnapshot_ClassifiesEnabledDisabledCooldown(t *testing.T) {
	p := New(DefaultPolicy())

	p.Add(Account{ID: "a-available", Provider: "kiro", Enabled: true})
	p.Add(Account{ID: "b-disabled", Provider: "kiro", Enabled: false})
	p.Add(Account{ID: "c-cooldown", Provider: "kiro", Enabled: true})

	// Drive "c" into quota cooldown.
	p.RecordFailure("c-cooldown", FailureQuota, "429")

	s := p.Snapshot()
	if s.Total != 3 {
		t.Errorf("Total = %d, want 3", s.Total)
	}
	if s.Available != 1 {
		t.Errorf("Available = %d, want 1", s.Available)
	}
	if s.Failed != 1 {
		t.Errorf("Failed = %d, want 1", s.Failed)
	}
	if s.Cooldown != 1 {
		t.Errorf("Cooldown = %d, want 1", s.Cooldown)
	}
}

func TestRecordFailure_EmitsCooldownTransitionOnly(t *testing.T) {
	reg := metrics.New(time.Now())
	p := New(DefaultPolicy())
	p.SetMetricsSink(reg.Sink())
	p.Add(Account{ID: "x", Provider: "kiro", Enabled: true})

	// First quota failure → cooldown just applied → metric +1.
	p.RecordFailure("x", FailureQuota, "429")

	// Transient failure below threshold → cooldown NOT applied yet, no emit.
	pol := DefaultPolicy()
	for i := 0; i < pol.ConsecutiveErrorThreshold-1; i++ {
		p.RecordFailure("x", FailureTransient, "5xx")
	}

	body := scrape(t, reg)
	if !strings.Contains(body, `kiroxy_account_cooldowns_total{reason="quota"} 1`) {
		t.Errorf("expected quota cooldown =1, body:\n%s", body)
	}
}

func TestRegisterPoolGauges_ExposesCounts(t *testing.T) {
	p := New(DefaultPolicy())
	p.Add(Account{ID: "a", Provider: "kiro", Enabled: true})
	p.Add(Account{ID: "b", Provider: "kiro", Enabled: false})

	reg := prometheus.NewRegistry()
	if err := RegisterPoolGauges(reg, p); err != nil {
		t.Fatalf("register: %v", err)
	}

	body := scrapeRegistry(t, reg)

	if !strings.Contains(body, `kiroxy_accounts_available 1`) {
		t.Errorf("expected available=1, got:\n%s", body)
	}
	if !strings.Contains(body, `kiroxy_accounts_failed 1`) {
		t.Errorf("expected failed=1, got:\n%s", body)
	}
	if !strings.Contains(body, `kiroxy_accounts_cooldown 0`) {
		t.Errorf("expected cooldown=0, got:\n%s", body)
	}
}

func scrape(t *testing.T, reg *metrics.Registry) string {
	t.Helper()
	rr := httptest.NewRecorder()
	reg.Handler().ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("scrape code=%d body=%s", rr.Code, rr.Body.String())
	}
	return rr.Body.String()
}

func scrapeRegistry(t *testing.T, reg *prometheus.Registry) string {
	t.Helper()
	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		data, _ := io.ReadAll(rr.Body)
		t.Fatalf("scrape code=%d body=%s", rr.Code, string(data))
	}
	return rr.Body.String()
}
