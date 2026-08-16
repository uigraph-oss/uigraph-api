package api

import (
	"context"
	"testing"

	"github.com/uigraph/app/internal/authz"
	"github.com/uigraph/app/internal/identity"
)

type panicAuthorizer struct{}

func (panicAuthorizer) ScopesForUser(context.Context, string, string) ([]authz.Scope, error) {
	panic("service-account authorization must not query user roles")
}

func TestPrincipalScopesRejectsServiceAccountFromAnotherOrg(t *testing.T) {
	principal := identity.Principal{Kind: identity.PrincipalServiceAccount, OrgID: "org-a", Scopes: []string{"integrations:write"}}
	if _, authorized := principalScopes(context.Background(), panicAuthorizer{}, principal, "org-b"); authorized {
		t.Fatal("cross-org service account was authorized")
	}
}

func TestPrincipalScopesAllowsServiceAccountInPathOrg(t *testing.T) {
	principal := identity.Principal{Kind: identity.PrincipalServiceAccount, OrgID: "org-a", Scopes: []string{"integrations:write"}}
	scopes, authorized := principalScopes(context.Background(), panicAuthorizer{}, principal, "org-a")
	if !authorized || len(scopes) != 1 || scopes[0] != "integrations:write" {
		t.Fatalf("same-org service account authorization = %v, %v", scopes, authorized)
	}
}
