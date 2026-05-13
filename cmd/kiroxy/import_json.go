package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"local/kiroxy/internal/tokenvault"
)

// kiroTokenEntry is the shape emitted by the Desktop-flow extractor.
// Every entry is one completed OAuth session against
// prod.us-east-1.auth.desktop.kiro.dev/login (redirect_uri
// kiro://kiro.kiroAgent/authenticate-success). accessToken + refreshToken
// are opaque Kiro tokens (aoa.../aor... prefix), profileArn is the scoped
// CodeWhisperer profile for API calls.
//
// Email was added in v1.0.1 to fix BUG 4: Google Workspace accounts within
// the same Kiro org share a profileArn, so profileArn alone cannot serve
// as a unique per-account id. Email comes from the onboarder's CLI flag
// (authoritative) and is the primary dedupe source; profileArn is a
// fallback for legacy JSON files that predate this field.
type kiroTokenEntry struct {
	Provider     string `json:"provider"`
	AuthMethod   string `json:"authMethod"`
	Email        string `json:"email,omitempty"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ProfileArn   string `json:"profileArn"`
	ExpiresIn    int64  `json:"expiresIn"`
	AddedAt      string `json:"addedAt"`
}

func runImportAccountsJSON(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import-accounts-json", flag.ContinueOnError)
	var (
		file           = fs.String("file", "", "path to JSON array of Kiro token entries")
		provider       = fs.String("provider", "kiro", "provider name to store under")
		dry            = fs.Bool("dry-run", false, "parse + validate only; do not write to vault")
		allowOverwrite = fs.Bool("allow-overwrite", false,
			"allow rotating tokens on an existing id if the new accessToken differs (default: skip with warning)")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*file) == "" {
		return errors.New("--file=<path> is required")
	}

	raw, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read %s: %w", *file, err)
	}

	var entries []kiroTokenEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		return fmt.Errorf("parse JSON: %w", err)
	}
	if len(entries) == 0 {
		return errors.New("empty JSON array (expected [{...}])")
	}

	type validated struct {
		entry    kiroTokenEntry
		id       string
		idSource string
		idx      int
	}
	var (
		ok      []validated
		reasons []string
	)
	for i, e := range entries {
		if strings.TrimSpace(e.AccessToken) == "" {
			reasons = append(reasons, fmt.Sprintf("entry[%d]: empty accessToken", i))
			continue
		}
		if strings.TrimSpace(e.RefreshToken) == "" {
			reasons = append(reasons, fmt.Sprintf("entry[%d]: empty refreshToken", i))
			continue
		}
		if strings.TrimSpace(e.ProfileArn) == "" {
			reasons = append(reasons, fmt.Sprintf("entry[%d]: empty profileArn", i))
			continue
		}
		id, src := deriveAccountID(e)
		if id == "" {
			reasons = append(reasons, fmt.Sprintf("entry[%d]: could not derive id from profileArn or accessToken", i))
			continue
		}
		ok = append(ok, validated{entry: e, id: id, idSource: src, idx: i})
	}

	if *dry {
		fmt.Printf("dry-run: %d valid, %d skipped\n", len(ok), len(reasons))
		for _, v := range ok {
			fmt.Printf("  entry[%d]: id=%s (from %s)  provider=%s authMethod=%s expiresIn=%ds\n",
				v.idx, v.id, v.idSource, v.entry.Provider, v.entry.AuthMethod, v.entry.ExpiresIn)
		}
		for _, r := range reasons {
			fmt.Printf("  skipped: %s\n", r)
		}
		return nil
	}

	cfg, err := configForCLI()
	if err != nil {
		return err
	}
	vault, err := tokenvault.Open(ctx, cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open vault: %w", err)
	}
	defer vault.Close()
	_ = os.Chmod(cfg.DBPath, 0o600)

	var added, updated, skipped int
	for _, v := range ok {
		existing, err := vault.Get(ctx, *provider, v.id)
		isNew := errors.Is(err, tokenvault.ErrNotFound)
		if err != nil && !isNew {
			reasons = append(reasons, fmt.Sprintf("entry[%d] lookup: %v", v.idx, err))
			continue
		}

		// Collision detection: if an entry with this id already exists in the
		// vault AND the access token prefix differs, that means we'd be
		// rotating someone else's tokens in place. Without --allow-overwrite,
		// refuse and surface it so the operator can investigate. This protects
		// against:
		//   - accidental re-import of the same file after a stale edit
		//   - hash/email collision that the cascade missed
		//   - two onboarder runs writing to the same output file without
		//     coordination
		if !isNew && existing != nil && !*allowOverwrite {
			newPrefix := tokenHeadForCompare(v.entry.AccessToken)
			oldPrefix := tokenHeadForCompare(existing.AccessToken)
			if newPrefix != "" && oldPrefix != "" && newPrefix != oldPrefix {
				fmt.Fprintf(os.Stderr,
					"warn: entry[%d] id=%s already exists with a different accessToken; "+
						"skipping. Re-run with -allow-overwrite to rotate in place.\n",
					v.idx, v.id)
				skipped++
				continue
			}
		}

		// Derive expires_at from the addedAt timestamp, not import time.
		// addedAt is when the upstream issued the token; import time can be
		// minutes to days later. Wrong base → Phase 2.5 proactive refresh
		// window is miscalibrated. RFC3339 + 2006-01-02T15:04:05 (local) both
		// accepted; falls back to time.Now() on empty / unparseable input.
		expiresAt := deriveExpiresAt(v.entry.AddedAt, v.entry.ExpiresIn)

		md, _ := json.Marshal(map[string]any{
			"source":       "import-accounts-json",
			"provider_sso": v.entry.Provider,
			"auth_method":  v.entry.AuthMethod,
			"profile_arn":  v.entry.ProfileArn,
			"expires_in":   v.entry.ExpiresIn,
			"expires_at":   expiresAt,
			"added_at":     v.entry.AddedAt,
			"id_source":    v.idSource,
			"email":        strings.ToLower(strings.TrimSpace(v.entry.Email)),
		})

		if _, err := vault.Save(ctx, *provider, v.id, tokenvault.Tokens{
			AccessToken:  v.entry.AccessToken,
			RefreshToken: v.entry.RefreshToken,
			Source:       "import-accounts-json",
			Metadata:     string(md),
		}); err != nil {
			reasons = append(reasons, fmt.Sprintf("entry[%d] save: %v", v.idx, err))
			continue
		}
		if isNew || existing == nil {
			added++
		} else {
			updated++
			fmt.Fprintf(os.Stderr, "warn: %s already existed, tokens rotated in-place\n", v.id)
		}
	}

	fmt.Printf("imported %d/%d (added=%d updated=%d collision-skipped=%d invalid-skipped=%d)\n",
		added+updated, len(entries), added, updated, skipped, len(entries)-added-updated-skipped)
	for _, r := range reasons {
		fmt.Printf("  skipped: %s\n", r)
	}
	if added+updated > 0 {
		fmt.Println("\nrestart `kiroxy serve` to pick up the imported accounts.")
	}
	return nil
}

// deriveAccountID extracts a stable id for the vault key using a 4-layer
// priority cascade:
//
//  1. entry.Email (normalized lowercase; authoritative for v1.0.1+ JSON)
//  2. JWT "email" / "sub" claim decoded from accessToken (defensive;
//     Kiro's opaque aoa... tokens will fall through this layer today)
//  3. last path segment of entry.ProfileArn (legacy; shared across
//     Workspace users in the same Kiro org, hence lower priority)
//  4. first 12 chars of entry.AccessToken (last resort; collision-prone
//     but avoids empty ids)
//
// Why this cascade: Google Workspace accounts within the same org share
// the same profileArn (BUG 4). Keying solely on profileArn silently
// overwrote earlier imports with later ones. Email is the only reliably
// unique identifier we have; everything else is a fallback.
func deriveAccountID(e kiroTokenEntry) (id, source string) {
	if em := strings.ToLower(strings.TrimSpace(e.Email)); em != "" {
		return em, "email"
	}
	if claim := jwtSubOrEmail(strings.TrimSpace(e.AccessToken)); claim != "" {
		return strings.ToLower(claim), "jwt_sub"
	}
	if arn := strings.TrimSpace(e.ProfileArn); arn != "" {
		if i := strings.LastIndexByte(arn, '/'); i >= 0 && i+1 < len(arn) {
			return arn[i+1:], "profileArn"
		}
		return arn, "profileArn_full"
	}
	if at := strings.TrimSpace(e.AccessToken); at != "" {
		if len(at) > 12 {
			return at[:12], "accessToken_prefix"
		}
		return at, "accessToken_prefix"
	}
	return "", ""
}

// jwtSubOrEmail extracts the "email" (preferred) or "sub" claim from a JWT
// access token, or returns "" if the token is not a JWT or the payload is
// malformed. Defensive: every error path returns "" so callers can cleanly
// fall back to other id sources. Mirrors kiro_oauth.jwt_sub_or_email in
// Python.
//
// Today's Kiro tokens are opaque (aoa... prefix) and not JWTs, so this
// helper returns "" in practice. It exists for parity with the Python
// side and for future-proofing against token-shape changes.
func jwtSubOrEmail(token string) string {
	if token == "" {
		return ""
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// Some JWTs are emitted with padding; try the std decoder as a fallback.
		payload, err = base64.URLEncoding.DecodeString(parts[1])
		if err != nil {
			return ""
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return ""
	}
	if v, ok := claims["email"].(string); ok {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	if v, ok := claims["sub"].(string); ok {
		if s := strings.TrimSpace(v); s != "" {
			return s
		}
	}
	return ""
}

// tokenHeadForCompare returns the first 16 chars of a trimmed token, or ""
// for empty input. Used only by the collision detector so we can identify
// "same id, different token" without comparing full opaque strings.
func tokenHeadForCompare(t string) string {
	t = strings.TrimSpace(t)
	if len(t) > 16 {
		return t[:16]
	}
	return t
}

// deriveExpiresAt parses an `addedAt` timestamp string (RFC3339 first, then
// the local-time variant `2006-01-02T15:04:05` for legacy kiro_login.py output)
// and adds expiresIn seconds. On empty input or parse failure it falls back to
// `time.Now() + expiresIn` and is reflected in Phase 2.5 metadata as a best-
// effort approximation.
func deriveExpiresAt(addedAt string, expiresIn int64) int64 {
	addedAt = strings.TrimSpace(addedAt)
	if addedAt == "" {
		return time.Now().Unix() + expiresIn
	}
	if t, err := time.Parse(time.RFC3339, addedAt); err == nil {
		return t.Unix() + expiresIn
	}
	if t, err := time.ParseInLocation("2006-01-02T15:04:05", addedAt, time.Local); err == nil {
		return t.Unix() + expiresIn
	}
	return time.Now().Unix() + expiresIn
}
