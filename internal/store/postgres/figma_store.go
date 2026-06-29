package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uigraph/app/internal/identity"
)

func (d *DB) GetFigmaAuth(ctx context.Context, userID string) (*identity.FigmaAuth, error) {
	const q = `
		SELECT user_id, provider_sub,
		       COALESCE(access_token,''), COALESCE(refresh_token,''),
		       token_expires_at
		FROM   user_auth
		WHERE  user_id = $1 AND provider = $2`

	var a identity.FigmaAuth
	var expires sql.NullTime
	err := d.db.QueryRowContext(ctx, q, userID, identity.FigmaProvider).Scan(
		&a.UserID, &a.FigmaUserID, &a.AccessToken, &a.RefreshToken, &expires,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetFigmaAuth: %w", err)
	}
	a.ExpiresAt = expires.Time
	return &a, nil
}

func (d *DB) UpsertFigmaAuth(ctx context.Context, a identity.FigmaAuth) error {
	const q = `
		INSERT INTO user_auth
		    (user_id, provider, provider_sub, access_token, refresh_token,
		     token_type, token_expires_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, 'Bearer', $6, NOW(), NOW())
		ON CONFLICT (provider, provider_sub) DO UPDATE SET
		    user_id          = EXCLUDED.user_id,
		    access_token     = EXCLUDED.access_token,
		    refresh_token    = EXCLUDED.refresh_token,
		    token_expires_at = EXCLUDED.token_expires_at,
		    updated_at       = NOW()`

	_, err := d.db.ExecContext(ctx, q,
		a.UserID, identity.FigmaProvider, a.FigmaUserID,
		a.AccessToken, a.RefreshToken, a.ExpiresAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: UpsertFigmaAuth: %w", err)
	}
	return nil
}

func (d *DB) DeleteFigmaAuth(ctx context.Context, userID string) error {
	const q = `DELETE FROM user_auth WHERE user_id = $1 AND provider = $2`
	if _, err := d.db.ExecContext(ctx, q, userID, identity.FigmaProvider); err != nil {
		return fmt.Errorf("postgres: DeleteFigmaAuth: %w", err)
	}
	return nil
}
