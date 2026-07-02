package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uigraph/app/internal/org"
)

func scanMember(row interface{ Scan(...any) error }) (org.OrgMember, error) {
	var m org.OrgMember
	return m, row.Scan(&m.UserID, &m.OrgID, &m.Role, &m.Source, &m.CreatedAt, &m.UpdatedAt)
}

func (d *DB) AddMember(ctx context.Context, m org.OrgMember) error {
	const q = `
		INSERT INTO org_members (user_id, org_id, role, source, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())`

	_, err := d.db.ExecContext(ctx, q, m.UserID, m.OrgID, m.Role, m.Source)
	if err != nil {
		return fmt.Errorf("postgres: AddMember: %w", err)
	}
	return nil
}

func (d *DB) GetMember(ctx context.Context, userID, orgID string) (*org.OrgMember, error) {
	const q = `
		SELECT user_id, org_id, role, source, created_at, updated_at
		FROM   org_members
		WHERE  user_id = $1 AND org_id = $2`

	m, err := scanMember(d.db.QueryRowContext(ctx, q, userID, orgID))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetMember: %w", err)
	}
	return &m, nil
}

func scanMemberEnriched(row interface{ Scan(...any) error }) (org.OrgMember, error) {
	var m org.OrgMember
	var teamID sql.NullString
	if err := row.Scan(
		&m.UserID, &m.OrgID, &m.Role, &m.Source, &m.Email, &m.Name,
		&teamID, &m.CreatedAt, &m.UpdatedAt,
	); err != nil {
		return m, err
	}
	if teamID.Valid {
		m.TeamID = &teamID.String
	}
	return m, nil
}

func (d *DB) ListMembers(ctx context.Context, orgID string) ([]org.OrgMember, error) {
	const q = `
		SELECT m.user_id, m.org_id, m.role, m.source,
		       u.email, u.name,
		       tm.team_id,
		       m.created_at, m.updated_at
		FROM   org_members m
		JOIN   users u ON u.id = m.user_id
		LEFT JOIN LATERAL (
			SELECT team_id FROM team_members
			WHERE  user_id = m.user_id AND org_id = m.org_id
			ORDER  BY created_at ASC
			LIMIT  1
		) tm ON true
		WHERE  m.org_id = $1
		ORDER  BY m.created_at`

	rows, err := d.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListMembers: %w", err)
	}
	defer rows.Close()

	var out []org.OrgMember
	for rows.Next() {
		m, err := scanMemberEnriched(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListMembers scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) UpdateMemberRole(ctx context.Context, userID, orgID, role, source string) error {
	const q = `
		UPDATE org_members
		SET    role = $1, source = $2, updated_at = NOW()
		WHERE  user_id = $3 AND org_id = $4`

	if _, err := d.db.ExecContext(ctx, q, role, source, userID, orgID); err != nil {
		return fmt.Errorf("postgres: UpdateMemberRole: %w", err)
	}
	return nil
}

func (d *DB) ListOrgsForUser(ctx context.Context, userID string) ([]org.OrgMembershipView, error) {
	const q = `
		SELECT o.id, o.name, o.logo_asset_id, o.disabled, o.created_at, o.updated_at, m.role
		FROM   org_members m
		JOIN   orgs o ON o.id = m.org_id
		WHERE  m.user_id = $1
		ORDER  BY o.name`

	rows, err := d.db.QueryContext(ctx, q, userID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListOrgsForUser: %w", err)
	}
	defer rows.Close()

	var out []org.OrgMembershipView
	for rows.Next() {
		var v org.OrgMembershipView
		var logoAssetID sql.NullString
		if err := rows.Scan(
			&v.Org.ID, &v.Org.Name, &logoAssetID, &v.Org.Disabled,
			&v.Org.CreatedAt, &v.Org.UpdatedAt, &v.Role,
		); err != nil {
			return nil, fmt.Errorf("postgres: ListOrgsForUser scan: %w", err)
		}
		if logoAssetID.Valid {
			v.Org.LogoAssetID = &logoAssetID.String
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (d *DB) RemoveMember(ctx context.Context, userID, orgID string) error {
	const q = `DELETE FROM org_members WHERE user_id = $1 AND org_id = $2`
	if _, err := d.db.ExecContext(ctx, q, userID, orgID); err != nil {
		return fmt.Errorf("postgres: RemoveMember: %w", err)
	}
	return nil
}
