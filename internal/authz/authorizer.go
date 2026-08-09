package authz

import (
	"context"
	"errors"

	"github.com/uigraph/app/internal/org"
)

// Authorizer evaluates RBAC decisions for UIGraph.
type Authorizer interface {
	// ScopesForUser resolves the user's org role into its explicit scope set.
	// Returns nil (deny everything) when the user has no membership in orgID.
	ScopesForUser(ctx context.Context, userID, orgID string) ([]Scope, error)

	// IsUserServerAdmin checks if the user has the instance-level server_admin role.
	// This is a separate axis from org scopes and governs only global endpoints.
	IsUserServerAdmin(ctx context.Context, userID string) (bool, error)
}

// rbacQuerier is the narrow subset of RBACStore that the authorizer actually uses.
// Any value satisfying RBACStore (or store.Store) satisfies this too.
type rbacQuerier interface {
	GetOrgMember(ctx context.Context, userID, orgID string) (OrgMember, error)
	UpsertOrgMember(ctx context.Context, userID, orgID string, role Role, source string) error
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

func (a *authorizer) ScopesForUser(ctx context.Context, userID, orgID string) ([]Scope, error) {
	m, err := a.store.GetOrgMember(ctx, userID, orgID)
	if errors.Is(err, ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ScopesForRole(m.Role), nil
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
