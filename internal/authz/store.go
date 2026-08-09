package authz

import (
	"context"
)

// OrgMember holds a user's org-level role.
type OrgMember struct {
	UserID string
	OrgID  string
	Role   Role
	Source string // "manual" | "sso"
}

// ResourcePermission is a per-resource role override for a specific user.
type ResourcePermission struct {
	UserID       string
	OrgID        string
	ResourceType ResourceType
	ResourceID   string
	Role         Role
	Source       string // "manual" | "sso"
}

// RBACStore is the persistence interface for role resolution.
// The postgres implementation lives in store/postgres.
//
// IdP claim → role mapping is not here: it is owned by an auth provider and
// evaluated in the login path via internal/rolemap, against rules read through
// identity.ProviderStore.
type RBACStore interface {
	GetOrgMember(ctx context.Context, userID, orgID string) (OrgMember, error)
	UpsertOrgMember(ctx context.Context, userID, orgID string, role Role, source string) error

	GetResourcePermission(ctx context.Context, userID, orgID string, rt ResourceType, resourceID string) (ResourcePermission, error)
	UpsertResourcePermission(ctx context.Context, userID, orgID string, rt ResourceType, resourceID string, role Role, source string) error
}
