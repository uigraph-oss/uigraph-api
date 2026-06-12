package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/org"
)

func scanInvitation(row interface{ Scan(...any) error }) (org.Invitation, error) {
	var inv org.Invitation
	var invitedBy sql.NullString
	err := row.Scan(
		&inv.ID, &inv.OrgID, &inv.Email, &inv.Role,
		&inv.Code, &invitedBy, &inv.Status,
		&inv.ExpiresAt, &inv.CreatedAt,
	)
	inv.InvitedBy = invitedBy.String
	return inv, err
}

func (d *DB) CreateInvitation(ctx context.Context, inv org.Invitation) error {
	const q = `
		INSERT INTO org_invitations
		    (id, org_id, email, role, code, invited_by, status, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6,'')::uuid, $7, $8, $9)`

	now := time.Now().UTC()
	if inv.CreatedAt.IsZero() {
		inv.CreatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q,
		inv.ID, inv.OrgID, inv.Email, inv.Role,
		inv.Code, inv.InvitedBy, inv.Status,
		inv.ExpiresAt, inv.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateInvitation: %w", err)
	}
	return nil
}

func (d *DB) GetInvitationByCode(ctx context.Context, code string) (*org.Invitation, error) {
	const q = `
		SELECT id, org_id, email, role, code, COALESCE(invited_by::text,''), status, expires_at, created_at
		FROM   org_invitations
		WHERE  code = $1`

	inv, err := scanInvitation(d.db.QueryRowContext(ctx, q, code))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetInvitationByCode: %w", err)
	}
	return &inv, nil
}

func (d *DB) GetInvitation(ctx context.Context, id string) (*org.Invitation, error) {
	const q = `
		SELECT id, org_id, email, role, code, COALESCE(invited_by::text,''), status, expires_at, created_at
		FROM   org_invitations
		WHERE  id = $1`

	inv, err := scanInvitation(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetInvitation: %w", err)
	}
	return &inv, nil
}

func (d *DB) ListInvitations(ctx context.Context, orgID string) ([]org.Invitation, error) {
	const q = `
		SELECT id, org_id, email, role, code, COALESCE(invited_by::text,''), status, expires_at, created_at
		FROM   org_invitations
		WHERE  org_id = $1
		ORDER  BY created_at DESC`

	rows, err := d.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListInvitations: %w", err)
	}
	defer rows.Close()

	var out []org.Invitation
	for rows.Next() {
		inv, err := scanInvitation(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListInvitations scan: %w", err)
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

func (d *DB) AcceptInvitation(ctx context.Context, id string) error {
	const q = `UPDATE org_invitations SET status = 'accepted' WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: AcceptInvitation: %w", err)
	}
	return nil
}

func (d *DB) RevokeInvitation(ctx context.Context, id string) error {
	const q = `UPDATE org_invitations SET status = 'revoked' WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: RevokeInvitation: %w", err)
	}
	return nil
}
