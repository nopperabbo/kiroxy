package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"local/kiroxy/internal/tokenvault"
)

// kiroTokenEntry is the shape emitted by the Desktop-flow extractor.
// Every entry is one completed OAuth session against
// prod.us-east-1.auth.desktop.kiro.dev/login (redirect_uri
// kiro://kiro.kiroAgent/authenticate-success). accessToken + refreshToken
// are opaque Kiro tokens (aoa.../aor... prefix), profileArn is the scoped
// CodeWhisperer profile for API calls.
type kiroTokenEntry struct {
	Provider     string `json:"provider"`
	AuthMethod   string `json:"authMethod"`
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
	ProfileArn   string `json:"profileArn"`
	ExpiresIn    int64  `json:"expiresIn"`
	AddedAt      string `json:"addedAt"`
}

func runImportAccountsJSON(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import-accounts-json", flag.ContinueOnError)
	var (
		file     = fs.String("file", "", "path to JSON array of Kiro token entries")
		provider = fs.String("provider", "kiro", "provider name to store under")
		dry      = fs.Bool("dry-run", false, "parse + validate only; do not write to vault")
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

	var added, updated int
	for _, v := range ok {
		existing, err := vault.Get(ctx, *provider, v.id)
		isNew := errors.Is(err, tokenvault.ErrNotFound)
		if err != nil && !isNew {
			reasons = append(reasons, fmt.Sprintf("entry[%d] lookup: %v", v.idx, err))
			continue
		}

		md, _ := json.Marshal(map[string]any{
			"source":       "import-accounts-json",
			"provider_sso": v.entry.Provider,
			"auth_method":  v.entry.AuthMethod,
			"profile_arn":  v.entry.ProfileArn,
			"expires_in":   v.entry.ExpiresIn,
			"added_at":     v.entry.AddedAt,
			"id_source":    v.idSource,
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

	fmt.Printf("imported %d/%d (added=%d updated=%d skipped=%d)\n",
		added+updated, len(entries), added, updated, len(entries)-added-updated)
	for _, r := range reasons {
		fmt.Printf("  skipped: %s\n", r)
	}
	if added+updated > 0 {
		fmt.Println("\nrestart `kiroxy serve` to pick up the imported accounts.")
	}
	return nil
}

// deriveAccountID extracts a stable id for the vault key. The JSON format
// guarantees profileArn; its final path segment (after the last '/') is the
// profile name, which is unique per Kiro account. Falls back to a shortened
// accessToken head if profileArn is somehow absent despite validation.
func deriveAccountID(e kiroTokenEntry) (id, source string) {
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
