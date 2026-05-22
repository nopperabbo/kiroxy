package main

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/nopperabbo/kiroxy/internal/pool"
)

func TestParseSlogLevel(t *testing.T) {
	tests := []struct {
		input string
		want  slog.Level
		err   bool
	}{
		{"debug", slog.LevelDebug, false},
		{"DEBUG", slog.LevelDebug, false},
		{"info", slog.LevelInfo, false},
		{"  Info  ", slog.LevelInfo, false},
		{"warn", slog.LevelWarn, false},
		{"warning", slog.LevelWarn, false},
		{"error", slog.LevelError, false},
		{"ERROR", slog.LevelError, false},
		{"", 0, true},
		{"trace", 0, true},
		{"fatal", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := parseSlogLevel(tt.input)
			if tt.err {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCurrentLogLevelString_NilLevel(t *testing.T) {
	s := &settingsProvider{logLevel: nil}
	if got := s.currentLogLevelString(); got != "" {
		t.Errorf("want empty, got %q", got)
	}
}

func TestCurrentLogLevelString_Set(t *testing.T) {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelWarn)
	s := &settingsProvider{logLevel: lv}
	if got := s.currentLogLevelString(); got != "warn" {
		t.Errorf("want 'warn', got %q", got)
	}
}

func TestUpdateLogLevel_NilLevel(t *testing.T) {
	s := &settingsProvider{logLevel: nil}
	err := s.UpdateLogLevel(context.Background(), "debug")
	if err == nil {
		t.Fatal("expected error for nil logLevel")
	}
}

func TestUpdateLogLevel_InvalidLevel(t *testing.T) {
	lv := new(slog.LevelVar)
	s := &settingsProvider{logLevel: lv}
	err := s.UpdateLogLevel(context.Background(), "bogus")
	if err == nil {
		t.Fatal("expected error for invalid level")
	}
}

func TestUpdateLogLevel_Success(t *testing.T) {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelInfo)
	s := &settingsProvider{logLevel: lv}
	if err := s.UpdateLogLevel(context.Background(), "error"); err != nil {
		t.Fatal(err)
	}
	if lv.Level() != slog.LevelError {
		t.Errorf("level = %v, want ERROR", lv.Level())
	}
}

func TestSettings_NilVaultAndPool(t *testing.T) {
	lv := new(slog.LevelVar)
	lv.Set(slog.LevelInfo)
	s := &settingsProvider{
		version:   "1.0.0",
		vaultPath: "",
		logLevel:  lv,
		startedAt: time.Now().Add(-10 * time.Second),
	}
	snap := s.Settings(context.Background())
	if snap.General.Version != "1.0.0" {
		t.Errorf("version = %q", snap.General.Version)
	}
	if snap.General.LogLevel != "info" {
		t.Errorf("logLevel = %q", snap.General.LogLevel)
	}
	if snap.General.UptimeS < 10 {
		t.Errorf("uptime = %d, want >= 10", snap.General.UptimeS)
	}
}

func TestSettings_WithPool(t *testing.T) {
	lv := new(slog.LevelVar)
	p := pool.New(pool.DefaultPolicy())
	p.Add(pool.Account{ID: "a1", Enabled: true})
	p.Add(pool.Account{ID: "a2", Enabled: false})
	s := &settingsProvider{
		version:   "2.0.0",
		logLevel:  lv,
		pool:      p,
		startedAt: time.Now(),
	}
	snap := s.Settings(context.Background())
	if snap.Vault.Total != 2 {
		t.Errorf("total = %d, want 2", snap.Vault.Total)
	}
	if snap.Vault.Healthy != 1 {
		t.Errorf("healthy = %d, want 1", snap.Vault.Healthy)
	}
	if snap.Vault.Disabled != 1 {
		t.Errorf("disabled = %d, want 1", snap.Vault.Disabled)
	}
}
