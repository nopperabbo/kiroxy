package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nopperabbo/kiroxy/internal/tokenvault"
)

// mintJWT builds an unsigned test JWT (header.payload.) from the given
// claims. Mirrors tools/onboard/test_oauth.py::_mint_jwt so the Go/Python
// cascades remain observably consistent.
func mintJWT(t *testing.T, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none","typ":"JWT"}`))
	payload, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal claims: %v", err)
	}
	body := base64.RawURLEncoding.EncodeToString(payload)
	return header + "." + body + "."
}

func TestDeriveAccountID_PriorityCascade(t *testing.T) {
	workspaceARN := "arn:aws:codewhisperer:us-east-1:111222/WORKSPACE_SHARED"
	jwtWithEmail := mintJWT(t, map[string]any{"email": "alice@jwt.example.com"})
	jwtWithSubOnly := mintJWT(t, map[string]any{"sub": "uuid-42"})

	tests := []struct {
		name       string
		entry      kiroTokenEntry
		wantID     string
		wantSource string
	}{
		{
			name: "P1 email wins over everything",
			entry: kiroTokenEntry{
				Email:       "alice@dineu.tech",
				AccessToken: jwtWithEmail,
				ProfileArn:  workspaceARN,
			},
			wantID:     "alice@dineu.tech",
			wantSource: "email",
		},
		{
			name: "P1 email is lowercased and trimmed",
			entry: kiroTokenEntry{
				Email:      "  Alice@Dineu.Tech  ",
				ProfileArn: workspaceARN,
			},
			wantID:     "alice@dineu.tech",
			wantSource: "email",
		},
		{
			name: "P2 JWT email claim used when entry.Email absent",
			entry: kiroTokenEntry{
				AccessToken: jwtWithEmail,
				ProfileArn:  workspaceARN,
			},
			wantID:     "alice@jwt.example.com",
			wantSource: "jwt_sub",
		},
		{
			name: "P2 JWT sub claim used when no email claim",
			entry: kiroTokenEntry{
				AccessToken: jwtWithSubOnly,
				ProfileArn:  workspaceARN,
			},
			wantID:     "uuid-42",
			wantSource: "jwt_sub",
		},
		{
			name: "P3 profileArn last segment when no email and opaque token",
			entry: kiroTokenEntry{
				// Opaque Kiro token (real shape): no dots, not a JWT.
				AccessToken: "aoa-opaque-no-dots-here",
				ProfileArn:  workspaceARN,
			},
			wantID:     "WORKSPACE_SHARED",
			wantSource: "profileArn",
		},
		{
			name: "P3 profileArn without slash returns full arn",
			entry: kiroTokenEntry{
				ProfileArn: "no-slash-arn",
			},
			wantID:     "no-slash-arn",
			wantSource: "profileArn_full",
		},
		{
			name: "P4 accessToken prefix when everything else missing",
			entry: kiroTokenEntry{
				AccessToken: "aoa-prefix-xyz-longer",
			},
			wantID:     "aoa-prefix-x",
			wantSource: "accessToken_prefix",
		},
		{
			name: "P4 short accessToken returned whole",
			entry: kiroTokenEntry{
				AccessToken: "short",
			},
			wantID:     "short",
			wantSource: "accessToken_prefix",
		},
		{
			name:       "empty entry yields empty id",
			entry:      kiroTokenEntry{},
			wantID:     "",
			wantSource: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, src := deriveAccountID(tc.entry)
			if id != tc.wantID {
				t.Errorf("id: want %q, got %q", tc.wantID, id)
			}
			if src != tc.wantSource {
				t.Errorf("source: want %q, got %q", tc.wantSource, src)
			}
		})
	}
}

func TestDeriveAccountID_WorkspaceCollisionRegression(t *testing.T) {
	// BUG 4 regression: two Workspace users with the SAME profileArn must
	// derive DIFFERENT ids when they supply distinct emails. Before the
	// cascade landed, both would collapse to the same profileArn segment.
	shared := "arn:aws:codewhisperer:us-east-1:111222/WORKSPACE_SHARED"

	id1, src1 := deriveAccountID(kiroTokenEntry{
		Email: "user1@dineu.tech", ProfileArn: shared, AccessToken: "aoa-one",
	})
	id2, src2 := deriveAccountID(kiroTokenEntry{
		Email: "user2@dineu.tech", ProfileArn: shared, AccessToken: "aoa-two",
	})

	if id1 == id2 {
		t.Fatalf("BUG 4 regression: both users collapsed to id=%q (source=%s)", id1, src1)
	}
	if src1 != "email" || src2 != "email" {
		t.Errorf("expected both sources to be 'email', got %q and %q", src1, src2)
	}
}

func TestJwtSubOrEmail_EdgeCases(t *testing.T) {
	emailJWT := mintJWT(t, map[string]any{"email": "a@b.com", "sub": "uuid"})
	subJWT := mintJWT(t, map[string]any{"sub": "uuid-only"})
	emptyClaimsJWT := mintJWT(t, map[string]any{"email": "   ", "sub": ""})
	unrelatedJWT := mintJWT(t, map[string]any{"iss": "kiro", "aud": "prod"})

	tests := []struct {
		name  string
		token string
		want  string
	}{
		{"empty token", "", ""},
		{"opaque no-dots kiro token", "aoaDummyTokenNoDotsHere", ""},
		{"two-segment", "hdr.body", ""},
		{"four-segment", "a.b.c.d", ""},
		{"bad base64 in payload", "hdr.!!!.sig", ""},
		{"email claim wins", emailJWT, "a@b.com"},
		{"sub fallback", subJWT, "uuid-only"},
		{"empty email + empty sub", emptyClaimsJWT, ""},
		{"unrelated claims", unrelatedJWT, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := jwtSubOrEmail(tc.token)
			if got != tc.want {
				t.Errorf("want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestJwtSubOrEmail_AcceptsPaddedPayload(t *testing.T) {
	// Some JWT emitters include '=' padding on the payload. RawURLEncoding
	// refuses padding; our fallback to URLEncoding should recover.
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"none"}`))
	// Craft a claims JSON whose encoded form needs '=' padding. 11-byte
	// input (`{"sub":"x"}`) base64-encodes to "eyJzdWIiOiJ4In0=" — exactly
	// one '=' pad char.
	raw := []byte(`{"sub":"x"}`)
	payload := base64.URLEncoding.EncodeToString(raw)
	if !strings.HasSuffix(payload, "=") {
		t.Fatalf("test precondition: payload %q should have '=' padding", payload)
	}
	tok := header + "." + payload + ".sig"
	if got := jwtSubOrEmail(tok); got != "x" {
		t.Errorf("padded JWT payload rejected: got %q, want %q", got, "x")
	}
}

func TestTokenHeadForCompare(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"", ""},
		{"   ", ""},
		{"short", "short"},
		{"1234567890123456", "1234567890123456"},
		{"this-is-a-much-longer-token-than-sixteen", "this-is-a-much-l"},
	}
	for _, tc := range tests {
		if got := tokenHeadForCompare(tc.in); got != tc.want {
			t.Errorf("tokenHeadForCompare(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestRunImportAccountsJSON_CollisionDetection verifies the new collision
// safeguard: two entries with the SAME id but DIFFERENT access tokens must
// be blocked without -allow-overwrite.
func TestRunImportAccountsJSON_CollisionDetection(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	t.Setenv("KIROXY_DB_PATH", filepath.Join(dir, "tokens.db"))

	jsonPath1 := filepath.Join(dir, "first.json")
	jsonPath2 := filepath.Join(dir, "second.json")

	first := `[{
		"provider": "Google", "authMethod": "social",
		"email": "collide@dineu.tech",
		"accessToken": "aoa-FIRST-token-xxxxxxxxxxxx",
		"refreshToken": "aor-FIRST-refresh-xxxxxxxxxxxx",
		"profileArn": "arn:aws:codewhisperer:us-east-1:x/SHARED",
		"expiresIn": 3600, "addedAt": "2026-05-13T00:00:00Z"
	}]`
	second := `[{
		"provider": "Google", "authMethod": "social",
		"email": "collide@dineu.tech",
		"accessToken": "aoa-SECOND-different-token-yyyyyyyyyy",
		"refreshToken": "aor-SECOND-refresh-yyyyyyyyyyyy",
		"profileArn": "arn:aws:codewhisperer:us-east-1:x/SHARED",
		"expiresIn": 3600, "addedAt": "2026-05-13T00:00:00Z"
	}]`

	if err := writeTestFile(jsonPath1, first); err != nil {
		t.Fatal(err)
	}
	if err := writeTestFile(jsonPath2, second); err != nil {
		t.Fatal(err)
	}

	// Initial import: should succeed.
	if err := runImportAccountsJSON(ctx, []string{"-file", jsonPath1, "-provider", "kiro"}); err != nil {
		t.Fatalf("initial import: %v", err)
	}

	// Second import without -allow-overwrite: must be skipped, not saved.
	if err := runImportAccountsJSON(ctx, []string{"-file", jsonPath2, "-provider", "kiro"}); err != nil {
		t.Fatalf("second import returned error (should succeed but skip): %v", err)
	}

	// Verify vault still has the FIRST token (not overwritten).
	v, err := tokenvault.Open(ctx, filepath.Join(dir, "tokens.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()
	b, err := v.Get(ctx, "kiro", "collide@dineu.tech")
	if err != nil {
		t.Fatalf("entry missing after collision-protected import: %v", err)
	}
	if !strings.HasPrefix(b.AccessToken, "aoa-FIRST") {
		t.Errorf("collision safeguard failed: token was overwritten to %q", b.AccessToken)
	}

	// Third import WITH -allow-overwrite: must rotate in place.
	if err := runImportAccountsJSON(ctx, []string{
		"-file", jsonPath2, "-provider", "kiro", "-allow-overwrite",
	}); err != nil {
		t.Fatalf("overwrite import: %v", err)
	}
	b2, err := v.Get(ctx, "kiro", "collide@dineu.tech")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(b2.AccessToken, "aoa-SECOND") {
		t.Errorf("overwrite failed to rotate: token is still %q", b2.AccessToken)
	}
}

// TestRunImportAccountsJSON_WorkspaceTwoUsersBothLand verifies the end-to-end
// fix for BUG 4: importing two Workspace users sharing a profileArn must
// result in two distinct vault entries, not one overwriting the other.
func TestRunImportAccountsJSON_WorkspaceTwoUsersBothLand(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	t.Setenv("KIROXY_DB_PATH", filepath.Join(dir, "tokens.db"))

	jsonPath := filepath.Join(dir, "workspace.json")
	payload := `[
		{
			"provider": "Google", "authMethod": "social",
			"email": "user1@dineu.tech",
			"accessToken": "aoa-user1-token-xxxxxxxxxxxx",
			"refreshToken": "aor-user1-refresh-xxxxxxxxxxxx",
			"profileArn": "arn:aws:codewhisperer:us-east-1:x/WORKSPACE_SHARED",
			"expiresIn": 3600, "addedAt": "2026-05-13T00:00:00Z"
		},
		{
			"provider": "Google", "authMethod": "social",
			"email": "user2@dineu.tech",
			"accessToken": "aoa-user2-token-yyyyyyyyyyyy",
			"refreshToken": "aor-user2-refresh-yyyyyyyyyyyy",
			"profileArn": "arn:aws:codewhisperer:us-east-1:x/WORKSPACE_SHARED",
			"expiresIn": 3600, "addedAt": "2026-05-13T00:00:00Z"
		}
	]`
	if err := writeTestFile(jsonPath, payload); err != nil {
		t.Fatal(err)
	}

	if err := runImportAccountsJSON(ctx, []string{"-file", jsonPath, "-provider", "kiro"}); err != nil {
		t.Fatalf("import: %v", err)
	}

	v, err := tokenvault.Open(ctx, filepath.Join(dir, "tokens.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer v.Close()

	b1, err := v.Get(ctx, "kiro", "user1@dineu.tech")
	if err != nil {
		t.Fatalf("user1 missing: %v", err)
	}
	b2, err := v.Get(ctx, "kiro", "user2@dineu.tech")
	if err != nil {
		t.Fatalf("user2 missing: %v", err)
	}
	if b1.AccessToken == b2.AccessToken {
		t.Errorf("BUG 4 regression: both users share access token (collapsed)")
	}
	if !strings.HasPrefix(b1.AccessToken, "aoa-user1") {
		t.Errorf("user1 token mismatch: %q", b1.AccessToken)
	}
	if !strings.HasPrefix(b2.AccessToken, "aoa-user2") {
		t.Errorf("user2 token mismatch: %q", b2.AccessToken)
	}
}

func writeTestFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o600)
}
