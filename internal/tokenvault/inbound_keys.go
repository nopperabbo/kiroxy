// Inbound API keys storage. Separate from token_bundles because the
// schema, lifecycle, and access patterns are different: inbound keys are
// granted to outside callers (clients of kiroxy), while token_bundles
// hold upstream Kiro credentials.
//
// Storage choices:
//   - Re-uses the same SQLite database as token_bundles. One file, one
//     vault, one migration story.
//   - Stores SHA-256 hashes of keys, never the plaintext. The plaintext
//     is shown to the operator exactly once (return value of Create).
//   - Includes a 4-character "tail" of the plaintext so the dashboard can
//     show '****abc1' without making round-trips to the operator.
//   - 'created_at' / 'last_used_at' are unix-ms ints, matching the
//     existing token_bundles.updated_at convention.

package tokenvault

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"
)

const inboundKeysSchema = `
CREATE TABLE IF NOT EXISTS inbound_keys (
    id            TEXT    PRIMARY KEY,
    label         TEXT    NOT NULL DEFAULT '',
    key_hash      TEXT    NOT NULL,
    tail          TEXT    NOT NULL,
    created_at    INTEGER NOT NULL,
    last_used_at  INTEGER NOT NULL DEFAULT 0,
    revoked       INTEGER NOT NULL DEFAULT 0
) WITHOUT ROWID;

CREATE INDEX IF NOT EXISTS idx_inbound_keys_revoked ON inbound_keys(revoked);
`

// InboundKey is one row from the inbound_keys table. Plaintext is never
// returned by reads; only Create surfaces the plaintext (once).
type InboundKey struct {
	ID         string
	Label      string
	Tail       string
	CreatedAt  time.Time
	LastUsedAt time.Time
	Revoked    bool
}

// inboundKeysReady is set by the lazy migrator. Calling code does not need
// to coordinate migration ordering — every operation calls ensureInboundKeys
// which is idempotent.
func (v *Vault) ensureInboundKeys(ctx context.Context) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	if _, err := v.db.ExecContext(ctx, inboundKeysSchema); err != nil {
		return fmt.Errorf("init inbound_keys schema: %w", err)
	}
	return nil
}

// CreateInboundKey generates a new key, persists its hash + 4-char tail,
// and returns the plaintext. The plaintext is shown to the operator once
// and then discarded; subsequent calls to ListInboundKeys return only the
// tail and metadata.
//
// Format of the returned plaintext:
//
//	kxy_<base64url-no-padding>   (~36 chars)
//
// 32 random bytes give 256 bits of entropy. The 'kxy_' prefix is operator
// affordance — it makes the key recognisable in shell history and makes
// kiroxy's audit log easy to grep.
func (v *Vault) CreateInboundKey(ctx context.Context, label string) (id, plaintext string, err error) {
	if err := v.ensureInboundKeys(ctx); err != nil {
		return "", "", err
	}
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", "", fmt.Errorf("generate key: %w", err)
	}
	plaintext = "kxy_" + base64.RawURLEncoding.EncodeToString(raw[:])
	hash := sha256.Sum256([]byte(plaintext))
	hashHex := hex.EncodeToString(hash[:])
	tail := plaintext[len(plaintext)-4:]
	id, err = randomID()
	if err != nil {
		return "", "", err
	}
	now := time.Now().UnixMilli()
	v.mu.Lock()
	_, err = v.db.ExecContext(ctx, `
		INSERT INTO inbound_keys (id, label, key_hash, tail, created_at, last_used_at, revoked)
		VALUES (?, ?, ?, ?, ?, 0, 0)`,
		id, strings.TrimSpace(label), hashHex, tail, now)
	v.mu.Unlock()
	if err != nil {
		return "", "", fmt.Errorf("insert inbound key: %w", err)
	}
	return id, plaintext, nil
}

// ListInboundKeys returns all keys (including revoked) ordered newest-first.
// Plaintext + hash are NEVER returned; the caller sees only the tail and
// metadata sufficient for the dashboard.
func (v *Vault) ListInboundKeys(ctx context.Context) ([]InboundKey, error) {
	if err := v.ensureInboundKeys(ctx); err != nil {
		return nil, err
	}
	rows, err := v.db.QueryContext(ctx, `
		SELECT id, label, tail, created_at, last_used_at, revoked
		FROM inbound_keys
		ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []InboundKey
	for rows.Next() {
		var k InboundKey
		var createdAt, lastUsedAt int64
		var revokedInt int64
		if err := rows.Scan(&k.ID, &k.Label, &k.Tail, &createdAt, &lastUsedAt, &revokedInt); err != nil {
			return nil, err
		}
		k.CreatedAt = time.UnixMilli(createdAt)
		if lastUsedAt > 0 {
			k.LastUsedAt = time.UnixMilli(lastUsedAt)
		}
		k.Revoked = revokedInt != 0
		out = append(out, k)
	}
	return out, rows.Err()
}

// RevokeInboundKey marks a key as revoked. Returns ErrInboundKeyNotFound
// when the id is unknown.
func (v *Vault) RevokeInboundKey(ctx context.Context, id string) error {
	if err := v.ensureInboundKeys(ctx); err != nil {
		return err
	}
	v.mu.Lock()
	res, err := v.db.ExecContext(ctx, `
		UPDATE inbound_keys SET revoked = 1 WHERE id = ?`, id)
	v.mu.Unlock()
	if err != nil {
		return fmt.Errorf("revoke inbound key: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrInboundKeyNotFound
	}
	return nil
}

// VerifyInboundKey checks plaintext against the stored hashes; returns the
// matched key id when valid AND not revoked. Touches last_used_at on success.
// Constant-time comparison is provided by SHA-256 + hex equality (both inputs
// have identical lengths so a string-equality timing leak doesn't disclose
// secret bits).
func (v *Vault) VerifyInboundKey(ctx context.Context, plaintext string) (string, error) {
	if err := v.ensureInboundKeys(ctx); err != nil {
		return "", err
	}
	if !strings.HasPrefix(plaintext, "kxy_") {
		return "", ErrInboundKeyInvalid
	}
	hash := sha256.Sum256([]byte(plaintext))
	hashHex := hex.EncodeToString(hash[:])

	row := v.db.QueryRowContext(ctx, `
		SELECT id, revoked FROM inbound_keys WHERE key_hash = ? LIMIT 1`, hashHex)
	var id string
	var revoked int64
	if err := row.Scan(&id, &revoked); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", ErrInboundKeyInvalid
		}
		return "", err
	}
	if revoked != 0 {
		return "", ErrInboundKeyRevoked
	}
	v.mu.Lock()
	_, _ = v.db.ExecContext(ctx, `
		UPDATE inbound_keys SET last_used_at = ? WHERE id = ?`,
		time.Now().UnixMilli(), id)
	v.mu.Unlock()
	return id, nil
}

// CountInboundKeys returns active and total counts for vault stats display.
func (v *Vault) CountInboundKeys(ctx context.Context) (active, total int, err error) {
	if err := v.ensureInboundKeys(ctx); err != nil {
		return 0, 0, err
	}
	row := v.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN revoked = 0 THEN 1 ELSE 0 END), 0),
			COUNT(*)
		FROM inbound_keys`)
	if err := row.Scan(&active, &total); err != nil {
		return 0, 0, err
	}
	return active, total, nil
}

// Errors returned by the inbound-key surface.
var (
	ErrInboundKeyNotFound = errors.New("tokenvault: inbound key not found")
	ErrInboundKeyInvalid  = errors.New("tokenvault: inbound key invalid")
	ErrInboundKeyRevoked  = errors.New("tokenvault: inbound key revoked")
)

// randomID returns a 16-byte hex id (32 chars). Distinct from the plaintext
// secret so the dashboard can refer to keys by id without leaking entropy.
func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
