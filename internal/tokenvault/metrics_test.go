// kiroxy addition — not derived from upstream.

package tokenvault

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func TestGenerationSum_EmptyVaultReturnsZero(t *testing.T) {
	v := openTestVault(t)
	if got := v.GenerationSum(context.Background()); got != 0 {
		t.Errorf("empty vault GenerationSum = %d, want 0", got)
	}
}

func TestGenerationSum_IncrementsOnSaveAndCommit(t *testing.T) {
	ctx := context.Background()
	v := openTestVault(t)

	b1, err := v.Save(ctx, "kiro", "a1", Tokens{AccessToken: "at1", RefreshToken: "rt1"})
	if err != nil {
		t.Fatal(err)
	}
	if b1.Generation != 1 {
		t.Fatalf("first save generation = %d, want 1", b1.Generation)
	}

	// Two more saves on two accounts → sum = 1 + 1 = 2.
	if _, err := v.Save(ctx, "kiro", "a2", Tokens{AccessToken: "at2", RefreshToken: "rt2"}); err != nil {
		t.Fatal(err)
	}
	if got := v.GenerationSum(ctx); got != 2 {
		t.Errorf("GenerationSum after 2 saves = %d, want 2", got)
	}

	// Refresh a1 → its generation becomes 2 → sum now 3.
	_, err = v.Refresh(ctx, "kiro", "a1", 0, func(_ context.Context, rt string) (Tokens, error) {
		return Tokens{AccessToken: "new", RefreshToken: rt}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := v.GenerationSum(ctx); got != 3 {
		t.Errorf("GenerationSum after refresh = %d, want 3", got)
	}
}

func TestRegisterVaultGauges_ExposesGeneration(t *testing.T) {
	ctx := context.Background()
	v := openTestVault(t)
	if _, err := v.Save(ctx, "kiro", "a1", Tokens{AccessToken: "at", RefreshToken: "rt"}); err != nil {
		t.Fatal(err)
	}

	reg := prometheus.NewRegistry()
	if err := RegisterVaultGauges(reg, v); err != nil {
		t.Fatal(err)
	}

	h := promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("scrape code=%d", rr.Code)
	}
	body := rr.Body.String()
	if !strings.Contains(body, `kiroxy_vault_generation 1`) {
		t.Errorf("expected kiroxy_vault_generation 1, got:\n%s", body)
	}
}

func openTestVault(t *testing.T) *Vault {
	t.Helper()
	dir := t.TempDir()
	v, err := Open(context.Background(), filepath.Join(dir, "v.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = v.Close() })
	return v
}
