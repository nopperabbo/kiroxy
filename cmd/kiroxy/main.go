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
	"strconv"
	"syscall"
	"time"

	"local/kiroxy/internal/config"
	"local/kiroxy/internal/server"
)

const (
	version = "0.1.0-mvp"
)

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// run is the testable entrypoint. args excludes the program name.
func run(ctx context.Context, args []string) error {
	// Subcommand dispatch. Default (no subcmd) = "serve" for compatibility with
	// `kiroxy` bare invocation.
	sub := "serve"
	rest := args
	if len(args) > 0 && !startsWithDash(args[0]) {
		sub = args[0]
		rest = args[1:]
	}

	switch sub {
	case "serve":
		return runServe(ctx, rest)
	case "version", "-v", "--version":
		fmt.Println(version)
		return nil
	case "add-account", "list-accounts", "remove-account", "status":
		return fmt.Errorf("subcommand %q is reserved for M9; not implemented yet", sub)
	default:
		return fmt.Errorf("unknown subcommand %q; try: serve, version", sub)
	}
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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: cfg.LogLevel(),
	})))

	srv := server.New(server.Options{
		Version: version,
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

	done := awaitShutdown(ctx, httpSrv, cfg.ShutdownTimeout)

	if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("listen: %w", err)
	}
	<-done
	return nil
}

// awaitShutdown registers SIGINT/SIGTERM handlers and drains the HTTP server
// with the configured timeout. Derived from d-kuro/kirocc's cmd/kirocc/main.go.
func awaitShutdown(ctx context.Context, httpSrv *http.Server, timeout time.Duration) <-chan struct{} {
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
	}()
	return done
}

// silenceUnused keeps the flag import alive; other subcommands use flag.FlagSet.
var _ = flag.ExitOnError
