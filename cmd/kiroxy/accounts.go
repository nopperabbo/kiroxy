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
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/nopperabbo/kiroxy/internal/builderid"
	"github.com/nopperabbo/kiroxy/internal/config"
	"github.com/nopperabbo/kiroxy/internal/pool"
	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

func runAddAccount(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("add-account", flag.ContinueOnError)
	var (
		label        = fs.String("label", "", "display label (defaults to a random 8-char id)")
		refreshToken = fs.String("refresh-token", "", "paste an existing Kiro refresh token (skips the OAuth flow)")
		accessToken  = fs.String("access-token", "", "optional initial access token; if empty, a placeholder is stored and refreshed on first use")
		provider     = fs.String("provider", "kiro", "provider name")
		region       = fs.String("region", "us-east-1", "AWS region for the OAuth flow")
		open         = fs.Bool("open", true, "open the verification URL in a browser (macOS/Linux)")
		timeout      = fs.Duration("timeout", 5*time.Minute, "OAuth completion timeout")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}

	if strings.TrimSpace(*refreshToken) != "" {
		return addAccountWithRefreshToken(ctx, *label, *refreshToken, *accessToken, *provider)
	}
	return addAccountViaOAuth(ctx, *label, *provider, *region, *open, *timeout)
}

func addAccountWithRefreshToken(ctx context.Context, label, refreshToken, accessToken, provider string) error {
	rt := strings.TrimSpace(refreshToken)
	if rt == "" {
		fmt.Fprint(os.Stderr, "paste Kiro refresh token (one line): ")
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return fmt.Errorf("read stdin: %w", err)
		}
		rt = strings.TrimSpace(line)
	}
	if rt == "" {
		return errors.New("refresh token is required")
	}
	at := strings.TrimSpace(accessToken)
	if at == "" {
		at = "placeholder-will-be-refreshed-on-first-use"
	}
	id := strings.TrimSpace(label)
	if id == "" {
		id = randomID()
	}
	return persistAccount(ctx, provider, id, at, rt, "", "add-account")
}

func addAccountViaOAuth(ctx context.Context, label, provider, region string, doOpen bool, timeout time.Duration) error {
	client := builderid.NewClient(region)
	sess, err := client.Start(ctx)
	if err != nil {
		return fmt.Errorf("start Builder ID flow: %w", err)
	}

	fmt.Println()
	fmt.Println("AWS Builder ID authorization started.")
	fmt.Println()
	fmt.Println("  1. Open this URL in a browser:")
	fmt.Printf("     %s\n", sess.VerificationURI)
	fmt.Println("  2. Confirm the code displayed there matches:")
	fmt.Printf("     %s\n", sess.UserCode)
	fmt.Println("  3. Sign in with your Builder ID and approve access.")
	fmt.Println()
	fmt.Printf("Waiting up to %v for you to complete sign-in...\n", timeout)
	fmt.Println()

	if doOpen {
		_ = openBrowser(sess.VerificationURI)
	}

	pollCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	ticker := 0
	result, err := client.WaitForCompletion(pollCtx, sess, func() {
		ticker++
		if ticker%3 == 0 {
			fmt.Print(".")
		}
	})
	fmt.Println()
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return fmt.Errorf("timed out waiting for sign-in after %v — run `kiroxy add-account` again", timeout)
		}
		return fmt.Errorf("oauth: %w", err)
	}

	id := strings.TrimSpace(label)
	if id == "" {
		id = randomID()
	}
	// auth_method=idc tags this bundle as an IdC/Builder-ID OAuth account so
	// pool dispatch (internal/pool/pool.go) routes it to the IdC code path
	// instead of the social one. Without this tag, the pool falls back to
	// AuthType="kiro" (the provider name) which sends the request to the
	// wrong upstream target and yields opaque 4xx errors. Phase 6.2 root-cause
	// fix — see git log around this commit for the broken-account incident
	// that motivated it.
	metadata := fmt.Sprintf(`{"client_id":%q,"client_secret":%q,"region":%q,"source":"builder-id-oauth","auth_method":"idc"}`,
		sess.ClientID, sess.ClientSecret, sess.Region)
	if err := persistAccountWithMetadata(ctx, provider, id, result.AccessToken, result.RefreshToken, metadata, "builder-id-oauth"); err != nil {
		return err
	}

	fmt.Println("sign-in complete.")
	fmt.Printf("stored account: provider=%s id=%s\n", provider, id)
	fmt.Printf("access token expires in ~%d seconds; kiroxy refreshes automatically.\n", result.ExpiresIn)
	fmt.Println("\nrestart `kiroxy serve` to pick up the new account.")
	return nil
}

func persistAccount(ctx context.Context, provider, id, accessToken, refreshToken, metadata, source string) error {
	return persistAccountWithMetadata(ctx, provider, id, accessToken, refreshToken, metadata, source)
}

func persistAccountWithMetadata(ctx context.Context, provider, id, accessToken, refreshToken, metadata, source string) error {
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
	b, err := vault.Save(ctx, provider, id, tokenvault.Tokens{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		Source:       source,
		Metadata:     metadata,
	})
	if err != nil {
		return fmt.Errorf("save bundle: %w", err)
	}
	if source == "add-account" {
		fmt.Printf("added account:\n  provider = %s\n  id       = %s\n  gen      = %d\n  updated  = %s\n",
			b.Provider, b.ConnectionID, b.Generation, b.UpdatedAt.Format(time.RFC3339))
		fmt.Println("\nrestart `kiroxy serve` to pick up the new account.")
	}
	return nil
}

func openBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return fmt.Errorf("auto-open not supported on %s", runtime.GOOS)
	}
	return cmd.Start()
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
