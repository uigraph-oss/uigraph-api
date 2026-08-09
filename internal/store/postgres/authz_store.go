package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/uigraph/app/internal/authz"
)

func (d *DB) GetOrgMember(ctx context.Context, userID, orgID string) (authz.OrgMember, error) {
	const q = `
		SELECT role, source
		FROM   org_members
		WHERE  user_id = $1
		  AND  org_id  = $2`

	var m authz.OrgMember
	m.UserID, m.OrgID = userID, orgID
	err := d.db.QueryRowContext(ctx, q, userID, orgID).Scan(&m.Role, &m.Source)
	if errors.Is(err, sql.ErrNoRows) {
		return authz.OrgMember{}, authz.ErrNotFound
	}
	if err != nil {
		return authz.OrgMember{}, fmt.Errorf("postgres: GetOrgMember: %w", err)
	}
	return m, nil
}

func (d *DB) UpsertOrgMember(ctx context.Context, userID, orgID string, role authz.Role, source string) error {
	const q = `
		INSERT INTO org_members (user_id, org_id, role, source, updated_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (user_id, org_id) DO UPDATE
			SET role       = EXCLUDED.role,
			    source     = EXCLUDED.source,
			    updated_at = NOW()`

	if _, err := d.db.ExecContext(ctx, q, userID, orgID, string(role), source); err != nil {
		return fmt.Errorf("postgres: UpsertOrgMember: %w", err)
	}
	return nil
}

func (d *DB) GetResourcePermission(
	ctx context.Context,
	userID, orgID string,
	rt authz.ResourceType, resourceID string,
) (authz.ResourcePermission, error) {
	const q = `
		SELECT role, source
		FROM   resource_permissions
		WHERE  user_id       = $1
		  AND  org_id        = $2
		  AND  resource_type = $3
		  AND  resource_id   = $4`

	var rp authz.ResourcePermission
	rp.UserID, rp.OrgID, rp.ResourceType, rp.ResourceID = userID, orgID, rt, resourceID
	err := d.db.QueryRowContext(ctx, q, userID, orgID, string(rt), resourceID).Scan(&rp.Role, &rp.Source)
	if errors.Is(err, sql.ErrNoRows) {
		return authz.ResourcePermission{}, authz.ErrNotFound
	}
	if err != nil {
		return authz.ResourcePermission{}, fmt.Errorf("postgres: GetResourcePermission: %w", err)
	}
	return rp, nil
}

func (d *DB) UpsertResourcePermission(
	ctx context.Context,
	userID, orgID string,
	rt authz.ResourceType, resourceID string,
	role authz.Role, source string,
) error {
	const q = `
		INSERT INTO resource_permissions (user_id, org_id, resource_type, resource_id, role, source, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (user_id, org_id, resource_type, resource_id) DO UPDATE
			SET role       = EXCLUDED.role,
			    source     = EXCLUDED.source,
			    updated_at = NOW()`

	if _, err := d.db.ExecContext(ctx, q, userID, orgID, string(rt), resourceID, string(role), source); err != nil {
		return fmt.Errorf("postgres: UpsertResourcePermission: %w", err)
	}
	return nil
}
