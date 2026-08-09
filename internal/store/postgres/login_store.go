package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/identity"
)

func (d *DB) UpsertAuthIdentity(ctx context.Context, a identity.AuthIdentity) error {
	const q = `
		INSERT INTO user_auth (user_id, provider, provider_sub, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (provider, provider_sub) DO UPDATE SET
		    user_id    = EXCLUDED.user_id,
		    updated_at = NOW()`

	if _, err := d.db.ExecContext(ctx, q, a.UserID, a.ProviderID, a.Sub); err != nil {
		return fmt.Errorf("postgres: UpsertAuthIdentity: %w", err)
	}
	return nil
}

func (d *DB) RecordLoginAttempt(ctx context.Context, username, ip string) error {
	const q = `INSERT INTO login_attempts (username, ip_address) VALUES ($1, $2)`

	if _, err := d.db.ExecContext(ctx, q, username, ip); err != nil {
		return fmt.Errorf("postgres: RecordLoginAttempt: %w", err)
	}
	return nil
}

func (d *DB) CountLoginAttempts(ctx context.Context, username string, since time.Time) (int, error) {
	const q = `SELECT COUNT(*) FROM login_attempts WHERE username = $1 AND created_at > $2`

	var n int
	if err := d.db.QueryRowContext(ctx, q, username, since).Scan(&n); err != nil {
		return 0, fmt.Errorf("postgres: CountLoginAttempts: %w", err)
	}
	return n, nil
}
