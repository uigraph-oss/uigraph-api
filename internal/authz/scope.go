package authz

import "strings"

// Scope is a named permission in the form "<resource>:<action>" (e.g.
// "diagrams:write"). Both service-account tokens and human users (via their
// org role, resolved through RoleScopes) are authorized against scopes.
//
// A granted scope may also be a per-resource wildcard "<resource>:*", which
// satisfies any concrete action of that resource. Required scopes are always
// concrete. Non-deterministic catch-alls (e.g. "all:*", "manage:*") are not valid.
type Scope string

const (
	ScopeDiagramsRead  Scope = "diagrams:read"
	ScopeDiagramsWrite Scope = "diagrams:write"

	ScopeDocsRead  Scope = "docs:read"
	ScopeDocsWrite Scope = "docs:write"

	ScopeMapsRead  Scope = "maps:read"
	ScopeMapsWrite Scope = "maps:write"

	ScopeServicesRead  Scope = "services:read"
	ScopeServicesWrite Scope = "services:write"

	ScopeFoldersRead  Scope = "folders:read"
	ScopeFoldersWrite Scope = "folders:write"

	ScopeMembersRead       Scope = "members:read"
	ScopeMembersAdd        Scope = "members:add"
	ScopeMembersRemove     Scope = "members:remove"
	ScopeMembersUpdateRole Scope = "members:update-role"

	ScopeTeamsRead         Scope = "teams:read"
	ScopeTeamsCreate       Scope = "teams:create"
	ScopeTeamsEdit         Scope = "teams:edit"
	ScopeTeamsDelete       Scope = "teams:delete"
	ScopeTeamsAddMember    Scope = "teams:add-member"
	ScopeTeamsRemoveMember Scope = "teams:remove-member"

	ScopeServiceAccountsRead        Scope = "serviceaccounts:read"
	ScopeServiceAccountsCreate      Scope = "serviceaccounts:create"
	ScopeServiceAccountsEdit        Scope = "serviceaccounts:edit"
	ScopeServiceAccountsDelete      Scope = "serviceaccounts:delete"
	ScopeServiceAccountsCreateToken Scope = "serviceaccounts:create-token"
	ScopeServiceAccountsRevokeToken Scope = "serviceaccounts:revoke-token"

	ScopeOrgUpdate Scope = "org:update"
	ScopeOrgDelete Scope = "org:delete"
)

// AllScopes is the catalog of concrete grantable scopes, returned by the discovery endpoint.
var AllScopes = []Scope{
	ScopeDiagramsRead, ScopeDiagramsWrite,
	ScopeDocsRead, ScopeDocsWrite,
	ScopeMapsRead, ScopeMapsWrite,
	ScopeServicesRead, ScopeServicesWrite,
	ScopeFoldersRead, ScopeFoldersWrite,
	ScopeMembersRead, ScopeMembersAdd, ScopeMembersRemove, ScopeMembersUpdateRole,
	ScopeTeamsRead, ScopeTeamsCreate, ScopeTeamsEdit, ScopeTeamsDelete, ScopeTeamsAddMember, ScopeTeamsRemoveMember,
	ScopeServiceAccountsRead, ScopeServiceAccountsCreate, ScopeServiceAccountsEdit, ScopeServiceAccountsDelete,
	ScopeServiceAccountsCreateToken, ScopeServiceAccountsRevokeToken,
	ScopeOrgUpdate, ScopeOrgDelete,
}

// RoleScopes is the static map from an org role to its explicit scope set.
// Each slice is written out literally — there is no inheritance between roles.
// The admin role uses per-resource wildcards, which are deterministic.
var RoleScopes = map[Role][]Scope{
	RoleViewer: {
		ScopeDiagramsRead, ScopeDocsRead, ScopeMapsRead, ScopeServicesRead, ScopeFoldersRead,
		ScopeMembersRead, ScopeTeamsRead, ScopeServiceAccountsRead,
	},
	RoleEditor: {
		ScopeDiagramsRead, ScopeDocsRead, ScopeMapsRead, ScopeServicesRead, ScopeFoldersRead,
		ScopeMembersRead, ScopeTeamsRead, ScopeServiceAccountsRead,
		ScopeDiagramsWrite, ScopeDocsWrite, ScopeMapsWrite, ScopeServicesWrite, ScopeFoldersWrite,
	},
	RoleAdmin: {
		"diagrams:*", "docs:*", "maps:*", "services:*", "folders:*",
		"members:*", "teams:*", "serviceaccounts:*", "org:*",
	},
}

// ScopesForRole returns the explicit scope slice for role, or nil for an
// unknown role (explicit deny — no fallback).
func ScopesForRole(role Role) []Scope {
	return RoleScopes[role]
}

// knownResources is the set of resource prefixes that may carry a "<resource>:*"
// wildcard. Derived from AllScopes so wildcards stay anchored to real resources.
var knownResources = func() map[string]bool {
	m := make(map[string]bool)
	for _, s := range AllScopes {
		if res, _, ok := strings.Cut(string(s), ":"); ok {
			m[res] = true
		}
	}
	return m
}()

var validScopes = func() map[Scope]bool {
	m := make(map[Scope]bool, len(AllScopes))
	for _, s := range AllScopes {
		m[s] = true
	}
	return m
}()

// ValidScope reports whether s is a known concrete scope or a "<resource>:*"
// wildcard for a known resource.
func ValidScope(s string) bool {
	if res, action, ok := strings.Cut(s, ":"); ok && action == "*" {
		return knownResources[res]
	}
	return validScopes[Scope(s)]
}

// Has reports whether the granted scope list satisfies want. A granted concrete
// scope must match want exactly; a granted "<resource>:*" wildcard satisfies any
// want of that resource.
func Has(scopes []string, want Scope) bool {
	wantRes, _, _ := strings.Cut(string(want), ":")
	for _, s := range scopes {
		if Scope(s) == want {
			return true
		}
		if res, action, ok := strings.Cut(s, ":"); ok && action == "*" && res == wantRes {
			return true
		}
	}
	return false
}
