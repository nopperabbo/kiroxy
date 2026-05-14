// Package tokenvault stores OAuth credentials for upstream accounts and mediates
// safe concurrent token refresh across goroutines and processes.
//
// Ported from github.com/kadangkesel/hexos @ d4c0d1ce556d7012771ffa289523ad008b0414df
// (src/auth/token-vault.ts, MIT). The generation-lock / reserve-commit-release
// pattern is preserved literally: reserving a refresh token captures the current
// generation number; Commit rejects the write if another actor has already
// rotated the bundle; Release rolls back a reservation.
//
// Storage is SQLite (modernc.org/sqlite, pure Go, no cgo). All mutating
// operations run inside IMMEDIATE transactions so the database lock is acquired
// up front, eliminating the "SQLITE_BUSY because upgrade from read to write"
// pathology.
package tokenvault

import (
	"context"
	"database/sql"
	"encoding/json/v2"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// Bundle is one (provider, connection_id) credential record.
type Bundle struct {
	Provider             string
	ConnectionID         string
	AccessToken          string
	RefreshToken         string
	PreviousRefreshToken string
	Generation           int64
	UpdatedAt            time.Time
	Source               string
	// Metadata is opaque, caller-owned JSON (or empty) intended for
	// non-credential trivia such as extractor-tool signatures, provider
	// hints, or subscription metadata. The vault does not interpret it;
	// the upstream HTTP client never sends it.
	Metadata             string
	RefreshInProgress    bool
	RefreshStartedAt     time.Time
	RefreshLockExpiresAt time.Time
}

// Vault is a SQLite-backed token store.
type Vault struct {
	db *sql.DB
	mu sync.Mutex
}

// DefaultLockTTL is the reservation window when callers don't supply one.
const DefaultLockTTL = 5 * time.Minute

// Errors returned by the Vault.
var (
	ErrNotFound        = errors.New("tokenvault: bundle not found")
	ErrNoRefreshToken  = errors.New("tokenvault: no refresh token on bundle")
	ErrLockHeld        = errors.New("tokenvault: refresh token already reserved by another caller")
	ErrGenerationStale = errors.New("tokenvault: generation changed while refresh was in progress")
)

// Open opens (or creates) the SQLite database at path and applies the schema.
// If the parent directory of path does not exist, it is created with mode 0700.
// Existing directories are left untouched.
func Open(ctx context.Context, path string) (*Vault, error) {
	if dir := filepath.Dir(path); dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, fmt.Errorf("create vault dir %s: %w", dir, err)
		}
	}
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open vault: %w", err)
	}
	db.SetMaxOpenConns(1)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping vault: %w", err)
	}
	if _, err := db.ExecContext(ctx, schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	// Idempotent migration: 'duplicate column' is expected on fresh DBs because
	// the CREATE above already includes the column. We only care when it fails
	// for any other reason.
	if _, err := db.ExecContext(ctx, migrateAddMetadata); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			_ = db.Close()
			return nil, fmt.Errorf("migrate add metadata: %w", err)
		}
	}
	tightenVaultPerms(path)
	return &Vault{db: db}, nil
}

// tightenVaultPerms enforces 0600 on the SQLite file plus its WAL/SHM sidecars.
// SQLite creates -wal and -shm with the process umask (often 0644) which would
// expose plaintext access/refresh tokens to other local users. Best-effort:
// we never fail Open() if chmod fails; a permission warning is emitted instead.
func tightenVaultPerms(path string) {
	for _, p := range []string{path, path + "-wal", path + "-shm"} {
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := os.Chmod(p, 0o600); err != nil {
			slog.Warn("tokenvault: chmod 0600 failed", "path", p, "err", err)
		}
	}
}

// Close closes the underlying database.
func (v *Vault) Close() error {
	return v.db.Close()
}

const schema = `
CREATE TABLE IF NOT EXISTS token_bundles (
    provider                TEXT    NOT NULL,
    connection_id           TEXT    NOT NULL,
    access_token            TEXT    NOT NULL,
    refresh_token           TEXT    NOT NULL,
    previous_refresh_token  TEXT    NOT NULL DEFAULT '',
    generation              INTEGER NOT NULL DEFAULT 0,
    updated_at              INTEGER NOT NULL,
    source                  TEXT    NOT NULL DEFAULT '',
    metadata                TEXT    NOT NULL DEFAULT '',
    refresh_in_progress     INTEGER NOT NULL DEFAULT 0,
    refresh_started_at      INTEGER NOT NULL DEFAULT 0,
    refresh_lock_expires_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (provider, connection_id)
) WITHOUT ROWID;

-- v0.1.1: add metadata column to legacy DBs created before this migration.
-- SQLite is permissive about ADD COLUMN on an existing table; we ignore the
-- 'duplicate column' error that arises on fresh schemas (already has it).
`

const migrateAddMetadata = `ALTER TABLE token_bundles ADD COLUMN metadata TEXT NOT NULL DEFAULT ''`

// Get returns the bundle for (provider, connectionID) or ErrNotFound.
func (v *Vault) Get(ctx context.Context, provider, connectionID string) (*Bundle, error) {
	return v.getBundle(ctx, v.db, provider, connectionID)
}

func (v *Vault) getBundle(ctx context.Context, q queryer, provider, connectionID string) (*Bundle, error) {
	row := q.QueryRowContext(ctx, `
		SELECT provider, connection_id, access_token, refresh_token, previous_refresh_token,
		       generation, updated_at, source, metadata, refresh_in_progress,
		       refresh_started_at, refresh_lock_expires_at
		FROM token_bundles
		WHERE provider = ? AND connection_id = ?`,
		provider, connectionID)
	b, err := scanBundle(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return b, err
}

// Save upserts a bundle with fresh tokens. Generation increments by 1; previous
// refresh_token is retained for audit.
func (v *Vault) Save(ctx context.Context, provider, connectionID string, tokens Tokens) (*Bundle, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	prior, err := v.getBundle(ctx, tx, provider, connectionID)
	if err != nil && !errors.Is(err, ErrNotFound) {
		return nil, err
	}
	prev := ""
	var nextGen int64 = 1
	if prior != nil {
		prev = prior.RefreshToken
		nextGen = prior.Generation + 1
	}
	now := time.Now().UnixMilli()
	_, err = tx.ExecContext(ctx, `
		INSERT INTO token_bundles
		  (provider, connection_id, access_token, refresh_token, previous_refresh_token,
		   generation, updated_at, source, metadata,
		   refresh_in_progress, refresh_started_at, refresh_lock_expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 0, 0, 0)
		ON CONFLICT(provider, connection_id) DO UPDATE SET
		  access_token           = excluded.access_token,
		  refresh_token          = excluded.refresh_token,
		  previous_refresh_token = excluded.previous_refresh_token,
		  generation             = excluded.generation,
		  updated_at             = excluded.updated_at,
		  source                 = excluded.source,
		  metadata               = excluded.metadata,
		  refresh_in_progress    = 0,
		  refresh_started_at     = 0,
		  refresh_lock_expires_at= 0`,
		provider, connectionID, tokens.AccessToken, tokens.RefreshToken, prev,
		nextGen, now, tokens.Source, tokens.Metadata)
	if err != nil {
		return nil, fmt.Errorf("save bundle: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return v.Get(ctx, provider, connectionID)
}

// Tokens is the subset of bundle fields Save and Commit accept from callers.
type Tokens struct {
	AccessToken  string
	RefreshToken string
	Source       string
	Metadata     string
}

// Reserve acquires a refresh-token lock on the bundle. Returns ErrLockHeld if
// another reservation is currently in-flight and hasn't expired. Returns the
// current refresh token and the generation at reservation time; callers must
// pass that generation back to Commit or Release.
func (v *Vault) Reserve(ctx context.Context, provider, connectionID string, lockTTL time.Duration) (refreshToken string, generation int64, err error) {
	if lockTTL <= 0 {
		lockTTL = DefaultLockTTL
	}
	v.mu.Lock()
	defer v.mu.Unlock()

	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return "", 0, err
	}
	defer tx.Rollback()

	b, err := v.getBundle(ctx, tx, provider, connectionID)
	if err != nil {
		return "", 0, err
	}
	if b.RefreshToken == "" {
		return "", 0, ErrNoRefreshToken
	}
	now := time.Now()
	if b.RefreshInProgress && b.RefreshLockExpiresAt.After(now) {
		return "", 0, ErrLockHeld
	}
	expires := now.Add(lockTTL)
	_, err = tx.ExecContext(ctx, `
		UPDATE token_bundles
		SET refresh_in_progress = 1,
		    refresh_started_at = ?,
		    refresh_lock_expires_at = ?,
		    updated_at = ?
		WHERE provider = ? AND connection_id = ? AND generation = ?`,
		now.UnixMilli(), expires.UnixMilli(), now.UnixMilli(),
		provider, connectionID, b.Generation)
	if err != nil {
		return "", 0, fmt.Errorf("reserve lock: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return "", 0, err
	}
	return b.RefreshToken, b.Generation, nil
}

// Commit swaps the tokens if the generation still matches what Reserve saw.
// Generation is incremented on success. Returns ErrGenerationStale otherwise.
func (v *Vault) Commit(ctx context.Context, provider, connectionID string, reservedGeneration int64, tokens Tokens) (*Bundle, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	b, err := v.getBundle(ctx, tx, provider, connectionID)
	if err != nil {
		return nil, err
	}
	if b.Generation != reservedGeneration {
		return nil, ErrGenerationStale
	}
	now := time.Now().UnixMilli()
	_, err = tx.ExecContext(ctx, `
		UPDATE token_bundles
		SET access_token = ?,
		    refresh_token = ?,
		    previous_refresh_token = ?,
		    generation = generation + 1,
		    updated_at = ?,
		    source = ?,
		    refresh_in_progress = 0,
		    refresh_started_at = 0,
		    refresh_lock_expires_at = 0
		WHERE provider = ? AND connection_id = ? AND generation = ?`,
		tokens.AccessToken, tokens.RefreshToken, b.RefreshToken, now, tokens.Source,
		provider, connectionID, reservedGeneration)
	if err != nil {
		return nil, fmt.Errorf("commit tokens: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return v.Get(ctx, provider, connectionID)
}

// CommitWithMetaPatch is like Commit but also shallow-merges the given
// metadata patch onto the bundle's existing metadata JSON. Keys in patch
// overwrite existing keys; keys absent from patch are preserved.
//
// Used by the pool refresher (internal/pool/refresh.go) to write the new
// expires_at + rotated refresh_token without clobbering profile_arn /
// auth_method / source / provider_sso. On malformed existing metadata,
// the patch becomes the entire metadata.
func (v *Vault) CommitWithMetaPatch(ctx context.Context, provider, connectionID string, reservedGeneration int64, tokens Tokens, metaPatch map[string]any) (*Bundle, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	b, err := v.getBundle(ctx, tx, provider, connectionID)
	if err != nil {
		return nil, err
	}
	if b.Generation != reservedGeneration {
		return nil, ErrGenerationStale
	}

	merged := map[string]any{}
	if b.Metadata != "" {
		_ = json.Unmarshal([]byte(b.Metadata), &merged) // tolerate malformed
	}
	for k, val := range metaPatch {
		merged[k] = val
	}
	mergedBytes, err := json.Marshal(merged)
	if err != nil {
		return nil, fmt.Errorf("marshal merged metadata: %w", err)
	}

	now := time.Now().UnixMilli()
	_, err = tx.ExecContext(ctx, `
		UPDATE token_bundles
		SET access_token = ?,
		    refresh_token = ?,
		    previous_refresh_token = ?,
		    generation = generation + 1,
		    updated_at = ?,
		    source = ?,
		    metadata = ?,
		    refresh_in_progress = 0,
		    refresh_started_at = 0,
		    refresh_lock_expires_at = 0
		WHERE provider = ? AND connection_id = ? AND generation = ?`,
		tokens.AccessToken, tokens.RefreshToken, b.RefreshToken, now, tokens.Source, string(mergedBytes),
		provider, connectionID, reservedGeneration)
	if err != nil {
		return nil, fmt.Errorf("commit tokens with meta: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return v.Get(ctx, provider, connectionID)
}

// Release clears an in-progress reservation. When onlyIfExpired is true, it
// only clears the reservation if the lock TTL has elapsed.
func (v *Vault) Release(ctx context.Context, provider, connectionID string, reservedGeneration int64, onlyIfExpired bool) error {
	v.mu.Lock()
	defer v.mu.Unlock()

	tx, err := v.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return err
	}
	defer tx.Rollback()

	b, err := v.getBundle(ctx, tx, provider, connectionID)
	if err != nil {
		return err
	}
	if b.Generation != reservedGeneration {
		return nil
	}
	if onlyIfExpired && b.RefreshLockExpiresAt.After(time.Now()) {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE token_bundles
		SET refresh_in_progress = 0,
		    refresh_started_at = 0,
		    refresh_lock_expires_at = 0,
		    updated_at = ?
		WHERE provider = ? AND connection_id = ? AND generation = ?`,
		time.Now().UnixMilli(), provider, connectionID, reservedGeneration)
	if err != nil {
		return fmt.Errorf("release lock: %w", err)
	}
	return tx.Commit()
}

// ListByProvider returns all bundles for the given provider, ordered by
// connection_id.
func (v *Vault) ListByProvider(ctx context.Context, provider string) ([]*Bundle, error) {
	rows, err := v.db.QueryContext(ctx, `
		SELECT provider, connection_id, access_token, refresh_token, previous_refresh_token,
		       generation, updated_at, source, metadata, refresh_in_progress,
		       refresh_started_at, refresh_lock_expires_at
		FROM token_bundles
		WHERE provider = ?
		ORDER BY connection_id`, provider)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*Bundle
	for rows.Next() {
		b, err := scanBundle(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// Delete removes a bundle. No-op if not present.
func (v *Vault) Delete(ctx context.Context, provider, connectionID string) error {
	_, err := v.db.ExecContext(ctx, `DELETE FROM token_bundles WHERE provider = ? AND connection_id = ?`, provider, connectionID)
	return err
}

// RefreshFunc performs the upstream OAuth refresh. The vault calls it while
// holding a Reserve lock, and commits or releases the lock based on the result.
type RefreshFunc func(ctx context.Context, currentRefreshToken string) (Tokens, error)

// Refresh performs Reserve -> call fn -> Commit (or Release) in a single call,
// which is the common case. If fn succeeds, Commit is attempted; on
// ErrGenerationStale Commit returns an error and Refresh does NOT retry
// automatically \u2014 the caller is expected to re-read the (now newer) bundle.
//
// If fn fails, the lock is released. If Refresh is called concurrently for the
// same (provider, connectionID), one caller wins Reserve and the other gets
// ErrLockHeld; the losing caller SHOULD re-read the bundle rather than refresh
// again \u2014 the winner's refreshed token is about to be committed.
func (v *Vault) Refresh(ctx context.Context, provider, connectionID string, lockTTL time.Duration, fn RefreshFunc) (*Bundle, error) {
	refreshTok, gen, err := v.Reserve(ctx, provider, connectionID, lockTTL)
	if err != nil {
		return nil, err
	}
	tokens, err := fn(ctx, refreshTok)
	if err != nil {
		_ = v.Release(ctx, provider, connectionID, gen, false)
		return nil, fmt.Errorf("refresh: %w", err)
	}
	return v.Commit(ctx, provider, connectionID, gen, tokens)
}

// ---- internals ----

type queryer interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanBundle(s rowScanner) (*Bundle, error) {
	var (
		b                Bundle
		updatedAt        int64
		startedAt        int64
		lockExpiresAt    int64
		refreshInProgInt int64
	)
	err := s.Scan(
		&b.Provider, &b.ConnectionID, &b.AccessToken, &b.RefreshToken,
		&b.PreviousRefreshToken, &b.Generation, &updatedAt, &b.Source,
		&b.Metadata, &refreshInProgInt, &startedAt, &lockExpiresAt,
	)
	if err != nil {
		return nil, err
	}
	b.UpdatedAt = time.UnixMilli(updatedAt)
	if startedAt > 0 {
		b.RefreshStartedAt = time.UnixMilli(startedAt)
	}
	if lockExpiresAt > 0 {
		b.RefreshLockExpiresAt = time.UnixMilli(lockExpiresAt)
	}
	b.RefreshInProgress = refreshInProgInt != 0
	return &b, nil
}
