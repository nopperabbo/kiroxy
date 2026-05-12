// Package main is the kiroxy entrypoint.
//
// kiroxy is a single-user, self-hosted proxy that exposes the user's Kiro IDE
// subscription (Amazon Q Developer / AWS CodeWhisperer) as an Anthropic Messages
// API-compatible endpoint.
//
// The command-line surface is:
//
//	kiroxy serve         run the HTTP proxy (default command)
//	kiroxy add-account   register a new Kiro account via OAuth (see M9)
//	kiroxy list-accounts show the account pool (see M9)
//	kiroxy remove-account <id>
//	kiroxy status        print pool + request stats (see M9)
//
// See ../BUILD_PLAN.md for milestone detail.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"local/kiroxy/internal/auth"
	"local/kiroxy/internal/config"
	"local/kiroxy/internal/kiroclient"
	"local/kiroxy/internal/messages"
	"local/kiroxy/internal/pool"
	"local/kiroxy/internal/server"
	"local/kiroxy/internal/tokenvault"
)

// version is overridden at build time via -ldflags "-X main.version=<tag>"
// (see Makefile). It must be a var (not const) for -X to take effect.
var version = "dev"

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			os.Exit(0)
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run is the testable entrypoint. args excludes the program name.
func run(ctx context.Context, args []string) error {
	// Top-level shortcuts: --version / -version / -v / --help / -h / help.
	// These beat the subcommand dispatch so that flag-looking invocations
	// don't get swallowed by the default "serve" path.
	if len(args) == 1 {
		switch args[0] {
		case "--version", "-version", "-v":
			fmt.Println(version)
			return nil
		case "--help", "-h":
			printHelp()
			return nil
		}
	}

	sub := "serve"
	rest := args
	if len(args) > 0 && !startsWithDash(args[0]) {
		sub = args[0]
		rest = args[1:]
	}

	switch sub {
	case "serve":
		return runServe(ctx, rest)
	case "version", "-v", "--version", "-version":
		fmt.Println(version)
		return nil
	case "add-account":
		return runAddAccount(ctx, rest)
	case "debug-refresh":
		return runDebugRefresh(ctx, rest)
	case "import-accounts":
		return runImportAccounts(ctx, rest)
	case "list-accounts":
		return runListAccounts(ctx, rest)
	case "remove-account":
		return runRemoveAccount(ctx, rest)
	case "status":
		return runStatus(ctx, rest)
	case "help", "-h", "--help":
		printHelp()
		return nil
	default:
		return fmt.Errorf("unknown subcommand %q; try: serve, add-account, import-accounts, list-accounts, remove-account, status, version, help", sub)
	}
}

func printHelp() {
	fmt.Println("kiroxy \u2014 self-hosted Kiro-to-Anthropic proxy")
	fmt.Println()
	fmt.Println("usage: kiroxy <command> [flags]")
	fmt.Println()
	fmt.Println("commands:")
	fmt.Println("  serve                  run the HTTP proxy (default)")
	fmt.Println("  add-account            store a single Kiro refresh token in the vault")
	fmt.Println("  import-accounts        bulk-import email:refresh_token[:signature] triplets (--file or --stdin)")
	fmt.Println("  list-accounts          list accounts in the vault")
	fmt.Println("  remove-account <id>    delete an account")
	fmt.Println("  status                 show pool+vault state")
	fmt.Println("  version                print kiroxy version")
	fmt.Println("  help                   this message")
	fmt.Println("\nenv vars: see .env.example and README.md")
}

func startsWithDash(s string) bool {
	return len(s) > 0 && s[0] == '-'
}

func runServe(ctx context.Context, args []string) error {
	cfg, err := config.FromEnvAndFlags(args)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	// TODO(M7): swap for structured json handler with request_id middleware.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel(),
	})))

	var (
		authMgr    messages.TokenGetter
		kiroClient kiroclient.Client
		vault      *tokenvault.Vault
		poolInst   *pool.Pool
	)
	if cfg.KiroDBPath != "" {
		authMgr = auth.NewAuthManager(cfg.KiroDBPath)
		kiroClient = kiroclient.NewHTTPClient(
			kiroclient.WithTokenRefresher(func(ctx context.Context) (string, error) {
				authMgr.(*auth.AuthManager).InvalidateCache()
				creds, err := authMgr.GetToken(ctx)
				if err != nil {
					return "", err
				}
				return creds.AccessToken, nil
			}),
		)
		slog.Info("upstream auth: kiro-cli SQLite",
			slog.String("db_path", cfg.KiroDBPath),
		)
	} else {
		if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0o700); err != nil {
			return fmt.Errorf("mkdir vault parent: %w", err)
		}
		v, err := tokenvault.Open(ctx, cfg.DBPath)
		if err != nil {
			return fmt.Errorf("open token vault: %w", err)
		}
		if err := os.Chmod(cfg.DBPath, 0o600); err != nil {
			slog.Warn("chmod vault db", slog.String("err", err.Error()))
		}
		vault = v
		poolInst = pool.New(pool.DefaultPolicy())

		accts, err := vault.ListByProvider(ctx, "kiro")
		if err != nil {
			return fmt.Errorf("list vault accounts: %w", err)
		}
		for _, a := range accts {
			poolInst.Add(pool.Account{
				ID:       a.ConnectionID,
				Label:    a.ConnectionID,
				Provider: a.Provider,
				Region:   cfg.KiroRegion,
				Enabled:  true,
			})
		}
		slog.Info("upstream auth: pool+tokenvault",
			slog.String("db_path", cfg.DBPath),
			slog.Int("account_count", poolInst.Count()),
		)
		if poolInst.Count() == 0 {
			slog.Warn("no accounts in token vault; /v1/messages will return 503 until 'kiroxy add-account' runs (M9)")
		}
		authMgr = &pool.TokenGetter{Pool: poolInst, Vault: vault}
		kiroClient = kiroclient.NewHTTPClient()
	}

	srv := server.New(server.Options{
		Version:         version,
		Auth:            authMgr,
		KiroClient:      kiroClient,
		APIKey:          cfg.APIKey,
		Logger:          slog.Default(),
		ReadinessChecks: buildReadinessChecks(vault, poolInst),
		DashboardStateProvider: &dashboardProvider{
			version:   version,
			vaultPath: cfg.DBPath,
			vault:     vault,
			pool:      poolInst,
			startedAt: time.Now(),
		},
	})

	addr := net.JoinHostPort(cfg.Bind, strconv.Itoa(cfg.Port))
	httpSrv := &http.Server{
		Addr:    addr,
		Handler: srv.Handler(),

		// Kirocc pattern: ReadHeaderTimeout guards against slowloris; IdleTimeout is
		// generous because our clients hold long-running SSE connections.
		// WriteTimeout is INTENTIONALLY unset — SSE streams can last minutes; a fixed
		// WriteTimeout would decapitate them mid-response.
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}

	slog.Info("kiroxy listening",
		slog.String("version", version),
		slog.String("addr", "http://"+addr),
	)
	if cfg.Bind != "127.0.0.1" && cfg.Bind != "localhost" && cfg.Bind != "::1" {
		slog.Warn("binding to non-loopback address; ensure you have TLS + a reverse proxy in front")
	}

	done := awaitShutdown(ctx, httpSrv, cfg.ShutdownTimeout, vault)

	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	<-done
	return nil
}

// awaitShutdown registers SIGINT/SIGTERM handlers and drains the HTTP server
// with the configured timeout. Derived from d-kuro/kirocc's cmd/kirocc/main.go.
// If vault is non-nil, closes it after the HTTP server stops.
func awaitShutdown(ctx context.Context, httpSrv *http.Server, timeout time.Duration, vault *tokenvault.Vault) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		select {
		case sig := <-sigCh:
			slog.Info("shutdown signal received", slog.String("signal", sig.String()))
		case <-ctx.Done():
			slog.Info("context cancelled", slog.String("err", ctx.Err().Error()))
		}

		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		if err := httpSrv.Shutdown(shutdownCtx); err != nil {
			slog.Error("graceful shutdown failed", slog.String("err", err.Error()))
		}
		if vault != nil {
			if err := vault.Close(); err != nil {
				slog.Error("vault close failed", slog.String("err", err.Error()))
			}
		}
	}()
	return done
}

// silenceUnused keeps the flag import alive; other subcommands use flag.FlagSet.
var _ = flag.ExitOnError

// buildReadinessChecks assembles the /readyz subchecks. We check:
//   - "vault":  SQLite ping
//   - "pool":   at least one account enabled
//
// Upstream reachability is intentionally NOT checked every /readyz poll — a
// DNS fluke or Kiro 5xx shouldn't mark us as unready; we rely on pool health
// tracking (M5) to cool down bad accounts and let the server keep serving.
func buildReadinessChecks(vault *tokenvault.Vault, p *pool.Pool) map[string]server.ReadinessChecker {
	checks := map[string]server.ReadinessChecker{}
	if vault != nil {
		checks["vault"] = func(ctx context.Context) error {
			_, err := vault.ListByProvider(ctx, "kiro")
			return err
		}
	}
	if p != nil {
		checks["pool"] = func(_ context.Context) error {
			if p.Count() == 0 {
				return errors.New("no accounts configured")
			}
			return nil
		}
	}
	return checks
}
