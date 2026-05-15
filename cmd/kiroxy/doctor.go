package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/nopperabbo/kiroxy/internal/config"
	"github.com/nopperabbo/kiroxy/internal/doctor"
	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

// runDoctor executes the doctor health-check subcommand from the CLI.
// Prints a human-readable report by default and exits 1 if any check
// is Error. --json emits machine-readable JSON suitable for scripts.
func runDoctor(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("doctor", flag.ExitOnError)
	asJSON := fs.Bool("json", false, "emit JSON instead of a human-readable report")
	if err := fs.Parse(args); err != nil {
		return err
	}

	cfg, err := config.FromEnvAndFlags(nil)
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	c := &doctor.Checker{
		VaultPath:   cfg.DBPath,
		UpstreamURL: cfg.KiroUpstreamURL,
	}

	if v, err := tokenvault.Open(ctx, cfg.DBPath); err == nil {
		defer v.Close()
		c.AccountsHint = func() int {
			accts, e := v.ListByProvider(ctx, "kiro")
			if e != nil {
				return 0
			}
			return len(accts)
		}
	}

	rep := c.Run(ctx)

	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			return err
		}
	} else {
		fmt.Print(rep.Format())
	}
	if !rep.OK {
		os.Exit(1)
	}
	return nil
}
