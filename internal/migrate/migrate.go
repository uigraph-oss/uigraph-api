// Package migrate runs ordered SQL migrations against a Postgres database.
// Migrations are embedded via the migrations package (uigraph-app/migrations/).
// Each file is applied exactly once; applied versions are tracked in schema_migrations.
package migrate

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/uigraph/app/migrations"
)

// Run applies all pending migrations from the embedded migrations/ directory.
// Safe to call on every startup — already-applied versions are skipped.
func Run(ctx context.Context, db *sql.DB) error {
	return RunFS(ctx, db, migrations.FS, ".")
}

// RunFS applies migrations from fsys rooted at dir. Exposed for testing.
func RunFS(ctx context.Context, db *sql.DB, fsys fs.FS, dir string) error {
	// Ensure the tracking table exists before we query it.
	// On a brand-new database 0001_schemas.sql hasn't run yet so we create it
	// here; 0001_schemas.sql also has CREATE TABLE IF NOT EXISTS so they're safe
	// to run in any order.
	const createTracking = `
		CREATE TABLE IF NOT EXISTS schema_migrations (
		    version    TEXT        NOT NULL PRIMARY KEY,
		    applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`
	if _, err := db.ExecContext(ctx, createTracking); err != nil {
		return fmt.Errorf("migrate: bootstrap schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("migrate: read dir %q: %w", dir, err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, name := range files {
		version := strings.TrimSuffix(name, ".sql")
		applied, err := isApplied(ctx, db, version)
		if err != nil {
			return err
		}
		if applied {
			continue
		}

		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			return fmt.Errorf("migrate: read %q: %w", name, err)
		}

		if err := applyMigration(ctx, db, version, string(data)); err != nil {
			return fmt.Errorf("migrate: apply %q: %w", name, err)
		}
	}
	return nil
}

func isApplied(ctx context.Context, db *sql.DB, version string) (bool, error) {
	const q = `SELECT 1 FROM schema_migrations WHERE version = $1`
	var one int
	err := db.QueryRowContext(ctx, q, version).Scan(&one)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func applyMigration(ctx context.Context, db *sql.DB, version, ddl string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.ExecContext(ctx, ddl); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version) VALUES ($1)`, version,
	); err != nil {
		return err
	}
	return tx.Commit()
}
