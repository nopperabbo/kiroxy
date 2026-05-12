// kiroxy addition — not derived from upstream.

package tokenvault

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
)

// GenerationSum returns the total of generation numbers across all bundles
// in the vault. Used as a monotonic-ish aggregate: each successful refresh
// increments exactly one bundle's generation by 1, so the sum is a running
// count of refreshes across the entire vault's lifetime (modulo Account
// removal, which decreases it — accepted trade-off for a single cheap SQL
// query vs. a separate counter table).
//
// Returns 0 on any error so callers (Prometheus GaugeFunc) never panic
// during scrape.
func (v *Vault) GenerationSum(ctx context.Context) int64 {
	row := v.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(generation), 0) FROM token_bundles`)
	var sum int64
	if err := row.Scan(&sum); err != nil {
		return 0
	}
	return sum
}

// RegisterVaultGauges attaches kiroxy_vault_generation GaugeFunc to the given
// registry. The callback is invoked per Prometheus scrape.
func RegisterVaultGauges(r prometheus.Registerer, v *Vault) error {
	if r == nil || v == nil {
		return nil
	}
	return r.Register(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "kiroxy_vault_generation",
			Help: "Sum of generation counters across all bundles (monotonic-ish; increments once per successful token refresh).",
		},
		func() float64 {
			return float64(v.GenerationSum(context.Background()))
		},
	))
}
