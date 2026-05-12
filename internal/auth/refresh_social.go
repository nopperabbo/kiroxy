// Package auth — standalone social-refresh helper.
//
// This file exposes RefreshSocial as a package-level function other packages
// (notably internal/pool) can call without needing an AuthManager or a
// kiro-cli SQLite DB. AuthManager.refreshSocialToken remains intact for
// kiro-cli mode compatibility.
//
// Error model is deliberately typed: callers classify 401 vs 5xx vs
// network failures differently (pool.maybeRefresh uses the split for its
// Phase 2.5 D4 failure-mode policy — 401 triggers 1h quota cooldown,
// 5xx/network retries with backoff).
package auth

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// RefreshResult is what RefreshSocial returns on success. ExpiresAt is an
// absolute unix-seconds timestamp computed from the server's expiresIn.
type RefreshResult struct {
	AccessToken  string
	RefreshToken string // server may return empty; caller should retain the old one in that case
	ExpiresAt    int64
	ProfileARN   string
}

// Refresh errors — callers switch on these via errors.Is.
var (
	// ErrRefreshUnauthorized means the refresh_token itself is revoked /
	// expired / invalidated. Not retryable; requires operator action.
	ErrRefreshUnauthorized = errors.New("auth: refresh token rejected (401/403)")

	// ErrRefreshTransient wraps 5xx, network, or timeout errors.
	// Retryable with backoff.
	ErrRefreshTransient = errors.New("auth: refresh transient failure")

	// ErrRefreshMalformed is returned when the endpoint returns 200 but the
	// body is missing required fields. Treated as transient by default.
	ErrRefreshMalformed = errors.New("auth: refresh response malformed")
)

// SocialEndpoint returns the Kiro Desktop refresh URL for the given region.
func SocialEndpoint(region string) string {
	if region == "" {
		region = "us-east-1"
	}
	return fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/refreshToken", region)
}

// RefreshSocial calls the Kiro Desktop refreshToken endpoint.
// Returned errors wrap one of ErrRefreshUnauthorized / ErrRefreshTransient /
// ErrRefreshMalformed so callers can errors.Is them.
func RefreshSocial(ctx context.Context, httpClient *http.Client, endpoint, refreshToken string) (*RefreshResult, error) {
	if refreshToken == "" {
		return nil, fmt.Errorf("%w: empty refresh token", ErrRefreshMalformed)
	}
	body, err := json.Marshal(map[string]string{"refreshToken": refreshToken})
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRefreshTransient, err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%w: status=%d body=%q", ErrRefreshUnauthorized, resp.StatusCode, string(truncSocial(errBody, 256)))
	case resp.StatusCode >= 500:
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%w: status=%d body=%q", ErrRefreshTransient, resp.StatusCode, string(truncSocial(errBody, 256)))
	case resp.StatusCode != http.StatusOK:
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("%w: status=%d body=%q", ErrRefreshTransient, resp.StatusCode, string(truncSocial(errBody, 256)))
	}

	var tr tokenResponse
	if err := json.UnmarshalRead(resp.Body, &tr); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrRefreshMalformed, err)
	}
	if tr.AccessToken == "" {
		return nil, fmt.Errorf("%w: empty access_token", ErrRefreshMalformed)
	}
	if tr.ExpiresIn <= 0 {
		return nil, fmt.Errorf("%w: invalid expires_in=%d", ErrRefreshMalformed, tr.ExpiresIn)
	}
	return &RefreshResult{
		AccessToken:  tr.AccessToken,
		RefreshToken: tr.RefreshToken, // may be empty; caller coalesces
		ExpiresAt:    time.Now().Unix() + tr.ExpiresIn,
		ProfileARN:   tr.ProfileArn,
	}, nil
}

func truncSocial(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return append(append([]byte{}, b[:n]...), "..."...)
}
