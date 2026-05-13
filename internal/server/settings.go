package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// SettingsSnapshot is the bundled payload for /dashboard/api/settings. It
// combines general info, env vars, and vault stats so the SettingsView can
// render all tabs from a single fetch.
type SettingsSnapshot struct {
	General VaultGeneral    `json:"general"`
	EnvVars []EnvVarEntry   `json:"env_vars"`
	Vault   VaultStats      `json:"vault"`
	Inbound InboundKeyStats `json:"inbound_keys"`
}

// VaultGeneral is the "General" tab content: version, uptime, config paths.
type VaultGeneral struct {
	Version   string `json:"version"`
	UptimeS   int64  `json:"uptime_s"`
	StartedAt string `json:"started_at"`
	VaultPath string `json:"vault_path,omitempty"`
	LogLevel  string `json:"log_level,omitempty"`
}

// EnvVarEntry is one KIROXY_* / GOEXPERIMENT env var shown in the Env tab.
// Values for secret-looking keys are redacted to the last 4 chars.
type EnvVarEntry struct {
	Key      string `json:"key"`
	Value    string `json:"value"`
	Redacted bool   `json:"redacted,omitempty"`
	Present  bool   `json:"present"`
}

// VaultStats describes the account pool breakdown + on-disk file size.
type VaultStats struct {
	Path      string `json:"path,omitempty"`
	SizeBytes int64  `json:"size_bytes,omitempty"`
	Healthy   int    `json:"healthy"`
	Cooldown  int    `json:"cooldown"`
	Disabled  int    `json:"disabled"`
	Total     int    `json:"total"`
}

// InboundKeyStats are the counts for the Inbound Keys tab header.
type InboundKeyStats struct {
	Active int `json:"active"`
	Total  int `json:"total"`
}

// SettingsProvider is the data source for /dashboard/api/settings. The
// server calls it once per request; implementations typically cache nothing.
// When nil, the endpoint returns 404.
type SettingsProvider interface {
	Settings(ctx context.Context) SettingsSnapshot
}

// knownEnvVars lists the kiroxy env vars operators touch. Secret-looking
// ones (keys containing KEY / TOKEN / SECRET) have their values redacted
// to the last 4 chars. Non-kiroxy env vars from the parent shell are not
// surfaced here to keep the table small and intentional.
var knownEnvVars = []string{
	"KIROXY_BIND",
	"KIROXY_PORT",
	"KIROXY_API_KEY",
	"KIROXY_DB_PATH",
	"KIROXY_LOG_LEVEL",
	"KIROXY_METRICS_PUBLIC",
	"KIROXY_KIRO_DB_PATH",
	"KIROXY_KIRO_REGION",
	"KIROXY_SHUTDOWN_TIMEOUT",
	"KIROXY_UPSTREAM_URL",
	"KIROCC_MODEL_MAPPINGS",
	"GOEXPERIMENT",
}

// BuildEnvVars returns a redacted snapshot of the known env vars. Exposed
// so cmd/kiroxy/settings.go's provider can reuse the redaction logic.
func BuildEnvVars() []EnvVarEntry {
	out := make([]EnvVarEntry, 0, len(knownEnvVars))
	for _, k := range knownEnvVars {
		v, ok := os.LookupEnv(k)
		entry := EnvVarEntry{Key: k, Present: ok}
		if !ok {
			out = append(out, entry)
			continue
		}
		if isSecretEnvKey(k) {
			entry.Redacted = true
			entry.Value = redactSuffix(v, 4)
		} else {
			entry.Value = v
		}
		out = append(out, entry)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out
}

func isSecretEnvKey(k string) bool {
	u := strings.ToUpper(k)
	return strings.Contains(u, "KEY") || strings.Contains(u, "TOKEN") || strings.Contains(u, "SECRET") || strings.Contains(u, "PASSWORD")
}

// redactSuffix returns '****' + last n chars of v. Empty strings stay empty;
// very short values get fully redacted so a 3-char secret can't be guessed.
func redactSuffix(v string, n int) string {
	if v == "" {
		return ""
	}
	if len(v) <= n {
		return strings.Repeat("*", len(v))
	}
	return "****" + v[len(v)-n:]
}

func (s *Server) registerSettingsHandler(mux *http.ServeMux) {
	if s.opts.SettingsProvider == nil {
		return
	}
	mux.HandleFunc("GET /dashboard/api/settings", s.handleSettings)
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	snap := s.opts.SettingsProvider.Settings(r.Context())
	if snap.General.Version == "" {
		snap.General.Version = s.opts.Version
	}
	if snap.General.StartedAt == "" {
		snap.General.StartedAt = s.startedAt.Format(time.RFC3339)
	}
	if snap.General.UptimeS == 0 {
		snap.General.UptimeS = int64(time.Since(s.startedAt).Seconds())
	}
	if snap.EnvVars == nil {
		snap.EnvVars = BuildEnvVars()
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(snap)
}
