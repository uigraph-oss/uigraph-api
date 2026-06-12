package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/identity"
)

func scanSession(row interface{ Scan(...any) error }) (identity.Session, error) {
	var s identity.Session
	var prevHash, userAgent, clientIP, samlIdx, samlHash sql.NullString
	var seenAt sql.NullTime
	err := row.Scan(
		&s.ID, &s.UserID,
		&s.TokenHash, &prevHash,
		&s.AuthTokenSeen, &seenAt,
		&userAgent, &clientIP,
		&s.RotatedAt, &s.CreatedAt, &s.ExpiresAt, &s.LastActiveAt,
		&samlIdx, &samlHash, &s.AuthProvider,
	)
	if err != nil {
		return identity.Session{}, err
	}
	s.PrevTokenHash = prevHash.String
	s.UserAgent = userAgent.String
	s.ClientIP = clientIP.String
	s.SAMLSessionIdx = samlIdx.String
	s.SAMLNameIDHash = samlHash.String
	if seenAt.Valid {
		s.SeenAt = &seenAt.Time
	}
	return s, nil
}

const sessionCols = `
	id, user_id,
	token_hash, prev_token_hash,
	auth_token_seen, seen_at,
	user_agent, client_ip,
	rotated_at, created_at, expires_at, last_active_at,
	saml_session_index, saml_name_id_hash, auth_provider`

func (d *DB) CreateSession(ctx context.Context, s identity.Session) error {
	const q = `
		INSERT INTO user_sessions
		    (id, user_id, token_hash, prev_token_hash, auth_token_seen,
		     seen_at, user_agent, client_ip, rotated_at, created_at, expires_at,
		     last_active_at, saml_session_index, saml_name_id_hash, auth_provider)
		VALUES ($1,$2,$3,NULLIF($4,''),$5,
		        $6,NULLIF($7,''),NULLIF($8,''),$9,$10,$11,
		        $12,NULLIF($13,''),NULLIF($14,''),$15)`

	now := time.Now().UTC()
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.RotatedAt.IsZero() {
		s.RotatedAt = now
	}
	if s.LastActiveAt.IsZero() {
		s.LastActiveAt = now
	}
	if s.AuthProvider == "" {
		s.AuthProvider = "password"
	}
	_, err := d.db.ExecContext(ctx, q,
		s.ID, s.UserID, s.TokenHash, s.PrevTokenHash, s.AuthTokenSeen,
		s.SeenAt, s.UserAgent, s.ClientIP, s.RotatedAt, s.CreatedAt, s.ExpiresAt,
		s.LastActiveAt, s.SAMLSessionIdx, s.SAMLNameIDHash, s.AuthProvider,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateSession: %w", err)
	}
	return nil
}

func (d *DB) GetSessionByToken(ctx context.Context, hash string) (*identity.Session, error) {
	q := "SELECT " + sessionCols + `
		FROM  user_sessions
		WHERE (token_hash = $1 OR (prev_token_hash = $1 AND auth_token_seen = FALSE))
		  AND expires_at > NOW()
		LIMIT 1`

	s, err := scanSession(d.db.QueryRowContext(ctx, q, hash))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSessionByToken: %w", err)
	}
	return &s, nil
}

func (d *DB) RotateSession(ctx context.Context, id, newHash, prevHash string) error {
	const q = `
		UPDATE user_sessions
		SET    token_hash      = $2,
		       prev_token_hash = $3,
		       auth_token_seen = FALSE,
		       rotated_at      = NOW(),
		       last_active_at  = NOW()
		WHERE  id = $1`

	if _, err := d.db.ExecContext(ctx, q, id, newHash, prevHash); err != nil {
		return fmt.Errorf("postgres: RotateSession: %w", err)
	}
	return nil
}

func (d *DB) MarkTokenSeen(ctx context.Context, id string) error {
	const q = `UPDATE user_sessions SET auth_token_seen = TRUE, seen_at = NOW() WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: MarkTokenSeen: %w", err)
	}
	return nil
}

func (d *DB) DeleteSession(ctx context.Context, id string) error {
	const q = `DELETE FROM user_sessions WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: DeleteSession: %w", err)
	}
	return nil
}

func (d *DB) DeleteUserSessions(ctx context.Context, userID string) error {
	const q = `DELETE FROM user_sessions WHERE user_id = $1`
	if _, err := d.db.ExecContext(ctx, q, userID); err != nil {
		return fmt.Errorf("postgres: DeleteUserSessions: %w", err)
	}
	return nil
}

func (d *DB) ListUserSessions(ctx context.Context, userID string) ([]identity.Session, error) {
	q := "SELECT " + sessionCols + `
		FROM  user_sessions
		WHERE user_id = $1
		ORDER BY last_active_at DESC`

	rows, err := d.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListUserSessions: %w", err)
	}
	defer rows.Close()

	var out []identity.Session
	for rows.Next() {
		s, err := scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListUserSessions scan: %w", err)
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
