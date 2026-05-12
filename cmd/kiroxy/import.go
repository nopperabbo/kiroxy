package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/mail"
	"os"
	"strings"

	"local/kiroxy/internal/tokenvault"
)

type importOutcome int

const (
	outcomeAdded importOutcome = iota + 1
	outcomeUpdated
	outcomeSkipped
)

type importReport struct {
	added   int
	updated int
	skipped int
	reasons []string
}

func runImportAccounts(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import-accounts", flag.ContinueOnError)
	var (
		file      = fs.String("file", "", "path to line-delimited triplet file (email:refresh_token:signature)")
		fromStdin = fs.Bool("stdin", false, "read triplets from stdin instead of --file")
		provider  = fs.String("provider", "kiro", "provider name")
		dry       = fs.Bool("dry-run", false, "parse + validate only; do not write to vault")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	var src io.Reader
	switch {
	case *fromStdin:
		src = os.Stdin
	case *file != "":
		f, err := os.Open(*file)
		if err != nil {
			return fmt.Errorf("open %s: %w", *file, err)
		}
		defer f.Close()
		src = f
	default:
		return errors.New("either --file=<path> or --stdin must be provided")
	}

	triplets, parseErrs := parseTriplets(src)
	if len(triplets) == 0 && len(parseErrs) == 0 {
		return errors.New("no triplets found (empty input)")
	}

	report := importReport{}
	for _, e := range parseErrs {
		report.skipped++
		report.reasons = append(report.reasons, e)
	}

	if *dry {
		for _, t := range triplets {
			fmt.Printf("  would import: email=%s  rt=%s  sig_len=%d\n",
				t.email, redact(t.refreshToken), len(t.signature))
		}
		fmt.Printf("\ndry-run: %d valid triplets, %d skipped\n", len(triplets), report.skipped)
		for _, r := range report.reasons {
			fmt.Printf("  skipped: %s\n", r)
		}
		return nil
	}

	cfg, err := configForCLI()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(parentDir(cfg.DBPath), 0o700); err != nil {
		return fmt.Errorf("mkdir vault parent: %w", err)
	}
	vault, err := tokenvault.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}
	defer vault.Close()
	if err := os.Chmod(cfg.DBPath, 0o600); err != nil {
		fmt.Fprintf(os.Stderr, "warn: chmod %s: %v\n", cfg.DBPath, err)
	}

	for _, t := range triplets {
		outcome, err := importOne(ctx, vault, *provider, t)
		if err != nil {
			report.skipped++
			report.reasons = append(report.reasons, fmt.Sprintf("%s: %v", t.email, err))
			continue
		}
		switch outcome {
		case outcomeAdded:
			report.added++
		case outcomeUpdated:
			report.updated++
			fmt.Fprintf(os.Stderr, "warn: %s already existed, refresh_token updated in-place\n", t.email)
		}
	}

	totalInput := len(triplets) + report.skipped
	fmt.Printf("imported %d/%d (added=%d updated=%d skipped=%d)\n",
		report.added+report.updated, totalInput,
		report.added, report.updated, report.skipped)
	for _, r := range report.reasons {
		fmt.Printf("  skipped: %s\n", r)
	}
	if report.added+report.updated > 0 {
		fmt.Println("\nrestart `kiroxy serve` to pick up the imported accounts.")
	}
	return nil
}

type triplet struct {
	email        string
	refreshToken string
	signature    string
	lineNo       int
}

func parseTriplets(r io.Reader) ([]triplet, []string) {
	var (
		out  []triplet
		errs []string
	)
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	seenEmail := map[string]bool{}
	line := 0
	for scanner.Scan() {
		line++
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" || strings.HasPrefix(raw, "#") {
			continue
		}
		parts := strings.SplitN(raw, ":", 3)
		if len(parts) < 2 {
			errs = append(errs, fmt.Sprintf("line %d: expected at least email:refresh_token, got %q", line, truncate(raw, 40)))
			continue
		}
		email := strings.TrimSpace(parts[0])
		refresh := strings.TrimSpace(parts[1])
		sig := ""
		if len(parts) == 3 {
			sig = strings.TrimSpace(parts[2])
		}

		if _, err := mail.ParseAddress(email); err != nil {
			errs = append(errs, fmt.Sprintf("line %d: invalid email %q: %v", line, email, err))
			continue
		}
		if len(refresh) < 8 {
			errs = append(errs, fmt.Sprintf("line %d: refresh_token too short (got %d chars)", line, len(refresh)))
			continue
		}
		if seenEmail[email] {
			errs = append(errs, fmt.Sprintf("line %d: email %s appears multiple times in file; keeping first", line, email))
			continue
		}
		seenEmail[email] = true

		out = append(out, triplet{
			email:        email,
			refreshToken: refresh,
			signature:    sig,
			lineNo:       line,
		})
	}
	if err := scanner.Err(); err != nil {
		errs = append(errs, fmt.Sprintf("read input: %v", err))
	}
	return out, errs
}

func importOne(ctx context.Context, vault *tokenvault.Vault, provider string, t triplet) (importOutcome, error) {
	existing, err := vault.Get(ctx, provider, t.email)
	isNew := errors.Is(err, tokenvault.ErrNotFound)
	if err != nil && !isNew {
		return 0, err
	}

	md, _ := json.Marshal(map[string]any{
		"source":    "import-accounts",
		"signature": t.signature,
	})

	tokens := tokenvault.Tokens{
		AccessToken:  "placeholder-will-be-refreshed-on-first-use",
		RefreshToken: t.refreshToken,
		Source:       "import-accounts",
		Metadata:     string(md),
	}
	if _, err := vault.Save(ctx, provider, t.email, tokens); err != nil {
		return 0, err
	}
	if isNew || existing == nil {
		return outcomeAdded, nil
	}
	return outcomeUpdated, nil
}

func redact(s string) string {
	if len(s) <= 8 {
		return strings.Repeat("*", len(s))
	}
	return s[:4] + "..." + s[len(s)-4:]
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func parentDir(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i <= 0 {
		return "."
	}
	return p[:i]
}
