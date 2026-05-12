package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"local/kiroxy/internal/tokenvault"
)

func runDebugRefresh(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("debug-refresh", flag.ContinueOnError)
	var (
		provider  = fs.String("provider", "kiro", "provider name in the vault")
		id        = fs.String("id", "", "connection_id (email for triplet-imported accounts)")
		region    = fs.String("region", "us-east-1", "AWS region for the Kiro desktop refresh endpoint")
		persist   = fs.Bool("persist", true, "on 2xx, write the returned access_token back to the vault")
		verbose   = fs.Bool("verbose", true, "print request + response details")
		wireDump  = fs.Bool("wire", false, "DIAG 1: dump full request + response headers and body")
		userAgent = fs.String("user-agent", "", "override User-Agent header (DIAG 2)")
		snakeCase = fs.Bool("snake-case", false, "DIAG 3: send refresh_token instead of refreshToken in body")
	)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*id) == "" {
		return errors.New("--id is required (use `kiroxy list-accounts` to find it)")
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

	b, err := vault.Get(ctx, *provider, *id)
	if err != nil {
		return fmt.Errorf("get %s/%s: %w", *provider, *id, err)
	}
	if b.RefreshToken == "" {
		return errors.New("account has no refresh_token stored")
	}

	endpoint := fmt.Sprintf("https://prod.%s.auth.desktop.kiro.dev/refreshToken", *region)
	bodyMap := map[string]string{"refreshToken": b.RefreshToken}
	if *snakeCase {
		bodyMap = map[string]string{"refresh_token": b.RefreshToken}
	}
	body, _ := json.Marshal(bodyMap)

	if *verbose {
		fmt.Fprintln(os.Stderr, "=== debug-refresh ===")
		fmt.Fprintf(os.Stderr, "endpoint: %s\n", endpoint)
		fmt.Fprintf(os.Stderr, "provider: %s\n", b.Provider)
		fmt.Fprintf(os.Stderr, "id:       %s\n", b.ConnectionID)
		fmt.Fprintf(os.Stderr, "rt:       %s...%s (len=%d)\n", redactToken(b.RefreshToken, 4), redactTokenTail(b.RefreshToken, 4), len(b.RefreshToken))
		if *snakeCase {
			fmt.Fprintln(os.Stderr, "body field: refresh_token (DIAG 3 snake_case)")
		} else {
			fmt.Fprintln(os.Stderr, "body field: refreshToken (default)")
		}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	if *userAgent != "" {
		req.Header.Set("User-Agent", *userAgent)
	}

	if *wireDump {
		fmt.Fprintln(os.Stderr, "--- request ---")
		fmt.Fprintf(os.Stderr, "%s %s\n", req.Method, req.URL.String())
		for k, v := range req.Header {
			fmt.Fprintf(os.Stderr, "%s: %s\n", k, strings.Join(v, ", "))
		}
		if req.Header.Get("User-Agent") == "" {
			fmt.Fprintln(os.Stderr, "User-Agent: (default — Go sets \"Go-http-client/1.1\" at wire time)")
		}
		rtRed := redactToken(b.RefreshToken, 4) + "..." + redactTokenTail(b.RefreshToken, 4)
		fmt.Fprintf(os.Stderr, "body: %s\n", strings.ReplaceAll(string(body), b.RefreshToken, rtRed))
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("refresh: %w", err)
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 32*1024))

	if *verbose {
		fmt.Fprintf(os.Stderr, "--- response ---\n")
		fmt.Fprintf(os.Stderr, "status: %d\n", resp.StatusCode)
		if *wireDump {
			for k, v := range resp.Header {
				fmt.Fprintf(os.Stderr, "%s: %s\n", k, strings.Join(v, ", "))
			}
		} else {
			fmt.Fprintf(os.Stderr, "content-type: %s\n", resp.Header.Get("Content-Type"))
		}
		fmt.Fprintf(os.Stderr, "body (verbatim, up to 32KB):\n%s\n", string(respBody))
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("refresh returned HTTP %d", resp.StatusCode)
	}

	var parsed struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		ExpiresIn    int64  `json:"expiresIn"`
		ProfileArn   string `json:"profileArn"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return fmt.Errorf("parse: %w", err)
	}
	if parsed.AccessToken == "" {
		return errors.New("refresh returned 2xx but no accessToken")
	}

	fmt.Fprintln(os.Stderr, "--- parsed ---")
	fmt.Fprintf(os.Stderr, "access_token: %s...%s (len=%d)\n", redactToken(parsed.AccessToken, 6), redactTokenTail(parsed.AccessToken, 6), len(parsed.AccessToken))
	fmt.Fprintf(os.Stderr, "refresh_token (rotated): %s...%s (len=%d)\n", redactToken(parsed.RefreshToken, 6), redactTokenTail(parsed.RefreshToken, 6), len(parsed.RefreshToken))
	fmt.Fprintf(os.Stderr, "expires_in: %ds\n", parsed.ExpiresIn)
	fmt.Fprintf(os.Stderr, "profile_arn: %q\n", parsed.ProfileArn)

	if *persist {
		newRefresh := parsed.RefreshToken
		if newRefresh == "" {
			newRefresh = b.RefreshToken
		}
		md := fmt.Sprintf(`{"source":"debug-refresh","profile_arn":%q,"expires_at":%d}`,
			parsed.ProfileArn, time.Now().Unix()+parsed.ExpiresIn)
		_, err = vault.Save(ctx, b.Provider, b.ConnectionID, tokenvault.Tokens{
			AccessToken:  parsed.AccessToken,
			RefreshToken: newRefresh,
			Source:       "debug-refresh",
			Metadata:     md,
		})
		if err != nil {
			return fmt.Errorf("persist: %w", err)
		}
		fmt.Println("refreshed and persisted.")
	} else {
		fmt.Println("refreshed (not persisted; --persist=false).")
	}
	return nil
}

func redactToken(s string, n int) string {
	if len(s) <= n*2+3 {
		return strings.Repeat("*", len(s))
	}
	return s[:n]
}

func redactTokenTail(s string, n int) string {
	if len(s) <= n*2+3 {
		return ""
	}
	return s[len(s)-n:]
}
