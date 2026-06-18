package authz

import (
	"context"
	"time"
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

// SSOMapping maps an IdP claim key/value pair to a UIGraph role.
type SSOMapping struct {
	ID           string       `json:"id"`
	OrgID        string       `json:"orgId"`
	ClaimKey     string       `json:"claimKey"`
	ClaimValue   string       `json:"claimValue"`
	Role         Role         `json:"role"`
	Scope        string       `json:"scope"`
	ResourceType ResourceType `json:"resourceType,omitempty"`
	ResourceID   string       `json:"resourceId,omitempty"`
	CreatedAt    time.Time    `json:"createdAt"`
}

// RBACStore is the persistence interface for role resolution and SSO mapping management.
// The postgres implementation lives in store/postgres.
type RBACStore interface {
	GetOrgMember(ctx context.Context, userID, orgID string) (OrgMember, error)
	UpsertOrgMember(ctx context.Context, userID, orgID string, role Role, source string) error

	GetResourcePermission(ctx context.Context, userID, orgID string, rt ResourceType, resourceID string) (ResourcePermission, error)
	UpsertResourcePermission(ctx context.Context, userID, orgID string, rt ResourceType, resourceID string, role Role, source string) error

	// SSO mapping reads — used by Authorizer.SyncRolesFromClaims.
	GetSSOMappings(ctx context.Context, orgID string) ([]SSOMapping, error)

	// SSO mapping CRUD — used by the SSO admin API.
	CreateSSOMapping(ctx context.Context, m SSOMapping) error
	ListSSOMappings(ctx context.Context, orgID string) ([]SSOMapping, error)
	DeleteSSOMapping(ctx context.Context, id string) error
}
