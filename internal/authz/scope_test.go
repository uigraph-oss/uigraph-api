package authz

import "testing"

func TestValidScope(t *testing.T) {
	if !ValidScope("diagrams:create") {
		t.Fatal("diagrams:create should be valid")
	}
	if !ValidScope("diagrams:*") {
		t.Fatal("diagrams:* wildcard should be valid")
	}
	if ValidScope("diagrams:explode") {
		t.Fatal("diagrams:explode should be invalid")
	}
	if ValidScope("all:*") {
		t.Fatal("all:* should be invalid")
	}
	if ValidScope("manage:x") {
		t.Fatal("manage:x should be invalid")
	}
	if ValidScope("") {
		t.Fatal("empty scope should be invalid")
	}
}

func TestHas(t *testing.T) {
	scopes := []string{"diagrams:view", "maps:edit"}
	if !Has(scopes, ScopeDiagramsView) {
		t.Fatal("expected diagrams:view to be present")
	}
	if Has(scopes, ScopeDiagramsCreate) {
		t.Fatal("did not expect diagrams:create")
	}
	if Has(nil, ScopeDiagramsView) {
		t.Fatal("nil scopes should grant nothing")
	}
}

func TestHasWildcard(t *testing.T) {
	scopes := []string{"diagrams:*"}
	if !Has(scopes, ScopeDiagramsCreate) {
		t.Fatal("diagrams:* should satisfy diagrams:create")
	}
	if !Has(scopes, ScopeDiagramsDelete) {
		t.Fatal("diagrams:* should satisfy diagrams:delete")
	}
	if Has(scopes, ScopeMapsCreate) {
		t.Fatal("diagrams:* must not leak into maps")
	}
}

func TestScopesForRole(t *testing.T) {
	viewer := toStrings(ScopesForRole(RoleViewer))
	if !Has(viewer, ScopeDiagramsView) {
		t.Fatal("viewer should have diagrams:view")
	}
	if Has(viewer, ScopeDiagramsCreate) {
		t.Fatal("viewer must not have diagrams:create")
	}

	editor := toStrings(ScopesForRole(RoleEditor))
	if !Has(editor, ScopeDiagramsCreate) || !Has(editor, ScopeDiagramsEdit) {
		t.Fatal("editor should have diagrams:create and diagrams:edit")
	}
	if Has(editor, ScopeDiagramsDelete) {
		t.Fatal("editor must not have diagrams:delete")
	}
	if Has(editor, ScopeTeamsCreate) {
		t.Fatal("editor must not have management write scopes")
	}

	admin := toStrings(ScopesForRole(RoleAdmin))
	for _, s := range AllScopes {
		if !Has(admin, s) {
			t.Fatalf("admin should satisfy %s", s)
		}
	}

	if ScopesForRole(Role("ghost")) != nil {
		t.Fatal("unknown role should resolve to nil (deny)")
	}
}

func toStrings(ss []Scope) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = string(s)
	}
	return out
}
