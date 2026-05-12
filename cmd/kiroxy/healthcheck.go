// Package main — healthcheck subcommand.
//
// Distroless images have no shell and no curl, so the container HEALTHCHECK
// cannot shell out. Instead, Docker runs `kiroxy healthcheck` inside the
// container; this command performs an in-process HTTP GET against
// /healthz on the loopback interface and exits non-zero on any failure.
//
// Flags:
//
//	--url URL  target to probe (default http://127.0.0.1:${KIROXY_PORT:-8787}/healthz)
//	--timeout  dial+read timeout in seconds (default 3)
//
// Exit codes:
//
//	0  /healthz returned 200 with JSON body status=="ok"
//	1  any failure (dial, timeout, non-200, bad body)
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// runHealthcheck is the container-side liveness probe entry point.
// It is intentionally dependency-free: no slog, no internal packages,
// so the runtime stage stays fast and fail-safe.
func runHealthcheck(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	url := fs.String("url", defaultHealthcheckURL(), "healthz URL to probe")
	timeout := fs.Int("timeout", 3, "timeout seconds for dial+read")
	if err := fs.Parse(args); err != nil {
		return err
	}

	probeCtx, cancel := context.WithTimeout(ctx, time.Duration(*timeout)*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, *url, nil)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	// Use a fresh transport so we don't inherit any ambient client config.
	client := &http.Client{Timeout: time.Duration(*timeout) * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("probe %s: %w", *url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// Drain a small prefix so callers can eyeball the error via `docker inspect`.
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(body))
	}

	var payload struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<10)).Decode(&payload); err != nil {
		return fmt.Errorf("decode body: %w", err)
	}
	if payload.Status != "ok" {
		return fmt.Errorf("status field = %q, want %q", payload.Status, "ok")
	}
	return nil
}

func defaultHealthcheckURL() string {
	port := os.Getenv("KIROXY_PORT")
	if port == "" {
		port = "8787"
	}
	// Always loopback — healthcheck runs inside the same container.
	return fmt.Sprintf("http://127.0.0.1:%s/healthz", port)
}
