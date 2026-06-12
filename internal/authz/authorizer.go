package authz

import (
	"context"
	"errors"

	"github.com/uigraph/app/internal/org"
)

// Authorizer evaluates RBAC decisions for UIGraph.
type Authorizer interface {
	// HasOrgRole returns true if userID holds at least minimum at org level.
	HasOrgRole(ctx context.Context, userID, orgID string, minimum Role) (bool, error)

	// HasResourceRole returns true if userID holds at least minimum for the
	// given resource. Falls back to org-level role when no resource-level row
	// exists (resolution order: resource → org → deny).
	HasResourceRole(ctx context.Context, userID, orgID string, rt ResourceType, resourceID string, minimum Role) (bool, error)

	// SyncRolesFromClaims upserts org and resource roles from IdP token claims.
	// Called on every SSO login after identity is established.
	SyncRolesFromClaims(ctx context.Context, userID, orgID string, claims map[string]any) error

	// IsUserServerAdmin checks if the user has the instance-level server_admin role.
	IsUserServerAdmin(ctx context.Context, userID string) (bool, error)
}

// rbacQuerier is the narrow subset of RBACStore that the authorizer actually uses.
// Any value satisfying RBACStore (or store.Store) satisfies this too.
type rbacQuerier interface {
	GetOrgMember(ctx context.Context, userID, orgID string) (OrgMember, error)
	UpsertOrgMember(ctx context.Context, userID, orgID string, role Role, source string) error
	GetResourcePermission(ctx context.Context, userID, orgID string, rt ResourceType, resourceID string) (ResourcePermission, error)
	UpsertResourcePermission(ctx context.Context, userID, orgID string, rt ResourceType, resourceID string, role Role, source string) error
	GetSSOMappings(ctx context.Context, orgID string) ([]SSOMapping, error)
}

type userRoleQuerier interface {
	GetUser(ctx context.Context, id string) (*org.User, error)
	GetUserByLogin(ctx context.Context, login string) (*org.User, error)
}

type authorizer struct {
	store    rbacQuerier
	userRole userRoleQuerier
}

// New returns an Authorizer. Any value that satisfies RBACStore (including store.Store) works.
func New(store rbacQuerier, users userRoleQuerier) Authorizer {
	return &authorizer{store: store, userRole: users}
}

func (a *authorizer) HasOrgRole(ctx context.Context, userID, orgID string, minimum Role) (bool, error) {
	m, err := a.store.GetOrgMember(ctx, userID, orgID)
	if errors.Is(err, ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return m.Role.AtLeast(minimum), nil
}

func (a *authorizer) HasResourceRole(
	ctx context.Context,
	userID, orgID string,
	rt ResourceType, resourceID string,
	minimum Role,
) (bool, error) {
	// Step 1: resource-level override.
	rp, err := a.store.GetResourcePermission(ctx, userID, orgID, rt, resourceID)
	if err == nil {
		return rp.Role.AtLeast(minimum), nil
	}
	if !errors.Is(err, ErrNotFound) {
		return false, err
	}

	// Step 2: fall back to org-level.
	return a.HasOrgRole(ctx, userID, orgID, minimum)
}

func (a *authorizer) SyncRolesFromClaims(ctx context.Context, userID, orgID string, claims map[string]any) error {
	mappings, err := a.store.GetSSOMappings(ctx, orgID)
	if err != nil {
		return err
	}
	for _, m := range mappings {
		if !claimMatches(claims, m.ClaimKey, m.ClaimValue) {
			continue
		}
		if m.Scope == "org" {
			if err = a.store.UpsertOrgMember(ctx, userID, orgID, m.Role, "sso"); err != nil {
				return err
			}
		} else {
			if err = a.store.UpsertResourcePermission(ctx, userID, orgID, m.ResourceType, m.ResourceID, m.Role, "sso"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *authorizer) IsUserServerAdmin(ctx context.Context, userID string) (bool, error) {
	u, err := a.userRole.GetUser(ctx, userID)
	if err != nil {
		return false, err
	}
	if u == nil {
		return false, nil
	}
	return u.Role == "server_admin", nil
}
