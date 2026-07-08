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
	var logoAssetID sql.NullString
	if err := row.Scan(&o.ID, &o.Name, &logoAssetID, &o.Disabled, &o.AutoJoin, &o.OnboardingDone, &o.CreatedAt, &o.UpdatedAt); err != nil {
		return o, err
	}
	if logoAssetID.Valid {
		o.LogoAssetID = &logoAssetID.String
	}
	return o, nil
}

func (d *DB) CreateOrg(ctx context.Context, o org.Org) error {
	const q = `
		INSERT INTO orgs (id, name, logo_asset_id, disabled, auto_join, onboarding_done, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	now := time.Now().UTC()
	if o.CreatedAt.IsZero() {
		o.CreatedAt = now
	}
	if o.UpdatedAt.IsZero() {
		o.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q, o.ID, o.Name, o.LogoAssetID, o.Disabled, o.AutoJoin, o.OnboardingDone, o.CreatedAt, o.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: CreateOrg: %w", err)
	}
	return nil
}

func (d *DB) GetOrg(ctx context.Context, id string) (*org.Org, error) {
	const q = `SELECT id, name, logo_asset_id, disabled, auto_join, onboarding_done, created_at, updated_at FROM orgs WHERE id = $1`
	o, err := scanOrg(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetOrg: %w", err)
	}
	return &o, nil
}

func (d *DB) ListOrgs(ctx context.Context) ([]org.Org, error) {
	const q = `SELECT id, name, logo_asset_id, disabled, auto_join, onboarding_done, created_at, updated_at FROM orgs ORDER BY name`
	rows, err := d.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListOrgs: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (d *DB) ListAutoJoinOrgs(ctx context.Context) ([]org.Org, error) {
	const q = `SELECT id, name, logo_asset_id, disabled, auto_join, onboarding_done, created_at, updated_at FROM orgs WHERE auto_join = TRUE AND disabled = FALSE ORDER BY name`
	rows, err := d.db.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListAutoJoinOrgs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []org.Org
	for rows.Next() {
		o, err := scanOrg(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListAutoJoinOrgs scan: %w", err)
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
		       disabled   = $2,
		       auto_join  = $3,
		       updated_at = NOW()
		WHERE  id = $4`

	if _, err := d.db.ExecContext(ctx, q, o.Name, o.Disabled, o.AutoJoin, o.ID); err != nil {
		return fmt.Errorf("postgres: UpdateOrg: %w", err)
	}
	return nil
}

func (d *DB) SetOrgLogo(ctx context.Context, id string, assetID *string) error {
	const q = `UPDATE orgs SET logo_asset_id = $1, updated_at = NOW() WHERE id = $2`
	if _, err := d.db.ExecContext(ctx, q, assetID, id); err != nil {
		return fmt.Errorf("postgres: SetOrgLogo: %w", err)
	}
	return nil
}

func (d *DB) SetOnboardingDone(ctx context.Context, id string) error {
	const q = `UPDATE orgs SET onboarding_done = TRUE, updated_at = NOW() WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: SetOnboardingDone: %w", err)
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
