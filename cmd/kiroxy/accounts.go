package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"local/kiroxy/internal/config"
	"local/kiroxy/internal/pool"
	"local/kiroxy/internal/tokenvault"
)

func runAddAccount(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add-account", flag.ContinueOnError)
	var (
		label        = fs.String("label", "", "display label (defaults to a random 8-char id)")
		refreshToken = fs.String("refresh-token", "", "Kiro refresh token (required); if unset kiroxy reads one line from stdin")
		accessToken  = fs.String("access-token", "", "optional initial access token; if empty, a placeholder is stored and refreshed on first use")
		provider     = fs.String("provider", "kiro", "provider name")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	rt := strings.TrimSpace(*refreshToken)
	if rt == "" {
		fmt.Fprint(os.Stderr, "paste Kiro refresh token (one line): ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read stdin: %w", err)
		}
		rt = strings.TrimSpace(line)
	}
	if rt == "" {
		return errors.New("refresh token is required (via --refresh-token or stdin)")
	}

	at := strings.TrimSpace(*accessToken)
	if at == "" {
		at = "placeholder-will-be-refreshed-on-first-use"
	}

	id := strings.TrimSpace(*label)
	if id == "" {
		id = randomID()
	}

	cfg, err := configForCLI()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o700); err != nil {
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

	b, err := vault.Save(ctx, *provider, id, tokenvault.Tokens{
		AccessToken:  at,
		RefreshToken: rt,
		Source:       "add-account",
	})
	if err != nil {
		return fmt.Errorf("save bundle: %w", err)
	}
	fmt.Printf("added account:\n  provider = %s\n  id       = %s\n  gen      = %d\n  updated  = %s\n",
		b.Provider, b.ConnectionID, b.Generation, b.UpdatedAt.Format(time.RFC3339))
	fmt.Println("\nrestart `kiroxy serve` to pick up the new account.")
	return nil
}

func runListAccounts(ctx context.Context, _ []string) error {
	cfg, err := configForCLI()
	if err != nil {
		return err
	}
	vault, err := tokenvault.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}
	defer vault.Close()

	accts, err := vault.ListByProvider(ctx, "kiro")
	if err != nil {
		return err
	}
	if len(accts) == 0 {
		fmt.Println("no accounts; run `kiroxy add-account --refresh-token=<token>` to seed one")
		return nil
	}

	tw := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "PROVIDER\tID\tGEN\tREFRESH_PENDING\tUPDATED")
	for _, a := range accts {
		fmt.Fprintf(tw, "%s\t%s\t%d\t%v\t%s\n",
			a.Provider, a.ConnectionID, a.Generation, a.RefreshInProgress,
			a.UpdatedAt.Format(time.RFC3339))
	}
	return tw.Flush()
}

func runRemoveAccount(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("remove-account", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() < 1 {
		return errors.New("usage: kiroxy remove-account <id>")
	}
	id := fs.Arg(0)

	cfg, err := configForCLI()
	if err != nil {
		return err
	}
	vault, err := tokenvault.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}
	defer vault.Close()

	if err := vault.Delete(ctx, "kiro", id); err != nil {
		return fmt.Errorf("delete: %w", err)
	}
	fmt.Printf("removed account %s\n", id)
	return nil
}

func runStatus(ctx context.Context, _ []string) error {
	cfg, err := configForCLI()
	if err != nil {
		return err
	}
	vault, err := tokenvault.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}
	defer vault.Close()

	accts, err := vault.ListByProvider(ctx, "kiro")
	if err != nil {
		return err
	}

	p := pool.New(pool.DefaultPolicy())
	for _, a := range accts {
		p.Add(pool.Account{
			ID: a.ConnectionID, Label: a.ConnectionID,
			Provider: a.Provider, Region: cfg.KiroRegion, Enabled: true,
		})
	}
	snap := p.List()

	fmt.Printf("kiroxy status\n")
	fmt.Printf("  vault:        %s\n", cfg.DBPath)
	fmt.Printf("  accounts:     %d\n", len(snap))
	fmt.Printf("  server addr:  http://%s:%d  (not currently probed)\n", cfg.Bind, cfg.Port)
	fmt.Printf("  api key set:  %v\n", cfg.APIKey != "")
	if len(snap) == 0 {
		return nil
	}
	fmt.Println()
	tw := tabwriter.NewWriter(os.Stdout, 2, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ID\tENABLED\tREQS\tERRORS\tCOOLDOWN_UNTIL\tLAST_ERROR")
	for _, s := range snap {
		cooldown := "-"
		if !s.CooldownUntil.IsZero() && s.CooldownUntil.After(time.Now()) {
			cooldown = s.CooldownUntil.Format(time.RFC3339)
		}
		fmt.Fprintf(tw, "%s\t%v\t%d\t%d\t%s\t%s\n",
			s.ID, s.Enabled, s.RequestCount, s.ErrorCount, cooldown, s.LastError)
	}
	return tw.Flush()
}

func configForCLI() (config.Config, error) {
	return config.FromEnvAndFlags(nil)
}

func randomID() string {
	var buf [4]byte
	_, _ = rand.Read(buf[:])
	return hex.EncodeToString(buf[:])
}
