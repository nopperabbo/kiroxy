// Package pool — Phase 2.5 refresh helper.
package pool

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/singleflight"

	"github.com/nopperabbo/kiroxy/internal/auth"
	"github.com/nopperabbo/kiroxy/internal/metrics"
	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

// RefreshFn refreshes a stored refresh_token against the upstream Kiro
// Desktop endpoint. Implementations should return:
//   - *auth.RefreshResult on success
//   - error wrapping auth.ErrRefreshUnauthorized for 401/403
//   - error wrapping auth.ErrRefreshTransient for 5xx / network (retryable)
type RefreshFn func(ctx context.Context, region, refreshToken string) (*auth.RefreshResult, error)

// DefaultRefreshFn builds a RefreshFn backed by the given http.Client using
// the canonical Kiro Desktop endpoint. Nil httpClient falls back to a new
// client with 10s timeout.
func DefaultRefreshFn(httpClient *http.Client) RefreshFn {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 10 * time.Second}
	}
	return func(ctx context.Context, region, refreshToken string) (*auth.RefreshResult, error) {
		endpoint := auth.SocialEndpoint(region)
		return auth.RefreshSocial(ctx, httpClient, endpoint, refreshToken)
	}
}

// RefreshConfig is injected into TokenGetter to control the refresh path.
// Zero value is usable (disables refresh): callers must set RefreshFn +
// Vault for refresh to engage.
type RefreshConfig struct {
	// RefreshFn is the upstream refresh call. When nil, no refresh happens.
	RefreshFn RefreshFn

	// Skew is how far before expiry we proactively refresh (default 5min).
	Skew time.Duration

	// LockTTL is the per-account vault reservation TTL. Should be greater
	// than the max expected refresh-roundtrip + retries (default 1min).
	LockTTL time.Duration

	// MaxRetries for transient failures (default 3).
	MaxRetries int

	// BaseBackoff is the first retry delay (default 500ms). Doubles each retry.
	BaseBackoff time.Duration

	// Metrics is the optional metrics sink. Nil-safe: a nil value disables
	// refresh-attempts emission at each call site.
	Metrics *metrics.Sink

	// group coalesces concurrent refreshes for the same (provider, id).
	group singleflight.Group
}

func (c *RefreshConfig) effectiveSkew() time.Duration {
	if c.Skew <= 0 {
		return 5 * time.Minute
	}
	return c.Skew
}

func (c *RefreshConfig) effectiveLockTTL() time.Duration {
	if c.LockTTL <= 0 {
		return time.Minute
	}
	return c.LockTTL
}

func (c *RefreshConfig) effectiveMaxRetries() int {
	if c.MaxRetries <= 0 {
		return 3
	}
	return c.MaxRetries
}

func (c *RefreshConfig) effectiveBaseBackoff() time.Duration {
	if c.BaseBackoff <= 0 {
		return 500 * time.Millisecond
	}
	return c.BaseBackoff
}

// accountMetadata is the subset of vault.Bundle.Metadata fields the refresher
// cares about. Every field is optional; missing fields result in zero-values.
//
// IdC fields (ClientID, ClientSecret, SSORegion) are scaffolded for the future
// IdC refresh path. They are written by cmd/kiroxy/accounts.go.addAccountViaOAuth
// (Phase 6.2) but not yet consumed by any refresh code (Phase 6.3 deferred per
// Karpathy-min: vault has 0 IdC accounts today). Parser-side support landing
// now means a later "extract auth.RefreshOIDC + plumb into pool" change touches
// only the call sites, not the schema. Without these tags, the JSON values are
// silently dropped on parse, so a future implementor would have to extend
// schema + onboarding + parser + refresh fn in the same commit — fragile.
type accountMetadata struct {
	AuthMethod   string `json:"auth_method"`
	ProfileArn   string `json:"profile_arn"`
	ExpiresAt    int64  `json:"expires_at"` // absolute unix seconds
	ExpiresIn    int64  `json:"expires_in"` // fallback — used with added_at
	AddedAt      string `json:"added_at"`
	ClientID     string `json:"client_id"`     // Phase 6.3 scaffold — IdC OAuth device-registration clientId
	ClientSecret string `json:"client_secret"` // Phase 6.3 scaffold — IdC OAuth device-registration clientSecret
	SSORegion    string `json:"region"`        // Phase 6.3 scaffold — OIDC endpoint region (Builder ID flow's sess.Region)
	MachineID    string `json:"machine_id"`    // Native-shape: per-account install fingerprint appended to UA. Lazy-generated on first GetToken if empty.
}

// parseAccountMetadata is tolerant of missing/malformed JSON — callers
// receive a zero-valued struct rather than an error so the caller can treat
// "unknown" as "don't refresh, pass through".
func parseAccountMetadata(raw string) accountMetadata {
	var md accountMetadata
	if raw == "" {
		return md
	}
	_ = json.Unmarshal([]byte(raw), &md)
	// Backfill: if only expires_in + added_at exist, compute expires_at.
	if md.ExpiresAt == 0 && md.ExpiresIn > 0 && md.AddedAt != "" {
		if t, err := time.Parse(time.RFC3339, md.AddedAt); err == nil {
			md.ExpiresAt = t.Unix() + md.ExpiresIn
		} else if t, err := time.Parse("2006-01-02T15:04:05", md.AddedAt); err == nil {
			md.ExpiresAt = t.Unix() + md.ExpiresIn
		}
	}
	return md
}

// needsRefresh reports whether we should proactively refresh this account.
// Returns false if not a social account, if expires_at is unknown (zero), or
// if the token is still valid beyond the skew window.
func needsRefresh(md accountMetadata, skew time.Duration, now time.Time) bool {
	if md.AuthMethod != "social" {
		return false
	}
	if md.ExpiresAt == 0 {
		return false
	}
	return time.Unix(md.ExpiresAt, 0).Before(now.Add(skew))
}

// refreshOne performs a single refresh attempt using the vault's
// Reserve/Commit generation-lock machinery and the caller's RefreshFn.
// Retries on transient errors with exponential backoff. Returns the NEW
// bundle on success.
//
// Concurrent calls for the same (provider, id) are coalesced via
// singleflight: only ONE refresh round-trip happens, all concurrent
// callers see the same result. This prevents the thundering-herd
// pattern where N goroutines that hit a stale token simultaneously
// all kick off independent refreshes against the upstream OIDC
// endpoint (each invalidating the others' grant on AWS).
//
// kind classifies WHY this refresh was triggered (proactive pre-expiry
// vs reactive 401/403), only for metric labelling — the refresh logic is
// identical either way.
func refreshOne(ctx context.Context, vault *tokenvault.Vault, cfg *RefreshConfig, provider, id, region string, kind metrics.RefreshKind) (*tokenvault.Bundle, error) {
	key := provider + "/" + id
	v, err, _ := cfg.group.Do(key, func() (any, error) {
		return refreshOneInner(ctx, vault, cfg, provider, id, region, kind)
	})
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	return v.(*tokenvault.Bundle), nil
}

func refreshOneInner(ctx context.Context, vault *tokenvault.Vault, cfg *RefreshConfig, provider, id, region string, kind metrics.RefreshKind) (*tokenvault.Bundle, error) {
	if cfg.RefreshFn == nil {
		cfg.Metrics.RefreshAttempt(kind, metrics.RefreshResultFailOther)
		return nil, errors.New("pool refresh: RefreshFn not configured")
	}
	refreshTok, gen, err := vault.Reserve(ctx, provider, id, cfg.effectiveLockTTL())
	if err != nil {
		cfg.Metrics.RefreshAttempt(kind, metrics.RefreshResultFailOther)
		return nil, fmt.Errorf("reserve: %w", err)
	}

	var lastErr error
	backoff := cfg.effectiveBaseBackoff()
	for attempt := 0; attempt <= cfg.effectiveMaxRetries(); attempt++ {
		if attempt > 0 {
			slog.Debug("pool refresh: retrying transient failure",
				"provider", provider, "id", id,
				"attempt", attempt, "delay", backoff, "last_err", lastErr)
			select {
			case <-ctx.Done():
				_ = vault.Release(ctx, provider, id, gen, false)
				cfg.Metrics.RefreshAttempt(kind, metrics.RefreshResultFailTransient)
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			backoff *= 2
		}
		result, rerr := cfg.RefreshFn(ctx, region, refreshTok)
		if rerr == nil {
			newRT := result.RefreshToken
			if newRT == "" {
				newRT = refreshTok
			}
			metaPatch := map[string]any{
				"expires_at":   result.ExpiresAt,
				"refreshed_at": time.Now().UTC().Format(time.RFC3339),
			}
			if result.ProfileARN != "" {
				metaPatch["profile_arn"] = result.ProfileARN
			}
			bundle, cerr := vault.CommitWithMetaPatch(ctx, provider, id, gen, tokenvault.Tokens{
				AccessToken:  result.AccessToken,
				RefreshToken: newRT,
				Source:       "pool-refresh",
			}, metaPatch)
			if cerr != nil {
				// Vault write failure — per D4, log WARN and bubble up.
				// The fresh access_token is lost but next request will
				// try again (same refresh_token still in vault from Reserve).
				slog.Warn("pool refresh: vault commit failed, fresh token not persisted",
					"account_id", id, "provider", provider,
					"err_type", fmt.Sprintf("%T", cerr),
					"err", cerr)
				_ = vault.Release(ctx, provider, id, gen, false)
				cfg.Metrics.RefreshAttempt(kind, metrics.RefreshResultFailOther)
				return nil, fmt.Errorf("vault commit: %w", cerr)
			}
			cfg.Metrics.RefreshAttempt(kind, metrics.RefreshResultSuccess)
			return bundle, nil
		}
		lastErr = rerr
		if errors.Is(rerr, auth.ErrRefreshUnauthorized) {
			// Terminal — refresh_token is dead. Release lock, bubble up
			// for caller to mark the account failed.
			_ = vault.Release(ctx, provider, id, gen, false)
			cfg.Metrics.RefreshAttempt(kind, metrics.RefreshResultFail401)
			return nil, rerr
		}
		if !errors.Is(rerr, auth.ErrRefreshTransient) {
			// Unknown error class — treat as terminal.
			_ = vault.Release(ctx, provider, id, gen, false)
			cfg.Metrics.RefreshAttempt(kind, metrics.RefreshResultFailOther)
			return nil, rerr
		}
		// Transient — continue retry loop.
	}
	_ = vault.Release(ctx, provider, id, gen, false)
	cfg.Metrics.RefreshAttempt(kind, metrics.RefreshResultFailTransient)
	return nil, fmt.Errorf("pool refresh: exhausted %d retries: %w", cfg.effectiveMaxRetries(), lastErr)
}
