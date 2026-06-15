package authz

import "strings"

// Scope is a named permission in the form "<resource>:<action>" (e.g.
// "diagrams:create"). Both service-account tokens and human users (via their
// org role, resolved through RoleScopes) are authorized against scopes.
//
// A granted scope may also be a per-resource wildcard "<resource>:*", which
// satisfies any concrete action of that resource. Required scopes are always
// concrete. Non-deterministic catch-alls (e.g. "all:*", "manage:*") are not valid.
type Scope string

const (
	ScopeDiagramsView   Scope = "diagrams:view"
	ScopeDiagramsCreate Scope = "diagrams:create"
	ScopeDiagramsEdit   Scope = "diagrams:edit"
	ScopeDiagramsDelete Scope = "diagrams:delete"

	ScopeMapsView   Scope = "maps:view"
	ScopeMapsCreate Scope = "maps:create"
	ScopeMapsEdit   Scope = "maps:edit"
	ScopeMapsDelete Scope = "maps:delete"

	ScopeServicesView   Scope = "services:view"
	ScopeServicesCreate Scope = "services:create"
	ScopeServicesEdit   Scope = "services:edit"
	ScopeServicesDelete Scope = "services:delete"

	ScopeFoldersView   Scope = "folders:view"
	ScopeFoldersCreate Scope = "folders:create"
	ScopeFoldersEdit   Scope = "folders:edit"
	ScopeFoldersDelete Scope = "folders:delete"

	ScopeMembersView       Scope = "members:view"
	ScopeMembersAdd        Scope = "members:add"
	ScopeMembersRemove     Scope = "members:remove"
	ScopeMembersUpdateRole Scope = "members:update-role"

	ScopeTeamsView         Scope = "teams:view"
	ScopeTeamsCreate       Scope = "teams:create"
	ScopeTeamsEdit         Scope = "teams:edit"
	ScopeTeamsDelete       Scope = "teams:delete"
	ScopeTeamsAddMember    Scope = "teams:add-member"
	ScopeTeamsRemoveMember Scope = "teams:remove-member"

	ScopeServiceAccountsView        Scope = "serviceaccounts:view"
	ScopeServiceAccountsCreate      Scope = "serviceaccounts:create"
	ScopeServiceAccountsEdit        Scope = "serviceaccounts:edit"
	ScopeServiceAccountsDelete      Scope = "serviceaccounts:delete"
	ScopeServiceAccountsCreateToken Scope = "serviceaccounts:create-token"
	ScopeServiceAccountsRevokeToken Scope = "serviceaccounts:revoke-token"

	ScopeInvitationsView   Scope = "invitations:view"
	ScopeInvitationsCreate Scope = "invitations:create"
	ScopeInvitationsRevoke Scope = "invitations:revoke"
	ScopeInvitationsResend Scope = "invitations:resend"

	ScopeOrgUpdate Scope = "org:update"
	ScopeOrgDelete Scope = "org:delete"
)

// AllScopes is the catalog of concrete grantable scopes, returned by the discovery endpoint.
var AllScopes = []Scope{
	ScopeDiagramsView, ScopeDiagramsCreate, ScopeDiagramsEdit, ScopeDiagramsDelete,
	ScopeMapsView, ScopeMapsCreate, ScopeMapsEdit, ScopeMapsDelete,
	ScopeServicesView, ScopeServicesCreate, ScopeServicesEdit, ScopeServicesDelete,
	ScopeFoldersView, ScopeFoldersCreate, ScopeFoldersEdit, ScopeFoldersDelete,
	ScopeMembersView, ScopeMembersAdd, ScopeMembersRemove, ScopeMembersUpdateRole,
	ScopeTeamsView, ScopeTeamsCreate, ScopeTeamsEdit, ScopeTeamsDelete, ScopeTeamsAddMember, ScopeTeamsRemoveMember,
	ScopeServiceAccountsView, ScopeServiceAccountsCreate, ScopeServiceAccountsEdit, ScopeServiceAccountsDelete,
	ScopeServiceAccountsCreateToken, ScopeServiceAccountsRevokeToken,
	ScopeInvitationsView, ScopeInvitationsCreate, ScopeInvitationsRevoke, ScopeInvitationsResend,
	ScopeOrgUpdate, ScopeOrgDelete,
}

// RoleScopes is the static map from an org role to its explicit scope set.
// Each slice is written out literally — there is no inheritance between roles.
// The admin role uses per-resource wildcards, which are deterministic.
var RoleScopes = map[Role][]Scope{
	RoleViewer: {
		ScopeDiagramsView, ScopeMapsView, ScopeServicesView, ScopeFoldersView,
		ScopeMembersView, ScopeTeamsView, ScopeServiceAccountsView, ScopeInvitationsView,
	},
	RoleEditor: {
		ScopeDiagramsView, ScopeMapsView, ScopeServicesView, ScopeFoldersView,
		ScopeMembersView, ScopeTeamsView, ScopeServiceAccountsView, ScopeInvitationsView,
		ScopeDiagramsCreate, ScopeDiagramsEdit,
		ScopeMapsCreate, ScopeMapsEdit,
		ScopeServicesCreate, ScopeServicesEdit,
		ScopeFoldersCreate, ScopeFoldersEdit,
	},
	RoleAdmin: {
		"diagrams:*", "maps:*", "services:*", "folders:*",
		"members:*", "teams:*", "serviceaccounts:*", "invitations:*", "org:*",
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
