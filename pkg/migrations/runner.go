// Package migrations manages PostgreSQL schema versions explicitly, without
// modifying schema from application startup.
package migrations

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io/fs"
	"regexp"
	"strconv"

	schema "github.com/gofxq/gaoming/deployments/sql/migrations"
)

const lockID int64 = 0x67616f6d696e67

var filenamePattern = regexp.MustCompile(`^([0-9]{3})-[a-z0-9-]+\.sql$`)

type migration struct {
	version              int
	name, body, checksum string
}

func load(source fs.FS) ([]migration, error) {
	names, err := fs.Glob(source, "*.sql")
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no migrations found")
	}
	items := make([]migration, 0, len(names))
	for i, name := range names {
		match := filenamePattern.FindStringSubmatch(name)
		if match == nil {
			return nil, fmt.Errorf("invalid migration filename %q", name)
		}
		version, _ := strconv.Atoi(match[1])
		if version != i+1 {
			return nil, fmt.Errorf("migration versions must be contiguous starting at 001: %q", name)
		}
		body, err := fs.ReadFile(source, name)
		if err != nil {
			return nil, err
		}
		hash := sha256.Sum256(body)
		items = append(items, migration{version: version, name: name, body: string(body), checksum: hex.EncodeToString(hash[:])})
	}
	return items, nil
}

// Run applies pending migrations atomically. An advisory transaction lock
// serializes administrators; cancellation releases the lock by rolling back.
// Existing pre-versioned installations are adopted by the idempotent baseline.
func Run(ctx context.Context, db *sql.DB) error { return run(ctx, db, schema.Files) }

func run(ctx context.Context, db *sql.DB, source fs.FS) error {
	items, err := load(source)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, lockID); err != nil {
		return fmt.Errorf("lock migration: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version integer PRIMARY KEY, name text NOT NULL, checksum text NOT NULL,
		applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return fmt.Errorf("create migration ledger: %w", err)
	}
	applied, err := verify(ctx, tx, items)
	if err != nil {
		return err
	}
	for _, item := range items[applied:] {
		if _, err := tx.ExecContext(ctx, item.body); err != nil {
			// Do not include the driver error: it can contain data from statements.
			return fmt.Errorf("migration %s failed; transaction rolled back", item.name)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_migrations(version,name,checksum) VALUES ($1,$2,$3)`, item.version, item.name, item.checksum); err != nil {
			return fmt.Errorf("record migration %s: %w", item.name, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func verify(ctx context.Context, db queryer, items []migration) (int, error) {
	rows, err := db.QueryContext(ctx, `SELECT version,name,checksum FROM schema_migrations ORDER BY version`)
	if err != nil {
		return 0, fmt.Errorf("schema is not versioned; run make migrate")
	}
	defer rows.Close()
	count := 0
	for rows.Next() {
		var version int
		var name, checksum string
		if err := rows.Scan(&version, &name, &checksum); err != nil {
			return 0, err
		}
		if count >= len(items) || version != count+1 {
			return 0, fmt.Errorf("unsupported or noncontiguous schema version %d", version)
		}
		item := items[count]
		if name != item.name || checksum != item.checksum {
			return 0, fmt.Errorf("applied migration %d differs from this binary; restore the original migration", version)
		}
		count++
	}
	return count, rows.Err()
}

// Check rejects old, unknown or edited schema versions. It is read-only.
func Check(ctx context.Context, db *sql.DB) error {
	items, err := load(schema.Files)
	if err != nil {
		return err
	}
	count, err := verify(ctx, db, items)
	if err != nil {
		return err
	}
	if count != len(items) {
		return fmt.Errorf("schema version %d requires %d; run make migrate", count, len(items))
	}
	return nil
}
