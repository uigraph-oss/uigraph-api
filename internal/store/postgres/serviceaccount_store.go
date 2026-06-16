package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
	"github.com/uigraph/app/internal/identity"
)

const saCols = `id, org_id, name, COALESCE(description,''), scopes, disabled,
	COALESCE(created_by::text,''), created_at, updated_at`

func scanSA(row interface{ Scan(...any) error }) (identity.ServiceAccount, error) {
	var sa identity.ServiceAccount
	err := row.Scan(
		&sa.ID, &sa.OrgID, &sa.Name, &sa.Description, pq.Array(&sa.Scopes), &sa.Disabled,
		&sa.CreatedBy, &sa.CreatedAt, &sa.UpdatedAt,
	)
	return sa, err
}

func (d *DB) CreateServiceAccount(ctx context.Context, sa identity.ServiceAccount) error {
	const q = `
		INSERT INTO service_accounts
		    (id, org_id, name, description, scopes, disabled, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, $8, $9)`

	now := time.Now().UTC()
	_, err := d.db.ExecContext(ctx, q,
		sa.ID, sa.OrgID, sa.Name, sa.Description, pq.Array(sa.Scopes), sa.Disabled,
		sa.CreatedBy, now, now,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateServiceAccount: %w", err)
	}
	return nil
}

func (d *DB) GetServiceAccount(ctx context.Context, id string) (*identity.ServiceAccount, error) {
	q := "SELECT " + saCols + " FROM service_accounts WHERE id = $1 AND deleted_at IS NULL"
	sa, err := scanSA(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetServiceAccount: %w", err)
	}
	return &sa, nil
}

func (d *DB) ListServiceAccounts(ctx context.Context, orgID string) ([]identity.ServiceAccount, error) {
	q := "SELECT " + saCols + " FROM service_accounts WHERE org_id=$1 AND deleted_at IS NULL ORDER BY created_at DESC"
	rows, err := d.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListServiceAccounts: %w", err)
	}
	defer rows.Close()

	var out []identity.ServiceAccount
	for rows.Next() {
		sa, err := scanSA(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListServiceAccounts scan: %w", err)
		}
		out = append(out, sa)
	}
	return out, rows.Err()
}

func (d *DB) UpdateServiceAccount(ctx context.Context, sa identity.ServiceAccount) error {
	const q = `
		UPDATE service_accounts
		SET    name        = $1,
		       description = $2,
		       scopes      = $3,
		       disabled    = $4,
		       updated_at  = NOW()
		WHERE  id = $5 AND deleted_at IS NULL`

	if _, err := d.db.ExecContext(ctx, q, sa.Name, sa.Description, pq.Array(sa.Scopes), sa.Disabled, sa.ID); err != nil {
		return fmt.Errorf("postgres: UpdateServiceAccount: %w", err)
	}
	return nil
}

func (d *DB) DeleteServiceAccount(ctx context.Context, id string) error {
	const q = `UPDATE service_accounts SET deleted_at = NOW() WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: DeleteServiceAccount: %w", err)
	}
	return nil
}

func (d *DB) CreateToken(ctx context.Context, t identity.Token) error {
	const q = `
		INSERT INTO service_account_tokens
		    (id, service_account_id, name, prefix, hash, expires_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, NULLIF($7, '')::uuid, $8)`

	now := time.Now().UTC()
	_, err := d.db.ExecContext(ctx, q,
		t.ID, t.ServiceAccountID, t.Name, t.Prefix, t.Hash,
		t.ExpiresAt, t.CreatedBy, now,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateToken: %w", err)
	}
	return nil
}

func (d *DB) GetTokenByPrefix(ctx context.Context, prefix string) (*identity.Token, error) {
	const q = `
		SELECT id, service_account_id, name, prefix, hash,
		       expires_at, last_used_at, revoked, created_at
		FROM   service_account_tokens
		WHERE  prefix = $1`

	var t identity.Token
	err := d.db.QueryRowContext(ctx, q, prefix).Scan(
		&t.ID, &t.ServiceAccountID, &t.Name, &t.Prefix, &t.Hash,
		&t.ExpiresAt, &t.LastUsedAt, &t.Revoked, &t.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetTokenByPrefix: %w", err)
	}
	return &t, nil
}

func (d *DB) ListTokens(ctx context.Context, serviceAccountID string) ([]identity.Token, error) {
	const q = `
		SELECT id, service_account_id, name, prefix, hash,
		       expires_at, last_used_at, revoked, created_at
		FROM   service_account_tokens
		WHERE  service_account_id = $1
		ORDER BY created_at DESC`

	rows, err := d.db.QueryContext(ctx, q, serviceAccountID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTokens: %w", err)
	}
	defer rows.Close()

	var out []identity.Token
	for rows.Next() {
		var t identity.Token
		if err := rows.Scan(
			&t.ID, &t.ServiceAccountID, &t.Name, &t.Prefix, &t.Hash,
			&t.ExpiresAt, &t.LastUsedAt, &t.Revoked, &t.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListTokens scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) RevokeToken(ctx context.Context, tokenID string) error {
	const q = `UPDATE service_account_tokens SET revoked = TRUE WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, tokenID); err != nil {
		return fmt.Errorf("postgres: RevokeToken: %w", err)
	}
	return nil
}

func (d *DB) TouchToken(ctx context.Context, tokenID string) error {
	const q = `UPDATE service_account_tokens SET last_used_at = NOW() WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, tokenID); err != nil {
		return fmt.Errorf("postgres: TouchToken: %w", err)
	}
	return nil
}
