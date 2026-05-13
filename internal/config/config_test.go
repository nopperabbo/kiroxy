package config

import (
	"testing"
)

// TestKiroUpstreamURL_EnvOverride verifies that KIROXY_UPSTREAM_URL is
// picked up by FromEnvAndFlags and surfaced on Config.KiroUpstreamURL.
// Downstream, main.go wires this into kiroclient.WithBaseURL.
func TestKiroUpstreamURL_EnvOverride(t *testing.T) {
	tests := []struct {
		name string
		set  string
		want string
	}{
		{"unset defaults to empty", "", ""},
		{"explicit override", "http://127.0.0.1:9999/", "http://127.0.0.1:9999/"},
		{"mock_kiro URL", "http://localhost:4000", "http://localhost:4000"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.set == "" {
				t.Setenv("KIROXY_UPSTREAM_URL", "")
			} else {
				t.Setenv("KIROXY_UPSTREAM_URL", tc.set)
			}
			cfg, err := FromEnvAndFlags(nil)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.KiroUpstreamURL != tc.want {
				t.Errorf("KiroUpstreamURL = %q, want %q", cfg.KiroUpstreamURL, tc.want)
			}
		})
	}
}
