// This file is derived from github.com/d-kuro/kirocc
// Original commit: 5633c47f0d65aaef748728bae1c68160b0ea538d
// Copyright (c) 2026 d-kuro. Licensed under Apache License, Version 2.0.
// Modifications (c) 2026 kiroxy contributors.

package kiroclient

import (
	"context"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const upstreamBodyLimit = 8192

// maxRetryAfter caps any server-supplied Retry-After to a sane ceiling.
// AWS Bedrock + CodeWhisperer rarely emit values above 30s; anything larger
// is more than the caller's likely deadline anyway.
const maxRetryAfter = 30 * time.Second

// backoffDelay returns exponential backoff delay with ±25% jitter.
func backoffDelay(attempt int) time.Duration {
	base := baseRetryDelay << attempt
	jitter := time.Duration(rand.Int64N(int64(base)/2)) - base/4
	return base + jitter
}

// effectiveDelay honors a server-supplied Retry-After header on 429 / 5xx,
// falling back to exponential backoff. The header may be either an integer
// number of seconds or an HTTP-date (RFC 7231 §7.1.3). Returns the LARGER
// of the parsed Retry-After and the natural backoffDelay so we never retry
// EARLIER than our own algorithm would, but always respect a longer pause
// when the upstream asks for one.
func effectiveDelay(resp *http.Response, attempt int) time.Duration {
	natural := backoffDelay(attempt)
	if resp == nil {
		return natural
	}
	h := resp.Header.Get("Retry-After")
	if h == "" {
		return natural
	}
	var server time.Duration
	if secs, err := strconv.Atoi(h); err == nil && secs >= 0 {
		server = time.Duration(secs) * time.Second
	} else if t, err := http.ParseTime(h); err == nil {
		if d := time.Until(t); d > 0 {
			server = d
		}
	}
	if server <= 0 {
		return natural
	}
	if server > maxRetryAfter {
		server = maxRetryAfter
	}
	if server > natural {
		return server
	}
	return natural
}

// readLimitedBody reads up to n bytes from body and closes it.
func readLimitedBody(body io.ReadCloser, n int64) string {
	b, _ := io.ReadAll(io.LimitReader(body, n))
	_ = body.Close()
	return string(b)
}

// retryWait waits for the given delay, respecting ctx cancellation.
func retryWait(ctx context.Context, delay time.Duration) error {
	t := time.NewTimer(delay)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
