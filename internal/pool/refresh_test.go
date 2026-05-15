package pool

import (
	"testing"
	"time"
)

func TestNeedsRefresh_Boundaries(t *testing.T) {
	skew := 5 * time.Minute
	now := time.Now()
	tests := []struct {
		name string
		meta accountMetadata
		want bool
	}{
		{"zero expires_at means fresh", accountMetadata{AuthMethod: "social", ExpiresAt: 0}, false},
		{"expired 1h ago", accountMetadata{AuthMethod: "social", ExpiresAt: now.Add(-1 * time.Hour).Unix()}, true},
		{"expires in 1h", accountMetadata{AuthMethod: "social", ExpiresAt: now.Add(1 * time.Hour).Unix()}, false},
		{"expires in 1min (inside skew)", accountMetadata{AuthMethod: "social", ExpiresAt: now.Add(1 * time.Minute).Unix()}, true},
		{"expires in 4m59s (inside skew)", accountMetadata{AuthMethod: "social", ExpiresAt: now.Add(4*time.Minute + 59*time.Second).Unix()}, true},
		{"expires in 5m1s (outside skew)", accountMetadata{AuthMethod: "social", ExpiresAt: now.Add(5*time.Minute + 1*time.Second).Unix()}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := needsRefresh(tc.meta, skew, now)
			if got != tc.want {
				t.Errorf("needsRefresh = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestParseAccountMetadata_Tolerant(t *testing.T) {
	tests := []struct {
		name      string
		in        string
		wantAuth  string
		wantExpAt int64
	}{
		{"empty", "", "", 0},
		{"malformed", "not json", "", 0},
		{"social with expires_at", `{"auth_method":"social","expires_at":12345}`, "social", 12345},
		{"fallback from added_at+expires_in", `{"auth_method":"social","added_at":"2026-05-12T14:39:18","expires_in":3600}`, "social", 1778600358},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			md := parseAccountMetadata(tc.in)
			if md.AuthMethod != tc.wantAuth {
				t.Errorf("AuthMethod = %q, want %q", md.AuthMethod, tc.wantAuth)
			}
			if tc.wantExpAt != 0 && md.ExpiresAt != tc.wantExpAt {
				t.Errorf("ExpiresAt = %d, want %d", md.ExpiresAt, tc.wantExpAt)
			}
		})
	}
}

// TestParseAccountMetadata_IdCFields locks in the Phase 6.3 scaffold: the
// parser MUST extract client_id, client_secret, and region from a Builder ID
// OAuth bundle's metadata JSON (matching what cmd/kiroxy/accounts.go writes
// at addAccountViaOAuth). Existing social bundles must produce zero-valued
// IdC fields — guards against regression where a parser tweak silently
// breaks the future RefreshOIDC plumbing path.
func TestParseAccountMetadata_IdCFields(t *testing.T) {
	// Real shape produced by addAccountViaOAuth (cmd/kiroxy/accounts.go:123).
	idcRaw := `{"client_id":"abc-client","client_secret":"shh-secret","region":"us-west-2","source":"builder-id-oauth","auth_method":"idc"}`
	md := parseAccountMetadata(idcRaw)
	if md.AuthMethod != "idc" {
		t.Errorf("AuthMethod = %q, want %q", md.AuthMethod, "idc")
	}
	if md.ClientID != "abc-client" {
		t.Errorf("ClientID = %q, want %q", md.ClientID, "abc-client")
	}
	if md.ClientSecret != "shh-secret" {
		t.Errorf("ClientSecret = %q, want %q", md.ClientSecret, "shh-secret")
	}
	if md.SSORegion != "us-west-2" {
		t.Errorf("SSORegion = %q, want %q", md.SSORegion, "us-west-2")
	}

	// Social bundles must NOT carry IdC fields — they're optional, omitted by
	// social onboarding, and parsing must produce zero-values. If this ever
	// regresses, RefreshOIDC plumbing would route social accounts through the
	// IdC code path and explode.
	socialRaw := `{"auth_method":"social","profile_arn":"arn:aws:codewhisperer:foo","expires_at":99999}`
	smd := parseAccountMetadata(socialRaw)
	if smd.AuthMethod != "social" {
		t.Errorf("social AuthMethod = %q, want %q", smd.AuthMethod, "social")
	}
	if smd.ClientID != "" || smd.ClientSecret != "" || smd.SSORegion != "" {
		t.Errorf("social bundle leaked IdC fields: ClientID=%q ClientSecret=%q SSORegion=%q",
			smd.ClientID, smd.ClientSecret, smd.SSORegion)
	}
}
