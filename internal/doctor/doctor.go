// Package doctor implements diagnostic health checks for kiroxy.
//
// It is invoked from two surfaces:
//
//   - The `kiroxy doctor` CLI subcommand prints a human-readable report
//     to stdout and exits 0 when all checks pass, 1 otherwise.
//
//   - The /dashboard/api/tools/doctor handler returns the same report as
//     JSON so the Mansion Tools view can render it.
//
// Each check is short, side-effect-free, and bounded in time: doctor must
// not hang. Network checks honour ctx cancellation.
package doctor

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Status is the categorical result of a single check.
type Status string

const (
	StatusOK    Status = "ok"
	StatusWarn  Status = "warn"
	StatusError Status = "error"
	StatusSkip  Status = "skip"
)

// Result is one check's outcome.
type Result struct {
	Name    string        `json:"name"`
	Status  Status        `json:"status"`
	Detail  string        `json:"detail"`
	Hint    string        `json:"hint,omitempty"`
	Elapsed time.Duration `json:"elapsed_ns"`
}

// Report bundles all check results plus a top-level pass/fail summary.
type Report struct {
	OK        bool      `json:"ok"`
	StartedAt time.Time `json:"started_at"`
	Elapsed   string    `json:"elapsed"`
	Results   []Result  `json:"results"`
	GoVersion string    `json:"go_version,omitempty"`
}

// Checker runs the configured checks. Zero-value Checker is usable; it
// simply skips checks that need fields it doesn't have.
//
// Fields are kept exported so cmd/kiroxy and the server handler can
// populate them differently (CLI knows config + paths, server knows the
// running pool).
type Checker struct {
	// VaultPath is checked for existence + readable permissions. Empty
	// skips the vault check.
	VaultPath string
	// UpstreamURL is dialled (TCP only — no auth) to validate DNS +
	// reachability. Empty skips the upstream check.
	UpstreamURL string
	// HTTPTimeout caps each network probe. Defaults to 4 seconds.
	HTTPTimeout time.Duration
	// AccountsHint is a callback that returns the configured-account count
	// at runtime; nil skips the accounts check.
	AccountsHint func() int
}

// Run executes every configured check in order and returns the populated
// report. The returned report is always non-nil; ctx cancellation just
// cuts the run short with whatever results are already gathered.
func (c *Checker) Run(ctx context.Context) *Report {
	if c.HTTPTimeout <= 0 {
		c.HTTPTimeout = 4 * time.Second
	}
	rep := &Report{
		StartedAt: time.Now().UTC(),
		GoVersion: runtime.Version(),
	}
	defer func() {
		rep.Elapsed = time.Since(rep.StartedAt).Round(time.Millisecond).String()
		rep.OK = allOK(rep.Results)
	}()

	checks := []func(context.Context) Result{
		c.checkRuntime,
		c.checkVault,
		c.checkUpstream,
		c.checkAccounts,
	}
	for _, fn := range checks {
		select {
		case <-ctx.Done():
			rep.Results = append(rep.Results, Result{
				Name:   "cancelled",
				Status: StatusWarn,
				Detail: "context cancelled before all checks ran",
			})
			return rep
		default:
		}
		started := time.Now()
		r := fn(ctx)
		r.Elapsed = time.Since(started)
		rep.Results = append(rep.Results, r)
	}
	return rep
}

func (c *Checker) checkRuntime(_ context.Context) Result {
	host, _ := os.Hostname()
	return Result{
		Name:   "runtime",
		Status: StatusOK,
		Detail: fmt.Sprintf("kiroxy on %s/%s, host=%s, pid=%d",
			runtime.GOOS, runtime.GOARCH, host, os.Getpid()),
	}
}

func (c *Checker) checkVault(_ context.Context) Result {
	if c.VaultPath == "" {
		return Result{
			Name:   "vault",
			Status: StatusSkip,
			Detail: "VaultPath unset (kiro-cli SQLite mode?)",
		}
	}
	info, err := os.Stat(c.VaultPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Result{
				Name:   "vault",
				Status: StatusError,
				Detail: fmt.Sprintf("%s does not exist", c.VaultPath),
				Hint:   "run `kiroxy add-account` or `kiroxy import-accounts` to create the vault",
			}
		}
		return Result{
			Name:   "vault",
			Status: StatusError,
			Detail: err.Error(),
		}
	}
	if info.IsDir() {
		return Result{
			Name:   "vault",
			Status: StatusError,
			Detail: c.VaultPath + " is a directory, expected a SQLite file",
		}
	}
	if info.Size() == 0 {
		return Result{
			Name:   "vault",
			Status: StatusWarn,
			Detail: c.VaultPath + " is empty",
			Hint:   "the vault was created but no accounts have been imported",
		}
	}
	mode := info.Mode().Perm()
	hint := ""
	status := StatusOK
	if runtime.GOOS != "windows" && mode&0o077 != 0 {
		status = StatusWarn
		hint = "vault file is world/group readable; chmod 600 recommended"
	}
	return Result{
		Name:   "vault",
		Status: status,
		Detail: fmt.Sprintf("%s (%d bytes, mode %04o)", filepath.Base(c.VaultPath), info.Size(), mode),
		Hint:   hint,
	}
}

func (c *Checker) checkUpstream(ctx context.Context) Result {
	target := c.UpstreamURL
	if target == "" {
		target = "https://q.us-east-1.amazonaws.com"
	}
	u, err := url.Parse(target)
	if err != nil {
		return Result{
			Name:   "upstream",
			Status: StatusError,
			Detail: fmt.Sprintf("parse %s: %v", target, err),
		}
	}
	host := u.Host
	if !strings.Contains(host, ":") {
		if u.Scheme == "https" {
			host = host + ":443"
		} else {
			host = host + ":80"
		}
	}
	dialer := &net.Dialer{Timeout: c.HTTPTimeout}
	dialCtx, cancel := context.WithTimeout(ctx, c.HTTPTimeout)
	defer cancel()
	conn, err := dialer.DialContext(dialCtx, "tcp", host)
	if err != nil {
		return Result{
			Name:   "upstream",
			Status: StatusError,
			Detail: fmt.Sprintf("dial %s: %v", host, err),
			Hint:   "check network reachability + DNS; firewall may block the Kiro endpoint",
		}
	}
	_ = conn.Close()
	return Result{
		Name:   "upstream",
		Status: StatusOK,
		Detail: fmt.Sprintf("dial %s ok", host),
	}
}

func (c *Checker) checkAccounts(_ context.Context) Result {
	if c.AccountsHint == nil {
		return Result{
			Name:   "accounts",
			Status: StatusSkip,
			Detail: "no AccountsHint configured (CLI mode)",
		}
	}
	n := c.AccountsHint()
	switch {
	case n == 0:
		return Result{
			Name:   "accounts",
			Status: StatusError,
			Detail: "no accounts in pool",
			Hint:   "run `kiroxy add-account` or import via Mansion ⌘K → Import",
		}
	case n < 3:
		return Result{
			Name:   "accounts",
			Status: StatusWarn,
			Detail: fmt.Sprintf("%d account(s) in pool", n),
			Hint:   "consider adding more for better cooldown headroom",
		}
	default:
		return Result{
			Name:   "accounts",
			Status: StatusOK,
			Detail: fmt.Sprintf("%d accounts in pool", n),
		}
	}
}

func allOK(results []Result) bool {
	for _, r := range results {
		if r.Status == StatusError {
			return false
		}
	}
	return true
}

// Format returns a human-readable rendering of the report. Used by the
// CLI; the dashboard handler calls Run directly and serializes JSON.
func (r *Report) Format() string {
	var b strings.Builder
	headline := "kiroxy doctor — "
	if r.OK {
		headline += "all checks passed"
	} else {
		headline += "issues found"
	}
	fmt.Fprintf(&b, "%s\n", headline)
	fmt.Fprintf(&b, "started=%s elapsed=%s go=%s\n\n",
		r.StartedAt.Format(time.RFC3339), r.Elapsed, r.GoVersion)
	for _, c := range r.Results {
		marker := "✓"
		switch c.Status {
		case StatusError:
			marker = "✗"
		case StatusWarn:
			marker = "!"
		case StatusSkip:
			marker = "·"
		}
		fmt.Fprintf(&b, "  %s %-12s %s\n", marker, c.Name, c.Detail)
		if c.Hint != "" {
			fmt.Fprintf(&b, "      hint: %s\n", c.Hint)
		}
	}
	return b.String()
}

// ProbeHTTP is a helper exported for callers that want a richer status
// check than checkUpstream (they can hit a specific path and inspect the
// status code). Currently unused internally; kept here so the doctor
// package owns all its network probes.
func ProbeHTTP(ctx context.Context, target string, timeout time.Duration) error {
	if timeout <= 0 {
		timeout = 4 * time.Second
	}
	client := &http.Client{Timeout: timeout}
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return err
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}
