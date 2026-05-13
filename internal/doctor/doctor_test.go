package doctor

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestChecker_RuntimeAlwaysPasses(t *testing.T) {
	c := &Checker{}
	rep := c.Run(context.Background())
	if rep == nil || len(rep.Results) == 0 {
		t.Fatalf("want non-empty report")
	}
	var runtime Result
	for _, r := range rep.Results {
		if r.Name == "runtime" {
			runtime = r
		}
	}
	if runtime.Status != StatusOK {
		t.Fatalf("runtime should be OK, got %q %q", runtime.Status, runtime.Detail)
	}
}

func TestChecker_VaultSkipWhenEmpty(t *testing.T) {
	c := &Checker{}
	rep := c.Run(context.Background())
	for _, r := range rep.Results {
		if r.Name == "vault" && r.Status != StatusSkip {
			t.Fatalf("want Skip when VaultPath empty, got %q", r.Status)
		}
	}
}

func TestChecker_VaultMissingFile(t *testing.T) {
	c := &Checker{VaultPath: filepath.Join(t.TempDir(), "nope.db")}
	rep := c.Run(context.Background())
	for _, r := range rep.Results {
		if r.Name == "vault" {
			if r.Status != StatusError {
				t.Fatalf("want Error for missing file, got %q", r.Status)
			}
			if r.Hint == "" {
				t.Fatalf("want remediation hint")
			}
			return
		}
	}
	t.Fatal("no vault result")
}

func TestChecker_VaultOKWhenPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "vault.db")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	c := &Checker{VaultPath: path}
	rep := c.Run(context.Background())
	for _, r := range rep.Results {
		if r.Name == "vault" {
			if r.Status != StatusOK {
				t.Fatalf("want OK, got %q: %s", r.Status, r.Detail)
			}
			return
		}
	}
	t.Fatal("no vault result")
}

func TestChecker_AccountsNone(t *testing.T) {
	c := &Checker{AccountsHint: func() int { return 0 }}
	rep := c.Run(context.Background())
	for _, r := range rep.Results {
		if r.Name == "accounts" {
			if r.Status != StatusError {
				t.Fatalf("want Error when 0 accounts, got %q", r.Status)
			}
			return
		}
	}
	t.Fatal("no accounts result")
}

func TestChecker_AccountsWarn(t *testing.T) {
	c := &Checker{AccountsHint: func() int { return 1 }}
	rep := c.Run(context.Background())
	for _, r := range rep.Results {
		if r.Name == "accounts" {
			if r.Status != StatusWarn {
				t.Fatalf("want Warn for 1 account, got %q", r.Status)
			}
			return
		}
	}
}

func TestChecker_AccountsOK(t *testing.T) {
	c := &Checker{AccountsHint: func() int { return 5 }}
	rep := c.Run(context.Background())
	for _, r := range rep.Results {
		if r.Name == "accounts" && r.Status != StatusOK {
			t.Fatalf("want OK for 5 accounts, got %q", r.Status)
		}
	}
}

func TestChecker_UpstreamTimeout(t *testing.T) {
	// 203.0.113.1 is TEST-NET-3 — guaranteed unreachable.
	c := &Checker{UpstreamURL: "https://203.0.113.1:443", HTTPTimeout: 200 * time.Millisecond}
	rep := c.Run(context.Background())
	for _, r := range rep.Results {
		if r.Name == "upstream" {
			if r.Status != StatusError {
				t.Fatalf("want Error for unreachable host, got %q: %s", r.Status, r.Detail)
			}
			return
		}
	}
	t.Fatal("no upstream result")
}

func TestReport_OKAggregate(t *testing.T) {
	rep := &Report{
		Results: []Result{
			{Name: "a", Status: StatusOK},
			{Name: "b", Status: StatusWarn},
			{Name: "c", Status: StatusSkip},
		},
	}
	if !allOK(rep.Results) {
		t.Fatalf("warn/skip should count as OK aggregate")
	}
	rep.Results = append(rep.Results, Result{Name: "d", Status: StatusError})
	if allOK(rep.Results) {
		t.Fatalf("error must flip OK=false")
	}
}

func TestReport_Format(t *testing.T) {
	rep := &Report{
		StartedAt: time.Now(),
		Elapsed:   "10ms",
		Results: []Result{
			{Name: "runtime", Status: StatusOK, Detail: "all good"},
			{Name: "vault", Status: StatusError, Detail: "missing", Hint: "run add-account"},
		},
	}
	out := rep.Format()
	if out == "" {
		t.Fatal("want non-empty formatted output")
	}
}
