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

func (d *DB) CreateSSOMapping(ctx context.Context, m authz.SSOMapping) error {
	const q = `
		INSERT INTO sso_role_mappings
		    (id, org_id, claim_key, claim_value, role, scope, resource_type, resource_id, created_at)
		VALUES ($1, NULLIF($2, '')::uuid, $3, $4, $5, $6, NULLIF($7, ''), NULLIF($8, ''), NOW())`

	_, err := d.db.ExecContext(ctx, q,
		m.ID, m.OrgID, m.ClaimKey, m.ClaimValue, string(m.Role), m.Scope,
		string(m.ResourceType), m.ResourceID,
	)
	if err != nil {
		return fmt.Errorf("postgres: CreateSSOMapping: %w", err)
	}
	return nil
}

func (d *DB) ListSSOMappings(ctx context.Context, orgID string) ([]authz.SSOMapping, error) {
	return d.GetSSOMappings(ctx, orgID)
}

func (d *DB) DeleteSSOMapping(ctx context.Context, id string) error {
	const q = `DELETE FROM sso_role_mappings WHERE id = $1`
	if _, err := d.db.ExecContext(ctx, q, id); err != nil {
		return fmt.Errorf("postgres: DeleteSSOMapping: %w", err)
	}
	return nil
}

func (d *DB) GetSSOMappings(ctx context.Context, orgID string) ([]authz.SSOMapping, error) {
	var q string
	var args []interface{}
	if orgID == "" {
		q = `
			SELECT id, COALESCE(org_id::text, ''), claim_key, claim_value, role, scope,
			       COALESCE(resource_type::text, ''),
			       COALESCE(resource_id::text, ''),
			       created_at
			FROM   sso_role_mappings
			WHERE  org_id IS NULL`
		args = []interface{}{}
	} else {
		q = `
			SELECT id, COALESCE(org_id::text, ''), claim_key, claim_value, role, scope,
			       COALESCE(resource_type::text, ''),
			       COALESCE(resource_id::text, ''),
			       created_at
			FROM   sso_role_mappings
			WHERE  org_id = $1`
		args = []interface{}{orgID}
	}
	rows, err := d.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: GetSSOMappings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []authz.SSOMapping
	for rows.Next() {
		var m authz.SSOMapping
		var rt string
		if err := rows.Scan(&m.ID, &m.OrgID, &m.ClaimKey, &m.ClaimValue, &m.Role, &m.Scope, &rt, &m.ResourceID, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("postgres: GetSSOMappings scan: %w", err)
		}
		m.ResourceType = authz.ResourceType(rt)
		out = append(out, m)
	}
	return out, rows.Err()
}
