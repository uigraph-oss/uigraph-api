package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/uigraph/app/internal/org"
)

func scanTeam(row interface{ Scan(...any) error }) (org.Team, error) {
	var t org.Team
	var email, extID sql.NullString
	err := row.Scan(&t.ID, &t.OrgID, &t.Name, &email, &extID, &t.CreatedAt, &t.UpdatedAt)
	t.Email = email.String
	t.ExternalID = extID.String
	return t, err
}

func (d *DB) CreateTeam(ctx context.Context, t org.Team) error {
	const q = `
		INSERT INTO teams (id, org_id, name, email, external_id, created_at, updated_at)
		VALUES ($1, $2, $3, NULLIF($4,''), NULLIF($5,''), $6, $7)`

	now := time.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.UpdatedAt.IsZero() {
		t.UpdatedAt = now
	}
	_, err := d.db.ExecContext(ctx, q, t.ID, t.OrgID, t.Name, t.Email, t.ExternalID, t.CreatedAt, t.UpdatedAt)
	if err != nil {
		return fmt.Errorf("postgres: CreateTeam: %w", err)
	}
	return nil
}

func (d *DB) GetTeam(ctx context.Context, id string) (*org.Team, error) {
	const q = `
		SELECT id, org_id, name, email, external_id, created_at, updated_at
		FROM   teams WHERE id = $1`

	t, err := scanTeam(d.db.QueryRowContext(ctx, q, id))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("postgres: GetTeam: %w", err)
	}
	return &t, nil
}

func (d *DB) ListTeams(ctx context.Context, orgID string) ([]org.Team, error) {
	const q = `
		SELECT id, org_id, name, email, external_id, created_at, updated_at
		FROM   teams
		WHERE  org_id = $1
		ORDER  BY name`

	rows, err := d.db.QueryContext(ctx, q, orgID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTeams: %w", err)
	}
	defer rows.Close()

	var out []org.Team
	for rows.Next() {
		t, err := scanTeam(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: ListTeams scan: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (d *DB) UpdateTeam(ctx context.Context, t org.Team) error {
	const q = `
		UPDATE teams
		SET    name        = $1,
		       email       = NULLIF($2, ''),
		       external_id = NULLIF($3, ''),
		       updated_at  = NOW()
		WHERE  id = $4`

	if _, err := d.db.ExecContext(ctx, q, t.Name, t.Email, t.ExternalID, t.ID); err != nil {
		return fmt.Errorf("postgres: UpdateTeam: %w", err)
	}
	return nil
}

func (d *DB) DeleteTeam(ctx context.Context, id string) error {
	const q = `DELETE FROM teams WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: DeleteTeam: %w", err)
	}
	return nil
}

func (d *DB) AddTeamMember(ctx context.Context, m org.TeamMember) error {
	const q = `
		INSERT INTO team_members (team_id, user_id, org_id, permission, created_at, updated_at)
		VALUES ($1, $2, $3, $4, NOW(), NOW())
		ON CONFLICT (team_id, user_id) DO UPDATE SET
		    permission = EXCLUDED.permission,
		    updated_at = NOW()`

	_, err := d.db.ExecContext(ctx, q, m.TeamID, m.UserID, m.OrgID, m.Permission)
	if err != nil {
		return fmt.Errorf("postgres: AddTeamMember: %w", err)
	}
	return nil
}

func (d *DB) ListTeamMembers(ctx context.Context, teamID string) ([]org.TeamMember, error) {
	const q = `
		SELECT team_id, user_id, org_id, permission, created_at, updated_at
		FROM   team_members
		WHERE  team_id = $1
		ORDER  BY created_at`

	rows, err := d.db.QueryContext(ctx, q, teamID)
	if err != nil {
		return nil, fmt.Errorf("postgres: ListTeamMembers: %w", err)
	}
	defer rows.Close()

	var out []org.TeamMember
	for rows.Next() {
		var m org.TeamMember
		if err := rows.Scan(&m.TeamID, &m.UserID, &m.OrgID, &m.Permission, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, fmt.Errorf("postgres: ListTeamMembers scan: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (d *DB) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	const q = `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`
	if _, err := d.db.ExecContext(ctx, q, teamID, userID); err != nil {
		return fmt.Errorf("postgres: RemoveTeamMember: %w", err)
	}
	return nil
}
