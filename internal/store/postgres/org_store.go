package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/org"
)

func scanOrg(row interface{ Scan(...any) error }) (org.Org, error) {
	var o org.Org
	return o, row.Scan(&o.ID, &o.Name, &o.Slug, &o.Disabled, &o.CreatedAt, &o.UpdatedAt)
}

func (d *DB) CreateOrg(ctx context.Context, o org.Org) error {
	const q = `
		INSERT INTO orgs (id, name, slug, disabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`

	now := time.Now().UTC()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = now
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q, o.ID, o.Name, o.Slug, o.Disabled, o.CreatedAt, o.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: CreateOrg: %w", err)
	}
	return nil
}

func (d *DB) GetOrg(ctx context.Context, id string) (*org.Org, error) {
	const q = `SELECT id, name, slug, disabled, created_at, updated_at FROM orgs WHERE id = $1`
	o, err := scanOrg(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetOrg: %w", err)
	}
	return &o, nil
}

func (d *DB) GetOrgBySlug(ctx context.Context, slug string) (*org.Org, error) {
	const q = `SELECT id, name, slug, disabled, created_at, updated_at FROM orgs WHERE slug = $1`
	o, err := scanOrg(d.db.QueryRowContext(ctx, q, slug))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetOrgBySlug: %w", err)
	}
	return &o, nil
}

func (d *DB) ListOrgs(ctx context.Context) ([]org.Org, error) {
	const q = `SELECT id, name, slug, disabled, created_at, updated_at FROM orgs ORDER BY name`
	rows, err := d.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListOrgs: %w", err)
	}
	defer rows.Close()

	var out []org.Org
	for rows.Next() {
		o, err := scanOrg(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListOrgs scan: %w", err)
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (d *DB) CountAllOrgs(ctx context.Context) (int, error) {
	var n int
	if err := d.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM orgs`).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres: CountAllOrgs: %w", err)
	}
	return n, nil
}

func (d *DB) UpdateOrg(ctx context.Context, o org.Org) error {
	const q = `
		UPDATE orgs
		SET    name       = $1,
		       slug       = $2,
		       disabled   = $3,
		       updated_at = NOW()
		WHERE  id = $4`

	if _, err := d.db.ExecContext(ctx, q, o.Name, o.Slug, o.Disabled, o.ID); err != nil {
		return fmt.Errorf("postgres: UpdateOrg: %w", err)
	}
	return nil
}

func (d *DB) DeleteOrg(ctx context.Context, id string) error {
	const q = `DELETE FROM orgs WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: DeleteOrg: %w", err)
	}
	return nil
}

func (d *DB) AnyOrgExists(ctx context.Context) (bool, error) {
	const q = `SELECT EXISTS(SELECT 1 FROM orgs LIMIT 1)`
	var exists bool
	if err := d.db.QueryRowContext(ctx, q).Scan(&exists); err != nil {
		return false, fmt.Errorf("postgres: AnyOrgExists: %w", err)
	}
	return exists, nil
}
